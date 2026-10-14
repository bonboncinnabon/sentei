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

func makeMigrateSummaryModel(result repo.MigrateResult) Model {
	m := NewMenuModel(nil, nil, "/repo", &config.Config{}, repo.ContextNoRepo)
	m.repo.result = result
	m.view = migrateSummaryView
	m.width = 80
	m.height = 24
	return m
}

func failedMigrateResult() repo.MigrateResult {
	return repo.MigrateResult{
		BareRoot: "/repo/proj",
		Phases: []progress.Phase{
			{Name: "Convert", Steps: []progress.StepResult{{Name: "Move working tree", Status: progress.StepFailed, Error: errors.New("permission denied")}}},
		},
	}
}

func TestMigrateSummaryShowsContractError(t *testing.T) {
	m := makeMigrateSummaryModel(repo.MigrateResult{Err: errors.New("delivery failed")})
	view := stripANSI(m.viewMigrateSummary())
	if !strings.Contains(view, "Migration failed") || !strings.Contains(view, "delivery failed") {
		t.Fatalf("view = %s", view)
	}
}

func TestViewMigrateSummary_FailureSaysWhatHappenedToTheRepository(t *testing.T) {
	out := stripAnsi(makeMigrateSummaryModel(failedMigrateResult()).viewMigrateSummary())

	for _, want := range []string{"Migration failed", "permission denied", "the repository is unchanged"} {
		if !strings.Contains(out, want) {
			t.Errorf("failure view missing %q:\n%s", want, out)
		}
	}
	for _, gone := range []string{"rm -rf", "backup", "open in sentei"} {
		if strings.Contains(out, gone) {
			t.Errorf("failure view must not mention %q:\n%s", gone, out)
		}
	}
}

