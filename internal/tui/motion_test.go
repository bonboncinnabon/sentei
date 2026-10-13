package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/progress"
	"github.com/bonboncinnabon/sentei/internal/repo"
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

// The band blends each ramp from its base to its peak: text away from the
// band stays at the base color and only the band moves off it.
func TestShimmerLine_BlendsFromTheRampBase(t *testing.T) {
	ramp := shimmerRamp{base: "#5f5fd7", peak: "#d3d3ff"}
	baseRune := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ramp.base)).Render("x")
	text := strings.Repeat("x", 60)

	// A quarter of the way through its sweep the band sits inside the text.
	quarter := int(shimmerPeriod / motionResolution / 4)
	out := shimmerLine(text, ramp, quarter)

	if got := stripANSI(out); got != text {
		t.Fatalf("stripped shimmer = %q, want the input", got)
	}
	atBase := strings.Count(out, baseRune)
	if atBase == 0 || atBase == len(text) {
		t.Errorf("%d of %d runes are at the base color, want most but not all", atBase, len(text))
	}
	if atBase < len(text)-2*int(shimmerBandHalf) {
		t.Errorf("only %d of %d runes are at the base color: the band is wider than %v cells either side", atBase, len(text), shimmerBandHalf)
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
	if msg := batch[len(batch)-1](); msg != (motionTickMsg{chain: model.motionChain}) {
		t.Errorf("appended command = %#v, want a tick of the model's current chain %d", msg, model.motionChain)
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
	m.remove.run = removalRun{}
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
	m.remove.run = removalRun{}
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

// Leaving a working view while a tick is in flight, then entering another
// within the tick period, starts a new chain while the old tick is still on
// its way. That tick must die on arrival, or two chains run and every
// animation moves at twice its speed.
func TestMotionTicks_TickFromAnEarlierChainIsDropped(t *testing.T) {
	m := NewMenuModel(nil, nil, "/repo", nil, repo.ContextBareRepo)
	updated, _ := m.Update(worktreeContextMsg{worktrees: []git.Worktree{{Path: "/repo/a", Branch: "refs/heads/a"}}, generation: m.worktreeGeneration})
	m = updated.(Model)
	m.menuCursor = 2 // Cleanup & exit

	enterScan := func(m Model) Model {
		updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		m = updated.(Model)
		if m.view != cleanupPreviewView || !m.motionActive() {
			t.Fatalf("enter did not start the cleanup scan: view=%d", m.view)
		}
		return m
	}

	m = enterScan(m)
	inFlight := motionTickMsg{chain: m.motionChain}

	// Back to the menu before that tick lands, and straight into a new scan.
	updated, _ = m.Update(keyEsc())
	m = enterScan(updated.(Model))

	updated, cmd := m.Update(inFlight)
	if cmd != nil || updated.(Model).motionTick != m.motionTick {
		t.Error("a tick from the chain that was left kept ticking beside the new chain")
	}
	updated, cmd = m.Update(motionTickMsg{chain: m.motionChain})
	if cmd == nil || updated.(Model).motionTick != m.motionTick+1 {
		t.Error("the tick of the current chain did not advance the clock")
	}
}
