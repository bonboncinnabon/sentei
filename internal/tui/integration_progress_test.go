package tui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/bonboncinnabon/sentei/internal/config"
	"github.com/bonboncinnabon/sentei/internal/integration"
	"github.com/bonboncinnabon/sentei/internal/progress"
	"github.com/bonboncinnabon/sentei/internal/repo"
	"github.com/bonboncinnabon/sentei/internal/state"
)

func TestIntegrationPreparingFrameFitsEveryResponsiveTier(t *testing.T) {
	for _, width := range []int{20, 40, 50, 80, 120} {
		for height := 1; height <= 40; height++ {
			m := NewModel(nil, nil, "/repo")
			m.view = integrationProgressView
			m.integ.lifecycle = integrationPreparing
			m.width, m.windowHeight = width, height

			view := m.viewIntegrationProgress()
			lines := strings.Split(view, "\n")
			if len(lines) > height {
				t.Fatalf("%dx%d preparing frame has %d rows:\n%s", width, height, len(lines), stripANSI(view))
			}
			for row, line := range lines {
				if got := lipgloss.Width(line); got > width {
					t.Fatalf("%dx%d row %d width=%d:\n%s", width, height, row+1, got, stripANSI(view))
				}
			}
		}
	}
}

func TestUpdateIntegrationProgress_FinalizedMsg_SaveError_DoesNotApply(t *testing.T) {
	m := makeIntegrationModel()
	m.view = integrationProgressView
	m.integ.returnView = integrationListView
	genBefore := m.worktreeGeneration

	// The apply tried to flip cocoindex-code on, but state.Save failed.
	updated, _ := m.updateIntegrationProgress(integrationFinalizedMsg{err: errors.New("save failed")})
	m = updated.(Model)

	// In-memory state must stay consistent with disk: the unsaved change is not applied.
	if m.integ.current["cocoindex-code"] {
		t.Error("a failed save must not mutate in-memory current state")
	}
	if m.integ.staged["cocoindex-code"] {
		t.Error("a failed save must not mutate staged state")
	}
	// worktreeGeneration must not advance (no reload of unsaved state).
	if m.worktreeGeneration != genBefore {
		t.Errorf("worktreeGeneration must not advance on a failed save: %d -> %d", genBefore, m.worktreeGeneration)
	}
}

func makeIntegrationModel() Model {
	m := NewMenuModel(nil, nil, "/repo", &config.Config{}, repo.ContextBareRepo)
	m.integ.integrations = integration.All()
	m.integ.current = map[string]bool{"code-review-graph": true, "cocoindex-code": false}
	m.integ.staged = map[string]bool{"code-review-graph": true, "cocoindex-code": false}
	m.integ.depStatus = map[string]bool{"python3.10+": true, "pipx": true, "python3.11+": true, "uv": true}
	m.width = 80
	m.height = 24
	return m
}

func TestIntegrationLifecycleTransitionsKeepErrorsSeparate(t *testing.T) {
	m := makeIntegrationModel()
	if m.integ.lifecycle != integrationIdle {
		t.Fatalf("initial lifecycle = %v, want idle", m.integ.lifecycle)
	}

	m.view = integrationProgressView
	m.integ.lifecycle = integrationPreparing
	prepareErr := errors.New("prepare failed")
	updated, _ := m.updateIntegrationProgress(integrationPreparedMsg{err: prepareErr})
	m = updated.(Model)
	if m.integ.lifecycle != integrationSettling || !errors.Is(m.integ.prepareErr, prepareErr) {
		t.Fatalf("after preparation failure: lifecycle=%v prepareErr=%v", m.integ.lifecycle, m.integ.prepareErr)
	}
	if m.integ.executionErr != nil || m.integ.saveErr != nil {
		t.Fatalf("preparation failure polluted later errors: execution=%v save=%v", m.integ.executionErr, m.integ.saveErr)
	}
}

func TestUpdateIntegrationProgress_EventMsg(t *testing.T) {
	m := makeIntegrationModel()
	m.view = integrationProgressView
	ch := make(chan progress.Event, 1)
	resultCh := make(chan integrationApplyResult, 1)
	m.integ.eventCh = ch
	m.integ.resultCh = resultCh

	ev := progress.Event{
		Phase:  "/repo/main",
		Step:   "Install code-review-graph",
		Status: progress.StepRunning,
	}
	updated, _ := m.updateIntegrationProgress(integrationEventMsg{Event: ev})
	m = updated.(Model)

	if len(m.integ.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(m.integ.events))
	}
	if m.integ.events[0].Step != "Install code-review-graph" {
		t.Errorf("unexpected event step: %q", m.integ.events[0].Step)
	}
}

