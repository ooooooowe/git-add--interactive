package main

import (
	"testing"
)

func TestProcessArgs(t *testing.T) {
	tests := []struct {
		name             string
		args             []string
		expectedMode     string
		expectedRevision string
		expectedFiles    []string
		expectError      bool
	}{
		{
			name:         "no arguments",
			args:         []string{},
			expectedMode: "",
		},
		{
			name:         "interactive mode with --",
			args:         []string{"--"},
			expectedMode: "",
		},
		{
			name:         "patch mode basic",
			args:         []string{"--patch", "--"},
			expectedMode: "stage",
		},
		{
			name:         "patch mode with stage",
			args:         []string{"--patch=stage", "--"},
			expectedMode: "stage",
		},
		{
			name:         "patch mode with stash",
			args:         []string{"--patch=stash", "--"},
			expectedMode: "stash",
		},
		{
			name:             "patch mode with reset",
			args:             []string{"--patch=reset", "--"},
			expectedMode:     "reset_head",
			expectedRevision: "HEAD",
		},
		{
			name:             "patch mode with reset and revision",
			args:             []string{"--patch=reset", "main", "--"},
			expectedMode:     "reset_nothead",
			expectedRevision: "main",
		},
		{
			name:         "patch mode with checkout index",
			args:         []string{"--patch=checkout", "--"},
			expectedMode: "checkout_index",
		},
		{
			name:             "patch mode with checkout head",
			args:             []string{"--patch=checkout", "HEAD", "--"},
			expectedMode:     "checkout_head",
			expectedRevision: "HEAD",
		},
		{
			name:             "patch mode with checkout revision",
			args:             []string{"--patch=checkout", "main", "--"},
			expectedMode:     "checkout_nothead",
			expectedRevision: "main",
		},
		{
			name:         "patch mode with worktree index",
			args:         []string{"--patch=worktree", "--"},
			expectedMode: "checkout_index",
		},
		{
			name:             "patch mode with worktree head",
			args:             []string{"--patch=worktree", "HEAD", "--"},
			expectedMode:     "worktree_head",
			expectedRevision: "HEAD",
		},
		{
			name:             "patch mode with worktree revision",
			args:             []string{"--patch=worktree", "main", "--"},
			expectedMode:     "worktree_nothead",
			expectedRevision: "main",
		},
		{
			name:          "patch mode with files",
			args:          []string{"--patch", "--", "file1.txt", "file2.txt"},
			expectedMode:  "stage",
			expectedFiles: []string{"file1.txt", "file2.txt"},
		},
		{
			name:        "invalid argument",
			args:        []string{"--invalid"},
			expectError: true,
		},
		{
			name:        "missing -- after patch",
			args:        []string{"--patch"},
			expectError: true,
		},
		{
			name:        "invalid patch mode",
			args:        []string{"--patch=invalid", "--"},
			expectError: true,
		},
		{
			name:        "missing -- after reset",
			args:        []string{"--patch=reset"},
			expectError: true,
		},
		{
			name:        "missing -- after checkout",
			args:        []string{"--patch=checkout"},
			expectError: true,
		},
		{
			name:        "invalid separator",
			args:        []string{"--patch", "not-dash-dash"},
			expectError: true,
		},
		{
			name:          "checkout with pathspec",
			args:          []string{"--patch=checkout", "--", ":(,prefix:0)salesforce/"},
			expectedMode:  "checkout_index",
			expectedFiles: []string{":(,prefix:0)salesforce/"},
		},
		{
			name:          "checkout with multiple pathspecs",
			args:          []string{"--patch=checkout", "--", "src/", "test/"},
			expectedMode:  "checkout_index",
			expectedFiles: []string{"src/", "test/"},
		},
		{
			name:          "checkout with complex pathspec",
			args:          []string{"--patch=checkout", "--", ":(exclude)*.tmp", "*.go"},
			expectedMode:  "checkout_index",
			expectedFiles: []string{":(exclude)*.tmp", "*.go"},
		},
		{
			name:             "checkout with revision and pathspec",
			args:             []string{"--patch=checkout", "HEAD~1", "--", "src/"},
			expectedMode:     "checkout_nothead",
			expectedRevision: "HEAD~1",
			expectedFiles:    []string{"src/"},
		},
		{
			name:          "stage with pathspec",
			args:          []string{"--patch=stage", "--", "modified.txt"},
			expectedMode:  "stage",
			expectedFiles: []string{"modified.txt"},
		},
		{
			name:             "reset with revision and pathspec",
			args:             []string{"--patch=reset", "HEAD~1", "--", "file.txt"},
			expectedMode:     "reset_nothead",
			expectedRevision: "HEAD~1",
			expectedFiles:    []string{"file.txt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mode, revision, files, err := processArgs(tt.args)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if mode != tt.expectedMode {
				t.Errorf("Expected mode %q, got %q", tt.expectedMode, mode)
			}

			if revision != tt.expectedRevision {
				t.Errorf("Expected revision %q, got %q", tt.expectedRevision, revision)
			}

			if len(files) != len(tt.expectedFiles) {
				t.Errorf("Expected %d files, got %d", len(tt.expectedFiles), len(files))
			} else {
				for i, expected := range tt.expectedFiles {
					if files[i] != expected {
						t.Errorf("Expected file[%d] %q, got %q", i, expected, files[i])
					}
				}
			}
		})
	}
}

