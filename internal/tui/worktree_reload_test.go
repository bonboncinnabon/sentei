package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/repo"
	"github.com/bonboncinnabon/sentei/internal/worktree"
)

func loadingMenuModel() Model {
	m := NewMenuModel(nil, nil, "/repo", nil, repo.ContextBareRepo)
	m.width = 100
	return m
}

// A load that fails must end the loading state and say so. It used to be
// dropped, which left the menu on "loading" for the rest of the session.
func TestWorktreeLoad_FailureIsShownAndRetryable(t *testing.T) {
	m := loadingMenuModel()
	updated, _ := m.Update(worktreeContextMsg{err: errors.New("git worktree list: fatal: not a git repository"), generation: m.worktreeGeneration})
	m = updated.(Model)

	menu := stripAnsi(m.viewMenu())
	if strings.Contains(menu, "loading") {
		t.Errorf("menu still reads as loading after the load failed:\n%s", menu)
	}
	if !strings.Contains(menu, "could not load") {
		t.Errorf("menu does not report the failed load:\n%s", menu)
	}

	// The entry stays selectable: choosing it opens the list and tries again.
	m.menuCursor = 1
	generation := m.worktreeGeneration
	updated, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(Model)
	if m.view != listView || cmd == nil || m.worktreeGeneration == generation {
		t.Fatalf("enter on the failed entry: view=%d cmd=%v generation %d -> %d, want the list and a new load", m.view, cmd != nil, generation, m.worktreeGeneration)
	}

	// A second failure shows on the list itself, with the reason.
	updated, _ = m.Update(worktreeContextMsg{err: errors.New("git worktree list: fatal: not a git repository"), generation: m.worktreeGeneration})
	list := stripAnsi(updated.(Model).viewList())
	if strings.Contains(list, "Loading") || strings.Contains(list, "No worktrees found") {
		t.Errorf("list hides the failed load:\n%s", list)
	}
	if !strings.Contains(list, "Could not load worktrees") || !strings.Contains(list, "not a git repository") {
		t.Errorf("list does not explain the failed load:\n%s", list)
	}
}

// Git's own message comes last in the error chain, after the command that
// produced it, and this view has no details portal. The reason wraps so the
// part that says what went wrong is never the part that is cut.
func TestWorktreeLoad_FailureShowsTheWholeReason(t *testing.T) {
	reasons := []string{
		"listing worktrees: git worktree list --porcelain: fatal: unable to read /repo/.bare/worktrees/feature-x/gitdir: Permission denied\nhint: check the permissions of the bare repository",
		`not a git repository "/Users/someone/code/personal/some-project/with-a-long-name": git rev-parse --git-dir: fatal: not a git repository (or any of the parent directories): .git`,
	}
	unspaced := func(s string) string { return strings.Join(strings.Fields(s), "") }

	for _, reason := range reasons {
		for _, width := range []int{80, 44, 24} {
			m := loadingMenuModel()
			m.width = width
			m.view = listView
			updated, _ := m.Update(worktreeContextMsg{err: errors.New(reason), generation: m.worktreeGeneration})
			view := stripAnsi(updated.(Model).viewList())

			// Wrapping may break a line anywhere, so compare without whitespace.
			if !strings.Contains(unspaced(view), unspaced(reason)) {
				t.Errorf("list at %d columns cut the reason %q:\n%s", width, reason, view)
			}
			if over := overflowingLines(view, width); len(over) > 0 {
				t.Errorf("list at %d columns has rows wider than the terminal:\n%s", width, strings.Join(over, "\n"))
			}
		}
	}
}

func TestWorktreeLoad_SuccessAfterFailureClearsTheError(t *testing.T) {
	m := loadingMenuModel()
	updated, _ := m.Update(worktreeContextMsg{err: errors.New("boom"), generation: m.worktreeGeneration})
	m = updated.(Model)
	m.worktreeGeneration++
	updated, _ = m.Update(worktreeContextMsg{worktrees: []git.Worktree{{Path: "/repo/a", Branch: "refs/heads/a"}}, generation: m.worktreeGeneration})

	menu := stripAnsi(updated.(Model).viewMenu())
	if strings.Contains(menu, "could not load") || !strings.Contains(menu, "1 available") {
		t.Errorf("menu after a successful reload:\n%s", menu)
	}
}

// The reload after a removal can take seconds. The worktrees the run removed
// must not stay listed, and selectable, until it lands.
func TestRemovalComplete_DropsRemovedWorktreesBeforeTheReload(t *testing.T) {
	m := loadingMenuModel()
	m.remove.worktrees = []git.Worktree{
		{Path: "/repo/a", Branch: "refs/heads/a"},
		{Path: "/repo/b", Branch: "refs/heads/b"},
		{Path: "/repo/c", Branch: "refs/heads/c"},
	}
	m.reindex()
	m.view = progressView
	m.remove.run.result = worktree.DeletionResult{
		SuccessCount: 1, FailureCount: 1,
		Outcomes: []worktree.WorktreeOutcome{
			{Path: "/repo/a", Success: true},
			{Path: "/repo/b", Success: false, Error: errors.New("in use")},
		},
	}

	updated, _ := m.updateProgress(removalEventsCompleteMsg{})
	m = updated.(Model)

	var paths []string
	for _, wt := range m.remove.worktrees {
		paths = append(paths, wt.Path)
	}
	if got := strings.Join(paths, " "); got != "/repo/b /repo/c" {
		t.Errorf("worktrees after the run = %q, want the removed one gone and the failed one kept", got)
	}
	if menu := stripAnsi(m.viewMenu()); !strings.Contains(menu, "2 available") {
		t.Errorf("menu count was not updated with the removal:\n%s", menu)
	}
}

// A selection made on rows that a reload then drops would keep counting, and
// enter would start a removal of worktrees that no longer exist.
func TestWorktreeLoad_DropsSelectionsThatAreGone(t *testing.T) {
	m := loadingMenuModel()
	m.remove.worktrees = []git.Worktree{
		{Path: "/repo/a", Branch: "refs/heads/a"},
		{Path: "/repo/b", Branch: "refs/heads/b"},
	}
	m.remove.selected = map[string]bool{"/repo/a": true, "/repo/b": true}
	m.reindex()

	updated, _ := m.Update(worktreeContextMsg{
		worktrees:     []git.Worktree{{Path: "/repo/b", Branch: "refs/heads/b"}, {Path: "/repo/main", Branch: "refs/heads/main"}},
		defaultBranch: "main",
		generation:    m.worktreeGeneration,
	})
	m = updated.(Model)

	if len(m.remove.selected) != 1 || !m.remove.selected["/repo/b"] {
		t.Errorf("selection after the reload = %v, want only /repo/b", m.remove.selected)
	}
	if bar := stripAnsi(m.viewStatusBar()); !strings.Contains(bar, "1 selected") {
		t.Errorf("status bar = %q, want it to count 1 selected", bar)
	}
}
