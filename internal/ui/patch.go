package ui

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/cwarden/git-add--interactive/internal/git"
)

var ErrQuit = errors.New("user quit")
var ErrAcceptAll = errors.New("accept all hunks in all files")

var patchPrompts = map[string]map[string]string{
	"stage": {
		"hunk":     "Stage this hunk [y,n,q,a,d%s,?]? ",
		"mode":     "Stage mode change [y,n,q,a,d%s,?]? ",
		"deletion": "Stage deletion [y,n,q,a,d%s,?]? ",
		"addition": "Stage addition [y,n,q,a,d%s,?]? ",
	},
	"reset_head": {
		"hunk":     "Unstage this hunk [y,n,q,a,d%s,?]? ",
		"mode":     "Unstage mode change [y,n,q,a,d%s,?]? ",
		"deletion": "Unstage deletion [y,n,q,a,d%s,?]? ",
		"addition": "Unstage addition [y,n,q,a,d%s,?]? ",
	},
	"checkout_index": {
		"hunk":     "Discard this hunk from worktree [y,n,q,a,d%s,?]? ",
		"mode":     "Discard mode change from worktree [y,n,q,a,d%s,?]? ",
		"deletion": "Discard deletion from worktree [y,n,q,a,d%s,?]? ",
		"addition": "Discard addition from worktree [y,n,q,a,d%s,?]? ",
	},
	"reset_nothead": {
		"hunk":     "Apply this hunk to index [y,n,q,a,d%s,?]? ",
		"mode":     "Apply mode change to index [y,n,q,a,d%s,?]? ",
		"deletion": "Apply deletion to index [y,n,q,a,d%s,?]? ",
		"addition": "Apply addition to index [y,n,q,a,d%s,?]? ",
	},
	"checkout_head": {
		"hunk":     "Discard this hunk from index and worktree [y,n,q,a,d%s,?]? ",
		"mode":     "Discard mode change from index and worktree [y,n,q,a,d%s,?]? ",
		"deletion": "Discard deletion from index and worktree [y,n,q,a,d%s,?]? ",
		"addition": "Discard addition from index and worktree [y,n,q,a,d%s,?]? ",
	},
	"checkout_nothead": {
		"hunk":     "Apply this hunk to index and worktree [y,n,q,a,d%s,?]? ",
		"mode":     "Apply mode change to index and worktree [y,n,q,a,d%s,?]? ",
		"deletion": "Apply deletion to index and worktree [y,n,q,a,d%s,?]? ",
		"addition": "Apply addition to index and worktree [y,n,q,a,d%s,?]? ",
	},
	"worktree_head": {
		"hunk":     "Discard this hunk from worktree [y,n,q,a,d%s,?]? ",
		"mode":     "Discard mode change from worktree [y,n,q,a,d%s,?]? ",
		"deletion": "Discard deletion from worktree [y,n,q,a,d%s,?]? ",
		"addition": "Discard addition from worktree [y,n,q,a,d%s,?]? ",
	},
	"worktree_nothead": {
		"hunk":     "Apply this hunk to worktree [y,n,q,a,d%s,?]? ",
		"mode":     "Apply mode change to worktree [y,n,q,a,d%s,?]? ",
		"deletion": "Apply deletion to worktree [y,n,q,a,d%s,?]? ",
		"addition": "Apply addition to worktree [y,n,q,a,d%s,?]? ",
	},
	"stash": {
		"hunk":     "Stash this hunk [y,n,q,a,d%s,?]? ",
		"mode":     "Stash mode change [y,n,q,a,d%s,?]? ",
		"deletion": "Stash deletion [y,n,q,a,d%s,?]? ",
		"addition": "Stash addition [y,n,q,a,d%s,?]? ",
	},
}

