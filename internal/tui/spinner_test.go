package tui

import (
	"strings"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/config"
	"github.com/bonboncinnabon/sentei/internal/repo"
)

func TestCleanupScanning_RendersStarFrame(t *testing.T) {
	m := NewMenuModel(bareDirRunner("/repo"), nil, "/repo", &config.Config{}, repo.ContextBareRepo)
	m.view = cleanupPreviewView
	m.cleanupScan = nil

	view := stripAnsi(m.viewCleanupPreview())
	if !strings.Contains(view, "Scanning repository") {
		t.Fatalf("scanning text missing:\n%s", view)
	}
	if !strings.Contains(view, starFrame(m.motionTick)+" Scanning repository") {
		t.Errorf("scanning line must carry the star frame:\n%s", view)
	}
}

func TestMenuLoading_RendersStar(t *testing.T) {
	m := NewMenuModel(bareDirRunner("/repo"), nil, "/repo", &config.Config{}, repo.ContextBareRepo)
	m.width = 80

	view := stripAnsi(m.viewMenu())
	if !strings.Contains(view, starFrame(m.motionTick)+" loading…") {
		t.Errorf("loading menu item must render star + loading…:\n%s", view)
	}
}

func TestMenuLoaded_NoSpinner(t *testing.T) {
	m := NewMenuModel(bareDirRunner("/repo"), nil, "/repo", &config.Config{}, repo.ContextBareRepo)
	m.width = 80
	updated, _ := m.Update(worktreeContextMsg{worktrees: nil, generation: m.worktreeGeneration})
	m = updated.(Model)

	if strings.Contains(stripAnsi(m.viewMenu()), "loading…") {
		t.Errorf("loaded menu must not show the loading state:\n%s", stripAnsi(m.viewMenu()))
	}
}

func TestMotionTick_WhileScanning_SchedulesNext(t *testing.T) {
	m := NewMenuModel(bareDirRunner("/repo"), nil, "/repo", &config.Config{}, repo.ContextBareRepo)
	m.view = cleanupPreviewView
	m.cleanupScan = nil

	updated, cmd := m.Update(motionTickMsg{})
	if cmd == nil {
		t.Error("tick during an indeterminate wait must schedule the next tick")
	}
	if updated.(Model).motionTick != m.motionTick+1 {
		t.Error("tick must advance the motion clock")
	}
}

func TestMotionTick_WhenIdle_Swallowed(t *testing.T) {
	m := NewMenuModel(bareDirRunner("/repo"), nil, "/repo", &config.Config{}, repo.ContextBareRepo)
	updated, _ := m.Update(worktreeContextMsg{worktrees: nil, generation: m.worktreeGeneration})
	m = updated.(Model)

	if _, cmd := m.Update(motionTickMsg{}); cmd != nil {
		t.Error("tick with no indeterminate wait visible must not schedule another tick")
	}
}
