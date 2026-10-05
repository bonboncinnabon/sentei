package tui

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/config"
	"github.com/bonboncinnabon/sentei/internal/repo"
	"github.com/bonboncinnabon/sentei/internal/testutil/mock"
)

func repoOptionsModel(t *testing.T) Model {
	t.Helper()
	tmp := t.TempDir()
	m := NewMenuModel(&stubRunner{responses: map[string]stubResponse{}}, nil, tmp, &config.Config{}, repo.ContextNoRepo)
	m.view = repoOptionsView
	m.width, m.height = 80, 24
	m.repo.nameInput.SetValue("myrepo")
	m.repo.locationInput.SetValue(filepath.Join(tmp, "myrepo"))
	m.repo.createWorktree = true
	m.repo.optionsCursor = repoOptPublish
	return m
}

func TestCheckGitHubAuth_StatusFromShellOutcome(t *testing.T) {
	cases := []struct {
		name       string
		response   mock.Response
		wantStatus string
	}{
		{"authenticated", mock.Response{Output: "Logged in"}, "authenticated"},
		{"gh missing", mock.Response{Err: errors.New(`exec: "gh": executable file not found in $PATH`)}, "gh not found"},
		{"not logged in", mock.Response{Err: errors.New("you are not logged into any GitHub hosts")}, "not authenticated"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			shell := &mock.Runner{Responses: map[string]mock.Response{
				".:shell[gh auth status]": tc.response,
			}}

			msg := checkGitHubAuth(shell)()

			status, ok := msg.(ghAuthStatusMsg)
			if !ok {
				t.Fatalf("expected ghAuthStatusMsg, got %T", msg)
			}
			if status.status != tc.wantStatus {
				t.Errorf("status = %q, want %q", status.status, tc.wantStatus)
			}
		})
	}
}

func TestRepoVisibleOptions_PublishExpandsList(t *testing.T) {
	m := repoOptionsModel(t)

	if got := m.repoVisibleOptions(); len(got) != 2 {
		t.Errorf("visible options without publish = %v, want worktree+publish", got)
	}

	m.repo.publishGitHub = true
	got := m.repoVisibleOptions()
	if len(got) != 4 || got[2] != repoOptVisibility || got[3] != repoOptDescription {
		t.Errorf("visible options with publish = %v, want all four", got)
	}
}

func TestUpdateRepoOptions_GhAuthStatusStored(t *testing.T) {
	m := repoOptionsModel(t)

	updated, _ := m.updateRepoOptions(ghAuthStatusMsg{status: "authenticated"})

	if updated.(Model).repo.ghStatus != "authenticated" {
		t.Error("ghAuthStatusMsg should store the status")
	}
}

func TestUpdateRepoOptions_NavigationFocusesDescription(t *testing.T) {
	m := repoOptionsModel(t)
	m.repo.publishGitHub = true

	updated, _ := m.updateRepoOptions(keyMsg("j"))
	model := updated.(Model)
	if model.repo.optionsCursor != repoOptVisibility {
		t.Fatalf("cursor = %d, want repoOptVisibility", model.repo.optionsCursor)
	}

	updated, cmd := model.updateRepoOptions(keyMsg("j"))
	model = updated.(Model)
	if model.repo.optionsCursor != repoOptDescription {
		t.Fatalf("cursor = %d, want repoOptDescription", model.repo.optionsCursor)
	}
	if cmd == nil || !model.repo.descInput.Focused() {
		t.Error("moving onto the description must focus its input")
	}

	updated, _ = model.updateRepoOptions(keyMsg("k"))
	model = updated.(Model)
	if model.repo.descInput.Focused() {
		t.Error("moving off the description must blur its input")
	}
	if model.repo.optionsCursor != repoOptVisibility {
		t.Errorf("cursor = %d, want repoOptVisibility", model.repo.optionsCursor)
	}
}

func TestUpdateRepoOptions_TogglePublishRequiresAuth(t *testing.T) {
	m := repoOptionsModel(t)
	m.repo.ghStatus = "not authenticated"

	updated, _ := m.updateRepoOptions(keyMsg(" "))
	if updated.(Model).repo.publishGitHub {
		t.Fatal("publish must not toggle on without authentication")
	}

	m.repo.ghStatus = "authenticated"
	updated, _ = m.updateRepoOptions(keyMsg(" "))
	model := updated.(Model)
	if !model.repo.publishGitHub {
		t.Fatal("publish should toggle on when authenticated")
	}

	// Toggling off resets the cursor and blurs the description.
	model.repo.optionsCursor = repoOptPublish
	model.repo.descInput.Focus()
	updated, _ = model.updateRepoOptions(keyMsg(" "))
	model = updated.(Model)
	if model.repo.publishGitHub || model.repo.descInput.Focused() {
		t.Error("toggling publish off should reset publish state and blur the description")
	}
}

