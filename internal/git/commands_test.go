package git

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/testutil/mock"
)

func TestRunShell_EmptyStderr_PreservesExitError(t *testing.T) {
	// `exit 7` fails with no stderr, exercising the empty-stderr %w fallback.
	_, err := (&DefaultShellRunner{}).RunShell(t.TempDir(), "exit 7")
	if err == nil {
		t.Fatal("expected an error from a non-zero exit")
	}
	if errors.Unwrap(err) == nil {
		t.Errorf("underlying exit error must be preserved when stderr is empty, got: %v", err)
	}
}

func TestGitRunner_Run_FailureNamesCommand(t *testing.T) {
	// A bad git invocation fails with stderr; the message must name the command
	// (exercises the non-empty-stderr branch of the real runner).
	_, err := (&GitRunner{}).Run(t.TempDir(), "rev-parse", "--definitely-not-a-flag")
	if err == nil {
		t.Fatal("expected an error from a bad git invocation")
	}
	if !strings.Contains(err.Error(), "git rev-parse") {
		t.Errorf("error should name the command, got: %v", err)
	}
}

func TestListWorktrees_Success(t *testing.T) {
	runner := &mock.Runner{
		Responses: map[string]mock.Response{
			"/repo:[rev-parse --git-dir]": {Output: "."},
			"/repo:[worktree list --porcelain]": {
				Output: "worktree /repo\nbare\n\nworktree /repo/main\nHEAD abc123\nbranch refs/heads/main",
			},
		},
	}

	wts, err := ListWorktrees(runner, "/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(wts) != 2 {
		t.Fatalf("got %d worktrees, want 2", len(wts))
	}
	if !wts[0].IsBare {
		t.Error("first worktree should be bare")
	}
	if wts[1].Branch != "refs/heads/main" {
		t.Errorf("second worktree branch = %q, want refs/heads/main", wts[1].Branch)
	}
}

func TestListWorktrees_RepoValidationFailure(t *testing.T) {
	runner := &mock.Runner{
		Responses: map[string]mock.Response{
			"/not-a-repo:[rev-parse --git-dir]": {Err: fmt.Errorf("git rev-parse --git-dir: fatal: not a git repository")},
		},
	}

	wts, err := ListWorktrees(runner, "/not-a-repo")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if wts != nil {
		t.Errorf("expected nil worktrees, got %v", wts)
	}
}

func TestListWorktrees_GitCommandFailure(t *testing.T) {
	runner := &mock.Runner{
		Responses: map[string]mock.Response{
			"/repo:[rev-parse --git-dir]":       {Output: "."},
			"/repo:[worktree list --porcelain]": {Err: fmt.Errorf("git worktree list --porcelain: permission denied")},
		},
	}

	_, err := ListWorktrees(runner, "/repo")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestListWorktrees_EmptyOutput(t *testing.T) {
	runner := &mock.Runner{
		Responses: map[string]mock.Response{
			"/repo:[rev-parse --git-dir]":       {Output: "."},
			"/repo:[worktree list --porcelain]": {Output: ""},
		},
	}

	wts, err := ListWorktrees(runner, "/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(wts) != 0 {
		t.Fatalf("got %d worktrees, want 0", len(wts))
	}
}

func TestValidateRepository_NotARepo(t *testing.T) {
	runner := &mock.Runner{
		Responses: map[string]mock.Response{
			"/bad:[rev-parse --git-dir]": {Err: fmt.Errorf("fatal: not a git repository")},
		},
	}

	err := ValidateRepository(runner, "/bad")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestValidateRepository_PreservesUnderlyingCause(t *testing.T) {
	cause := errors.New("exec: \"git\": executable file not found in $PATH")
	runner := &mock.Runner{
		Responses: map[string]mock.Response{
			"/x:[rev-parse --git-dir]": {Err: cause},
		},
	}

	err := ValidateRepository(runner, "/x")
	if err == nil {
		t.Fatal("expected error")
	}
	// The real cause (git missing) must survive instead of being asserted away.
	if !errors.Is(err, cause) {
		t.Errorf("underlying cause not preserved: %v", err)
	}
}

func TestValidateRepository_PathDoesNotExist(t *testing.T) {
	runner := &mock.Runner{
		Responses: map[string]mock.Response{
			"/nonexistent:[rev-parse --git-dir]": {Err: fmt.Errorf("cannot change to '/nonexistent': No such file or directory")},
		},
	}

	err := ValidateRepository(runner, "/nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestBranchExists(t *testing.T) {
	tests := []struct {
		name   string
		branch string
		err    error
		want   bool
	}{
		{name: "existing branch", branch: "main", want: true},
		{name: "missing branch", branch: "ghost", err: fmt.Errorf("not found"), want: false},
		{name: "slash branch", branch: "feature/auth", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &mock.Runner{Responses: map[string]mock.Response{
				fmt.Sprintf("/repo:[show-ref --verify refs/heads/%s]", tt.branch): {Output: "abc123", Err: tt.err},
			}}
			if got := BranchExists(runner, "/repo", tt.branch); got != tt.want {
				t.Errorf("BranchExists(%q) = %v, want %v", tt.branch, got, tt.want)
			}
		})
	}
}

func TestShellQuote(t *testing.T) {
	cases := map[string]string{
		"/plain/path": "'/plain/path'",
		"/with space": "'/with space'",
		"a&&rm -rf x": "'a&&rm -rf x'",
		"it's":        `'it'\''s'`,
	}
	for in, want := range cases {
		if got := ShellQuote(in); got != want {
			t.Errorf("ShellQuote(%q) = %q, want %q", in, got, want)
		}
	}
}