var patchHelp = map[string]string{
	"stage": `y - stage this hunk
n - do not stage this hunk
q - quit; do not stage this hunk or any of the remaining ones
a - stage this hunk and all later hunks in the file
d - do not stage this hunk or any of the later hunks in the file`,
	"reset_head": `y - unstage this hunk
n - do not unstage this hunk
q - quit; do not unstage this hunk or any of the remaining ones
a - unstage this hunk and all later hunks in the file
d - do not unstage this hunk or any of the later hunks in the file`,
	"checkout_index": `y - discard this hunk from worktree
n - do not discard this hunk from worktree
q - quit; do not discard this hunk or any of the remaining ones
a - discard this hunk and all later hunks in the file
d - do not discard this hunk or any of the later hunks in the file`,
	"reset_nothead": `y - apply this hunk to index
n - do not apply this hunk to index
q - quit; do not apply this hunk or any of the remaining ones
a - apply this hunk and all later hunks in the file
d - do not apply this hunk or any of the later hunks in the file`,
	"checkout_head": `y - discard this hunk from index and worktree
n - do not discard this hunk from index and worktree
q - quit; do not discard this hunk or any of the remaining ones
a - discard this hunk and all later hunks in the file
d - do not discard this hunk or any of the later hunks in the file`,
	"checkout_nothead": `y - apply this hunk to index and worktree
n - do not apply this hunk to index and worktree
q - quit; do not apply this hunk or any of the remaining ones
a - apply this hunk and all later hunks in the file
d - do not apply this hunk or any of the later hunks in the file`,
	"worktree_head": `y - discard this hunk from worktree
n - do not discard this hunk from worktree
q - quit; do not discard this hunk or any of the remaining ones
a - discard this hunk and all later hunks in the file
d - do not discard this hunk or any of the later hunks in the file`,
	"worktree_nothead": `y - apply this hunk to worktree
n - do not apply this hunk to worktree
q - quit; do not apply this hunk or any of the remaining ones
a - apply this hunk and all later hunks in the file
d - do not apply this hunk or any of the later hunks in the file`,
	"stash": `y - stash this hunk
n - do not stash this hunk
q - quit; do not stash this hunk or any of the remaining ones
a - stash this hunk and all later hunks in the file
d - do not stash this hunk or any of the later hunks in the file`,
}

