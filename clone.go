package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	ghbrowser "github.com/cli/go-gh/v2/pkg/browser"
	git "github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

// CommandRunner abstracts running external commands (used for bump-version).
type CommandRunner interface {
	RunCommand(dir, name string, args ...string) (string, error)
}

type execRunner struct{}

func (r *execRunner) RunCommand(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...) // #nosec G204,G702 -- callers use hardcoded command names
	if dir != "" {
		cmd.Dir = dir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%w: %s", err, stderr.String())
	}
	return strings.TrimRight(stdout.String(), "\n"), nil
}

// getSSHAuth returns the best available SSH authentication method.
func getSSHAuth() (gitssh.AuthMethod, error) {
	// Try SSH agent first.
	auth, err := gitssh.NewSSHAgentAuth("git")
	if err == nil {
		return auth, nil
	}
	// Fall back to common key files.
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("no SSH authentication available: %w", err)
	}
	for _, keyFile := range []string{"id_ed25519", "id_rsa", "id_ecdsa", "id_dsa"} {
		keyPath := filepath.Join(homeDir, ".ssh", keyFile) // #nosec G304 -- path built from home dir and hardcoded filename
		auth, err := gitssh.NewPublicKeysFromFile("git", keyPath, "")
		if err == nil {
			return auth, nil
		}
	}
	return nil, fmt.Errorf("no SSH authentication available: ssh-agent not running and no default key files found")
}

// createRemoteRepo creates a new public repository on GitHub using the REST API.
// It tries to create in the organization first; if that fails, it falls back to the
// authenticated user's account.
func createRemoteRepo(destRepo, destOrg string, client RESTClient) error {
	payload := map[string]interface{}{
		"name":    destRepo,
		"private": false,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("json marshal failed: %w", err)
	}

	var resp interface{}
	// Try organization endpoint first.
	if err := client.Post("orgs/"+destOrg+"/repos", bytes.NewReader(data), &resp); err == nil {
		return nil
	}
	// Fall back to user endpoint (for personal accounts).
	return client.Post("user/repos", bytes.NewReader(data), &resp)
}

