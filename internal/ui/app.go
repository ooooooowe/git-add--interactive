package ui

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/cwarden/git-add--interactive/internal/git"
)

type App struct {
	repo             *git.Repository
	colors           ColorConfig
	globalFilter     string // Global regex filter for all files
	autoSplitEnabled bool   // Global flag to automatically split hunks to smallest possible
}

type ColorConfig struct {
	UseColor      bool
	PromptColor   string
	HeaderColor   string
	HelpColor     string
	ErrorColor    string
	NormalColor   string
	FragInfoColor string
	DiffOldColor  string
	DiffNewColor  string
	DiffCtxColor  string
}

func NewApp(repo *git.Repository) *App {
	app := &App{
		repo: repo,
	}
	app.initColors()
	return app
}

func (a *App) showInteractiveStatus() {
	files, err := a.repo.ListModified("")
	if err != nil {
		return // Silently skip status on error
	}

	if len(files) == 0 {
		return // No files to show
	}

	fmt.Printf("           %s     %s %s\n", "staged", "unstaged", "path")
	for i, file := range files {
		stagePart := file.Index
		if stagePart == "" {
			stagePart = "unchanged"
		}
		unstagePart := file.File
		if unstagePart == "" {
			unstagePart = "nothing"
		}
		fmt.Printf("  %d:    %-12s %s %s\n",
			i+1, stagePart, unstagePart, file.Path)
	}
}

func (a *App) initColors() {
	a.colors.UseColor = a.repo.GetColorBool("color.interactive")

	if a.colors.UseColor {
		a.colors.PromptColor = a.repo.GetColor("color.interactive.prompt", "bold blue")
		a.colors.HeaderColor = a.repo.GetColor("color.interactive.header", "bold")
		a.colors.HelpColor = a.repo.GetColor("color.interactive.help", "red bold")
		a.colors.ErrorColor = a.repo.GetColor("color.interactive.error", "red bold")
		a.colors.NormalColor = a.repo.GetColor("", "reset")
	}

	if a.repo.GetColorBool("color.diff") {
		a.colors.FragInfoColor = a.repo.GetColor("color.diff.frag", "cyan")
		a.colors.DiffOldColor = a.repo.GetColor("color.diff.old", "red")
		a.colors.DiffNewColor = a.repo.GetColor("color.diff.new", "green")
		a.colors.DiffCtxColor = a.repo.GetColor("color.diff.context", "")
	}
}

func (a *App) RunInteractive() error {
	commands := []Command{
		{"status", "show paths with changes", a.statusCmd},
		{"update", "add working tree state to the staged set of changes", a.updateCmd},
		{"revert", "revert staged set of changes back to the HEAD version", a.revertCmd},
		{"add untracked", "add contents of untracked files to the staged set of changes", a.addUntrackedCmd},
		{"patch", "pick hunks and update selectively", a.patchCmd},
		{"diff", "view diff between HEAD and index", a.diffCmd},
		{"quit", "quit", a.quitCmd},
		{"help", "show help", a.helpCmd},
	}

	for {
		// Show status first like Perl version
		a.showInteractiveStatus()

		// Show commands in compact format like Perl version
		fmt.Print(a.colored(a.colors.HeaderColor, "\n*** Commands ***\n"))

		// Display commands in compact horizontal format like Perl version
		cmdLine1 := fmt.Sprintf("  1: %s       2: %s       3: %s       4: %s",
			a.colored(a.colors.PromptColor, "s")+"tatus",
			a.colored(a.colors.PromptColor, "u")+"pdate",
			a.colored(a.colors.PromptColor, "r")+"evert",
			a.colored(a.colors.PromptColor, "a")+"dd untracked")
		cmdLine2 := fmt.Sprintf("  5: %s        6: %s         7: %s         8: %s",
			a.colored(a.colors.PromptColor, "p")+"atch",
			a.colored(a.colors.PromptColor, "d")+"iff",
			a.colored(a.colors.PromptColor, "q")+"uit",
			a.colored(a.colors.PromptColor, "h")+"elp")

		fmt.Println(cmdLine1)
		fmt.Println(cmdLine2)

		// Interactive prompt
		fmt.Print(a.colored(a.colors.PromptColor, "What now> "))
		input, err := a.promptSingleChar()
		if err != nil {
			return err
		}

		if input == "" {
			break
		}

		// Handle single letter commands
		var selectedCmd *Command
		for _, cmd := range commands {
			if strings.ToLower(input) == strings.ToLower(cmd.Name[:1]) {
				selectedCmd = &cmd
				break
			}
		}

		// Handle numeric commands
		if selectedCmd == nil {
			if num, err := strconv.Atoi(input); err == nil {
				if num >= 1 && num <= len(commands) {
					selectedCmd = &commands[num-1]
				}
			}
		}

		if selectedCmd == nil {
			a.printError(fmt.Sprintf("Invalid input: %s\n", input))
			continue
		}

		if err := selectedCmd.Action(); err != nil {
			a.printError(fmt.Sprintf("Error: %v\n", err))
		}
	}

	return nil
}

