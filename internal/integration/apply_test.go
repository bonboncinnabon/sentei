package integration

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/progress"
)

type applyShell struct {
	mu        sync.Mutex
	responses map[string]mockShellResponse
	calls     []string
}

type mockShellResponse struct {
	output string
	err    error
}

type applyShellFunc func(dir, command string) (string, error)

func (fn applyShellFunc) RunShell(dir, command string) (string, error) { return fn(dir, command) }

type failingApplyFiles struct {
	removeErr    error
	copyErr      error
	gitignoreErr error
}

func (f failingApplyFiles) removeAll(string) error                 { return f.removeErr }
func (f failingApplyFiles) copyDir(string, string) error           { return f.copyErr }
func (f failingApplyFiles) appendGitignore(string, []string) error { return f.gitignoreErr }

func findStep(t *testing.T, events []progress.Event, label string) progress.StepState {
	t.Helper()
	for _, phase := range progress.Snapshot(events) {
		for _, step := range phase.Steps {
			if step.Name == label {
				return step
			}
		}
	}
	t.Fatalf("step %q not found", label)
	return progress.StepState{}
}

func (s *applyShell) RunShell(dir, command string) (string, error) {
	key := fmt.Sprintf("%s:shell[%s]", dir, command)
	s.mu.Lock()
	s.calls = append(s.calls, key)
	response, ok := s.responses[key]
	s.mu.Unlock()
	if !ok {
		return "", fmt.Errorf("unexpected shell call: %s", key)
	}
	return response.output, response.err
}

func testIntegration(name string, dependencies ...Dependency) Integration {
	return Integration{
		Name:         name,
		Dependencies: dependencies,
		Detect:       DetectSpec{Command: name + " detect"},
		Install:      InstallSpec{Command: name + " install"},
		Setup:        SetupSpec{Command: name + " setup {path}", WorkingDir: "worktree"},
	}
}

func plannedLabels(plan progress.Plan) []string {
	var labels []string
	for _, phase := range plan.Phases {
		for _, step := range phase.Steps {
			labels = append(labels, phase.Label+"/"+step.Label)
		}
	}
	return labels
}

func plannedIdentities(plan progress.Plan) map[string]string {
	identities := make(map[string]string)
	for _, phase := range plan.Phases {
		for _, step := range phase.Steps {
			identities[phase.Label+"/"+step.Label] = phase.ID + "\x00" + step.ID
		}
	}
	return identities
}

func collectPreparedEvents(prepared PreparedApply, shell *applyShell) ([]progress.Event, []progress.Phase) {
	var events []progress.Event
	phases, err := prepared.Run(shell, func(event progress.Event) { events = append(events, event) })
	if err != nil {
		panic(err)
	}
	return events, phases
}

func assertSettledStream(t *testing.T, events []progress.Event) {
	t.Helper()
	if err := progress.ValidateStream(events); err != nil {
		t.Fatalf("invalid stream: %v\n%#v", err, events)
	}
	for _, phase := range progress.Snapshot(events) {
		if phase.Total > 0 && !phase.Settled() {
			t.Fatalf("phase did not settle: %#v", phase)
		}
	}
}

func TestPrepareApply_MissingToolPlansPrerequisiteOnce(t *testing.T) {
	integ := testIntegration("tool", Dependency{Name: "dep", Detect: "dep detect"})
	probeDir := "/wt/a"
	shell := &applyShell{responses: map[string]mockShellResponse{
		probeDir + ":shell[tool detect]": {err: errors.New("missing")},
		probeDir + ":shell[dep detect]":  {output: "present"},
	}}
	prepared, err := PrepareApply(shell, "/repo", probeDir, []Integration{integ}, nil, []string{probeDir, "/wt/b"})
	if err != nil {
		t.Fatal(err)
	}
	labels := plannedLabels(prepared.Plan())
	if got := strings.Count(strings.Join(labels, "\n"), "Prerequisites/Install tool"); got != 1 {
		t.Fatalf("install steps = %d, want 1: %v", got, labels)
	}
	if got := strings.Count(strings.Join(labels, "\n"), "Setup tool"); got != 2 {
		t.Fatalf("setup steps = %d, want 2: %v", got, labels)
	}
}

func TestPrepareApplyForTarget_ProbesExistingWorktreeAndBindsSharedExecution(t *testing.T) {
	integ := testIntegration("tool")
	shell := &applyShell{responses: map[string]mockShellResponse{
		"/repo/main:shell[tool detect]":                   {output: "installed"},
		"/repo/feature:shell[tool setup '/repo/feature']": {},
	}}
	prepared, err := PrepareApplyForTarget(shell, "/repo", "/repo/main", "/repo/main", []Integration{integ}, nil, []string{"/repo/feature"})
	if err != nil {
		t.Fatal(err)
	}
	prepared, err = prepared.BindPhase("integrations", "Integrations")
	if err != nil {
		t.Fatal(err)
	}
	execution, err := progress.Start(prepared.Plan(), func(progress.Event) {})
	if err != nil {
		t.Fatal(err)
	}
	if err := prepared.RunIn(execution, shell); err != nil {
		t.Fatal(err)
	}
	if err := execution.Finish("done"); err != nil {
		t.Fatal(err)
	}
	if got := prepared.Plan().Phases; len(got) != 1 || got[0].ID != "integrations" {
		t.Fatalf("bound phases = %#v", got)
	}
	for _, call := range shell.calls {
		if strings.Contains(call, "/repo/feature:shell[tool detect]") {
			t.Fatalf("future target was probed: %v", shell.calls)
		}
	}
}