func TestUpdateRepoOptions_ToggleWorktreeAndVisibility(t *testing.T) {
	m := repoOptionsModel(t)
	m.repo.optionsCursor = repoOptWorktree

	updated, _ := m.updateRepoOptions(keyMsg(" "))
	if updated.(Model).repo.createWorktree {
		t.Error("space should toggle the worktree option off")
	}

	m.repo.publishGitHub = true
	m.repo.optionsCursor = repoOptVisibility
	updated, _ = m.updateRepoOptions(keyMsg(" "))
	model := updated.(Model)
	if model.repo.visibility != "public" {
		t.Errorf("visibility = %q, want public", model.repo.visibility)
	}
	updated, _ = model.updateRepoOptions(keyMsg(" "))
	if updated.(Model).repo.visibility != "private" {
		t.Error("second toggle should flip visibility back to private")
	}
}

func TestUpdateRepoOptions_DescriptionReceivesTyping(t *testing.T) {
	m := repoOptionsModel(t)
	m.repo.publishGitHub = true
	m.repo.optionsCursor = repoOptDescription
	m.repo.descInput.Focus()

	updated, _ := m.updateRepoOptions(keyMsg("hi"))

	if got := updated.(Model).repo.descInput.Value(); got != "hi" {
		t.Errorf("description = %q, want %q", got, "hi")
	}
}

func TestUpdateRepoOptions_DescriptionReceivesPaste(t *testing.T) {
	m := repoOptionsModel(t)
	m.repo.publishGitHub = true
	m.repo.optionsCursor = repoOptDescription
	m.repo.descInput.Focus()

	updated, _ := m.updateRepoOptions(tea.PasteMsg{Content: "release 界 notes"})

	if got := updated.(Model).repo.descInput.Value(); got != "release 界 notes" {
		t.Errorf("description = %q, want pasted text", got)
	}
}

func TestUpdateRepoOptions_PasteIgnoresHiddenDescription(t *testing.T) {
	m := repoOptionsModel(t)
	m.repo.publishGitHub = false
	m.repo.optionsCursor = repoOptPublish

	updated, _ := m.updateRepoOptions(tea.PasteMsg{Content: "hidden"})

	if got := updated.(Model).repo.descInput.Value(); got != "" {
		t.Errorf("hidden description changed to %q", got)
	}
}

func TestUpdateRepoOptions_BackReturnsToNameView(t *testing.T) {
	m := repoOptionsModel(t)

	updated, cmd := m.updateRepoOptions(tea.KeyPressMsg{Code: tea.KeyEsc})

	if updated.(Model).view != repoNameView {
		t.Error("esc should return to the name view")
	}
	if cmd == nil {
		t.Error("expected the name input focus command")
	}
}

func TestUpdateRepoOptions_EnterStartsCreatePipeline(t *testing.T) {
	m := repoOptionsModel(t)

	updated, cmd := m.updateRepoOptions(tea.KeyPressMsg{Code: tea.KeyEnter})
	model := updated.(Model)

	if model.view != repoProgressView {
		t.Fatalf("view = %d, want repoProgressView", model.view)
	}
	if model.repo.opType != "create" {
		t.Errorf("opType = %q, want create", model.repo.opType)
	}
	if cmd == nil {
		t.Fatal("expected a command waiting for pipeline events")
	}

	result, _ := drainRepoPipeline(t, model)
	if _, ok := result.(repo.CreateResult); !ok {
		t.Errorf("pipeline result = %T, want repo.CreateResult", result)
	}
}

func TestViewRepoOptions_AuthStates(t *testing.T) {
	cases := []struct {
		ghStatus string
		want     string
	}{
		{"", "checking"},
		{"authenticated", "authenticated"},
		{"not authenticated", "not authenticated"},
		{"gh not found", "gh not found"},
	}
	for _, tc := range cases {
		t.Run(tc.ghStatus+"_state", func(t *testing.T) {
			m := repoOptionsModel(t)
			m.repo.ghStatus = tc.ghStatus

			view := stripANSI(m.viewRepoOptions())

			if !strings.Contains(view, tc.want) {
				t.Errorf("view missing %q:\n%s", tc.want, view)
			}
		})
	}
}

// Asserted substrings must not appear in the test name: t.TempDir() embeds it,
// and the location path is rendered into the view.
func TestViewRepoOptions_ProgressiveDisclosure(t *testing.T) {
	m := repoOptionsModel(t)

	view := stripANSI(m.viewRepoOptions())
	if strings.Contains(view, "Visibility") {
		t.Errorf("visibility must stay hidden while publish is off:\n%s", view)
	}

	m.repo.publishGitHub = true
	view = stripANSI(m.viewRepoOptions())
	for _, want := range []string{"Create initial worktree", "Publish to GitHub", "Visibility", "private", "Description", "(optional)"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q:\n%s", want, view)
		}
	}
}