func runClone(srcRepo, destRepo, srcOrg, destOrg, changeDir string, runner CommandRunner, client RESTClient) error {
	// Get authenticated user.
	var user struct {
		Login string
	}
	if err := client.Get("user", &user); err != nil {
		return fmt.Errorf("failed to get authenticated user: %w", err)
	}
	logOk("You are logged into GitHub as %s.", user.Login)

	logInfo("Creating %s/%s from skeleton %s/%s.", destOrg, destRepo, srcOrg, srcRepo)

	// Determine clone base directory.
	cloneDir := "."
	if changeDir != "" {
		cloneDir = changeDir
	}
	destRepoDir := filepath.Join(cloneDir, destRepo)

	// Set up SSH auth for git operations.
	sshAuth, err := getSSHAuth()
	if err != nil {
		return fmt.Errorf("SSH auth setup failed: %w", err)
	}

	logInfo("Cloning skeleton remote repository to the new local repository.")
	repo, err := git.PlainClone(destRepoDir, false, &git.CloneOptions{
		URL:        "git@github.com:" + srcOrg + "/" + srcRepo + ".git",
		RemoteName: srcRepo,
		Auth:       sshAuth,
	})
	if err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}

	logInfo("Adding a new remote origin for the repository.")
	if _, err := repo.CreateRemote(&gitconfig.RemoteConfig{
		Name: "origin",
		URLs: []string{"git@github.com:" + destOrg + "/" + destRepo + ".git"},
	}); err != nil {
		return fmt.Errorf("git remote add failed: %w", err)
	}

	logInfo("Setting base repository for pull request and issue creation.")
	logInfo("Disabling pushing to the upstream (parent) repository.")
	cfg, err := repo.Config()
	if err != nil {
		return fmt.Errorf("failed to get repo config: %w", err)
	}
	cfg.Raw.AddOption("remote", "origin", "gh-resolved", "base")
	cfg.Raw.SetOption("remote", srcRepo, "pushurl", "no_push")
	if err := repo.SetConfig(cfg); err != nil {
		return fmt.Errorf("failed to set repo config: %w", err)
	}

	logInfo("Searching and replacing repository name in source files.")
	if err := replaceInFiles(destRepoDir, srcOrg, srcRepo, destOrg, destRepo); err != nil {
		return fmt.Errorf("replaceInFiles failed: %w", err)
	}

	// Get worktree for subsequent git operations.
	w, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	logInfo("Checking for bump-version script.")
	bumpVersionPath := filepath.Join(destRepoDir, "bump-version")
	if _, err := os.Stat(bumpVersionPath); err == nil { // #nosec G703 -- path constructed from validated repo name
		logOk("bump-version script found. Resetting version to %s.", versionReset)

		currentVersion, err := runner.RunCommand(destRepoDir, "./bump-version", "show")
		currentVersion = strings.TrimSpace(currentVersion)
		switch {
		case err != nil || currentVersion == "":
			logError("Failed to determine current version. Skipping version reset.")
		case currentVersion == versionReset:
			logOk("Current version is already %s. Skipping version reset.", versionReset)
		default:
			logInfo("Current version is %s. Resetting to %s.", currentVersion, versionReset)

			listOutput, err := runner.RunCommand(destRepoDir, "./bump-version", "--list-files")
			if err != nil {
				return fmt.Errorf("bump-version --list-files failed: %w", err)
			}
			var versionFiles []string
			for _, line := range strings.Split(listOutput, "\n") {
				if line != "" {
					versionFiles = append(versionFiles, line)
				}
			}

			for _, vf := range versionFiles {
				vfPath := filepath.Join(destRepoDir, vf)
				if _, err := os.Stat(vfPath); err == nil { // #nosec G703 -- path constructed from validated repo name
					logInfo("Resetting version in %s to %s.", vf, versionReset)
					if err := replaceInFile(vfPath, currentVersion, versionReset); err != nil {
						return fmt.Errorf("replaceInFile failed for %s: %w", vf, err)
					}
				} else {
					logWarn("Expected version file %s not found.", vf)
				}
			}

			logInfo("Staging version reset files.")
			for _, vf := range versionFiles {
				if _, err := w.Add(vf); err != nil {
					return fmt.Errorf("git add version file %s failed: %w", vf, err)
				}
			}

			logInfo("Committing version reset to the %s branch.", defaultBranch)
			if _, err := w.Commit("Reset version to "+versionReset+" for new repository", &git.CommitOptions{}); err != nil {
				return fmt.Errorf("git commit version reset failed: %w", err)
			}
		}
	} else {
		logWarn("bump-version script not found. Skipping version reset.")
	}

	logInfo("Staging modified files.")
	if err := w.AddWithOptions(&git.AddOptions{All: true}); err != nil {
		return fmt.Errorf("git add failed: %w", err)
	}

	logInfo("Committing staged files to the %s branch.", defaultBranch)
	if _, err := w.Commit("Rename repository references after clone", &git.CommitOptions{}); err != nil {
		return fmt.Errorf("git commit rename failed: %w", err)
	}

	logInfo("Creating the lineage.yml file.")
	lineagePath := filepath.Join(destRepoDir, ".github", "lineage.yml")
	lineageContent := fmt.Sprintf("---\nlineage:\n  skeleton:\n    remote-url: https://github.com/%s/%s.git\nversion: \"1\"\n",
		srcOrg, srcRepo)
	if err := os.WriteFile(lineagePath, []byte(lineageContent), 0o600); err != nil { // #nosec G703 -- path constructed from validated repo name
		return fmt.Errorf("write lineage.yml failed: %w", err)
	}

	logInfo("Staging modified files.")
	if err := w.AddWithOptions(&git.AddOptions{All: true}); err != nil {
		return fmt.Errorf("git add lineage failed: %w", err)
	}

	logInfo("Committing staged files to the %s branch.", defaultBranch)
	if _, err := w.Commit("Add lineage configuration", &git.CommitOptions{}); err != nil {
		return fmt.Errorf("git commit lineage failed: %w", err)
	}

	logInfo("Creating first-commits branch.")
	if err := w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("first-commits"),
		Create: true,
	}); err != nil {
		return fmt.Errorf("git checkout -b first-commits failed: %w", err)
	}

	// Check if remote repo exists.
	var repoResp struct {
		Name string
	}
	repoExists := client.Get("repos/"+destOrg+"/"+destRepo, &repoResp) == nil
	status := "unknown"
	if repoExists {
		logOk("%s/%s exists.", destOrg, destRepo)
		status = "exists"
	} else {
		logWarn("%s/%s does not yet exist.", destOrg, destRepo)
		logInfo("Attempting to create a new remote repository.")
		status = "created"
		if err := createRemoteRepo(destRepo, destOrg, client); err != nil {
			logWarn("The remote repository %s/%s could not be created.", destOrg, destRepo)
			status = "failed"
		}
	}

	switch status {
	case "created":
		logOk("The remote repository %s/%s was successfully created.", destOrg, destRepo)
		fallthrough
	case "exists":
		logInfo("Pushing %s and first-commits branches to the remote.", defaultBranch)
		pushErr := repo.Push(&git.PushOptions{
			RemoteName: "origin",
			RefSpecs: []gitconfig.RefSpec{
				gitconfig.RefSpec("refs/heads/" + defaultBranch + ":refs/heads/" + defaultBranch),
				gitconfig.RefSpec("refs/heads/first-commits:refs/heads/first-commits"),
			},
			Auth: sshAuth,
		})
		if pushErr != nil {
			return fmt.Errorf("git push failed: %w", pushErr)
		}
		// Set tracking info (equivalent to --set-upstream).
		if pushCfg, err := repo.Config(); err == nil {
			pushCfg.Branches[defaultBranch] = &gitconfig.Branch{
				Name:   defaultBranch,
				Remote: "origin",
				Merge:  plumbing.NewBranchReferenceName(defaultBranch),
			}
			pushCfg.Branches["first-commits"] = &gitconfig.Branch{
				Name:   "first-commits",
				Remote: "origin",
				Merge:  plumbing.NewBranchReferenceName("first-commits"),
			}
			_ = repo.SetConfig(pushCfg)
		}
		logInfo("Opening a new pull request for the first-commits branch.")
		prURL := "https://github.com/" + destOrg + "/" + destRepo +
			"/compare/" + defaultBranch + "...first-commits" +
			"?quick_pull=1&title=" + url.QueryEscape("First commits") +
			"&assignees=" + url.QueryEscape(user.Login)
		b := ghbrowser.New("", os.Stdout, os.Stderr)
		if err := b.Browse(prURL); err != nil {
			return fmt.Errorf("opening pull request in browser failed: %w", err)
		}
	case "failed":
		cdDir := destRepo
		if changeDir != "" {
			cdDir = changeDir + "/" + destRepo
		}
		fmt.Printf("%s/%s was created locally from the %s skeleton.\n", destOrg, destRepo, srcRepo)
		fmt.Printf("A remote repository was unable to be created automatically.\n")
		fmt.Printf("Please manually create the %s/%s repository on GitHub.\n\n", destOrg, destRepo)
		fmt.Printf("Once %s/%s is created use the following commands to push\n", destOrg, destRepo)
		fmt.Printf("the local repository to the remote and create the initial pull request:\n")
		fmt.Printf("    cd %s\n", cdDir)
		fmt.Printf("    git push origin %s first-commits --set-upstream\n", defaultBranch)
		fmt.Printf("    gh pr create --title \"First commits\" --assignee=@me --web\n\n")
		return fmt.Errorf("remote repository creation failed")
	}

	return nil
}