func TestPreparedApply_InstallFailureSkipsEverySetup(t *testing.T) {
	integ := testIntegration("tool")
	shell := &applyShell{responses: map[string]mockShellResponse{
		"/wt/a:shell[tool detect]":        {err: errors.New("missing")},
		"/wt/a:shell[tool install]":       {err: errors.New("install broke")},
		"/wt/a:shell[tool setup '/wt/a']": {output: "must not run"},
		"/wt/b:shell[tool setup '/wt/b']": {output: "must not run"},
	}}
	prepared, err := PrepareApply(shell, "/repo", "/wt/a", []Integration{integ}, nil, []string{"/wt/a", "/wt/b"})
	if err != nil {
		t.Fatal(err)
	}
	events, _ := collectPreparedEvents(prepared, shell)
	assertSettledStream(t, events)
	states := progress.Snapshot(events)
	for _, phase := range states {
		for _, step := range phase.Steps {
			if strings.HasPrefix(step.Name, "Setup tool") && (step.Status != progress.StepSkipped || !strings.Contains(step.Message, "blocked by Install tool")) {
				t.Fatalf("setup step = %#v", step)
			}
		}
	}
}

func TestPreparedApply_DeclarationTotalIsFixedAcrossPrefixes(t *testing.T) {
	a := testIntegration("alpha")
	b := testIntegration("beta")
	shell := &applyShell{responses: map[string]mockShellResponse{
		"/wt/a:shell[alpha detect]":        {output: "installed"},
		"/wt/a:shell[beta detect]":         {err: errors.New("missing")},
		"/wt/a:shell[alpha setup '/wt/a']": {output: "ok"},
		"/wt/b:shell[alpha setup '/wt/b']": {output: "ok"},
		"/wt/a:shell[beta install]":        {output: "ok"},
		"/wt/a:shell[beta setup '/wt/a']":  {output: "ok"},
		"/wt/b:shell[beta setup '/wt/b']":  {output: "ok"},
	}}
	prepared, err := PrepareApply(shell, "/repo", "/wt/a", []Integration{a, b}, nil, []string{"/wt/a", "/wt/b"})
	if err != nil {
		t.Fatal(err)
	}
	events, _ := collectPreparedEvents(prepared, shell)
	assertSettledStream(t, events)
	declarationEnd := 0
	for i, event := range events {
		if event.Close {
			declarationEnd = i + 1
		}
	}
	want := -1
	for i := declarationEnd; i <= len(events); i++ {
		_, total := progress.CheckpointProgress(progress.Snapshot(events[:i]))
		if want < 0 {
			want = total
		} else if total != want {
			t.Fatalf("prefix %d total = %d, want fixed %d", i, total, want)
		}
	}
}

func TestPrepareApply_InstalledToolPlansSetupOnlyAndDoesNotReprobe(t *testing.T) {
	integ := testIntegration("tool", Dependency{Name: "dep", Detect: "dep detect"})
	shell := &applyShell{responses: map[string]mockShellResponse{
		"/wt/a:shell[tool detect]":        {output: "installed"},
		"/wt/a:shell[dep detect]":         {output: "installed"},
		"/wt/a:shell[tool setup '/wt/a']": {output: "ok"},
	}}
	prepared, err := PrepareApply(shell, "/repo", "/wt/a", []Integration{integ}, nil, []string{"/wt/a"})
	if err != nil {
		t.Fatal(err)
	}
	labels := strings.Join(plannedLabels(prepared.Plan()), "\n")
	if strings.Contains(labels, "Install") || strings.Count(labels, "Setup tool") != 1 {
		t.Fatalf("plan = %s", labels)
	}
	events, _ := collectPreparedEvents(prepared, shell)
	assertSettledStream(t, events)
	shell.mu.Lock()
	calls := append([]string(nil), shell.calls...)
	shell.mu.Unlock()
	if strings.Count(strings.Join(calls, "\n"), "tool detect") != 1 || strings.Count(strings.Join(calls, "\n"), "dep detect") != 1 {
		t.Fatalf("detection calls repeated: %v", calls)
	}
}

