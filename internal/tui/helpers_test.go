package tui

import (
	"path/filepath"
	"testing"
	"time"

	progressbar "charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/stopwatch"
	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/testutil/mock"
)

// bareDirRunner returns a runner that answers `git rev-parse --git-common-dir`
// for root with root/.bare, letting flows that resolve the bare dir run without
// a real git repository on disk.
func bareDirRunner(root string) *mock.Runner {
	return &mock.Runner{Responses: map[string]mock.Response{
		root + ":[rev-parse --git-common-dir]": {Output: filepath.Join(root, ".bare")},
	}}
}

func stripAnsi(s string) string {
	var result []byte
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' {
			for i < len(s) && s[i] != 'm' {
				i++
			}
			i++
			continue
		}
		result = append(result, s[i])
		i++
	}
	return string(result)
}

// pumpCmds drives a model's command chain to quiescence the way the runtime
// would, expanding batches. Animation messages (spring frames, stopwatch
// control, spinner ticks) are dropped so the pump never sleeps on a tick.
func pumpCmds(model tea.Model, cmd tea.Cmd) tea.Model {
	queue := []tea.Cmd{cmd}
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		if c == nil {
			continue
		}
		msg := c()
		if msg == nil {
			continue
		}
		if batch, ok := msg.(tea.BatchMsg); ok {
			queue = append(queue, batch...)
			continue
		}
		switch msg.(type) {
		case progressbar.FrameMsg, stopwatch.StartStopMsg, spinner.TickMsg, motionTickMsg:
			continue
		}
		var next tea.Cmd
		model, next = model.Update(msg)
		if next != nil {
			queue = append(queue, next)
		}
	}
	return model
}

// settleNow fast-forwards the completion settle in tests: backdates the
// settling clock past the hard timeout and runs the observation, returning
// the advanced model. Fails the test if the flow was not settling.
func settleNow(t *testing.T, m Model) Model {
	t.Helper()
	if !m.progressSettling {
		t.Fatal("flow is not in the completion settle")
	}
	m.progressSettlingSince = time.Now().Add(-progressSettleTimeout - time.Millisecond)
	model, advanced := m.observeSettle(time.Now())
	if !advanced {
		t.Fatal("settle observation did not advance the view")
	}
	return model.completeProgressTransition()
}
