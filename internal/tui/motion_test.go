package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/progress"
	"github.com/bonboncinnabon/sentei/internal/testutil/mock"
)

func runningLayout() ProgressLayout {
	return ProgressLayout{
		Title: "T", Width: 80, Height: 30,
		Phases: []progress.PhaseState{
			{Name: "Removing worktrees", Total: 3, Done: 1, Steps: []progress.StepState{
				{Name: "done-step", Status: progress.StepDone},
				{Name: "active-step", Status: progress.StepRunning},
				{Name: "pending-step", Status: progress.StepPending},
			}},
		},
	}
}

func TestStarFrame_CyclesSingleCellFrames(t *testing.T) {
	ticksPerFrame := int(starInterval / motionResolution)
	for i, want := range starFrames {
		if n := len([]rune(want)); n != 1 {
			t.Errorf("frame %q is %d runes; status columns need single-cell frames", want, n)
		}
		if got := starFrame(i * ticksPerFrame); got != want {
			t.Errorf("starFrame(%d) = %q, want %q", i*ticksPerFrame, got, want)
		}
	}
	if got := starFrame(len(starFrames) * ticksPerFrame); got != starFrames[0] {
		t.Errorf("frames must wrap, got %q", got)
	}
}

func TestShimmerLine_PreservesContentAndMoves(t *testing.T) {
	text := "Removing worktrees"
	a := shimmerLine(text, shimmerRamp{base: "#5f5fd7", peak: "#d3d3ff"}, 0)
	b := shimmerLine(text, shimmerRamp{base: "#5f5fd7", peak: "#d3d3ff"}, 10)

	if got := stripAnsi(a); got != text {
		t.Errorf("stripped shimmer = %q, want %q", got, text)
	}
	if a == b {
		t.Error("the band must move between ticks")
	}
	if !strings.Contains(a, "\x1b[1m") && !strings.Contains(a, ";1m") && !strings.Contains(a, "1;") {
		t.Errorf("shimmered text must be bold, got %q", a)
	}
}

func TestLerpHex_Endpoints(t *testing.T) {
	if got := lerpHex("#000000", "#ffffff", 0); got != "#000000" {
		t.Errorf("t=0 = %q", got)
	}
	if got := lerpHex("#000000", "#ffffff", 1); got != "#ffffff" {
		t.Errorf("t=1 = %q", got)
	}
	if got := lerpHex("#202020", "#404040", 0.5); got != "#303030" {
		t.Errorf("midpoint = %q", got)
	}
}

func TestProgressLayout_StaticFallbackUsesStar(t *testing.T) {
	view := stripANSI(runningLayout().View())

	for _, retired := range []string{"●", "◐", "✓"} {
		if strings.Contains(view, retired) {
			t.Errorf("static layouts must not render retired glyph %q, view:\n%s", retired, view)
		}
	}
	if !strings.Contains(view, "✻ Removing worktrees") {
		t.Errorf("expected static star fallback on the active phase, view:\n%s", view)
	}
	if !strings.Contains(view, "    ✻ active-step") {
		t.Errorf("expected static star fallback on the running step, view:\n%s", view)
	}
	if !strings.Contains(view, "✦ done-step") {
		t.Errorf("expected crystallized done step, view:\n%s", view)
	}
}

func TestRenderProgressLayout_ShimmersWorkingLines(t *testing.T) {
	m := NewModel([]git.Worktree{}, nil, "/repo")
	m.view = progressView

	out := m.renderProgressLayout(runningLayout())
	plain := stripAnsi(out)
	if !strings.Contains(plain, starFrames[0]+" Removing worktrees") {
		t.Errorf("expected star frame inside the phase headline, view:\n%s", plain)
	}

	// Two motion ticks advance the clock; tick 2 = frame index 1.
	for range 2 {
		updated, _ := m.Update(motionTickMsg{})
		m = updated.(Model)
	}
	out2 := m.renderProgressLayout(runningLayout())
	if !strings.Contains(stripAnsi(out2), starFrames[1]+" Removing worktrees") {
		t.Errorf("expected the twinkle to advance, view:\n%s", stripAnsi(out2))
	}
	if out == out2 {
		t.Error("ticks must move the shimmer band even within one star frame")
	}
}

