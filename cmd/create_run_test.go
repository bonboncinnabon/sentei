package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/config"
	"github.com/bonboncinnabon/sentei/internal/creator"
	"github.com/bonboncinnabon/sentei/internal/git"
)

func TestCreateResultErrorPropagatesContractError(t *testing.T) {
	want := errors.New("progress delivery failed")
	if got := createResultError(creator.Result{Err: want}); !errors.Is(got, want) {
		t.Fatalf("createResultError() = %v, want wrapped %v", got, want)
	}
}

func TestMatchEcosystems(t *testing.T) {
	available := []config.EcosystemConfig{
		{Name: "pnpm"},
		{Name: "go"},
		{Name: "uv"},
	}
	tests := []struct {
		name      string
		requested []string
		want      []string
	}{
		{"single match", []string{"go"}, []string{"go"}},
		{"multiple preserve config order", []string{"uv", "pnpm"}, []string{"pnpm", "uv"}},
		{"unknown name ignored", []string{"cargo"}, nil},
		{"empty request", nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchEcosystems(available, tt.requested)
			var names []string
			for _, eco := range got {
				names = append(names, eco.Name)
			}
			if !reflect.DeepEqual(names, tt.want) {
				t.Errorf("matchEcosystems(%v) = %v, want %v", tt.requested, names, tt.want)
			}
		})
	}
}

func TestFindSource(t *testing.T) {
	tests := []struct {
		name      string
		worktrees []git.Worktree
		want      string
	}{
		{
			"prefers main",
			[]git.Worktree{
				{Path: "/wt/feature", Branch: "refs/heads/feature"},
				{Path: "/wt/main", Branch: "refs/heads/main"},
			},
			"/wt/main",
		},
		{
			"prefers master",
			[]git.Worktree{
				{Path: "/wt/feature", Branch: "refs/heads/feature"},
				{Path: "/wt/master", Branch: "refs/heads/master"},
			},
			"/wt/master",
		},
		{
			"falls back to first worktree",
			[]git.Worktree{
				{Path: "/wt/one", Branch: "refs/heads/one"},
				{Path: "/wt/two", Branch: "refs/heads/two"},
			},
			"/wt/one",
		},
		{"no worktrees", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := findSource(tt.worktrees); got != tt.want {
				t.Errorf("findSource() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRunCreate_ParseError(t *testing.T) {
	err := RunCreate([]string{"--no-such-flag"})
	if err == nil {
		t.Fatal("expected error for unknown flag")
	}
}

func TestRunCreate_MissingBranch(t *testing.T) {
	err := RunCreate([]string{"--base", "main"})
	if err == nil {
		t.Fatal("expected error for missing --branch")
	}
	if !strings.Contains(err.Error(), "missing required flag: --branch") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunCreate_MissingBase(t *testing.T) {
	err := RunCreate([]string{"--branch", "feature/x"})
	if err == nil {
		t.Fatal("expected error for missing --base")
	}
	if !strings.Contains(err.Error(), "missing required flag: --base") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunCreate_RequiresBareRepo(t *testing.T) {
	dir := t.TempDir()
	err := RunCreate([]string{"--branch", "feature/x", "--base", "main", dir})
	if err == nil {
		t.Fatal("expected error for non-bare path")
	}
	if !strings.Contains(err.Error(), "create requires a bare repository") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunCreate_Success(t *testing.T) {
	bareRepo := setupBareRepo(t)

	var err error
	out := captureStdout(t, func() {
		err = RunCreate([]string{"--branch", "feature/x", "--base", "main", bareRepo})
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Worktree created:") {
		t.Errorf("expected 'Worktree created:' confirmation, got:\n%s", out)
	}
}

func TestRunCreate_CopyEnvFindsSourceWorktree(t *testing.T) {
	bareRepo := setupBareRepo(t)

	var err error
	captureStdout(t, func() {
		err = RunCreate([]string{"--branch", "feature/env", "--base", "main", "--copy-env", bareRepo})
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunCreate_EcosystemFlagLoadsConfig(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	bareRepo := setupBareRepo(t)

	var err error
	captureStdout(t, func() {
		err = RunCreate([]string{"--branch", "feature/eco", "--base", "main", "--ecosystems", "no-such-ecosystem", bareRepo})
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunCreate_WarnsOnConfigLoadFailure(t *testing.T) {
	xdgDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdgDir)
	if err := os.MkdirAll(filepath.Join(xdgDir, "sentei"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWriteFile(t, filepath.Join(xdgDir, "sentei", "config.yaml"), "ecosystems: [not valid yaml")
	bareRepo := setupBareRepo(t)

	var err error
	stderr := captureStderr(t, func() {
		captureStdout(t, func() {
			err = RunCreate([]string{"--branch", "feature/badcfg", "--base", "main", "--ecosystems", "pnpm", bareRepo})
		})
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stderr, "Warning: failed to load config") {
		t.Errorf("expected config-load warning on stderr, got:\n%s", stderr)
	}
}

func TestRunCreate_FailsForMissingBaseBranch(t *testing.T) {
	bareRepo := setupBareRepo(t)

	var err error
	captureStdout(t, func() {
		err = RunCreate([]string{"--branch", "feature/x", "--base", "no-such-base", bareRepo})
	})
	if err == nil {
		t.Fatal("expected error for missing base branch")
	}
	if !strings.Contains(err.Error(), "create completed with errors") {
		t.Errorf("unexpected error: %v", err)
	}
}
