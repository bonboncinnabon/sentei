package tui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/cleanup"
	"github.com/bonboncinnabon/sentei/internal/creator"
	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/repo"
)

var keyCtrlC = tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}

// allViews returns every viewState, counted from the declaration itself: the
// constants are consecutive from menuView, so their number is all that is
// needed. No constant has to be named as the last one, and a view added
// anywhere in the block is walked by the every-view tests.
func allViews(t *testing.T) []viewState {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "model.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var views []viewState
	for _, decl := range file.Decls {
		block, ok := decl.(*ast.GenDecl)
		if !ok || block.Tok != token.CONST {
			continue
		}
		if typ, ok := block.Specs[0].(*ast.ValueSpec).Type.(*ast.Ident); !ok || typ.Name != "viewState" {
			continue
		}
		for _, spec := range block.Specs {
			for range spec.(*ast.ValueSpec).Names {
				views = append(views, viewState(len(views)))
			}
		}
	}
	if len(views) == 0 || views[0] != menuView {
		t.Fatal("found no viewState constants in model.go: the every-view tests would cover nothing")
	}
	return views
}

func quits(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

// quitTestModel is a menu model with one selected worktree, showing view.
// Every flow has finished: progress views are in their settle hold and the
// cleanup result is on screen, so a quit key has nothing to interrupt.
func quitTestModel(view viewState) Model {
	m := NewMenuModel(nil, nil, "/repo", nil, repo.ContextBareRepo)
	m.width, m.windowHeight = 80, 24
	m.remove.worktrees = []git.Worktree{{Path: "/repo/a", Branch: "refs/heads/a"}}
	m.remove.selected = map[string]bool{"/repo/a": true}
	m.reindex()
	m.view = view
	m.remove.run.pruned = true
	m.create.result = &creator.Result{}
	m.repo.result = repo.CloneResult{}
	m.cleanupResult = &cleanup.Result{}
	return m
}

// inFlightModel is quitTestModel with the operation behind view still running.
func inFlightModel(view viewState) Model {
	m := quitTestModel(view)
	m.remove.run.pruned = false
	m.create.result = nil
	m.repo.result = nil
	m.cleanupResult = nil
	return m
}

// operationViews are the views that show an operation while it runs.
var operationViews = []viewState{progressView, createProgressView, repoProgressView, migrateProgressView, cleanupResultView}

func TestCtrlC_QuitsFromEveryView(t *testing.T) {
	for _, view := range allViews(t) {
		m := quitTestModel(view)
		if _, cmd := m.Update(keyCtrlC); !quits(cmd) {
			t.Errorf("view %d: ctrl+c did not quit", view)
		}
	}
}

// A declared view with no case in viewContent is a blank screen. This also
// pins that allViews walks real views, the last declared one included.
func TestEveryViewRenders(t *testing.T) {
	for _, view := range allViews(t) {
		if quitTestModel(view).viewContent() == "" {
			t.Errorf("view %d renders nothing: viewContent has no case for it", view)
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

// Quitting stops the in-process work where it stands, so while an operation
// runs the first quit key only asks. The second one quits.
func TestQuit_DuringAnOperationNeedsASecondPress(t *testing.T) {
	for _, view := range operationViews {
		for name, quitKey := range map[string]tea.KeyPressMsg{"q": keyRune('q'), "ctrl+c": keyCtrlC} {
			m := inFlightModel(view)

			updated, cmd := m.Update(quitKey)
			m = updated.(Model)
			if quits(cmd) {
				t.Fatalf("view %d: the first %s quit a running operation", view, name)
			}
			if screen := stripAnsi(m.View().Content); !strings.Contains(screen, "quit now, leaving the operation unfinished") {
				t.Fatalf("view %d: the first %s did not show the quit prompt:\n%s", view, name, screen)
			}

			if _, cmd = m.Update(quitKey); !quits(cmd) {
				t.Errorf("view %d: the second %s did not quit", view, name)
			}
		}
	}
}

func TestQuit_AnyOtherKeyCancelsThePrompt(t *testing.T) {
	for _, view := range operationViews {
		m := inFlightModel(view)
		updated, _ := m.Update(keyRune('q'))
		updated, cmd := updated.(Model).Update(keyRune('x'))
		m = updated.(Model)
		if quits(cmd) {
			t.Fatalf("view %d: a key other than quit answered the prompt with a quit", view)
		}
		if screen := stripAnsi(m.View().Content); strings.Contains(screen, "quit now") {
			t.Fatalf("view %d: the quit prompt survived another key:\n%s", view, screen)
		}
		// The prompt starts over: one more quit key asks again.
		if _, cmd = m.Update(keyRune('q')); quits(cmd) {
			t.Errorf("view %d: a cancelled prompt left the next quit key armed", view)
		}
	}
}

func TestQuit_PromptClosesAPortalThatWouldHideIt(t *testing.T) {
	m := inFlightModel(progressView)
	m.portal = m.portal.SetSize(80, 24).Open(portalHelp, "Help", "body")

	updated, cmd := m.Update(keyRune('q'))
	m = updated.(Model)
	if quits(cmd) || m.portal.Visible() {
		t.Fatalf("q over a portal during an operation: quit=%v portal open=%v, want the prompt alone", quits(cmd), m.portal.Visible())
	}
}

// Once the result has arrived there is nothing left to interrupt: the view
// is only holding its final frame, and q quits at once.
func TestQuit_AfterTheOperationFinishedIsImmediate(t *testing.T) {
	for _, view := range operationViews {
		m := quitTestModel(view)
		if _, cmd := m.Update(keyRune('q')); !quits(cmd) {
			t.Errorf("view %d: q did not quit a finished operation", view)
		}
	}
}

func TestQuit_PromptDoesNotOutliveItsOperation(t *testing.T) {
	m := inFlightModel(progressView)
	updated, _ := m.Update(keyRune('q'))
	m = updated.(Model)

	// The run finishes while the prompt is up; a later run starts clean.
	m.remove.run.pruned = true
	updated, _ = m.Update(keyRune('x'))
	m = updated.(Model)
	m.remove.run.pruned = false

	if _, cmd := m.Update(keyRune('q')); quits(cmd) {
		t.Error("a prompt left over from the previous operation quit the next one on the first key")
	}
}

func TestProgressHelp_SaysWhatQuittingDoes(t *testing.T) {
	m := inFlightModel(progressView)
	_, help := m.helpContent()
	help = stripAnsi(help)
	if strings.Contains(help, "keeps running") {
		t.Errorf("progress help still claims the operation keeps running:\n%s", help)
	}
	if !strings.Contains(help, "asks first") {
		t.Errorf("progress help does not mention the quit prompt:\n%s", help)
	}
}

// main prints "quit during <operation>" from InterruptedFlow after the
// program exits. A flow that finished and is only holding its last frame was
// not interrupted.
func TestInterruptedFlow_OnlyNamesAnOperationStillRunning(t *testing.T) {
	for _, view := range operationViews {
		if got := inFlightModel(view).InterruptedFlow(); got == "" {
			t.Errorf("view %d: a quit during the running operation leaves no trace", view)
		}
		if got := quitTestModel(view).InterruptedFlow(); got != "" {
			t.Errorf("view %d: InterruptedFlow() = %q for an operation that finished", view, got)
		}
	}
}
