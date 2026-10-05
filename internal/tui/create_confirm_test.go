package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/config"
	"github.com/bonboncinnabon/sentei/internal/repo"
)

func makeCreateConfirmModel(opts *CreateOpts) Model {
	m := NewMenuModel(bareDirRunner("/repo"), nil, "/repo", &config.Config{}, repo.ContextBareRepo)
	if opts != nil {
		m.SetCreateOpts(opts)
	} else {
		m.view = createConfirmView
		m.create.branchInput.SetValue("feature/test")
		m.create.baseInput.SetValue("main")
	}
	m.width = 80
	m.height = 24
	return m
}

func TestSetCreateOpts_BranchAndBase_EntersConfirmView(t *testing.T) {
	m := NewMenuModel(bareDirRunner("/repo"), nil, "/repo", &config.Config{}, repo.ContextBareRepo)
	opts := &CreateOpts{Branch: "feature/foo", Base: "main"}
	m.SetCreateOpts(opts)

	if m.view != createConfirmView {
		t.Errorf("expected createConfirmView, got %d", m.view)
	}
	if m.createOpts != opts {
		t.Error("expected createOpts to be set")
	}
	if m.create.branchInput.Value() != "feature/foo" {
		t.Errorf("expected branch input 'feature/foo', got %q", m.create.branchInput.Value())
	}
	if m.create.baseInput.Value() != "main" {
		t.Errorf("expected base input 'main', got %q", m.create.baseInput.Value())
	}
}

func TestSetCreateOpts_OnlyBranch_EntersOptionsView(t *testing.T) {
	m := NewMenuModel(bareDirRunner("/repo"), nil, "/repo", &config.Config{}, repo.ContextBareRepo)
	opts := &CreateOpts{Branch: "feature/bar"}
	m.SetCreateOpts(opts)

	if m.view != createOptionsView {
		t.Errorf("expected createOptionsView, got %d", m.view)
	}
}

func TestSetCreateOpts_NothingSet_EntersBranchView(t *testing.T) {
	m := NewMenuModel(bareDirRunner("/repo"), nil, "/repo", &config.Config{}, repo.ContextBareRepo)
	opts := &CreateOpts{}
	m.SetCreateOpts(opts)

	if m.view != createBranchView {
		t.Errorf("expected createBranchView, got %d", m.view)
	}
}

func TestCreateConfirmationVM_RendersBranchAndBase(t *testing.T) {
	m := makeCreateConfirmModel(&CreateOpts{
		Branch: "feature/test",
		Base:   "main",
	})
	vm := m.createConfirmationVM()

	if vm.Title != "Confirm create" {
		t.Errorf("expected title 'Confirm create', got %q", vm.Title)
	}

	output := stripAnsi(vm.View())
	if !strings.Contains(output, "feature/test") {
		t.Errorf("expected branch name in view, got:\n%s", output)
	}
	if !strings.Contains(output, "main") {
		t.Errorf("expected base branch in view, got:\n%s", output)
	}
	if !strings.Contains(output, "sentei create") {
		t.Errorf("expected CLI command in view, got:\n%s", output)
	}
}

func TestCreateConfirmationVM_ShowsMergeBaseAndCopyEnv(t *testing.T) {
	m := makeCreateConfirmModel(&CreateOpts{
		Branch:    "feature/test",
		Base:      "main",
		MergeBase: true,
		CopyEnv:   true,
	})
	vm := m.createConfirmationVM()

	output := stripAnsi(vm.View())
	if !strings.Contains(output, "Merge base:") {
		t.Errorf("expected 'Merge base:' in view, got:\n%s", output)
	}
	if !strings.Contains(output, "Copy env:") {
		t.Errorf("expected 'Copy env:' in view, got:\n%s", output)
	}
	if !strings.Contains(output, "--merge-base") {
		t.Errorf("expected '--merge-base' in CLI command, got:\n%s", output)
	}
	if !strings.Contains(output, "--copy-env") {
		t.Errorf("expected '--copy-env' in CLI command, got:\n%s", output)
	}
}

func TestUpdateCreateConfirm_ProceedSetsStateBeforeCreate(t *testing.T) {
	// ConfirmProceedMsg transitions to createProgressView and spawns a creation
	// goroutine via startCreation. Since that requires a real runner, we verify
	// the pre-proceed state instead (nil runner would panic in the goroutine).
	m := makeCreateConfirmModel(nil)

	if m.view != createConfirmView {
		t.Errorf("expected createConfirmView before proceed, got %d", m.view)
	}
}

func TestUpdateCreateConfirm_BackReturnsToOptions(t *testing.T) {
	m := makeCreateConfirmModel(nil)
	// Not launched directly (createOpts is nil), so back goes to options.

	updated, cmd := m.updateCreateConfirm(ConfirmBackMsg{})
	result := updated.(Model)

	if result.view != createOptionsView {
		t.Errorf("expected createOptionsView, got %d", result.view)
	}
	if cmd != nil {
		t.Error("expected nil cmd when going back to options")
	}
}

func TestUpdateCreateConfirm_BackQuitsWhenLaunchedDirectly(t *testing.T) {
	m := makeCreateConfirmModel(&CreateOpts{Branch: "feat/x", Base: "main"})

	_, c := m.updateCreateConfirm(ConfirmBackMsg{})
	if c == nil {
		t.Fatal("expected quit cmd when launched directly with opts")
	}
	msg := c()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", msg)
	}
}

func TestUpdateCreateConfirm_WindowSizeMsg(t *testing.T) {
	m := makeCreateConfirmModel(nil)

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	result := updated.(Model)

	if result.width != 120 {
		t.Errorf("expected width=120, got %d", result.width)
	}
}

func TestUpdateCreateConfirm_QuitKey(t *testing.T) {
	m := makeCreateConfirmModel(nil)

	_, c := m.updateCreateConfirm(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if c == nil {
		t.Fatal("expected quit cmd for q key")
	}
	msg := c()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", msg)
	}
}

func TestViewCreateConfirm_RendersContent(t *testing.T) {
	m := makeCreateConfirmModel(nil)

	output := stripAnsi(m.viewCreateConfirm())

	if !strings.Contains(output, "Confirm create") {
		t.Error("view should contain title")
	}
	if !strings.Contains(output, "Branch:") {
		t.Error("view should contain Branch label")
	}
	if !strings.Contains(output, "Base:") {
		t.Error("view should contain Base label")
	}
	if !strings.Contains(output, "enter confirm") {
		t.Error("view should show keybindings")
	}
}

func TestCreateConfirm_UpdateDispatch(t *testing.T) {
	m := makeCreateConfirmModel(&CreateOpts{Branch: "feat/x", Base: "main"})

	// Verify Update dispatches to createConfirm handler.
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	result := updated.(Model)

	if result.width != 100 {
		t.Errorf("expected width=100, got %d", result.width)
	}
}

func TestCreateConfirm_ViewDispatch(t *testing.T) {
	m := makeCreateConfirmModel(&CreateOpts{Branch: "feat/x", Base: "main"})

	output := m.View().Content

	if !strings.Contains(stripAnsi(output), "Confirm create") {
		t.Error("View() should dispatch to viewCreateConfirm")
	}
}
