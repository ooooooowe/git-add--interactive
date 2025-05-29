package git

import (
	"strings"
	"testing"
)

func TestPatchModeExists(t *testing.T) {
	expectedModes := []string{
		"stage", "stash", "reset_head", "reset_nothead",
		"checkout_index", "checkout_head", "checkout_nothead",
		"worktree_head", "worktree_nothead",
	}

	for _, mode := range expectedModes {
		if _, exists := PatchModes[mode]; !exists {
			t.Errorf("Patch mode %s should exist", mode)
		}
	}
}

func TestParseHunkHeader(t *testing.T) {
	repo := &Repository{}
	hunk := &Hunk{
		Text: []string{"@@ -1,5 +2,6 @@ function test"},
		Type: HunkTypeHunk,
	}

	err := repo.parseHunkHeader(hunk)
	if err != nil {
		t.Fatalf("Failed to parse hunk header: %v", err)
	}

	if hunk.OldLine != 1 {
		t.Errorf("Expected OldLine=1, got %d", hunk.OldLine)
	}
	if hunk.OldCnt != 5 {
		t.Errorf("Expected OldCnt=5, got %d", hunk.OldCnt)
	}
	if hunk.NewLine != 2 {
		t.Errorf("Expected NewLine=2, got %d", hunk.NewLine)
	}
	if hunk.NewCnt != 6 {
		t.Errorf("Expected NewCnt=6, got %d", hunk.NewCnt)
	}
}

func TestParseHunkHeaderSingleLine(t *testing.T) {
	repo := &Repository{}
	hunk := &Hunk{
		Text: []string{"@@ -1 +2 @@ function test"},
		Type: HunkTypeHunk,
	}

	err := repo.parseHunkHeader(hunk)
	if err != nil {
		t.Fatalf("Failed to parse hunk header: %v", err)
	}

	if hunk.OldLine != 1 {
		t.Errorf("Expected OldLine=1, got %d", hunk.OldLine)
	}
	if hunk.OldCnt != 1 {
		t.Errorf("Expected OldCnt=1, got %d", hunk.OldCnt)
	}
	if hunk.NewLine != 2 {
		t.Errorf("Expected NewLine=2, got %d", hunk.NewLine)
	}
	if hunk.NewCnt != 1 {
		t.Errorf("Expected NewCnt=1, got %d", hunk.NewCnt)
	}
}

func TestHunkSplittable(t *testing.T) {
	repo := &Repository{}

	splittableHunk := &Hunk{
		Type: HunkTypeHunk,
		Text: []string{
			"@@ -1,10 +1,10 @@",
			"-old line 1",
			"+new line 1",
			" context line",
			" context line",
			" context line",
			"-old line 2",
			"+new line 2",
		},
	}

	if !repo.HunkSplittable(splittableHunk) {
		t.Error("Hunk with context between changes should be splittable")
	}

	nonSplittableHunk := &Hunk{
		Type: HunkTypeHunk,
		Text: []string{
			"@@ -1,5 +1,5 @@",
			"-old line 1",
			"+new line 1",
			"-old line 2",
			"+new line 2",
		},
	}

	if repo.HunkSplittable(nonSplittableHunk) {
		t.Error("Hunk without context between changes should not be splittable")
	}
}

func TestUpdateHunkHeader(t *testing.T) {
	repo := &Repository{}
	hunk := &Hunk{
		OldLine: 10,
		OldCnt:  5,
		NewLine: 15,
		NewCnt:  7,
	}

	repo.updateHunkHeader(hunk)

	expectedHeader := "@@ -10,5 +15,7 @@"
	if len(hunk.Text) == 0 || hunk.Text[0] != expectedHeader {
		t.Errorf("Expected header %s, got %v", expectedHeader, hunk.Text)
	}
}

func TestParseHunks(t *testing.T) {
	repo := &Repository{}
	diffLines := []string{
		"diff --git a/test.txt b/test.txt",
		"index 1234567..abcdefg 100644",
		"--- a/test.txt",
		"+++ b/test.txt",
		"@@ -1,3 +1,4 @@",
		" line 1",
		"-line 2",
		"+line 2 modified",
		"+line 2.5",
		" line 3",
		"@@ -10,2 +11,3 @@",
		" line 10",
		"+new line",
		" line 11",
	}

	hunks, err := repo.parseHunks(diffLines, diffLines)
	if err != nil {
		t.Fatalf("Failed to parse hunks: %v", err)
	}

	if len(hunks) != 3 {
		t.Errorf("Expected 3 hunks (header + 2 actual hunks), got %d", len(hunks))
	}

	if hunks[0].Type != HunkTypeHeader {
		t.Errorf("First hunk should be header, got %s", hunks[0].Type)
	}

	if hunks[1].Type != HunkTypeHunk {
		t.Errorf("Second hunk should be hunk, got %s", hunks[1].Type)
	}

	if hunks[2].Type != HunkTypeHunk {
		t.Errorf("Third hunk should be hunk, got %s", hunks[2].Type)
	}

	if len(hunks[1].Text) != 6 {
		t.Errorf("First actual hunk should have 6 lines, got %d", len(hunks[1].Text))
	}

	if !strings.HasPrefix(hunks[1].Text[0], "@@ -1,3 +1,4 @@") {
		t.Errorf("First hunk should start with @@ header, got %s", hunks[1].Text[0])
	}
}
