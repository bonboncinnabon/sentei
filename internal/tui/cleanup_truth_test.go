package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/cleanup"
	"github.com/bonboncinnabon/sentei/internal/config"
	"github.com/bonboncinnabon/sentei/internal/repo"
)

func cleanupModelWithScan(scan *cleanup.DryRunResult) Model {
	m := NewMenuModel(bareDirRunner("/repo"), nil, "/repo", &config.Config{}, repo.ContextBareRepo)
	m.view = cleanupPreviewView
	m.width = 80
	m.height = 24
	m.cleanupScan = scan
	return m
}

func TestCleanupPreview_HeadlineStatesEffectiveCount(t *testing.T) {
	mixed := cleanupModelWithScan(&cleanup.DryRunResult{AggressiveBranches: []cleanup.BranchInfo{
		{Name: "a", Merged: true}, {Name: "b", Merged: false}, {Name: "c", Merged: false},
	}})
	view := stripAnsi(mixed.viewCleanupPreview())
	if !strings.Contains(view, "1 of 3 branches") {
		t.Errorf("mixed headline must state the effective count:\n%s", view)
	}

	none := cleanupModelWithScan(&cleanup.DryRunResult{AggressiveBranches: []cleanup.BranchInfo{
		{Name: "a", Merged: false}, {Name: "b", Merged: false},
	}})
	view = stripAnsi(none.viewCleanupPreview())
	if !strings.Contains(view, "none deletable without --force") {
		t.Errorf("all-unmerged headline must say none deletable:\n%s", view)
	}
	if strings.Contains(view, "would be deleted") {
		t.Errorf("all-unmerged headline must not promise deletions:\n%s", view)
	}
	if strings.Contains(view, "a aggressive") {
		t.Errorf("aggressive hint must vanish when nothing is deletable:\n%s", view)
	}
}

func TestCleanupPreview_AggressiveGateNeedsDeletable(t *testing.T) {
	none := cleanupModelWithScan(&cleanup.DryRunResult{AggressiveBranches: []cleanup.BranchInfo{
		{Name: "a", Merged: false},
	}})
	updated, _ := none.updateCleanupPreview(keyMsg("a"))
	if updated.(Model).cleanupAggressiveConfirm {
		t.Error("the aggressive confirm must not open when nothing is deletable")
	}

	some := cleanupModelWithScan(&cleanup.DryRunResult{AggressiveBranches: []cleanup.BranchInfo{
		{Name: "a", Merged: true},
	}})
	updated, _ = some.updateCleanupPreview(keyMsg("a"))
	if !updated.(Model).cleanupAggressiveConfirm {
		t.Error("the aggressive confirm must open when deletions are possible")
	}
}

func TestCleanupPreview_CleanStateShowsChecks(t *testing.T) {
	m := cleanupModelWithScan(&cleanup.DryRunResult{})
	view := stripAnsi(m.viewCleanupPreview())
	for _, check := range []string{"No stale remote refs", "No branches with gone upstream", "No stale worktrees"} {
		if !strings.Contains(view, check) {
			t.Errorf("clean preview must show what was checked (%q):\n%s", check, view)
		}
	}
}

func TestCleanupPreview_ScanningHasChrome(t *testing.T) {
	m := cleanupModelWithScan(nil)
	view := stripAnsi(m.viewCleanupPreview())
	if !strings.Contains(view, "q quit") {
		t.Errorf("scanning view must keep an escape affordance:\n%s", view)
	}
}

func TestCleanupResult_SkipsNeverReadClean(t *testing.T) {
	m := NewModel(nil, nil, "/repo")
	m.width = 80
	m.cleanupRanMode = cleanup.ModeAggressive
	m.cleanupResult = &cleanup.Result{BranchesSkipped: []cleanup.SkippedBranch{{Name: "a"}, {Name: "b"}}}

	view := stripAnsi(m.viewCleanupResult())
	if strings.Contains(view, "Repository is clean") {
		t.Errorf("a run that skipped branches must not claim clean:\n%s", view)
	}
	if !strings.Contains(view, "2 branches remain") {
		t.Errorf("headline must state what remains:\n%s", view)
	}
	if !strings.Contains(view, "ran: sentei cleanup") {
		t.Errorf("the command echo must be labeled:\n%s", view)
	}
}

func TestCleanupResult_CheckOrderMatchesPreview(t *testing.T) {
	m := NewModel(nil, nil, "/repo")
	m.width = 80
	m.cleanupRanMode = cleanup.ModeSafe
	m.cleanupResult = &cleanup.Result{}

	view := stripAnsi(m.viewCleanupResult())
	refs := strings.Index(view, "stale remote refs")
	gone := strings.Index(view, "gone upstream")
	dup := strings.Index(view, "config duplicates")
	if refs >= gone || gone >= dup {
		t.Errorf("result check order must match the preview (refs, gone, duplicates):\n%s", view)
	}
}

var _ = tea.KeyPressMsg{}
