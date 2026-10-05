package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/bonboncinnabon/sentei/internal/config"
	"github.com/bonboncinnabon/sentei/internal/creator"
	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/progress"
	"github.com/bonboncinnabon/sentei/internal/repo"
)

func TestProgressLayout_SkippedPhasesAtCompletion(t *testing.T) {
	l := ProgressLayout{
		Completed: true,
		Width:     80,
		Height:    20,
		Phases: []progress.PhaseState{
			{Name: "Setup", Done: 1, Total: 1},
			{Name: "Dependencies", Total: 0},
		},
	}

	done, total := l.overall()
	if done != 1 || total != 1 {
		t.Errorf("overall() at completion = %d/%d, want 1/1 (no-work phase not outstanding)", done, total)
	}
	view := stripAnsi(l.View())
	if !strings.Contains(view, "skipped") {
		t.Errorf("completed no-work phase must read skipped:\n%s", view)
	}
	if strings.Contains(view, "pending") {
		t.Errorf("completed flow must not show pending:\n%s", view)
	}
}

func TestProgressLayout_PendingMidRunUnchanged(t *testing.T) {
	l := ProgressLayout{
		Width:  80,
		Height: 20,
		Phases: []progress.PhaseState{
			{Name: "Setup", Done: 1, Total: 1},
			{Name: "Dependencies", Total: 0},
		},
	}

	done, total := l.overall()
	if done != 1 || total != 2 {
		t.Errorf("overall() mid-run = %d/%d, want 1/2 (undiscovered phase outstanding)", done, total)
	}
	if !strings.Contains(stripAnsi(l.View()), "pending") {
		t.Error("mid-run no-work phase must keep the pending treatment")
	}
}

func TestCreateComplete_FinalSyncTargetsFull(t *testing.T) {
	m := NewMenuModel(bareDirRunner("/repo"), nil, "/repo", &config.Config{}, repo.ContextBareRepo)
	m.view = createProgressView
	m.minProgressDuration = time.Hour
	m.progressStartedAt = time.Now()

	updated, cmd := m.updateCreateProgress(createCompleteMsg{Result: creator.Result{}})
	if cmd == nil {
		t.Fatal("completion must return commands")
	}
	if pct := updated.(Model).bar.Percent(); pct != 1.0 {
		t.Errorf("final spring target = %.2f, want 1.0", pct)
	}
}

func TestInterruptedFlow_NamesLiveOperation(t *testing.T) {
	m := NewModel([]git.Worktree{}, nil, "/repo")

	cases := []struct {
		view   viewState
		opType string
		want   string
	}{
		{progressView, "", "worktree removal"},
		{createProgressView, "", "worktree creation"},
		{repoProgressView, "clone", "repository clone"},
		{repoProgressView, "create", "repository creation"},
		{integrationProgressView, "", "integration apply"},
		{menuView, "", ""},
	}
	for _, tc := range cases {
		m.view = tc.view
		m.repo.opType = tc.opType
		if got := m.InterruptedFlow(); got != tc.want {
			t.Errorf("InterruptedFlow(view=%d) = %q, want %q", tc.view, got, tc.want)
		}
	}
}

func TestCreateSummary_FullPathNeverTruncated(t *testing.T) {
	m := NewModel([]git.Worktree{}, nil, "/repo")
	m.width = 60
	longPath := "/very/long/playground/path/that/overflows/sixty/columns/worktrees/audit-new-feature"
	m.create.result = &creator.Result{WorktreePath: longPath}

	view := stripAnsi(m.viewCreateSummary())
	flat := strings.ReplaceAll(view, "\n", "")
	flat = strings.ReplaceAll(flat, " ", "")
	if !strings.Contains(flat, strings.ReplaceAll(longPath, " ", "")) {
		t.Errorf("cd path must be fully present (wrapped, not truncated):\n%s", view)
	}
}
