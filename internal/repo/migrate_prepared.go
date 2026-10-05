package repo

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/bonboncinnabon/sentei/internal/fileutil"
	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/progress"
)

type migrateOperationKind uint8

const (
	migrateRegular migrateOperationKind = iota
	migrateStatus
	migrateBranch
	migrateBackupCopy
	migrateBackupSize
	migrateWorktree
)

type migrateOperation struct {
	phaseID progress.PhaseID
	stepID  progress.StepID
	label   string
	kind    migrateOperationKind
	run     func(*progress.Execution) (string, error)
}

type preparedMigrate struct {
	result          MigrateResult
	plan            progress.Plan
	operations      []migrateOperation
	backupCopyIndex int
	backupPath      *string
	branch          *string
	isDirty         *bool
	err             error
}

func prepareMigrate(runner git.CommandRunner, _ git.ShellRunner, opts MigrateOptions) preparedMigrate {
	repoPath := opts.RepoPath
	barePath := filepath.Join(repoPath, ".bare")
	originURL, originErr := runner.Run(repoPath, "remote", "get-url", "origin")
	preferredBackupPath := fmt.Sprintf("%s_backup_%s", repoPath, time.Now().Format("20060102_150405"))
	backupPath := new(string)
	branch := ""
	isDirty := false
	prepared := preparedMigrate{
		result: MigrateResult{BareRoot: repoPath}, backupPath: backupPath,
		branch: &branch, isDirty: &isDirty, backupCopyIndex: -1,
	}
	if originErr != nil && !isMissingOrigin(originErr) {
		prepared.err = fmt.Errorf("reading origin remote before migration: %w", originErr)
	}
	add := func(phaseID, phaseLabel, stepID, label string, kind migrateOperationKind, run func(*progress.Execution) (string, error)) {
		if len(prepared.plan.Phases) == 0 || prepared.plan.Phases[len(prepared.plan.Phases)-1].ID != phaseID {
			prepared.plan.Phases = append(prepared.plan.Phases, progress.PlannedPhase{ID: phaseID, Label: phaseLabel})
		}
		phase := &prepared.plan.Phases[len(prepared.plan.Phases)-1]
		phase.Steps = append(phase.Steps, progress.PlannedStep{ID: stepID, Label: label})
		prepared.operations = append(prepared.operations, migrateOperation{phaseID: phaseID, stepID: stepID, label: label, kind: kind, run: run})
	}
	add("migrate:validate", "Validate", "status", "Check repository status", migrateStatus, func(*progress.Execution) (string, error) {
		output, err := runner.Run(repoPath, "status", "--porcelain")
		if err != nil {
			return "", err
		}
		isDirty = strings.TrimSpace(output) != ""
		if isDirty {
			return "uncommitted changes detected", nil
		}
		return "clean", nil
	})
	add("migrate:validate", "Validate", "branch", "Detect current branch", migrateBranch, func(*progress.Execution) (string, error) {
		var err error
		branch, err = runner.Run(repoPath, "branch", "--show-current")
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(branch) == "" {
			return "", errors.New("cannot migrate a detached HEAD; check out a branch first")
		}
		return branch, nil
	})
	add("migrate:backup", "Backup", "copy", "Copy repository to backup", migrateBackupCopy, func(*progress.Execution) (string, error) {
		path, err := copyRepositoryBackup(repoPath, preferredBackupPath, copyRepositoryContents)
		if err != nil {
			return "", err
		}
		*backupPath = path
		return path, nil
	})
	prepared.backupCopyIndex = len(prepared.operations) - 1
	add("migrate:backup", "Backup", "size", "Calculate backup size", migrateBackupSize, func(*progress.Execution) (string, error) {
		return calculateDirSize(*backupPath), nil
	})
	add("migrate:convert", "Migrate", "bare-repository", "Create bare repository", migrateRegular, func(*progress.Execution) (string, error) {
		_, err := runner.Run(repoPath, "clone", "--bare", ".git", barePath)
		return "", err
	})
	add("migrate:convert", "Migrate", "remove-git", "Remove original .git", migrateRegular, func(*progress.Execution) (string, error) {
		return "", fileutil.RemoveAllRetry(filepath.Join(repoPath, ".git"))
	})
	add("migrate:convert", "Migrate", "git-pointer", "Create .git pointer", migrateRegular, func(*progress.Execution) (string, error) {
		return "", os.WriteFile(filepath.Join(repoPath, ".git"), []byte("gitdir: .bare\n"), 0644)
	})
	add("migrate:convert", "Migrate", "refspec", "Configure refspec", migrateRegular, func(*progress.Execution) (string, error) {
		_, err := runner.Run(barePath, "config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*")
		return "", err
	})
	if strings.TrimSpace(originURL) != "" {
		add("migrate:convert", "Migrate", "restore-origin", "Restore origin remote", migrateRegular, func(*progress.Execution) (string, error) {
			_, err := runner.Run(barePath, "remote", "set-url", "origin", originURL)
			return originURL, err
		})
	}
	add("migrate:convert", "Migrate", "clean-root", "Clean root directory", migrateRegular, func(execution *progress.Execution) (string, error) {
		entries, err := os.ReadDir(repoPath)
		if err != nil {
			return "", err
		}
		for _, entry := range entries {
			name := entry.Name()
			if name == ".bare" || name == ".git" {
				continue
			}
			if err := os.RemoveAll(filepath.Join(repoPath, name)); err != nil {
				_ = execution.Running("migrate:convert", "clean-root", 0, fmt.Sprintf("warning: could not remove %s: %v", name, err))
			}
		}
		return "", nil
	})
	add("migrate:convert", "Migrate", "worktree", "Create worktree", migrateWorktree, func(*progress.Execution) (string, error) {
		_, err := runner.Run(repoPath, "worktree", "add", git.WorktreePath(repoPath, branch), branch)
		return "", err
	})
	add("migrate:copy", "Copy", "restore-files", "Restore working files", migrateRegular, func(execution *progress.Execution) (string, error) {
		return restoreWorkingFiles(*backupPath, git.WorktreePath(repoPath, branch), copyTree, func(message string) {
			_ = execution.Running("migrate:copy", "restore-files", 0, message)
		})
	})
	return prepared
}

