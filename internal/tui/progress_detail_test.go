package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/bonboncinnabon/sentei/internal/progress"
)

func progressDetailModel() Model {
	m := NewModel(nil, nil, "/repo")
	m.view = progressView
	m.width, m.height, m.windowHeight = 50, 18, 24
	m.portal = m.portal.SetSize(50, 24)
	m.remove.run.events = []progress.Event{
		{Phase: "phase-id", PhaseLabel: "Readable phase", Step: "skip-id", StepLabel: "界 skipped step", Status: progress.StepPending, Of: 1},
		{Phase: "phase-id", PhaseLabel: "Readable phase", Step: "fail-id", StepLabel: "failed step", Status: progress.StepPending, Of: 1},
		{Phase: "phase-id", PhaseLabel: "Readable phase", Close: true},
		{Phase: "phase-id", Step: "skip-id", Status: progress.StepSkipped, Message: "already installed with a very long reason"},
		{Phase: "phase-id", Step: "fail-id", Status: progress.StepFailed, Error: errors.New("\x1b[31mfull failure detail\x1b[0m")},
	}
	return m
}

func compactProgressDetailText(s string) string {
	return strings.Join(strings.Fields(stripANSI(s)), "")
}

func TestProgressDetail_IncludesStructuredFullText(t *testing.T) {
	m := progressDetailModel()
	title, content := m.detailContent()
	plain := stripANSI(content)
	if title != "Progress details" {
		t.Fatalf("title = %q", title)
	}
	for _, want := range []string{"phase-id", "Readable phase", "skip-id", "界 skipped step", "skipped", "already installed with a very long reason", "fail-id", "failed", "full failure detail"} {
		if !strings.Contains(compactProgressDetailText(plain), compactProgressDetailText(want)) {
			t.Errorf("detail missing %q:\n%s", want, plain)
		}
	}
	for i, line := range strings.Split(m.viewProgress(), "\n") {
		if width := lipgloss.Width(line); width > 50 {
			t.Fatalf("main line %d width = %d", i+1, width)
		}
	}
}

func TestProgressDetail_OfferedOnlyForFailureOmissionOrTopError(t *testing.T) {
	m := NewModel(nil, nil, "/repo")
	m.view = progressView
	m.width, m.height, m.windowHeight = 80, 18, 24
	m.remove.run.events = []progress.Event{{Phase: "p", PhaseLabel: "P", Step: "s", StepLabel: "S", Status: progress.StepPending, Of: 1}, {Phase: "p", Close: true}}
	if _, content := m.detailContent(); content != "" {
		t.Fatalf("simple pending progress unexpectedly offered details: %q", content)
	}
	m.remove.run.result.Err = errors.New("delivery failed")
	if _, content := m.detailContent(); !strings.Contains(content, "delivery failed") {
		t.Fatalf("top-level error did not offer details: %q", content)
	}
	if footer := stripANSI(viewFooter(80, m.removalLayout().Hints)); !strings.Contains(footer, "? details") {
		t.Fatalf("details hint missing: %q", footer)
	}
}

func TestProgressDetail_OfferedWhenConstrainedLiveRegionOmitsAllSteps(t *testing.T) {
	layout := ProgressLayout{Height: 4, Phases: []progress.PhaseState{{
		ID: "phase", Name: "Phase", Total: 1, Steps: []progress.StepState{{ID: "step", Name: "Step", Status: progress.StepRunning}},
	}}}
	viewport := BuildProgressViewport(layout.Phases, layout.Height, false)
	if viewport.DetailRows != 1 {
		t.Fatalf("test requires one focus row, got %d", viewport.DetailRows)
	}
	if !progressNeedsDetails(layout, nil) {
		t.Fatal("omitting every step from a constrained live region must offer details")
	}
}

func TestProgressDetail_WrapsLongSourceTextToPortalWidth(t *testing.T) {
	skipReason := strings.Repeat("界🙂 reason ", 18)
	errorText := strings.Repeat("錯誤🙂 failure ", 18)
	m := NewModel(nil, nil, "/repo")
	m.view = progressView
	m.width, m.height, m.windowHeight = 32, 6, 12
	m.portal = m.portal.SetSize(32, 12)
	m.remove.run.events = []progress.Event{
		{Phase: "phase-id", PhaseLabel: "Readable phase", Step: "skip-id", StepLabel: "Skipped step", Status: progress.StepPending, Of: 2},
		{Phase: "phase-id", Step: "fail-id", StepLabel: "Failed step", Status: progress.StepPending, Of: 2},
		{Phase: "phase-id", PhaseLabel: "Readable phase", Close: true},
		{Phase: "phase-id", Step: "skip-id", Status: progress.StepSkipped, Message: skipReason},
		{Phase: "phase-id", Step: "fail-id", Status: progress.StepFailed, Error: errors.New("\x1b[31m" + errorText + "\x1b[0m")},
	}

	_, content := m.detailContent()
	plain := stripANSI(content)
	for name, source := range map[string]string{"skip reason": skipReason, "error": errorText} {
		if !strings.Contains(compactProgressDetailText(plain), compactProgressDetailText(source)) {
			t.Errorf("%s is not recoverable after wrapping:\n%s", name, plain)
		}
	}
	for i, line := range strings.Split(content, "\n") {
		if got := lipgloss.Width(line); got > m.portal.contentWidth() {
			t.Errorf("line %d width = %d, portal width = %d: %q", i+1, got, m.portal.contentWidth(), stripANSI(line))
		}
	}
}

