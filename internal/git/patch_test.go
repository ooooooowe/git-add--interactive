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

	tests := []struct {
		name       string
		hunk       *Hunk
		splittable bool
	}{
		{
			name: "hunk with context after first change",
			hunk: &Hunk{
				Type:    HunkTypeHunk,
				OldLine: 1,
				NewLine: 1,
				Text: []string{
					"@@ -1,7 +1,8 @@",
					"-old line 1",
					"+new line 1",
					" context line",
					" context line",
					"-old line 2",
					"+new line 2",
				},
			},
			splittable: true,
		},
		{
			name: "hunk without context between changes",
			hunk: &Hunk{
				Type:    HunkTypeHunk,
				OldLine: 1,
				NewLine: 1,
				Text: []string{
					"@@ -1,4 +1,4 @@",
					"-old line 1",
					"+new line 1",
					"-old line 2",
					"+new line 2",
				},
			},
			splittable: false,
		},
		{
			name: "single change hunk",
			hunk: &Hunk{
				Type:    HunkTypeHunk,
				OldLine: 1,
				NewLine: 1,
				Text: []string{
					"@@ -1,3 +1,3 @@",
					" context",
					"-old line",
					"+new line",
				},
			},
			splittable: false,
		},
		{
			name: "non-hunk type",
			hunk: &Hunk{
				Type: HunkTypeHeader,
				Text: []string{"diff --git a/file b/file"},
			},
			splittable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := repo.HunkSplittable(tt.hunk)
			if result != tt.splittable {
				t.Errorf("HunkSplittable() = %v, want %v", result, tt.splittable)
			}
		})
	}
}

