package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/cleanup"
	"github.com/bonboncinnabon/sentei/internal/config"
	"github.com/bonboncinnabon/sentei/internal/repo"
)

func previewModel() Model {
	m := NewMenuModel(nil, nil, "/repo", &config.Config{}, repo.ContextBareRepo)
	m.width, m.height = 90, 28
	m.portal = m.portal.SetSize(90, 34)
	m.view = cleanupPreviewView
	return m
}

func scanWithAggressive(n int) *cleanup.DryRunResult {
	r := &cleanup.DryRunResult{StaleRefs: 2, GoneBranches: []string{"feature/gone"}}
	for i := 0; i < n; i++ {
		r.AggressiveBranches = append(r.AggressiveBranches, cleanup.BranchInfo{
			Name:              "old/branch-" + string(rune('a'+i)),
			LastCommitDate:    time.Date(2026, 3, 1+i, 12, 0, 0, 0, time.UTC),
			LastCommitSubject: "subject " + string(rune('a'+i)),
			Merged:            true,
		})
	}
	return r
}

func TestViewCleanupPreview_ScanningState(t *testing.T) {
	m := previewModel()
	view := stripANSI(m.viewCleanupPreview())
	for _, want := range []string{"sentei ─ Cleanup preview", starFrame(m.motionTick) + " Scanning repository…"} {
		if !strings.Contains(view, want) {
			t.Errorf("missing %q:\n%s", want, view)
		}
	}
}

func TestViewCleanupPreview_CleanRepository(t *testing.T) {
	m := previewModel()
	m.cleanupScan = &cleanup.DryRunResult{}
	view := stripANSI(m.viewCleanupPreview())
	if !strings.Contains(view, "✦ Repository is clean") {
		t.Errorf("expected clean message:\n%s", view)
	}
	if !strings.Contains(view, "enter back · q quit") {
		t.Errorf("expected clean-state hints:\n%s", view)
	}
}

func TestViewCleanupPreview_SafeResults(t *testing.T) {
	m := previewModel()
	m.cleanupScan = &cleanup.DryRunResult{StaleRefs: 3, ConfigDuplicates: 1, PrunableWorktrees: 1}
	view := stripANSI(m.viewCleanupPreview())

	for _, want := range []string{
		"Safe cleanup:",
		"▸ 3 stale remote refs would be pruned",
		"▸ 1 config duplicate would be removed",
		"▸ 1 stale worktree would be pruned",
		"· No branches with gone upstream",
		"· No orphaned config sections",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "Aggressive cleanup available") {
		t.Error("aggressive section must be absent when there are no candidates")
	}
	if !strings.Contains(view, "enter safe cleanup · esc back · q quit") {
		t.Errorf("expected safe-only hints:\n%s", view)
	}
}

func TestViewCleanupPreview_AggressiveOfferInlineAndMore(t *testing.T) {
	m := previewModel()
	m.cleanupScan = scanWithAggressive(5)
	view := stripANSI(m.viewCleanupPreview())

	for _, want := range []string{
		"Aggressive cleanup available:",
		"⚠ 5 branches not in any worktree would be deleted",
		"old/branch-a", "old/branch-b", "old/branch-c",
		"and 2 more — ? for details",
		"enter safe cleanup · a aggressive · ? details · esc back · q quit",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "old/branch-d") {
		t.Error("only the first three names render inline")
	}
}

func TestViewCleanupPreview_AggressiveFewNamesNoMore(t *testing.T) {
	m := previewModel()
	m.cleanupScan = scanWithAggressive(2)
	view := stripANSI(m.viewCleanupPreview())

	if !strings.Contains(view, "old/branch-a") || !strings.Contains(view, "old/branch-b") {
		t.Errorf("both names should render inline:\n%s", view)
	}
	if strings.Contains(view, "more — ?") {
		t.Error("no overflow line for two branches")
	}
}

func TestUpdateCleanupPreview_ScanRevealsImmediatelyWithoutHold(t *testing.T) {
	m := previewModel()
	updated, _ := m.updateCleanupPreview(cleanupScanDoneMsg{result: *scanWithAggressive(1)})
	model := updated.(Model)
	if model.cleanupScan == nil {
		t.Error("scan must reveal immediately when no hold is configured")
	}
}

func TestUpdateCleanupPreview_HoldStashesThenReveals(t *testing.T) {
	m := previewModel()
	m.minProgressDuration = time.Hour
	m.progressStartedAt = time.Now()

	updated, cmd := m.updateCleanupPreview(cleanupScanDoneMsg{result: *scanWithAggressive(1)})
	model := updated.(Model)
	if model.cleanupScan != nil {
		t.Fatal("scan must stay hidden during the hold")
	}
	if model.cleanupScanPending == nil || cmd == nil {
		t.Fatal("expected pending result and a reveal tick")
	}

	updated, _ = model.updateCleanupPreview(cleanupScanRevealMsg{token: model.progressToken})
	model = updated.(Model)
	if model.cleanupScan == nil || model.cleanupScanPending != nil {
		t.Error("reveal must promote the pending scan")
	}
}

