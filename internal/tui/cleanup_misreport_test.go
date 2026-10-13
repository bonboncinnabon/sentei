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
		GoneBranchesDeleted:  []string{"feature/gone"},
		ConfigOrphanResult:   cleanup.ConfigResult{Removed: 1},
		ConfigDedupResult:    cleanup.ConfigResult{Removed: 3},
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
		"3 duplicate config lines would be removed",
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

func TestViewCleanupResult_OneRemainingBranchReadsAsSingular(t *testing.T) {
	skipped := []cleanup.SkippedBranch{{Name: "develop", Reason: cleanup.SkipProtected}}
	for name, result := range map[string]cleanup.Result{
		"nothing cleaned": {BranchesSkipped: skipped},
		"some cleaned":    {StaleRefsRemoved: 1, BranchesSkipped: skipped},
	} {
		view := stripANSI(resultModel(nil, result).viewCleanupResult())
		if !strings.Contains(view, "1 branch remains") {
			t.Errorf("%s: expected \"1 branch remains\":\n%s", name, view)
		}
	}
}

// A run with a failed step is not "complete": the title and the first line
// say so, whatever else the run did.
func TestViewCleanupResult_FailedStepIsNamedInTheTitle(t *testing.T) {
	failure := []cleanup.OperationError{{Step: "prune-refs", Err: errors.New("could not read from remote repository")}}
	m := resultModel(nil, cleanup.Result{
		ConfigDedupResult: cleanup.ConfigResult{Removed: 2},
		BranchesSkipped:   []cleanup.SkippedBranch{{Name: "wip", Reason: cleanup.SkipUnmerged}},
		Errors:            failure,
	})

	view := stripANSI(m.viewCleanupResult())
	if strings.Contains(view, "Cleanup complete") {
		t.Errorf("a run with a failed step reads as complete:\n%s", view)
	}
	for _, want := range []string{"sentei ─ Cleanup finished with errors", "✗ 1 step failed", "Removed 2 duplicate config lines", "prune-refs: could not read"} {
		if !strings.Contains(view, want) {
			t.Errorf("missing %q:\n%s", want, view)
		}
	}

	clean := stripANSI(resultModel(nil, cleanup.Result{StaleRefsRemoved: 1}).viewCleanupResult())
	if !strings.Contains(clean, "sentei ─ Cleanup complete") {
		t.Errorf("a run without failures lost its title:\n%s", clean)
	}
	dry := stripANSI(resultModel(&cleanup.Options{Mode: cleanup.ModeSafe, DryRun: true}, cleanup.Result{Errors: failure}).viewCleanupResult())
	if !strings.Contains(dry, "sentei ─ Cleanup preview") {
		t.Errorf("a dry run changed nothing and keeps the preview title:\n%s", dry)
	}
}

// Branches deleted for a gone upstream are gone for good, like the ones
// aggressive mode deletes: both lists are named under their own line.
func TestViewCleanupResult_NamesGoneBranchesUnderTheirCheck(t *testing.T) {
	result := cleanup.Result{
		GoneBranchesDeleted:  []string{"feature/gone-a", "feature/gone-b"},
		NonWtBranchesDeleted: []string{"old/one"},
	}
	for name, tc := range map[string]struct {
		opts *cleanup.Options
		line string
	}{
		"real run": {&cleanup.Options{Mode: cleanup.ModeAggressive}, "✦ Deleted 2 branches with gone upstream"},
		"dry run":  {&cleanup.Options{Mode: cleanup.ModeAggressive, DryRun: true}, "▸ 2 branches with gone upstream would be deleted"},
	} {
		view := stripANSI(resultModel(tc.opts, result).viewCleanupResult())
		want := "  " + tc.line + "\n      feature/gone-a\n      feature/gone-b\n  · No orphaned config sections\n"
		if !strings.Contains(view, want) {
			t.Errorf("%s: gone branches are not named under their check, want\n%s\ngot:\n%s", name, want, view)
		}
		if !strings.Contains(view, "not in any worktree") || !strings.Contains(view, "\n      old/one\n") {
			t.Errorf("%s: aggressive deletions lost their names:\n%s", name, view)
		}
	}
}

