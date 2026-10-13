package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestModel_PasteRoutesFocusedFields(t *testing.T) {
	t.Run("create branch inserts at cursor without navigation", func(t *testing.T) {
		m := createBranchModel()
		m.create.branchInput.SetValue("start")
		m.create.branchInput.SetCursor(2)

		updated, _ := m.Update(tea.PasteMsg{Content: "q/enter"})
		model := updated.(Model)

		if got := model.create.branchInput.Value(); got != "stq/enterart" {
			t.Errorf("branch = %q, want cursor insertion", got)
		}
		if model.view != createBranchView {
			t.Errorf("view = %d, want createBranchView", model.view)
		}
	})

	t.Run("repository name leaves the location field alone", func(t *testing.T) {
		m := repoNameModel("/repo")

		updated, _ := m.Update(tea.PasteMsg{Content: "project"})
		model := updated.(Model)

		if got := model.repo.nameInput.Value(); got != "project" {
			t.Errorf("name = %q, want project", got)
		}
		if got := model.repo.locationInput.Value(); got != "/repo" {
			t.Errorf("location = %q, want the launch directory untouched", got)
		}
	})

	t.Run("clone URL preserves name derivation", func(t *testing.T) {
		m := cloneInputModel("/repo")

		updated, _ := m.Update(tea.PasteMsg{Content: "https://github.com/user/project.git"})
		model := updated.(Model)

		if got := model.repo.cloneNameInput.Value(); got != "project" {
			t.Errorf("clone name = %q, want project", got)
		}
	})

	t.Run("active filter reindexes", func(t *testing.T) {
		m := makeRemoveModel(sampleWorktrees())
		m.view = listView
		m.remove.filterActive = true
		m.remove.filterInput.Focus()

		updated, _ := m.Update(tea.PasteMsg{Content: "alpha"})
		model := updated.(Model)

		if got := len(model.remove.visibleIndices); got != 1 {
			t.Errorf("visible worktrees = %d, want 1", got)
		}
	})

	t.Run("visible description receives paste", func(t *testing.T) {
		m := repoOptionsModel(t)
		m.repo.publishGitHub = true
		m.repo.optionsCursor = repoOptDescription
		m.repo.descInput.Focus()

		updated, _ := m.Update(tea.PasteMsg{Content: "description"})
		model := updated.(Model)

		if got := model.repo.descInput.Value(); got != "description" {
			t.Errorf("description = %q, want pasted value", got)
		}
	})
}

func TestModel_CtrlVPasteCommand(t *testing.T) {
	m := createBranchModel()

	updated, cmd := m.Update(tea.KeyPressMsg{Code: 'v', Mod: tea.ModCtrl})
	model := updated.(Model)

	if cmd == nil {
		t.Fatal("focused text input must return its clipboard command")
	}
	if model.view != createBranchView {
		t.Fatalf("view = %d, want createBranchView", model.view)
	}
}