func TestUpdateCleanupPreview_StaleRevealTokenIgnored(t *testing.T) {
	m := previewModel()
	pending := *scanWithAggressive(1)
	m.cleanupScanPending = &pending

	updated, _ := m.updateCleanupPreview(cleanupScanRevealMsg{token: m.progressToken - 1})
	model := updated.(Model)
	if model.cleanupScan != nil {
		t.Error("a stale reveal token must not promote the scan")
	}
}

func TestUpdateCleanupPreview_ScanError(t *testing.T) {
	m := previewModel()
	updated, _ := m.updateCleanupPreview(cleanupScanDoneMsg{err: errors.New("boom")})
	model := updated.(Model)

	view := stripANSI(model.viewCleanupPreview())
	if !strings.Contains(view, "Scan failed: boom") {
		t.Errorf("expected scan error surfaced:\n%s", view)
	}
}

func TestUpdateCleanupPreview_EnterRunsSafeCleanup(t *testing.T) {
	m := previewModel()
	m.cleanupScan = scanWithAggressive(0)
	m.runner = &stubRunner{responses: map[string]stubResponse{}}

	updated, cmd := m.updateCleanupPreview(tea.KeyPressMsg{Code: tea.KeyEnter})
	model := updated.(Model)

	if model.view != cleanupResultView {
		t.Errorf("expected cleanupResultView, got %d", model.view)
	}
	if model.cleanupResult != nil {
		t.Error("previous cleanup result must be cleared before the run")
	}
	if cmd == nil {
		t.Fatal("expected the cleanup run Cmd")
	}
	if _, ok := cmd().(standaloneCleanupDoneMsg); !ok {
		t.Error("expected the run to deliver a standaloneCleanupDoneMsg")
	}
}

func TestUpdateCleanupPreview_EnterWhileScanningIsNoop(t *testing.T) {
	m := previewModel()
	updated, cmd := m.updateCleanupPreview(tea.KeyPressMsg{Code: tea.KeyEnter})
	if updated.(Model).view != cleanupPreviewView || cmd != nil {
		t.Error("enter during the scan must do nothing")
	}
}

func TestUpdateCleanupPreview_AggressiveConfirmFlow(t *testing.T) {
	m := previewModel()
	m.cleanupScan = scanWithAggressive(4)
	m.runner = &stubRunner{responses: map[string]stubResponse{}}

	// a → confirm prompt
	updated, _ := m.updateCleanupPreview(keyRune('a'))
	model := updated.(Model)
	if !model.cleanupAggressiveConfirm {
		t.Fatal("a must arm the aggressive confirmation")
	}
	view := stripANSI(model.viewCleanupPreview())
	if !strings.Contains(view, "Delete 4 branches?") || !strings.Contains(view, "y delete · n go back") {
		t.Errorf("expected confirm prompt:\n%s", view)
	}

	// n → back to preview
	updated, _ = model.updateCleanupPreview(keyRune('n'))
	if updated.(Model).cleanupAggressiveConfirm {
		t.Fatal("n must disarm the confirmation")
	}

	// a, y → aggressive run
	updated, _ = model.updateCleanupPreview(keyRune('a'))
	updated, cmd := updated.(Model).updateCleanupPreview(keyRune('y'))
	model = updated.(Model)
	if model.view != cleanupResultView || cmd == nil {
		t.Error("y must start the aggressive cleanup run")
	}
}

func TestUpdateCleanupPreview_AggressiveKeyNoopWithoutCandidates(t *testing.T) {
	m := previewModel()
	m.cleanupScan = scanWithAggressive(0)

	updated, _ := m.updateCleanupPreview(keyRune('a'))
	if updated.(Model).cleanupAggressiveConfirm {
		t.Error("a must be a no-op when aggressive has nothing to do")
	}
}

func TestUpdateCleanupPreview_EscReturnsToMenu(t *testing.T) {
	m := previewModel()
	m.cleanupScan = scanWithAggressive(1)
	updated, _ := m.updateCleanupPreview(tea.KeyPressMsg{Code: tea.KeyEsc})
	if updated.(Model).view != menuView {
		t.Error("esc must return to the menu")
	}
}

func TestCleanupDetailContent_PortalIntegration(t *testing.T) {
	m := previewModel()
	m.cleanupScan = scanWithAggressive(4)

	title, content := m.detailContent()
	if title != "Aggressive cleanup details" {
		t.Fatalf("title = %q", title)
	}
	plain := stripANSI(content)
	for _, want := range []string{"4 branches aggressive cleanup targets", "old/branch-d", "2026-03-01", "subject a"} {
		if !strings.Contains(plain, want) {
			t.Errorf("detail content missing %q:\n%s", want, plain)
		}
	}

	// The global ? handler opens the portal with this content.
	updated, _ := m.Update(keyRune('?'))
	model := updated.(Model)
	if !model.portal.Visible() || model.portal.title != "Aggressive cleanup details" {
		t.Errorf("expected details portal, visible=%v title=%q", model.portal.Visible(), model.portal.title)
	}

	// No aggressive work → no portal.
	m2 := previewModel()
	m2.cleanupScan = scanWithAggressive(0)
	updated, _ = m2.Update(keyRune('?'))
	if updated.(Model).portal.Visible() {
		t.Error("? must be a no-op when there are no aggressive candidates")
	}
}

