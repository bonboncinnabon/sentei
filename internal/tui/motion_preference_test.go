package tui

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/config"
	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/progress"
	"github.com/bonboncinnabon/sentei/internal/repo"
)

func TestMotionPreference_EnvironmentResolution(t *testing.T) {
	tests := []struct {
		name   string
		motion string
		term   string
		want   MotionPreference
	}{
		{name: "default", term: "xterm-256color", want: MotionFull},
		{name: "explicit off case insensitive", motion: "OFF", term: "xterm-256color", want: MotionOff},
		{name: "dumb terminal case insensitive", motion: "full", term: "DuMb", want: MotionOff},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := map[string]string{"SENTEI_MOTION": tc.motion, "TERM": tc.term}
			if got := motionPreference(func(key string) string { return env[key] }); got != tc.want {
				t.Fatalf("motionPreference = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestMotionOff_InitSchedulesNoDecorativeTick(t *testing.T) {
	t.Setenv("TERM", "dumb")
	m := NewMenuModel(nil, nil, "/repo", &config.Config{}, repo.ContextBareRepo)
	batch, ok := m.Init()().(tea.BatchMsg)
	if !ok {
		t.Fatalf("Init command = %T, want batch", m.Init()())
	}
	if len(batch) != 2 {
		t.Fatalf("static Init scheduled %d commands, want background detection and loading only", len(batch))
	}
}

func TestMotionPreference_ResolvedOnceAtConstruction(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("SENTEI_MOTION", "off")
	m := NewModel(nil, nil, "/repo")
	t.Setenv("SENTEI_MOTION", "full")
	if m.motionPreference != MotionOff {
		t.Fatalf("constructed preference changed with environment: %v", m.motionPreference)
	}
}

func TestMotionOff_GatesDecorativeCommandsButKeepsStopwatch(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("SENTEI_MOTION", "off")
	m := NewModel([]git.Worktree{}, nil, "/repo")
	m.view = progressView
	m.remove.run.events = []progress.Event{
		{Phase: "p", PhaseLabel: "P", Step: "a", StepLabel: "A", Status: progress.StepPending, Of: 2},
		{Phase: "p", Close: true},
		{Phase: "p", Step: "a", Status: progress.StepRunning, Checkpoint: 1, Of: 2},
	}

	if m.motionActive() {
		t.Fatal("decorative motion remains active")
	}
	if updated, cmd := m.Update(motionTickMsg{}); cmd != nil || updated.(Model).motionTick != 0 {
		t.Fatal("motion-off accepted or rescheduled a decorative tick")
	}
	cmd := m.syncProgressBar()
	if cmd == nil {
		t.Fatal("motion-off must still start the stopwatch")
	}
	if m.progressTarget != 0.5 {
		t.Fatalf("exact target = %v, want 0.5", m.progressTarget)
	}
	if m.bar.IsAnimating() {
		t.Fatal("motion-off scheduled spring frames")
	}
	if got := stripANSI(m.renderProgressLayout(m.removalLayout())); !strings.Contains(got, "50%") {
		t.Fatalf("static bar did not render exact target:\n%s", got)
	}
}

func TestMotionOff_CompletionRespectsNonDecorativeMinimumDuration(t *testing.T) {
	t.Setenv("SENTEI_MOTION", "off")
	m := NewModel(nil, nil, "/repo")
	m.view = progressView
	m.minProgressDuration = time.Hour
	m.progressStartedAt = time.Now()

	updated, cmd := m.holdOrAdvance(summaryView)
	if cmd == nil || updated.(Model).view != progressView {
		t.Fatal("static completion must retain the configured minimum visible duration")
	}
}

func TestProgressMotion_OnlyFocusedRunningHeadlineAnimates(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("SENTEI_MOTION", "full")
	m := NewModel(nil, nil, "/repo")
	m.view = progressView
	m.motionTick = 2
	layout := ProgressLayout{Title: "T", Width: 80, Height: 24, Phases: []progress.PhaseState{
		{ID: "history", Name: "Resolved history", Total: 1, Done: 1, Closed: true, Steps: []progress.StepState{{Name: "historic step", Status: progress.StepDone}}},
		{ID: "focus", Name: "Focused running", Total: 3, Done: 1, Closed: true, Steps: []progress.StepState{
			{Name: "resolved step", Status: progress.StepDone},
			{Name: "running step", Status: progress.StepRunning},
			{Name: "pending step", Status: progress.StepPending},
		}},
		{ID: "queued", Name: "Queued pending", Total: 1, Closed: true, Steps: []progress.StepState{{Name: "queued step", Status: progress.StepPending}}},
	}}
	out := m.renderProgressLayout(layout)
	lineFor := func(text string) string {
		for _, line := range strings.Split(out, "\n") {
			if strings.Contains(stripANSI(line), text) {
				return line
			}
		}
		return ""
	}
	animatedHeadline := m.motion().Accent(starFrame(m.motionTick) + " Focused running")
	if line := lineFor("Focused running"); line == "" || !strings.Contains(line, animatedHeadline) {
		t.Fatalf("focused running headline is not shimmered: %q", line)
	}
	for _, static := range []string{"Resolved history", "running step", "pending step", "phase waiting"} {
		if line := lineFor(static); line == "" || strings.Contains(line, starFrame(m.motionTick)) {
			t.Errorf("%q must remain static: %q", static, line)
		}
	}
	if got := strings.Count(stripANSI(out), starFrame(m.motionTick)); got != 1 {
		t.Fatalf("animated frame appears %d times, want exactly one focused headline:\n%s", got, stripANSI(out))
	}
}

func TestCompletedFailureBarNeverUsesSuccessPalette(t *testing.T) {
	t.Setenv("SENTEI_MOTION", "off")
	m := NewModel(nil, nil, "/repo")
	m.view = progressView
	m.progressTarget = 1
	layout := ProgressLayout{Title: "T", Width: 80, Height: 24, Completed: true, Phases: []progress.PhaseState{{
		Name: "Failed", Total: 1, Done: 1, Failed: 1, Closed: true, Steps: []progress.StepState{{Name: "failure", Status: progress.StepFailed}},
	}}}
	if out := m.renderProgressLayout(layout); strings.Contains(out, "0;135;95") {
		t.Fatalf("failure completion used success palette: %q", out)
	}
}

func TestMotionOff_SuccessCrystallizesOnceWithGreenBar(t *testing.T) {
	t.Setenv("SENTEI_MOTION", "off")
	m := NewModel(nil, nil, "/repo")
	m.view = progressView
	m.progressTarget = 1
	layout := ProgressLayout{Title: "T", Width: 80, Height: 24, Completed: true, Phases: []progress.PhaseState{{
		Name: "Complete", Total: 1, Done: 1, Closed: true,
	}}}
	out := m.renderProgressLayout(layout)
	if !strings.Contains(out, "0;135;95") {
		t.Fatalf("failure-free completion did not use success palette: %q", out)
	}
	if got := strings.Count(stripANSI(out), indicatorDone); got != 1 {
		t.Fatalf("completion crystallized %d times, want one:\n%s", got, stripANSI(out))
	}
}

func TestMotionTick_DoesNotRescheduleAfterSettleTransition(t *testing.T) {
	m := NewModel(nil, nil, "/repo")
	m.view = progressView
	m.progressTargetView = summaryView
	m.progressSettling = true
	m.progressSettlingSince = time.Now()
	m.progressSettledAt = time.Now().Add(-progressSettleBeat)

	updated, cmd := m.Update(motionTickMsg{})
	model := updated.(Model)
	if model.view != progressView || !model.progressTransitionPending {
		t.Fatalf("settle tick must schedule transition, view=%v pending=%v", model.view, model.progressTransitionPending)
	}
	if cmd == nil {
		t.Fatal("settle tick must schedule the refresh-then-transition sequence")
	}
	if _, rescheduledMotion := cmd().(motionTickMsg); rescheduledMotion {
		t.Fatal("settled motion tick rescheduled decorative motion instead of the transition")
	}
}

func TestSettledProgressTransition_DoesNotReplayCapturedWindowSize(t *testing.T) {
	cmd := settledProgressTransitionCmd(7)
	sequence := reflect.ValueOf(cmd())
	if sequence.Kind() != reflect.Slice {
		t.Fatalf("transition command emitted %T, want ordered command sequence", cmd())
	}

	var messages []tea.Msg
	for i := 0; i < sequence.Len(); i++ {
		step, ok := sequence.Index(i).Interface().(tea.Cmd)
		if !ok {
			t.Fatalf("sequence step %d has type %T, want tea.Cmd", i, sequence.Index(i).Interface())
		}
		messages = append(messages, step())
	}
	for _, msg := range messages {
		if size, ok := msg.(tea.WindowSizeMsg); ok {
			t.Fatalf("transition replayed stale terminal size %dx%d", size.Width, size.Height)
		}
	}
	if len(messages) != 2 || reflect.TypeOf(messages[0]) != reflect.TypeOf(tea.ClearScreen()) {
		t.Fatalf("transition messages = %T, %T; want clear then private transition", messages[0], messages[1])
	}
	if transition, ok := messages[1].(progressTransitionMsg); !ok || transition.token != 7 {
		t.Fatalf("final transition message = %#v, want live token", messages[1])
	}
}

func TestCompletedTopLevelErrorBarNeverUsesSuccessPalette(t *testing.T) {
	t.Setenv("SENTEI_MOTION", "off")
	m := NewModel(nil, nil, "/repo")
	m.view = progressView
	m.progressTarget = 1
	m.remove.run.result.Err = errors.New("progress delivery failed")
	layout := ProgressLayout{Title: "T", Width: 80, Height: 24, Completed: true, Phases: []progress.PhaseState{{
		Name: "Complete", Total: 1, Done: 1, Closed: true,
	}}}
	if out := m.renderProgressLayout(layout); strings.Contains(out, "0;135;95") {
		t.Fatalf("top-level progress error used success palette: %q", out)
	}
}

func TestMotionOff_CompletionAdvancesWithoutDecorativeDelay(t *testing.T) {
	t.Setenv("TERM", "dumb")
	m := NewModel(nil, nil, "/repo")
	m.view = progressView
	m.progressStartedAt = time.Now()
	m.progressTarget = 1

	updated, cmd := m.holdOrAdvance(summaryView)
	model := updated.(Model)
	if cmd == nil || model.view != progressView || !model.progressTransitionPending {
		t.Fatalf("static completion = view %v pending=%v cmd=%v, want immediate refresh-then-transition", model.view, model.progressTransitionPending, cmd != nil)
	}
}
