package git

import (
	"os"
	"testing"
)

func TestNewRepository(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	repo, err := NewRepository(wd)
	if err != nil {
		t.Skip("Not in a git repository, skipping test")
	}

	if repo.GitDir() == "" {
		t.Error("GitDir() should not be empty")
	}

	if repo.WorkTree() == "" {
		t.Error("WorkTree() should not be empty")
	}
}

func TestGetConfigBool(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	repo, err := NewRepository(wd)
	if err != nil {
		t.Skip("Not in a git repository, skipping test")
	}

	result := repo.GetConfigBool("nonexistent.config.key")
	if result {
		t.Error("Non-existent config key should return false")
	}
}

func TestIsInitialCommit(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	repo, err := NewRepository(wd)
	if err != nil {
		t.Skip("Not in a git repository, skipping test")
	}

	repo.IsInitialCommit()
}
