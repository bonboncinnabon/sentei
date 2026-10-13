package cmd

import (
	"strings"
	"testing"
)

func TestParseMigrateFlags_NoArgs(t *testing.T) {
	opts, err := ParseMigrateFlags([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.RepoPath != "" {
		t.Errorf("expected empty RepoPath, got %q", opts.RepoPath)
	}
}

func TestParseMigrateFlags_WithRepoPath(t *testing.T) {
	opts, err := ParseMigrateFlags([]string{"/some/repo/path"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.RepoPath != "/some/repo/path" {
		t.Errorf("RepoPath = %q, want %q", opts.RepoPath, "/some/repo/path")
	}
}

// Migrate no longer makes a backup, so there is none to delete. The flag is
// rejected rather than accepted and ignored.
func TestParseMigrateFlags_RejectsDeleteBackup(t *testing.T) {
	_, err := ParseMigrateFlags([]string{"--delete-backup", "/some/repo/path"})
	if err == nil || !strings.Contains(err.Error(), "delete-backup") {
		t.Fatalf("err = %v, want the removed flag rejected by name", err)
	}
}

func TestValidateMigrateForNonInteractive_MissingPath(t *testing.T) {
	opts := &MigrateOptions{}
	err := ValidateMigrateForNonInteractive(opts)
	if err == nil {
		t.Fatal("expected error for missing repo path")
	}
	if !strings.Contains(err.Error(), "repo path required") {
		t.Errorf("error = %q, want message containing %q", err.Error(), "repo path required")
	}
}

func TestValidateMigrateForNonInteractive_Valid(t *testing.T) {
	opts := &MigrateOptions{RepoPath: "/some/repo"}
	err := ValidateMigrateForNonInteractive(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
