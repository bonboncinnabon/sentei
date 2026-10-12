package cleanup

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/testutil/mock"
)

func collectEvents(t *testing.T) *mock.EventCollector[Event] {
	t.Helper()
	return &mock.EventCollector[Event]{}
}

// orchestratorMock answers every git call a clean safe run makes: main in a
// worktree, feature/old in none, an origin with nothing stale, and one config
// section for a branch that no longer exists.
func orchestratorMock() *mock.Runner {
	return &mock.Runner{Responses: map[string]mock.Response{
		"/repo:[rev-parse --git-dir]":                          {Output: "/repo/.bare"},
		"/repo:[remote]":                                       {Output: "origin"},
		"/repo:[remote prune origin --dry-run]":                {Output: ""},
		"/repo:[fetch --prune origin]":                         {Output: ""},
		"/repo:[symbolic-ref --short HEAD]":                    {Output: "main"},
		goneBranchCall:                                         {Output: refLine("main", "refs/remotes/origin/main", "", "/repo/main") + "\n" + refLine("feature/old", "", "", "")},
		"/repo:[worktree list --porcelain]":                    {Output: "worktree /repo/.bare\nbare\n\nworktree /repo/main\nHEAD abc\nbranch refs/heads/main"},
		"/repo:[branch --format=%(refname:lstrip=2)]":          {Output: "main\nfeature/old"},
		"/repo:[config --local --name-only --list]":            {Output: "core.bare\nbranch.main.remote\nbranch.ghost.remote\nbranch.ghost.merge"},
		"/repo:[config --local --remove-section branch.ghost]": {Output: ""},
	}}
}

func TestRun_SafeMode(t *testing.T) {
	runner := orchestratorMock()

	result := Run(runner, "/repo", Options{Mode: ModeSafe}, collectEvents(t).Emit)

	if len(result.NonWtBranchesDeleted) != 0 {
		t.Errorf("safe mode deleted non-worktree branches: %v", result.NonWtBranchesDeleted)
	}
	if result.NonWtBranchesRemaining != 1 {
		t.Errorf("NonWtBranchesRemaining = %d, want 1", result.NonWtBranchesRemaining)
	}
	if result.ConfigOrphanResult.Removed != 1 {
		t.Errorf("ConfigOrphanResult.Removed = %d, want 1", result.ConfigOrphanResult.Removed)
	}
	if len(result.Errors) > 0 {
		t.Errorf("unexpected errors: %v", result.Errors)
	}
}

func TestRun_AggressiveMode(t *testing.T) {
	runner := orchestratorMock()
	runner.Responses["/repo:[branch -d feature/old]"] = mock.Response{Output: "Deleted"}

	result := Run(runner, "/repo", Options{Mode: ModeAggressive}, collectEvents(t).Emit)

	if want := []string{"feature/old"}; !slices.Equal(result.NonWtBranchesDeleted, want) {
		t.Errorf("NonWtBranchesDeleted = %v, want %v", result.NonWtBranchesDeleted, want)
	}
}

func TestRun_DryRunIssuesNoMutatingCall(t *testing.T) {
	runner := orchestratorMock()
	runner.Responses["/repo:[remote prune origin --dry-run]"] = mock.Response{Output: " * [would prune] origin/gone"}

	result := Run(runner, "/repo", Options{Mode: ModeAggressive, Force: true, DryRun: true}, collectEvents(t).Emit)

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}
	assertNoMutatingCall(t, runner)
	if result.StaleRefsRemoved != 1 || len(result.NonWtBranchesDeleted) != 1 || result.ConfigOrphanResult.Removed != 1 {
		t.Errorf("a dry run must still report what a real run would do, got %+v", result)
	}
}

func assertNoMutatingCall(t *testing.T, runner *mock.Runner) {
	t.Helper()
	readOnly := []string{
		"/repo:[rev-parse ", "/repo:[remote]", "/repo:[remote prune origin --dry-run]", "/repo:[symbolic-ref ",
		"/repo:[for-each-ref ", "/repo:[worktree list ", "/repo:[branch --format=", "/repo:[config --local --name-only --list]",
		"/repo:[merge-base ", "/repo:[show-ref ",
	}
	for _, call := range runner.Calls {
		if !slices.ContainsFunc(readOnly, func(prefix string) bool { return strings.HasPrefix(call, prefix) }) {
			t.Errorf("issued a git call that is not known to be read-only: %s", call)
		}
	}
}

func TestRun_ErrorContinues(t *testing.T) {
	runner := orchestratorMock()
	runner.Responses["/repo:[remote prune origin --dry-run]"] = mock.Response{Err: fmt.Errorf("network error")}

	result := Run(runner, "/repo", Options{Mode: ModeSafe}, collectEvents(t).Emit)

	if stepError(result.Errors, "prune-refs") == nil {
		t.Errorf("expected a prune-refs error, got %v", result.Errors)
	}
	if result.ConfigOrphanResult.Removed != 1 {
		t.Error("the later steps must still run after prune-refs failed")
	}
}

func TestRun_NotARepository(t *testing.T) {
	runner := &mock.Runner{Responses: map[string]mock.Response{
		"/repo:[rev-parse --git-dir]": {Err: fmt.Errorf("fatal: not a git repository")},
	}}

	result := Run(runner, "/repo", Options{Mode: ModeSafe}, collectEvents(t).Emit)

	if len(result.Errors) != 1 || result.Errors[0].Step != "repository" {
		t.Errorf("Errors = %v, want one repository error", result.Errors)
	}
	if len(runner.Calls) != 1 {
		t.Errorf("no step may run outside a repository, calls: %v", runner.Calls)
	}
}

func TestCountPrunable_NoPrunable(t *testing.T) {
	porcelain := "worktree /repo\nbare\n\nworktree /repo/main\nHEAD abc123\nbranch refs/heads/main\n"
	if got := countPrunable(porcelain); got != 0 {
		t.Errorf("expected 0, got %d", got)
	}
}

func TestCountPrunable_OnePrunable(t *testing.T) {
	porcelain := "worktree /repo\nbare\n\nworktree /repo/stale\nHEAD def456\nbranch refs/heads/feat/stale\nprunable gitdir file points to non-existent location\n"
	if got := countPrunable(porcelain); got != 1 {
		t.Errorf("expected 1, got %d", got)
	}
}

func TestCountPrunable_MultiplePrunable(t *testing.T) {
	porcelain := "worktree /a\nHEAD a1\nprunable\n\nworktree /b\nHEAD b1\nprunable\n\nworktree /c\nHEAD c1\nbranch refs/heads/main\n"
	if got := countPrunable(porcelain); got != 2 {
		t.Errorf("expected 2, got %d", got)
	}
}
