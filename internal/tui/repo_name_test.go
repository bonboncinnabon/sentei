package tui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/config"
	"github.com/bonboncinnabon/sentei/internal/repo"
)

func repoNameModel(repoPath string) Model {
	m := NewMenuModel(nil, nil, repoPath, &config.Config{}, repo.ContextNoRepo)
	m.view = repoNameView
	m.width, m.height = 80, 24
	m.repo.nameInput.Focus()
	return m
}

func TestUpdateRepoName_EscReturnsToMenu(t *testing.T) {
	m := repoNameModel("/repo")

	updated, _ := m.updateRepoName(tea.KeyPressMsg{Code: tea.KeyEsc})

	if updated.(Model).view != menuView {
		t.Error("esc should return to the menu")
	}
}

func TestUpdateRepoName_TabSwitchesFields(t *testing.T) {
	m := repoNameModel("/repo")

	updated, _ := m.updateRepoName(tea.KeyPressMsg{Code: tea.KeyTab})
	model := updated.(Model)
	if model.repo.focusedField != 1 || !model.repo.locationInput.Focused() || model.repo.nameInput.Focused() {
		t.Fatal("tab should move focus to the location field")
	}

	updated, _ = model.updateRepoName(tea.KeyPressMsg{Code: tea.KeyTab})
	model = updated.(Model)
	if model.repo.focusedField != 0 || !model.repo.nameInput.Focused() {
		t.Fatal("second tab should move focus back to the name field")
	}
}

func TestUpdateRepoName_EnterValidates(t *testing.T) {
	tmp := t.TempDir()
	cases := []struct {
		name     string
		repoName string
		location string
		wantErr  string
	}{
		{"empty name", "", tmp, "name is required"},
		{"name with space", "my repo", tmp, "cannot contain spaces"},
		{"empty location", "myrepo", "", "location is required"},
		{"missing parent dir", "myrepo", filepath.Join(tmp, "nope", "child"), "parent directory does not exist"},
		{"destination exists", "myrepo", tmp, "directory already exists"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := repoNameModel(tmp)
			m.repo.nameInput.SetValue(tc.repoName)
			m.repo.locationInput.SetValue(tc.location)

			updated, _ := m.updateRepoName(tea.KeyPressMsg{Code: tea.KeyEnter})
			model := updated.(Model)

			if !strings.Contains(model.repo.validationErr, tc.wantErr) {
				t.Errorf("validationErr = %q, want it to contain %q", model.repo.validationErr, tc.wantErr)
			}
			if model.view != repoNameView {
				t.Error("validation failure must keep the name view")
			}
		})
	}
}

func TestUpdateRepoName_EnterValidAdvancesToOptions(t *testing.T) {
	tmp := t.TempDir()
	m := repoNameModel(tmp)
	m.repo.nameInput.SetValue("myrepo")
	m.repo.locationInput.SetValue(filepath.Join(tmp, "myrepo"))

	updated, cmd := m.updateRepoName(tea.KeyPressMsg{Code: tea.KeyEnter})
	model := updated.(Model)

	if model.view != repoOptionsView {
		t.Fatalf("view = %d, want repoOptionsView", model.view)
	}
	if model.repo.validationErr != "" {
		t.Errorf("unexpected validation error %q", model.repo.validationErr)
	}
	if !model.repo.createWorktree || model.repo.publishGitHub {
		t.Error("options should reset to worktree on, publish off")
	}
	if model.repo.optionsCursor != repoOptPublish {
		t.Errorf("cursor = %d, want repoOptPublish", model.repo.optionsCursor)
	}
	if cmd == nil {
		t.Error("expected the GitHub auth check command")
	}
}

func TestUpdateRepoName_TypingAutoFillsLocation(t *testing.T) {
	m := repoNameModel("/repo")

	updated, _ := m.updateRepoName(keyMsg("a"))
	model := updated.(Model)
	if got := model.repo.locationInput.Value(); got != filepath.Join("/repo", "a") {
		t.Errorf("location = %q, want %q", got, "/repo/a")
	}

	updated, _ = model.updateRepoName(tea.KeyPressMsg{Code: tea.KeyBackspace})
	model = updated.(Model)
	if got := model.repo.locationInput.Value(); got != "/repo" {
		t.Errorf("location after clearing name = %q, want %q", got, "/repo")
	}
}

func TestUpdateRepoName_PasteAutoFillsLocation(t *testing.T) {
	m := repoNameModel("/repo")
	m.repo.validationErr = "repository name is required"

	updated, _ := m.updateRepoName(tea.PasteMsg{Content: "myrepo"})
	model := updated.(Model)

	if got := model.repo.nameInput.Value(); got != "myrepo" {
		t.Errorf("name = %q, want myrepo", got)
	}
	if got := model.repo.locationInput.Value(); got != filepath.Join("/repo", "myrepo") {
		t.Errorf("location = %q, want derived path", got)
	}
	if model.repo.validationErr != "" {
		t.Errorf("paste should clear validation error, got %q", model.repo.validationErr)
	}
}

func TestUpdateRepoName_PasteEditsFocusedLocationOnly(t *testing.T) {
	m := repoNameModel("/repo")
	m.repo.nameInput.SetValue("myrepo")
	m.repo.focusedField = 1
	m.repo.nameInput.Blur()
	m.repo.locationInput.SetValue("")
	m.repo.locationInput.Focus()

	updated, _ := m.updateRepoName(tea.PasteMsg{Content: "/tmp/myrepo"})
	model := updated.(Model)

	if got := model.repo.locationInput.Value(); got != "/tmp/myrepo" {
		t.Errorf("location = %q, want pasted path", got)
	}
	if got := model.repo.nameInput.Value(); got != "myrepo" {
		t.Errorf("name = %q, want untouched", got)
	}
}

func TestViewRepoName_RendersFieldsAndError(t *testing.T) {
	m := repoNameModel("/repo")
	m.repo.validationErr = "repository name is required"

	view := stripANSI(m.viewRepoName())

	for _, want := range []string{"Create repository", "Repository name", "Location", "/repo", "repository name is required", "enter continue"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q:\n%s", want, view)
		}
	}
}

func TestViewRepoName_BlurredEmptyShowsInputPlaceholder(t *testing.T) {
	m := repoNameModel("/repo")
	m.repo.focusedField = 1
	m.repo.nameInput.Blur()
	m.repo.locationInput.Focus()

	view := stripANSI(m.viewRepoName())

	if strings.Contains(view, "(empty)") {
		t.Errorf("fields render persistently now; (empty) is retired:\n%s", view)
	}
	if !strings.Contains(view, "my-project") {
		t.Errorf("blurred empty field should show its placeholder:\n%s", view)
	}
}