func (a *App) patchUpdateFile(path string, mode git.PatchMode, revision string) error {
	hunks, err := a.repo.ParseDiff(path, mode, revision)
	if err != nil {
		return err
	}

	if len(hunks) == 0 {
		return nil
	}

	for _, line := range hunks[0].Display {
		fmt.Println(line)
	}

	actualHunks := hunks[1:]
	if len(actualHunks) == 0 {
		return nil
	}

	// Apply auto-splitting FIRST if enabled (before filtering)
	if a.autoSplitEnabled {
		originalCount := len(actualHunks)
		actualHunks = a.autoSplitAllHunks(actualHunks)
		if len(actualHunks) > originalCount {
			fmt.Printf("Auto-split enabled: expanded %d hunks into %d smaller hunks\n", originalCount, len(actualHunks))
		} else {
			fmt.Printf("Auto-split enabled: %d hunks (no further splitting possible)\n", len(actualHunks))
		}
	}

	// Apply global filter AFTER auto-splitting
	if a.globalFilter != "" {
		filteredHunks := a.filterHunksByRegex(actualHunks, a.globalFilter)
		if len(filteredHunks) == 0 {
			fmt.Printf("No hunks in this file match global filter: %s\n", a.globalFilter)
			return nil
		}
		fmt.Printf("Applied global filter '%s': showing %d of %d hunks\n", a.globalFilter, len(filteredHunks), len(actualHunks))
		actualHunks = filteredHunks
	}

	ix := 0
	for {
		if ix >= len(actualHunks) {
			break
		}

		hunk := &actualHunks[ix]
		if hunk.Use != nil {
			ix++
			continue
		}

		other := a.buildOtherOptions(actualHunks, ix)

		for _, line := range hunk.Display {
			fmt.Println(line)
		}

		promptKey := "hunk"
		if hunk.Type == git.HunkTypeMode {
			promptKey = "mode"
		} else if hunk.Type == git.HunkTypeDeletion {
			promptKey = "deletion"
		} else if hunk.Type == git.HunkTypeAddition {
			promptKey = "addition"
		}

		prompt := fmt.Sprintf(patchPrompts[mode.Name][promptKey], other)
		statusInfo := ""
		if a.globalFilter != "" {
			statusInfo += fmt.Sprintf(" [filter: %s]", a.globalFilter)
		}
		if a.autoSplitEnabled {
			statusInfo += " [auto-split]"
		}
		fmt.Printf("(%d/%d)%s %s", ix+1, len(actualHunks), statusInfo, a.colored(a.colors.PromptColor, prompt))

		input, err := a.promptSingleChar()
		if err != nil {
			return err
		}

		if input == "" {
			continue
		}

		// Check for uppercase commands first
		if len(input) > 0 && input[0] == 'S' {
			// Enable auto-splitting and split all current hunks
			a.autoSplitEnabled = true
			originalCount := len(actualHunks)
			actualHunks = a.autoSplitAllHunks(actualHunks)
			ix = 0 // Reset to beginning since hunk indices changed
			fmt.Printf(a.colored(a.colors.HeaderColor, "Auto-split enabled globally: expanded %d hunks into %d smaller hunks\n"), originalCount, len(actualHunks))
			continue
		}

		if len(input) > 0 && input[0] == 'A' {
			// Accept all hunks in current file and signal to accept all hunks in all remaining files
			for i := 0; i < len(actualHunks); i++ {
				if actualHunks[i].Use == nil {
					use := true
					actualHunks[i].Use = &use
				}
			}

			// Apply current file's changes first
			selectedHunks := []git.Hunk{hunks[0]}
			for _, hunk := range actualHunks {
				if hunk.Use != nil && *hunk.Use {
					selectedHunks = append(selectedHunks, hunk)
				}
			}

			if len(selectedHunks) > 1 {
				patchData := a.reassemblePatch(selectedHunks)
				if err := a.repo.ApplyPatch(patchData, mode); err != nil {
					a.printError(fmt.Sprintf("Failed to apply patch: %v\n", err))
				}
				a.repo.UpdateIndex()
			}

			fmt.Println()
			return ErrAcceptAll
		}

		switch strings.ToLower(input)[0] {
		case 'y':
			use := true
			hunk.Use = &use
			ix++

		case 'n':
			use := false
			hunk.Use = &use
			ix++

		case 'q':
			for i := ix; i < len(actualHunks); i++ {
				if actualHunks[i].Use == nil {
					use := false
					actualHunks[i].Use = &use
				}
			}

			selectedHunks := []git.Hunk{hunks[0]}
			for _, hunk := range actualHunks {
				if hunk.Use != nil && *hunk.Use {
					selectedHunks = append(selectedHunks, hunk)
				}
			}

			if len(selectedHunks) > 1 {
				patchData := a.reassemblePatch(selectedHunks)
				if err := a.repo.ApplyPatch(patchData, mode); err != nil {
					a.printError(fmt.Sprintf("Failed to apply patch: %v\n", err))
				}
				a.repo.UpdateIndex()
			}

			fmt.Println()
			return ErrQuit

		case 'a':
			for i := ix; i < len(actualHunks); i++ {
				if actualHunks[i].Use == nil {
					use := true
					actualHunks[i].Use = &use
				}
			}
			goto applyPatch

		case 'd':
			for i := ix; i < len(actualHunks); i++ {
				if actualHunks[i].Use == nil {
					use := false
					actualHunks[i].Use = &use
				}
			}
			goto applyPatch

		case 's':
			if !a.repo.HunkSplittable(hunk) {
				a.printError("Sorry, cannot split this hunk\n")
				continue
			}

			splits := a.repo.SplitHunk(hunk)
			if len(splits) > 1 {
				fmt.Printf(a.colored(a.colors.HeaderColor, "Split into %d hunks.\n"), len(splits))
				copy(actualHunks[ix:], actualHunks[ix+1:])
				actualHunks = actualHunks[:len(actualHunks)-1]

				for i, split := range splits {
					actualHunks = append(actualHunks[:ix+i], append([]git.Hunk{split}, actualHunks[ix+i:]...)...)
				}
			}

		case 'e':
			newHunk, err := a.editHunk(hunk, mode, hunks[0])
			if err != nil {
				a.printError(fmt.Sprintf("Error editing hunk: %v\n", err))
				continue
			}
			if newHunk != nil {
				actualHunks[ix] = *newHunk
			}

		case 'j':
			ix++
			for ix < len(actualHunks) && actualHunks[ix].Use != nil {
				ix++
			}

		case 'k':
			ix--
			for ix >= 0 && actualHunks[ix].Use != nil {
				ix--
			}
			if ix < 0 {
				ix = 0
			}

		case 'g':
			if strings.ToUpper(input) == "G" || (len(input) > 1 && strings.ToUpper(input)[0:1] == "G") {
				// G <regex> command for global filtering
				regexStr := strings.TrimSpace(input[1:])
				if regexStr == "" {
					fmt.Print("search for which pattern (empty to clear global filter)? ")
					regexInput, err := a.promptSingleChar()
					if err != nil {
						continue
					}
					regexStr = strings.TrimSpace(regexInput)
				}

				if regexStr == "" {
					// Clear global filter
					a.globalFilter = ""
					fmt.Println("Global filter cleared")
					// Reparse the current file without filter
					hunks, err := a.repo.ParseDiff(path, mode, revision)
					if err != nil {
						a.printError(fmt.Sprintf("Error reparsing hunks: %v\n", err))
						continue
					}
					actualHunks = hunks[1:]
					ix = 0
					continue
				}

				// Set global filter
				a.globalFilter = regexStr

				// Filter current hunks and replace current hunks list
				filteredHunks := a.filterHunksByRegex(actualHunks, regexStr)
				if len(filteredHunks) == 0 {
					a.printError(fmt.Sprintf("No hunks in current file match pattern: %s\n", regexStr))
					continue
				}

				fmt.Printf("Global filter set to '%s': showing %d hunks in current file\n", regexStr, len(filteredHunks))
				actualHunks = filteredHunks
				ix = 0
				continue
			} else {
				// Original 'g' command for goto hunk number
				fmt.Print("go to which hunk? ")
				gotoInput, err := a.promptSingleChar()
				if err != nil {
					continue
				}
				if gotoNum, err := strconv.Atoi(gotoInput); err == nil {
					if gotoNum >= 1 && gotoNum <= len(actualHunks) {
						ix = gotoNum - 1
					} else {
						a.printError(fmt.Sprintf("Sorry, only %d hunks available.\n", len(actualHunks)))
					}
				} else {
					a.printError(fmt.Sprintf("Invalid number: '%s'\n", gotoInput))
				}
			}

		case '/':
			// Search within current file hunks (same as G but doesn't set global filter)
			fmt.Print("search for which pattern? ")
			regexInput, err := a.promptSingleChar()
			if err != nil {
				continue
			}
			regexStr := strings.TrimSpace(regexInput)
			if regexStr == "" {
				continue
			}

			// Find first matching hunk starting from current position
			found := false
			for i := ix + 1; i < len(actualHunks); i++ {
				if a.hunkMatchesRegex(&actualHunks[i], regexStr) {
					ix = i
					found = true
					break
				}
			}
			if !found {
				// Search from beginning
				for i := 0; i <= ix; i++ {
					if a.hunkMatchesRegex(&actualHunks[i], regexStr) {
						ix = i
						found = true
						break
					}
				}
			}
			if !found {
				a.printError(fmt.Sprintf("Pattern not found: %s\n", regexStr))
			}

		case '?':
			help := patchHelp[mode.Name]
			if help == "" {
				help = patchHelp["stage"]
			}
			help += `
/ - search for a pattern in current file
g - select a hunk to go to
G - set global filter for all files (empty pattern clears filter)
A - accept all hunks (after auto-splitting and filtering)
j - leave this hunk undecided, see next undecided hunk
k - leave this hunk undecided, see previous undecided hunk
s - split the current hunk into smaller hunks
S - enable auto-splitting globally and split all hunks
e - manually edit the current hunk
? - print help`
			fmt.Print(a.colored(a.colors.HelpColor, help+"\n"))

		default:
			help := patchHelp[mode.Name]
			if help == "" {
				help = patchHelp["stage"]
			}
			fmt.Print(a.colored(a.colors.HelpColor, help+"\n"))
		}
	}

applyPatch:
	selectedHunks := []git.Hunk{hunks[0]}
	for _, hunk := range actualHunks {
		if hunk.Use != nil && *hunk.Use {
			selectedHunks = append(selectedHunks, hunk)
		}
	}

	if len(selectedHunks) > 1 {
		patchData := a.reassemblePatch(selectedHunks)
		if err := a.repo.ApplyPatch(patchData, mode); err != nil {
			a.printError(fmt.Sprintf("Failed to apply patch: %v\n", err))
		}
		a.repo.UpdateIndex()
	}

	fmt.Println()
	return nil
}

