package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/git"
)

func TestWindowTitle_RestAndFlight(t *testing.T) {
	m := NewModel([]git.Worktree{}, nil, "/work/repo")

	if got := m.windowTitle(); got != "sentei · repo" {
		t.Errorf("title at rest = %q, want sentei · repo", got)
	}

	m.view = progressView
	m.remove.run = newRemovalRun(nil)
	// removalLayout with no statuses yields counts; the verb must appear.
	got := m.windowTitle()
	if got == "sentei · repo" || !strings.Contains(got, "removing") {
		t.Errorf("title in flight = %q, want a removing verb with counts", got)
	}
}

func TestWindowTitle_Scanning(t *testing.T) {
	m := NewModel([]git.Worktree{}, nil, "/work/repo")
	m.view = cleanupPreviewView
	m.cleanupScan = nil

	if got := m.windowTitle(); !strings.Contains(got, "scanning") {
		t.Errorf("scanning title = %q, want scanning marker", got)
	}
}

func TestTerminalProgress_States(t *testing.T) {
	m := NewModel([]git.Worktree{}, nil, "/work/repo")

	if got := m.terminalProgress(); got != nil {
		t.Errorf("no native progress expected at rest, got %+v", got)
	}

	m.view = cleanupPreviewView
	m.cleanupScan = nil
	if got := m.terminalProgress(); got == nil || got.State != tea.ProgressBarIndeterminate {
		t.Errorf("scanning must be indeterminate, got %+v", got)
	}

	m.view = createProgressView
	m.cleanupScan = nil
	m.create.result = nil
	if got := m.terminalProgress(); got == nil || got.State != tea.ProgressBarDefault {
		t.Errorf("live flow must carry a default-state value, got %+v", got)
	}
}