func TestUpdateHunkHeader(t *testing.T) {
	repo := &Repository{}

	tests := []struct {
		name           string
		hunk           *Hunk
		expectedHeader string
	}{
		{
			name: "header with multi-line counts",
			hunk: &Hunk{
				OldLine: 10,
				OldCnt:  5,
				NewLine: 15,
				NewCnt:  7,
				Text:    []string{" existing content"},
				Display: []string{" existing content"},
			},
			expectedHeader: "@@ -10,5 +15,7 @@",
		},
		{
			name: "header with single line counts",
			hunk: &Hunk{
				OldLine: 5,
				OldCnt:  1,
				NewLine: 8,
				NewCnt:  1,
				Text:    []string{"+new line"},
				Display: []string{"+new line"},
			},
			expectedHeader: "@@ -5 +8 @@",
		},
		{
			name: "empty hunk",
			hunk: &Hunk{
				OldLine: 20,
				OldCnt:  3,
				NewLine: 25,
				NewCnt:  2,
			},
			expectedHeader: "@@ -20,3 +25,2 @@",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalTextLen := len(tt.hunk.Text)
			originalDisplayLen := len(tt.hunk.Display)

			repo.updateHunkHeader(tt.hunk)

			// Header should be inserted at the beginning
			if len(tt.hunk.Text) == 0 || tt.hunk.Text[0] != tt.expectedHeader {
				t.Errorf("Expected header %s, got %v", tt.expectedHeader, tt.hunk.Text)
			}

			// Length should increase by 1
			if len(tt.hunk.Text) != originalTextLen+1 {
				t.Errorf("Expected text length %d, got %d", originalTextLen+1, len(tt.hunk.Text))
			}

			if len(tt.hunk.Display) != originalDisplayLen+1 {
				t.Errorf("Expected display length %d, got %d", originalDisplayLen+1, len(tt.hunk.Display))
			}

			// Display should also have the header
			if len(tt.hunk.Display) == 0 || tt.hunk.Display[0] != tt.expectedHeader {
				t.Errorf("Expected display header %s, got %v", tt.expectedHeader, tt.hunk.Display)
			}
		})
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

func TestHunkSplittingWithContext(t *testing.T) {
	repo := &Repository{}

	tests := []struct {
		name           string
		hunk           *Hunk
		expectedSplits int
		expectedSplit1 []string
		expectedSplit2 []string
	}{
		{
			name: "basic split with context line - real test case",
			hunk: &Hunk{
				Type:    HunkTypeHunk,
				OldLine: 19,
				NewLine: 19,
				Text: []string{
					"@@ -19,7 +19,9 @@",
					" 	}",
					" }",
					" ",
					"+// comment1",
					" func TestParseHunkHeader(t *testing.T) {",
					"+	//comment2",
					" 	repo := &Repository{}",
					" 	hunk := &Hunk{",
					" 		Text: []string{\"@@ -1,5 +2,6 @@ function test\"},",
				},
				Display: []string{
					"@@ -19,7 +19,9 @@",
					" 	}",
					" }",
					" ",
					"+// comment1",
					" func TestParseHunkHeader(t *testing.T) {",
					"+	//comment2",
					" 	repo := &Repository{}",
					" 	hunk := &Hunk{",
					" 		Text: []string{\"@@ -1,5 +2,6 @@ function test\"},",
				},
			},
			expectedSplits: 2,
			expectedSplit1: []string{
				"@@ -19,4 +19,5 @@",
				" 	}",
				" }",
				" ",
				"+// comment1",
				" func TestParseHunkHeader(t *testing.T) {",
			},
			expectedSplit2: []string{
				"@@ -22,4 +23,5 @@",
				" func TestParseHunkHeader(t *testing.T) {",
				"+	//comment2",
				" 	repo := &Repository{}",
				" 	hunk := &Hunk{",
				" 		Text: []string{\"@@ -1,5 +2,6 @@ function test\"},",
			},
		},
		{
			name: "complex hunk with multiple context sections",
			hunk: &Hunk{
				Type:    HunkTypeHunk,
				OldLine: 1,
				NewLine: 1,
				Text: []string{
					"@@ -1,10 +1,12 @@",
					" function test() {",
					"-    var a = 1;",
					"-    var b = 2;",
					"+    let a = 1;",
					"+    let b = 2;",
					"+    let c = 3;",
					"     ",
					"     console.log(a);",
					"-    var d = 4;",
					"-    var e = 5;",
					"+    let d = 4;",
					"+    let e = 5;",
					"+    let f = 6;",
					" }",
				},
				Display: []string{
					"@@ -1,10 +1,12 @@",
					" function test() {",
					"-    var a = 1;",
					"-    var b = 2;",
					"+    let a = 1;",
					"+    let b = 2;",
					"+    let c = 3;",
					"     ",
					"     console.log(a);",
					"-    var d = 4;",
					"-    var e = 5;",
					"+    let d = 4;",
					"+    let e = 5;",
					"+    let f = 6;",
					" }",
				},
			},
			expectedSplits: 2,
			expectedSplit1: []string{
				"@@ -1,6 +1,7 @@",
				" function test() {",
				"-    var a = 1;",
				"-    var b = 2;",
				"+    let a = 1;",
				"+    let b = 2;",
				"+    let c = 3;",
				"     ",
			},
			expectedSplit2: []string{
				"@@ -6,6 +7,7 @@",
				"     ",
				"     console.log(a);",
				"-    var d = 4;",
				"-    var e = 5;",
				"+    let d = 4;",
				"+    let e = 5;",
				"+    let f = 6;",
				" }",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the hunk header first
			err := repo.parseHunkHeader(tt.hunk)
			if err != nil {
				t.Fatalf("Failed to parse hunk header: %v", err)
			}

			// Test that it's splittable
			if !repo.HunkSplittable(tt.hunk) {
				t.Error("Hunk with context separating changes should be splittable")
			}

			// Test the actual splitting
			splits := repo.SplitHunk(tt.hunk)
			if len(splits) != tt.expectedSplits {
				t.Errorf("Expected %d splits, got %d", tt.expectedSplits, len(splits))
			}

			// Verify each split has a valid header
			for i, split := range splits {
				if len(split.Text) == 0 {
					t.Errorf("Split %d has no text", i)
					continue
				}
				if !strings.HasPrefix(split.Text[0], "@@") {
					t.Errorf("Split %d has invalid header: %s", i, split.Text[0])
				}
			}

			// Test specific split content for the first test case
			if tt.name == "basic split with context line - real test case" && len(splits) == 2 {
				// Verify split 1 content
				if len(splits[0].Text) != len(tt.expectedSplit1) {
					t.Errorf("Split 1 length mismatch: expected %d, got %d", len(tt.expectedSplit1), len(splits[0].Text))
				} else {
					for j, expectedLine := range tt.expectedSplit1 {
						if j < len(splits[0].Text) && splits[0].Text[j] != expectedLine {
							t.Errorf("Split 1 line %d: expected %q, got %q", j, expectedLine, splits[0].Text[j])
						}
					}
				}

				// Verify split 2 content
				if len(splits[1].Text) != len(tt.expectedSplit2) {
					t.Errorf("Split 2 length mismatch: expected %d, got %d", len(tt.expectedSplit2), len(splits[1].Text))
				} else {
					for j, expectedLine := range tt.expectedSplit2 {
						if j < len(splits[1].Text) && splits[1].Text[j] != expectedLine {
							t.Errorf("Split 2 line %d: expected %q, got %q", j, expectedLine, splits[1].Text[j])
						}
					}
				}

				// Verify overlapping context line
				if len(splits[0].Text) > 4 && len(splits[1].Text) > 1 {
					lastLineOfSplit1 := splits[0].Text[len(splits[0].Text)-1]
					firstContentLineOfSplit2 := splits[1].Text[1] // Skip header
					if lastLineOfSplit1 != firstContentLineOfSplit2 {
						t.Errorf("Expected overlapping context line: split1 ends with %q, split2 starts with %q",
							lastLineOfSplit1, firstContentLineOfSplit2)
					}
				}
			}
		})
	}
}

func TestSplitHunkNonSplittable(t *testing.T) {
	repo := &Repository{}

	tests := []struct {
		name string
		hunk *Hunk
	}{
		{
			name: "hunk without context between changes",
			hunk: &Hunk{
				Type:    HunkTypeHunk,
				OldLine: 1,
				NewLine: 1,
				Text: []string{
					"@@ -1,4 +1,4 @@",
					"-old line 1",
					"+new line 1",
					"-old line 2",
					"+new line 2",
				},
				Display: []string{
					"@@ -1,4 +1,4 @@",
					"-old line 1",
					"+new line 1",
					"-old line 2",
					"+new line 2",
				},
			},
		},
		{
			name: "single change hunk",
			hunk: &Hunk{
				Type:    HunkTypeHunk,
				OldLine: 5,
				NewLine: 5,
				Text: []string{
					"@@ -5,3 +5,3 @@",
					" context before",
					"-old line",
					"+new line",
					" context after",
				},
				Display: []string{
					"@@ -5,3 +5,3 @@",
					" context before",
					"-old line",
					"+new line",
					" context after",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the hunk header first
			err := repo.parseHunkHeader(tt.hunk)
			if err != nil {
				t.Fatalf("Failed to parse hunk header: %v", err)
			}

			// Should not be splittable
			if repo.HunkSplittable(tt.hunk) {
				t.Error("Hunk should not be splittable")
			}

			// SplitHunk should return the original hunk unchanged
			splits := repo.SplitHunk(tt.hunk)
			if len(splits) != 1 {
				t.Errorf("Expected 1 split (original hunk), got %d", len(splits))
			}

			if len(splits) > 0 {
				// Should be the same hunk
				if len(splits[0].Text) != len(tt.hunk.Text) {
					t.Errorf("Split hunk text length changed: expected %d, got %d",
						len(tt.hunk.Text), len(splits[0].Text))
				}

				for i, line := range tt.hunk.Text {
					if i < len(splits[0].Text) && splits[0].Text[i] != line {
						t.Errorf("Split hunk text changed at line %d: expected %q, got %q",
							i, line, splits[0].Text[i])
					}
				}
			}
		})
	}
}