func TestUpdateIntegrationProgress_FinalizedMsg_DoesNotMutateInMemory(t *testing.T) {
	m := makeIntegrationModel()
	m.view = integrationProgressView
	m.integ.returnView = integrationListView

	updated, _ := m.updateIntegrationProgress(integrationFinalizedMsg{err: nil})
	m = updated.(Model)

	m = settleNow(t, m)
	if m.view != integrationSummaryView {
		t.Errorf("expected integrationSummaryView, got %d", m.view)
	}
	// current/staged are reconciled from disk when the summary is dismissed,
	// never mutated in-memory at finalize time.
	if m.integ.current["cocoindex-code"] {
		t.Error("finalize must not mutate in-memory current state")
	}
	if m.integ.staged["cocoindex-code"] {
		t.Error("finalize must not mutate staged state")
	}
}

func TestUpdateIntegrationProgress_FinalizedMsg_Migration(t *testing.T) {
	m := makeIntegrationModel()
	m.view = integrationProgressView
	m.integ.returnView = migrateNextView
	// current starts as code-review-graph=true, cocoindex-code=false
	originalCurrent := map[string]bool{
		"code-review-graph": true,
		"cocoindex-code":    false,
	}
	m.integ.current = originalCurrent

	updated, _ := m.updateIntegrationProgress(integrationFinalizedMsg{err: nil})
	m = updated.(Model)

	m = settleNow(t, m)
	if m.view != migrateNextView {
		t.Errorf("expected migrateNextView, got %d", m.view)
	}
	if m.integ.lifecycle != integrationIdle {
		t.Fatalf("lifecycle=%v, want idle after clean migration hand-off", m.integ.lifecycle)
	}
	// current should NOT be updated for migration flow
	if m.integ.current["cocoindex-code"] {
		t.Error("cocoindex-code should not be updated in current for migration flow")
	}
}

func TestUpdateIntegrationProgress_MigrationSaveErrorShowsSummaryThenReturns(t *testing.T) {
	m := makeIntegrationModel()
	m.view = integrationProgressView
	m.integ.returnView = migrateNextView
	saveErr := errors.New("disk full")

	updated, _ := m.updateIntegrationProgress(integrationFinalizedMsg{err: saveErr})
	m = settleNow(t, updated.(Model))
	if m.view != integrationSummaryView || !errors.Is(m.integ.saveErr, saveErr) {
		t.Fatalf("view=%v saveErr=%v, want migration integration error summary", m.view, m.integ.saveErr)
	}
	for _, key := range []tea.KeyPressMsg{{Code: tea.KeyEnter}, {Code: tea.KeyEsc}} {
		returned, _ := m.updateIntegrationSummary(key)
		if returned.(Model).view != migrateNextView {
			t.Fatalf("key %v did not return to migrateNext", key.Code)
		}
	}
}

func TestViewIntegrationProgress_GroupsByWorktree(t *testing.T) {
	m := makeIntegrationModel()
	m.view = integrationProgressView
	m.integ.events = []progress.Event{
		{Phase: "/repo/main", Step: "Install step", Status: progress.StepDone},
		{Phase: "/repo/feature", Step: "Install step", Status: progress.StepRunning},
	}

	output := stripAnsi(m.viewIntegrationProgress())

	if !strings.Contains(output, "main") {
		t.Error("expected output to contain 'main' worktree name")
	}
	if !strings.Contains(output, "feature") {
		t.Error("expected output to contain 'feature' worktree name")
	}
}

func TestViewIntegrationProgress_ShowsProgressBar(t *testing.T) {
	m := makeIntegrationModel()
	m.view = integrationProgressView
	m.integ.events = []progress.Event{
		{Phase: "/repo/main", Step: "step1", Status: progress.StepRunning},
		{Phase: "/repo/main", Step: "step1", Status: progress.StepDone},
		{Phase: "/repo/main", Step: "step2", Status: progress.StepRunning},
		{Phase: "/repo/main", Step: "step2", Status: progress.StepDone},
		{Phase: "/repo/main", Step: "step3", Status: progress.StepRunning},
	}

	output := stripAnsi(m.viewIntegrationProgress())

	// 2 done out of 3 total → 66%.
	if !strings.Contains(output, "66%") {
		t.Errorf("expected output to contain progress '66%%', got:\n%s", output)
	}
}

