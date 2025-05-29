package git

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type PatchMode struct {
	Name      string
	DiffCmd   []string
	ApplyCmd  []string
	CheckCmd  []string
	Filter    string
	IsReverse bool
}

var PatchModes = map[string]PatchMode{
	"stage": {
		Name:      "stage",
		DiffCmd:   []string{"diff-files", "-p"},
		ApplyCmd:  []string{"apply", "--cached"},
		CheckCmd:  []string{"apply", "--cached", "--check"},
		Filter:    "file-only",
		IsReverse: false,
	},
	"stash": {
		Name:      "stash",
		DiffCmd:   []string{"diff-index", "-p", "HEAD"},
		ApplyCmd:  []string{"apply", "--cached"},
		CheckCmd:  []string{"apply", "--cached", "--check"},
		Filter:    "",
		IsReverse: false,
	},
	"reset_head": {
		Name:      "reset_head",
		DiffCmd:   []string{"diff-index", "-p", "--cached"},
		ApplyCmd:  []string{"apply", "-R", "--cached"},
		CheckCmd:  []string{"apply", "-R", "--cached", "--check"},
		Filter:    "index-only",
		IsReverse: true,
	},
	"reset_nothead": {
		Name:      "reset_nothead",
		DiffCmd:   []string{"diff-index", "-R", "-p", "--cached"},
		ApplyCmd:  []string{"apply", "--cached"},
		CheckCmd:  []string{"apply", "--cached", "--check"},
		Filter:    "index-only",
		IsReverse: false,
	},
	"checkout_index": {
		Name:      "checkout_index",
		DiffCmd:   []string{"diff-files", "-p"},
		ApplyCmd:  []string{"apply", "-R"},
		CheckCmd:  []string{"apply", "-R", "--check"},
		Filter:    "file-only",
		IsReverse: true,
	},
	"checkout_head": {
		Name:      "checkout_head",
		DiffCmd:   []string{"diff-index", "-p"},
		ApplyCmd:  []string{"apply", "-R"},
		CheckCmd:  []string{"apply", "-R", "--check"},
		Filter:    "",
		IsReverse: true,
	},
	"checkout_nothead": {
		Name:      "checkout_nothead",
		DiffCmd:   []string{"diff-index", "-R", "-p"},
		ApplyCmd:  []string{"apply"},
		CheckCmd:  []string{"apply", "--check"},
		Filter:    "",
		IsReverse: false,
	},
	"worktree_head": {
		Name:      "worktree_head",
		DiffCmd:   []string{"diff-index", "-p"},
		ApplyCmd:  []string{"apply", "-R"},
		CheckCmd:  []string{"apply", "-R", "--check"},
		Filter:    "",
		IsReverse: true,
	},
	"worktree_nothead": {
		Name:      "worktree_nothead",
		DiffCmd:   []string{"diff-index", "-R", "-p"},
		ApplyCmd:  []string{"apply"},
		CheckCmd:  []string{"apply", "--check"},
		Filter:    "",
		IsReverse: false,
	},
}

type HunkType string

const (
	HunkTypeHeader   HunkType = "header"
	HunkTypeHunk     HunkType = "hunk"
	HunkTypeMode     HunkType = "mode"
	HunkTypeDeletion HunkType = "deletion"
	HunkTypeAddition HunkType = "addition"
)

type Hunk struct {
	Text     []string
	Display  []string
	Type     HunkType
	Use      *bool
	Dirty    bool
	OldLine  int
	NewLine  int
	OldCnt   int
	NewCnt   int
	OfsDelta int
}

func (r *Repository) ParseDiff(path string, mode PatchMode, revision string) ([]Hunk, error) {
	var diffCmd []string
	diffCmd = append(diffCmd, mode.DiffCmd...)

	if diffAlgo, err := r.GetConfig("diff.algorithm"); err == nil && diffAlgo != "" {
		diffCmd = append([]string{diffCmd[0], "--diff-algorithm=" + diffAlgo}, diffCmd[1:]...)
	}

	if revision != "" {
		reference := revision
		if r.IsInitialCommit() && revision == "HEAD" {
			emptyTree, err := r.GetEmptyTree()
			if err != nil {
				return nil, err
			}
			reference = emptyTree
		}
		diffCmd = append(diffCmd, reference)
	}

	diffCmd = append(diffCmd, "--no-color", "--", path)

	diffLines, err := r.RunCommandLines(diffCmd...)
	if err != nil {
		return nil, err
	}

	var coloredLines []string
	if r.GetColorBool("color.diff") {
		colorCmd := append([]string{}, mode.DiffCmd...)
		colorCmd = append(colorCmd, "--color", "--", path)
		if revision != "" {
			reference := revision
			if r.IsInitialCommit() && revision == "HEAD" {
				emptyTree, err := r.GetEmptyTree()
				if err != nil {
					return nil, err
				}
				reference = emptyTree
			}
			colorCmd = append(colorCmd[:len(colorCmd)-2], reference, "--", path)
		}
		coloredLines, _ = r.RunCommandLines(colorCmd...)
	}

	if len(coloredLines) == 0 {
		coloredLines = diffLines
	}

	return r.parseHunks(diffLines, coloredLines)
}

