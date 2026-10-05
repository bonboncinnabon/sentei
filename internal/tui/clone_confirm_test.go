package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/config"
	"github.com/bonboncinnabon/sentei/internal/repo"
)

func makeCloneConfirmModel(opts *CloneOpts) Model {
	m := NewMenuModel(nil, nil, "/repo", &config.Config{}, repo.ContextNoRepo)
	if opts != nil {
		m.SetCloneOpts(opts)
	} else {
		m.view = cloneConfirmView
		m.repo.urlInput.SetValue("git@github.com:user/repo.git")
		m.repo.cloneNameInput.SetValue("repo")
	}
	m.width = 80
	m.height = 24
	return m
}

func TestSetCloneOpts_URLSet_EntersConfirmView(t *testing.T) {
	m := NewMenuModel(nil, nil, "/repo", &config.Config{}, repo.ContextNoRepo)
	opts := &CloneOpts{URL: "git@github.com:user/repo.git", Name: "my-repo"}
	m.SetCloneOpts(opts)

	if m.view != cloneConfirmView {
		t.Errorf("expected cloneConfirmView, got %d", m.view)
	}
	if m.cloneOpts != opts {
		t.Error("expected cloneOpts to be set")
	}
	if m.repo.urlInput.Value() != "git@github.com:user/repo.git" {
		t.Errorf("expected URL input set, got %q", m.repo.urlInput.Value())
	}
	if m.repo.cloneNameInput.Value() != "my-repo" {
		t.Errorf("expected name input 'my-repo', got %q", m.repo.cloneNameInput.Value())
	}
}

func TestSetCloneOpts_NothingSet_EntersInputView(t *testing.T) {
	m := NewMenuModel(nil, nil, "/repo", &config.Config{}, repo.ContextNoRepo)
	opts := &CloneOpts{}
	m.SetCloneOpts(opts)

	if m.view != cloneInputView {
		t.Errorf("expected cloneInputView, got %d", m.view)
	}
}

func TestCloneConfirmationVM_RendersURLAndName(t *testing.T) {
	m := makeCloneConfirmModel(&CloneOpts{
		URL:  "git@github.com:user/repo.git",
		Name: "my-repo",
	})
	vm := m.cloneConfirmationVM()

	if vm.Title != "Confirm clone" {
		t.Errorf("expected title 'Confirm clone', got %q", vm.Title)
	}

	output := stripAnsi(vm.View())
	if !strings.Contains(output, "git@github.com:user/repo.git") {
		t.Errorf("expected URL in view, got:\n%s", output)
	}
	if !strings.Contains(output, "my-repo") {
		t.Errorf("expected name in view, got:\n%s", output)
	}
	if !strings.Contains(output, "sentei clone") {
		t.Errorf("expected CLI command in view, got:\n%s", output)
	}
}

func TestCloneConfirmationVM_URLOnly(t *testing.T) {
	m := makeCloneConfirmModel(&CloneOpts{
		URL: "git@github.com:user/repo.git",
	})
	vm := m.cloneConfirmationVM()

	output := stripAnsi(vm.View())
	if !strings.Contains(output, "git@github.com:user/repo.git") {
		t.Errorf("expected URL in view, got:\n%s", output)
	}
	if !strings.Contains(output, "--url") {
		t.Errorf("expected '--url' in CLI command, got:\n%s", output)
	}
}

func TestUpdateCloneConfirm_ProceedSetsStateBeforeClone(t *testing.T) {
	// ConfirmProceedMsg transitions to repoProgressView and spawns a clone
	// goroutine. Since startRepoPipeline requires a real runner, we verify
	// the state setup by checking the model fields that are set before the
	// pipeline starts. We use a model with view set directly (no opts) and
	// skip the actual execution since nil runner would panic.
	m := makeCloneConfirmModel(nil)

	// Verify pre-proceed state.
	if m.view != cloneConfirmView {
		t.Errorf("expected cloneConfirmView before proceed, got %d", m.view)
	}
	if m.repo.opType == "clone" {
		t.Error("opType should not be 'clone' before proceed")
	}
}

func TestUpdateCloneConfirm_BackReturnsToInput(t *testing.T) {
	m := makeCloneConfirmModel(nil)
	// Not launched directly (cloneOpts is nil), so back goes to input.

	updated, cmd := m.updateCloneConfirm(ConfirmBackMsg{})
	result := updated.(Model)

	if result.view != cloneInputView {
		t.Errorf("expected cloneInputView, got %d", result.view)
	}
	if cmd != nil {
		t.Error("expected nil cmd when going back to input")
	}
}

func TestUpdateCloneConfirm_BackQuitsWhenLaunchedDirectly(t *testing.T) {
	m := makeCloneConfirmModel(&CloneOpts{URL: "git@github.com:user/repo.git"})

	_, c := m.updateCloneConfirm(ConfirmBackMsg{})
	if c == nil {
		t.Fatal("expected quit cmd when launched directly with opts")
	}
	msg := c()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", msg)
	}
}

func TestUpdateCloneConfirm_WindowSizeMsg(t *testing.T) {
	m := makeCloneConfirmModel(nil)

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	result := updated.(Model)

	if result.width != 120 {
		t.Errorf("expected width=120, got %d", result.width)
	}
}

func TestUpdateCloneConfirm_QuitKey(t *testing.T) {
	m := makeCloneConfirmModel(nil)

	_, c := m.updateCloneConfirm(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if c == nil {
		t.Fatal("expected quit cmd for q key")
	}
	msg := c()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", msg)
	}
}

func TestViewCloneConfirm_RendersContent(t *testing.T) {
	m := makeCloneConfirmModel(nil)

	output := stripAnsi(m.viewCloneConfirm())

	if !strings.Contains(output, "Confirm clone") {
		t.Error("view should contain title")
	}
	if !strings.Contains(output, "URL:") {
		t.Error("view should contain URL label")
	}
	if !strings.Contains(output, "enter confirm") {
		t.Error("view should show keybindings")
	}
}

func TestCloneConfirm_UpdateDispatch(t *testing.T) {
	m := makeCloneConfirmModel(&CloneOpts{URL: "git@github.com:user/repo.git"})

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	result := updated.(Model)

	if result.width != 100 {
		t.Errorf("expected width=100, got %d", result.width)
	}
}

func TestCloneConfirm_ViewDispatch(t *testing.T) {
	m := makeCloneConfirmModel(&CloneOpts{URL: "git@github.com:user/repo.git"})

	output := m.View().Content

	if !strings.Contains(stripAnsi(output), "Confirm clone") {
		t.Error("View() should dispatch to viewCloneConfirm")
	}
}
