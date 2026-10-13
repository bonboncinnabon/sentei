package tui

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/repo"
)

// realBareRepo is bareRepoWithWorktrees with symlinks resolved, so paths built
// from it equal the ones git reports (the macOS temp dir is behind a symlink).
func realBareRepo(t *testing.T, names ...string) string {
	t.Helper()
	bare, err := filepath.EvalSymlinks(bareRepoWithWorktrees(t, names...))
	if err != nil {
		t.Fatal(err)
	}
	return bare
}

// directRemoveModel builds the model the way `sentei remove --all` does: a
// menu model that has not loaded anything yet, sent straight to the list with
// a pre-selection. Motion is off so Init schedules no animation tick.
func directRemoveModel(t *testing.T, bare string, paths ...string) Model {
	t.Helper()
	t.Setenv("SENTEI_MOTION", "off")
	m := NewMenuModel(&git.GitRunner{}, nil, bare, nil, repo.ContextBareRepo)
	m.SetRemoveOpts(RemovePreSelection{Paths: paths, FilterLabel: "all"})
	return m
}

// `sentei remove --all|--merged|--stale` without --yes opens the list
// directly. The worktrees are loaded by Init, so the pre-selection has to
// wait for them instead of matching against an empty list.
func TestDirectRemove_LoadsWorktreesAndAppliesThePreSelection(t *testing.T) {
	bare := realBareRepo(t, "alpha", "beta")
	alpha, beta := filepath.Join(bare, "alpha"), filepath.Join(bare, "beta")
	m := directRemoveModel(t, bare, alpha)

	if view := stripAnsi(m.viewList()); strings.Contains(view, "No worktrees found") {
		t.Errorf("the list claims there are no worktrees before the load has landed:\n%s", view)
	}

	m = pumpCmds(m, m.Init()).(Model)

	if got := len(m.remove.worktrees); got != 2 {
		t.Fatalf("worktrees after Init = %d, want 2", got)
	}
	if !m.remove.selected[alpha] || m.remove.selected[beta] || len(m.remove.selected) != 1 {
		t.Errorf("selection after the load = %v, want only %s", m.remove.selected, alpha)
	}
	view := stripAnsi(m.viewList())
	for _, want := range []string{"feature/alpha", "feature/beta", "1 selected", "pre-filter: all"} {
		if !strings.Contains(view, want) {
			t.Errorf("list view is missing %q:\n%s", want, view)
		}
	}
}

// The pre-selection is a starting point: a reload after the first one must
// not select again what the user has since deselected.
func TestDirectRemove_PreSelectionAppliesOnce(t *testing.T) {
	bare := realBareRepo(t, "alpha")
	alpha := filepath.Join(bare, "alpha")
	m := directRemoveModel(t, bare, alpha)
	m = pumpCmds(m, m.Init()).(Model)

	delete(m.remove.selected, alpha)
	m.worktreeGeneration++
	m = pumpCmds(m, loadWorktreeContext(m.runner, m.repoPath, m.worktreeGeneration)).(Model)

	if m.remove.selected[alpha] {
		t.Error("a later reload selected the pre-selection again")
	}
}

// A pre-selected path never overrides protection: the default branch stays
// unselectable however the selection was made.
func TestDirectRemove_PreSelectionSkipsProtectedWorktrees(t *testing.T) {
	bare := realBareRepo(t, "alpha")
	trunk := filepath.Join(bare, "trunk")
	runTestGit(t, bare, "worktree", "add", trunk, "main")
	m := directRemoveModel(t, bare, trunk, filepath.Join(bare, "alpha"))
	m = pumpCmds(m, m.Init()).(Model)

	if m.remove.selected[trunk] {
		t.Error("the default-branch worktree was pre-selected for removal")
	}
	if len(m.remove.selected) != 1 {
		t.Errorf("selection = %v, want only the alpha worktree", m.remove.selected)
	}
}

// Leaving a direct-entry flow lands on the menu, which needs the same load.
func TestDirectRemove_EscapeReachesALoadedMenu(t *testing.T) {
	bare := realBareRepo(t, "alpha", "beta")
	m := directRemoveModel(t, bare)
	m = pumpCmds(m, m.Init()).(Model)

	updated, _ := m.Update(keyEsc())
	m = updated.(Model)
	if m.view != menuView {
		t.Fatalf("view after esc = %d, want the menu", m.view)
	}
	menu := stripAnsi(m.viewMenu())
	if strings.Contains(menu, "loading") || !strings.Contains(menu, "2 available") {
		t.Errorf("menu after leaving the direct list is not loaded:\n%s", menu)
	}
}