func (r *Repository) parseHunks(diffLines, coloredLines []string) ([]Hunk, error) {
	var hunks []Hunk
	currentHunk := Hunk{
		Type:    HunkTypeHeader,
		Text:    []string{},
		Display: []string{},
	}

	for i, line := range diffLines {
		displayLine := line
		if i < len(coloredLines) {
			displayLine = coloredLines[i]
		}

		if strings.HasPrefix(line, "@@ ") {
			if len(currentHunk.Text) > 0 {
				hunks = append(hunks, currentHunk)
			}
			currentHunk = Hunk{
				Type:    HunkTypeHunk,
				Text:    []string{},
				Display: []string{},
			}
		}

		currentHunk.Text = append(currentHunk.Text, line)
		currentHunk.Display = append(currentHunk.Display, displayLine)
	}

	if len(currentHunk.Text) > 0 {
		hunks = append(hunks, currentHunk)
	}

	for i := range hunks {
		if hunks[i].Type == HunkTypeHunk && len(hunks[i].Text) > 0 {
			if err := r.parseHunkHeader(&hunks[i]); err != nil {
				return nil, err
			}
		}
	}

	return hunks, nil
}

func (r *Repository) parseHunkHeader(hunk *Hunk) error {
	if len(hunk.Text) == 0 {
		return fmt.Errorf("empty hunk")
	}

	hunkHeaderRe := regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)
	matches := hunkHeaderRe.FindStringSubmatch(hunk.Text[0])
	if len(matches) < 4 {
		return fmt.Errorf("invalid hunk header: %s", hunk.Text[0])
	}

	oldLine, _ := strconv.Atoi(matches[1])
	oldCnt := 1
	if matches[2] != "" {
		oldCnt, _ = strconv.Atoi(matches[2])
	}

	newLine, _ := strconv.Atoi(matches[3])
	newCnt := 1
	if matches[4] != "" {
		newCnt, _ = strconv.Atoi(matches[4])
	}

	hunk.OldLine = oldLine
	hunk.OldCnt = oldCnt
	hunk.NewLine = newLine
	hunk.NewCnt = newCnt

	return nil
}

func (r *Repository) ApplyPatch(patch []byte, mode PatchMode) error {
	cmd := append(mode.ApplyCmd, "--allow-overlap")
	return r.RunCommandWithStdin(patch, cmd...)
}

func (r *Repository) CheckPatch(patch []byte, mode PatchMode) error {
	cmd := append(mode.CheckCmd, "--allow-overlap")
	return r.RunCommandWithStdin(patch, cmd...)
}

func (r *Repository) HunkSplittable(hunk *Hunk) bool {
	if hunk.Type != HunkTypeHunk {
		return false
	}

	contextLines := 0
	inChanges := false

	for i := 1; i < len(hunk.Text); i++ {
		line := hunk.Text[i]
		if strings.HasPrefix(line, " ") {
			if inChanges {
				contextLines++
			}
		} else if strings.HasPrefix(line, "+") || strings.HasPrefix(line, "-") {
			if contextLines > 0 {
				return true
			}
			inChanges = true
			contextLines = 0
		}
	}

	return false
}

func (r *Repository) SplitHunk(hunk *Hunk) []Hunk {
	if !r.HunkSplittable(hunk) {
		return []Hunk{*hunk}
	}

	var splits []Hunk
	currentSplit := Hunk{
		Type:    HunkTypeHunk,
		Text:    []string{hunk.Text[0]},
		Display: []string{hunk.Display[0]},
		OldLine: hunk.OldLine,
		NewLine: hunk.NewLine,
	}

	oldOffset := hunk.OldLine
	newOffset := hunk.NewLine
	oldCnt := 0
	newCnt := 0
	inChanges := false
	contextLines := 0

	for i := 1; i < len(hunk.Text); i++ {
		line := hunk.Text[i]
		displayLine := hunk.Display[i]

		if strings.HasPrefix(line, " ") {
			if inChanges && contextLines >= 3 {
				currentSplit.OldCnt = oldCnt
				currentSplit.NewCnt = newCnt
				r.updateHunkHeader(&currentSplit)
				splits = append(splits, currentSplit)

				oldOffset += oldCnt
				newOffset += newCnt
				currentSplit = Hunk{
					Type:    HunkTypeHunk,
					Text:    []string{},
					Display: []string{},
					OldLine: oldOffset,
					NewLine: newOffset,
				}
				oldCnt = 0
				newCnt = 0
				inChanges = false
				contextLines = 0
			}

			if inChanges {
				contextLines++
			}

			currentSplit.Text = append(currentSplit.Text, line)
			currentSplit.Display = append(currentSplit.Display, displayLine)
			oldCnt++
			newCnt++
		} else {
			inChanges = true
			contextLines = 0

			currentSplit.Text = append(currentSplit.Text, line)
			currentSplit.Display = append(currentSplit.Display, displayLine)

			if strings.HasPrefix(line, "-") {
				oldCnt++
			} else if strings.HasPrefix(line, "+") {
				newCnt++
			}
		}
	}

	currentSplit.OldCnt = oldCnt
	currentSplit.NewCnt = newCnt
	r.updateHunkHeader(&currentSplit)
	splits = append(splits, currentSplit)

	return splits
}

func (r *Repository) updateHunkHeader(hunk *Hunk) {
	header := fmt.Sprintf("@@ -%d", hunk.OldLine)
	if hunk.OldCnt != 1 {
		header += fmt.Sprintf(",%d", hunk.OldCnt)
	}
	header += fmt.Sprintf(" +%d", hunk.NewLine)
	if hunk.NewCnt != 1 {
		header += fmt.Sprintf(",%d", hunk.NewCnt)
	}
	header += " @@"

	if len(hunk.Text) > 0 {
		hunk.Text[0] = header
		hunk.Display[0] = header
	} else {
		hunk.Text = []string{header}
		hunk.Display = []string{header}
	}
}
