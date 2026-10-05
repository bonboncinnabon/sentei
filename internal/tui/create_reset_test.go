package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/creator"
	"github.com/bonboncinnabon/sentei/internal/progress"
	"github.com/bonboncinnabon/sentei/internal/repo"
)

func TestMenuEntry_CreateWorktree_ResetsFlowState(t *testing.T) {
	m := NewMenuModel(nil, nil, "/repo", nil, repo.ContextBareRepo)
	m.menuCursor = 0 // "Create new worktree" is the first bare-repo menu item
	m.view = menuView

	// Pollute with a previous run's state.
	m.create.branchInput.SetValue("feature/old")
	m.create.baseInput.SetValue("develop")
	m.create.focusedField = 1
	m.create.validationErr = "stale error"
	m.create.ecoEnabled = map[string]bool{"node": false}
	m.create.mergeBase = false
	m.create.copyEnvFiles = false
	m.create.optionsCursor = 3
	m.create.events = []progress.Event{{Phase: "Setup", Step: "old"}}
	m.create.result = &creator.Result{}

	updated, _ := m.updateMenu(tea.KeyPressMsg{Code: tea.KeyEnter})
	model := updated.(Model)

	if model.view != createBranchView {
		t.Fatalf("expected createBranchView, got %d", model.view)
	}
	if got := model.create.branchInput.Value(); got != "" {
		t.Errorf("expected empty branch input, got %q", got)
	}
	if got := model.create.baseInput.Value(); got != defaultBaseBranch {
		t.Errorf("expected base input %q, got %q", defaultBaseBranch, got)
	}
	if model.create.focusedField != 0 {
		t.Errorf("expected focus on branch field, got %d", model.create.focusedField)
	}
	if model.create.validationErr != "" {
		t.Errorf("expected cleared validation error, got %q", model.create.validationErr)
	}
	if len(model.create.ecoEnabled) != 0 {
		t.Errorf("expected fresh ecosystem toggles, got %v", model.create.ecoEnabled)
	}
	if !model.create.mergeBase || !model.create.copyEnvFiles {
		t.Error("expected option toggles restored to defaults (true)")
	}
	if model.create.optionsCursor != 0 {
		t.Errorf("expected options cursor reset, got %d", model.create.optionsCursor)
	}
	if model.create.events != nil || model.create.result != nil {
		t.Error("expected previous run's events and result cleared")
	}
	if !model.create.branchInput.Focused() {
		t.Error("expected branch input focused after reset")
	}
}

func TestMenuEntry_CreateWorktree_ResetAfterAbandonedFlow(t *testing.T) {
	m := NewMenuModel(nil, nil, "/repo", nil, repo.ContextBareRepo)
	m.menuCursor = 0
	m.view = menuView

	updated, _ := m.updateMenu(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(Model)

	// Abandon mid-flow: type a partial name, then leave for the menu.
	m.create.branchInput.SetValue("feature/abando")
	m.view = menuView

	updated, _ = m.updateMenu(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(Model)

	if got := m.create.branchInput.Value(); got != "" {
		t.Errorf("expected pristine branch input after abandoned flow, got %q", got)
	}
}