// TestE2E_CleanupPreviewFlow drives menu → scan → preview → safe run →
// result through direct model updates, asserting each screen.
func TestE2E_CleanupPreviewFlow(t *testing.T) {
	m := NewMenuModel(nil, nil, "/repo", &config.Config{}, repo.ContextBareRepo)
	m.width, m.height = 90, 28
	m.runner = &stubRunner{responses: map[string]stubResponse{}}
	m.menuCursor = 3 // "Cleanup & exit"
	m.view = menuView

	updated, cmd := m.updateMenu(tea.KeyPressMsg{Code: tea.KeyEnter})
	model := updated.(Model)
	if model.view != cleanupPreviewView || cmd == nil {
		t.Fatalf("menu must enter the preview and fire the scan, view=%d", model.view)
	}
	if !strings.Contains(stripANSI(model.viewCleanupPreview()), "Scanning repository…") {
		t.Fatal("expected the scanning state before results arrive")
	}

	updated, _ = model.updateCleanupPreview(cleanupScanDoneMsg{result: *scanWithAggressive(4)})
	model = updated.(Model)
	view := stripANSI(model.viewCleanupPreview())
	if !strings.Contains(view, "Safe cleanup:") || !strings.Contains(view, "Aggressive cleanup available:") {
		t.Fatalf("expected preview sections:\n%s", view)
	}

	updated, runCmd := model.updateCleanupPreview(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = updated.(Model)
	if model.view != cleanupResultView || runCmd == nil {
		t.Fatalf("enter must start the safe run, view=%d", model.view)
	}
	if !strings.Contains(stripANSI(model.viewCleanupResult()), "Running cleanup") {
		t.Fatal("result view must show the running state first")
	}

	msg := runCmd()
	done, ok := msg.(standaloneCleanupDoneMsg)
	if !ok {
		t.Fatalf("expected standaloneCleanupDoneMsg, got %T", msg)
	}
	updated, _ = model.updateCleanupResult(done)
	model = updated.(Model)
	final := stripANSI(model.viewCleanupResult())
	if !strings.Contains(final, "Cleanup complete") {
		t.Fatalf("expected completion screen:\n%s", final)
	}
	if !strings.Contains(final, "sentei cleanup --mode safe") {
		t.Fatalf("expected the CLI command echo on the result:\n%s", final)
	}
}

func TestViewCleanupPreview_UnmergedDisclosure(t *testing.T) {
	m := previewModel()
	scan := scanWithAggressive(3)
	scan.AggressiveBranches[1].Merged = false
	m.cleanupScan = scan

	view := stripANSI(m.viewCleanupPreview())
	if !strings.Contains(view, "old/branch-b (not merged)") {
		t.Errorf("unmerged candidates must be marked inline:\n%s", view)
	}
	if !strings.Contains(view, "1 not fully merged — only deleted with --force") {
		t.Errorf("expected the unmerged disclosure line:\n%s", view)
	}

	// The confirm prompt promises only what will actually happen.
	updated, _ := m.updateCleanupPreview(keyRune('a'))
	prompt := stripANSI(updated.(Model).viewCleanupPreview())
	if !strings.Contains(prompt, "Delete 2 branches? (1 unmerged will be skipped)") {
		t.Errorf("prompt must disclose skips:\n%s", prompt)
	}
}

func TestViewCleanupResult_SkippedBranchesSurfaced(t *testing.T) {
	m := previewModel()
	m.cleanupRanMode = cleanup.ModeAggressive
	m.view = cleanupResultView
	m.cleanupResult = &cleanup.Result{
		BranchesSkipped: []cleanup.SkippedBranch{{Name: "old/unmerged", Reason: cleanup.SkipUnmerged}},
	}

	view := stripANSI(m.viewCleanupResult())
	if !strings.Contains(view, "⚠ 1 branch skipped (not fully merged)") {
		t.Errorf("skips must be reported:\n%s", view)
	}
	if !strings.Contains(view, "old/unmerged") {
		t.Errorf("skipped branch names must be listed:\n%s", view)
	}
	if !strings.Contains(view, "--mode aggressive --force") {
		t.Errorf("tip must point at --force after an aggressive run with skips:\n%s", view)
	}
	if strings.Contains(view, "Run `sentei cleanup --mode aggressive` to remove them") {
		t.Errorf("must not re-recommend the mode that just ran:\n%s", view)
	}
}
