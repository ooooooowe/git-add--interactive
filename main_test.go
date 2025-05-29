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
