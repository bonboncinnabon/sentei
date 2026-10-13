package worktree

import (
	"testing"

	"github.com/bonboncinnabon/sentei/internal/git"
)

func TestAtRisk(t *testing.T) {
	tests := []struct {
		name string
		wt   git.Worktree
		want bool
	}{
		{"clean and pushed", git.Worktree{IsEnriched: true}, false},
		{"uncommitted changes", git.Worktree{IsEnriched: true, HasUncommittedChanges: true}, true},
		{"untracked files", git.Worktree{IsEnriched: true, HasUntrackedFiles: true}, true},
		{"unpushed commits", git.Worktree{IsEnriched: true, HasUnpushedCommits: true}, true},
		// Enrichment stops at the first failing git call, before status runs,
		// so every flag is false. Unknown must not read as clean.
		{"status could not be read", git.Worktree{EnrichmentError: "git log -1: fatal: no commits yet"}, true},
		{"locked and otherwise clean", git.Worktree{IsEnriched: true, IsLocked: true}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AtRisk(tt.wt); got != tt.want {
				t.Errorf("AtRisk(%+v) = %v, want %v", tt.wt, got, tt.want)
			}
		})
	}
}