func (a *App) buildOtherOptions(hunks []git.Hunk, currentIx int) string {
	var options []string

	hasPrev := false
	hasNext := false

	for i := 0; i < currentIx; i++ {
		if hunks[i].Use == nil {
			hasPrev = true
			break
		}
	}

	for i := currentIx + 1; i < len(hunks); i++ {
		if hunks[i].Use == nil {
			hasNext = true
			break
		}
	}

	if currentIx > 0 {
		options = append(options, "K")
	}
	if currentIx < len(hunks)-1 {
		options = append(options, "J")
	}
	if hasPrev {
		options = append(options, "k")
	}
	if hasNext {
		options = append(options, "j")
	}
	if len(hunks) > 1 {
		options = append(options, "g")
	}

	// Always show G for global filter and A for accept all
	options = append(options, "G")
	options = append(options, "A")

	hunk := &hunks[currentIx]
	if a.repo.HunkSplittable(hunk) {
		options = append(options, "s")
	}
	// Always show S for auto-splitting all hunks
	options = append(options, "S")
	if hunk.Type == git.HunkTypeHunk {
		options = append(options, "e")
	}

	if len(options) > 0 {
		return "," + strings.Join(options, ",")
	}
	return ""
}

func (a *App) editHunk(hunk *git.Hunk, mode git.PatchMode, header git.Hunk) (*git.Hunk, error) {
	hunkFile := filepath.Join(a.repo.GitDir(), "addp-hunk-edit.diff")

	content := "# Manual hunk edit mode -- see bottom for a quick guide.\n"
	for _, line := range hunk.Text {
		content += line + "\n"
	}

	content += "# ---\n"
	content += "# To remove '-' lines, make them ' ' lines (context).\n"
	content += "# To remove '+' lines, delete them.\n"
	content += "# Lines starting with # will be removed.\n"

	if err := ioutil.WriteFile(hunkFile, []byte(content), 0644); err != nil {
		return nil, err
	}

	defer os.Remove(hunkFile)

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editorOutput, err := a.repo.RunCommand("var", "GIT_EDITOR")
		if err != nil {
			editor = "vi"
		} else {
			editor = strings.TrimSpace(string(editorOutput))
		}
	}

	cmd := exec.Command("sh", "-c", editor+" "+hunkFile)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return nil, err
	}

	editedContent, err := ioutil.ReadFile(hunkFile)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(editedContent), "\n")
	var newText []string
	var newDisplay []string

	for _, line := range lines {
		if !strings.HasPrefix(line, "#") && strings.TrimSpace(line) != "" {
			newText = append(newText, line)
			newDisplay = append(newDisplay, line)
		}
	}

	if len(newText) == 0 {
		return nil, nil
	}

	if !strings.HasPrefix(newText[0], "@@") {
		newText = append([]string{hunk.Text[0]}, newText...)
		newDisplay = append([]string{hunk.Display[0]}, newDisplay...)
	}

	newHunk := &git.Hunk{
		Text:    newText,
		Display: newDisplay,
		Type:    hunk.Type,
		Dirty:   true,
	}

	use := true
	newHunk.Use = &use

	patchData := a.reassemblePatch([]git.Hunk{header, *newHunk})
	if err := a.repo.CheckPatch(patchData, mode); err != nil {
		retry, err := a.promptYesNo("Your edited hunk does not apply. Edit again (saying \"no\" discards!) [y/n]? ")
		if err != nil || !retry {
			return nil, nil
		}
		return a.editHunk(hunk, mode, header)
	}

	return newHunk, nil
}

