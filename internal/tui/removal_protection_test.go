package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/testtmp"
	"github.com/bonboncinnabon/sentei/internal/worktree"
)

func runTestGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = testtmp.HermeticGitEnv()
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// bareRepoWithWorktrees creates a bare repo with one commit on main and one
// worktree per name (on branch feature/<name>), and returns the bare path.
func bareRepoWithWorktrees(t *testing.T, names ...string) string {
	t.Helper()
	root := t.TempDir()
	bare := filepath.Join(root, "test.git")
	clone := filepath.Join(root, "clone")
	runTestGit(t, root, "init", "--bare", "--initial-branch=main", bare)
	runTestGit(t, root, "clone", bare, clone)
	runTestGit(t, clone, "-c", "user.name=Test", "-c", "user.email=test@test.com", "commit", "--allow-empty", "-m", "initial commit")
	runTestGit(t, clone, "push", "origin", "main")
	for _, name := range names {
		runTestGit(t, bare, "worktree", "add", "-b", "feature/"+name, filepath.Join(bare, name), "main")
	}
	return bare
}

func unreadableStatusModel() (Model, string) {
	const path = "/repo/scratch"
	m := makeRemoveModel([]git.Worktree{{
		Path:            path,
		Branch:          "refs/heads/scratch",
		EnrichmentError: "git log -1: fatal: no commits yet",
	}})
	m.width = 120
	m.remove.selected[path] = true
	return m, path
}

func TestWorktreeAtRisk_UnreadableStatus(t *testing.T) {
	wt := git.Worktree{EnrichmentError: "git log -1: fatal: no commits yet"}
	if !worktreeAtRisk(wt) {
		t.Error("a worktree whose status could not be read must count as at risk")
	}
}

// Enter deletes clean selections without a confirmation stop. A worktree
// whose status could not be read has every risk flag false and used to take
// that path.
func TestList_EnterOnUnreadableStatusStopsAtConfirm(t *testing.T) {
	m, _ := unreadableStatusModel()
	m.view = listView

	updated, _ := m.updateList(tea.KeyPressMsg{Code: tea.KeyEnter})

	if got := updated.(Model).view; got != confirmView {
		t.Errorf("view after enter = %d, want confirmView (%d)", got, confirmView)
	}
}

func TestViewList_UnreadableStatusIsNotShownAsClean(t *testing.T) {
	m, _ := unreadableStatusModel()
	m.view = listView

	// The legend below the table always mentions [ok]; only the row matters.
	var row string
	for line := range strings.SplitSeq(stripANSI(m.viewList()), "\n") {
		if strings.Contains(line, "scratch") {
			row = line
		}
	}

	if !strings.Contains(row, "[?]") {
		t.Errorf("an unreadable status must be marked [?], got row %q", row)
	}
	if strings.Contains(row, "[ok]") {
		t.Errorf("an unreadable status must not read as [ok], got row %q", row)
	}
}

func TestViewConfirm_UnreadableStatusRowIsExplained(t *testing.T) {
	m, _ := unreadableStatusModel()
	m.view = confirmView

	view := stripANSI(m.viewConfirm())

	for _, want := range []string{"[?]", "status could not be read", "1 worktree whose status could not be read"} {
		if !strings.Contains(view, want) {
			t.Errorf("confirm view is missing %q:\n%s", want, view)
		}
	}
}

func TestList_LaunchDirectoryWorktreeCannotBeSelected(t *testing.T) {
	const launch, ordinary = "/repo/wt-alpha", "/repo/wt-beta"
	m := makeRemoveModel(sampleWorktrees())
	m.view = listView
	m.remove.launchedFrom = map[string]bool{launch: true}

	press := func(m Model, k string) Model {
		updated, _ := m.updateList(keyMsg(k))
		return updated.(Model)
	}
	cursorOn := func(m Model, path string) Model {
		for i, idx := range m.remove.visibleIndices {
			if m.remove.worktrees[idx].Path == path {
				m.remove.cursor = i
				return m
			}
		}
		t.Fatalf("worktree %s is not visible", path)
		return m
	}

	m = press(m, "a")
	if !m.remove.selected[ordinary] {
		t.Fatal("select-all should select an ordinary worktree")
	}
	if m.remove.selected[launch] {
		t.Error("select-all must skip the worktree sentei was launched from")
	}

	// Positive control: the same key toggles an ordinary row.
	m = press(cursorOn(m, ordinary), " ")
	if m.remove.selected[ordinary] {
		t.Fatal("space should toggle an ordinary worktree off")
	}
	m = press(cursorOn(m, launch), " ")
	if m.remove.selected[launch] {
		t.Error("space must not select the worktree sentei was launched from")
	}
}

func TestLoadWorktreeContext_MarksLaunchDirectoryWorktree(t *testing.T) {
	bare := bareRepoWithWorktrees(t, "here", "elsewhere")
	t.Chdir(filepath.Join(bare, "here"))

	msg, ok := loadWorktreeContext(&git.GitRunner{}, bare, 1)().(worktreeContextMsg)
	if !ok || msg.err != nil {
		t.Fatalf("unexpected load result: ok=%v err=%v", ok, msg.err)
	}

	if len(msg.worktrees) != 2 {
		t.Fatalf("loaded %d worktrees, want 2", len(msg.worktrees))
	}
	for _, wt := range msg.worktrees {
		want := filepath.Base(wt.Path) == "here"
		if got := msg.launchedFrom[wt.Path]; got != want {
			t.Errorf("launchedFrom[%s] = %v, want %v", wt.Path, got, want)
		}
	}
}

// The list's status is a snapshot. A row that looked clean is removed without
// --force, so git refuses a tree that gained files after the list loaded.
// Rows that were at risk went through the confirmation screen and are forced.
func TestWorktreeRemover_RechecksRowsThatLookedClean(t *testing.T) {
	bare := bareRepoWithWorktrees(t, "stale", "confirmed", "clean")
	path := func(name string) string { return filepath.Join(bare, name) }
	targets := []worktree.RemovalTarget{
		{Worktree: git.Worktree{Path: path("stale")}},
		{Worktree: git.Worktree{Path: path("confirmed"), HasUntrackedFiles: true}},
		{Worktree: git.Worktree{Path: path("clean")}},
	}
	// Files that appear after the snapshot was taken.
	newWork := filepath.Join(path("stale"), "new-work.txt")
	for _, file := range []string{newWork, filepath.Join(path("confirmed"), "draft.txt")} {
		if err := os.WriteFile(file, []byte("unsaved\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	remove := worktreeRemover(&git.GitRunner{}, bare, targets)

	if err := remove(path("stale")); err == nil {
		t.Error("a row that looked clean but now holds new files must not be removed")
	}
	if _, err := os.Stat(newWork); err != nil {
		t.Errorf("the new file must survive the refused removal: %v", err)
	}
	if err := remove(path("confirmed")); err != nil {
		t.Errorf("a confirmed at-risk row must be removed: %v", err)
	}
	if err := remove(path("clean")); err != nil {
		t.Errorf("a clean row must be removed: %v", err)
	}
	for _, name := range []string{"confirmed", "clean"} {
		if _, err := os.Stat(path(name)); !os.IsNotExist(err) {
			t.Errorf("worktree %s should be gone (stat err: %v)", name, err)
		}
	}
}