func (a *App) RunPatchMode(mode, revision string, paths []string) error {
	patchMode, exists := git.PatchModes[mode]
	if !exists {
		return fmt.Errorf("unknown patch mode: %s", mode)
	}

	files, err := a.repo.ListModifiedWithRevisionAndPaths(patchMode.Filter, revision, paths)
	if err != nil {
		return err
	}

	var filteredFiles []git.FileStatus
	for _, file := range files {
		if !file.Unmerged && !file.Binary {
			filteredFiles = append(filteredFiles, file)
		}
	}

	if len(filteredFiles) == 0 {
		fmt.Println("No changes.")
		return nil
	}

	for i, file := range filteredFiles {
		if err := a.patchUpdateFile(file.Path, patchMode, revision); err != nil {
			if errors.Is(err, ErrQuit) {
				break
			}
			if errors.Is(err, ErrAcceptAll) {
				// Accept all hunks in all remaining files
				for j := i + 1; j < len(filteredFiles); j++ {
					remainingFile := filteredFiles[j]
					if err := a.acceptAllHunksInFile(remainingFile.Path, patchMode, revision); err != nil {
						return err
					}
				}
				break
			}
			return err
		}
	}

	return nil
}

func (a *App) containsPath(paths []string, target string) bool {
	for _, path := range paths {
		if a.matchesPathspec(path, target) {
			return true
		}
	}
	return false
}

func (a *App) matchesPathspec(pathspec, target string) bool {
	// Handle exact match first
	if pathspec == target {
		return true
	}

	// Parse pathspec magic signatures like :(,prefix:0)internal/
	actualPath := a.parsePathspec(pathspec)

	// Handle exact match with parsed path
	if actualPath == target {
		return true
	}

	// Handle directory prefix matching
	if strings.HasSuffix(actualPath, "/") {
		if strings.HasPrefix(target, actualPath) {
			return true
		}
	}

	// Handle relative path matching by normalizing both paths
	normalizedPath := a.normalizePath(actualPath)
	normalizedTarget := a.normalizePath(target)

	// Exact match after normalization
	if normalizedPath == normalizedTarget {
		return true
	}

	// Directory prefix matching after normalization
	if strings.HasSuffix(normalizedPath, "/") {
		if strings.HasPrefix(normalizedTarget, normalizedPath) {
			return true
		}
	}

	return false
}

func (a *App) parsePathspec(pathspec string) string {
	// Handle Git pathspec magic signatures like :(magic)path
	if strings.HasPrefix(pathspec, ":(") {
		// Find the closing parenthesis
		closeParen := strings.Index(pathspec, ")")
		if closeParen != -1 {
			// Extract the path part after the magic signature
			return pathspec[closeParen+1:]
		}
	}

	return pathspec
}

func (a *App) normalizePath(path string) string {
	// Remove leading "./" if present
	if strings.HasPrefix(path, "./") {
		path = path[2:]
	}

	// Remove trailing slashes
	path = strings.TrimSuffix(path, "/")

	return path
}

