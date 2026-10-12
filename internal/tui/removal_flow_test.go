package tui

import (
	"errors"
	"os"
	"path/filepath"
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
// the repository: the worktree reload has landed and the run is recorded.
type removalToSummary struct {
	model    Model
	recorded bool
}

func (r removalToSummary) Init() tea.Cmd { return nil }

func (r removalToSummary) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(milestoneMsg); ok {
		r.recorded = true
	}
	updated, cmd := r.model.Update(msg)
	r.model = updated.(Model)
	if r.model.view == summaryView && r.recorded && !r.model.worktreesLoading() {
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