func TestViewIntegrationProgress_ProgressCountsUniqueSteps(t *testing.T) {
	m := makeIntegrationModel()
	m.view = integrationProgressView
	m.integ.events = []progress.Event{
		{Phase: "/repo/main", Step: "Setup code-review-graph", Status: progress.StepRunning},
		{Phase: "/repo/main", Step: "Setup code-review-graph", Status: progress.StepDone},
		{Phase: "/repo/main", Step: "Install cocoindex-code", Status: progress.StepRunning},
		{Phase: "/repo/main", Step: "Install cocoindex-code", Status: progress.StepDone},
		{Phase: "/repo/main", Step: "Setup cocoindex-code", Status: progress.StepRunning},
		{Phase: "/repo/main", Step: "Setup cocoindex-code", Status: progress.StepDone},
	}

	output := stripAnsi(m.viewIntegrationProgress())

	// 3/3 done → 100%.
	if !strings.Contains(output, "100%") {
		t.Errorf("expected progress '100%%', got:\n%s", output)
	}
}

func TestViewIntegrationProgress_TotalKnownUpfront(t *testing.T) {
	m := makeIntegrationModel()
	m.view = integrationProgressView
	// The declared plan establishes the denominator before work starts: nine
	// pending steps across three worktrees, one resolved so far.
	m.integ.events = []progress.Event{}
	for _, wt := range []string{"/repo/main", "/repo/feat-1", "/repo/feat-2"} {
		for _, step := range []string{"Setup crg", "Setup ccc", "Setup third"} {
			m.integ.events = append(m.integ.events, progress.Event{Phase: wt, Step: step, Status: progress.StepPending, Of: 1})
		}
	}
	m.integ.events = append(m.integ.events,
		progress.Event{Phase: "/repo/main", Step: "Setup crg", Status: progress.StepDone},
		progress.Event{Phase: "/repo/main", Step: "Setup ccc", Status: progress.StepRunning},
	)

	// 1 done out of 9: the declared total is the spring target's denominator.
	done, total := m.integrationLayout().overall()
	if done != 1 || total != 9 {
		t.Errorf("overall() = %d/%d, want 1/9 (declared total)", done, total)
	}
	if cmd := m.syncProgressBar(); cmd == nil {
		t.Error("expected a spring target command from the upfront total")
	}
	if pct := m.bar.Percent(); pct < 0.11 || pct > 0.12 {
		t.Errorf("spring target = %.3f, want ~0.111 (1/9)", pct)
	}
}

func TestViewIntegrationProgress_Loading(t *testing.T) {
	m := makeIntegrationModel()
	m.view = integrationProgressView
	m.integ.events = nil

	output := stripAnsi(m.viewIntegrationProgress())

	if !strings.Contains(output, "Applying integration changes") {
		t.Errorf("expected title 'Applying integration changes', got:\n%s", output)
	}
}

func TestViewIntegrationProgress_PreparingPlanIsIndeterminate(t *testing.T) {
	m := makeIntegrationModel()
	m.view = integrationProgressView
	m.integ.lifecycle = integrationPreparing

	output := stripAnsi(m.viewIntegrationProgress())
	if !strings.Contains(output, "Preparing plan...") {
		t.Fatalf("preparation copy missing:\n%s", output)
	}
	bar := m.terminalProgress()
	if bar == nil || bar.State != tea.ProgressBarIndeterminate {
		t.Fatalf("terminal progress = %#v, want indeterminate", bar)
	}
}

func TestUpdateIntegrationProgress_PreparationErrorIsReported(t *testing.T) {
	m := makeIntegrationModel()
	m.view = integrationProgressView
	m.integ.lifecycle = integrationPreparing
	m.integ.returnView = integrationListView
	prepareErr := errors.New("no target worktree")

	updated, _ := m.updateIntegrationProgress(integrationPreparedMsg{err: prepareErr})
	m = updated.(Model)
	if m.integ.lifecycle != integrationSettling || !errors.Is(m.integ.prepareErr, prepareErr) {
		t.Fatalf("lifecycle=%v prepareErr=%v", m.integ.lifecycle, m.integ.prepareErr)
	}
	if m.integ.eventCh != nil {
		t.Fatal("execution channel created after preparation failure")
	}
}

