package tui

import (
	"os/exec"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/testtmp"
)

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
	path := "/repo/scratch"
	m := makeRemoveModel([]git.Worktree{{
		Path:            path,
		Branch:          "refs/heads/scratch",
		EnrichmentError: "git log -1: fatal: no commits yet",
	}})
	m.view = listView
	m.remove.selected[path] = true

	updated, _ := m.updateList(tea.KeyPressMsg{Code: tea.KeyEnter})

	if got := updated.(Model).view; got != confirmView {
		t.Errorf("view after enter = %d, want confirmView (%d)", got, confirmView)
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
	root := t.TempDir()
	bare := filepath.Join(root, "test.git")
	clone := filepath.Join(root, "clone")
	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = testtmp.HermeticGitEnv()
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run(root, "init", "--bare", "--initial-branch=main", bare)
	run(root, "clone", bare, clone)
	run(clone, "-c", "user.name=Test", "-c", "user.email=test@test.com", "commit", "--allow-empty", "-m", "initial commit")
	run(clone, "push", "origin", "main")
	run(bare, "worktree", "add", "-b", "feature/here", filepath.Join(bare, "here"), "main")
	run(bare, "worktree", "add", "-b", "feature/elsewhere", filepath.Join(bare, "elsewhere"), "main")
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
