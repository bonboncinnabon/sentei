package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/creator"
	"github.com/bonboncinnabon/sentei/internal/progress"
)

func TestCreateSummaryShowsCreatorContractError(t *testing.T) {
	m := createOptionsModel()
	m.width = 100
	m.create.result = &creator.Result{Err: errors.New("progress delivery failed")}

	view := stripANSI(m.viewCreateSummary())
	if !strings.Contains(view, "progress delivery failed") || strings.Contains(view, " ready") {
		t.Fatalf("summary did not propagate creator error:\n%s", view)
	}
}

func TestUpdateCreateSummary_EnterReturnsToMenuWhenMenuLaunched(t *testing.T) {
	m := createOptionsModel()
	m.view = createSummaryView

	updated, cmd := m.updateCreateSummary(tea.KeyPressMsg{Code: tea.KeyEnter})

	if updated.(Model).view != menuView {
		t.Errorf("view = %d, want menuView", updated.(Model).view)
	}
	if cmd != nil {
		t.Error("returning to menu should not emit a command")
	}
}

func TestUpdateCreateSummary_EnterQuitsWhenDirectLaunch(t *testing.T) {
	m := createOptionsModel()
	m.view = createSummaryView
	m.menuItems = nil

	_, cmd := m.updateCreateSummary(tea.KeyPressMsg{Code: tea.KeyEnter})

	if cmd == nil {
		t.Fatal("expected a quit command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", cmd())
	}
}

func TestUpdateCreateSummary_QuitKey(t *testing.T) {
	m := createOptionsModel()
	m.view = createSummaryView

	_, cmd := m.updateCreateSummary(keyMsg("q"))

	if cmd == nil {
		t.Fatal("expected a quit command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", cmd())
	}
}

func TestViewCreateSummary_SuccessShowsReady(t *testing.T) {
	m := createOptionsModel()
	m.create.result = &creator.Result{WorktreePath: "/repo/feature-x"}

	view := stripANSI(m.viewCreateSummary())

	for _, want := range []string{"Worktree created", "feature/x ready", "Path", "/repo/feature-x", "feature/x (from main)", "cd /repo/feature-x", "enter menu"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q:\n%s", want, view)
		}
	}
}

func TestViewCreateSummary_NilResultFallsBackToDerivedPath(t *testing.T) {
	m := createOptionsModel()

	view := stripANSI(m.viewCreateSummary())

	if !strings.Contains(view, "feature/x ready") {
		t.Errorf("view should render ready state without a result:\n%s", view)
	}
	if !strings.Contains(view, "cd ") {
		t.Errorf("view should still offer a cd hint from the derived path:\n%s", view)
	}
}

func TestViewCreateSummary_FailuresShowDepsAndIndexSteps(t *testing.T) {
	m := createOptionsModel()
	m.create.result = &creator.Result{
		WorktreePath: "/repo/feature-x",
		Phases: []progress.Phase{
			{Name: "Setup", Steps: []progress.StepResult{{Name: "Create worktree", Status: progress.StepDone}}},
			{Name: "Dependencies", Steps: []progress.StepResult{
				{Name: "npm install", Status: progress.StepFailed, Error: errors.New("npm exploded")},
			}},
			{Name: "Integrations", Steps: []progress.StepResult{
				{Name: "Index code-review-graph", Status: progress.StepDone},
			}},
		},
	}

	view := stripANSI(m.viewCreateSummary())

	for _, want := range []string{"created with issues", "Deps", "npm install", "npm exploded", "Index", "Index code-review-graph"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "Create worktree") {
		t.Error("Setup phase steps must not be listed in the summary")
	}
}

func TestViewCreateSummary_DirectLaunchHintsQuit(t *testing.T) {
	m := createOptionsModel()
	m.menuItems = nil

	view := stripANSI(m.viewCreateSummary())

	if !strings.Contains(view, "enter quit") {
		t.Errorf("direct launch should hint enter quits:\n%s", view)
	}
}
