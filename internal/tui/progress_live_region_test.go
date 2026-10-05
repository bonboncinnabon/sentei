package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/bonboncinnabon/sentei/internal/progress"
)

func TestProgressViewport_TierBoundaries(t *testing.T) {
	cases := []struct {
		height int
		want   progressViewportTier
	}{{18, progressViewportNormal}, {17, progressViewportCompact}, {12, progressViewportCompact}, {11, progressViewportMinimal}, {4, progressViewportMinimal}, {3, progressViewportEmergency}}
	for _, tc := range cases {
		if got := BuildProgressViewport(denseProgressPhases(), tc.height, false).Tier; got != tc.want {
			t.Errorf("height %d tier = %v, want %v", tc.height, got, tc.want)
		}
	}
}

func TestModel_KeepsRawWindowHeightAndExactProgressTarget(t *testing.T) {
	m := NewModel(nil, nil, "/repo")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updated.(Model)
	if m.windowHeight != 24 || m.height != 18 {
		t.Fatalf("heights raw/body = %d/%d, want 24/18", m.windowHeight, m.height)
	}
	m.view = progressView
	m.remove.run.events = []progress.Event{
		{Phase: "remove", PhaseLabel: "Remove", Step: "a", StepLabel: "A", Status: progress.StepPending, Of: 2},
		{Phase: "remove", PhaseLabel: "Remove", Close: true},
		{Phase: "remove", Step: "a", Status: progress.StepRunning, Checkpoint: 1, Of: 2},
	}
	if cmd := m.syncProgressBar(); cmd == nil {
		t.Fatal("syncProgressBar returned nil")
	}
	if m.progressTarget != 0.5 {
		t.Fatalf("progressTarget = %v, want 0.5", m.progressTarget)
	}
	if got := m.removalLayout().Height; got != 24 {
		t.Fatalf("layout height = %d, want raw 24", got)
	}
}

func TestProgressLayout_ResizeRoundTripRestoresPinnedRows(t *testing.T) {
	layout := ProgressLayout{Title: "T", Width: 80, Height: 24, Phases: denseProgressPhases(), Hints: progressFooter}
	want := layout.View()
	layout.Height = 11
	if got := lipgloss.Height(layout.View()); got > 11 {
		t.Fatalf("minimal resize rendered %d rows", got)
	}
	layout.Height = 24
	if got := layout.View(); got != want {
		t.Fatal("24→11→24 changed the projected frame")
	}
}

func TestProgressLayout_CompactAndMinimalPinBarFooterToFinalRows(t *testing.T) {
	for _, height := range []int{17, 12, 11, 4} {
		out := stripANSI((ProgressLayout{Title: "T", Width: 80, Height: height, Phases: denseProgressPhases(), Hints: progressFooter}).View())
		lines := strings.Split(out, "\n")
		if len(lines) != height || !strings.Contains(lines[height-2], "%") || !strings.Contains(lines[height-1], "quit") {
			t.Fatalf("height %d final rows = %q / %q", height, lines[height-2], lines[height-1])
		}
	}
}

func TestProgressLayout_PinnedRowsAcrossEventPrefixes(t *testing.T) {
	steps := []progress.StepState{{ID: "a", Name: "A", Status: progress.StepPending}, {ID: "b", Name: "B", Status: progress.StepPending}}
	statuses := [][2]progress.StepStatus{{progress.StepPending, progress.StepPending}, {progress.StepRunning, progress.StepPending}, {progress.StepDone, progress.StepRunning}, {progress.StepDone, progress.StepDone}}
	for prefix, pair := range statuses {
		steps[0].Status, steps[1].Status = pair[0], pair[1]
		done := 0
		for _, step := range steps {
			if step.Status == progress.StepDone {
				done++
			}
		}
		out := stripANSI((ProgressLayout{Title: "T", Width: 80, Height: 24, Phases: []progress.PhaseState{{ID: "p", Name: "P", Total: 2, Done: done, Closed: true, Steps: append([]progress.StepState(nil), steps...)}}, Hints: progressFooter}).View())
		lines := strings.Split(out, "\n")
		if !strings.Contains(lines[19], "┄") || !strings.Contains(lines[21], "%") || !strings.Contains(lines[23], "quit") {
			t.Fatalf("prefix %d displaced chrome", prefix)
		}
	}
}

func TestProgressLayout_EmptyPlanHasNoSyntheticPhase(t *testing.T) {
	out := stripANSI((ProgressLayout{Title: "T", Width: 40, Height: 12, Hints: progressFooter}).View())
	if strings.Contains(out, "phase waiting") || strings.Contains(out, "pending") || strings.Contains(out, "skipped") {
		t.Fatalf("empty plan synthesized work:\n%s", out)
	}
}

func TestProgressLayout_BarDegradesAfterDroppingElapsed(t *testing.T) {
	narrow := stripANSI((ProgressLayout{Title: "T", Width: 20, Height: 12, Phases: denseProgressPhases(), Hints: progressFooter}).View())
	bar := strings.Split(narrow, "\n")[10]
	if strings.ContainsAny(bar, "█░") || !strings.Contains(bar, "%") {
		t.Fatalf("20-cell bar must be percentage-only: %q", bar)
	}
	layout := ProgressLayout{Title: "T", Width: 39, Height: 12, Phases: denseProgressPhases(), Hints: progressFooter, Bar: "  " + strings.Repeat("█", 20) + " 50%", Elapsed: "elapsed 999s"}
	bar = stripANSI(strings.Split(layout.View(), "\n")[10])
	if !strings.Contains(bar, "█") || strings.Contains(bar, "elapsed") {
		t.Fatalf("elapsed must drop before useful bar: %q", bar)
	}
}

