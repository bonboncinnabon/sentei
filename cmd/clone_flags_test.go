package cmd

import (
	"testing"
)

func TestParseCloneFlags_URLOnly(t *testing.T) {
	opts, err := ParseCloneFlags([]string{"--url", "https://github.com/org/repo.git"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.URL != "https://github.com/org/repo.git" {
		t.Errorf("URL = %q, want %q", opts.URL, "https://github.com/org/repo.git")
	}
	if opts.Name != "" {
		t.Errorf("Name = %q, want empty string", opts.Name)
	}
}

func TestParseCloneFlags_URLAndName(t *testing.T) {
	opts, err := ParseCloneFlags([]string{"--url", "https://github.com/org/repo.git", "--name", "my-repo"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.URL != "https://github.com/org/repo.git" {
		t.Errorf("URL = %q, want %q", opts.URL, "https://github.com/org/repo.git")
	}
	if opts.Name != "my-repo" {
		t.Errorf("Name = %q, want %q", opts.Name, "my-repo")
	}
}

func TestParseCloneFlags_NoFlags(t *testing.T) {
	opts, err := ParseCloneFlags([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.URL != "" {
		t.Errorf("URL = %q, want empty string", opts.URL)
	}
	if opts.Name != "" {
		t.Errorf("Name = %q, want empty string", opts.Name)
	}
}

func TestValidateCloneForNonInteractive_MissingURL(t *testing.T) {
	opts := &CloneOptions{URL: ""}
	err := ValidateCloneForNonInteractive(opts)
	if err == nil {
		t.Fatal("expected error for missing URL")
	}
	if err.Error() != "missing required flag: --url" {
		t.Errorf("error = %q, want %q", err.Error(), "missing required flag: --url")
	}
}

func TestValidateCloneForNonInteractive_Valid(t *testing.T) {
	opts := &CloneOptions{URL: "https://github.com/org/repo.git"}
	err := ValidateCloneForNonInteractive(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