func TestViewMigrateSummary_SuccessShowsWhereTheWorktreeIs(t *testing.T) {
	result := repo.MigrateResult{BareRoot: "/repo/proj", WorktreePath: "/repo/proj/feature-x", Branch: "feature/x"}

	out := stripAnsi(makeMigrateSummaryModel(result).viewMigrateSummary())

	for _, want := range []string{"Migration complete", "proj migrated", "feature/x", "cd /repo/proj/feature-x", "enter open in sentei", "q exit"} {
		if !strings.Contains(out, want) {
			t.Errorf("success view missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(strings.ToLower(out), "backup") {
		t.Errorf("there is no backup to offer for deletion:\n%s", out)
	}
}

func TestViewMigrateSummary_NoResult(t *testing.T) {
	m := NewMenuModel(nil, nil, "/repo", &config.Config{}, repo.ContextNonBareRepo)

	if view := m.viewMigrateSummary(); !strings.Contains(view, "Migration result unavailable") {
		t.Errorf("view = %q, want unavailable notice", view)
	}
}

func TestUpdateMigrateSummary_NoResultQuits(t *testing.T) {
	m := NewMenuModel(nil, nil, "/repo", &config.Config{}, repo.ContextNonBareRepo)
	m.view = migrateSummaryView

	_, cmd := m.updateMigrateSummary(keyMsg("x"))

	if cmd == nil {
		t.Fatal("expected a quit command without a migrate result")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", cmd())
	}
}

func TestUpdateMigrateSummary_QuitKey(t *testing.T) {
	for name, result := range map[string]repo.MigrateResult{
		"success": {BareRoot: "/bare", Branch: "main"},
		"failure": failedMigrateResult(),
	} {
		t.Run(name, func(t *testing.T) {
			_, cmd := makeMigrateSummaryModel(result).updateMigrateSummary(keyMsg("q"))

			if cmd == nil {
				t.Fatal("expected a quit command")
			}
			if _, ok := cmd().(tea.QuitMsg); !ok {
				t.Errorf("expected tea.QuitMsg, got %T", cmd())
			}
		})
	}
}

func TestUpdateMigrateSummary_EnterRelaunchesAfterSuccess(t *testing.T) {
	m := makeMigrateSummaryModel(repo.MigrateResult{BareRoot: "/bare", Branch: "main"})

	_, cmd := m.updateMigrateSummary(tea.KeyPressMsg{Code: tea.KeyEnter})

	// relaunchSentei execs a process; assert only that a command was issued.
	if cmd == nil {
		t.Fatal("enter should relaunch sentei at the migrated repo")
	}
}

func TestUpdateMigrateSummary_EnterIsInertAfterFailure(t *testing.T) {
	m := makeMigrateSummaryModel(failedMigrateResult())

	_, cmd := m.updateMigrateSummary(tea.KeyPressMsg{Code: tea.KeyEnter})

	if cmd != nil {
		t.Error("a failed migration must not relaunch sentei as if the bare layout existed")
	}
}

func TestUpdateMigrateSummary_WindowSize(t *testing.T) {
	m := makeMigrateSummaryModel(repo.MigrateResult{BareRoot: "/bare", Branch: "main"})

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	model := updated.(Model)

	if model.width != 100 || model.height != 34 {
		t.Errorf("size = %dx%d, want 100x34", model.width, model.height)
	}
}

func TestViewMigrateSummary_ValuesShareAColumn(t *testing.T) {
	result := repo.MigrateResult{BareRoot: "/repo/proj", WorktreePath: "/repo/proj/feature-x", Branch: "feature/x"}

	out := stripAnsi(makeMigrateSummaryModel(result).viewMigrateSummary())

	if path, branch := columnOf(t, out, "/repo/proj"), columnOf(t, out, "feature/x"); path != branch {
		t.Errorf("values start at columns %d and %d, want one column:\n%s", path, branch, out)
	}
}

func TestViewMigrateSummary_WrappedFailureKeepsItsIndent(t *testing.T) {
	result := failedMigrateResult()
	result.Phases[0].Steps[0].Error = errors.New(strings.Repeat("blocked ", 30))

	assertWrappedIndent(t, stripAnsi(makeMigrateSummaryModel(result).viewMigrateSummary()), "blocked")
}

// The migrate screens draw their rows, errors and cd line with the helpers
// the other flows use, so on a narrow screen a long path keeps its end and
// the cd line keeps all of it.
func TestMigrateScreens_FitANarrowScreen(t *testing.T) {
	long := "/Users/someone/code/clients/" + strings.Repeat("deeply-nested/", 4) + "project"
	result := repo.MigrateResult{BareRoot: long, WorktreePath: long + "/feature-x", Branch: "feature/" + strings.Repeat("x", 50)}
	m := makeMigrateSummaryModel(result)
	m.width = 40

	m.repo.migrateInfo = MigrateInfo{Loaded: true, Plan: repo.MigratePlan{
		RepoPath: long, WorktreePath: long + "/main", Branch: result.Branch, DefaultBranch: "main",
	}}
	failed := m
	failed.repo.result = repo.MigrateResult{Err: errors.New(strings.Repeat("a long reason ", 8))}

	for name, view := range map[string]string{
		"summary": m.viewMigrateSummary(),
		"confirm": m.viewMigrateConfirm(),
		"failure": failed.viewMigrateSummary(),
	} {
		for line := range strings.SplitSeq(stripAnsi(view), "\n") {
			// The footer and the title have their own fitting.
			if w := len([]rune(line)); w > m.width && !strings.Contains(line, "┄") {
				t.Errorf("%s: line is %d cells on a %d-cell screen: %q", name, w, m.width, line)
			}
		}
	}
	summary := stripAnsi(m.viewMigrateSummary())
	if !strings.Contains(summary, "project") {
		t.Errorf("the path row must keep the end of the path:\n%s", summary)
	}
	var cd strings.Builder
	_, after, _ := strings.Cut(summary, "cd ")
	for line := range strings.SplitSeq(after, "\n") {
		cd.WriteString(strings.TrimSpace(line))
	}
	if !strings.Contains(cd.String(), long+"/feature-x") {
		t.Errorf("the cd line must keep the whole path:\n%s", summary)
	}
}
