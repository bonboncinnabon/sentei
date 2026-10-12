package tui

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/repo"
)

// recordingRunner runs real git, records every call and fails the calls
// whose arguments start with a key of fail.
type recordingRunner struct {
	git.GitRunner
	fail map[string]error

	mu    sync.Mutex
	calls []string
}

func (r *recordingRunner) Run(dir string, args ...string) (string, error) {
	call := strings.Join(args, " ")
	r.mu.Lock()
	r.calls = append(r.calls, call)
	r.mu.Unlock()
	for prefix, err := range r.fail {
		if strings.HasPrefix(call, prefix) {
			return "", err
		}
	}
	return r.GitRunner.Run(dir, args...)
}

func (r *recordingRunner) called(prefix string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, call := range r.calls {
		if strings.HasPrefix(call, prefix) {
			return true
		}
	}
	return false
}

// loadedListModel is a menu model on the list view with the repository's
// worktrees loaded through the real load, motion off.
func loadedListModel(t *testing.T, runner git.CommandRunner, bare string) Model {
	t.Helper()
	t.Setenv("SENTEI_MOTION", "off")
	m := NewMenuModel(runner, nil, bare, nil, repo.ContextBareRepo)
	m.view = listView
	m = pumpCmds(m, m.Init()).(Model)
	if len(m.remove.worktrees) == 0 {
		t.Fatal("no worktrees loaded")
	}
	return m
}

// removalToSummary wraps a Model for a real Bubble Tea program and quits once
// the removal has reached its summary and nothing is still running against
// the repository: the worktree reload has landed.
type removalToSummary struct{ model Model }

func (r removalToSummary) Init() tea.Cmd { return nil }

func (r removalToSummary) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	updated, cmd := r.model.Update(msg)
	r.model = updated.(Model)
	if r.model.view == summaryView && !r.model.worktreesLoading() {
		return r, tea.Quit
	}
	return r, cmd
}

func (r removalToSummary) View() tea.View { return r.model.View() }

// runRemoval selects the worktrees at paths, confirms their removal in a real
// Bubble Tea program and returns the model as it stands on the summary.
func runRemoval(t *testing.T, m Model, paths ...string) Model {
	t.Helper()
	for _, path := range paths {
		m.remove.selected[path] = true
	}
	m.view = confirmView
	tm := teatest.NewTestModel(t, removalToSummary{model: m}, teatest.WithInitialTermSize(100, 30))
	tm.Send(keyRune('y'))
	return tm.FinalModel(t, teatest.WithFinalTimeout(testTimeout)).(removalToSummary).model
}

// After the worktrees are gone the TUI used to run a full safe cleanup: two
// network commands against the remote, deletion of branches whose upstream
// is gone and config rewrites. A remote that cannot be reached made a
// successful removal read as failed.
func TestRemoval_PostStepIsLocalPruneOnly(t *testing.T) {
	bare := realBareRepo(t, "alpha", "gone")
	alpha := filepath.Join(bare, "alpha")
	// A branch whose upstream is gone, which safe cleanup would delete.
	runTestGit(t, bare, "worktree", "remove", "--force", filepath.Join(bare, "gone"))
	runTestGit(t, bare, "config", "branch.feature/gone.remote", "origin")
	runTestGit(t, bare, "config", "branch.feature/gone.merge", "refs/heads/feature/gone")
	// A remote that fails every network command.
	runTestGit(t, bare, "remote", "add", "origin", filepath.Join(bare, "no-such-remote"))

	runner := &recordingRunner{}
	m := runRemoval(t, loadedListModel(t, runner, bare), alpha)

	summary := stripAnsi(m.viewSummary())
	if m.remove.run.result.HasFailures() || !strings.Contains(summary, "1 worktree removed successfully") {
		t.Errorf("a removal that succeeded does not read as a success:\n%s", summary)
	}
	if _, err := os.Stat(alpha); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("worktree directory still exists (stat error: %v)", err)
	}
	for _, forbidden := range []string{"fetch", "remote prune", "branch"} {
		if runner.called(forbidden) {
			t.Errorf("the removal ran `git %s ...`; the post-removal step must stay local and leave branches alone", forbidden)
		}
	}
	if !runner.called("worktree prune") {
		t.Error("the removal did not prune worktree metadata")
	}
	if _, err := (&git.GitRunner{}).Run(bare, "show-ref", "--verify", "refs/heads/feature/gone"); err != nil {
		t.Errorf("a branch with a gone upstream was deleted by a worktree removal: %v", err)
	}
}

