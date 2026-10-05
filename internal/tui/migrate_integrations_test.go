package tui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/integration"
	"github.com/bonboncinnabon/sentei/internal/progress"
	"github.com/bonboncinnabon/sentei/internal/repo"
	"github.com/bonboncinnabon/sentei/internal/testutil/mock"
)

func TestUpdateMigrateIntegrations_DetectedMsg(t *testing.T) {
	m := makeIntegrationModel()
	m.view = migrateIntegrationsView

	detected := map[string]bool{
		"code-review-graph": true,
		"cocoindex-code":    false,
	}
	updated, _ := m.updateMigrateIntegrations(migrateIntegrationDetectedMsg{
		integrations: integration.All(),
		detected:     detected,
	})
	m = updated.(Model)

	if !m.integ.staged["code-review-graph"] {
		t.Error("expected code-review-graph to be pre-checked (detected=true)")
	}
	if m.integ.staged["cocoindex-code"] {
		t.Error("expected cocoindex-code to be unchecked (detected=false)")
	}
	if !m.integ.detected["code-review-graph"] {
		t.Error("expected detected map to be stored")
	}
}

func TestUpdateMigrateIntegrations_Toggle(t *testing.T) {
	m := makeIntegrationModel()
	m.view = migrateIntegrationsView
	m.integ.integrations = integration.All()
	m.integ.cursor = 0
	firstName := m.integ.integrations[0].Name
	initialState := m.integ.staged[firstName]

	updated, _ := m.updateMigrateIntegrations(keyMsg(" "))
	m = updated.(Model)

	if m.integ.staged[firstName] == initialState {
		t.Errorf("expected staged[%q] to be toggled from %v", firstName, initialState)
	}
}

