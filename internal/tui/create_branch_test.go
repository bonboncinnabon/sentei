package tui

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/config"
	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/repo"
	"github.com/bonboncinnabon/sentei/internal/testutil/mock"
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

// knowRefs makes the model's git answer for the given refs, so a branch
// exists locally (refs/heads/...) or only on origin (refs/remotes/origin/...).
func knowRefs(m Model, refs ...string) Model {
	runner := m.runner.(*mock.Runner)
	for _, ref := range refs {
		runner.Responses["/repo:[show-ref --verify "+ref+"]"] = mock.Response{}
	}
	return m
}

// feed sends the messages to the model one after another.
func feed(m Model, msgs ...tea.Msg) Model {
	var model tea.Model = m
	for _, msg := range msgs {
		model, _ = model.Update(msg)
	}
	return model.(Model)
}

// Opening a worktree for a branch that already exists must not merge the base
// into it unless the user asks: that puts a merge commit on a branch they only
// wanted to check out. A new branch keeps the merge on.
func TestCreateOptions_MergeStartsUncheckedForABranchThatExists(t *testing.T) {
	cases := map[string]struct {
		ref       string
		wantMerge bool
	}{
		"local branch":       {"refs/heads/feature/there", false},
		"branch only origin": {"refs/remotes/origin/feature/there", false},
		"new branch":         {"refs/heads/some/other", true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			m, _ := unloadedWorktreesModel(t)
			m = feed(knowRefs(m, tc.ref),
				tea.KeyPressMsg{Code: tea.KeyEnter}, // menu: Create new worktree
				tea.KeyPressMsg{Text: "feature/there"},
				tea.KeyPressMsg{Code: tea.KeyEnter}, // to the options screen
			)

			if got := m.buildCreatorOptions().MergeBase; got != tc.wantMerge {
				t.Errorf("MergeBase = %v, want %v", got, tc.wantMerge)
			}
			row := "[ ] Merge default branch"
			if tc.wantMerge {
				row = "[x] Merge default branch"
			}
			if view := stripANSI(m.viewCreateOptions()); !strings.Contains(view, row) {
				t.Errorf("options screen does not show %q:\n%s", row, view)
			}
		})
	}
}

// The unchecked row is a default, not a lock. A merge the user turned on stays
// on when they come back with the same branch, and does not carry over to a
// different branch that also exists.
func TestCreateOptions_MergeChoiceBelongsToTheBranchItWasMadeFor(t *testing.T) {
	m, _ := unloadedWorktreesModel(t)
	m = feed(knowRefs(m, "refs/heads/feature/there", "refs/heads/feature/there-too"),
		tea.KeyPressMsg{Code: tea.KeyEnter}, // menu: Create new worktree
		tea.KeyPressMsg{Text: "feature/there"},
		tea.KeyPressMsg{Code: tea.KeyEnter}, // to the options screen
		tea.KeyPressMsg{Code: tea.KeyDown},  // uv -> go
		tea.KeyPressMsg{Code: tea.KeyDown},  // go -> merge
		keyMsg(" "),                         // check the merge row
	)
	if !m.buildCreatorOptions().MergeBase {
		t.Fatal("the merge row could not be checked for an existing branch")
	}

	m = feed(m, tea.KeyPressMsg{Code: tea.KeyEsc}, tea.KeyPressMsg{Code: tea.KeyEnter})
	if !m.buildCreatorOptions().MergeBase {
		t.Error("MergeBase = false after returning with the same branch, want the user's choice kept")
	}

	m = feed(m, tea.KeyPressMsg{Code: tea.KeyEsc}, tea.KeyPressMsg{Text: "-too"}, tea.KeyPressMsg{Code: tea.KeyEnter})
	if got := m.create.branchInput.Value(); got != "feature/there-too" {
		t.Fatalf("branch = %q, want feature/there-too", got)
	}
	if m.buildCreatorOptions().MergeBase {
		t.Error("MergeBase = true for a different existing branch: the choice made for the first one carried over")
	}
}

// Entering the flow from the menu again starts over: the same existing branch
// typed a second time must get the merge turned off a second time.
func TestCreateOptions_MergeDefaultIsDecidedAgainOnEachVisitFromTheMenu(t *testing.T) {
	m, _ := unloadedWorktreesModel(t)
	visit := []tea.Msg{
		tea.KeyPressMsg{Code: tea.KeyEnter}, // menu: Create new worktree
		tea.KeyPressMsg{Text: "feature/there"},
		tea.KeyPressMsg{Code: tea.KeyEnter}, // to the options screen
	}
	m = feed(knowRefs(m, "refs/heads/feature/there"), visit...)
	m = feed(m, tea.KeyPressMsg{Code: tea.KeyEsc}, tea.KeyPressMsg{Code: tea.KeyEsc}) // back to the menu
	if m.view != menuView {
		t.Fatalf("view = %d, want menuView", m.view)
	}

	m = feed(m, visit...)

	if m.view != createOptionsView || m.buildCreatorOptions().MergeBase {
		t.Errorf("view=%d MergeBase=%v on the second visit, want the options screen with the merge off", m.view, m.buildCreatorOptions().MergeBase)
	}
}

