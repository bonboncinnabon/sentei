package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/bonboncinnabon/sentei/internal/cleanup"
	"github.com/bonboncinnabon/sentei/internal/config"
	"github.com/bonboncinnabon/sentei/internal/repo"
	"github.com/bonboncinnabon/sentei/internal/testutil/mock"
)

func TestConfirmationView_ContainsTitle(t *testing.T) {
	vm := ConfirmationViewModel{
		Title: "Confirm cleanup",
	}

	output := stripAnsi(vm.View())

	if !strings.Contains(output, "Confirm cleanup") {
		t.Error("view should contain the title")
	}
}

func TestConfirmationView_ContainsAllItems(t *testing.T) {
	vm := ConfirmationViewModel{
		Title: "Review",
		Items: []ConfirmationItem{
			{Label: "Mode", Value: "aggressive"},
			{Label: "Force", Value: "true"},
			{Label: "Path", Value: "/home/user/repo"},
		},
	}

	output := stripAnsi(vm.View())

	for _, item := range vm.Items {
		if !strings.Contains(output, item.Label) {
			t.Errorf("view should contain label %q", item.Label)
		}
		if !strings.Contains(output, item.Value) {
			t.Errorf("view should contain value %q", item.Value)
		}
	}
}

func TestConfirmationView_ContainsCLICommand(t *testing.T) {
	vm := ConfirmationViewModel{
		Title:      "Confirm",
		CLICommand: "sentei cleanup --mode aggressive",
	}

	output := stripAnsi(vm.View())

	if !strings.Contains(output, "sentei cleanup --mode aggressive") {
		t.Error("view should contain the CLI command")
	}
}

func TestConfirmationView_ContainsKeybindings(t *testing.T) {
	vm := ConfirmationViewModel{Title: "Test"}

	output := stripAnsi(vm.View())

	if !strings.Contains(output, "enter confirm") {
		t.Error("view should show enter keybinding")
	}
	if !strings.Contains(output, "esc back") {
		t.Error("view should show esc keybinding")
	}
	if !strings.Contains(output, "q quit") {
		t.Error("view should show q keybinding")
	}
}

func TestConfirmationView_EmptyItems(t *testing.T) {
	vm := ConfirmationViewModel{
		Title:      "Empty",
		CLICommand: "sentei test",
	}

	output := stripAnsi(vm.View())

	if !strings.Contains(output, "Empty") {
		t.Error("view should still render title with no items")
	}
	if !strings.Contains(output, "sentei test") {
		t.Error("view should still render CLI command with no items")
	}
}

func TestUpdateConfirmation_EnterReturnsProceedMsg(t *testing.T) {
	msg := tea.KeyPressMsg{Code: tea.KeyEnter}

	cmd := UpdateConfirmation(msg)
	if cmd == nil {
		t.Fatal("enter should return a command")
	}

	result := cmd()
	if _, ok := result.(ConfirmProceedMsg); !ok {
		t.Errorf("enter should return ConfirmProceedMsg, got %T", result)
	}
}

func TestUpdateConfirmation_EscReturnsBackMsg(t *testing.T) {
	msg := tea.KeyPressMsg{Code: tea.KeyEscape}

	cmd := UpdateConfirmation(msg)
	if cmd == nil {
		t.Fatal("esc should return a command")
	}

	result := cmd()
	if _, ok := result.(ConfirmBackMsg); !ok {
		t.Errorf("esc should return ConfirmBackMsg, got %T", result)
	}
}

func TestUpdateConfirmation_QKeyReturnsQuit(t *testing.T) {
	msg := tea.KeyPressMsg{Code: 'q', Text: "q"}

	cmd := UpdateConfirmation(msg)
	if cmd == nil {
		t.Fatal("q should return a command")
	}

	result := cmd()
	// tea.Quit returns a tea.QuitMsg
	if _, ok := result.(tea.QuitMsg); !ok {
		t.Errorf("q should return tea.QuitMsg, got %T", result)
	}
}

func TestUpdateConfirmation_CtrlCReturnsQuit(t *testing.T) {
	msg := tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}

	cmd := UpdateConfirmation(msg)
	if cmd == nil {
		t.Fatal("ctrl+c should return a command")
	}

	result := cmd()
	if _, ok := result.(tea.QuitMsg); !ok {
		t.Errorf("ctrl+c should return tea.QuitMsg, got %T", result)
	}
}

func TestUpdateConfirmation_UnhandledKeyReturnsNil(t *testing.T) {
	msg := tea.KeyPressMsg{Code: 'x', Text: "x"}

	cmd := UpdateConfirmation(msg)
	if cmd != nil {
		t.Error("unhandled key should return nil")
	}
}