func TestRemoval_PruneFailureIsAWarningNotAFailedRemoval(t *testing.T) {
	bare := realBareRepo(t, "alpha")
	runner := &recordingRunner{fail: map[string]error{"worktree prune": errors.New("git worktree prune: permission denied")}}
	m := runRemoval(t, loadedListModel(t, runner, bare), filepath.Join(bare, "alpha"))

	summary := stripAnsi(m.viewSummary())
	if m.remove.run.result.HasFailures() || !strings.Contains(summary, "1 worktree removed successfully") {
		t.Errorf("a failed prune turned a successful removal into a failure:\n%s", summary)
	}
	for _, want := range []string{"permission denied", "git worktree prune"} {
		if !strings.Contains(summary, want) {
			t.Errorf("summary does not carry the prune warning (%q):\n%s", want, summary)
		}
	}
}

// A removal changes the repository root by exactly the worktrees it removed.
// It used to leave a sentei.json there to count lifetime removals.
func TestRemoval_LeavesNothingBehindInTheRepositoryRoot(t *testing.T) {
	bare := realBareRepo(t, "alpha", "beta")
	before := dirNames(t, bare)

	runRemoval(t, loadedListModel(t, &recordingRunner{}, bare), filepath.Join(bare, "alpha"))

	want := slices.DeleteFunc(before, func(name string) bool { return name == "alpha" })
	if got := dirNames(t, bare); !slices.Equal(got, want) {
		t.Errorf("repository root after removing alpha = %v, want %v", got, want)
	}
}

func dirNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, len(entries))
	for i, entry := range entries {
		names[i] = entry.Name()
	}
	return names
}

// lockReason returns the lock reason git reports for the worktree at path,
// and whether it is locked at all.
func lockReason(t *testing.T, bare, path string) (string, bool) {
	t.Helper()
	wts, err := git.ListWorktrees(&git.GitRunner{}, bare)
	if err != nil {
		t.Fatal(err)
	}
	for _, wt := range wts {
		if wt.Path == path {
			return wt.LockReason, wt.IsLocked
		}
	}
	t.Fatalf("worktree %s is not listed", path)
	return "", false
}

// A locked worktree used to be unlocked first, by a blocking git call inside
// Update, and stayed unlocked when its removal then failed or never started.
func TestRemoval_FailedRemovalLeavesTheLockInPlace(t *testing.T) {
	bare := realBareRepo(t, "alpha")
	alpha := filepath.Join(bare, "alpha")
	runTestGit(t, bare, "worktree", "lock", "--reason", "on a USB disk", alpha)

	runner := &recordingRunner{fail: map[string]error{"worktree remove": errors.New("git worktree remove: device busy")}}
	m := runRemoval(t, loadedListModel(t, runner, bare), alpha)

	if !m.remove.run.result.HasFailures() {
		t.Fatal("the removal was meant to fail")
	}
	if reason, locked := lockReason(t, bare, alpha); !locked || reason != "on a USB disk" {
		t.Errorf("after a failed removal: locked=%v reason=%q, want the lock and its reason intact", locked, reason)
	}
	if runner.called("worktree unlock") {
		t.Error("the removal unlocked the worktree")
	}
}

func TestRemoval_LockedWorktreeIsRemoved(t *testing.T) {
	bare := realBareRepo(t, "alpha")
	alpha := filepath.Join(bare, "alpha")
	runTestGit(t, bare, "worktree", "lock", alpha)

	m := runRemoval(t, loadedListModel(t, &recordingRunner{}, bare), alpha)

	if summary := stripAnsi(m.viewSummary()); m.remove.run.result.HasFailures() {
		t.Fatalf("removing a confirmed locked worktree failed:\n%s", summary)
	}
	if _, err := os.Stat(alpha); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("locked worktree directory still exists (stat error: %v)", err)
	}
	wts, err := git.ListWorktrees(&git.GitRunner{}, bare)
	if err != nil {
		t.Fatal(err)
	}
	for _, wt := range wts {
		if wt.Path == alpha {
			t.Error("git still lists the locked worktree after its removal")
		}
	}
}

// Only a lock the user saw on the confirm screen is overridden. A worktree
// locked after the list loaded is refused by git and stays where it is.
func TestRemoval_LockAddedAfterTheListLoadedIsRespected(t *testing.T) {
	bare := realBareRepo(t, "alpha")
	alpha := filepath.Join(bare, "alpha")
	m := loadedListModel(t, &recordingRunner{}, bare)
	runTestGit(t, bare, "worktree", "lock", alpha)

	m = runRemoval(t, m, alpha)

	if !m.remove.run.result.HasFailures() {
		t.Error("a worktree locked after the list loaded was removed")
	}
	if _, err := os.Stat(alpha); err != nil {
		t.Errorf("worktree locked after the list loaded is gone: %v", err)
	}
}