func (a *App) autoSplitAllHunks(hunks []git.Hunk) []git.Hunk {
	var result []git.Hunk

	for _, hunk := range hunks {
		if a.repo.HunkSplittable(&hunk) {
			// Keep splitting until no more splits are possible
			currentSplits := []git.Hunk{hunk}
			totalSplitRounds := 0

			for {
				var newSplits []git.Hunk
				splitOccurred := false

				for _, splitHunk := range currentSplits {
					if a.repo.HunkSplittable(&splitHunk) {
						splits := a.repo.SplitHunk(&splitHunk)
						if len(splits) > 1 {
							splitOccurred = true
							newSplits = append(newSplits, splits...)
						} else {
							newSplits = append(newSplits, splitHunk)
						}
					} else {
						newSplits = append(newSplits, splitHunk)
					}
				}

				currentSplits = newSplits
				totalSplitRounds++
				if !splitOccurred || totalSplitRounds > 10 { // Safety limit
					break
				}
			}

			result = append(result, currentSplits...)
		} else {
			result = append(result, hunk)
		}
	}

	return result
}

func (a *App) hunkMatchesRegex(hunk *git.Hunk, regexStr string) bool {
	regex, err := regexp.Compile(regexStr)
	if err != nil {
		return false
	}

	for _, line := range hunk.Text {
		if regex.MatchString(line) {
			return true
		}
	}
	return false
}

