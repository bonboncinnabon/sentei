package tui

import (
	"strings"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/config"
	"github.com/bonboncinnabon/sentei/internal/repo"
	"github.com/bonboncinnabon/sentei/internal/worktreefile"
)

func worktreeFilesModel() Model {
	cfg := &config.Config{WorktreeFiles: []worktreefile.Rule{{
		Source: ".sentei/private/codex.toml", Destination: ".codex/config.toml",
	}}}
	m := NewMenuModel(bareDirRunner("/repo"), nil, "/repo", cfg, repo.ContextBareRepo)
	m.width, m.height = 80, 24
	m.create.branchInput.SetValue("feature/files")
	m.create.baseInput.SetValue("main")
	return m
}

func TestCreateOptionsShowsAutomaticWorktreeFiles(t *testing.T) {
	output := stripAnsi(worktreeFilesModel().viewCreateOptions())
	if !strings.Contains(output, "Worktree files: 1 automatic") {
		t.Fatalf("options missing automatic worktree files:\n%s", output)
	}
	if strings.Contains(output, "[ ] Worktree files") || strings.Contains(output, "[x] Worktree files") {
		t.Fatalf("automatic worktree files rendered as toggle:\n%s", output)
	}
}

func TestCreateConfirmationShowsAutomaticWorktreeFiles(t *testing.T) {
	output := stripAnsi(worktreeFilesModel().createConfirmationVM().View())
	if !strings.Contains(output, "Worktree files:") || !strings.Contains(output, "1 automatic") {
		t.Fatalf("confirmation missing automatic worktree files:\n%s", output)
	}
}

func TestBuildCreatorOptionsForwardsWorktreeFiles(t *testing.T) {
	m := worktreeFilesModel()
	opts := m.buildCreatorOptions(nil)
	if len(opts.WorktreeFiles) != 1 {
		t.Fatalf("WorktreeFiles length = %d, want 1", len(opts.WorktreeFiles))
	}
	if opts.WorktreeFiles[0].Destination != ".codex/config.toml" {
		t.Errorf("Destination = %q", opts.WorktreeFiles[0].Destination)
	}
}
