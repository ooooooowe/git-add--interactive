package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/cwarden/git-add--interactive/internal/git"
	"github.com/cwarden/git-add--interactive/internal/ui"
)

func main() {
	patchMode, patchRevision, files, err := parseFlags()
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

func parseFlags() (patchMode, patchRevision string, files []string, err error) {
	return processArgs(os.Args[1:])
}

func processArgs(args []string) (patchMode, patchRevision string, files []string, err error) {
	var patchFlag string
	var patchProvided bool

	// Create a new flag set to avoid conflicts with testing
	fs := flag.NewFlagSet("git-add--interactive", flag.ContinueOnError)
	fs.StringVar(&patchFlag, "patch", "", "enable patch mode (stage, reset, checkout, worktree, stash)")

	// Disable default error output from flag parsing
	fs.SetOutput(&nullWriter{})

	// Parse arguments
	err = fs.Parse(args)
	if err != nil {
		// Convert flag errors to our expected format
		if strings.Contains(err.Error(), "flag provided but not defined") {
			return "", "", nil, fmt.Errorf("unknown option: %s", extractUnknownFlag(err.Error()))
		}
		return "", "", nil, err
	}

	// Check if --patch flag was provided (even without value)
	for _, arg := range args {
		if arg == "--patch" || strings.HasPrefix(arg, "--patch=") {
			patchProvided = true
			break
		}
	}

	// Get remaining arguments (files/paths)
	remaining := fs.Args()

	// Handle the case where we have paths without --patch (assume stage mode)
	if !patchProvided && len(remaining) > 0 {
		// Check if first arg is "--" (interactive mode with paths)
		if len(remaining) > 0 && remaining[0] == "--" {
			return "", "", remaining[1:], nil
		}
		// Otherwise assume stage mode with paths
		return "stage", "", remaining, nil
	}

	// Handle --patch flag
	if patchProvided {
		// Special case: if patchFlag is "--", it means --patch was followed by --
		if patchFlag == "--" {
			patchFlag = ""
		}

		// Validate that -- separator is present for certain modes
		if err := validatePatchMode(patchFlag, remaining, args); err != nil {
			return "", "", nil, err
		}

		// Handle different patch modes
		switch patchFlag {
		case "", "stage":
			patchMode = "stage"
		case "stash":
			patchMode = "stash"
		case "reset":
			patchMode, patchRevision = parsePatchReset(remaining)
			remaining = skipRevisionAndSeparator(remaining)
		case "checkout":
			patchMode, patchRevision = parsePatchCheckout(remaining)
			remaining = skipRevisionAndSeparator(remaining)
		case "worktree":
			patchMode, patchRevision = parsePatchWorktree(remaining)
			remaining = skipRevisionAndSeparator(remaining)
		default:
			return "", "", nil, fmt.Errorf("unknown --patch mode: %s", patchFlag)
		}

		// Skip "--" separator if present
		if len(remaining) > 0 && remaining[0] == "--" {
			remaining = remaining[1:]
		}

		return patchMode, patchRevision, remaining, nil
	}

	// No patch mode, remaining args after "--" are files for interactive mode
	if len(remaining) > 0 && remaining[0] == "--" {
		return "", "", remaining[1:], nil
	}

	return "", "", nil, nil
}

// nullWriter discards all writes (used to suppress flag error output)
type nullWriter struct{}

func (nw *nullWriter) Write(p []byte) (n int, err error) {
	return len(p), nil
}

func extractUnknownFlag(errMsg string) string {
	// Extract flag name from error message like "flag provided but not defined: -unknown"
	parts := strings.Split(errMsg, ": ")
	if len(parts) > 1 {
		return parts[1]
	}
	return "unknown"
}

func validatePatchMode(mode string, remaining []string, originalArgs []string) error {
	// Check if -- was present in original args
	hasSeparator := false
	for _, arg := range originalArgs {
		if arg == "--" {
			hasSeparator = true
			break
		}
	}

	switch mode {
	case "":
		// Basic --patch requires --
		if !hasSeparator {
			return fmt.Errorf("expected '--' after --patch")
		}
		// Check for invalid separator case: --patch not-dash-dash
		for i, arg := range originalArgs {
			if arg == "--patch" && i+1 < len(originalArgs) && originalArgs[i+1] != "--" {
				return fmt.Errorf("expected '--' after --patch")
			}
		}
	case "reset":
		// --patch=reset requires --
		if !hasSeparator {
			return fmt.Errorf("expected '--' after --patch=reset")
		}
	case "checkout":
		// --patch=checkout requires --
		if !hasSeparator {
			return fmt.Errorf("expected '--' after --patch=checkout")
		}
	}
	return nil
}

func parsePatchReset(args []string) (mode, revision string) {
	if len(args) == 0 || args[0] == "--" {
		return "reset_head", "HEAD"
	}

	revision = args[0]
	if revision == "HEAD" {
		return "reset_head", revision
	}
	return "reset_nothead", revision
}

func parsePatchCheckout(args []string) (mode, revision string) {
	if len(args) == 0 || args[0] == "--" {
		return "checkout_index", ""
	}

	revision = args[0]
	if revision == "HEAD" {
		return "checkout_head", revision
	}
	return "checkout_nothead", revision
}

func parsePatchWorktree(args []string) (mode, revision string) {
	if len(args) == 0 || args[0] == "--" {
		return "checkout_index", ""
	}

	revision = args[0]
	if revision == "HEAD" {
		return "worktree_head", revision
	}
	return "worktree_nothead", revision
}

func skipRevisionAndSeparator(args []string) []string {
	if len(args) == 0 {
		return args
	}

	// Skip the revision if it's not "--"
	if args[0] != "--" && len(args) > 0 {
		args = args[1:]
	}

	return args
}
