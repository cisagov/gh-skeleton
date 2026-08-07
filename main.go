package main

import (
	"fmt"
	"os"

	"github.com/cli/go-gh/v2/pkg/api"
)

const (
	defaultBranch  = "develop"
	defaultDestOrg = "cisagov"
	defaultSrcOrg  = "cisagov"
	versionReset   = "0.0.1"
)

var usage = `GitHub CLI extension to start a new GitHub project from a skeleton GitHub repository.

Usage:
  gh skeleton (-h | --help)
  gh skeleton (-V | --version)
  gh skeleton list [--src-org <name>]
  gh skeleton clone [options] <parent-repo-name> <new-repo-name>

Options:
  -c --change-dir <dir> Create clone in this directory.
  -d --dest-org <name>  Organization to create clone into [default: ` + defaultDestOrg + `].
  -h --help             Show this message.
  -s --src-org <name>   Organization to search for skeletons [default: ` + defaultSrcOrg + `].
  -V --version          Show the version of this extension.
`

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		fmt.Print(usage)
		os.Exit(0)
	}

	changeDir := ""
	destOrg := defaultDestOrg
	srcOrg := defaultSrcOrg
	var positional []string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-c", "--change-dir":
			i++
			if i >= len(args) {
				fmt.Fprintf(os.Stderr, "Error: %s requires an argument\n", args[i-1]) // #nosec G705 -- writing CLI error to stderr
				os.Exit(255)
			}
			changeDir = args[i]
		case "-d", "--dest-org":
			i++
			if i >= len(args) {
				fmt.Fprintf(os.Stderr, "Error: %s requires an argument\n", args[i-1]) // #nosec G705 -- writing CLI error to stderr
				os.Exit(255)
			}
			destOrg = args[i]
		case "-h", "--help":
			fmt.Print(usage)
			os.Exit(0)
		case "-V", "--version":
			fmt.Println(version)
			os.Exit(0)
		case "-s", "--src-org":
			i++
			if i >= len(args) {
				fmt.Fprintf(os.Stderr, "Error: %s requires an argument\n", args[i-1]) // #nosec G705 -- writing CLI error to stderr
				os.Exit(255)
			}
			srcOrg = args[i]
		default:
			if len(args[i]) > 0 && args[i][0] == '-' {
				fmt.Fprintf(os.Stderr, "Error: Unsupported skeleton flag %s\n", args[i]) // #nosec G705 -- writing CLI error to stderr
				os.Exit(255)
			}
			positional = append(positional, args[i])
		}
	}

	if len(positional) == 0 {
		fmt.Print(usage)
		os.Exit(0)
	}

	command := positional[0]

	switch command {
	case "list":
		gqlClient, err := api.DefaultGraphQLClient()
		if err != nil {
			logError("Failed to create GraphQL client: %v", err)
			os.Exit(1)
		}
		if err := listSkeletons(srcOrg, gqlClient); err != nil {
			logError("list failed: %v", err)
			os.Exit(1)
		}

	case "clone":
		if len(positional) < 3 {
			fmt.Fprintf(os.Stderr, "Clone command requires <parent-repo-name> <new-repo-name>\n\n")
			fmt.Print(usage)
			os.Exit(255)
		}
		srcRepo := positional[1]
		destRepo := positional[2]

		restClient, err := api.DefaultRESTClient()
		if err != nil {
			logError("Failed to create REST client: %v", err)
			os.Exit(1)
		}

		runner := &execRunner{}
		if err := runClone(srcRepo, destRepo, srcOrg, destOrg, changeDir, runner, restClient); err != nil {
			logError("clone failed: %v", err)
			os.Exit(1)
		}
		if err := configureRepoOptions(destRepo, destOrg, defaultBranch, restClient); err != nil {
			logError("configureRepoOptions failed: %v", err)
			os.Exit(1)
		}
		if err := configureBranchProtection(destRepo, destOrg, defaultBranch, restClient); err != nil {
			logError("configureBranchProtection failed: %v", err)
			os.Exit(1)
		}
		logOk("Success!")

	default:
		fmt.Fprintf(os.Stderr, "Unknown command %s\n", command) // #nosec G705 -- writing CLI error to stderr
		fmt.Print(usage)
		os.Exit(255)
	}
}
