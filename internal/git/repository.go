package git

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Repository struct {
	gitDir   string
	workTree string
}

func NewRepository(path string) (*Repository, error) {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = path
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("not a git repository")
	}

	gitDir := strings.TrimSpace(string(output))
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(path, gitDir)
	}

	workTreeCmd := exec.Command("git", "rev-parse", "--show-toplevel")
	workTreeCmd.Dir = path
	workTreeOutput, err := workTreeCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("could not determine work tree: %v", err)
	}

	workTree := strings.TrimSpace(string(workTreeOutput))

	return &Repository{
		gitDir:   gitDir,
		workTree: workTree,
	}, nil
}

func (r *Repository) GitDir() string {
	return r.gitDir
}

func (r *Repository) WorkTree() string {
	return r.workTree
}

func (r *Repository) RunCommand(args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.workTree
	return cmd.Output()
}

func (r *Repository) RunCommandWithStdin(stdin []byte, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.workTree
	cmd.Stdin = bytes.NewReader(stdin)
	return cmd.Run()
}

func (r *Repository) GetConfig(key string) (string, error) {
	output, err := r.RunCommand("config", key)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func (r *Repository) GetConfigBool(key string) bool {
	output, err := r.RunCommand("config", "--bool", key)
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) == "true"
}

func (r *Repository) GetColor(key, defaultColor string) string {
	output, err := r.RunCommand("config", "--get-color", key, defaultColor)
	if err != nil {
		return ""
	}
	return string(output)
}

func (r *Repository) GetColorBool(key string) bool {
	output, err := r.RunCommand("config", "--get-colorbool", key, "true")
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) == "true"
}

func (r *Repository) IsInitialCommit() bool {
	_, err := r.RunCommand("rev-parse", "HEAD")
	return err != nil
}

func (r *Repository) GetEmptyTree() (string, error) {
	output, err := r.RunCommand("hash-object", "-t", "tree", "/dev/null")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func (r *Repository) UpdateIndex() error {
	cmd := exec.Command("git", "update-index", "--refresh")
	cmd.Dir = r.workTree
	cmd.Stdout = nil
	cmd.Stderr = nil
	_ = cmd.Run()
	return nil
}

func (r *Repository) RepoPath(path string) string {
	return filepath.Join(r.gitDir, path)
}

func (r *Repository) RunCommandLines(args ...string) ([]string, error) {
	output, err := r.RunCommand(args...)
	if err != nil {
		return nil, err
	}

	var lines []string
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

func (r *Repository) FileExists(path string) bool {
	fullPath := filepath.Join(r.workTree, path)
	_, err := os.Stat(fullPath)
	return err == nil
}
