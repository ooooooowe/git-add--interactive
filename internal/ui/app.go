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
		fmt.Print(a.colored(a.colors.HeaderColor, "*** Commands ***\n"))

		var cmdItems []interface{}
		for _, cmd := range commands {
			cmdItems = append(cmdItems, cmd)
		}

		choice, err := a.listAndChoose("What now", cmdItems, true, true)
		if err != nil {
			return err
		}

		if len(choice) == 0 {
			break
		}

		if err := choice[0].(Command).Action(); err != nil {
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

	files, err := a.repo.ListModifiedWithRevision(patchMode.Filter, revision)
	if err != nil {
		return err
	}

	var filteredFiles []git.FileStatus
	for _, file := range files {
		if !file.Unmerged && !file.Binary {
			if len(paths) == 0 || a.containsPath(paths, file.Path) {
				filteredFiles = append(filteredFiles, file)
			}
		}
	}

	if len(filteredFiles) == 0 {
		fmt.Println("No changes.")
		return nil
	}

	for _, file := range filteredFiles {
		if err := a.patchUpdateFile(file.Path, patchMode, revision); err != nil {
			if errors.Is(err, ErrQuit) {
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
	return a.RunPatchMode("stage", "", nil)
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

	for {
		for i, item := range items {
			fmt.Printf("%2d: %s\n", i+1, a.formatItem(item))
		}

		if immediate && len(items) == 1 {
			return []interface{}{items[0]}, nil
		}

		promptStr := prompt
		if singleton {
			promptStr += "> "
		} else {
			promptStr += ">> "
		}

		fmt.Print(a.colored(a.colors.PromptColor, promptStr))

		input, err := a.promptSingleChar()
		if err != nil {
			return nil, err
		}

		if input == "" {
			break
		}

		if input == "?" {
			a.printSelectionHelp(singleton)
			continue
		}

		choices := strings.Split(input, ",")
		var selected []interface{}

		for _, choice := range choices {
			choice = strings.TrimSpace(choice)

			if num, err := strconv.Atoi(choice); err == nil {
				if num >= 1 && num <= len(items) {
					selected = append(selected, items[num-1])
				} else {
					a.printError(fmt.Sprintf("Invalid number: %s\n", choice))
					continue
				}
			} else {
				a.printError(fmt.Sprintf("Invalid input: %s\n", choice))
				continue
			}
		}

		return selected, nil
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
           - (empty) select nothing
`)
		fmt.Print(help)
	} else {
		help := a.colored(a.colors.HelpColor, `Prompt help:
1          - select a single item
3-5        - select a range of items
2-3,6-9    - select multiple ranges
           - (empty) finish selecting
`)
		fmt.Print(help)
	}
}
