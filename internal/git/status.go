package git

import (
	"regexp"
	"strconv"
	"strings"
)

type FileStatus struct {
	Path        string
	Binary      bool
	Index       string
	File        string
	IndexAddDel string
	FileAddDel  string
	Unmerged    bool
}

func (r *Repository) ListModified(filter string) ([]FileStatus, error) {
	return r.ListModifiedWithRevision(filter, "")
}

func (r *Repository) ListModifiedWithRevision(filter, revision string) ([]FileStatus, error) {
	return r.ListModifiedWithRevisionAndPaths(filter, revision, nil)
}

func (r *Repository) ListModifiedWithRevisionAndPaths(filter, revision string, paths []string) ([]FileStatus, error) {
	var files []FileStatus
	statusMap := make(map[string]*FileStatus)

	reference := "HEAD"
	if revision != "" {
		reference = revision
	}
	if r.IsInitialCommit() && reference == "HEAD" {
		emptyTree, err := r.GetEmptyTree()
		if err != nil {
			return nil, err
		}
		reference = emptyTree
	}

	// Only run diff-index if we're not doing file-only filtering
	if filter != "file-only" {
		// Build the diff-index command with optional paths
		indexCmd := []string{"diff-index", "--cached", "--numstat", "--summary", reference}
		if len(paths) > 0 {
			indexCmd = append(indexCmd, "--")
			indexCmd = append(indexCmd, paths...)
		} else {
			indexCmd = append(indexCmd, "--")
		}
		indexLines, err := r.RunCommandLines(indexCmd...)
		if err != nil {
			return nil, err
		}

		for _, line := range indexLines {
			if err := r.parseIndexLine(line, statusMap); err != nil {
				continue
			}
		}
	}

	// Only run diff-files if we're not doing index-only filtering
	if filter != "index-only" {
		// Build the diff-files command with optional paths
		fileCmd := []string{"diff-files", "--ignore-submodules=dirty", "--numstat", "--summary", "--raw"}
		if len(paths) > 0 {
			fileCmd = append(fileCmd, "--")
			fileCmd = append(fileCmd, paths...)
		} else {
			fileCmd = append(fileCmd, "--")
		}
		fileLines, err := r.RunCommandLines(fileCmd...)
		if err != nil {
			return nil, err
		}

		for _, line := range fileLines {
			if err := r.parseFileLine(line, statusMap); err != nil {
				continue
			}
		}
	}

	for path, status := range statusMap {
		if filter == "index-only" && status.Index == "unchanged" {
			continue
		}
		if filter == "file-only" && status.File == "nothing" {
			continue
		}

		status.Path = path
		files = append(files, *status)
	}

	return files, nil
}

func (r *Repository) parseIndexLine(line string, statusMap map[string]*FileStatus) error {
	parts := strings.Split(line, "\t")
	if len(parts) >= 3 {
		add, del, file := parts[0], parts[1], parts[2]
		file = unquotePath(file)

		status := statusMap[file]
		if status == nil {
			status = &FileStatus{
				Index: "unchanged",
				File:  "nothing",
			}
			statusMap[file] = status
		}

		if add == "-" && del == "-" {
			status.Index = "binary"
			status.Binary = true
		} else {
			status.Index = "+" + add + "/-" + del
		}
		return nil
	}

	createDeleteRe := regexp.MustCompile(`^ (create|delete) mode [0-7]+ (.*)$`)
	if matches := createDeleteRe.FindStringSubmatch(line); len(matches) == 3 {
		op, file := matches[1], unquotePath(matches[2])
		status := statusMap[file]
		if status == nil {
			status = &FileStatus{
				Index: "unchanged",
				File:  "nothing",
			}
			statusMap[file] = status
		}
		status.IndexAddDel = op
		return nil
	}

	return nil
}

func (r *Repository) parseFileLine(line string, statusMap map[string]*FileStatus) error {
	parts := strings.Split(line, "\t")
	if len(parts) >= 3 {
		add, del, file := parts[0], parts[1], parts[2]
		file = unquotePath(file)

		status := statusMap[file]
		if status == nil {
			status = &FileStatus{
				Index: "unchanged",
				File:  "nothing",
			}
			statusMap[file] = status
		}

		if add == "-" && del == "-" {
			status.File = "binary"
			status.Binary = true
		} else {
			status.File = "+" + add + "/-" + del
		}
		return nil
	}

	createDeleteRe := regexp.MustCompile(`^ (create|delete) mode [0-7]+ (.*)$`)
	if matches := createDeleteRe.FindStringSubmatch(line); len(matches) == 3 {
		op, file := matches[1], unquotePath(matches[2])
		status := statusMap[file]
		if status == nil {
			status = &FileStatus{
				Index: "unchanged",
				File:  "nothing",
			}
			statusMap[file] = status
		}
		status.FileAddDel = op
		return nil
	}

	rawRe := regexp.MustCompile(`^:[0-7]+ [0-7]+ [0-9a-f]{7,40} [0-9a-f]{7,40} (.)\t(.*)$`)
	if matches := rawRe.FindStringSubmatch(line); len(matches) == 3 {
		statusType, file := matches[1], unquotePath(matches[2])
		fileStatus := statusMap[file]
		if fileStatus == nil {
			fileStatus = &FileStatus{
				Index: "unchanged",
				File:  "nothing",
			}
			statusMap[file] = fileStatus
		}
		if statusType == "U" {
			fileStatus.Unmerged = true
		}
		return nil
	}

	return nil
}

func (r *Repository) ListUntracked() ([]string, error) {
	lines, err := r.RunCommandLines("ls-files", "--others", "--exclude-standard", "--")
	if err != nil {
		return nil, err
	}

	var untracked []string
	for _, line := range lines {
		if line != "" {
			untracked = append(untracked, unquotePath(line))
		}
	}

	return untracked, nil
}

func unquotePath(path string) string {
	if len(path) >= 2 && path[0] == '"' && path[len(path)-1] == '"' {
		if unquoted, err := strconv.Unquote(path); err == nil {
			return unquoted
		}
	}
	return path
}