func copyRepositoryBackup(source, preferredPath string, copyFn func(string, string) error) (string, error) {
	backupPath := preferredPath
	for suffix := 0; ; suffix++ {
		if suffix > 0 {
			backupPath = fmt.Sprintf("%s_%d", preferredPath, suffix)
		}
		err := os.Mkdir(backupPath, 0700)
		if err == nil {
			break
		}
		if errors.Is(err, os.ErrExist) {
			continue
		}
		return "", fmt.Errorf("reserve migration backup %q: %w", backupPath, err)
	}

	cleanupFailure := func(operationErr error) (string, error) {
		if cleanupErr := fileutil.RemoveAllRetry(backupPath); cleanupErr != nil {
			return "", errors.Join(operationErr, fmt.Errorf("clean partial migration backup %q: %w", backupPath, cleanupErr))
		}
		return "", operationErr
	}
	if err := copyFn(source, backupPath); err != nil {
		return cleanupFailure(fmt.Errorf("copy repository to migration backup %q: %w", backupPath, err))
	}
	if err := os.Chmod(backupPath, 0700); err != nil {
		return cleanupFailure(fmt.Errorf("secure migration backup %q: %w", backupPath, err))
	}
	return backupPath, nil
}

func copyRepositoryContents(source, destination string) error {
	separator := string(os.PathSeparator)
	command := exec.Command("cp", "-a", source+separator+".", destination+separator)
	output, err := command.CombinedOutput()
	if err == nil {
		return nil
	}
	if detail := strings.TrimSpace(string(output)); detail != "" {
		return fmt.Errorf("cp -a: %s: %w", detail, err)
	}
	return fmt.Errorf("cp -a: %w", err)
}

// restoreWorkingFiles attempts every backup entry so one bad file does not
// hide later failures, but returns the aggregate error so callers preserve the
// backup instead of treating a partial restore as a successful migration.
func restoreWorkingFiles(backupPath, targetPath string, copyFn func(string, string) error, warn func(string)) (string, error) {
	entries, err := os.ReadDir(backupPath)
	if err != nil {
		return "nothing restored", fmt.Errorf("read migration backup: %w", err)
	}
	copied := 0
	var restoreErr error
	for _, entry := range entries {
		name := entry.Name()
		if name == ".git" || name == ".bare" {
			continue
		}
		if err := copyFn(filepath.Join(backupPath, name), filepath.Join(targetPath, name)); err != nil {
			warn(fmt.Sprintf("warning: could not copy %s: %v", name, err))
			restoreErr = errors.Join(restoreErr, fmt.Errorf("copy %s from migration backup: %w", name, err))
			continue
		}
		copied++
	}
	message := "nothing to restore"
	if copied > 0 {
		message = fmt.Sprintf("%d items restored", copied)
	}
	return message, restoreErr
}

func (p preparedMigrate) run(emit func(progress.Event)) MigrateResult {
	result := p.result
	if p.err != nil {
		result.Err = p.err
		return result
	}
	execution, err := progress.Start(p.plan, emit)
	if err != nil {
		result.Err = fmt.Errorf("starting repository migration: %w", err)
		return result
	}
	failedBy := ""
	for index, operation := range p.operations {
		if failedBy != "" {
			_, err = execution.Skip(operation.phaseID, operation.stepID, "blocked by "+failedBy)
		} else {
			var step progress.StepResult
			step, err = execution.Run(operation.phaseID, operation.stepID, func() (string, error) { return operation.run(execution) })
			if operation.kind == migrateBackupCopy && step.Status == progress.StepDone {
				result.BackupPath = *p.backupPath
			}
			if err == nil && step.Status == progress.StepFailed {
				failedBy = operation.label
			}
		}
		if err != nil {
			result.Err = errors.Join(result.Err, fmt.Errorf("executing %s: %w", operation.label, err))
			failedBy = operation.label
		}
		if failedBy != "" {
			continue
		}
		switch operation.kind {
		case migrateStatus:
			result.IsDirty = *p.isDirty
		case migrateBranch:
			result.Branch = *p.branch
		case migrateBackupCopy:
			result.BackupPath = *p.backupPath
		case migrateBackupSize:
			result.BackupSize = calculateDirSize(*p.backupPath)
		case migrateWorktree:
			result.WorktreePath = git.WorktreePath(result.BareRoot, result.Branch)
		}
		_ = index
	}
	finishErr := execution.Finish("repository migration finished")
	result.Phases = execution.Phases()
	result.Err = errors.Join(result.Err, finishErr)
	return result
}

func isMissingOrigin(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "no such remote") && strings.Contains(message, "origin")
}
