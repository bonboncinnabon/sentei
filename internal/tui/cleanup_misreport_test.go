package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/cleanup"
)

func resultModel(opts *cleanup.Options, result cleanup.Result) Model {
	m := previewModel()
	m.view = cleanupResultView
	m.cleanupOpts = opts
	m.cleanupResult = &result
	return m
}

// An aggressive run deletes branches for good; the result has to say so.
func TestViewCleanupResult_ListsAggressiveDeletions(t *testing.T) {
	m := resultModel(&cleanup.Options{Mode: cleanup.ModeAggressive}, cleanup.Result{
		NonWtBranchesDeleted: []string{"old/one", "old/two"},
	})

	view := stripANSI(m.viewCleanupResult())
	for _, want := range []string{"Deleted 2 branches not in any worktree", "old/one", "old/two"} {
		if !strings.Contains(view, want) {
			t.Errorf("missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "Repository is clean") {
		t.Errorf("a run that deleted branches must not read as clean:\n%s", view)
	}
}

// The names must not push the footer off a short terminal.
func TestViewCleanupResult_DeletedNamesFitTheTerminal(t *testing.T) {
	var names []string
	for i := range 40 {
		names = append(names, "old/branch-"+string(rune('a'+i%26))+string(rune('a'+i/26)))
	}
	m := resultModel(&cleanup.Options{Mode: cleanup.ModeAggressive}, cleanup.Result{NonWtBranchesDeleted: names})
	m.windowHeight = 24

	view := stripANSI(m.viewCleanupResult())
	if rows := strings.Count(view, "\n"); rows > 24 {
		t.Errorf("view is %d rows on a 24-row terminal:\n%s", rows, view)
	}
	for _, want := range []string{"Deleted 40 branches", "more", "enter quit"} {
		if !strings.Contains(view, want) {
			t.Errorf("missing %q:\n%s", want, view)
		}
	}
}

func TestViewCleanupResult_DryRunSaysWouldNotDid(t *testing.T) {
	m := resultModel(&cleanup.Options{Mode: cleanup.ModeAggressive, DryRun: true}, cleanup.Result{
		StaleRefsRemoved:     2,
		GoneBranchesDeleted:  1,
		ConfigOrphanResult:   cleanup.ConfigResult{Removed: 1},
		WorktreesPruned:      1,
		NonWtBranchesDeleted: []string{"old/one"},
	})

	view := stripANSI(m.viewCleanupResult())
	for _, done := range []string{"Cleanup complete", "Pruned", "Deleted", "Removed", indicatorDone} {
		if strings.Contains(view, done) {
			t.Errorf("a dry run changed nothing, but the view says %q:\n%s", done, view)
		}
	}
	for _, want := range []string{
		"nothing was changed",
		"2 stale remote refs would be pruned",
		"1 branch with gone upstream would be deleted",
		"1 orphaned config section would be removed",
		"1 stale worktree would be pruned",
		"1 branch not in any worktree would be deleted",
		"old/one",
		"ran: sentei cleanup --dry-run --mode aggressive",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("missing %q:\n%s", want, view)
		}
	}
}

func TestCleanupConfirm_ShowsForce(t *testing.T) {
	forced := makeCleanupConfirmModel(&cleanup.Options{Mode: cleanup.ModeAggressive, Force: true})
	view := stripANSI(forced.viewCleanupConfirm())
	for _, want := range []string{"Force:", "unmerged", "sentei cleanup --force --mode aggressive"} {
		if !strings.Contains(view, want) {
			t.Errorf("missing %q:\n%s", want, view)
		}
	}

	plain := makeCleanupConfirmModel(&cleanup.Options{Mode: cleanup.ModeAggressive})
	if view := stripANSI(plain.viewCleanupConfirm()); strings.Contains(view, "--force") {
		t.Errorf("--force must only be echoed when it was given:\n%s", view)
	}
}

// --force only force-deletes in aggressive mode. Saying otherwise on the
// safe-mode confirm screen would overstate the run.
func TestCleanupConfirm_ForceInSafeModeIsStatedAsNoEffect(t *testing.T) {
	m := makeCleanupConfirmModel(&cleanup.Options{Mode: cleanup.ModeSafe, Force: true})
	view := stripANSI(m.viewCleanupConfirm())
	if !strings.Contains(view, "no effect in safe mode") {
		t.Errorf("expected the safe-mode force note:\n%s", view)
	}
}

func TestCleanupResult_EchoesTheFlagsThatRan(t *testing.T) {
	m := resultModel(&cleanup.Options{Mode: cleanup.ModeAggressive, Force: true}, cleanup.Result{})
	view := stripANSI(m.viewCleanupResult())
	if !strings.Contains(view, "ran: sentei cleanup --force --mode aggressive") {
		t.Errorf("the echo must carry every flag of the run:\n%s", view)
	}
}

// After `sentei cleanup --mode aggressive`, the tip must not recommend the
// mode that just ran.
func TestCleanupResult_TipAfterAggressiveRunFromFlags(t *testing.T) {
	m := resultModel(&cleanup.Options{Mode: cleanup.ModeAggressive}, cleanup.Result{
		NonWtBranchesRemaining: 1,
		BranchesSkipped:        []cleanup.SkippedBranch{{Name: "old/unmerged", Reason: cleanup.SkipUnmerged}},
	})
	view := stripANSI(m.viewCleanupResult())
	if strings.Contains(view, "Run `sentei cleanup --mode aggressive` to remove them") {
		t.Errorf("must not re-recommend the mode that just ran:\n%s", view)
	}
	if !strings.Contains(view, "--mode aggressive --force") {
		t.Errorf("expected the --force tip:\n%s", view)
	}
}

func TestViewCleanupResult_SkippedBranchesCarryTheirReason(t *testing.T) {
	m := resultModel(nil, cleanup.Result{BranchesSkipped: []cleanup.SkippedBranch{
		{Name: "fix/in-worktree", Reason: cleanup.SkipInWorktree},
		{Name: "develop", Reason: cleanup.SkipProtected},
	}})

	view := stripANSI(m.viewCleanupResult())
	for _, want := range []string{"2 branches skipped", "fix/in-worktree", "checked out in worktree", "develop", "protected branch"} {
		if !strings.Contains(view, want) {
			t.Errorf("missing %q:\n%s", want, view)
		}
	}
	for _, wrong := range []string{"unmerged", "not fully merged"} {
		if strings.Contains(view, wrong) {
			t.Errorf("no branch was skipped as %q:\n%s", wrong, view)
		}
	}
}

func TestViewCleanupPreview_FailedProbeIsNotClean(t *testing.T) {
	m := previewModel()
	m.cleanupScan = &cleanup.DryRunResult{Errors: []cleanup.OperationError{
		{Step: "prune-refs", Err: errors.New("could not read from remote repository")},
	}}

	view := stripANSI(m.viewCleanupPreview())
	if strings.Contains(view, "Repository is clean") {
		t.Errorf("a scan with a failed check cannot call the repository clean:\n%s", view)
	}
	if strings.Contains(view, "No stale remote refs") {
		t.Errorf("the failed check must not report that it found nothing:\n%s", view)
	}
	for _, want := range []string{"1 check failed", "prune-refs", "could not read from remote repository", "No branches with gone upstream"} {
		if !strings.Contains(view, want) {
			t.Errorf("missing %q:\n%s", want, view)
		}
	}
}

func TestViewCleanupResult_FailedStepDoesNotReportNothingFound(t *testing.T) {
	m := resultModel(nil, cleanup.Result{Errors: []cleanup.OperationError{
		{Step: "prune-refs", Err: errors.New("could not read from remote repository")},
	}})

	view := stripANSI(m.viewCleanupResult())
	if strings.Contains(view, "No stale remote refs") || strings.Contains(view, "Repository is clean") {
		t.Errorf("the failed step must not read as nothing found:\n%s", view)
	}
	if !strings.Contains(view, "No branches with gone upstream") {
		t.Errorf("steps that ran must still report:\n%s", view)
	}
}

func TestCleanupPreview_RescanClearsThePreviousScanError(t *testing.T) {
	m := previewModel()
	m.runner = &stubRunner{responses: map[string]stubResponse{}}
	m.cleanupScanErr = errors.New("boom")

	m, _ = m.startCleanupScan()
	updated, _ := m.updateCleanupPreview(cleanupScanDoneMsg{result: *scanWithAggressive(1)})

	view := stripANSI(updated.(Model).viewCleanupPreview())
	if strings.Contains(view, "Scan failed") {
		t.Errorf("the error of an earlier scan is still on screen after a successful rescan:\n%s", view)
	}
	if !strings.Contains(view, "Safe cleanup:") {
		t.Errorf("expected the new scan's preview:\n%s", view)
	}
}

// The clean preview's footer reads "enter back". Enter must do that, not
// start a cleanup run.
func TestCleanupPreview_EnterOnCleanPreviewGoesBack(t *testing.T) {
	m := previewModel()
	m.runner = &stubRunner{responses: map[string]stubResponse{}}
	m.cleanupScan = &cleanup.DryRunResult{}

	updated, cmd := m.updateCleanupPreview(tea.KeyPressMsg{Code: tea.KeyEnter})

	if got := updated.(Model).view; got != menuView {
		t.Errorf("view = %d, want the menu (%d)", got, menuView)
	}
	if cmd != nil {
		t.Error("enter on a clean preview must not start a cleanup run")
	}
}

// git puts the cause at the end of its message. A long failure must keep it
// on screen instead of being cut to one row.
func TestViewCleanupResult_LongErrorKeepsItsCause(t *testing.T) {
	const reason = "removing config section of branch ghost: git config --local --remove-section branch.ghost: " +
		"error: could not lock config file /repo/.bare/config: File exists"
	m := resultModel(nil, cleanup.Result{Errors: []cleanup.OperationError{{Step: "orphaned-configs", Err: errors.New(reason + "\nhint: ignored")}}})
	m.width = 60

	view := stripANSI(m.viewCleanupResult())
	unwrapped := strings.Join(strings.Fields(view), "")
	if !strings.Contains(unwrapped, strings.Join(strings.Fields(reason), "")) {
		t.Errorf("the reason was cut:\n%s", view)
	}
	if strings.Contains(view, "hint: ignored") {
		t.Errorf("only the first line of a git error belongs on the row:\n%s", view)
	}
	for _, line := range strings.Split(view, "\n") {
		if len([]rune(line)) > 60 {
			t.Errorf("row is wider than the 60-column terminal: %q", line)
		}
	}
}

// A step that deletes branches fails once per branch and joins the failures.
// Each one is a branch that is still there, so each needs its own row.
func TestViewCleanupResult_EveryFailedDeletionIsListed(t *testing.T) {
	m := resultModel(nil, cleanup.Result{Errors: []cleanup.OperationError{{
		Step: "gone-branches",
		Err: errors.Join(
			errors.New("deleting branch gone/a: cannot lock ref\n\nAnother git process seems to be running"),
			errors.New("deleting branch gone/b: cannot lock ref"),
		),
	}}})

	view := stripANSI(m.viewCleanupResult())
	for _, want := range []string{"deleting branch gone/a", "deleting branch gone/b"} {
		if !strings.Contains(view, want) {
			t.Errorf("missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "Another git process") {
		t.Errorf("only the first line of each failure belongs on its row:\n%s", view)
	}
}
