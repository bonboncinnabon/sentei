package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/creator"
	"github.com/bonboncinnabon/sentei/internal/progress"
)

func createProgressModel() Model {
	m := createOptionsModel()
	m.view = createProgressView
	return m
}

func TestUpdateCreateProgress_QuitKey(t *testing.T) {
	m := createProgressModel()

	_, cmd := m.updateCreateProgress(keyMsg("q"))

	if cmd == nil {
		t.Fatal("expected a quit command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", cmd())
	}
}

func TestUpdateCreateProgress_OtherKeysIgnored(t *testing.T) {
	m := createProgressModel()

	updated, cmd := m.updateCreateProgress(keyMsg("x"))

	if updated.(Model).view != createProgressView || cmd != nil {
		t.Error("non-quit keys should be ignored during progress")
	}
}

func TestUpdateCreateProgress_WindowSize(t *testing.T) {
	m := createProgressModel()

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	model := updated.(Model)

	if model.width != 100 || model.height != 34 {
		t.Errorf("size = %dx%d, want 100x34", model.width, model.height)
	}
}

func TestUpdateCreateProgress_EventAppendsAndWaitsForNext(t *testing.T) {
	m := createProgressModel()
	m.create.eventCh = make(chan progress.Event, 1)

	ev := progress.Event{Phase: "Setup", Step: "Create worktree", Status: progress.StepRunning}
	updated, cmd := m.updateCreateProgress(createEventMsg{Event: ev})
	model := updated.(Model)

	if len(model.create.events) != 1 || model.create.events[0].Step != "Create worktree" {
		t.Errorf("events = %+v, want the received event appended", model.create.events)
	}
	if cmd == nil {
		t.Error("expected a command to wait for the next event")
	}
}

func TestUpdateCreateProgress_CompleteAdvancesToSummary(t *testing.T) {
	m := createProgressModel()
	genBefore := m.worktreeGeneration

	result := creator.Result{WorktreePath: "/repo/feature-x"}
	updated, cmd := m.updateCreateProgress(createCompleteMsg{Result: result})
	model := updated.(Model)

	model = settleNow(t, model)
	if model.view != createSummaryView {
		t.Errorf("view = %d, want createSummaryView", model.view)
	}
	if model.create.result == nil || model.create.result.WorktreePath != "/repo/feature-x" {
		t.Errorf("result = %+v, want stored", model.create.result)
	}
	if model.worktreeGeneration != genBefore+1 {
		t.Error("completion must bump worktreeGeneration to refresh the worktree list")
	}
	if cmd == nil {
		t.Error("expected a command batch (worktree context reload)")
	}
}

func TestViewCreateProgress_ShowsDeclaredPhases(t *testing.T) {
	m := createProgressModel()
	m.create.events = []progress.Event{
		{Phase: "Setup", Step: "Create worktree", Status: progress.StepDone},
		{Phase: "Dependencies", Step: "npm install", Status: progress.StepFailed, Error: errors.New("boom")},
	}

	view := stripANSI(m.viewCreateProgress())

	for _, want := range []string{"Creating worktree", "feature/x", "from main", "Setup", "Dependencies", "q quit"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q:\n%s", want, view)
		}
	}
}
