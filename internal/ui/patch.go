package ui

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/cwarden/git-add--interactive/internal/git"
)

var ErrQuit = errors.New("user quit")

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
		fmt.Printf("(%d/%d) %s", ix+1, len(actualHunks), a.colored(a.colors.PromptColor, prompt))

		input, err := a.promptSingleChar()
		if err != nil {
			return err
		}

		if input == "" {
			continue
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
			newHunk, err := a.editHunk(hunk, mode)
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

		case '?':
			help := patchHelp[mode.Name]
			if help == "" {
				help = patchHelp["stage"]
			}
			help += `
g - select a hunk to go to
j - leave this hunk undecided, see next undecided hunk
k - leave this hunk undecided, see previous undecided hunk
s - split the current hunk into smaller hunks
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

	hunk := &hunks[currentIx]
	if a.repo.HunkSplittable(hunk) {
		options = append(options, "s")
	}
	if hunk.Type == git.HunkTypeHunk {
		options = append(options, "e")
	}

	if len(options) > 0 {
		return "," + strings.Join(options, ",")
	}
	return ""
}

func (a *App) editHunk(hunk *git.Hunk, mode git.PatchMode) (*git.Hunk, error) {
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

	patchData := a.reassemblePatch([]git.Hunk{*newHunk})
	if err := a.repo.CheckPatch(patchData, mode); err != nil {
		retry, err := a.promptYesNo("Your edited hunk does not apply. Edit again (saying \"no\" discards!) [y/n]? ")
		if err != nil || !retry {
			return nil, nil
		}
		return a.editHunk(hunk, mode)
	}

	return newHunk, nil
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