func TestMotionTicks_GatedToWorkingSurfaces(t *testing.T) {
	m := NewModel([]git.Worktree{}, nil, "/repo")
	m.view = listView

	if _, cmd := m.Update(motionTickMsg{}); cmd != nil {
		t.Error("motion ticks with no working surface must be swallowed")
	}

	m.view = progressView
	updated, cmd := m.Update(motionTickMsg{})
	if cmd == nil {
		t.Error("motion ticks in a progress view must continue the chain")
	}
	if updated.(Model).motionTick != 1 {
		t.Error("ticks must advance the clock")
	}

	m.view = cleanupPreviewView
	m.cleanupScan = nil
	if _, cmd := m.Update(motionTickMsg{}); cmd == nil {
		t.Error("motion ticks during the cleanup scan must continue the chain")
	}
}

func TestMotionClock_StartsOnFlowEntry(t *testing.T) {
	m := NewModel([]git.Worktree{{Path: "/work/a", Branch: "refs/heads/a"}}, &mock.Runner{}, "/repo")
	m.view = confirmView
	m.remove.selected = map[string]bool{"/work/a": true}

	updated, cmd := m.Update(keyMsg("y"))
	model := updated.(Model)
	if model.view != progressView {
		t.Fatalf("expected progressView after confirm, got %d", model.view)
	}
	batch, ok := cmd().(tea.BatchMsg)
	if !ok || len(batch) != 2 {
		t.Fatalf("entering a progress view must append one motion command, got %T with %d commands", batch, len(batch))
	}
	if msg := batch[len(batch)-1](); msg != (motionTickMsg{}) {
		t.Errorf("appended command = %T, want motionTickMsg", msg)
	}
}

func TestCleanupRunningLine_Shimmers(t *testing.T) {
	m := NewModel([]git.Worktree{}, nil, "/repo")
	m.view = cleanupResultView

	out := stripAnsi(m.viewCleanupResult())
	if !strings.Contains(out, starFrames[0]+" Running cleanup…") {
		t.Errorf("expected star + label on the running line, view:\n%s", out)
	}
}

func TestVocabulary_NoRetiredGlyphsInMajorViews(t *testing.T) {
	m := NewModel([]git.Worktree{{Path: "/work/a", Branch: "refs/heads/a"}}, nil, "/repo")
	m.width, m.height = 100, 40
	m.remove.run = newRemovalRun([]git.Worktree{{Path: "/work/a", Branch: "refs/heads/a"}})
	m.remove.run.result.SuccessCount = 1

	views := map[string]string{
		"progress": func() string { m.view = progressView; return m.viewProgress() }(),
		"summary":  func() string { m.view = summaryView; return m.viewSummary() }(),
		"cleanup":  func() string { m.view = cleanupResultView; return m.viewCleanupResult() }(),
	}
	for name, view := range views {
		plain := stripAnsi(view)
		for _, retired := range []string{"●", "◐", "✓"} {
			if strings.Contains(plain, retired) {
				t.Errorf("%s view contains retired glyph %q:\n%s", name, retired, plain)
			}
		}
	}
}

func TestSummaryVerdict_CrystallizedStar(t *testing.T) {
	m := NewModel(nil, nil, "/repo")
	m.width = 80
	m.remove.run = newRemovalRun(nil)
	m.remove.run.result.SuccessCount = 2
	m.view = summaryView

	view := stripAnsi(m.viewSummary())
	if !strings.Contains(view, "✦ 2 worktrees removed successfully") {
		t.Errorf("expected ✦ verdict headline, view:\n%s", view)
	}
}

func TestCleanupPreview_WouldActUsesArrow(t *testing.T) {
	var b strings.Builder
	writePreviewLine(&b, 3, "stale remote %s pruned", "ref", "refs", "No stale remote refs")
	if got := stripAnsi(b.String()); !strings.Contains(got, "▸ 3 stale remote refs pruned") {
		t.Errorf("expected would-act arrow line, got %q", got)
	}
}

func TestCompletedBar_SettlesGreen(t *testing.T) {
	m := NewModel([]git.Worktree{}, nil, "/repo")
	m.view = progressView

	// Drive the spring to full so the bar has filled cells to color.
	cmd := m.bar.SetPercent(1)
	for range 400 {
		msg := cmd()
		updated, next := m.Update(msg)
		m = updated.(Model)
		if next == nil {
			break
		}
		cmd = next
	}

	working := runningLayout()
	done := runningLayout()
	done.Phases[0].Done = done.Phases[0].Total
	done.Phases[0].Steps = nil
	done.Completed = true

	wOut := m.renderProgressLayout(working)
	dOut := m.renderProgressLayout(done)
	// Success-green truecolor must appear only in the completed bar.
	if strings.Contains(wOut, "0;135;95") {
		t.Error("working bar must not carry the done-green fill")
	}
	if !strings.Contains(dOut, "0;135;95") {
		t.Errorf("completed bar must settle to the done-green fill:\n%q", dOut)
	}
}
