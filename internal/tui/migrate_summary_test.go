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

func TestMigrateSummaryShowsContractError(t *testing.T) {
	m := makeMigrateSummaryModel(repo.MigrateResult{Err: errors.New("delivery failed")})
	view := stripANSI(m.viewMigrateSummary())
	if !strings.Contains(view, "delivery failed") {
		t.Fatalf("view = %s", view)
	}
}

func makeMigrateSummaryModel(result repo.MigrateResult) Model {
	m := NewMenuModel(nil, nil, "/repo", &config.Config{}, repo.ContextNoRepo)
	m.repo.result = result
	m.width = 80
	m.height = 24
	return m
}

func TestViewMigrateSummary_BackupFailure_NoDestructiveRestore(t *testing.T) {
	// Backup failed -> BackupPath empty (the data-loss fix). The destructive
	// restore command must NOT render against the still-intact repo.
	result := repo.MigrateResult{
		BareRoot: "/repo/proj",
		Phases: []progress.Phase{
			{Name: "Validate", Steps: []progress.StepResult{{Name: "Detect current branch", Status: progress.StepDone}}},
			{Name: "Backup", Steps: []progress.StepResult{{Name: "Copy repository to backup", Status: progress.StepFailed, Error: errors.New("no space left")}}},
		},
		// BackupPath intentionally empty
	}
	out := stripAnsi(makeMigrateSummaryModel(result).viewMigrateSummary())
	if !strings.Contains(out, "Migration failed") {
		t.Errorf("expected a failure header:\n%s", out)
	}
	if strings.Contains(out, "rm -rf") {
		t.Errorf("a backup failure must NOT show a destructive restore command:\n%s", out)
	}
}

func TestViewMigrateSummary_MigrateFailure_ShowsRestore(t *testing.T) {
	result := repo.MigrateResult{
		BareRoot:   "/repo/proj",
		BackupPath: "/repo/proj_backup_1",
		Phases: []progress.Phase{
			{Name: "Backup", Steps: []progress.StepResult{{Name: "Copy repository to backup", Status: progress.StepDone}}},
			{Name: "Migrate", Steps: []progress.StepResult{{Name: "Create bare repository", Status: progress.StepFailed, Error: errors.New("boom")}}},
		},
	}
	out := stripAnsi(makeMigrateSummaryModel(result).viewMigrateSummary())
	if !strings.Contains(out, "Migration failed") {
		t.Errorf("expected a failure header:\n%s", out)
	}
	if !strings.Contains(out, "rm -rf") || !strings.Contains(out, "/repo/proj_backup_1") {
		t.Errorf("a real backup should yield a restore command:\n%s", out)
	}
}

func TestViewMigrateSummary_Success_OffersDeleteBackup(t *testing.T) {
	result := repo.MigrateResult{
		BareRoot:   "/repo/proj",
		BackupPath: "/repo/proj_backup_1",
		Branch:     "main",
		Phases: []progress.Phase{
			{Name: "Migrate", Steps: []progress.StepResult{{Name: "Create worktree", Status: progress.StepDone}}},
		},
	}
	out := stripAnsi(makeMigrateSummaryModel(result).viewMigrateSummary())
	if !strings.Contains(out, "migrated") {
		t.Errorf("expected a success header:\n%s", out)
	}
	if !strings.Contains(out, "Delete backup?") {
		t.Errorf("expected the delete-backup prompt:\n%s", out)
	}
}

func TestUpdateMigrateSummary_Failure_YIsInert(t *testing.T) {
	result := repo.MigrateResult{
		BareRoot: "/repo/proj",
		Phases: []progress.Phase{
			{Name: "Backup", Steps: []progress.StepResult{{Name: "x", Status: progress.StepFailed, Error: errors.New("no space")}}},
		},
	}
	m := makeMigrateSummaryModel(result)
	// On a failed migration, 'y' (delete backup) must be inert — never delete a
	// backup / advance. Only quit responds.
	_, cmd := m.updateMigrateSummary(tea.KeyPressMsg{Code: 'y', Text: "y"})
	if cmd != nil {
		t.Error("'y' on a failed migration must not delete backup or advance")
	}
}

func TestUpdateMigrateNext_NoResultQuits(t *testing.T) {
	m := NewMenuModel(nil, nil, "/repo", &config.Config{}, repo.ContextNonBareRepo)
	m.view = migrateNextView

	_, cmd := m.updateMigrateNext(keyMsg("x"))

	if cmd == nil {
		t.Fatal("expected a quit command without a migrate result")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", cmd())
	}
}

func TestUpdateMigrateNext_QuitKey(t *testing.T) {
	m := makeMigrateSummaryModel(repo.MigrateResult{BareRoot: "/bare", Branch: "main"})
	m.view = migrateNextView

	_, cmd := m.updateMigrateNext(keyMsg("q"))

	if cmd == nil {
		t.Fatal("expected a quit command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", cmd())
	}
}

func TestUpdateMigrateNext_EnterRelaunches(t *testing.T) {
	m := makeMigrateSummaryModel(repo.MigrateResult{BareRoot: "/bare", Branch: "main"})
	m.view = migrateNextView

	_, cmd := m.updateMigrateNext(tea.KeyPressMsg{Code: tea.KeyEnter})

	// relaunchSentei execs a process; assert only that a command was issued.
	if cmd == nil {
		t.Fatal("enter should relaunch sentei at the migrated repo")
	}
}

func TestUpdateMigrateNext_WindowSize(t *testing.T) {
	m := makeMigrateSummaryModel(repo.MigrateResult{BareRoot: "/bare", Branch: "main"})
	m.view = migrateNextView

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	model := updated.(Model)

	if model.width != 100 || model.height != 34 {
		t.Errorf("size = %dx%d, want 100x34", model.width, model.height)
	}
}

func TestViewMigrateNext_NoResult(t *testing.T) {
	m := NewMenuModel(nil, nil, "/repo", &config.Config{}, repo.ContextNonBareRepo)

	view := m.viewMigrateNext()

	if !strings.Contains(view, "Migration result unavailable") {
		t.Errorf("view = %q, want unavailable notice", view)
	}
}

func TestViewMigrateNext_SuccessShowsWorktreePath(t *testing.T) {
	m := makeMigrateSummaryModel(repo.MigrateResult{BareRoot: "/bare/myrepo", Branch: "main"})

	view := stripANSI(m.viewMigrateNext())

	for _, want := range []string{"Migration complete", "myrepo ready", "cd ", "enter open in sentei", "q exit"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q:\n%s", want, view)
		}
	}
}
