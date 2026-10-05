package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/config"
	"github.com/bonboncinnabon/sentei/internal/repo"
)

func cloneInputModel(repoPath string) Model {
	m := NewMenuModel(&stubRunner{responses: map[string]stubResponse{}}, nil, repoPath, &config.Config{}, repo.ContextNoRepo)
	m.view = cloneInputView
	m.width, m.height = 80, 24
	m.repo.urlInput.Focus()
	return m
}

func TestUpdateCloneInput_EscReturnsToMenu(t *testing.T) {
	m := cloneInputModel("/repo")

	updated, _ := m.updateCloneInput(tea.KeyPressMsg{Code: tea.KeyEsc})

	if updated.(Model).view != menuView {
		t.Error("esc should return to the menu")
	}
}

func TestUpdateCloneInput_TabSwitchesFields(t *testing.T) {
	m := cloneInputModel("/repo")

	updated, _ := m.updateCloneInput(tea.KeyPressMsg{Code: tea.KeyTab})
	model := updated.(Model)
	if model.repo.cloneFocusedField != 1 || !model.repo.cloneNameInput.Focused() || model.repo.urlInput.Focused() {
		t.Fatal("tab should move focus to the name field")
	}

	updated, _ = model.updateCloneInput(tea.KeyPressMsg{Code: tea.KeyTab})
	model = updated.(Model)
	if model.repo.cloneFocusedField != 0 || !model.repo.urlInput.Focused() {
		t.Fatal("second tab should move focus back to the URL field")
	}
}

func TestUpdateCloneInput_EnterEmptyURLRejected(t *testing.T) {
	m := cloneInputModel("/repo")

	updated, _ := m.updateCloneInput(tea.KeyPressMsg{Code: tea.KeyEnter})
	model := updated.(Model)

	if model.repo.validationErr != "repository URL is required" {
		t.Errorf("validationErr = %q, want URL-required error", model.repo.validationErr)
	}
	if model.view != cloneInputView {
		t.Error("validation failure must keep the clone input view")
	}
}

func TestUpdateCloneInput_EnterExistingDestinationRejected(t *testing.T) {
	tmp := t.TempDir()
	m := cloneInputModel(tmp)
	m.repo.urlInput.SetValue("git@github.com:user/myrepo.git")
	if err := os.Mkdir(filepath.Join(tmp, "myrepo"), 0o755); err != nil {
		t.Fatal(err)
	}

	updated, _ := m.updateCloneInput(tea.KeyPressMsg{Code: tea.KeyEnter})
	model := updated.(Model)

	if !strings.Contains(model.repo.validationErr, "already exists") {
		t.Errorf("validationErr = %q, want directory-exists error", model.repo.validationErr)
	}
}

func TestUpdateCloneInput_EnterStartsClonePipeline(t *testing.T) {
	tmp := t.TempDir()
	m := cloneInputModel(tmp)
	m.repo.urlInput.SetValue("git@github.com:user/myrepo.git")

	updated, cmd := m.updateCloneInput(tea.KeyPressMsg{Code: tea.KeyEnter})
	model := updated.(Model)

	if model.view != repoProgressView {
		t.Fatalf("view = %d, want repoProgressView", model.view)
	}
	if model.repo.opType != "clone" {
		t.Errorf("opType = %q, want clone", model.repo.opType)
	}
	if cmd == nil {
		t.Fatal("expected a command waiting for pipeline events")
	}

	result, _ := drainRepoPipeline(t, model)
	if _, ok := result.(repo.CloneResult); !ok {
		t.Errorf("pipeline result = %T, want repo.CloneResult", result)
	}
}

func TestUpdateCloneInput_TypingDerivesNameUntilManuallyEdited(t *testing.T) {
	m := cloneInputModel("/repo")

	updated, _ := m.updateCloneInput(keyMsg("x"))
	model := updated.(Model)
	if got := model.repo.cloneNameInput.Value(); got != "x" {
		t.Errorf("derived name = %q, want %q", got, "x")
	}

	// Move to the name field and edit it: derivation must stop.
	updated, _ = model.updateCloneInput(tea.KeyPressMsg{Code: tea.KeyTab})
	model = updated.(Model)
	updated, _ = model.updateCloneInput(keyMsg("y"))
	model = updated.(Model)
	if !model.repo.nameManuallyEdited {
		t.Fatal("editing the name field should mark it manually edited")
	}

	updated, _ = model.updateCloneInput(tea.KeyPressMsg{Code: tea.KeyTab})
	model = updated.(Model)
	updated, _ = model.updateCloneInput(keyMsg("z"))
	model = updated.(Model)
	if got := model.repo.cloneNameInput.Value(); got != "xy" {
		t.Errorf("name = %q, want %q (no re-derivation after manual edit)", got, "xy")
	}
}

func TestUpdateCloneInput_PasteDerivesName(t *testing.T) {
	m := cloneInputModel("/repo")
	m.repo.validationErr = "repository URL is required"

	updated, _ := m.updateCloneInput(tea.PasteMsg{Content: "git@github.com:user/myrepo.git"})
	model := updated.(Model)

	if got := model.repo.urlInput.Value(); got != "git@github.com:user/myrepo.git" {
		t.Errorf("URL = %q, want pasted URL", got)
	}
	if got := model.repo.cloneNameInput.Value(); got != "myrepo" {
		t.Errorf("derived name = %q, want myrepo", got)
	}
	if model.repo.validationErr != "" {
		t.Errorf("paste should clear validation error, got %q", model.repo.validationErr)
	}
}

func TestUpdateCloneInput_PasteMarksNameManuallyEdited(t *testing.T) {
	m := cloneInputModel("/repo")
	m.repo.urlInput.SetValue("git@github.com:user/original.git")
	m.repo.cloneFocusedField = 1
	m.repo.urlInput.Blur()
	m.repo.cloneNameInput.Focus()

	updated, _ := m.updateCloneInput(tea.PasteMsg{Content: "custom"})
	model := updated.(Model)

	if got := model.repo.cloneNameInput.Value(); got != "custom" {
		t.Errorf("clone name = %q, want custom", got)
	}
	if !model.repo.nameManuallyEdited {
		t.Fatal("pasting clone name must mark it manually edited")
	}
}

func TestViewCloneInput_RendersFieldsAndError(t *testing.T) {
	m := cloneInputModel("/repo")
	m.repo.urlInput.SetValue("git@github.com:user/myrepo.git")
	m.repo.validationErr = "repository URL is required"

	view := stripANSI(m.viewCloneInput())

	for _, want := range []string{"Clone repository", "Repository URL", "Clone to", "/repo/myrepo", "repository URL is required", "enter clone"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q:\n%s", want, view)
		}
	}
}

func TestViewCloneInput_NameFieldFocusedShowsDestination(t *testing.T) {
	m := cloneInputModel("/repo")
	m.repo.urlInput.SetValue("git@github.com:user/myrepo.git")
	m.repo.cloneFocusedField = 1
	m.repo.cloneNameInput.SetValue("custom")

	view := stripANSI(m.viewCloneInput())

	if !strings.Contains(view, "/repo/custom") {
		t.Errorf("view should show the destination for the custom name:\n%s", view)
	}
	if !strings.Contains(view, "git@github.com:user/myrepo.git") {
		t.Errorf("blurred URL field should show its value:\n%s", view)
	}
	if strings.Contains(view, "(empty)") {
		t.Errorf("non-empty URL must not render the (empty) placeholder:\n%s", view)
	}
}
