package tui

import (
	"testing"

	"charm.land/bubbles/v2/cursor"
	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/config"
	"github.com/bonboncinnabon/sentei/internal/repo"
	"github.com/bonboncinnabon/sentei/internal/testutil/mock"
)

// Messages that reach a form without changing its text: the virtual cursor's
// blink (every 530ms while a field is focused), mouse motion, and a key that
// only moves the cursor.
var nonEditMessages = map[string]tea.Msg{
	"cursor blink": cursor.BlinkMsg{},
	"mouse motion": tea.MouseMotionMsg{},
	"cursor key":   tea.KeyPressMsg{Code: tea.KeyLeft},
}

// A validation error describes the text in the form, so it stays until the
// text changes. Every message used to clear it, which erased the error with
// the next cursor blink, about half a second after it appeared.
func TestForms_ValidationErrorOutlivesMessagesThatAreNotEdits(t *testing.T) {
	forms := map[string]struct {
		model Model
		err   func(Model) string
	}{
		"create worktree":   {createBranchModel(), func(m Model) string { return m.create.validationErr }},
		"create repository": {repoNameModel("/repo"), func(m Model) string { return m.repo.validationErr }},
		"clone":             {cloneInputModel("/repo"), func(m Model) string { return m.repo.validationErr }},
	}
	for formName, form := range forms {
		for msgName, msg := range nonEditMessages {
			t.Run(formName+"/"+msgName, func(t *testing.T) {
				m := form.model
				m.create.validationErr, m.repo.validationErr = "still wrong", "still wrong"

				updated, _ := m.Update(msg)

				if got := form.err(updated.(Model)); got != "still wrong" {
					t.Errorf("validation error = %q after a message that changed nothing, want it kept", got)
				}
			})
		}
		t.Run(formName+"/typing clears it", func(t *testing.T) {
			m := form.model
			m.create.validationErr, m.repo.validationErr = "still wrong", "still wrong"

			updated, _ := m.Update(keyMsg("x"))

			if got := form.err(updated.(Model)); got != "" {
				t.Errorf("validation error = %q after an edit, want it cleared", got)
			}
		})
	}
}

// While the clone name field is focused, a blink used to count as typing a
// name: the name then stopped following the URL.
func TestCloneForm_BlinkOnTheNameFieldIsNotAManualEdit(t *testing.T) {
	m := cloneInputModel("/repo")

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyTab}) // to the name field
	updated, _ = updated.(Model).Update(cursor.BlinkMsg{})
	updated, _ = updated.(Model).Update(tea.KeyPressMsg{Code: tea.KeyTab}) // back to the URL
	updated, _ = updated.(Model).Update(tea.PasteMsg{Content: "git@github.com:user/myrepo.git"})
	model := updated.(Model)

	if model.repo.nameManuallyEdited {
		t.Error("nameManuallyEdited = true, but the name was never typed")
	}
	if got := model.repo.cloneNameInput.Value(); got != "myrepo" {
		t.Errorf("clone name = %q, want it derived from the URL", got)
	}
}

// Create repository, clone and migrate share one error field. Whatever one of
// them left there used to show up on the next screen opened from the menu.
func TestMenu_RepoFlowsOpenWithoutAnotherScreensError(t *testing.T) {
	menu := NewMenuModel(&mock.Runner{}, &mock.Runner{}, "/repo", &config.Config{}, repo.ContextNonBareRepo)
	for cursor, item := range menu.menuItems {
		t.Run(item.label, func(t *testing.T) {
			m := menu
			m.menuCursor = cursor
			m.repo.validationErr = "left by another screen"

			updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			model := updated.(Model)

			if model.view == menuView {
				t.Fatalf("%q did not open a screen", item.label)
			}
			if model.repo.validationErr != "" {
				t.Errorf("%q opened showing %q", item.label, model.repo.validationErr)
			}
		})
	}
}