func TestParsePatchCheckout(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		hasSeparator bool
		expectedMode string
		expectedRev  string
	}{
		{
			name:         "checkout with separator and single pathspec - should be checkout_index",
			args:         []string{":(,prefix:0)salesforce/"},
			hasSeparator: true,
			expectedMode: "checkout_index",
			expectedRev:  "",
		},
		{
			name:         "checkout with separator and revision plus pathspec",
			args:         []string{"HEAD~1", "src/"},
			hasSeparator: true,
			expectedMode: "checkout_nothead",
			expectedRev:  "HEAD~1",
		},
		{
			name:         "checkout with separator and multiple pathspecs",
			args:         []string{"src/", "test/"},
			hasSeparator: true,
			expectedMode: "checkout_index",
			expectedRev:  "",
		},
		{
			name:         "checkout with separator and pathspec magic",
			args:         []string{":(exclude)*.tmp", "*.go"},
			hasSeparator: true,
			expectedMode: "checkout_index",
			expectedRev:  "",
		},
		{
			name:         "checkout without separator - should parse as revision",
			args:         []string{"HEAD"},
			hasSeparator: false,
			expectedMode: "checkout_head",
			expectedRev:  "HEAD",
		},
		{
			name:         "checkout with custom revision",
			args:         []string{"main"},
			hasSeparator: false,
			expectedMode: "checkout_nothead",
			expectedRev:  "main",
		},
		{
			name:         "checkout with empty args and separator",
			args:         []string{},
			hasSeparator: true,
			expectedMode: "checkout_index",
			expectedRev:  "",
		},
		{
			name:         "checkout with -- in args",
			args:         []string{"--"},
			hasSeparator: true,
			expectedMode: "checkout_index",
			expectedRev:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mode, rev := parsePatchCheckout(tt.args, tt.hasSeparator)

			if mode != tt.expectedMode {
				t.Errorf("Expected mode %q, got %q", tt.expectedMode, mode)
			}

			if rev != tt.expectedRev {
				t.Errorf("Expected revision %q, got %q", tt.expectedRev, rev)
			}
		})
	}
}

func TestSkipRevisionAndSeparator(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected []string
	}{
		{
			name:     "skip revision",
			args:     []string{"HEAD", "file.txt"},
			expected: []string{"file.txt"},
		},
		{
			name:     "skip -- separator",
			args:     []string{"--", "file.txt"},
			expected: []string{"--", "file.txt"},
		},
		{
			name:     "empty args",
			args:     []string{},
			expected: []string{},
		},
		{
			name:     "single file",
			args:     []string{"file.txt"},
			expected: []string{},
		},
		{
			name:     "pathspec gets skipped incorrectly",
			args:     []string{":(,prefix:0)salesforce/"},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := skipRevisionAndSeparator(tt.args)

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d args, got %d", len(tt.expected), len(result))
				return
			}

			for i, expected := range tt.expected {
				if result[i] != expected {
					t.Errorf("Expected arg[%d] %q, got %q", i, expected, result[i])
				}
			}
		})
	}
}
