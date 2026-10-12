package tui

import (
	"strings"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/cleanup"
	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/progress"
	"github.com/bonboncinnabon/sentei/internal/repo"
	"github.com/bonboncinnabon/sentei/internal/worktree"
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

func TestBuildRemovalPhases_PruneStaging(t *testing.T) {
	m := NewModel(nil, nil, "/repo")
	m.remove.run = removalRun{}
	m.remove.run.events = []progress.Event{
		{Phase: worktree.RemovalPhaseID, PhaseLabel: worktree.RemovalPhaseName, Step: "remove-0", StepLabel: "a", Status: progress.StepPending, Of: 2},
		{Phase: cleanupPhaseID, PhaseLabel: "Prune & cleanup", Step: pruneStepID, StepLabel: "Prune worktree metadata", Status: progress.StepPending, Of: 1},
		{Phase: cleanupPhaseID, PhaseLabel: "Prune & cleanup", Step: cleanupStepID, StepLabel: "Repository cleanup", Status: progress.StepPending, Of: 1},
		{Phase: worktree.RemovalPhaseID, PhaseLabel: worktree.RemovalPhaseName, Close: true},
		{Phase: cleanupPhaseID, PhaseLabel: "Prune & cleanup", Close: true},
	}

	phases := m.buildRemovalPhases()
	last := phases[len(phases)-1]
	if last.Name != "Prune & cleanup" || last.Total != 2 {
		t.Errorf("expected pending prune phase before removal completes, got %+v", last)
	}

	// Removal finished: prune phase becomes active work.
	m.remove.run.events = append(m.remove.run.events, progress.Event{Phase: worktree.RemovalPhaseID, Step: "remove-0", Status: progress.StepDone})
	phases = m.buildRemovalPhases()
	last = phases[len(phases)-1]
	if last.Total != 2 || last.Done != 0 {
		t.Errorf("expected active 0/2 prune phase after removal completes, got %+v", last)
	}

	pruneErr := error(nil)
	m.remove.run.pruneErr = &pruneErr
	m.remove.run.cleanupResult = &cleanup.Result{}
	m.remove.run.events = append(m.remove.run.events,
		progress.Event{Phase: cleanupPhaseID, Step: pruneStepID, Status: progress.StepDone},
		progress.Event{Phase: cleanupPhaseID, Step: cleanupStepID, Status: progress.StepDone},
	)
	phases = m.buildRemovalPhases()
	last = phases[len(phases)-1]
	if last.Done != 2 || last.Failed != 0 {
		t.Errorf("expected completed prune phase, got %+v", last)
	}
}

func TestViewSummary_VerdictMarkerAndNoEmptyCleanupHeader(t *testing.T) {
	m := NewModel(nil, nil, "/repo")
	m.width = 80
	m.remove.run = removalRun{}
	m.remove.run.result.SuccessCount = 2
	m.remove.run.cleanupResult = &cleanup.Result{NonWtBranchesRemaining: 1}
	m.view = summaryView

	view := stripANSI(m.viewSummary())
	if !strings.Contains(view, "✦ 2 worktrees removed successfully") {
		t.Errorf("expected ✦ verdict marker on the headline, view:\n%s", view)
	}
	if strings.Contains(view, "Cleanup:") {
		t.Errorf("Cleanup header must be omitted when nothing was cleaned, view:\n%s", view)
	}
	if !strings.Contains(view, "Tip: 1 local branch ") {
		t.Errorf("tip should render independently of the Cleanup header, view:\n%s", view)
	}
	if strings.Contains(view, " v ") {
		t.Errorf("legacy \"v\" marker must be gone, view:\n%s", view)
	}
}

func TestViewSummary_CleanupLinesPluralizeSingular(t *testing.T) {
	m := NewModel(nil, nil, "/repo")
	m.width = 80
	m.remove.run = removalRun{}
	m.remove.run.result.SuccessCount = 1
	m.remove.run.cleanupResult = &cleanup.Result{
		StaleRefsRemoved:    1,
		GoneBranchesDeleted: 1,
		ConfigOrphanResult:  cleanup.ConfigResult{Removed: 1},
	}
	m.view = summaryView

	view := stripANSI(m.viewSummary())

	for _, want := range []string{
		"1 worktree removed successfully",
		"Pruned 1 remote ref",
		"Deleted 1 branch with gone upstream",
		"Removed 1 orphaned config section",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("expected singular %q, view:\n%s", want, view)
		}
	}
	// Singular nouns are prefixes of their plurals, so guard against the naive
	// always-plural regression explicitly.
	for _, plural := range []string{"worktrees removed", "remote refs", "branches with gone", "config sections"} {
		if strings.Contains(view, plural) {
			t.Errorf("count of 1 must not read as plural %q, view:\n%s", plural, view)
		}
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
