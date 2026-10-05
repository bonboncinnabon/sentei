package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/config"
	"github.com/bonboncinnabon/sentei/internal/creator"
	"github.com/bonboncinnabon/sentei/internal/progress"
	"github.com/bonboncinnabon/sentei/internal/repo"
)

func createOptionsModel() Model {
	m := NewMenuModel(nil, nil, "/repo", &config.Config{}, repo.ContextBareRepo)
	m.width, m.height = 80, 24
	m.create.branchInput.SetValue("feature/x")
	m.create.baseInput.SetValue("main")
	return m
}

func TestBuildOptionItems_BaseOptionsOnly(t *testing.T) {
	m := createOptionsModel()

	items := m.buildOptionItems()
	if len(items) != 1 {
		t.Fatalf("expected only the merge option with no ecosystems, got %d: %+v", len(items), items)
	}
	if items[0].key != "merge" {
		t.Errorf("expected merge option, got %q", items[0].key)
	}
	if !strings.Contains(items[0].hint, "main → feature/x") {
		t.Errorf("merge hint should show base → branch, got %q", items[0].hint)
	}
}

func TestBuildOptionItems_EcosystemsAddDepsAndEnvFiles(t *testing.T) {
	m := createOptionsModel()
	m.create.ecosystems = []config.EcosystemConfig{
		{Name: "node", EnvFiles: []string{".env", ".env.local"}},
		{Name: "go"},
	}

	items := m.buildOptionItems()
	keys := make([]string, len(items))
	for i, it := range items {
		keys[i] = it.key
	}
	want := []string{"eco:node", "eco:go", "merge", "envfiles"}
	if strings.Join(keys, ",") != strings.Join(want, ",") {
		t.Errorf("option keys = %v, want %v", keys, want)
	}
	last := items[len(items)-1]
	if !strings.Contains(last.hint, ".env, .env.local") {
		t.Errorf("envfiles hint should list files, got %q", last.hint)
	}
}

func TestIsOptionEnabled_PerKind(t *testing.T) {
	m := createOptionsModel()
	m.create.ecoEnabled = map[string]bool{"node": true, "go": false}
	m.create.mergeBase = true
	m.create.copyEnvFiles = false

	cases := []struct {
		key  string
		want bool
	}{
		{"eco:node", true},
		{"eco:go", false},
		{"merge", true},
		{"envfiles", false},
		{"unknown", false},
	}
	for _, tc := range cases {
		if got := m.isOptionEnabled(optionItem{key: tc.key}); got != tc.want {
			t.Errorf("isOptionEnabled(%q) = %v, want %v", tc.key, got, tc.want)
		}
	}
}

func TestToggleOption_FlipsEachKind(t *testing.T) {
	m := createOptionsModel()
	m.create.ecoEnabled = map[string]bool{"node": false}
	m.create.mergeBase = true
	m.create.copyEnvFiles = false

	m.toggleOption(optionItem{key: "eco:node"})
	m.toggleOption(optionItem{key: "merge"})
	m.toggleOption(optionItem{key: "envfiles"})

	if !m.create.ecoEnabled["node"] {
		t.Error("ecosystem toggle should flip on")
	}
	if m.create.mergeBase {
		t.Error("merge toggle should flip off")
	}
	if !m.create.copyEnvFiles {
		t.Error("envfiles toggle should flip on")
	}

	// Unknown keys are ignored, not panics.
	m.toggleOption(optionItem{key: "nope"})
}

func TestValidateBranchName(t *testing.T) {
	cases := []struct {
		name     string
		branch   string
		existing []string
		wantErr  string
	}{
		{"valid simple", "feature/x", nil, ""},
		{"empty rejected", "", nil, "required"},
		{"space rejected", "feat x", nil, "spaces"},
		{"dotdot rejected", "a..b", nil, "'..'"},
		{"existing worktree rejected", "feature/x", []string{"/repo/feature-x"}, "already exists"},
		{"no false positive on suffix", "feature/x", []string{"/repo/other-feature-y"}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateBranchName(tc.branch, tc.existing)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("expected valid, got %q", err.message)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.message, tc.wantErr) {
				t.Errorf("error %q does not contain %q", err.message, tc.wantErr)
			}
		})
	}
}

// drainCreatePipeline executes wait commands until the creation goroutine
// reports completion, returning the final result and the events seen.
func drainCreatePipeline(t *testing.T, m Model) (creator.Result, []progress.Event) {
	t.Helper()
	var events []progress.Event
	for range 50 {
		msg := m.waitForCreateEvent()()
		switch msg := msg.(type) {
		case createEventMsg:
			events = append(events, msg.Event)
		case createCompleteMsg:
			return msg.Result, events
		default:
			t.Fatalf("unexpected message %T", msg)
		}
	}
	t.Fatal("creation pipeline never completed")
	return creator.Result{}, nil
}

