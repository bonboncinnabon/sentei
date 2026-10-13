package creator

import (
	"testing"

	"github.com/bonboncinnabon/sentei/internal/git"
)

func TestSourceWorktree(t *testing.T) {
	tests := []struct {
		name      string
		worktrees []git.Worktree
		want      string
	}{
		{
			"prefers main",
			[]git.Worktree{
				{Path: "/wt/feature", Branch: "refs/heads/feature"},
				{Path: "/wt/main", Branch: "refs/heads/main"},
			},
			"/wt/main",
		},
		{
			"prefers master",
			[]git.Worktree{
				{Path: "/wt/feature", Branch: "refs/heads/feature"},
				{Path: "/wt/master", Branch: "refs/heads/master"},
			},
			"/wt/master",
		},
		{
			"falls back to first worktree",
			[]git.Worktree{
				{Path: "/wt/one", Branch: "refs/heads/one"},
				{Path: "/wt/two", Branch: "refs/heads/two"},
			},
			"/wt/one",
		},
		{
			"never the bare entry: it holds no working files",
			[]git.Worktree{
				{Path: "/repo/.bare", IsBare: true},
				{Path: "/repo/trunk", Branch: "refs/heads/trunk"},
			},
			"/repo/trunk",
		},
		{
			"never a prunable entry: its directory is gone",
			[]git.Worktree{
				{Path: "/repo/main", Branch: "refs/heads/main", IsPrunable: true},
				{Path: "/repo/trunk", Branch: "refs/heads/trunk"},
			},
			"/repo/trunk",
		},
		{"no worktrees", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SourceWorktree(tt.worktrees); got != tt.want {
				t.Errorf("SourceWorktree() = %q, want %q", got, tt.want)
			}
		})
	}
}
