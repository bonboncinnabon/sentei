package tui

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/config"
	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/repo"
)

func createBranchModel() Model {
	m := NewMenuModel(bareDirRunner("/repo"), nil, "/repo", &config.Config{}, repo.ContextBareRepo)
	m.width, m.height = 80, 24
	m.view = createBranchView
	m.remove.worktrees = []git.Worktree{{Path: "/repo/feature-taken", Branch: "refs/heads/feature/taken"}}
	return m
}

func TestUpdateCreateBranch_TypingFillsFocusedField(t *testing.T) {
	m := createBranchModel()

	updated, _ := m.updateCreateBranch(tea.KeyPressMsg{Text: "feature/new"})
	model := updated.(Model)

	if got := model.create.branchInput.Value(); got != "feature/new" {
		t.Errorf("branch input = %q, want %q", got, "feature/new")
	}
	if model.create.baseInput.Value() != defaultBaseBranch {
		t.Errorf("base input should be untouched, got %q", model.create.baseInput.Value())
	}
}

func TestUpdateCreateBranch_PasteFillsFocusedField(t *testing.T) {
	m := createBranchModel()
	m.create.validationErr = "branch name is required"

	updated, _ := m.updateCreateBranch(tea.PasteMsg{Content: "feature/界"})
	model := updated.(Model)

	if got := model.create.branchInput.Value(); got != "feature/界" {
		t.Errorf("branch input = %q, want %q", got, "feature/界")
	}
	if got := model.create.baseInput.Value(); got != defaultBaseBranch {
		t.Errorf("base input = %q, want untouched default", got)
	}
	if model.create.validationErr != "" {
		t.Errorf("paste should clear validation error, got %q", model.create.validationErr)
	}
}

func TestUpdateCreateBranch_PasteSanitizesFocusedBaseField(t *testing.T) {
	m := createBranchModel()
	m.create.focusedField = 1
	m.create.branchInput.Blur()
	m.create.baseInput.Focus()
	m.create.baseInput.SetValue("")

	updated, _ := m.updateCreateBranch(tea.PasteMsg{Content: "ma\n\tin\x1b界"})
	model := updated.(Model)

	if got := model.create.baseInput.Value(); got != "ma  in界" {
		t.Errorf("base input = %q, want sanitized %q", got, "ma  in界")
	}
	if got := model.create.branchInput.Value(); got != "" {
		t.Errorf("branch input = %q, want untouched", got)
	}
}

func TestUpdateCreateBranch_TabSwitchesFields(t *testing.T) {
	m := createBranchModel()

	updated, _ := m.updateCreateBranch(tea.KeyPressMsg{Code: tea.KeyTab})
	model := updated.(Model)
	if model.create.focusedField != 1 || !model.create.baseInput.Focused() || model.create.branchInput.Focused() {
		t.Fatal("tab should move focus to the base field")
	}

	updated, _ = model.updateCreateBranch(tea.KeyPressMsg{Code: tea.KeyTab})
	model = updated.(Model)
	if model.create.focusedField != 0 || !model.create.branchInput.Focused() {
		t.Fatal("second tab should move focus back to the branch field")
	}
}

func TestUpdateCreateBranch_EnterValidates(t *testing.T) {
	cases := []struct {
		name     string
		branch   string
		wantView viewState
		wantErr  bool
	}{
		{"valid advances to options", "feature/new", createOptionsView, false},
		{"empty stays with error", "", createBranchView, true},
		{"duplicate worktree stays with error", "feature/taken", createBranchView, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := createBranchModel()
			m.create.branchInput.SetValue(tc.branch)

			updated, _ := m.updateCreateBranch(tea.KeyPressMsg{Code: tea.KeyEnter})
			model := updated.(Model)

			if model.view != tc.wantView {
				t.Errorf("view = %d, want %d", model.view, tc.wantView)
			}
			if tc.wantErr && model.create.validationErr == "" {
				t.Error("expected a validation error")
			}
			if !tc.wantErr && model.create.validationErr != "" {
				t.Errorf("unexpected validation error %q", model.create.validationErr)
			}
		})
	}
}

