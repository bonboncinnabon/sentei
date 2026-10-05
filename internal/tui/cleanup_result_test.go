package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/cleanup"
	"github.com/bonboncinnabon/sentei/internal/config"
	"github.com/bonboncinnabon/sentei/internal/repo"
)

func makeCleanupModel() Model {
	m := NewMenuModel(nil, nil, "/repo", &config.Config{}, repo.ContextBareRepo)
	m.view = cleanupResultView
	m.width = 80
	m.height = 24
	return m
}

func TestUpdateCleanupResult_DoneMsg(t *testing.T) {
	m := makeCleanupModel()

	result := cleanup.Result{StaleRefsRemoved: 3}
	updated, _ := m.updateCleanupResult(standaloneCleanupDoneMsg{result: result})
	m = updated.(Model)

	if m.cleanupResult == nil {
		t.Fatal("expected cleanupResult to be set")
	}
	if m.cleanupResult.StaleRefsRemoved != 3 {
		t.Errorf("expected StaleRefsRemoved=3, got %d", m.cleanupResult.StaleRefsRemoved)
	}
}

func TestUpdateCleanupResult_QuitKeys(t *testing.T) {
	result := cleanup.Result{}

	for _, k := range []tea.Msg{
		tea.KeyPressMsg{Code: tea.KeyEnter},
		tea.KeyPressMsg{Code: tea.KeyEsc},
		keyMsg("q"),
	} {
		m := makeCleanupModel()
		m.cleanupResult = &result

		_, cmd := m.updateCleanupResult(k)
		if cmd == nil {
			t.Fatalf("expected a Cmd for key %v, got nil", k)
		}
		msg := cmd()
		if _, ok := msg.(tea.QuitMsg); !ok {
			t.Errorf("expected tea.QuitMsg, got %T", msg)
		}
	}
}

func TestUpdateCleanupResult_IgnoresKeysBeforeResult(t *testing.T) {
	m := makeCleanupModel()
	// cleanupResult is nil

	_, cmd := m.updateCleanupResult(keyMsg("\r"))
	if cmd != nil {
		t.Error("expected nil cmd when cleanupResult is not yet set")
	}
}

func TestViewCleanupResult_Loading(t *testing.T) {
	m := makeCleanupModel()
	// cleanupResult is nil

	output := stripAnsi(m.viewCleanupResult())

	if !strings.Contains(output, "Running cleanup") {
		t.Errorf("expected 'Running cleanup' in loading state, got:\n%s", output)
	}
}

func TestViewCleanupResult_CleanRepo(t *testing.T) {
	m := makeCleanupModel()
	result := cleanup.Result{} // all zeros, no errors
	m.cleanupResult = &result

	output := stripAnsi(m.viewCleanupResult())

	if !strings.Contains(output, "Repository is clean") {
		t.Errorf("expected 'Repository is clean', got:\n%s", output)
	}
}

func TestViewCleanupResult_ShowsActions(t *testing.T) {
	m := makeCleanupModel()
	result := cleanup.Result{
		StaleRefsRemoved:    3,
		GoneBranchesDeleted: 1,
	}
	m.cleanupResult = &result

	output := stripAnsi(m.viewCleanupResult())

	if !strings.Contains(output, "Pruned 3") {
		t.Errorf("expected 'Pruned 3' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "Deleted 1") {
		t.Errorf("expected 'Deleted 1' in output, got:\n%s", output)
	}
}

func TestViewCleanupResult_ShowsErrors(t *testing.T) {
	m := makeCleanupModel()
	result := cleanup.Result{
		Errors: []cleanup.OperationError{
			{Step: "prune-refs", Err: fmt.Errorf("permission denied")},
		},
	}
	m.cleanupResult = &result

	output := stripAnsi(m.viewCleanupResult())

	if !strings.Contains(output, "prune-refs") {
		t.Errorf("expected error step name 'prune-refs' in output, got:\n%s", output)
	}
}

func TestViewCleanupResult_AggressiveTip(t *testing.T) {
	m := makeCleanupModel()
	result := cleanup.Result{
		NonWtBranchesRemaining: 4,
	}
	m.cleanupResult = &result

	output := stripAnsi(m.viewCleanupResult())

	if !strings.Contains(output, "4 local") {
		t.Errorf("expected '4 local' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "aggressive") {
		t.Errorf("expected 'aggressive' in output, got:\n%s", output)
	}
}

func TestViewCleanupResult_SkippedBranchesAreNotPending(t *testing.T) {
	m := makeCleanupModel()
	m.cleanupRanMode = cleanup.ModeAggressive
	m.cleanupResult = &cleanup.Result{
		BranchesSkipped: []cleanup.SkippedBranch{{Name: "feature/wip"}},
	}

	output := stripAnsi(m.viewCleanupResult())

	if !strings.Contains(output, "1 branch skipped") {
		t.Errorf("expected skipped count line, got:\n%s", output)
	}
	if !strings.Contains(output, "feature/wip") {
		t.Errorf("expected skipped branch name, got:\n%s", output)
	}
	// A terminal result has nothing pending: the skipped name must not borrow
	// the pending indicator as a list bullet.
	if strings.Contains(output, indicatorPending+" feature/wip") {
		t.Errorf("skipped name must not use the pending indicator, got:\n%s", output)
	}
}
