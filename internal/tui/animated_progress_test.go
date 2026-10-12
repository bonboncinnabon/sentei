package tui

import (
	"strings"
	"testing"
	"time"

	progressbar "charm.land/bubbles/v2/progress"

	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/progress"
	"github.com/bonboncinnabon/sentei/internal/repo"
)

func TestProgressLayout_Overall(t *testing.T) {
	cases := []struct {
		name                string
		layout              ProgressLayout
		wantDone, wantTotal int
	}{
		{"discovered phases summed", ProgressLayout{Phases: []progress.PhaseState{{Done: 2, Total: 4}, {Done: 1, Total: 2}}}, 3, 6},
		{"undiscovered phase counts as outstanding", ProgressLayout{Phases: []progress.PhaseState{{Done: 2, Total: 2}, {Total: 0}}}, 2, 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			done, total := tc.layout.overall()
			if done != tc.wantDone || total != tc.wantTotal {
				t.Errorf("overall() = %d/%d, want %d/%d", done, total, tc.wantDone, tc.wantTotal)
			}
		})
	}
}

func TestProgressFrame_GatedToProgressViews(t *testing.T) {
	m := NewModel([]git.Worktree{}, nil, "/repo")

	cmd := m.bar.SetPercent(0.5)
	frame, ok := cmd().(progressbar.FrameMsg)
	if !ok {
		t.Fatal("SetPercent must yield a FrameMsg")
	}

	m.view = listView
	if _, cmd := m.Update(frame); cmd != nil {
		t.Error("frames outside progress views must be swallowed")
	}

	m.view = progressView
	if _, cmd := m.Update(frame); cmd == nil {
		t.Error("frames in a progress view must continue the animation")
	}
}

func TestSyncProgressBar_TargetsActiveFlowOnly(t *testing.T) {
	m := NewModel([]git.Worktree{}, nil, "/repo")
	m.view = menuView
	if cmd := m.syncProgressBar(); cmd != nil {
		t.Error("no spring target outside progress views")
	}

	m.view = progressView
	if cmd := m.syncProgressBar(); cmd == nil {
		t.Error("progress view must produce a spring target (and stopwatch start)")
	}
}

func TestRenderProgressLayout_InjectsBarAndElapsed(t *testing.T) {
	m := NewModel([]git.Worktree{}, nil, "/repo")
	m.view = progressView

	out := stripAnsi(m.renderProgressLayout(m.removalLayout()))
	if !strings.Contains(out, "elapsed") {
		t.Errorf("rendered layout missing elapsed readout:\n%s", out)
	}
	if !strings.Contains(out, "%") {
		t.Errorf("rendered layout missing percentage text:\n%s", out)
	}
	if !strings.Contains(out, "░") {
		t.Errorf("rendered layout missing the bar track:\n%s", out)
	}
}

// The stopwatch is the once-a-second repaint ticker for the elapsed readout.
// Each tick schedules the next one after Interval, so a zero interval is a
// busy loop that re-renders the progress view as fast as the CPU allows.
func TestElapsedTicker_TicksAtMostOncePerSecond(t *testing.T) {
	models := map[string]Model{
		"NewModel":     NewModel(nil, nil, "/repo"),
		"NewMenuModel": NewMenuModel(nil, nil, "/repo", nil, repo.ContextBareRepo),
	}
	for name, m := range models {
		if m.watch.Interval < time.Second {
			t.Errorf("%s: stopwatch ticks every %v, want at most one tick per second", name, m.watch.Interval)
		}
	}
}
