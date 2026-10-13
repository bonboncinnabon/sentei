package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/config"
	"github.com/bonboncinnabon/sentei/internal/creator"
	"github.com/bonboncinnabon/sentei/internal/progress"
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
	if !strings.Contains(err.Error(), "no worktree was created") {
		t.Errorf("unexpected error: %v", err)
	}
}

// The error is the command's verdict: it must not say "completed" when
// nothing was created, and must point at the worktree when one exists.
func TestCreateResultErrorSaysWhetherAWorktreeExists(t *testing.T) {
	failed := []progress.Phase{{Name: "Setup", Steps: []progress.StepResult{{Status: progress.StepFailed, Error: errors.New("boom")}}}}

	notCreated := createResultError(creator.Result{Phases: failed})
	if notCreated == nil || !strings.Contains(notCreated.Error(), "no worktree was created") {
		t.Errorf("without a worktree path: %v, want an error saying nothing was created", notCreated)
	}
	created := createResultError(creator.Result{WorktreePath: "/repo/feature-x", Phases: failed})
	if created == nil || !strings.Contains(created.Error(), "the worktree exists at /repo/feature-x") {
		t.Errorf("with a worktree path: %v, want an error naming the worktree", created)
	}
	if err := createResultError(creator.Result{WorktreePath: "/repo/feature-x"}); err != nil {
		t.Errorf("a clean run returned %v", err)
	}
}

// --base is ignored for a branch that already exists. The output has to say
// so, or the caller believes the worktree starts from the base.
func TestRunCreate_ExistingBranchSaysBaseWasNotApplied(t *testing.T) {
	bareRepo := setupBareRepo(t)
	mustGit(t, bareRepo, "branch", "feature/existing", "main")

	var err error
	out := captureStdout(t, func() {
		err = RunCreate([]string{"--branch", "feature/existing", "--base", "main", bareRepo})
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "checked out existing branch feature/existing; base main was not applied") {
		t.Errorf("output does not say the existing branch was used:\n%s", out)
	}
}

// The header is printed before git says whether the branch already exists, so
// it must not promise the base: for an existing branch, local or only on
// origin, "from main" was false. The create step says what happened.
func TestRunCreate_HeaderDoesNotClaimTheBase(t *testing.T) {
	bareRepo := setupBareRepo(t)
	seed := filepath.Join(filepath.Dir(bareRepo), "clone")
	mustGit(t, bareRepo, "branch", "feature/local", "main")
	mustGit(t, seed, "branch", "feature/remote", "main")
	mustGit(t, bareRepo, "remote", "add", "origin", seed)
	mustGit(t, bareRepo, "fetch", "-q", "origin")

	for _, branch := range []string{"feature/new", "feature/local", "feature/remote"} {
		var err error
		out := captureStdout(t, func() {
			err = RunCreate([]string{"--branch", branch, "--base", "main", bareRepo})
		})
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", branch, err)
		}
		header, _, _ := strings.Cut(out, "\n")
		if want := fmt.Sprintf("Creating worktree %q...", branch); header != want {
			t.Errorf("%s: header = %q, want %q", branch, header, want)
		}
		if existed := branch != "feature/new"; strings.Contains(out, "base main was not applied") != existed {
			t.Errorf("%s: output must say the base was not applied exactly when the branch existed:\n%s", branch, out)
		}
	}
}
