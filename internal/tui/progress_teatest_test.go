package tui

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/bonboncinnabon/sentei/internal/cleanup"
	"github.com/bonboncinnabon/sentei/internal/progress"
)

type progressFrameRecorder struct {
	mu     sync.Mutex
	frames []progressRecordedFrame
	notify chan struct{}
}

type progressRecordedFrame struct {
	width, height int
	content       string
}

func newProgressFrameRecorder() *progressFrameRecorder {
	return &progressFrameRecorder{notify: make(chan struct{}, 1)}
}

func (r *progressFrameRecorder) add(m Model, content string) {
	height := m.progressHeight()
	if m.width <= 0 || height <= 0 {
		return
	}
	r.mu.Lock()
	r.frames = append(r.frames, progressRecordedFrame{m.width, height, content})
	r.mu.Unlock()
	select {
	case r.notify <- struct{}{}:
	default:
	}
}

func (r *progressFrameRecorder) snapshot() []progressRecordedFrame {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]progressRecordedFrame(nil), r.frames...)
}

func (r *progressFrameRecorder) waitFor(t *testing.T, condition func([]progressRecordedFrame) bool) {
	t.Helper()
	timer := time.NewTimer(testTimeout)
	defer timer.Stop()
	for {
		if condition(r.snapshot()) {
			return
		}
		select {
		case <-r.notify:
		case <-timer.C:
			t.Fatal("timed out waiting for progress frame")
		}
	}
}

type progressScenarioMsg func(*Model)
type progressScenarioQuitMsg struct{}

type recordedProgressModel struct {
	model    Model
	recorder *progressFrameRecorder
}

func (m recordedProgressModel) Init() tea.Cmd { return nil }

func (m recordedProgressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case progressScenarioMsg:
		msg(&m.model)
		return m, nil
	case progressScenarioQuitMsg:
		return m, tea.Quit
	default:
		updated, cmd := m.model.Update(msg)
		m.model = updated.(Model)
		return m, cmd
	}
}

func (m recordedProgressModel) View() tea.View {
	view := m.model.View()
	m.recorder.add(m.model, view.Content)
	return view
}

func TestTeatestProgressFramesStayTruthfulAndBounded(t *testing.T) {
	t.Setenv("SENTEI_MOTION", "off")
	recorder := newProgressFrameRecorder()
	m := NewModel(nil, nil, "/repo")
	m.view = progressView
	m.remove.run.events = removalProgressEvents(progress.StepRunning, progress.StepPending)

	tm := teatest.NewTestModel(t, recordedProgressModel{model: m, recorder: recorder}, teatest.WithInitialTermSize(80, 24))
	recorder.waitFor(t, func(frames []progressRecordedFrame) bool { return len(frames) > 0 })

	tm.Send(progressScenarioMsg(func(m *Model) {
		m.remove.run.events = removalProgressEvents(progress.StepDone, progress.StepRunning)
	}))
	recorder.waitFor(t, func(frames []progressRecordedFrame) bool {
		return frameContains(frames, "second worktree")
	})
	tm.Send(tea.WindowSizeMsg{Width: 52, Height: 16})
	recorder.waitFor(t, func(frames []progressRecordedFrame) bool {
		return frameWithSize(frames, 52, 16)
	})
	tm.Send(tea.WindowSizeMsg{Width: 80, Height: 24})
	tm.Send(progressScenarioMsg(func(m *Model) {
		m.remove.run.events = removalProgressEvents(progress.StepDone, progress.StepDone)
		result := cleanup.Result{}
		m.remove.run.cleanupResult = &result
	}))
	recorder.waitFor(t, func(frames []progressRecordedFrame) bool {
		return frameContains(frames, "100%") && frameWithSize(frames, 80, 24)
	})
	tm.Send(progressScenarioQuitMsg{})
	_ = tm.FinalModel(t, teatest.WithFinalTimeout(testTimeout))

	frames := recorder.snapshot()
	assertProgressFrameBounds(t, frames)
	assertProgressCheckpointsMonotonic(t, frames)
	last := stripANSI(frames[len(frames)-1].content)
	if strings.Contains(last, indicatorActiveFallback) || !strings.Contains(last, "100%") {
		t.Fatalf("successful terminal frame is not settled:\n%s", last)
	}
}