func TestProgressDetail_NarrowPortalPreservesPrefixedUnicodeValues(t *testing.T) {
	skipReason := "界🙂 already installed with 完整 reason"
	errorText := "錯誤🙂 complete failure detail"
	m := NewModel(nil, nil, "/repo")
	m.view = progressView
	m.width, m.height, m.windowHeight = 20, 6, 12
	m.portal = m.portal.SetSize(20, 12)
	m.remove.run.events = []progress.Event{
		{Phase: "phase-id", PhaseLabel: "Readable phase", Step: "skip-id", StepLabel: "Skipped step", Status: progress.StepPending, Of: 2},
		{Phase: "phase-id", Step: "fail-id", StepLabel: "Failed step", Status: progress.StepPending, Of: 2},
		{Phase: "phase-id", PhaseLabel: "Readable phase", Close: true},
		{Phase: "phase-id", Step: "skip-id", Status: progress.StepSkipped, Message: skipReason},
		{Phase: "phase-id", Step: "fail-id", Status: progress.StepFailed, Error: errors.New(errorText)},
	}

	_, content := m.detailContent()
	plain := stripANSI(content)
	for name, source := range map[string]string{"skip reason": skipReason, "error": errorText} {
		if !strings.Contains(compactProgressDetailText(plain), compactProgressDetailText(source)) {
			t.Errorf("narrow detail lost %s %q:\n%s", name, source, plain)
		}
	}
	for row, line := range strings.Split(content, "\n") {
		if got := lipgloss.Width(line); got > m.portal.contentWidth() {
			t.Errorf("row %d width=%d exceeds narrow portal width=%d: %q", row+1, got, m.portal.contentWidth(), stripANSI(line))
		}
	}
}

func TestProgressPortal_StaysOpenAcrossBackgroundEventAndResize(t *testing.T) {
	m := progressDetailModel()
	title, content := m.detailContent()
	m.portal = m.portal.Open(portalDetails, title, content)
	updated, _ := m.Update(removalEventMsg{event: progress.Event{Phase: "phase-id", Step: "skip-id", Status: progress.StepSkipped, Message: "updated"}})
	m = updated.(Model)
	if !m.portal.Visible() {
		t.Fatal("background progress event closed portal")
	}
	if got := stripANSI(m.portal.viewport.View()); !strings.Contains(got, "updated") {
		t.Fatalf("background progress event left stale detail content:\n%s", got)
	}
	updated, _ = m.Update(tea.WindowSizeMsg{Width: 40, Height: 12})
	m = updated.(Model)
	if !m.portal.Visible() || m.portal.viewport.Width() != m.portal.contentWidth() {
		t.Fatal("resize did not preserve/refit portal")
	}
}

func TestProgressPortal_BackgroundRefreshPreservesAndClampsScroll(t *testing.T) {
	m := progressDetailModel()
	m.remove.run.events[4].Error = errors.New(strings.Repeat("failure line\n", 40))
	title, content := m.detailContent()
	m.portal = m.portal.Open(portalDetails, title, content)
	m.portal.viewport.ScrollDown(8)
	before := m.portal.viewport.YOffset()
	if before == 0 {
		t.Fatal("precondition: portal did not scroll")
	}

	updated, _ := m.Update(removalEventMsg{event: progress.Event{
		Phase: "phase-id", Step: "skip-id", Status: progress.StepSkipped,
		Message: "fresh background detail",
	}})
	m = updated.(Model)
	if got := m.portal.viewport.YOffset(); got != before {
		t.Fatalf("refresh moved scroll offset from %d to %d", before, got)
	}
}

func TestProgressPortal_BackgroundEventClosesWhenDetailsResolve(t *testing.T) {
	m := NewModel(nil, nil, "/repo")
	m.view = progressView
	m.width, m.height, m.windowHeight = 80, 18, 18
	m.portal = m.portal.SetSize(80, 12)
	m.remove.run.events = []progress.Event{
		{Phase: "p1", PhaseLabel: "First", Step: "s1", StepLabel: "First step", Status: progress.StepPending, Of: 1},
		{Phase: "p1", Close: true},
		{Phase: "p2", PhaseLabel: "Second", Step: "s2", StepLabel: "Second step", Status: progress.StepPending, Of: 1},
		{Phase: "p2", Close: true},
	}
	title, content := m.detailContent()
	if content == "" {
		t.Fatal("precondition: queued phase did not offer details")
	}
	m.portal = m.portal.Open(portalDetails, title, content)
	m.portal.viewport.ScrollDown(4)
	if m.portal.viewport.YOffset() == 0 {
		t.Fatal("precondition: portal did not scroll")
	}

	updated, _ := m.Update(removalEventMsg{event: progress.Event{Phase: "p1", Step: "s1", Status: progress.StepDone}})
	m = updated.(Model)
	if m.portal.Visible() {
		t.Fatal("resolved omission left a stale progress details portal open")
	}
}

func TestProgressPortal_ClosesWhenProgressTransitionsToSummary(t *testing.T) {
	m := progressDetailModel()
	title, content := m.detailContent()
	m.portal = m.portal.Open(portalDetails, title, content)
	m.progressToken = 7
	m.progressTransitionPending = true
	m.progressTargetView = summaryView

	updated, _ := m.Update(progressTransitionMsg{token: 7})
	m = updated.(Model)
	if m.view != summaryView {
		t.Fatalf("view=%v, want summary", m.view)
	}
	if m.portal.Visible() {
		t.Fatal("progress details portal persisted over summary transition")
	}
}
