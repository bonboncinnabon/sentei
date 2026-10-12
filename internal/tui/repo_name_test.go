package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/config"
	"github.com/bonboncinnabon/sentei/internal/repo"
	"github.com/bonboncinnabon/sentei/internal/testutil/mock"
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
	if err := os.Mkdir(filepath.Join(tmp, "taken"), 0o755); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name     string
		repoName string
		location string
		wantErr  string
	}{
		{"empty name", "", tmp, "name is required"},
		{"name with space", "my repo", tmp, "cannot contain spaces"},
		{"name with a slash would nest", "org/repo", tmp, "not a path"},
		{"name that is a parent reference", "..", tmp, "not a path"},
		{"missing location", "myrepo", filepath.Join(tmp, "nope"), "location is not an existing directory"},
		{"destination exists", "taken", tmp, "directory already exists"},
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

// createRepoPath walks the form to the end and returns where the pipeline
// was told to put the repository.
func createRepoPath(t *testing.T, m Model, name, location string) string {
	t.Helper()
	m.runner = &stubRunner{responses: map[string]stubResponse{}}
	m.shell = &mock.Runner{}
	m.repo.nameInput.SetValue(name)
	m.repo.locationInput.SetValue(location)

	updated, _ := m.updateRepoName(tea.KeyPressMsg{Code: tea.KeyEnter})
	if updated.(Model).view != repoOptionsView {
		t.Fatalf("form did not advance: %q", updated.(Model).repo.validationErr)
	}
	updated, _ = updated.(Model).updateRepoOptions(tea.KeyPressMsg{Code: tea.KeyEnter})
	result, _ := drainRepoPipeline(t, updated.(Model))
	return result.(repo.CreateResult).RepoPath
}

// The Location field names the directory the repository is created in. The
// run used to take its parent instead, so an edited Location was ignored.
func TestCreateRepo_LandsInTheLocationField(t *testing.T) {
	launch, elsewhere := t.TempDir(), t.TempDir()

	got := createRepoPath(t, repoNameModel(launch), "myrepo", elsewhere)

	if want := filepath.Join(elsewhere, "myrepo"); got != want {
		t.Errorf("repository path = %q, want %q", got, want)
	}
}

// A relative Location reached the pipeline as a relative path and broke the
// run. It resolves against the directory sentei was opened on.
func TestCreateRepo_RelativeLocationResolvesAgainstTheLaunchDirectory(t *testing.T) {
	launch := t.TempDir()
	if err := os.Mkdir(filepath.Join(launch, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}

	got := createRepoPath(t, repoNameModel(launch), "myrepo", "sub")

	if want := filepath.Join(launch, "sub", "myrepo"); got != want {
		t.Errorf("repository path = %q, want %q", got, want)
	}
}

func TestCreateRepo_LocationExpandsHomeAndDefaultsToTheLaunchDirectory(t *testing.T) {
	launch, home := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	if err := os.Mkdir(filepath.Join(home, "code"), 0o755); err != nil {
		t.Fatal(err)
	}

	if got, want := createRepoPath(t, repoNameModel(launch), "myrepo", "~/code"), filepath.Join(home, "code", "myrepo"); got != want {
		t.Errorf("repository path = %q, want %q", got, want)
	}
	if got, want := createRepoPath(t, repoNameModel(launch), "other", ""), filepath.Join(launch, "other"); got != want {
		t.Errorf("repository path with an empty Location = %q, want %q", got, want)
	}
}

func TestUpdateRepoName_EnterValidAdvancesToOptions(t *testing.T) {
	tmp := t.TempDir()
	m := repoNameModel(tmp)
	m.shell = &mock.Runner{}
	m.repo.nameInput.SetValue("myrepo")

	updated, cmd := m.updateRepoName(tea.KeyPressMsg{Code: tea.KeyEnter})
	model := updated.(Model)

	if model.view != repoOptionsView {
		t.Fatalf("view = %d, want repoOptionsView", model.view)
	}
	if model.repo.validationErr != "" {
		t.Errorf("unexpected validation error %q", model.repo.validationErr)
	}
	if model.repo.publishGitHub {
		t.Error("options should reset to publish off")
	}
	if model.repo.optionsCursor != repoOptPublish {
		t.Errorf("cursor = %d, want repoOptPublish", model.repo.optionsCursor)
	}
	if cmd == nil {
		t.Error("expected the GitHub auth check command")
	}
}

// Location is the user's field. Typing or pasting the name must leave it
// alone; the destination preview is what follows the name.
func TestUpdateRepoName_NameEditsLeaveLocationAlone(t *testing.T) {
	m := repoNameModel("/repo")
	m.repo.locationInput.SetValue("/elsewhere")
	m.repo.validationErr = "repository name is required"

	updated, _ := m.updateRepoName(keyMsg("a"))
	updated, _ = updated.(Model).updateRepoName(tea.PasteMsg{Content: "bc"})
	model := updated.(Model)

	if got := model.repo.nameInput.Value(); got != "abc" {
		t.Errorf("name = %q, want abc", got)
	}
	if got := model.repo.locationInput.Value(); got != "/elsewhere" {
		t.Errorf("location = %q, want it untouched", got)
	}
	if model.repo.validationErr != "" {
		t.Errorf("an edit should clear the validation error, got %q", model.repo.validationErr)
	}
	if view := stripANSI(model.viewRepoName()); !strings.Contains(view, "→ /elsewhere/abc") {
		t.Errorf("view should preview the destination:\n%s", view)
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
