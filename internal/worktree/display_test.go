package worktree

import (
	"testing"
	"time"

	"github.com/bonboncinnabon/sentei/internal/git"
)

func TestRelativeTime(t *testing.T) {
	tests := []struct {
		name string
		ago  time.Duration
		want string
	}{
		{"seconds ago", 30 * time.Second, "just now"},
		{"1 minute ago", time.Minute, "1 minute ago"},
		{"5 minutes ago", 5 * time.Minute, "5 minutes ago"},
		{"1 hour ago", time.Hour, "1 hour ago"},
		{"2 hours ago", 2 * time.Hour, "2 hours ago"},
		{"1 day ago", 24 * time.Hour, "1 day ago"},
		{"15 days ago", 15 * 24 * time.Hour, "15 days ago"},
		{"1 month ago", 45 * 24 * time.Hour, "1 month ago"},
		{"3 months ago", 90 * 24 * time.Hour, "3 months ago"},
		{"1 year ago", 365 * 24 * time.Hour, "1 year ago"},
		{"2 years ago", 730 * 24 * time.Hour, "2 years ago"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RelativeTime(time.Now().Add(-tt.ago)); got != tt.want {
				t.Errorf("RelativeTime(%v ago) = %q, want %q", tt.ago, got, tt.want)
			}
		})
	}
	if got := RelativeTime(time.Time{}); got != "unknown" {
		t.Errorf("RelativeTime(zero) = %q, want %q", got, "unknown")
	}
}

func TestShortBranch(t *testing.T) {
	for input, want := range map[string]string{
		"refs/heads/main":         "main",
		"refs/heads/feature/auth": "feature/auth",
		"main":                    "main",
		"":                        "",
	} {
		if got := ShortBranch(input); got != want {
			t.Errorf("ShortBranch(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestLabel(t *testing.T) {
	tests := []struct {
		name string
		wt   git.Worktree
		want string
	}{
		{"branch", git.Worktree{Path: "/work/a", Branch: "refs/heads/feature/a"}, "feature/a"},
		{"detached uses the short hash", git.Worktree{Path: "/work/d", HEAD: "e635cdfa8afe577ac4345997dba1bab0ca223810", IsDetached: true}, "e635cdf"},
		{"prunable", git.Worktree{Path: "/work/gone", IsPrunable: true}, "(prunable)"},
		{"directory name as a last resort", git.Worktree{Path: "/work/unknown"}, "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Label(tt.wt); got != tt.want {
				t.Errorf("Label() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStatusBadge(t *testing.T) {
	tests := []struct {
		name string
		wt   git.Worktree
		want string
	}{
		{"clean", git.Worktree{}, "[ok]"},
		{"uncommitted changes", git.Worktree{HasUncommittedChanges: true}, "[~]"},
		{"untracked files", git.Worktree{HasUntrackedFiles: true}, "[!]"},
		{"locked takes priority", git.Worktree{IsLocked: true, HasUncommittedChanges: true}, "[L]"},
		{"uncommitted takes priority over untracked", git.Worktree{HasUncommittedChanges: true, HasUntrackedFiles: true}, "[~]"},
		{"unreadable status is not clean", git.Worktree{EnrichmentError: "git log -1: fatal: no commits yet"}, "[?]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StatusBadge(tt.wt); got != tt.want {
				t.Errorf("StatusBadge() = %q, want %q", got, tt.want)
			}
		})
	}
}
