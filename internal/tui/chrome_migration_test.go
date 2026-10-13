package tui

import (
	"strings"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/repo"
)

func TestViewProgress_NoPurpleBadge_HasChromeAndBar(t *testing.T) {
	m := NewModel([]git.Worktree{{Path: "/work/a", Branch: "refs/heads/a"}}, nil, "/repo")
	m.width, m.height = 80, 30
	m.remove.run = removalRun{}
	m.view = progressView

	view := stripANSI(m.viewProgress())
	if !strings.Contains(view, "sentei ─ Removing worktrees") {
		t.Errorf("expected standard title, view:\n%s", view)
	}
	if !strings.Contains(view, "░") {
		t.Errorf("expected overall progress bar, view:\n%s", view)
	}
	if !strings.Contains(view, "q quit") {
		t.Errorf("expected key hints, view:\n%s", view)
	}
}

func TestViewSummary_VerdictMarker(t *testing.T) {
	m := NewModel(nil, nil, "/repo")
	m.width = 80
	m.remove.run = removalRun{}
	m.remove.run.result.SuccessCount = 2
	m.view = summaryView

	view := stripANSI(m.viewSummary())
	if !strings.Contains(view, "✦ 2 worktrees removed successfully") {
		t.Errorf("expected ✦ verdict marker on the headline, view:\n%s", view)
	}
	if strings.Contains(view, " v ") {
		t.Errorf("legacy \"v\" marker must be gone, view:\n%s", view)
	}
}

func TestViewSummary_HeadlinePluralizesSingular(t *testing.T) {
	m := NewModel(nil, nil, "/repo")
	m.width = 80
	m.remove.run = removalRun{}
	m.remove.run.result.SuccessCount = 1
	m.view = summaryView

	view := stripANSI(m.viewSummary())
	if !strings.Contains(view, "1 worktree removed successfully") {
		t.Errorf("a count of 1 must read as singular, view:\n%s", view)
	}
}

func TestViewCleanupResult_RunningTitleWhileNilResult(t *testing.T) {
	m := NewMenuModel(nil, nil, "/repo", nil, repo.ContextBareRepo)
	m.width = 80
	m.view = cleanupResultView

	view := stripANSI(m.viewCleanupResult())
	if !strings.Contains(view, "sentei ─ Running cleanup") {
		t.Errorf("expected Running cleanup title while result is nil, view:\n%s", view)
	}
	if strings.Contains(view, "Cleanup complete") {
		t.Errorf("must not claim completion while running, view:\n%s", view)
	}
}

func TestViewConfirm_NoBorderAndDetachedHeadLabel(t *testing.T) {
	wts := []git.Worktree{{Path: "/work/detached-head", Branch: ""}}
	m := NewModel(wts, nil, "/repo")
	m.width = 80
	m.remove.selected = map[string]bool{"/work/detached-head": true}
	m.view = confirmView

	view := stripANSI(m.viewConfirm())
	if strings.Contains(view, "╭") {
		t.Errorf("confirmation must not render a border box, view:\n%s", view)
	}
	if !strings.Contains(view, "detached-head") {
		t.Errorf("detached HEAD worktree must show its directory name, view:\n%s", view)
	}
	if !strings.Contains(view, "y delete · n go back") {
		t.Errorf("expected standard hints, view:\n%s", view)
	}
}

func TestConfirmationViewModel_NoBorderStandardHints(t *testing.T) {
	vm := ConfirmationViewModel{
		Title:      "Confirm cleanup",
		Items:      []ConfirmationItem{{Label: "Mode:", Value: "safe"}},
		CLICommand: "sentei cleanup --mode safe",
	}
	view := stripANSI(vm.View())
	if strings.Contains(view, "╭") {
		t.Errorf("confirmation view must not render a border, view:\n%s", view)
	}
	if !strings.Contains(view, "enter confirm · esc back · q quit") {
		t.Errorf("expected `·`-separated hints, view:\n%s", view)
	}
	if strings.Contains(view, "•") {
		t.Errorf("bullet separators must be gone, view:\n%s", view)
	}
}

func TestViewList_HasRepoSubtitleAndRules(t *testing.T) {
	m := NewModel([]git.Worktree{{Path: "/work/a", Branch: "refs/heads/a"}}, nil, "/repo/myrepo.git")
	m.width = 80
	m.view = listView

	view := stripANSI(m.viewList())
	if !strings.Contains(view, "myrepo.git (bare)") {
		t.Errorf("expected repo subtitle framing, view:\n%s", view)
	}
	if !strings.Contains(view, "┄") {
		t.Errorf("expected rule framing, view:\n%s", view)
	}
}