func TestPreparedApply_PlanInspectionCannotMutateExecution(t *testing.T) {
	integ := testIntegration("tool")
	shell := &applyShell{responses: map[string]mockShellResponse{
		"/wt/a:shell[tool detect]":        {output: "installed"},
		"/wt/a:shell[tool setup '/wt/a']": {output: "ok"},
	}}
	prepared, err := PrepareApply(shell, "/repo", "/wt/a", []Integration{integ}, nil, []string{"/wt/a"})
	if err != nil {
		t.Fatal(err)
	}

	inspection := prepared.Plan()
	inspection.Phases[0].Label = "mutated phase"
	inspection.Phases[0].Steps[0].Label = "mutated step"

	var events []progress.Event
	if _, err := prepared.Run(shell, func(event progress.Event) { events = append(events, event) }); err != nil {
		t.Fatal(err)
	}
	states := progress.Snapshot(events)
	if len(states) != 1 || states[0].Name != "/wt/a" || states[0].Steps[0].Name != "Setup tool" {
		t.Fatalf("inspection mutated execution: %#v", states)
	}
}

func TestPreparedApply_Empty(t *testing.T) {
	prepared, err := PrepareApply(&applyShell{}, "/repo", "", nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !prepared.Empty() {
		t.Fatal("an apply with no frozen operations must be empty")
	}

	prepared, err = PrepareApply(&applyShell{}, "/repo", "/wt/a", nil, []Integration{{
		Name: "tool", Teardown: TeardownSpec{Dirs: []string{".tool/"}},
	}}, []string{"/wt/a"})
	if err != nil {
		t.Fatal(err)
	}
	if prepared.Empty() {
		t.Fatal("an apply with a frozen operation must not be empty")
	}
}

func TestPreparedApply_RunSurfacesStartError(t *testing.T) {
	prepared := PreparedApply{plan: progress.Plan{Phases: []progress.PlannedPhase{{Label: "missing ID"}}}}
	if _, err := prepared.Run(&applyShell{}, func(progress.Event) {}); err == nil || !strings.Contains(err.Error(), "empty ID") {
		t.Fatalf("Run error = %v, want invalid-plan error", err)
	}
}

func TestPreparedApply_RunSurfacesTransitionDeliveryError(t *testing.T) {
	phaseID := progress.PhaseID("phase")
	stepID := progress.StepID("declared")
	prepared := PreparedApply{
		plan: progress.Plan{Phases: []progress.PlannedPhase{{
			ID: phaseID, Label: "Phase", Steps: []progress.PlannedStep{{ID: stepID, Label: "Step"}},
		}}},
		operations: []applyOperation{{
			phaseID: phaseID, phaseName: "Phase", stepID: progress.StepID("other"), label: "Step",
			kind: applyFailure, failure: errors.New("failed"),
		}},
	}
	if _, err := prepared.Run(&applyShell{}, func(progress.Event) {}); err == nil || !strings.Contains(err.Error(), "no step ID") {
		t.Fatalf("Run error = %v, want transition error", err)
	}
}

func TestPreparedApply_RunSurfacesFinishDeliveryError(t *testing.T) {
	phaseID := progress.PhaseID("phase")
	stepID := progress.StepID("declared")
	prepared := PreparedApply{plan: progress.Plan{Phases: []progress.PlannedPhase{{
		ID: phaseID, Label: "Phase", Steps: []progress.PlannedStep{{ID: stepID, Label: "Step"}},
	}}}}
	emitErr := errors.New("sink closed")
	_, err := prepared.Run(&applyShell{}, func(event progress.Event) {
		if event.Status == progress.StepSkipped {
			panic(emitErr)
		}
	})
	if !errors.Is(err, emitErr) {
		t.Fatalf("Run error = %v, want %v", err, emitErr)
	}
}

func TestPrepareApply_DisableOnlyWithoutTargetsFails(t *testing.T) {
	_, err := PrepareApply(&applyShell{}, "/repo", "", nil, []Integration{{Name: "tool"}}, nil)
	if err == nil || !strings.Contains(err.Error(), "no target worktree") {
		t.Fatalf("PrepareApply error = %v, want no-target error", err)
	}
}

func TestPreparedApply_DependencyInstallFailureSkipsToolAndSetup(t *testing.T) {
	integ := testIntegration("tool", Dependency{Name: "dep", Detect: "dep detect", Install: "dep install"})
	shell := &applyShell{responses: map[string]mockShellResponse{
		"/wt/a:shell[tool detect]": {err: errors.New("missing")},
		"/wt/a:shell[dep detect]":  {err: errors.New("missing")},
		"/wt/a:shell[dep install]": {err: errors.New("dep broke")},
	}}
	prepared, err := PrepareApply(shell, "/repo", "/wt/a", []Integration{integ}, nil, []string{"/wt/a"})
	if err != nil {
		t.Fatal(err)
	}
	events, _ := collectPreparedEvents(prepared, shell)
	assertSettledStream(t, events)
	if step := findStep(t, events, "Install dependency dep"); step.Status != progress.StepFailed {
		t.Fatalf("dependency = %#v", step)
	}
	if step := findStep(t, events, "Install tool"); step.Status != progress.StepSkipped || !strings.Contains(step.Message, "blocked by Install dependency dep") {
		t.Fatalf("tool = %#v", step)
	}
	if step := findStep(t, events, "Setup tool"); step.Status != progress.StepSkipped || !strings.Contains(step.Message, "blocked by Install tool") {
		t.Fatalf("setup = %#v", step)
	}
}

func TestPreparedApply_MissingInstallableDependencyRunsPipeline(t *testing.T) {
	integ := testIntegration("tool", Dependency{Name: "dep", Detect: "dep detect", Install: "dep install"})
	shell := &applyShell{responses: map[string]mockShellResponse{
		"/wt/a:shell[tool detect]":        {err: errors.New("missing")},
		"/wt/a:shell[dep detect]":         {err: errors.New("missing")},
		"/wt/a:shell[dep install]":        {output: "ok"},
		"/wt/a:shell[tool install]":       {output: "ok"},
		"/wt/a:shell[tool setup '/wt/a']": {output: "ok"},
	}}
	prepared, err := PrepareApply(shell, "/repo", "/wt/a", []Integration{integ}, nil, []string{"/wt/a"})
	if err != nil {
		t.Fatal(err)
	}
	events, _ := collectPreparedEvents(prepared, shell)
	assertSettledStream(t, events)
	for _, label := range []string{"Install dependency dep", "Install tool", "Setup tool"} {
		if step := findStep(t, events, label); step.Status != progress.StepDone {
			t.Fatalf("%s = %#v", label, step)
		}
	}
}

func TestPreparedApply_MissingDependencyWithoutInstallerFailsHonestly(t *testing.T) {
	integ := testIntegration("tool", Dependency{Name: "dep", Detect: "dep detect"})
	shell := &applyShell{responses: map[string]mockShellResponse{
		"/wt/a:shell[tool detect]": {err: errors.New("missing")},
		"/wt/a:shell[dep detect]":  {err: errors.New("missing")},
	}}
	prepared, err := PrepareApply(shell, "/repo", "/wt/a", []Integration{integ}, nil, []string{"/wt/a"})
	if err != nil {
		t.Fatal(err)
	}
	events, _ := collectPreparedEvents(prepared, shell)
	assertSettledStream(t, events)
	dep := findStep(t, events, "Install dependency dep")
	if dep.Status != progress.StepFailed || dep.Error == nil || !strings.Contains(dep.Error.Error(), "no install command") {
		t.Fatalf("dependency = %#v", dep)
	}
}

func TestPreparedApply_SetupFailureStillSettles(t *testing.T) {
	integ := testIntegration("tool")
	shell := &applyShell{responses: map[string]mockShellResponse{
		"/wt/a:shell[tool detect]":        {output: "installed"},
		"/wt/a:shell[tool setup '/wt/a']": {err: errors.New("setup broke")},
	}}
	prepared, err := PrepareApply(shell, "/repo", "/wt/a", []Integration{integ}, nil, []string{"/wt/a"})
	if err != nil {
		t.Fatal(err)
	}
	events, _ := collectPreparedEvents(prepared, shell)
	assertSettledStream(t, events)
	if step := findStep(t, events, "Setup tool"); step.Status != progress.StepFailed {
		t.Fatalf("setup = %#v", step)
	}
}

func TestPreparedApply_TeardownFailureStillRemovesArtifacts(t *testing.T) {
	wt := t.TempDir()
	artifact := filepath.Join(wt, ".tool")
	if err := os.MkdirAll(artifact, 0o755); err != nil {
		t.Fatal(err)
	}
	integ := Integration{Name: "tool", Teardown: TeardownSpec{Command: "tool clean", Dirs: []string{".tool/"}}}
	shell := &applyShell{responses: map[string]mockShellResponse{wt + ":shell[tool clean]": {err: errors.New("clean broke")}}}
	prepared, err := PrepareApply(shell, "/repo", wt, nil, []Integration{integ}, []string{wt})
	if err != nil {
		t.Fatal(err)
	}
	events, _ := collectPreparedEvents(prepared, shell)
	assertSettledStream(t, events)
	if _, err := os.Stat(artifact); !os.IsNotExist(err) {
		t.Fatalf("artifact still exists: %v", err)
	}
	if step := findStep(t, events, "Teardown tool"); step.Status != progress.StepFailed {
		t.Fatalf("teardown = %#v", step)
	}
	if step := findStep(t, events, RemoveDirStepName(".tool/", wt)); step.Status != progress.StepDone {
		t.Fatalf("removal = %#v", step)
	}
}

func TestPreparedApply_RevalidatesTeardownPathAfterCommand(t *testing.T) {
	wt := t.TempDir()
	outside := t.TempDir()
	managedParent := filepath.Join(wt, "managed")
	outsideArtifact := filepath.Join(outside, "artifact")
	if err := os.MkdirAll(filepath.Join(managedParent, "artifact"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outsideArtifact, 0o755); err != nil {
		t.Fatal(err)
	}
	integ := Integration{Name: "tool", Teardown: TeardownSpec{Command: "tool clean", Dirs: []string{"managed/artifact"}}}
	shell := applyShellFunc(func(dir, command string) (string, error) {
		if dir != wt || command != "tool clean" {
			return "", fmt.Errorf("unexpected shell call: %s %s", dir, command)
		}
		if err := os.RemoveAll(managedParent); err != nil {
			return "", err
		}
		return "", os.Symlink(outside, managedParent)
	})
	prepared, err := PrepareApply(shell, "/repo", wt, nil, []Integration{integ}, []string{wt})
	if err != nil {
		t.Fatal(err)
	}
	var events []progress.Event
	if _, err := prepared.Run(shell, func(event progress.Event) { events = append(events, event) }); err != nil {
		t.Fatal(err)
	}
	if step := findStep(t, events, RemoveDirStepName("managed/artifact", wt)); step.Status != progress.StepFailed || step.Error == nil || !strings.Contains(step.Error.Error(), "symlink") {
		t.Fatalf("removal step = %#v, want visible symlink failure", step)
	}
	if _, err := os.Stat(outsideArtifact); err != nil {
		t.Fatalf("outside artifact was touched: %v", err)
	}
}

func TestPreparedApply_RevalidatesIndexDestinationAfterInstall(t *testing.T) {
	mainWT := t.TempDir()
	targetWT := t.TempDir()
	outside := t.TempDir()
	sourceIndex := filepath.Join(mainWT, "managed", "index")
	targetParent := filepath.Join(targetWT, "managed")
	outsideIndex := filepath.Join(outside, "index")
	if err := os.MkdirAll(sourceIndex, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(targetParent, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outsideIndex, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(outsideIndex, "keep")
	if err := os.WriteFile(marker, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	integ := Integration{
		Name: "tool", Detect: DetectSpec{Command: "tool detect"},
		Install: InstallSpec{Command: "tool install"}, IndexCopyDir: "managed/index",
	}
	shell := applyShellFunc(func(dir, command string) (string, error) {
		switch command {
		case "tool detect":
			return "", errors.New("missing")
		case "tool install":
			if err := os.RemoveAll(targetParent); err != nil {
				return "", err
			}
			return "", os.Symlink(outside, targetParent)
		default:
			return "", fmt.Errorf("unexpected shell call: %s %s", dir, command)
		}
	})
	prepared, err := PrepareApply(shell, "/repo", mainWT, []Integration{integ}, nil, []string{targetWT})
	if err != nil {
		t.Fatal(err)
	}
	var events []progress.Event
	if _, err := prepared.Run(shell, func(event progress.Event) { events = append(events, event) }); err != nil {
		t.Fatal(err)
	}
	if step := findStep(t, events, "Copy index for tool"); step.Status != progress.StepFailed || step.Error == nil || !strings.Contains(step.Error.Error(), "symlink") {
		t.Fatalf("copy step = %#v, want visible symlink failure", step)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("outside index was touched: %v", err)
	}
}

func TestPrepareApplySemanticIDsDoNotDependOnInputOrder(t *testing.T) {
	alpha := testIntegration("alpha")
	beta := testIntegration("beta")
	responses := map[string]mockShellResponse{
		"/wt/a:shell[alpha detect]": {output: "installed"},
		"/wt/a:shell[beta detect]":  {output: "installed"},
		"/wt/b:shell[alpha detect]": {output: "installed"},
		"/wt/b:shell[beta detect]":  {output: "installed"},
	}
	first, err := PrepareApply(&applyShell{responses: responses}, "/repo", "/wt/a", []Integration{alpha, beta}, nil, []string{"/wt/a", "/wt/b"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := PrepareApply(&applyShell{responses: responses}, "/repo", "/wt/a", []Integration{beta, alpha}, nil, []string{"/wt/b", "/wt/a"})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := plannedIdentities(second.Plan()), plannedIdentities(first.Plan()); !reflect.DeepEqual(got, want) {
		t.Fatalf("reordered identities = %#v, want %#v", got, want)
	}
}

func TestPrepareApplyRejectsDuplicateIntegrationIdentityBeforeDetection(t *testing.T) {
	integ := testIntegration("tool")
	shell := &applyShell{}
	_, err := PrepareApply(shell, "/repo", "/wt/a", []Integration{integ, integ}, nil, []string{"/wt/a"})
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("PrepareApply error = %v, want duplicate identity error", err)
	}
	if len(shell.calls) != 0 {
		t.Fatalf("detection ran before duplicate validation: %v", shell.calls)
	}
}

func TestPrepareApplyRejectsConflictingDependencySpecsBeforeDetection(t *testing.T) {
	one := testIntegration("one", Dependency{Name: "dep", Detect: "dep one", Install: "install one"})
	two := testIntegration("two", Dependency{Name: "dep", Detect: "dep two", Install: "install two"})
	shell := &applyShell{}
	_, err := PrepareApply(shell, "/repo", "/wt/a", []Integration{one, two}, nil, []string{"/wt/a"})
	if err == nil || !strings.Contains(err.Error(), "conflicting") {
		t.Fatalf("PrepareApply error = %v, want dependency conflict", err)
	}
	if len(shell.calls) != 0 {
		t.Fatalf("detection ran before dependency validation: %v", shell.calls)
	}
}

func TestPrepareApplyRejectsMalformedDetectionBeforeSideEffects(t *testing.T) {
	tests := []struct {
		name  string
		integ Integration
	}{
		{name: "empty integration detect", integ: Integration{Name: "tool", Setup: SetupSpec{Command: "setup"}}},
		{name: "empty dependency detect", integ: testIntegration("tool", Dependency{Name: "dep", Install: "dep install"})},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			shell := &applyShell{}
			_, err := PrepareApply(shell, "/repo", "/wt/a", []Integration{tc.integ}, nil, []string{"/wt/a"})
			if err == nil {
				t.Fatal("malformed detection accepted")
			}
			if len(shell.calls) != 0 {
				t.Fatalf("shell called before preflight completed: %v", shell.calls)
			}
		})
	}
}

func TestPrepareApplyRejectsUnsafeManagedPathsBeforeDetection(t *testing.T) {
	tests := []struct {
		name      string
		toEnable  []Integration
		toDisable []Integration
	}{
		{
			name: "absolute index copy directory",
			toEnable: []Integration{{
				Name: "tool", Detect: DetectSpec{Command: "tool detect"}, IndexCopyDir: filepath.Join(string(filepath.Separator), "victim"),
			}},
		},
		{
			name: "parent index copy directory",
			toEnable: []Integration{{
				Name: "tool", Detect: DetectSpec{Command: "tool detect"}, IndexCopyDir: "../victim",
			}},
		},
		{
			name: "unclean index copy directory",
			toEnable: []Integration{{
				Name: "tool", Detect: DetectSpec{Command: "tool detect"}, IndexCopyDir: "cache/../victim",
			}},
		},
		{
			name:      "current teardown directory",
			toDisable: []Integration{{Name: "tool", Teardown: TeardownSpec{Dirs: []string{"."}}}},
		},
		{
			name:      "empty teardown directory",
			toDisable: []Integration{{Name: "tool", Teardown: TeardownSpec{Dirs: []string{""}}}},
		},
		{
			name:      "parent teardown directory",
			toDisable: []Integration{{Name: "tool", Teardown: TeardownSpec{Dirs: []string{"../victim"}}}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			shell := &applyShell{}
			_, err := PrepareApplyForTarget(shell, t.TempDir(), t.TempDir(), t.TempDir(), tc.toEnable, tc.toDisable, []string{t.TempDir()})
			if err == nil || !strings.Contains(err.Error(), "managed path") {
				t.Fatalf("PrepareApplyForTarget error = %v, want managed path validation error", err)
			}
			if len(shell.calls) != 0 {
				t.Fatalf("detection ran before managed path validation: %v", shell.calls)
			}
		})
	}
}

func TestPrepareApplyRejectsSymlinkedManagedPathBeforeDetection(t *testing.T) {
	outside := t.TempDir()
	tests := []struct {
		name      string
		toEnable  []Integration
		toDisable []Integration
		linkRoot  string
	}{
		{
			name: "index source",
			toEnable: []Integration{{
				Name: "tool", Detect: DetectSpec{Command: "tool detect"}, IndexCopyDir: "escape/index",
			}},
			linkRoot: "main",
		},
		{
			name: "index destination",
			toEnable: []Integration{{
				Name: "tool", Detect: DetectSpec{Command: "tool detect"}, IndexCopyDir: "escape/index",
			}},
			linkRoot: "target",
		},
		{
			name:      "teardown destination",
			toDisable: []Integration{{Name: "tool", Teardown: TeardownSpec{Dirs: []string{"escape/artifact"}}}},
			linkRoot:  "target",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mainWT := t.TempDir()
			targetWT := t.TempDir()
			linkBase := mainWT
			if tc.linkRoot == "target" {
				linkBase = targetWT
			}
			if err := os.Symlink(outside, filepath.Join(linkBase, "escape")); err != nil {
				t.Fatal(err)
			}
			shell := &applyShell{}
			_, err := PrepareApplyForTarget(shell, t.TempDir(), mainWT, targetWT, tc.toEnable, tc.toDisable, []string{targetWT})
			if err == nil || !strings.Contains(err.Error(), "symlink") {
				t.Fatalf("PrepareApplyForTarget error = %v, want symlink validation error", err)
			}
			if len(shell.calls) != 0 {
				t.Fatalf("detection ran before symlink validation: %v", shell.calls)
			}
		})
	}
}

func TestPrepareApplyDetectionFallsBackFromCommandToBinary(t *testing.T) {
	integ := Integration{
		Name: "tool", Detect: DetectSpec{Command: "tool detect", BinaryName: "tool"},
		Setup: SetupSpec{Command: "tool setup"},
	}
	shell := &applyShell{responses: map[string]mockShellResponse{
		"/wt/a:shell[tool detect]":     {err: errors.New("command unavailable")},
		"/wt/a:shell[command -v tool]": {output: "/usr/local/bin/tool"},
	}}
	prepared, err := PrepareApply(shell, "/repo", "/wt/a", []Integration{integ}, nil, []string{"/wt/a"})
	if err != nil {
		t.Fatal(err)
	}
	labels := strings.Join(plannedLabels(prepared.Plan()), "\n")
	if strings.Contains(labels, "Install tool") || !strings.Contains(labels, "Setup tool") {
		t.Fatalf("fallback detection plan = %s", labels)
	}
	wantCalls := []string{"/wt/a:shell[tool detect]", "/wt/a:shell[command -v tool]"}
	if !reflect.DeepEqual(shell.calls, wantCalls) {
		t.Fatalf("detection calls = %v, want %v", shell.calls, wantCalls)
	}
}

func TestPreparedApplyRejectsInvalidOperationGraphBeforeStart(t *testing.T) {
	op := func(id string, dependencies ...string) applyOperation {
		return applyOperation{
			phaseID: "phase", phaseName: "Phase", stepID: progress.StepID(id), label: id,
			kind: applyShellCommand, dir: "/wt", command: id, dependsOn: dependencies,
		}
	}
	tests := []struct {
		name       string
		operations []applyOperation
	}{
		{name: "missing reference", operations: []applyOperation{op("a", "phase\x00missing")}},
		{name: "out of order", operations: []applyOperation{op("a", "phase\x00b"), op("b")}},
		{name: "cycle", operations: []applyOperation{op("a", "phase\x00b"), op("b", "phase\x00a")}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			prepared := PreparedApply{plan: planForOperations(tc.operations), operations: tc.operations}
			shell := &applyShell{}
			var events []progress.Event
			if _, err := prepared.Run(shell, func(event progress.Event) { events = append(events, event) }); err == nil {
				t.Fatal("invalid operation graph accepted")
			}
			if len(events) != 0 || len(shell.calls) != 0 {
				t.Fatalf("side effects before graph validation: events=%#v calls=%v", events, shell.calls)
			}
		})
	}
}

func TestPrepareApplyOmitsStaticallyEmptyOperations(t *testing.T) {
	shell := &applyShell{responses: map[string]mockShellResponse{
		"/wt/a:shell[tool detect]": {output: "installed"},
	}}
	prepared, err := PrepareApply(shell, "/repo", "/wt/a", []Integration{{
		Name: "tool", Detect: DetectSpec{Command: "tool detect"},
	}}, []Integration{{Name: "old"}}, []string{"/wt/a"})
	if err != nil {
		t.Fatal(err)
	}
	if !prepared.Empty() || len(prepared.Plan().Phases) != 0 {
		t.Fatalf("empty operations were declared: %#v", prepared.Plan())
	}
}

func TestPrepareApplyKeepsMissingInstallerAsExplicitFailure(t *testing.T) {
	shell := &applyShell{responses: map[string]mockShellResponse{
		"/wt/a:shell[tool detect]": {err: errors.New("missing")},
	}}
	prepared, err := PrepareApply(shell, "/repo", "/wt/a", []Integration{{
		Name: "tool", Detect: DetectSpec{Command: "tool detect"},
	}}, nil, []string{"/wt/a"})
	if err != nil {
		t.Fatal(err)
	}
	labels := plannedLabels(prepared.Plan())
	if !reflect.DeepEqual(labels, []string{"Prerequisites/Install tool"}) {
		t.Fatalf("plan labels = %v, want only explicit installer failure", labels)
	}
}

func TestPrepareApplyDeclaresIndexCopyAndGitignoreOperations(t *testing.T) {
	mainWT := t.TempDir()
	targetWT := t.TempDir()
	if err := os.Mkdir(filepath.Join(mainWT, ".index"), 0o755); err != nil {
		t.Fatal(err)
	}
	integ := Integration{
		Name: "tool", Detect: DetectSpec{Command: "tool detect"},
		Setup: SetupSpec{Command: "tool setup"}, IndexCopyDir: ".index",
		GitignoreEntries: []string{".index/"},
	}
	shell := &applyShell{responses: map[string]mockShellResponse{
		targetWT + ":shell[tool detect]": {output: "installed"},
	}}
	prepared, err := PrepareApply(shell, "/repo", mainWT, []Integration{integ}, nil, []string{targetWT})
	if err != nil {
		t.Fatal(err)
	}
	labels := strings.Join(plannedLabels(prepared.Plan()), "\n")
	for _, label := range []string{"Copy index for tool", "Setup tool", "Update .gitignore for tool"} {
		if !strings.Contains(labels, label) {
			t.Fatalf("plan does not declare %q: %s", label, labels)
		}
	}
}

func TestPreparedApplySurfacesFileOperationFailures(t *testing.T) {
	tests := []struct {
		name  string
		op    applyOperation
		files failingApplyFiles
	}{
		{
			name: "copy", files: failingApplyFiles{copyErr: errors.New("copy broke")},
			op: applyOperation{phaseID: "p", phaseName: "Phase", stepID: "copy", label: "Copy", kind: applyCopy, seedSource: "source", seedDest: "dest"},
		},
		{
			name: "gitignore write", files: failingApplyFiles{gitignoreErr: errors.New("write broke")},
			op: applyOperation{phaseID: "p", phaseName: "Phase", stepID: "write", label: "Write", kind: applyGitignore, gitignoreDir: "dir", gitignore: []string{"entry"}},
		},
		{
			name: "remove", files: failingApplyFiles{removeErr: errors.New("remove broke")},
			op: applyOperation{phaseID: "p", phaseName: "Phase", stepID: "remove", label: "Remove", kind: applyRemove, dir: "dir"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			prepared := PreparedApply{plan: planForOperations([]applyOperation{tc.op}), operations: []applyOperation{tc.op}, files: tc.files}
			phases, err := prepared.Run(&applyShell{}, nil)
			if err != nil {
				t.Fatal(err)
			}
			step := phases[0].Steps[0]
			if step.Status != progress.StepFailed || step.Error == nil {
				t.Fatalf("file failure was not surfaced: %#v", step)
			}
		})
	}
}

func TestPreparedApplyReturnsExecutionProjectionAndCompletedStream(t *testing.T) {
	integ := testIntegration("tool")
	shell := &applyShell{responses: map[string]mockShellResponse{
		"/wt/a:shell[tool detect]":        {output: "installed"},
		"/wt/a:shell[tool setup '/wt/a']": {output: "configured"},
	}}
	prepared, err := PrepareApply(shell, "/repo", "/wt/a", []Integration{integ}, nil, []string{"/wt/a"})
	if err != nil {
		t.Fatal(err)
	}
	var events []progress.Event
	phases, err := prepared.Run(shell, func(event progress.Event) { events = append(events, event) })
	if err != nil {
		t.Fatal(err)
	}
	if err := progress.ValidateCompletedStream(events); err != nil {
		t.Fatalf("completed stream invalid: %v", err)
	}
	plan := prepared.Plan()
	if len(phases) != 1 || phases[0].ID != plan.Phases[0].ID || len(phases[0].Steps) != 1 ||
		phases[0].Steps[0].ID != plan.Phases[0].Steps[0].ID || phases[0].Steps[0].Status != progress.StepDone {
		t.Fatalf("Run projection = %#v, plan = %#v", phases, plan)
	}
}

func TestPrepareApplyDeduplicatesIdenticalDependencySpecs(t *testing.T) {
	dep := Dependency{Name: "dep", Detect: "dep detect", Install: "dep install"}
	one := testIntegration("one", dep)
	two := testIntegration("two", dep)
	shell := &applyShell{responses: map[string]mockShellResponse{
		"/wt/a:shell[one detect]": {err: errors.New("missing")},
		"/wt/a:shell[two detect]": {err: errors.New("missing")},
		"/wt/a:shell[dep detect]": {err: errors.New("missing")},
	}}
	prepared, err := PrepareApply(shell, "/repo", "/wt/a", []Integration{one, two}, nil, []string{"/wt/a"})
	if err != nil {
		t.Fatal(err)
	}
	labels := strings.Join(plannedLabels(prepared.Plan()), "\n")
	if strings.Count(labels, "Install dependency dep") != 1 {
		t.Fatalf("dependency operation was not deduplicated: %s", labels)
	}
	if strings.Count(strings.Join(shell.calls, "\n"), "dep detect") != 1 {
		t.Fatalf("dependency detection was not deduplicated: %v", shell.calls)
	}
}

func TestPreparedApplyPrerequisiteFailureBlocksOnlyDependentOperations(t *testing.T) {
	dependent := testIntegration("dependent", Dependency{Name: "dep", Detect: "dep detect", Install: "dep install"})
	independent := testIntegration("independent")
	shell := &applyShell{responses: map[string]mockShellResponse{
		"/wt/a:shell[dependent detect]":          {err: errors.New("missing")},
		"/wt/a:shell[independent detect]":        {output: "installed"},
		"/wt/a:shell[dep detect]":                {err: errors.New("missing")},
		"/wt/a:shell[dep install]":               {err: errors.New("dep broke")},
		"/wt/a:shell[independent setup '/wt/a']": {output: "ok"},
	}}
	prepared, err := PrepareApply(shell, "/repo", "/wt/a", []Integration{dependent, independent}, nil, []string{"/wt/a"})
	if err != nil {
		t.Fatal(err)
	}
	events, _ := collectPreparedEvents(prepared, shell)
	if got := findStep(t, events, "Setup independent"); got.Status != progress.StepDone {
		t.Fatalf("independent setup = %#v", got)
	}
	if got := findStep(t, events, "Setup dependent"); got.Status != progress.StepSkipped {
		t.Fatalf("dependent setup = %#v", got)
	}
}
