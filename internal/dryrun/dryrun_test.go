package dryrun

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/bonboncinnabon/sentei/internal/git"
)

func TestPrint(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name       string
		worktrees  []git.Worktree
		wantLines  []string
		wantAbsent []string
	}{
		{
			name:      "empty input",
			worktrees: nil,
			wantLines: []string{"STATUS"},
		},
		{
			name: "clean worktree",
			worktrees: []git.Worktree{
				{
					Branch:            "refs/heads/feature/auth",
					LastCommitDate:    now.Add(-48 * time.Hour),
					LastCommitSubject: "Add OAuth2 flow",
				},
			},
			wantLines: []string{"[ok]", "feature/auth", "2 days ago", "Add OAuth2 flow"},
		},
		{
			name: "dirty worktree",
			worktrees: []git.Worktree{
				{
					Branch:                "refs/heads/feature/wip",
					LastCommitDate:        now.Add(-1 * time.Hour),
					LastCommitSubject:     "WIP changes",
					HasUncommittedChanges: true,
				},
			},
			wantLines: []string{"[~]", "feature/wip"},
		},
		{
			name: "untracked files",
			worktrees: []git.Worktree{
				{
					Branch:            "refs/heads/experiment/test",
					LastCommitDate:    now.Add(-24 * time.Hour),
					LastCommitSubject: "Test stuff",
					HasUntrackedFiles: true,
				},
			},
			wantLines: []string{"[!]", "experiment/test"},
		},
		{
			name: "locked worktree",
			worktrees: []git.Worktree{
				{
					Branch:            "refs/heads/release/v1",
					LastCommitDate:    now.Add(-2 * time.Hour),
					LastCommitSubject: "Bump version",
					IsLocked:          true,
				},
			},
			wantLines: []string{"[L]", "release/v1"},
		},
		{
			name: "enrichment error",
			worktrees: []git.Worktree{
				{
					Branch:          "refs/heads/broken",
					EnrichmentError: "failed to read log",
				},
			},
			wantLines: []string{"error", "failed to read log"},
		},
		{
			name: "sort order oldest first",
			worktrees: []git.Worktree{
				{
					Branch:            "refs/heads/newer",
					LastCommitDate:    now.Add(-1 * time.Hour),
					LastCommitSubject: "New commit",
				},
				{
					Branch:            "refs/heads/older",
					LastCommitDate:    now.Add(-72 * time.Hour),
					LastCommitSubject: "Old commit",
				},
			},
			wantLines: []string{"older"},
		},
		{
			name: "zero dates sort to end",
			worktrees: []git.Worktree{
				{
					Branch:          "refs/heads/no-date",
					EnrichmentError: "no log",
				},
				{
					Branch:            "refs/heads/has-date",
					LastCommitDate:    now.Add(-1 * time.Hour),
					LastCommitSubject: "Recent",
				},
			},
			wantLines: []string{"has-date"},
		},
		{
			name: "detached HEAD",
			worktrees: []git.Worktree{
				{
					HEAD:              "abc123def456",
					IsDetached:        true,
					LastCommitDate:    now.Add(-1 * time.Hour),
					LastCommitSubject: "Detached work",
				},
			},
			wantLines: []string{"abc123d"},
		},
		{
			name: "prunable worktree",
			worktrees: []git.Worktree{
				{
					IsPrunable: true,
				},
			},
			wantLines: []string{"(prunable)"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := Print(tt.worktrees, &buf); err != nil {
				t.Fatalf("Print() error: %v", err)
			}
			output := buf.String()

			for _, want := range tt.wantLines {
				if !strings.Contains(output, want) {
					t.Errorf("output missing %q\ngot:\n%s", want, output)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(output, absent) {
					t.Errorf("output should not contain %q\ngot:\n%s", absent, output)
				}
			}
		})
	}
}

func TestPrintSortOrder(t *testing.T) {
	now := time.Now()
	worktrees := []git.Worktree{
		{Branch: "refs/heads/newer", LastCommitDate: now.Add(-1 * time.Hour), LastCommitSubject: "New"},
		{Branch: "refs/heads/oldest", LastCommitDate: now.Add(-720 * time.Hour), LastCommitSubject: "Old"},
		{Branch: "refs/heads/middle", LastCommitDate: now.Add(-48 * time.Hour), LastCommitSubject: "Mid"},
	}

	var buf bytes.Buffer
	if err := Print(worktrees, &buf); err != nil {
		t.Fatalf("Print() error: %v", err)
	}
	output := buf.String()

	oldestIdx := strings.Index(output, "oldest")
	middleIdx := strings.Index(output, "middle")
	newerIdx := strings.Index(output, "newer")

	if oldestIdx == -1 || middleIdx == -1 || newerIdx == -1 {
		t.Fatalf("missing branches in output:\n%s", output)
	}
	if oldestIdx >= middleIdx || middleIdx >= newerIdx {
		t.Errorf("expected oldest < middle < newer, got positions %d, %d, %d\noutput:\n%s",
			oldestIdx, middleIdx, newerIdx, output)
	}
}

func TestPrintNoAnsi(t *testing.T) {
	worktrees := []git.Worktree{
		{
			Branch:                "refs/heads/test",
			LastCommitDate:        time.Now().Add(-1 * time.Hour),
			LastCommitSubject:     "Test",
			HasUncommittedChanges: true,
			IsLocked:              true,
		},
	}

	var buf bytes.Buffer
	if err := Print(worktrees, &buf); err != nil {
		t.Fatalf("Print() error: %v", err)
	}
	output := buf.String()

	if strings.Contains(output, "\x1b[") {
		t.Errorf("output contains ANSI escape codes:\n%s", output)
	}
}