func TestUpdateCreateBranch_QuickCreateSkipsOptions(t *testing.T) {
	m := createBranchModel()
	m.create.branchInput.SetValue("feature/new")

	updated, cmd := m.updateCreateBranch(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	model := updated.(Model)

	if model.view != createProgressView {
		t.Errorf("view = %d, want createProgressView (quick create skips options)", model.view)
	}
	if cmd == nil {
		t.Error("quick create should return the creation event wait Cmd")
	}
}

func TestUpdateCreateBranch_QuickCreateValidates(t *testing.T) {
	m := createBranchModel()
	m.create.branchInput.SetValue("")

	updated, _ := m.updateCreateBranch(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	model := updated.(Model)

	if model.view != createBranchView {
		t.Error("invalid branch must stay on the input view")
	}
	if model.create.validationErr == "" {
		t.Error("expected a validation error")
	}
}

func TestUpdateCreateBranch_EscReturnsToMenu(t *testing.T) {
	m := createBranchModel()
	updated, _ := m.updateCreateBranch(tea.KeyPressMsg{Code: tea.KeyEsc})
	if updated.(Model).view != menuView {
		t.Error("esc should return to the menu")
	}
}

func TestUpdateCreateBranch_TypingClearsValidationError(t *testing.T) {
	m := createBranchModel()
	m.create.validationErr = "branch name is required"

	updated, _ := m.updateCreateBranch(tea.KeyPressMsg{Code: 'f', Text: "f"})
	if got := updated.(Model).create.validationErr; got != "" {
		t.Errorf("typing should clear the validation error, got %q", got)
	}
}

func TestViewCreateBranch_RendersChromeFieldsAndError(t *testing.T) {
	m := createBranchModel()
	m.create.branchInput.SetValue("feat x")
	m.create.validationErr = "branch name cannot contain spaces"

	view := stripANSI(m.viewCreateBranch())
	for _, want := range []string{"sentei ─ Create worktree", "Branch name", "Base branch", "branch name cannot contain spaces", "enter continue"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q:\n%s", want, view)
		}
	}
}

// The menu loads the worktree list in the background. Choosing "Create new
// worktree" before it landed used to find no source worktree, so the options
// screen offered no install or env rows and the run skipped both.
func TestCreateFromMenu_BeforeWorktreesLoad_StillOffersInstallAndEnvRows(t *testing.T) {
	m, source := unloadedWorktreesModel(t)

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter}) // "Create new worktree"
	updated, _ = updated.(Model).Update(tea.KeyPressMsg{Text: "feature/early"})
	updated, _ = updated.(Model).Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model := updated.(Model)

	if model.view != createOptionsView {
		t.Fatalf("view = %d, want createOptionsView", model.view)
	}
	view := stripANSI(model.viewCreateOptions())
	for _, want := range []string{"Install dependencies (uv)", "Copy environment files"} {
		if !strings.Contains(view, want) {
			t.Errorf("options screen missing %q:\n%s", want, view)
		}
	}
	if got := model.buildCreatorOptions().SourceWorktree; got != source {
		t.Errorf("SourceWorktree = %q, want %q", got, source)
	}
}

// Leaving the branch form detects ecosystems again. A row the user unchecked
// before going back must not come back checked.
func TestCreateOptions_UncheckedRowSurvivesAReturnToTheBranchForm(t *testing.T) {
	m, _ := unloadedWorktreesModel(t)

	steps := []tea.Msg{
		tea.KeyPressMsg{Code: tea.KeyEnter}, // menu: Create new worktree
		tea.KeyPressMsg{Text: "feature/back"},
		tea.KeyPressMsg{Code: tea.KeyEnter}, // to the options screen
		keyMsg(" "),                         // uncheck the first row (uv)
		tea.KeyPressMsg{Code: tea.KeyEsc},   // back to the branch form
		tea.KeyPressMsg{Code: tea.KeyEnter}, // forward again
	}
	var model tea.Model = m
	for _, step := range steps {
		model, _ = model.Update(step)
	}

	if got := installNames(model.(Model).buildCreatorOptions().Ecosystems); !slices.Equal(got, []string{"go"}) {
		t.Errorf("installs = %v, want [go]: uv was unchecked before going back", got)
	}
}
