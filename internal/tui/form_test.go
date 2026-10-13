package tui

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/cursor"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

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

// The preview under a form names the directory about to be created. A long
// parent path must give way from the left: clipped at the terminal edge, the
// line lost the one part the user had typed.
func TestForms_DestinationPreviewKeepsItsTailOnANarrowTerminal(t *testing.T) {
	launch := "/home/someone/" + strings.Repeat("deeply/nested/", 6) + "launch"

	repoForm := repoNameModel(launch)
	repoForm.repo.nameInput.SetValue("demo")
	cloneForm := cloneInputModel(launch)
	cloneForm.repo.urlInput.SetValue("git@github.com:user/demo.git")

	for name, m := range map[string]Model{"create repository": repoForm, "clone": cloneForm} {
		t.Run(name, func(t *testing.T) {
			m.width = 60
			view := stripANSI(m.viewRepoName())
			if name == "clone" {
				view = stripANSI(m.viewCloneInput())
			}
			var preview string
			for _, line := range strings.Split(view, "\n") {
				if strings.Contains(line, "→") {
					preview = line
				}
			}
			if !strings.HasSuffix(preview, "/launch/demo") || !strings.Contains(preview, "…") {
				t.Errorf("preview = %q, want the head elided and the destination's tail kept", preview)
			}
			if width := ansi.StringWidth(preview); width > m.width {
				t.Errorf("preview is %d cells wide on a %d-cell terminal: %q", width, m.width, preview)
			}
		})
	}

	repoForm.width = 60
	header := strings.Split(stripANSI(repoForm.viewRepoOptions()), "\n")[2]
	if !strings.HasSuffix(header, "/launch/demo") || ansi.StringWidth(header) > repoForm.width {
		t.Errorf("options header = %q, want the destination's tail within %d cells", header, repoForm.width)
	}
}