func TestUpdateMigrateIntegrations_ConfirmNoSelections(t *testing.T) {
	m := makeIntegrationModel()
	m.view = migrateIntegrationsView
	m.integ.integrations = integration.All()
	// Set all staged to false
	for _, integ := range m.integ.integrations {
		m.integ.staged[integ.Name] = false
	}

	updated, _ := m.updateMigrateIntegrations(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(Model)

	if m.view != migrateNextView {
		t.Errorf("expected migrateNextView when no selections, got %d", m.view)
	}
}

func TestUpdateMigrateIntegrations_Skip(t *testing.T) {
	m := makeIntegrationModel()
	m.view = migrateIntegrationsView
	m.integ.integrations = integration.All()

	updated, _ := m.updateMigrateIntegrations(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = updated.(Model)

	if m.view != migrateNextView {
		t.Errorf("expected migrateNextView after esc, got %d", m.view)
	}
}

func TestViewMigrateIntegrations_ShowsDetectedHint(t *testing.T) {
	m := makeIntegrationModel()
	m.view = migrateIntegrationsView
	m.integ.integrations = integration.All()
	m.integ.detected = map[string]bool{
		"code-review-graph": true,
		"cocoindex-code":    false,
	}

	output := stripAnsi(m.viewMigrateIntegrations())

	if !strings.Contains(output, "detected") {
		t.Errorf("expected output to contain 'detected' hint, got:\n%s", output)
	}
}

func TestViewMigrateIntegrations_ShowsIntroText(t *testing.T) {
	m := makeIntegrationModel()
	m.view = migrateIntegrationsView
	m.integ.integrations = integration.All()

	output := stripAnsi(m.viewMigrateIntegrations())

	if !strings.Contains(output, "dev tools") {
		t.Errorf("expected output to contain 'dev tools', got:\n%s", output)
	}
}

// drainIntegrationApply executes wait commands until the apply goroutine
// reports completion, returning the events seen.
func drainIntegrationApply(t *testing.T, m Model) []progress.Event {
	t.Helper()
	var events []progress.Event
	for range 100 {
		msg := waitForIntegrationEvent(m.integ.eventCh, m.integ.resultCh)()
		switch msg := msg.(type) {
		case integrationEventMsg:
			events = append(events, msg.Event)
		case integrationApplyDoneMsg:
			return events
		default:
			t.Fatalf("unexpected message %T", msg)
		}
	}
	t.Fatal("integration apply never completed")
	return nil
}

func TestLoadMigrateIntegrations_DetectsArtifactDirs(t *testing.T) {
	dir := t.TempDir()
	first := integration.All()[0]
	artifact := strings.TrimSuffix(first.GitignoreEntries[0], "/")
	if err := os.Mkdir(filepath.Join(dir, artifact), 0o755); err != nil {
		t.Fatal(err)
	}

	msg := loadMigrateIntegrations(dir)()

	detected, ok := msg.(migrateIntegrationDetectedMsg)
	if !ok {
		t.Fatalf("expected migrateIntegrationDetectedMsg, got %T", msg)
	}
	if len(detected.integrations) != len(integration.All()) {
		t.Errorf("integrations = %d, want all %d", len(detected.integrations), len(integration.All()))
	}
	if !detected.detected[first.Name] {
		t.Errorf("%s should be detected via its artifact dir", first.Name)
	}
	for _, integ := range detected.integrations[1:] {
		if detected.detected[integ.Name] {
			t.Errorf("%s should not be detected in an empty dir", integ.Name)
		}
	}
}

func TestMigrateWorktreePath(t *testing.T) {
	m := makeIntegrationModel()

	explicit := repo.MigrateResult{WorktreePath: "/bare/main", BareRoot: "/bare", Branch: "dev"}
	if got := m.migrateWorktreePath(explicit); got != "/bare/main" {
		t.Errorf("explicit path = %q, want /bare/main", got)
	}

	derived := repo.MigrateResult{BareRoot: "/bare", Branch: "feature/dev"}
	if got := m.migrateWorktreePath(derived); got != filepath.Join("/bare", "feature-dev") {
		t.Errorf("derived path = %q, want /bare/feature-dev", got)
	}
}

func TestStartMigrateIntegrationApply_MissingResultShowsPreparationError(t *testing.T) {
	m := makeIntegrationModel()
	m.view = integrationProgressView
	m.integ.returnView = migrateNextView
	m.integ.executionErr = errors.New("stale execution")
	m.integ.saveErr = errors.New("stale save")
	m.repo.result = nil

	updated, cmd := m.startMigrateIntegrationApply()

	if cmd == nil {
		t.Fatal("missing migration result must settle into a visible summary")
	}
	if updated.integ.eventCh != nil {
		t.Error("channels must not be wired without a migrate result")
	}
	if updated.integ.lifecycle != integrationSettling || updated.integ.prepareErr == nil {
		t.Fatalf("lifecycle=%v prepareErr=%v", updated.integ.lifecycle, updated.integ.prepareErr)
	}
	if updated.integ.executionErr != nil || updated.integ.saveErr != nil {
		t.Fatalf("stale errors survived: execution=%v save=%v", updated.integ.executionErr, updated.integ.saveErr)
	}
	updated = settleNow(t, updated)
	if updated.view != integrationSummaryView {
		t.Fatalf("view = %v, want integration summary", updated.view)
	}
	returned, _ := updated.updateIntegrationSummary(tea.KeyPressMsg{Code: tea.KeyEnter})
	if returned.(Model).view != migrateNextView {
		t.Fatal("enter from migration integration error must return to migrateNext")
	}
}

func TestStartMigrateIntegrationApply_NoStagedCompletesImmediately(t *testing.T) {
	m := makeIntegrationModel()
	bareRoot := t.TempDir()
	m.repo.result = repo.MigrateResult{BareRoot: bareRoot, Branch: "main"}
	m.integ.staged = map[string]bool{}

	updated, cmd := m.startMigrateIntegrationApply()

	if cmd == nil {
		t.Fatal("expected a preparation command")
	}
	wantWT := filepath.Join(bareRoot, "main")
	if len(updated.integ.targetWorktrees) != 1 || updated.integ.targetWorktrees[0] != wantWT {
		t.Errorf("targetWorktrees = %v, want [%s] (derived from BareRoot/Branch)", updated.integ.targetWorktrees, wantWT)
	}

	preparedModel, _ := updated.updateIntegrationProgress(cmd())
	updated = preparedModel.(Model)
	events := drainIntegrationApply(t, updated)
	if len(events) != 0 {
		t.Errorf("expected no events with nothing staged, got %v", events)
	}
}

func TestStartMigrateIntegrationApply_EnablesStagedIntegrations(t *testing.T) {
	m := makeIntegrationModel()
	wtPath := "/bare/main"
	m.repo.result = repo.MigrateResult{BareRoot: "/bare", Branch: "main", WorktreePath: wtPath}
	m.integ.integrations = []integration.Integration{{
		Name:   "fake-tool",
		Detect: integration.DetectSpec{Command: "fake-tool --version"},
		Setup:  integration.SetupSpec{Command: "fake-tool init"},
	}}
	m.integ.staged = map[string]bool{"fake-tool": true}
	m.shell = &mock.Runner{Responses: map[string]mock.Response{
		wtPath + ":shell[fake-tool --version]": {Output: "1.0"},
		wtPath + ":shell[fake-tool init]":      {Output: "ok"},
	}}

	updated, cmd := m.startMigrateIntegrationApply()

	if cmd == nil {
		t.Fatal("expected a preparation command")
	}
	preparedModel, _ := updated.updateIntegrationProgress(cmd())
	updated = preparedModel.(Model)
	events := drainIntegrationApply(t, updated)

	var sawSetupDone bool
	for _, ev := range events {
		if ev.StepLabel == "Setup fake-tool" && ev.Status == progress.StepDone {
			sawSetupDone = true
		}
	}
	if !sawSetupDone {
		t.Errorf("expected a successful setup event, got %v", events)
	}
}
