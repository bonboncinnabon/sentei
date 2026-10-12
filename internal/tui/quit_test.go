package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/repo"
)

var keyCtrlC = tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}

// lastView is the highest viewState; the every-view tests walk up to it.
const lastView = cloneConfirmView

func quits(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

// quitTestModel is a menu model with one selected worktree, showing view.
func quitTestModel(view viewState) Model {
	m := NewMenuModel(nil, nil, "/repo", nil, repo.ContextBareRepo)
	m.remove.worktrees = []git.Worktree{{Path: "/repo/a", Branch: "refs/heads/a"}}
	m.remove.selected = map[string]bool{"/repo/a": true}
	m.reindex()
	m.view = view
	return m
}

func TestCtrlC_QuitsFromEveryView(t *testing.T) {
	for view := menuView; view <= lastView; view++ {
		m := quitTestModel(view)
		if _, cmd := m.Update(keyCtrlC); !quits(cmd) {
			t.Errorf("view %d: ctrl+c did not quit", view)
		}
	}
}

func TestCtrlC_QuitsFromTextInputsAndOverlays(t *testing.T) {
	filtering := quitTestModel(listView)
	filtering.remove.filterActive = true
	filtering.remove.filterInput.Focus()

	description := quitTestModel(repoOptionsView)
	description.repo.publishGitHub = true
	description.repo.optionsCursor = 3
	description.repo.descInput.Focus()

	help := quitTestModel(createBranchView)
	help.portal = help.portal.SetSize(80, 24).Open(portalHelp, "Help", "body")

	for name, m := range map[string]Model{
		"list filter input":       filtering,
		"repository description":  description,
		"help portal over a form": help,
	} {
		if _, cmd := m.Update(keyCtrlC); !quits(cmd) {
			t.Errorf("%s: ctrl+c did not quit", name)
		}
	}
}
