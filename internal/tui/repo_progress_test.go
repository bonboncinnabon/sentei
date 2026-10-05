package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/config"
	"github.com/bonboncinnabon/sentei/internal/progress"
	"github.com/bonboncinnabon/sentei/internal/repo"
)

// drainRepoPipeline executes wait commands until the repo pipeline goroutine
// reports completion, returning the final result and the events seen.
func drainRepoPipeline(t *testing.T, m Model) (interface{}, []progress.Event) {
	t.Helper()
	var events []progress.Event
	for range 100 {
		msg := m.waitForRepoEvent()()
		switch msg := msg.(type) {
		case repoEventMsg:
			events = append(events, progress.Event(msg))
		case repoDoneMsg:
			return msg.result, events
		default:
			t.Fatalf("unexpected message %T", msg)
		}
	}
	t.Fatal("repo pipeline never completed")
	return nil, nil
}

func repoProgressModel(t *testing.T) Model {
	t.Helper()
	m := NewMenuModel(&stubRunner{responses: map[string]stubResponse{}}, nil, t.TempDir(), &config.Config{}, repo.ContextNoRepo)
	m.width, m.height = 80, 24
	return m
}

func TestStartRepoPipeline_DispatchesByOptionsType(t *testing.T) {
	cases := []struct {
		name       string
		opts       func(location string) interface{}
		wantResult func(result interface{}) bool
	}{
		{
			"create", func(location string) interface{} {
				return repo.CreateOptions{Name: "myrepo", Location: location}
			},
			func(result interface{}) bool { _, ok := result.(repo.CreateResult); return ok },
		},
		{
			"clone", func(location string) interface{} {
				return repo.CloneOptions{URL: "git@github.com:user/myrepo.git", Location: location, Name: "myrepo"}
			},
			func(result interface{}) bool { _, ok := result.(repo.CloneResult); return ok },
		},
		{
			"migrate", func(location string) interface{} {
				return repo.MigrateOptions{RepoPath: location}
			},
			func(result interface{}) bool { _, ok := result.(repo.MigrateResult); return ok },
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := repoProgressModel(t)
			location := t.TempDir()
			if tc.name == "migrate" {
				m.runner.(*stubRunner).responses[location+" remote get-url origin"] = stubResponse{err: fmt.Errorf("error: No such remote 'origin'")}
			}

			cmd := m.startRepoPipeline(tc.opts(location))

			if m.repo.eventCh == nil || m.repo.resultCh == nil {
				t.Fatal("pipeline channels must be wired")
			}
			if cmd == nil {
				t.Fatal("expected a command waiting for pipeline events")
			}
			result, events := drainRepoPipeline(t, m)
			if !tc.wantResult(result) {
				t.Errorf("result = %T, want the %s result type", result, tc.name)
			}
			if len(events) == 0 {
				t.Error("expected progress events from the pipeline")
			}
		})
	}
}

func TestWaitForRepoEvent_DeliversEventThenDone(t *testing.T) {
	m := repoProgressModel(t)
	m.repo.eventCh = make(chan progress.Event, 1)
	m.repo.resultCh = make(chan interface{}, 1)

	m.repo.eventCh <- progress.Event{Phase: "Clone", Step: "Clone bare repository", Status: progress.StepRunning}
	msg := m.waitForRepoEvent()()
	ev, ok := msg.(repoEventMsg)
	if !ok {
		t.Fatalf("expected repoEventMsg, got %T", msg)
	}
	if ev.Step != "Clone bare repository" {
		t.Errorf("event step = %q, want %q", ev.Step, "Clone bare repository")
	}

	close(m.repo.eventCh)
	m.repo.resultCh <- repo.CloneResult{RepoPath: "/tmp/myrepo"}
	msg = m.waitForRepoEvent()()
	done, ok := msg.(repoDoneMsg)
	if !ok {
		t.Fatalf("expected repoDoneMsg after channel close, got %T", msg)
	}
	if result, ok := done.result.(repo.CloneResult); !ok || result.RepoPath != "/tmp/myrepo" {
		t.Errorf("done result = %+v, want the clone result", done.result)
	}
}

func TestViewRepoProgress_TitlePerOperation(t *testing.T) {
	cases := []struct {
		opType    string
		wantTitle string
	}{
		{"create", "Creating repository"},
		{"clone", "Cloning repository"},
		{"migrate", "Migrating repository"},
	}
	for _, tc := range cases {
		t.Run(tc.opType, func(t *testing.T) {
			m := repoProgressModel(t)
			m.repo.opType = tc.opType
			m.repo.events = []progress.Event{
				{Phase: "Validate", Step: "Check repo", Status: progress.StepDone},
			}

			view := stripANSI(m.viewRepoProgress())

			if !strings.Contains(view, tc.wantTitle) {
				t.Errorf("view missing title %q:\n%s", tc.wantTitle, view)
			}
			if !strings.Contains(view, "Validate") {
				t.Errorf("view missing phase name:\n%s", view)
			}
		})
	}
}