// Two long lists share the rows a short terminal leaves free; neither may
// push the footer off screen or starve the other.
func TestViewCleanupResult_BothNameListsFitTheTerminal(t *testing.T) {
	names := func(prefix string, n int) []string {
		var out []string
		for i := range n {
			out = append(out, prefix+itoa(i))
		}
		return out
	}
	for name, tc := range map[string]struct {
		gone, deleted int
		want          []string
	}{
		"both long":      {30, 30, []string{"gone/0", "old/0", "and 2"}},
		"short gone":     {2, 30, []string{"gone/0", "gone/1", "old/0", "and 2"}},
		"short deleted":  {30, 2, []string{"gone/0", "old/0", "old/1", "and 2"}},
		"only gone":      {30, 0, []string{"gone/0", "and 2"}},
		"everything fit": {3, 3, []string{"gone/2", "old/2"}},
	} {
		m := resultModel(&cleanup.Options{Mode: cleanup.ModeAggressive}, cleanup.Result{
			GoneBranchesDeleted:  names("gone/", tc.gone),
			NonWtBranchesDeleted: names("old/", tc.deleted),
		})
		m.windowHeight = 24

		view := stripANSI(m.viewCleanupResult())
		if rows := strings.Count(view, "\n"); rows > 24 {
			t.Errorf("%s: view is %d rows on a 24-row terminal:\n%s", name, rows, view)
		}
		for _, want := range append(tc.want, "enter quit") {
			if !strings.Contains(view, want) {
				t.Errorf("%s: missing %q:\n%s", name, want, view)
			}
		}
		if tc.gone+tc.deleted <= 6 && strings.Contains(view, "more") {
			t.Errorf("%s: names were dropped although they fit:\n%s", name, view)
		}
	}
}

// The F1 help lists the keys of the screen as it is now. On a preview with
// nothing to clean enter goes back, so the help must not promise a cleanup.
func TestCleanupPreviewHelp_MatchesWhatTheKeysDo(t *testing.T) {
	unmerged := &cleanup.DryRunResult{AggressiveBranches: []cleanup.BranchInfo{{Name: "wip"}}}
	confirming := previewModel()
	confirming.cleanupScan = scanWithAggressive(2)
	confirming.cleanupAggressiveConfirm = true

	for name, tc := range map[string]struct {
		model         func() Model
		want, notWant []string
	}{
		"scanning": {previewModel, []string{"esc back"}, []string{"enter", "cleanup", "details"}},
		"scan failed": {func() Model {
			m := previewModel()
			m.cleanupScanErr = errors.New("boom")
			return m
		}, []string{"esc back"}, []string{"enter", "cleanup", "details"}},
		"clean": {func() Model {
			m := previewModel()
			m.cleanupScan = &cleanup.DryRunResult{}
			return m
		}, []string{"enter back"}, []string{"cleanup", "aggressive", "details"}},
		"safe work only": {func() Model {
			m := previewModel()
			m.cleanupScan = scanWithAggressive(0)
			return m
		}, []string{"enter safe cleanup", "esc back"}, []string{"aggressive", "details"}},
		"nothing deletable without --force": {func() Model {
			m := previewModel()
			m.cleanupScan = unmerged
			return m
		}, []string{"enter safe cleanup", "? details"}, []string{"aggressive"}},
		"aggressive available": {func() Model {
			m := previewModel()
			m.cleanupScan = scanWithAggressive(2)
			return m
		}, []string{"enter safe cleanup", "a aggressive", "? details", "esc back"}, nil},
		"aggressive prompt": {func() Model { return confirming }, []string{"y delete", "n go back"}, []string{"enter", "safe cleanup"}},
	} {
		m := tc.model()
		_, help := m.helpContent()
		help = strings.Join(strings.Fields(stripANSI(help)), " ")
		for _, want := range tc.want {
			if !strings.Contains(help, want) {
				t.Errorf("%s: help is missing %q:\n%s", name, want, help)
			}
		}
		for _, notWant := range tc.notWant {
			if strings.Contains(help, notWant) {
				t.Errorf("%s: help offers %q, which does nothing of the kind here:\n%s", name, notWant, help)
			}
		}
		// The footer and the help are the same list.
		footer := stripANSI(m.cleanupPreviewFooter())
		for _, action := range m.cleanupPreviewActions() {
			if hint := action.Help().Key + " " + action.Help().Desc; !strings.Contains(footer, hint) {
				t.Errorf("%s: footer %q does not offer %q", name, footer, hint)
			}
		}
	}
}

// A message can change what the keys do while the help is open: the scan
// lands behind it. The help must not go on listing the keys of the state
// that ended.
func TestCleanupPreviewHelp_FollowsAScanThatLandsWhileOpen(t *testing.T) {
	updated, _ := previewModel().Update(tea.KeyPressMsg{Code: tea.KeyF1})
	m := updated.(Model)
	if help := stripANSI(m.portal.viewport.View()); !m.portal.Visible() || strings.Contains(help, "safe cleanup") {
		t.Fatalf("want the help of a preview that is still scanning, got:\n%s", help)
	}

	updated, _ = m.Update(cleanupScanDoneMsg{result: *scanWithAggressive(2)})
	m = updated.(Model)

	help := strings.Join(strings.Fields(stripANSI(m.portal.viewport.View())), " ")
	for _, want := range []string{"enter safe cleanup", "a aggressive", "? details", "esc back"} {
		if !strings.Contains(help, want) {
			t.Errorf("help still lists the keys of the scanning state, missing %q:\n%s", want, help)
		}
	}
}