func (a *App) filterHunksByRegex(hunks []git.Hunk, regexStr string) []git.Hunk {
	// Validate regex first
	_, err := regexp.Compile(regexStr)
	if err != nil {
		a.printError(fmt.Sprintf("Invalid regex pattern: %v\n", err))
		return nil
	}

	var filteredHunks []git.Hunk

	for _, hunk := range hunks {
		if a.hunkMatchesRegex(&hunk, regexStr) {
			filteredHunks = append(filteredHunks, hunk)
		}
	}

	return filteredHunks
}

func (a *App) reassemblePatch(hunks []git.Hunk) []byte {
	var lines []string

	if len(hunks) > 0 {
		for _, line := range hunks[0].Text {
			if !strings.HasPrefix(line, "+++") && !strings.HasPrefix(line, "---") {
				lines = append(lines, line)
			}
		}

		headerAdded := false
		for _, hunk := range hunks[1:] {
			if hunk.Type == git.HunkTypeHunk && !headerAdded {
				for _, line := range hunks[0].Text {
					if strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---") {
						lines = append(lines, line)
					}
				}
				headerAdded = true
			}
			for _, line := range hunk.Text {
				lines = append(lines, line)
			}
		}
	}

	return []byte(strings.Join(lines, "\n") + "\n")
}