func (a *App) acceptAllHunksInFile(path string, mode git.PatchMode, revision string) error {
	hunks, err := a.repo.ParseDiff(path, mode, revision)
	if err != nil {
		return err
	}

	if len(hunks) == 0 {
		return nil
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
			fmt.Printf("Auto-split enabled: expanded %d hunks into %d smaller hunks in %s\n", originalCount, len(actualHunks), path)
		}
	}

	// Apply global filter AFTER auto-splitting
	if a.globalFilter != "" {
		filteredHunks := a.filterHunksByRegex(actualHunks, a.globalFilter)
		if len(filteredHunks) == 0 {
			fmt.Printf("No hunks in %s match global filter: %s\n", path, a.globalFilter)
			return nil
		}
		fmt.Printf("Applied global filter '%s' to %s: accepting %d of %d hunks\n", a.globalFilter, path, len(filteredHunks), len(actualHunks))
		actualHunks = filteredHunks
	}

	// Accept all hunks
	for i := 0; i < len(actualHunks); i++ {
		use := true
		actualHunks[i].Use = &use
	}

	// Apply the patch
	selectedHunks := []git.Hunk{hunks[0]}
	for _, hunk := range actualHunks {
		if hunk.Use != nil && *hunk.Use {
			selectedHunks = append(selectedHunks, hunk)
		}
	}

	if len(selectedHunks) > 1 {
		patchData := a.reassemblePatch(selectedHunks)
		if err := a.repo.ApplyPatch(patchData, mode); err != nil {
			return fmt.Errorf("failed to apply patch for %s: %v", path, err)
		}
		a.repo.UpdateIndex()
		fmt.Printf("Accepted all hunks in %s\n", path)
	}

	return nil
}

func (a *App) colored(color, text string) string {
	if !a.colors.UseColor || color == "" {
		return text
	}
	return color + text + a.colors.NormalColor
}

func (a *App) printError(text string) {
	fmt.Fprint(os.Stderr, a.colored(a.colors.ErrorColor, text))
}

func (a *App) promptSingleChar() (string, error) {
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(input), nil
}

func (a *App) promptYesNo(prompt string) (bool, error) {
	for {
		fmt.Print(a.colored(a.colors.PromptColor, prompt))
		input, err := a.promptSingleChar()
		if err != nil {
			return false, err
		}

		if strings.ToLower(input) == "y" || strings.ToLower(input) == "yes" {
			return true, nil
		}
		if strings.ToLower(input) == "n" || strings.ToLower(input) == "no" {
			return false, nil
		}
	}
}

type Command struct {
	Name        string
	Description string
	Action      func() error
}

func (c Command) String() string {
	return c.Name
}

func (a *App) statusCmd() error {
	files, err := a.repo.ListModified("")
	if err != nil {
		return err
	}

	fmt.Printf(a.colored(a.colors.HeaderColor, "%12s %12s %s\n"), "staged", "unstaged", "path")

	for _, file := range files {
		fmt.Printf("%12s %12s %s\n", file.Index, file.File, file.Path)
	}

	fmt.Println()
	return nil
}

func (a *App) updateCmd() error {
	files, err := a.repo.ListModified("file-only")
	if err != nil {
		return err
	}

	if len(files) == 0 {
		return nil
	}

	var fileItems []interface{}
	for _, file := range files {
		fileItems = append(fileItems, file)
	}

	fmt.Printf(a.colored(a.colors.HeaderColor, "%12s %12s %s\n"), "staged", "unstaged", "path")
	chosen, err := a.listAndChoose("Update", fileItems, false, false)
	if err != nil {
		return err
	}

	if len(chosen) > 0 {
		var paths []string
		for _, item := range chosen {
			file := item.(git.FileStatus)
			paths = append(paths, file.Path)
		}

		args := append([]string{"update-index", "--add", "--remove", "--"}, paths...)
		_, err := a.repo.RunCommand(args...)
		if err != nil {
			return err
		}

		fmt.Printf("updated %d path(s)\n", len(paths))
	}

	fmt.Println()
	return nil
}

func (a *App) revertCmd() error {
	files, err := a.repo.ListModified("")
	if err != nil {
		return err
	}

	if len(files) == 0 {
		return nil
	}

	var fileItems []interface{}
	for _, file := range files {
		fileItems = append(fileItems, file)
	}

	fmt.Printf(a.colored(a.colors.HeaderColor, "%12s %12s %s\n"), "staged", "unstaged", "path")
	chosen, err := a.listAndChoose("Revert", fileItems, false, false)
	if err != nil {
		return err
	}

	if len(chosen) > 0 {
		var paths []string
		for _, item := range chosen {
			file := item.(git.FileStatus)
			paths = append(paths, file.Path)
		}

		if a.repo.IsInitialCommit() {
			args := append([]string{"rm", "--cached"}, paths...)
			_, err := a.repo.RunCommand(args...)
			if err != nil {
				return err
			}
		} else {
			args := append([]string{"ls-tree", "HEAD", "--"}, paths...)
			lines, err := a.repo.RunCommandLines(args...)
			if err != nil {
				return err
			}

			updateCmd := []string{"update-index", "--index-info"}
			patchData := strings.Join(lines, "\n")
			if err := a.repo.RunCommandWithStdin([]byte(patchData), updateCmd...); err != nil {
				return err
			}
		}

		a.repo.UpdateIndex()
		fmt.Printf("reverted %d path(s)\n", len(paths))
	}

	fmt.Println()
	return nil
}