func TestUpdateIntegrationProgress_ExecutionErrorDoesNotFinalizeState(t *testing.T) {
	m := makeIntegrationModel()
	m.view = integrationProgressView
	m.integ.returnView = integrationListView
	runErr := errors.New("progress sink failed")

	updated, _ := m.updateIntegrationProgress(integrationApplyDoneMsg{result: integrationApplyResult{err: runErr}})
	m = updated.(Model)
	if !errors.Is(m.integ.executionErr, runErr) {
		t.Fatalf("executionErr = %v, want %v", m.integ.executionErr, runErr)
	}
	if m.integ.lifecycle != integrationSettling {
		t.Fatal("execution error must finalize into the error summary")
	}
}

func TestClassifyIntegrationExecution(t *testing.T) {
	one := func(status progress.StepStatus) []progress.Phase {
		return []progress.Phase{{Steps: []progress.StepResult{{Status: status}}}}
	}
	tests := []struct {
		name   string
		phases []progress.Phase
		empty  bool
		want   integrationExecutionOutcome
	}{
		{name: "valid empty plan", empty: true, want: integrationExecutionEmpty},
		{name: "missing result is not an empty plan", want: integrationExecutionMalformed},
		{name: "all done", phases: one(progress.StepDone), want: integrationExecutionCompleted},
		{name: "ordinary failure", phases: one(progress.StepFailed), want: integrationExecutionDomainFailed},
		{name: "skip without failure", phases: one(progress.StepSkipped), want: integrationExecutionMalformed},
		{name: "unresolved", phases: one(progress.StepPending), want: integrationExecutionMalformed},
		{name: "failure with blocked skip", phases: []progress.Phase{{Steps: []progress.StepResult{{Status: progress.StepFailed}, {Status: progress.StepSkipped}}}}, want: integrationExecutionDomainFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyIntegrationExecution(integrationApplyResult{phases: tt.phases, empty: tt.empty}); got != tt.want {
				t.Fatalf("classifyIntegrationExecution() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUpdateIntegrationProgress_MalformedResultIsContractErrorAndPreservesState(t *testing.T) {
	tests := []struct {
		name   string
		phases []progress.Phase
		events []progress.Event
	}{
		{name: "missing phases"},
		{name: "pending", phases: []progress.Phase{{Name: "/repo/main", Steps: []progress.StepResult{{Name: "setup", Status: progress.StepPending}}}}, events: []progress.Event{{Phase: "/repo/main", Step: "setup", Status: progress.StepPending}}},
		{name: "running", phases: []progress.Phase{{Name: "/repo/main", Steps: []progress.StepResult{{Name: "setup", Status: progress.StepRunning}}}}, events: []progress.Event{{Phase: "/repo/main", Step: "setup", Status: progress.StepRunning}}},
		{name: "zero status", phases: []progress.Phase{{Name: "/repo/main", Steps: []progress.StepResult{{Name: "setup"}}}}},
		{name: "skip only", phases: []progress.Phase{{Name: "/repo/main", Steps: []progress.StepResult{{Name: "setup", Status: progress.StepSkipped}}}}, events: []progress.Event{{Phase: "/repo/main", Step: "setup", Status: progress.StepSkipped}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmp := t.TempDir()
			bareDir := filepath.Join(tmp, ".bare")
			if err := os.Mkdir(bareDir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := state.Save(bareDir, &state.State{Integrations: []string{"code-review-graph"}, LifetimeRemoved: 4}); err != nil {
				t.Fatal(err)
			}
			m := makeIntegrationModel()
			m.view = integrationProgressView
			m.repoPath = tmp
			m.runner = bareDirRunner(tmp)
			m.integ.returnView = integrationListView
			m.integ.staged = map[string]bool{"cocoindex-code": true}
			m.integ.events = tt.events

			updated, _ := m.updateIntegrationProgress(integrationApplyDoneMsg{result: integrationApplyResult{
				phases: tt.phases,
			}})
			m = updated.(Model)
			if m.integ.lifecycle != integrationSettling || m.integ.executionErr == nil || !strings.Contains(m.integ.executionErr.Error(), "incomplete terminal result") {
				t.Fatalf("lifecycle=%v executionErr=%v, want incomplete-result contract error", m.integ.lifecycle, m.integ.executionErr)
			}
			persisted, err := state.Load(bareDir)
			if err != nil {
				t.Fatal(err)
			}
			if len(persisted.Integrations) != 1 || persisted.Integrations[0] != "code-review-graph" || persisted.LifetimeRemoved != 4 {
				t.Fatalf("state changed after malformed result: %#v", persisted)
			}
			m = settleNow(t, m)
			if m.view != integrationSummaryView {
				t.Fatalf("view=%v, want integration error summary", m.view)
			}
			view := stripAnsi(m.viewIntegrationSummary())
			if strings.Count(view, "Integration execution failed:") != 1 {
				t.Fatalf("expected exactly one execution-error verdict:\n%s", view)
			}
			for _, forbidden := range []string{"No integration work was needed", "steps applied", "step applied"} {
				if strings.Contains(view, forbidden) {
					t.Fatalf("malformed result rendered success %q:\n%s", forbidden, view)
				}
			}
		})
	}
}

func TestUpdateIntegrationProgress_DomainFailureWithBlockedSkipIsNotContractError(t *testing.T) {
	tmp := t.TempDir()
	bareDir := filepath.Join(tmp, ".bare")
	if err := os.Mkdir(bareDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := state.Save(bareDir, &state.State{Integrations: []string{"code-review-graph"}, LifetimeRemoved: 4}); err != nil {
		t.Fatal(err)
	}
	m := makeIntegrationModel()
	m.view = integrationProgressView
	m.repoPath = tmp
	m.runner = bareDirRunner(tmp)
	m.integ.returnView = integrationListView
	m.integ.staged = map[string]bool{"cocoindex-code": true}
	m.integ.events = []progress.Event{
		{Phase: "/repo/main", Step: "install", Status: progress.StepFailed, Error: errors.New("install failed")},
		{Phase: "/repo/main", Step: "setup", Status: progress.StepSkipped, Message: "blocked by install"},
	}
	result := integrationApplyResult{phases: []progress.Phase{{Name: "/repo/main", Steps: []progress.StepResult{
		{Name: "install", Status: progress.StepFailed, Error: errors.New("install failed")},
		{Name: "setup", Status: progress.StepSkipped, Message: "blocked by install"},
	}}}}

	updated, _ := m.updateIntegrationProgress(integrationApplyDoneMsg{result: result})
	m = updated.(Model)
	if m.integ.executionErr != nil || m.integ.lifecycle != integrationSettling {
		t.Fatalf("executionErr=%v lifecycle=%v, want ordinary failed outcome", m.integ.executionErr, m.integ.lifecycle)
	}
	persisted, err := state.Load(bareDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(persisted.Integrations) != 1 || persisted.Integrations[0] != "code-review-graph" || persisted.LifetimeRemoved != 4 {
		t.Fatalf("state changed after domain failure: %#v", persisted)
	}
	m = settleNow(t, m)
	view := stripAnsi(m.viewIntegrationSummary())
	if strings.Contains(view, "Integration execution failed:") || strings.Count(view, "1 failed") != 1 {
		t.Fatalf("domain failure rendered as contract error:\n%s", view)
	}
}

func TestUpdateIntegrationProgress_EmptyExecutionSavesDesiredState(t *testing.T) {
	tmp := t.TempDir()
	bareDir := filepath.Join(tmp, ".bare")
	if err := os.Mkdir(bareDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := state.Save(bareDir, &state.State{Integrations: []string{"code-review-graph"}, LifetimeRemoved: 6}); err != nil {
		t.Fatal(err)
	}
	m := makeIntegrationModel()
	m.view = integrationProgressView
	m.repoPath = tmp
	m.runner = bareDirRunner(tmp)
	m.integ.returnView = integrationListView
	m.integ.staged = map[string]bool{}

	updated, saveCmd := m.updateIntegrationProgress(integrationApplyDoneMsg{result: integrationApplyResult{phases: nil, empty: true}})
	m = updated.(Model)
	if m.integ.lifecycle != integrationSaving || saveCmd == nil {
		t.Fatalf("lifecycle=%v saveCmd=%v, want saving", m.integ.lifecycle, saveCmd)
	}
	updated, _ = m.updateIntegrationProgress(saveCmd())
	m = updated.(Model)
	if m.integ.lifecycle != integrationSettling || m.integ.saveErr != nil {
		t.Fatalf("lifecycle=%v saveErr=%v, want clean settling", m.integ.lifecycle, m.integ.saveErr)
	}
	persisted, err := state.Load(bareDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(persisted.Integrations) != 0 || persisted.LifetimeRemoved != 6 {
		t.Fatalf("persisted state = %#v, want empty integrations and preserved lifetime", persisted)
	}
}

func TestFinalizeIntegrationApply_SavesStagedIntegrations(t *testing.T) {
	tmp := t.TempDir()
	if err := os.Mkdir(filepath.Join(tmp, ".bare"), 0o755); err != nil {
		t.Fatal(err)
	}
	m := makeIntegrationModel()
	m.repoPath = tmp
	m.runner = bareDirRunner(tmp)
	m.integ.returnView = integrationListView
	m.integ.staged = map[string]bool{"code-review-graph": true, "cocoindex-code": false}

	msg := m.finalizeIntegrationApply()()

	finalized, ok := msg.(integrationFinalizedMsg)
	if !ok {
		t.Fatalf("expected integrationFinalizedMsg, got %T", msg)
	}
	if finalized.err != nil {
		t.Fatalf("unexpected save error: %v", finalized.err)
	}

	st, err := state.Load(filepath.Join(tmp, ".bare"))
	if err != nil {
		t.Fatal(err)
	}
	if !st.HasIntegration("code-review-graph") || st.HasIntegration("cocoindex-code") {
		t.Errorf("persisted integrations = %v, want only code-review-graph", st.Integrations)
	}
}

func TestFinalizeIntegrationApply_PreservesLifetimeRemoved(t *testing.T) {
	tmp := t.TempDir()
	bareDir := filepath.Join(tmp, ".bare")
	if err := os.Mkdir(bareDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := state.Save(bareDir, &state.State{Integrations: []string{"old"}, LifetimeRemoved: 17}); err != nil {
		t.Fatal(err)
	}
	m := makeIntegrationModel()
	m.repoPath = tmp
	m.runner = bareDirRunner(tmp)
	m.integ.staged = map[string]bool{"code-review-graph": true}

	if err := m.finalizeIntegrationApply()().(integrationFinalizedMsg).err; err != nil {
		t.Fatal(err)
	}
	got, err := state.Load(bareDir)
	if err != nil {
		t.Fatal(err)
	}
	if got.LifetimeRemoved != 17 {
		t.Fatalf("LifetimeRemoved = %d, want 17", got.LifetimeRemoved)
	}
}

func TestFinalizeIntegrationApply_InvalidJSONFailsWithoutOverwrite(t *testing.T) {
	tmp := t.TempDir()
	bareDir := filepath.Join(tmp, ".bare")
	if err := os.Mkdir(bareDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(bareDir, "sentei.json")
	original := []byte("{ definitely not json\n")
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}
	m := makeIntegrationModel()
	m.repoPath = tmp
	m.runner = bareDirRunner(tmp)
	m.integ.staged = map[string]bool{"code-review-graph": true}

	if err := m.finalizeIntegrationApply()().(integrationFinalizedMsg).err; err == nil {
		t.Fatal("invalid existing state must fail the save")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(original) {
		t.Fatalf("invalid state was overwritten: %q", got)
	}
}

func TestFinalizeIntegrationApply_MigrateSavesUnderBareRoot(t *testing.T) {
	bareRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(bareRoot, ".bare"), 0o755); err != nil {
		t.Fatal(err)
	}
	m := makeIntegrationModel()
	m.repoPath = "/elsewhere"
	m.runner = bareDirRunner(bareRoot)
	m.integ.returnView = migrateNextView
	m.repo.result = repo.MigrateResult{BareRoot: bareRoot, Branch: "main"}
	m.integ.staged = map[string]bool{"code-review-graph": true}

	msg := m.finalizeIntegrationApply()()

	if err := msg.(integrationFinalizedMsg).err; err != nil {
		t.Fatalf("unexpected save error: %v", err)
	}
	st, err := state.Load(filepath.Join(bareRoot, ".bare"))
	if err != nil {
		t.Fatal(err)
	}
	if !st.HasIntegration("code-review-graph") {
		t.Errorf("persisted integrations = %v, want code-review-graph under the migrated bare root", st.Integrations)
	}
}

func TestFinalizeIntegrationApply_SaveFailureReturnsError(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing") // .bare dir does not exist
	m := makeIntegrationModel()
	m.repoPath = missing
	m.runner = bareDirRunner(missing)
	m.integ.returnView = integrationListView

	msg := m.finalizeIntegrationApply()()

	if msg.(integrationFinalizedMsg).err == nil {
		t.Error("expected a save error when the .bare dir is missing")
	}
}