func TestTeatestFailedProgressFrameShowsBlockedSuffixes(t *testing.T) {
	t.Setenv("SENTEI_MOTION", "off")
	recorder := newProgressFrameRecorder()
	m := NewModel(nil, nil, "/repo")
	m.view = integrationProgressView
	m.integ.lifecycle = integrationExecuting
	m.integ.events = integrationFailureEvents()

	tm := teatest.NewTestModel(t, recordedProgressModel{model: m, recorder: recorder}, teatest.WithInitialTermSize(80, 24))
	recorder.waitFor(t, func(frames []progressRecordedFrame) bool {
		return frameContains(frames, "dependency failed")
	})
	tm.Send(progressScenarioMsg(func(m *Model) { m.integ.lifecycle = integrationSettling }))
	recorder.waitFor(t, func(frames []progressRecordedFrame) bool {
		return frameContains(frames, "index exited 17")
	})
	tm.Send(progressScenarioQuitMsg{})
	_ = tm.FinalModel(t, teatest.WithFinalTimeout(testTimeout))

	frames := recorder.snapshot()
	assertProgressFrameBounds(t, frames)
	last := stripANSI(frames[len(frames)-1].content)
	for _, want := range []string{"index exited 17", "skipped (dependency failed)"} {
		if !strings.Contains(last, want) {
			t.Fatalf("failed terminal frame missing %q:\n%s", want, last)
		}
	}
	if strings.Contains(last, indicatorActiveFallback) {
		t.Fatalf("failed terminal frame retained active glyph:\n%s", last)
	}
}

func assertProgressFrameBounds(t *testing.T, frames []progressRecordedFrame) {
	t.Helper()
	for i, frame := range frames {
		lines := strings.Split(stripANSI(frame.content), "\n")
		if len(lines) != frame.height {
			t.Errorf("frame %d has %d rows, want %d", i, len(lines), frame.height)
		}
		for row, line := range lines {
			if ansi.StringWidth(line) > frame.width {
				t.Errorf("frame %d row %d width %d exceeds %d", i, row+1, ansi.StringWidth(line), frame.width)
			}
		}
		if frame.width == 80 && frame.height == 24 {
			if !strings.Contains(lines[21], "%") || !strings.Contains(lines[23], "q quit") {
				t.Errorf("frame %d drifted from fixed 80x24 bar/footer rows", i)
			}
		}
	}
}

func assertProgressCheckpointsMonotonic(t *testing.T, frames []progressRecordedFrame) {
	t.Helper()
	previous := -1
	for i, frame := range frames {
		if frame.width != 80 || frame.height != 24 {
			continue
		}
		line := stripANSI(strings.Split(frame.content, "\n")[21])
		pct := progressPercent(line)
		if pct < previous {
			t.Errorf("frame %d progress regressed from %d%% to %d%%", i, previous, pct)
		}
		previous = pct
	}
}

func progressPercent(line string) int {
	for value := 0; value <= 100; value++ {
		if strings.Contains(line, " "+itoa(value)+"%") {
			return value
		}
	}
	return -1
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	var digits [3]byte
	i := len(digits)
	for value > 0 {
		i--
		digits[i] = byte('0' + value%10)
		value /= 10
	}
	return string(digits[i:])
}

func frameContains(frames []progressRecordedFrame, text string) bool {
	for _, frame := range frames {
		if strings.Contains(stripANSI(frame.content), text) {
			return true
		}
	}
	return false
}

func frameWithSize(frames []progressRecordedFrame, width, height int) bool {
	for _, frame := range frames {
		if frame.width == width && frame.height == height {
			return true
		}
	}
	return false
}

func removalProgressEvents(first, second progress.StepStatus) []progress.Event {
	events := []progress.Event{
		{Phase: "removal", PhaseLabel: "Removing worktrees", Step: "one", StepLabel: "first worktree", Status: progress.StepPending, Of: 1},
		{Phase: "removal", PhaseLabel: "Removing worktrees", Step: "two", StepLabel: "second worktree", Status: progress.StepPending, Of: 1},
		{Phase: "removal", PhaseLabel: "Removing worktrees", Close: true},
	}
	for _, item := range []struct {
		id     string
		status progress.StepStatus
	}{{"one", first}, {"two", second}} {
		if item.status != progress.StepPending {
			events = append(events, progress.Event{Phase: "removal", Step: item.id, Status: item.status, Checkpoint: 1, Of: 1})
		}
	}
	return events
}

func integrationFailureEvents() []progress.Event {
	return []progress.Event{
		{Phase: "ccc", PhaseLabel: "ccc", Step: "presence", StepLabel: "Check presence", Status: progress.StepPending, Of: 1},
		{Phase: "ccc", PhaseLabel: "ccc", Step: "init", StepLabel: "Initialize", Status: progress.StepPending, Of: 1},
		{Phase: "ccc", PhaseLabel: "ccc", Step: "index", StepLabel: "Index", Status: progress.StepPending, Of: 1},
		{Phase: "ccc", PhaseLabel: "ccc", Step: "setup", StepLabel: "Dependent setup", Status: progress.StepPending, Of: 1},
		{Phase: "ccc", PhaseLabel: "ccc", Close: true},
		{Phase: "ccc", Step: "presence", Status: progress.StepDone, Checkpoint: 1, Of: 1},
		{Phase: "ccc", Step: "init", Status: progress.StepDone, Checkpoint: 1, Of: 1},
		{Phase: "ccc", Step: "index", Status: progress.StepFailed, Checkpoint: 1, Of: 1, Error: errors.New("index exited 17")},
		{Phase: "ccc", Step: "setup", Status: progress.StepSkipped, Checkpoint: 1, Of: 1, Message: "dependency failed"},
	}
}