func (a *App) addUntrackedCmd() error {
	untracked, err := a.repo.ListUntracked()
	if err != nil {
		return err
	}

	if len(untracked) == 0 {
		fmt.Println("No untracked files.")
		fmt.Println()
		return nil
	}

	var items []interface{}
	for _, file := range untracked {
		items = append(items, file)
	}

	chosen, err := a.listAndChoose("Add untracked", items, false, false)
	if err != nil {
		return err
	}

	if len(chosen) > 0 {
		var paths []string
		for _, item := range chosen {
			paths = append(paths, item.(string))
		}

		args := append([]string{"update-index", "--add", "--"}, paths...)
		_, err := a.repo.RunCommand(args...)
		if err != nil {
			return err
		}

		fmt.Printf("added %d path(s)\n", len(paths))
	}

	fmt.Println()
	return nil
}

func (a *App) patchCmd() error {
	files, err := a.repo.ListModified("file-only")
	if err != nil {
		return err
	}

	if len(files) == 0 {
		fmt.Println("No changes.")
		fmt.Println()
		return nil
	}

	var fileItems []interface{}
	for _, file := range files {
		if !file.Unmerged && !file.Binary {
			fileItems = append(fileItems, file)
		}
	}

	if len(fileItems) == 0 {
		fmt.Println("No changes.")
		fmt.Println()
		return nil
	}

	fmt.Printf(a.colored(a.colors.HeaderColor, "%12s %12s %s\n"), "staged", "unstaged", "path")
	chosen, err := a.listAndChoose("Patch update", fileItems, false, false)
	if err != nil {
		return err
	}

	if len(chosen) > 0 {
		var paths []string
		for _, item := range chosen {
			file := item.(git.FileStatus)
			paths = append(paths, file.Path)
		}

		return a.RunPatchMode("stage", "", paths)
	}

	fmt.Println()
	return nil
}

func (a *App) diffCmd() error {
	files, err := a.repo.ListModified("index-only")
	if err != nil {
		return err
	}

	var nonBinaryFiles []interface{}
	for _, file := range files {
		if !file.Binary {
			nonBinaryFiles = append(nonBinaryFiles, file)
		}
	}

	if len(nonBinaryFiles) == 0 {
		return nil
	}

	fmt.Printf(a.colored(a.colors.HeaderColor, "%12s %12s %s\n"), "staged", "unstaged", "path")
	chosen, err := a.listAndChoose("Review diff", nonBinaryFiles, false, true)
	if err != nil {
		return err
	}

	if len(chosen) > 0 {
		var paths []string
		for _, item := range chosen {
			file := item.(git.FileStatus)
			paths = append(paths, file.Path)
		}

		reference := "HEAD"
		if a.repo.IsInitialCommit() {
			emptyTree, err := a.repo.GetEmptyTree()
			if err != nil {
				return err
			}
			reference = emptyTree
		}

		args := append([]string{"diff", "-p", "--cached", reference, "--"}, paths...)
		output, err := a.repo.RunCommand(args...)
		if err != nil {
			return err
		}

		fmt.Print(string(output))
	}

	return nil
}

func (a *App) quitCmd() error {
	fmt.Println("Bye.")
	os.Exit(0)
	return nil
}

func (a *App) helpCmd() error {
	help := a.colored(a.colors.HelpColor, `status        - show paths with changes
update        - add working tree state to the staged set of changes
revert        - revert staged set of changes back to the HEAD version
patch         - pick hunks and update selectively
diff          - view diff between HEAD and index
add untracked - add contents of untracked files to the staged set of changes
`)
	fmt.Print(help)
	return nil
}

