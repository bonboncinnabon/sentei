package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/internal/config"
	"github.com/bonboncinnabon/sentei/internal/progress"
	"github.com/bonboncinnabon/sentei/internal/repo"
)

func TestCloneFailedPropagatesContractError(t *testing.T) {
	want := errors.New("delivery")
	failed, err := cloneFailed(repo.CloneResult{Err: want})
	if !failed || !errors.Is(err, want) {
		t.Fatalf("cloneFailed = %v, %v", failed, err)
	}
}

func makeRepoSummaryModel(result any) Model {
	m := NewMenuModel(nil, nil, "/repo", &config.Config{}, repo.ContextNoRepo)
	m.repo.result = result
	m.width = 80
	m.height = 24
	return m
}

func cloneFailureResult() repo.CloneResult {
	return repo.CloneResult{
		RepoPath:      "/tmp/myrepo",
		DefaultBranch: "main",
		// WorktreePath intentionally empty: the worktree was never created.
		Phases: []progress.Phase{
			{Name: "Clone", Steps: []progress.StepResult{{Name: "Clone bare repository", Status: progress.StepDone}}},
			{Name: "Worktree", Steps: []progress.StepResult{
				{Name: "Create worktree", Status: progress.StepFailed, Error: errors.New("fatal: invalid reference: main")},
			}},
		},
	}
}

func cloneSuccessResult() repo.CloneResult {
	return repo.CloneResult{
		RepoPath:      "/tmp/myrepo",
		DefaultBranch: "main",
		WorktreePath:  "/tmp/myrepo/main",
		OriginURL:     "git@github.com:user/myrepo.git",
		Phases: []progress.Phase{
			{Name: "Worktree", Steps: []progress.StepResult{{Name: "Create worktree", Status: progress.StepDone}}},
		},
	}
}

func TestViewCloneRepoSummary_WorktreeFailure_ShowsErrorNotReady(t *testing.T) {
	m := makeRepoSummaryModel(cloneFailureResult())
	out := stripAnsi(m.viewCloneRepoSummary(cloneFailureResult()))

	if strings.Contains(out, "ready") {
		t.Errorf("failed clone summary must not say 'ready':\n%s", out)
	}
	if !strings.Contains(out, "clone failed") {
		t.Errorf("expected 'clone failed' header:\n%s", out)
	}
	if !strings.Contains(out, "invalid reference: main") {
		t.Errorf("expected the git error surfaced:\n%s", out)
	}
	if strings.Contains(out, "cd ") {
		t.Errorf("must not print a cd hint when no worktree exists:\n%s", out)
	}
	if strings.Contains(out, "enter open") {
		t.Errorf("must not offer relaunch on failure:\n%s", out)
	}
}

func TestViewCloneRepoSummary_Success_ShowsReady(t *testing.T) {
	m := makeRepoSummaryModel(cloneSuccessResult())
	out := stripAnsi(m.viewCloneRepoSummary(cloneSuccessResult()))

	if !strings.Contains(out, "ready") {
		t.Errorf("successful clone summary should say 'ready':\n%s", out)
	}
	if !strings.Contains(out, "cd /tmp/myrepo/main") {
		t.Errorf("expected cd hint to the worktree:\n%s", out)
	}
	if !strings.Contains(out, "enter open") {
		t.Errorf("expected relaunch affordance on success:\n%s", out)
	}
}

func TestViewCreateRepoSummary_SetupFailure_ShowsFailed(t *testing.T) {
	result := repo.CreateResult{
		RepoPath: "/tmp/my-project",
		Phases: []progress.Phase{
			{Name: "Setup", Steps: []progress.StepResult{
				{Name: "Initial commit", Status: progress.StepFailed, Error: errors.New("git commit: exit status 128")},
			}},
		},
	}
	m := makeRepoSummaryModel(result)
	out := stripAnsi(m.viewCreateRepoSummary(result))

	if strings.Contains(out, "ready") {
		t.Errorf("setup failure must not say 'ready':\n%s", out)
	}
	if !strings.Contains(out, "create failed") {
		t.Errorf("expected 'create failed' header:\n%s", out)
	}
	if !strings.Contains(out, "exit status 128") {
		t.Errorf("expected the setup error surfaced:\n%s", out)
	}
	if strings.Contains(out, "cd ") || strings.Contains(out, "enter open") {
		t.Errorf("must not offer cd/relaunch on a broken repo:\n%s", out)
	}
}

func TestViewCreateRepoSummary_GitHubFailure_ShowsLocalOnly(t *testing.T) {
	result := repo.CreateResult{
		RepoPath:     "/tmp/my-project",
		WorktreePath: "/tmp/my-project/main",
		Phases: []progress.Phase{
			{Name: "Setup", Steps: []progress.StepResult{{Name: "Create main worktree", Status: progress.StepDone}}},
			{Name: "GitHub", Steps: []progress.StepResult{
				{Name: "Push to GitHub", Status: progress.StepFailed, Error: errors.New("permission denied (publickey)")},
			}},
		},
	}
	m := makeRepoSummaryModel(result)
	out := stripAnsi(m.viewCreateRepoSummary(result))

	if !strings.Contains(out, "ready (local only)") {
		t.Errorf("GitHub-only failure should render 'ready (local only)':\n%s", out)
	}
	if !strings.Contains(out, "enter open") {
		t.Errorf("local repo is usable; relaunch should still be offered:\n%s", out)
	}
}

// quitsOnConfirm executes the Confirm key handler and reports whether the
// returned command resolves to tea.QuitMsg (quit) rather than a relaunch.
func quitsOnConfirm(t *testing.T, m Model) bool {
	t.Helper()
	_, cmd := m.updateRepoSummary(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected a command from Confirm")
	}
	_, isQuit := cmd().(tea.QuitMsg)
	return isQuit
}

func TestUpdateRepoSummary_CloneFailure_QuitsInsteadOfRelaunch(t *testing.T) {
	m := makeRepoSummaryModel(cloneFailureResult())
	if !quitsOnConfirm(t, m) {
		t.Error("Confirm on a failed clone must quit, not relaunch into a worktree-less repo")
	}
}

func TestUpdateRepoSummary_CloneSuccess_Relaunches(t *testing.T) {
	m := makeRepoSummaryModel(cloneSuccessResult())
	if quitsOnConfirm(t, m) {
		t.Error("Confirm on a successful clone should relaunch, not quit")
	}
}

func TestViewRepoSummary_DispatchesByResultType(t *testing.T) {
	cases := []struct {
		name   string
		result any
		want   string
	}{
		{"create result", repo.CreateResult{RepoPath: "/tmp/myrepo"}, "Repository created"},
		{"clone result", cloneSuccessResult(), "Repository cloned"},
		{"unknown result", nil, "Operation complete"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := makeRepoSummaryModel(tc.result)

			view := stripANSI(m.viewRepoSummary())

			if !strings.Contains(view, tc.want) {
				t.Errorf("view missing %q:\n%s", tc.want, view)
			}
		})
	}
}
