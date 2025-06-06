package git

import (
	"testing"
)

func TestUnquotePath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple.txt", "simple.txt"},
		{"\"quoted path.txt\"", "quoted path.txt"},
		{"\"path with\\ttab.txt\"", "path with\ttab.txt"},
		{"\"path with\\nnewline.txt\"", "path with\nnewline.txt"},
		{"not-quoted", "not-quoted"},
		{"\"unclosed quote", "\"unclosed quote"},
	}

	for _, test := range tests {
		result := unquotePath(test.input)
		if result != test.expected {
			t.Errorf("unquotePath(%q) = %q, expected %q", test.input, result, test.expected)
		}
	}
}

func TestParseIndexLine(t *testing.T) {
	repo := &Repository{}
	statusMap := make(map[string]*FileStatus)

	line := "10\t5\ttest.txt"
	err := repo.parseIndexLine(line, statusMap)
	if err != nil {
		t.Fatalf("Failed to parse index line: %v", err)
	}

	if len(statusMap) != 1 {
		t.Errorf("Expected 1 file in status map, got %d", len(statusMap))
	}

	status := statusMap["test.txt"]
	if status == nil {
		t.Fatal("Expected test.txt in status map")
	}

	if status.Index != "+10/-5" {
		t.Errorf("Expected Index=+10/-5, got %s", status.Index)
	}
}

func TestParseIndexLineBinary(t *testing.T) {
	repo := &Repository{}
	statusMap := make(map[string]*FileStatus)

	line := "-\t-\tbinary.jpg"
	err := repo.parseIndexLine(line, statusMap)
	if err != nil {
		t.Fatalf("Failed to parse binary index line: %v", err)
	}

	status := statusMap["binary.jpg"]
	if status == nil {
		t.Fatal("Expected binary.jpg in status map")
	}

	if status.Index != "binary" {
		t.Errorf("Expected Index=binary, got %s", status.Index)
	}

	if !status.Binary {
		t.Error("Expected Binary=true for binary file")
	}
}

func TestParseCreateDeleteLine(t *testing.T) {
	repo := &Repository{}
	statusMap := make(map[string]*FileStatus)

	line := " create mode 100644 newfile.txt"
	err := repo.parseIndexLine(line, statusMap)
	if err != nil {
		t.Fatalf("Failed to parse create line: %v", err)
	}

	status := statusMap["newfile.txt"]
	if status == nil {
		t.Fatal("Expected newfile.txt in status map")
	}

	if status.IndexAddDel != "create" {
		t.Errorf("Expected IndexAddDel=create, got %s", status.IndexAddDel)
	}
}

func TestParseRawLine(t *testing.T) {
	repo := &Repository{}
	statusMap := make(map[string]*FileStatus)

	line := ":100644 100644 1234567890abcdef 1234567890abcdef M\tmodified.txt"
	t.Logf("Line to parse: %q", line)
	err := repo.parseFileLine(line, statusMap)
	if err != nil {
		t.Fatalf("Failed to parse raw line: %v", err)
	}

	t.Logf("Status map after parsing: %+v", statusMap)
	status := statusMap["modified.txt"]
	if status == nil {
		t.Fatal("Expected modified.txt in status map")
	}

	if status.Unmerged {
		t.Error("Expected Unmerged=false for normal file")
	}
}

func TestParseUnmergedLine(t *testing.T) {
	repo := &Repository{}
	statusMap := make(map[string]*FileStatus)

	line := ":100644 100644 1234567890abcdef 1234567890abcdef U\tconflicted.txt"
	err := repo.parseFileLine(line, statusMap)
	if err != nil {
		t.Fatalf("Failed to parse unmerged line: %v", err)
	}

	status := statusMap["conflicted.txt"]
	if status == nil {
		t.Fatal("Expected conflicted.txt in status map")
	}

	if !status.Unmerged {
		t.Error("Expected Unmerged=true for conflicted file")
	}
}

func TestListModifiedWithRevisionAndPaths(t *testing.T) {
	// This test validates the function signature and argument passing
	// Actual git command testing would require a real git repository
	repo := &Repository{
		workTree: "/test/repo",
	}

	// Test that paths are properly passed through
	// We can't easily test the actual git commands without a real repo,
	// but we can verify the function exists and has the right signature
	_, err := repo.ListModifiedWithRevisionAndPaths("file-only", "", []string{"test.txt"})

	// We expect an error since this isn't a real git repo
	if err == nil {
		t.Error("Expected error for non-git directory, but got none")
	}
}

func TestListModifiedWithRevisionAndPathsFilters(t *testing.T) {
	// Test the filter logic without actual git commands
	repo := &Repository{
		workTree: "/test/repo",
	}

	// Test file-only filter - should skip diff-index
	_, err := repo.ListModifiedWithRevisionAndPaths("file-only", "HEAD", []string{})
	if err == nil {
		t.Error("Expected error for non-git directory, but got none")
	}

	// Test index-only filter - should skip diff-files
	_, err = repo.ListModifiedWithRevisionAndPaths("index-only", "HEAD", []string{})
	if err == nil {
		t.Error("Expected error for non-git directory, but got none")
	}

	// Test no filter - should run both commands
	_, err = repo.ListModifiedWithRevisionAndPaths("", "HEAD", []string{})
	if err == nil {
		t.Error("Expected error for non-git directory, but got none")
	}
}