func (a *App) listAndChoose(prompt string, items []interface{}, singleton, immediate bool) ([]interface{}, error) {
	if len(items) == 0 {
		return nil, nil
	}

	if singleton {
		return a.listAndChooseSingleton(prompt, items, immediate)
	}

	// Multi-select mode with persistent selection
	selected := make(map[int]bool)

	for {
		// Display items with selection markers
		fmt.Printf(a.colored(a.colors.HeaderColor, "%12s %12s %s\n"), "staged", "unstaged", "path")
		for i, item := range items {
			marker := " "
			if selected[i] {
				marker = "*"
			}
			fmt.Printf("%s%2d: %s\n", marker, i+1, a.formatItem(item))
		}

		promptStr := prompt + ">> "
		fmt.Print(a.colored(a.colors.PromptColor, promptStr))

		input, err := a.promptSingleChar()
		if err != nil {
			return nil, err
		}

		if input == "" {
			// Empty input - finish selecting
			var result []interface{}
			for i, item := range items {
				if selected[i] {
					result = append(result, item)
				}
			}
			return result, nil
		}

		if input == "?" {
			a.printSelectionHelp(singleton)
			continue
		}

		if input == "*" {
			// Select all items
			for i := range items {
				selected[i] = true
			}
			continue
		}

		// Handle deselection with leading dash
		isDeselect := strings.HasPrefix(input, "-")
		if isDeselect {
			input = input[1:]
		}

		choices := strings.Split(input, ",")
		hasError := false

		for _, choice := range choices {
			choice = strings.TrimSpace(choice)
			if choice == "" {
				continue
			}

			// Handle ranges (e.g., "3-5", "1-3")
			if strings.Contains(choice, "-") {
				parts := strings.SplitN(choice, "-", 2)
				if len(parts) == 2 {
					start, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
					end, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))

					if err1 != nil || err2 != nil || start < 1 || end < 1 || start > len(items) || end > len(items) {
						a.printError(fmt.Sprintf("Invalid range: %s\n", choice))
						hasError = true
						break
					}

					if start > end {
						start, end = end, start
					}

					for i := start; i <= end; i++ {
						selected[i-1] = !isDeselect
					}
					continue
				}
			}

			// Handle single numbers
			if num, err := strconv.Atoi(choice); err == nil {
				if num >= 1 && num <= len(items) {
					selected[num-1] = !isDeselect
				} else {
					a.printError(fmt.Sprintf("Invalid number: %s\n", choice))
					hasError = true
					break
				}
			} else {
				a.printError(fmt.Sprintf("Invalid input: %s\n", choice))
				hasError = true
				break
			}
		}

		if hasError {
			continue
		}
	}
}

func (a *App) listAndChooseSingleton(prompt string, items []interface{}, immediate bool) ([]interface{}, error) {
	for {
		for i, item := range items {
			fmt.Printf("%2d: %s\n", i+1, a.formatItem(item))
		}

		if immediate && len(items) == 1 {
			return []interface{}{items[0]}, nil
		}

		promptStr := prompt + "> "
		fmt.Print(a.colored(a.colors.PromptColor, promptStr))

		input, err := a.promptSingleChar()
		if err != nil {
			return nil, err
		}

		if input == "" {
			break
		}

		if input == "?" {
			a.printSelectionHelp(true)
			continue
		}

		// Handle single letter commands
		var selectedCmd *Command
		for _, item := range items {
			if cmd, ok := item.(Command); ok {
				if strings.ToLower(input) == strings.ToLower(cmd.Name[:1]) {
					selectedCmd = &cmd
					break
				}
			}
		}

		// Handle numeric commands
		if selectedCmd == nil {
			if num, err := strconv.Atoi(input); err == nil {
				if num >= 1 && num <= len(items) {
					return []interface{}{items[num-1]}, nil
				}
			}
		}

		if selectedCmd != nil {
			return []interface{}{*selectedCmd}, nil
		}

		a.printError(fmt.Sprintf("Invalid input: %s\n", input))
	}

	return nil, nil
}

func (a *App) formatItem(item interface{}) string {
	switch v := item.(type) {
	case git.FileStatus:
		return fmt.Sprintf("%12s %12s %s", v.Index, v.File, v.Path)
	case string:
		return v
	case Command:
		return v.Name
	default:
		return fmt.Sprintf("%v", v)
	}
}

func (a *App) printSelectionHelp(singleton bool) {
	if singleton {
		help := a.colored(a.colors.HelpColor, `Prompt help:
1          - select a numbered item
foo        - select item based on unique prefix
           - (empty) select nothing
`)
		fmt.Print(help)
	} else {
		help := a.colored(a.colors.HelpColor, `Prompt help:
1          - select a single item
3-5        - select a range of items
2-3,6-9    - select multiple ranges
foo        - select item based on unique prefix
-...       - unselect specified items
*          - choose all items
           - (empty) finish selecting
`)
		fmt.Print(help)
	}
}