func TestProgressLayout_TruncatesCompleteSkipAndErrorTextByCells(t *testing.T) {
	phase := progress.PhaseState{ID: "p", Name: "P", Total: 2, Done: 2, Failed: 1, Closed: true, Steps: []progress.StepState{
		{ID: "skip", Name: "界界界 skipped", Status: progress.StepSkipped, Message: strings.Repeat("reason ", 20)},
		{ID: "fail", Name: "failed", Status: progress.StepFailed, Error: fmt.Errorf("\x1b[31m%s\x1b[0m", strings.Repeat("error ", 20))},
	}}
	for _, width := range []int{20, 40, 50} {
		out := (ProgressLayout{Title: "T", Width: width, Height: 12, Phases: []progress.PhaseState{phase}, Hints: progressFooter}).View()
		for i, line := range strings.Split(out, "\n") {
			if got := lipgloss.Width(line); got > width {
				t.Fatalf("width %d line %d = %d cells", width, i+1, got)
			}
		}
	}
}

func denseProgressPhases() []progress.PhaseState {
	steps := make([]progress.StepState, 40)
	for i := range steps {
		steps[i] = progress.StepState{ID: fmt.Sprintf("step-%d", i), Name: fmt.Sprintf("step %02d", i), Status: progress.StepPending}
	}
	steps[2].Status = progress.StepRunning
	steps[8].Status = progress.StepFailed
	steps[8].Error = fmt.Errorf("failure detail")
	return []progress.PhaseState{
		{ID: "done", Name: "Done", Total: 1, Done: 1, Closed: true, Steps: []progress.StepState{{ID: "old", Name: "old", Status: progress.StepDone}}},
		{ID: "active", Name: "Active", Total: len(steps), Done: 1, Failed: 1, Closed: true, Steps: steps},
		{ID: "queued", Name: "Queued", Total: 1, Closed: true, Steps: []progress.StepState{{ID: "later", Name: "later", Status: progress.StepPending}}},
	}
}

func TestProgressLayout_PinsNormalChromeAt80x24(t *testing.T) {
	out := stripANSI((ProgressLayout{Title: "T", Width: 80, Height: 24, Phases: denseProgressPhases(), Hints: progressFooter}).View())
	lines := strings.Split(out, "\n")
	if len(lines) != 24 {
		t.Fatalf("rows = %d, want 24:\n%s", len(lines), out)
	}
	if !strings.Contains(lines[19], "┄") || !strings.Contains(lines[21], "%") || !strings.Contains(lines[23], "quit") {
		t.Fatalf("pinned rows wrong: row20=%q row22=%q row24=%q", lines[19], lines[21], lines[23])
	}
}

func TestProgressLayout_NeverExceedsTerminal(t *testing.T) {
	for _, width := range []int{20, 40, 50, 80, 120} {
		for height := 1; height <= 40; height++ {
			out := (ProgressLayout{Title: "Unicode 界", Subtitle: strings.Repeat("long ", 40), Width: width, Height: height, Phases: denseProgressPhases(), Hints: progressFooter}).View()
			if got := lipgloss.Height(out); got > height {
				t.Fatalf("%dx%d rendered %d rows", width, height, got)
			}
			for i, line := range strings.Split(out, "\n") {
				if got := lipgloss.Width(line); got > width {
					t.Fatalf("%dx%d line %d rendered %d cols: %q", width, height, i+1, got, stripANSI(line))
				}
			}
		}
	}
}

func TestBuildProgressViewport_FocusRules(t *testing.T) {
	phase := func(id string, status progress.StepStatus) progress.PhaseState {
		return progress.PhaseState{ID: id, Name: id, Total: 1, Closed: true, Steps: []progress.StepState{{ID: id + "-step", Status: status}}}
	}
	tests := []struct {
		name      string
		phases    []progress.PhaseState
		completed bool
		want      string
	}{
		{"first running", []progress.PhaseState{phase("failed", progress.StepFailed), phase("run-a", progress.StepRunning), phase("run-b", progress.StepRunning)}, false, "run-a"},
		{"latest failed", []progress.PhaseState{phase("fail-a", progress.StepFailed), phase("fail-b", progress.StepFailed)}, false, "fail-b"},
		{"earliest unresolved", []progress.PhaseState{phase("pending-a", progress.StepPending), phase("pending-b", progress.StepPending)}, false, "pending-a"},
		{"latest terminal completion", []progress.PhaseState{phase("done-a", progress.StepDone), phase("done-b", progress.StepFailed)}, true, "done-b"},
		{"empty", nil, false, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			viewport := BuildProgressViewport(tc.phases, 24, tc.completed)
			got := ""
			if viewport.Focus != nil {
				got = viewport.Focus.ID
			}
			if got != tc.want {
				t.Fatalf("focus = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestWindowSteps_HardBoundsManyActiveFailures(t *testing.T) {
	steps := make([]progress.StepState, 30)
	for i := range steps {
		status := progress.StepFailed
		if i < 10 {
			status = progress.StepRunning
		}
		steps[i] = progress.StepState{ID: fmt.Sprintf("s-%d", i), Name: fmt.Sprintf("step-%d", i), Status: status}
	}
	for budget := 0; budget <= 8; budget++ {
		window := WindowSteps(steps, budget)
		if len(window.Steps)+btoi(window.Windowed && budget > 0) > budget {
			t.Fatalf("budget %d returned %d steps plus stat", budget, len(window.Steps))
		}
	}
}

func btoi(value bool) int {
	if value {
		return 1
	}
	return 0
}
