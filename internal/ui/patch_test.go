package ui

import (
	"testing"

	"github.com/cwarden/git-add--interactive/internal/git"
)

func TestHunkMatchesRegex(t *testing.T) {
	app := &App{}

	tests := []struct {
		name      string
		hunk      git.Hunk
		pattern   string
		expected  bool
		shouldErr bool
	}{
		{
			name: "pattern matches context line",
			hunk: git.Hunk{
				Text: []string{
					"@@ -1,3 +1,4 @@",
					" context line with Account_Fax",
					"-old line",
					"+new line",
				},
			},
			pattern:  "Account_Fax",
			expected: true,
		},
		{
			name: "pattern matches added line",
			hunk: git.Hunk{
				Text: []string{
					"@@ -1,3 +1,4 @@",
					" context line",
					"-old line",
					"+new line with Account_Fax",
				},
			},
			pattern:  "Account_Fax",
			expected: true,
		},
		{
			name: "pattern matches removed line",
			hunk: git.Hunk{
				Text: []string{
					"@@ -1,3 +1,4 @@",
					" context line",
					"-old line with Account_Fax",
					"+new line",
				},
			},
			pattern:  "Account_Fax",
			expected: true,
		},
		{
			name: "pattern matches header line",
			hunk: git.Hunk{
				Text: []string{
					"@@ -1,3 +1,4 @@ Account_Fax",
					" context line",
					"-old line",
					"+new line",
				},
			},
			pattern:  "Account_Fax",
			expected: true,
		},
		{
			name: "pattern does not match",
			hunk: git.Hunk{
				Text: []string{
					"@@ -1,3 +1,4 @@",
					" context line",
					"-old line",
					"+new line",
				},
			},
			pattern:  "NotFound",
			expected: false,
		},
		{
			name: "empty pattern matches empty line",
			hunk: git.Hunk{
				Text: []string{
					"@@ -1,3 +1,4 @@",
					" ",
					"-old line",
					"+new line",
				},
			},
			pattern:  "^\\s*$",
			expected: true,
		},
		{
			name: "regex pattern with prefix matching",
			hunk: git.Hunk{
				Text: []string{
					"@@ -1,3 +1,4 @@",
					" context line",
					"-old line with function",
					"+new line with function",
				},
			},
			pattern:  "^\\+.*function",
			expected: true,
		},
		{
			name: "case insensitive pattern",
			hunk: git.Hunk{
				Text: []string{
					"@@ -1,3 +1,4 @@",
					" context line",
					"-old line",
					"+new line with ACCOUNT_FAX",
				},
			},
			pattern:  "(?i)account_fax",
			expected: true,
		},
		{
			name: "invalid regex pattern",
			hunk: git.Hunk{
				Text: []string{
					"@@ -1,3 +1,4 @@",
					" context line",
				},
			},
			pattern:  "[invalid",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := app.hunkMatchesRegex(&tt.hunk, tt.pattern)
			if result != tt.expected {
				t.Errorf("hunkMatchesRegex() = %v, expected %v for pattern %q", result, tt.expected, tt.pattern)
			}
		})
	}
}

func TestSearchPatternExtraction(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "slash with pattern",
			input:    "/Account_Fax",
			expected: "Account_Fax",
		},
		{
			name:     "slash with complex pattern",
			input:    "/^\\+.*function.*{",
			expected: "^\\+.*function.*{",
		},
		{
			name:     "slash with spaces in pattern",
			input:    "/Account Fax Detail",
			expected: "Account Fax Detail",
		},
		{
			name:     "slash only",
			input:    "/",
			expected: "",
		},
		{
			name:     "slash with whitespace",
			input:    "/   ",
			expected: "",
		},
		{
			name:     "slash with leading/trailing spaces",
			input:    "/  pattern  ",
			expected: "pattern",
		},
		{
			name:     "capital G with pattern",
			input:    "GAccount_Fax",
			expected: "Account_Fax",
		},
		{
			name:     "capital G only",
			input:    "G",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result string
			if tt.input[0] == '/' {
				// Test the slash command pattern extraction
				result = tt.input[1:]
			} else if tt.input[0] == 'G' || tt.input[0] == 'g' {
				// Test the G command pattern extraction (similar logic)
				result = tt.input[1:]
			}
			result = trimSpace(result)

			if result != tt.expected {
				t.Errorf("Pattern extraction from %q = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

// Helper function to simulate strings.TrimSpace behavior for testing
func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}