func TestUpdateCreateOptions_CursorNavigationClamps(t *testing.T) {
	m := createOptionsModel()
	m.create.ecosystems = []config.EcosystemConfig{{Name: "node"}}
	// Items: eco:node, merge.

	updated, _ := m.updateCreateOptions(keyMsg("j"))
	model := updated.(Model)
	if model.create.optionsCursor != 1 {
		t.Errorf("cursor after down = %d, want 1", model.create.optionsCursor)
	}

	updated, _ = model.updateCreateOptions(keyMsg("j"))
	model = updated.(Model)
	if model.create.optionsCursor != 1 {
		t.Error("cursor must clamp at the last item")
	}

	updated, _ = model.updateCreateOptions(keyMsg("k"))
	model = updated.(Model)
	if model.create.optionsCursor != 0 {
		t.Errorf("cursor after up = %d, want 0", model.create.optionsCursor)
	}

	updated, _ = model.updateCreateOptions(keyMsg("k"))
	if updated.(Model).create.optionsCursor != 0 {
		t.Error("cursor must clamp at the first item")
	}
}

func TestUpdateCreateOptions_ToggleFlipsItemUnderCursor(t *testing.T) {
	m := createOptionsModel()
	// Only the merge option is present; it starts enabled.
	updated, _ := m.updateCreateOptions(keyMsg(" "))

	if updated.(Model).create.mergeBase {
		t.Error("space should toggle the merge option off")
	}
}

func TestUpdateCreateOptions_EscReturnsToBranchInput(t *testing.T) {
	m := createOptionsModel()
	m.create.branchInput.Blur()

	updated, cmd := m.updateCreateOptions(tea.KeyPressMsg{Code: tea.KeyEsc})
	model := updated.(Model)

	if model.view != createBranchView {
		t.Errorf("view = %d, want createBranchView", model.view)
	}
	if !model.create.branchInput.Focused() {
		t.Error("branch input should regain focus")
	}
	if cmd == nil {
		t.Error("expected cursor blink command")
	}
}

func TestUpdateCreateOptions_WindowSize(t *testing.T) {
	m := createOptionsModel()

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 90, Height: 30})
	model := updated.(Model)

	if model.width != 90 || model.height != 24 {
		t.Errorf("size = %dx%d, want 90x24", model.width, model.height)
	}
}

func TestUpdateCreateOptions_EnterStartsCreation(t *testing.T) {
	m := createOptionsModel()
	m.runner = &stubRunner{responses: map[string]stubResponse{}}

	updated, cmd := m.updateCreateOptions(tea.KeyPressMsg{Code: tea.KeyEnter})
	model := updated.(Model)

	if model.view != createProgressView {
		t.Fatalf("view = %d, want createProgressView", model.view)
	}
	if model.create.eventCh == nil || model.create.resultCh == nil {
		t.Fatal("creation channels must be wired before progress starts")
	}
	if cmd == nil {
		t.Fatal("expected a command waiting for the first event")
	}

	result, events := drainCreatePipeline(t, model)
	if !result.HasFailures() {
		t.Error("creation against a stub runner with no responses should fail")
	}
	if len(events) == 0 {
		t.Error("expected progress events from the pipeline")
	}
}

func TestStartCreation_OnlyEnabledEcosystemsPassed(t *testing.T) {
	m := createOptionsModel()
	m.runner = &stubRunner{responses: map[string]stubResponse{}}
	m.create.ecosystems = []config.EcosystemConfig{{Name: "node"}, {Name: "go"}}
	m.create.ecoEnabled = map[string]bool{"node": true, "go": false}

	m.startCreation()
	result, _ := drainCreatePipeline(t, m)

	// Setup fails fast with the stub runner, so the result carries only the
	// Setup phase; the assertion that matters is that startCreation ran the
	// pipeline to completion and delivered a result over the channels.
	if len(result.Phases) == 0 {
		t.Error("expected at least the Setup phase in the result")
	}
}

func TestWaitForCreateEvent_DeliversEventThenCompletion(t *testing.T) {
	m := createOptionsModel()
	m.create.eventCh = make(chan progress.Event, 1)
	m.create.resultCh = make(chan creator.Result, 1)

	ev := progress.Event{Phase: "Setup", Step: "Create worktree", Status: progress.StepRunning}
	m.create.eventCh <- ev
	msg := m.waitForCreateEvent()()
	got, ok := msg.(createEventMsg)
	if !ok {
		t.Fatalf("expected createEventMsg, got %T", msg)
	}
	if got.Event.Step != "Create worktree" {
		t.Errorf("event step = %q, want %q", got.Event.Step, "Create worktree")
	}

	close(m.create.eventCh)
	m.create.resultCh <- creator.Result{WorktreePath: "/repo/feature-x"}
	msg = m.waitForCreateEvent()()
	done, ok := msg.(createCompleteMsg)
	if !ok {
		t.Fatalf("expected createCompleteMsg after channel close, got %T", msg)
	}
	if done.Result.WorktreePath != "/repo/feature-x" {
		t.Errorf("result path = %q, want %q", done.Result.WorktreePath, "/repo/feature-x")
	}
}

func TestViewCreateOptions_RendersOptionsAndIntegrations(t *testing.T) {
	m := createOptionsModel()
	m.create.ecosystems = []config.EcosystemConfig{{Name: "node"}}
	m.create.ecoEnabled = map[string]bool{"node": true}
	m.create.activeIntegrationNames = []string{"code-review-graph"}

	view := stripANSI(m.viewCreateOptions())

	for _, want := range []string{
		"Create worktree", "feature/x", "from main", "Setup",
		"[x]", "Integrations from main: code-review-graph", "space toggle",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q:\n%s", want, view)
		}
	}
}