// Quick create shows no options screen, so the default is the whole decision.
func TestUpdateCreateBranch_QuickCreateDoesNotMergeIntoAnExistingBranch(t *testing.T) {
	m, _ := unloadedWorktreesModel(t)
	m = feed(knowRefs(m, "refs/remotes/origin/feature/there"),
		tea.KeyPressMsg{Code: tea.KeyEnter}, // menu: Create new worktree
		tea.KeyPressMsg{Text: "feature/there"},
		tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl},
	)

	if m.view != createProgressView {
		t.Fatalf("view = %d, want createProgressView", m.view)
	}
	if m.buildCreatorOptions().MergeBase {
		t.Error("quick create merges the base into a branch that already exists on origin")
	}
}

// directCreateRepo is a real repository as a direct `sentei create` finds it:
// a worktree named taken, a branch local-only with no worktree, and a branch
// remote-only that only origin has. The model's worktree list is not loaded.
func directCreateRepo(t *testing.T) Model {
	t.Helper()
	bare := bareRepoWithWorktrees(t, "taken")
	runTestGit(t, bare, "branch", "local-only", "main")
	runTestGit(t, bare, "update-ref", "refs/remotes/origin/remote-only", "main")
	m := NewMenuModel(&git.GitRunner{}, nil, bare, &config.Config{}, repo.ContextBareRepo)
	m.width, m.height = 100, 24
	m.view = createBranchView
	return m
}

// The worktree list loads in the background. A duplicate submitted before it
// lands used to pass the form and fail in git one screen later.
func TestUpdateCreateBranch_RefusesExistingWorktreeBeforeTheListLoads(t *testing.T) {
	for name, key := range map[string]tea.KeyPressMsg{
		"enter":      {Code: tea.KeyEnter},
		"ctrl+enter": {Code: tea.KeyEnter, Mod: tea.ModCtrl},
	} {
		m := directCreateRepo(t)
		m.create.branchInput.SetValue("taken")

		updated, cmd := m.updateCreateBranch(key)
		model := updated.(Model)

		if model.view != createBranchView || cmd != nil {
			t.Errorf("%s: the form advanced to view %d for a worktree that exists", name, model.view)
		}
		if !strings.Contains(model.create.validationErr, `worktree "taken" already exists`) {
			t.Errorf("%s: validationErr = %q, want the duplicate named", name, model.create.validationErr)
		}
	}
}

// The base only starts a branch that exists nowhere. The line under the title
// must not say "from <base>" for a branch that is checked out as it is.
func TestCreateFlow_SubtitleSaysWhereTheBranchComesFrom(t *testing.T) {
	for branch, want := range map[string]string{
		"brand-new":   "brand-new → from main",
		"local-only":  "local-only → existing branch, not created from main",
		"remote-only": "remote-only → tracks origin/remote-only, not created from main",
	} {
		m := directCreateRepo(t)
		m.create.branchInput.SetValue(branch)
		updated, _ := m.updateCreateBranch(tea.KeyPressMsg{Code: tea.KeyEnter})
		model := updated.(Model)
		if model.view != createOptionsView {
			t.Fatalf("%s: view = %d, want the options screen: %q", branch, model.view, model.create.validationErr)
		}

		options := stripANSI(model.viewCreateOptions())
		if !strings.Contains(options, "  "+want+"\n") {
			t.Errorf("%s: options screen does not say %q:\n%s", branch, want, options)
		}
		if got := model.createLayout().Subtitle; got != want {
			t.Errorf("%s: progress subtitle = %q, want %q", branch, got, want)
		}
	}
}

// `sentei create --branch <b> --base <base>` skips the options screen, so its
// confirmation is where an unused base has to be stated.
func TestCreateConfirm_SaysWhenTheBaseIsNotUsed(t *testing.T) {
	for branch, want := range map[string]string{
		"brand-new":   "Branch:      brand-new\n",
		"local-only":  "Branch:      local-only (existing branch, not created from main)\n",
		"remote-only": "Branch:      remote-only (tracks origin/remote-only, not created from main)\n",
	} {
		m := directCreateRepo(t)
		m.SetCreateOpts(&CreateOpts{Branch: branch, Base: "main"})

		view := stripANSI(m.viewCreateConfirm())
		if m.view != createConfirmView || !strings.Contains(view, want) {
			t.Errorf("%s: confirmation does not contain %q:\n%s", branch, want, view)
		}
	}
}
