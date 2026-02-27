package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CommandRunner abstracts running external commands.
type CommandRunner interface {
	RunCommand(dir, name string, args ...string) (string, error)
}

type execRunner struct{}

func (r *execRunner) RunCommand(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
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

	logInfo("Cloning skeleton remote repository to the new local repository.")
	if _, err := runner.RunCommand(cloneDir, "git", "clone", "--origin", srcRepo,
		"git@github.com:"+srcOrg+"/"+srcRepo+".git", destRepo); err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}

	logInfo("Adding a new remote origin for the repository.")
	if _, err := runner.RunCommand(destRepoDir, "git", "remote", "add", "origin",
		"git@github.com:"+destOrg+"/"+destRepo+".git"); err != nil {
		return fmt.Errorf("git remote add failed: %w", err)
	}

	logInfo("Setting base repository for pull request and issue creation.")
	if _, err := runner.RunCommand(destRepoDir, "git", "config", "--local", "--add",
		"remote.origin.gh-resolved", "base"); err != nil {
		return fmt.Errorf("git config failed: %w", err)
	}

	logInfo("Disabling pushing to the upstream (parent) repository.")
	if _, err := runner.RunCommand(destRepoDir, "git", "remote", "set-url", "--push",
		srcRepo, "no_push"); err != nil {
		return fmt.Errorf("git remote set-url failed: %w", err)
	}

	logInfo("Searching and replacing repository name in source files.")
	if err := replaceInFiles(destRepoDir, srcOrg, srcRepo, destOrg, destRepo); err != nil {
		return fmt.Errorf("replaceInFiles failed: %w", err)
	}

	logInfo("Checking for bump-version script.")
	bumpVersionPath := filepath.Join(destRepoDir, "bump-version")
	if _, err := os.Stat(bumpVersionPath); err == nil {
		logOk("bump-version script found. Resetting version to %s.", versionReset)

		currentVersion, err := runner.RunCommand(destRepoDir, "./bump-version", "show")
		currentVersion = strings.TrimSpace(currentVersion)
		if err != nil || currentVersion == "" {
			logError("Failed to determine current version. Skipping version reset.")
		} else if currentVersion == versionReset {
			logOk("Current version is already %s. Skipping version reset.", versionReset)
		} else {
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
				if _, err := os.Stat(vfPath); err == nil {
					logInfo("Resetting version in %s to %s.", vf, versionReset)
					if err := replaceInFile(vfPath, currentVersion, versionReset); err != nil {
						return fmt.Errorf("replaceInFile failed for %s: %w", vf, err)
					}
				} else {
					logWarn("Expected version file %s not found.", vf)
				}
			}

			logInfo("Staging version reset files.")
			gitAddArgs := append([]string{"add", "--verbose"}, versionFiles...)
			if _, err := runner.RunCommand(destRepoDir, "git", gitAddArgs...); err != nil {
				return fmt.Errorf("git add version files failed: %w", err)
			}

			logInfo("Committing version reset to the %s branch.", defaultBranch)
			if _, err := runner.RunCommand(destRepoDir, "git", "commit", "--message",
				"Reset version to "+versionReset+" for new repository"); err != nil {
				return fmt.Errorf("git commit version reset failed: %w", err)
			}
		}
	} else {
		logWarn("bump-version script not found. Skipping version reset.")
	}

	logInfo("Staging modified files.")
	if _, err := runner.RunCommand(destRepoDir, "git", "add", "--verbose", "."); err != nil {
		return fmt.Errorf("git add failed: %w", err)
	}

	logInfo("Committing staged files to the %s branch.", defaultBranch)
	if _, err := runner.RunCommand(destRepoDir, "git", "commit", "--message",
		"Rename repository references after clone"); err != nil {
		return fmt.Errorf("git commit rename failed: %w", err)
	}

	logInfo("Creating the lineage.yml file.")
	lineagePath := filepath.Join(destRepoDir, ".github", "lineage.yml")
	lineageContent := fmt.Sprintf("---\nlineage:\n  skeleton:\n    remote-url: https://github.com/%s/%s.git\nversion: \"1\"\n",
		srcOrg, srcRepo)
	if err := os.WriteFile(lineagePath, []byte(lineageContent), 0o644); err != nil {
		return fmt.Errorf("write lineage.yml failed: %w", err)
	}

	logInfo("Staging modified files.")
	if _, err := runner.RunCommand(destRepoDir, "git", "add", "--verbose", "."); err != nil {
		return fmt.Errorf("git add lineage failed: %w", err)
	}

	logInfo("Committing staged files to the %s branch.", defaultBranch)
	if _, err := runner.RunCommand(destRepoDir, "git", "commit", "--message",
		"Add lineage configuration"); err != nil {
		return fmt.Errorf("git commit lineage failed: %w", err)
	}

	logInfo("Creating first-commits branch.")
	if _, err := runner.RunCommand(destRepoDir, "git", "checkout", "-b", "first-commits"); err != nil {
		return fmt.Errorf("git checkout -b first-commits failed: %w", err)
	}

	// Check if remote repo exists.
	repoName, err := runner.RunCommand("", "gh", "repo", "view", "--json", "name", "--jq", ".name",
		destOrg+"/"+destRepo)
	status := "unknown"
	if err == nil && strings.TrimSpace(repoName) != "" {
		logOk("%s/%s exists.", destOrg, destRepo)
		status = "exists"
	} else {
		logWarn("%s/%s does not yet exist.", destOrg, destRepo)
		logInfo("Attempting to create a new remote repository.")
		status = "created"
		if _, err := runner.RunCommand("", "gh", "repo", "create", "--public",
			destOrg+"/"+destRepo); err != nil {
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
		if _, err := runner.RunCommand(destRepoDir, "git", "push", "origin",
			defaultBranch, "first-commits", "--set-upstream"); err != nil {
			return fmt.Errorf("git push failed: %w", err)
		}
		logInfo("Opening a new pull request for the first-commits branch.")
		if _, err := runner.RunCommand(destRepoDir, "gh", "pr", "create",
			"--title", "First commits", "--assignee=@me", "--web"); err != nil {
			return fmt.Errorf("gh pr create failed: %w", err)
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
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			base := info.Name()
			// Skip .git and .github: .git contains internal git data and .github
			// contains CI configuration that should not have repo refs replaced.
			// Note: .github/lineage.yml is created fresh after this step.
			if base == ".git" || base == ".github" {
				return filepath.SkipDir
			}
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			// Skip unreadable files.
			return nil
		}
		original := string(content)
		updated := strings.ReplaceAll(original, srcOrg+"/"+srcRepo, destOrg+"/"+destRepo)
		updated = strings.ReplaceAll(updated, srcRepo, destRepo)
		if updated != original {
			if err := os.WriteFile(path, []byte(updated), info.Mode()); err != nil {
				return fmt.Errorf("write %s: %w", path, err)
			}
		}
		return nil
	})
}

func replaceInFile(path, oldStr, newStr string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	updated := strings.ReplaceAll(string(content), oldStr, newStr)
	return os.WriteFile(path, []byte(updated), 0o644)
}