func replaceInFiles(dir, srcOrg, srcRepo, destOrg, destRepo string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error { // #nosec G703 -- dir is the cloned repo directory
		if err != nil {
			return err
		}
		if info.IsDir() {
			base := info.Name()
			// Skip .git and .github, matching the original bash script behavior.
			// .git contains internal git data. .github is skipped so that
			// lineage.yml (which will be written fresh) and other CI
			// configuration files are preserved as-is from the skeleton.
			if base == ".git" || base == ".github" {
				return filepath.SkipDir
			}
			return nil
		}
		content, err := os.ReadFile(filepath.Clean(path)) // #nosec G122 -- path is from filepath.Walk over a trusted cloned directory
		if err != nil {
			// Skip unreadable files.
			return nil
		}
		original := string(content)
		updated := strings.ReplaceAll(original, srcOrg+"/"+srcRepo, destOrg+"/"+destRepo)
		updated = strings.ReplaceAll(updated, srcRepo, destRepo)
		if updated != original {
			if err := os.WriteFile(path, []byte(updated), info.Mode()); err != nil { // #nosec G122,G703 -- path is from filepath.Walk over a trusted cloned directory
				return fmt.Errorf("write %s: %w", path, err)
			}
		}
		return nil
	})
}

func replaceInFile(path, oldStr, newStr string) error {
	cleanPath := filepath.Clean(path)
	info, err := os.Stat(cleanPath)
	if err != nil {
		return err
	}
	content, err := os.ReadFile(cleanPath)
	if err != nil {
		return err
	}
	updated := strings.ReplaceAll(string(content), oldStr, newStr)
	return os.WriteFile(cleanPath, []byte(updated), info.Mode()) // #nosec G703 -- cleanPath is filepath.Clean of a version file within the cloned repo
}