func TestUpdateConfirmation_NonKeyMsgReturnsNil(t *testing.T) {
	msg := tea.WindowSizeMsg{Width: 80, Height: 24}

	cmd := UpdateConfirmation(msg)
	if cmd != nil {
		t.Error("non-key message should return nil")
	}
}

// The confirmation a launch with flags opens acts on the key itself. Enter
// starts the operation in that same update, and esc and q quit: nothing is
// behind the screen to go back to.
func TestLaunchConfirm_KeysActInOneUpdate(t *testing.T) {
	cleanupModel := makeCleanupConfirmModel(&cleanup.Options{Mode: cleanup.ModeSafe})

	createModel := makeCreateConfirmModel(&CreateOpts{Branch: "feat/x", Base: "main"})
	createModel.runner = &stubRunner{responses: map[string]stubResponse{}}

	cloneModel := NewMenuModel(&mock.Runner{}, nil, t.TempDir(), &config.Config{}, repo.ContextNoRepo)
	cloneModel.SetCloneOpts(&CloneOpts{URL: "git@github.com:user/myrepo.git"})

	for name, tc := range map[string]struct {
		model   Model
		started viewState
		drain   func(Model)
	}{
		"cleanup": {cleanupModel, cleanupResultView, func(Model) {}},
		"create":  {createModel, createProgressView, func(m Model) { drainCreatePipeline(t, m) }},
		"clone":   {cloneModel, repoProgressView, func(m Model) { drainRepoPipeline(t, m) }},
	} {
		updated, cmd := tc.model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		if got := updated.(Model).view; got != tc.started || cmd == nil {
			t.Errorf("%s: enter left the view at %d, want the operation started (view %d)", name, got, tc.started)
			continue
		}
		tc.drain(updated.(Model))

		for _, quitKey := range []tea.KeyPressMsg{{Code: tea.KeyEsc}, {Code: 'q', Text: "q"}} {
			if _, cmd := tc.model.Update(quitKey); !quits(cmd) {
				t.Errorf("%s: %q did not quit", name, quitKey.String())
			}
		}
	}
}

// A row wider than the screen used to run off its edge, where the terminal
// cut it: the end of a long branch name, the part that tells two apart, was
// the part lost. It wraps under its value column, at its own break points,
// and on a narrow screen every label goes above its value. The command echo
// breaks so that a mouse copy pastes back as the same command.
func TestConfirmationView_LongValuesStayWholeAndTheEchoStaysRunnable(t *testing.T) {
	branch := "feature/payments-reconciliation-for-multi-currency-ledgers"
	command := "sentei create --base main --branch " + branch + " --copy-env --ecosystems pnpm,go,uv --merge-base"
	for _, width := range []int{40, 60, 80, 120} {
		vm := ConfirmationViewModel{
			Title:      "Confirm create",
			Items:      []ConfirmationItem{{Label: "Branch:", Value: branch}, {Label: "Base:", Value: "main"}},
			CLICommand: command,
			Width:      width,
		}
		out := stripAnsi(vm.View())
		for line := range strings.SplitSeq(out, "\n") {
			if w := ansi.StringWidth(line); w > width {
				t.Errorf("width %d: line is %d cells: %q", width, w, line)
			}
		}

		_, after, _ := strings.Cut(out, "Branch:")
		rows, _, _ := strings.Cut(after, "Base:")
		var value strings.Builder
		for _, line := range strings.Split(rows, "\n") {
			value.WriteString(strings.TrimSpace(line))
		}
		if value.String() != branch {
			t.Errorf("width %d: the branch reads %q across its lines, want it whole", width, value.String())
		}
		// Labels stack when the value column would be under stackBelow.
		stacked := strings.HasPrefix(rows, "\n")
		if want := width-len("Branch:")-4 < stackBelow; stacked != want {
			t.Errorf("width %d: stacked = %v, want %v:\n%s", width, stacked, want, out)
		}
		// A row breaks at a hyphen or slash, never inside a word part.
		for _, line := range strings.Split(strings.TrimSpace(rows), "\n") {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasSuffix(line, "-") && !strings.HasSuffix(line, "/") && !strings.HasSuffix(branch, line) {
				t.Errorf("width %d: the branch breaks mid-word at %q", width, line)
			}
		}

		_, echo, _ := strings.Cut(out, "  sentei create")
		echo, _, _ = strings.Cut("sentei create"+echo, "\n\n")
		if got := pasted(echo); got != command {
			t.Errorf("width %d: the echo pastes as %q, want %q:\n%s", width, got, command, out)
		}
	}
}
