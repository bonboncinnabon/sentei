package main

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Interactive decision commands used to detect the repository from the
// current directory and ignore the positional path, so
// `sentei cleanup /path/repoB` opened (and cleaned) the repository in the cwd.
func TestParseDecision_PositionalPathSelectsRepository(t *testing.T) {
	other := t.TempDir()
	cwd, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		command string
		args    []string
		want    string
	}{
		{"cleanup with path", "cleanup", []string{"--mode", "aggressive", other}, other},
		{"create with path", "create", []string{"--branch", "x", "--base", "main", other}, other},
		{"remove with path", "remove", []string{"--all", other}, other},
		{"migrate with path", "migrate", []string{other}, other},
		{"remove without path", "remove", []string{"--all"}, cwd},
		{"clone takes no repo path", "clone", []string{"--url", "git@example.com:a/b.git"}, cwd},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := parseDecision(tt.command, tt.args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if parsed.repoPath != tt.want {
				t.Errorf("repoPath = %q, want %q", parsed.repoPath, tt.want)
			}
		})
	}
}

func TestParseDecision_ResolvesRelativePath(t *testing.T) {
	parsed, err := parseDecision("migrate", []string{"."})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !filepath.IsAbs(parsed.repoPath) {
		t.Errorf("repoPath %q must be absolute before the migrate flow uses it", parsed.repoPath)
	}
}

func TestParseDecision_ReturnsFlagErrors(t *testing.T) {
	if _, err := parseDecision("remove", []string{"--all", "/repo", "--dry-run"}); err == nil {
		t.Error("expected the argument after the repo path to be rejected")
	}
}

func TestVersionFlag(t *testing.T) {
	bin := t.TempDir() + "/sentei"
	build := exec.Command("go", "build", "-ldflags", "-X main.version=v1.2.3 -X main.commit=abc1234 -X main.date=1970-01-01", "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}

	t.Run("prints version and exits", func(t *testing.T) {
		out, err := exec.Command(bin, "--version").Output()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := strings.TrimSpace(string(out))
		want := "sentei v1.2.3 (abc1234, 1970-01-01)"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("default version is dev", func(t *testing.T) {
		devBin := t.TempDir() + "/sentei"
		build := exec.Command("go", "build", "-o", devBin, ".")
		if out, err := build.CombinedOutput(); err != nil {
			t.Fatalf("build failed: %v\n%s", err, out)
		}
		out, err := exec.Command(devBin, "--version").Output()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := strings.TrimSpace(string(out))
		want := "sentei dev (none, unknown)"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("version takes precedence over other flags", func(t *testing.T) {
		out, err := exec.Command(bin, "--version", "--dry-run").Output()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := strings.TrimSpace(string(out))
		want := "sentei v1.2.3 (abc1234, 1970-01-01)"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}
