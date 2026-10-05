package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/cleanup"
	"github.com/bonboncinnabon/sentei/internal/config"
	"github.com/bonboncinnabon/sentei/internal/repo"
)

func makeCleanupConfirmModel(opts *cleanup.Options) Model {
	m := NewMenuModel(nil, nil, "/repo", &config.Config{}, repo.ContextBareRepo)
	m.view = cleanupConfirmView
	m.width = 80
	m.height = 24
	m.cleanupOpts = opts
	return m
}

func TestResolvedCleanupOpts_DefaultsToSafe(t *testing.T) {
	m := makeCleanupConfirmModel(nil)

	opts := m.resolvedCleanupOpts()
	if opts.Mode != cleanup.ModeSafe {
		t.Errorf("expected mode=safe, got %q", opts.Mode)
	}
	if opts.DryRun {
		t.Error("expected DryRun=false by default")
	}
}

func TestResolvedCleanupOpts_RespectsExplicitOpts(t *testing.T) {
	m := makeCleanupConfirmModel(&cleanup.Options{
		Mode:   cleanup.ModeAggressive,
		DryRun: true,
	})

	opts := m.resolvedCleanupOpts()
	if opts.Mode != cleanup.ModeAggressive {
		t.Errorf("expected mode=aggressive, got %q", opts.Mode)
	}
	if !opts.DryRun {
		t.Error("expected DryRun=true")
	}
}

func TestCleanupConfirmationVM_SafeMode(t *testing.T) {
	m := makeCleanupConfirmModel(nil)
	vm := m.cleanupConfirmationVM()

	if vm.Title != "Confirm cleanup" {
		t.Errorf("expected title 'Confirm cleanup', got %q", vm.Title)
	}

	output := stripAnsi(vm.View())
	if !strings.Contains(output, "safe") {
		t.Errorf("expected 'safe' in view, got:\n%s", output)
	}
	if !strings.Contains(output, "Dry run:") {
		t.Errorf("expected 'Dry run:' in view, got:\n%s", output)
	}
	if !strings.Contains(output, "sentei cleanup") {
		t.Errorf("expected CLI command in view, got:\n%s", output)
	}
}

func TestCleanupConfirmationVM_AggressiveWithDryRun(t *testing.T) {
	m := makeCleanupConfirmModel(&cleanup.Options{
		Mode:   cleanup.ModeAggressive,
		DryRun: true,
	})
	vm := m.cleanupConfirmationVM()

	output := stripAnsi(vm.View())
	if !strings.Contains(output, "aggressive") {
		t.Errorf("expected 'aggressive' in view, got:\n%s", output)
	}
	if !strings.Contains(output, "yes") {
		t.Errorf("expected dry run 'yes' in view, got:\n%s", output)
	}
	if !strings.Contains(output, "--dry-run") {
		t.Errorf("expected '--dry-run' in CLI command, got:\n%s", output)
	}
}

func TestUpdateCleanupConfirm_ProceedTransitionsToResult(t *testing.T) {
	m := makeCleanupConfirmModel(nil)

	updated, _ := m.updateCleanupConfirm(ConfirmProceedMsg{})
	result := updated.(Model)

	if result.view != cleanupResultView {
		t.Errorf("expected view=cleanupResultView, got %d", result.view)
	}
	if result.cleanupResult != nil {
		t.Error("expected cleanupResult to be nil initially")
	}
}

func TestUpdateCleanupConfirm_BackReturnsToMenu(t *testing.T) {
	m := makeCleanupConfirmModel(nil)

	updated, cmd := m.updateCleanupConfirm(ConfirmBackMsg{})
	result := updated.(Model)

	if result.view != menuView {
		t.Errorf("expected view=menuView, got %d", result.view)
	}
	if cmd != nil {
		t.Error("expected nil cmd when going back to menu")
	}
}

func TestUpdateCleanupConfirm_BackQuitsWhenLaunchedDirectly(t *testing.T) {
	m := makeCleanupConfirmModel(&cleanup.Options{Mode: cleanup.ModeSafe})

	_, cmd := m.updateCleanupConfirm(ConfirmBackMsg{})
	if cmd == nil {
		t.Fatal("expected quit cmd when launched directly with opts")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", msg)
	}
}

func TestUpdateCleanupConfirm_WindowSizeMsg(t *testing.T) {
	m := makeCleanupConfirmModel(nil)

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	result := updated.(Model)

	if result.width != 120 {
		t.Errorf("expected width=120, got %d", result.width)
	}
}

func TestUpdateCleanupConfirm_QuitKey(t *testing.T) {
	m := makeCleanupConfirmModel(nil)

	_, cmd := m.updateCleanupConfirm(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if cmd == nil {
		t.Fatal("expected quit cmd for q key")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", msg)
	}
}

func TestMenuCleanupTransitionsToPreview(t *testing.T) {
	m := NewMenuModel(nil, nil, "/repo", &config.Config{}, repo.ContextBareRepo)
	m.width = 80
	m.height = 24

	// Move cursor to "Cleanup & exit" (index 3 in bare repo menu).
	m.menuCursor = 3

	updated, cmd := m.updateMenu(tea.KeyPressMsg{Code: tea.KeyEnter})
	result := updated.(Model)

	if result.view != cleanupPreviewView {
		t.Errorf("expected view=cleanupPreviewView, got %d", result.view)
	}
	if cmd == nil {
		t.Error("expected the dry-run scan Cmd to fire on entry")
	}
	if result.cleanupScan != nil || result.cleanupAggressiveConfirm {
		t.Error("preview state must start pristine")
	}
}

// The CLI path (sentei cleanup --mode=X) keeps the confirmation view.
func TestSetCleanupOptsKeepsConfirmView(t *testing.T) {
	m := NewMenuModel(nil, nil, "/repo", &config.Config{}, repo.ContextBareRepo)
	opts := cleanup.Options{Mode: cleanup.ModeSafe}
	m.SetCleanupOpts(&opts)

	if m.view != cleanupConfirmView {
		t.Errorf("expected view=cleanupConfirmView for CLI path, got %d", m.view)
	}
}

func TestViewCleanupConfirm_RendersContent(t *testing.T) {
	m := makeCleanupConfirmModel(nil)

	output := stripAnsi(m.viewCleanupConfirm())

	if !strings.Contains(output, "Confirm cleanup") {
		t.Error("view should contain title")
	}
	if !strings.Contains(output, "Mode:") {
		t.Error("view should contain Mode label")
	}
	if !strings.Contains(output, "enter confirm") {
		t.Error("view should show keybindings")
	}
}

func TestSetCleanupOpts_SetsViewAndOpts(t *testing.T) {
	m := NewMenuModel(nil, nil, "/repo", &config.Config{}, repo.ContextBareRepo)

	opts := &cleanup.Options{Mode: cleanup.ModeAggressive, DryRun: true}
	m.SetCleanupOpts(opts)

	if m.view != cleanupConfirmView {
		t.Errorf("expected view=cleanupConfirmView, got %d", m.view)
	}
	if m.cleanupOpts != opts {
		t.Error("expected cleanupOpts to be set")
	}
}
