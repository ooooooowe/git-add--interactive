package main

import (
	"fmt"
	"os"
	"regexp"

	"github.com/cwarden/git-add--interactive/internal/git"
	"github.com/cwarden/git-add--interactive/internal/ui"
)

func main() {
	args := os.Args[1:]
	patchMode, patchRevision, files, err := processArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	repo, err := git.NewRepository(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	app := ui.NewApp(repo)

	if patchMode != "" {
		if err := app.RunPatchMode(patchMode, patchRevision, files); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	} else {
		if err := app.RunInteractive(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}
}

func processArgs(args []string) (patchMode, patchRevision string, files []string, err error) {
	if len(args) == 0 {
		return "", "", nil, nil
	}

	arg := args[0]
	args = args[1:]

	patchRe := regexp.MustCompile(`^--patch(?:=(.*))?$`)
	if matches := patchRe.FindStringSubmatch(arg); len(matches) > 0 {
		if len(matches) > 1 && matches[1] != "" {
			mode := matches[1]
			switch mode {
			case "reset":
				patchMode = "reset_head"
				patchRevision = "HEAD"
				if len(args) > 0 {
					arg = args[0]
					args = args[1:]
					if arg != "--" {
						patchRevision = arg
						if arg == "HEAD" {
							patchMode = "reset_head"
						} else {
							patchMode = "reset_nothead"
						}
						if len(args) > 0 && args[0] == "--" {
							args = args[1:]
						}
					} else {
						// Skip the "--"
					}
				}
			case "checkout":
				if len(args) > 0 {
					arg = args[0]
					args = args[1:]
					if arg == "--" {
						patchMode = "checkout_index"
					} else {
						patchRevision = arg
						if arg == "HEAD" {
							patchMode = "checkout_head"
						} else {
							patchMode = "checkout_nothead"
						}
						if len(args) > 0 && args[0] == "--" {
							args = args[1:]
						}
					}
				} else {
					patchMode = "checkout_index"
				}
			case "worktree":
				if len(args) > 0 {
					arg = args[0]
					args = args[1:]
					if arg == "--" {
						patchMode = "checkout_index"
					} else {
						patchRevision = arg
						if arg == "HEAD" {
							patchMode = "worktree_head"
						} else {
							patchMode = "worktree_nothead"
						}
						if len(args) > 0 && args[0] == "--" {
							args = args[1:]
						}
					}
				} else {
					patchMode = "checkout_index"
				}
			case "stage", "stash":
				patchMode = mode
				if len(args) > 0 && args[0] == "--" {
					args = args[1:]
				}
			default:
				return "", "", nil, fmt.Errorf("unknown --patch mode: %s", mode)
			}
		} else {
			patchMode = "stage"
			if len(args) > 0 && args[0] == "--" {
				args = args[1:]
			}
		}

		return patchMode, patchRevision, args, nil
	} else if arg == "--" {
		// Handle: ./git-add-interactive -- path1 path2
		return "", "", args, nil
	} else {
		// Handle: ./git-add-interactive path1 path2 (assume stage mode)
		return "stage", "", append([]string{arg}, args...), nil
	}
}
