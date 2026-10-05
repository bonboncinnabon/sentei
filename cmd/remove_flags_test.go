package cmd

import (
	"strings"
	"testing"
	"time"

	"github.com/bonboncinnabon/sentei/internal/git"
)

func TestParseStaleDuration_Days(t *testing.T) {
	d, err := ParseStaleDuration("30d")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d != 30*24*time.Hour {
		t.Errorf("expected 30 days (%v), got %v", 30*24*time.Hour, d)
	}
}

func TestParseStaleDuration_Weeks(t *testing.T) {
	d, err := ParseStaleDuration("2w")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d != 14*24*time.Hour {
		t.Errorf("expected 14 days (%v), got %v", 14*24*time.Hour, d)
	}
}

func TestParseStaleDuration_Months(t *testing.T) {
	d, err := ParseStaleDuration("3m")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d != 90*24*time.Hour {
		t.Errorf("expected 90 days (%v), got %v", 90*24*time.Hour, d)
	}
}

func TestParseStaleDuration_Invalid(t *testing.T) {
	tests := []string{"", "30", "abc", "30x", "0d", "-5d"}
	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			_, err := ParseStaleDuration(input)
			if err == nil {
				t.Errorf("expected error for %q, got nil", input)
			}
		})
	}
}

func TestParseRemoveFlags_MergedOnly(t *testing.T) {
	opts, err := ParseRemoveFlags([]string{"--merged"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !opts.Merged {
		t.Error("expected Merged=true")
	}
	if opts.All {
		t.Error("expected All=false")
	}
	if opts.Stale != 0 {
		t.Errorf("expected Stale=0, got %v", opts.Stale)
	}
}

func TestParseRemoveFlags_StaleOnly(t *testing.T) {
	opts, err := ParseRemoveFlags([]string{"--stale", "30d"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.Stale != 30*24*time.Hour {
		t.Errorf("expected Stale=30 days, got %v", opts.Stale)
	}
	if opts.Merged {
		t.Error("expected Merged=false")
	}
}

func TestParseRemoveFlags_All(t *testing.T) {
	opts, err := ParseRemoveFlags([]string{"--all"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !opts.All {
		t.Error("expected All=true")
	}
}

func TestParseRemoveFlags_Combined(t *testing.T) {
	opts, err := ParseRemoveFlags([]string{"--stale", "2w", "--merged"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !opts.Merged {
		t.Error("expected Merged=true")
	}
	if opts.Stale != 14*24*time.Hour {
		t.Errorf("expected Stale=14 days, got %v", opts.Stale)
	}
}

func TestParseRemoveFlags_InvalidStale(t *testing.T) {
	_, err := ParseRemoveFlags([]string{"--stale", "bad"})
	if err == nil {
		t.Fatal("expected error for invalid stale duration")
	}
}

func TestParseRemoveFlags_RepoPath(t *testing.T) {
	opts, err := ParseRemoveFlags([]string{"--all", "/some/path"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.RepoPath != "/some/path" {
		t.Errorf("expected RepoPath=/some/path, got %s", opts.RepoPath)
	}
}

func TestValidateRemoveForNonInteractive_NoFilters(t *testing.T) {
	opts := &RemoveOptions{}
	err := ValidateRemoveForNonInteractive(opts)
	if err == nil {
		t.Fatal("expected error when no filters specified")
	}
	if !strings.Contains(err.Error(), "at least one filter") {
		t.Errorf("expected 'at least one filter' error, got: %v", err)
	}
}

func TestValidateRemoveForNonInteractive_Valid(t *testing.T) {
	tests := []struct {
		name string
		opts *RemoveOptions
	}{
		{"merged", &RemoveOptions{Merged: true}},
		{"stale", &RemoveOptions{Stale: 30 * 24 * time.Hour}},
		{"all", &RemoveOptions{All: true}},
		{"combined", &RemoveOptions{Merged: true, Stale: 14 * 24 * time.Hour}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateRemoveForNonInteractive(tt.opts); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestRemoveCLICommand_MergedOnly(t *testing.T) {
	opts := &RemoveOptions{Merged: true}
	cmd := RemoveCLICommand(opts)
	if !strings.Contains(cmd, "sentei remove") {
		t.Errorf("expected 'sentei remove', got %s", cmd)
	}
	if !strings.Contains(cmd, "--merged") {
		t.Errorf("expected '--merged', got %s", cmd)
	}
}

func TestRemoveCLICommand_StaleOnly(t *testing.T) {
	opts := &RemoveOptions{Stale: 30 * 24 * time.Hour}
	cmd := RemoveCLICommand(opts)
	if !strings.Contains(cmd, "--stale 30d") {
		t.Errorf("expected '--stale 30d', got %s", cmd)
	}
}

func TestRemoveCLICommand_All(t *testing.T) {
	opts := &RemoveOptions{All: true}
	cmd := RemoveCLICommand(opts)
	if !strings.Contains(cmd, "--all") {
		t.Errorf("expected '--all', got %s", cmd)
	}
}

func TestRemoveCLICommand_DryRun(t *testing.T) {
	opts := &RemoveOptions{All: true, DryRun: true}
	cmd := RemoveCLICommand(opts)
	if !strings.Contains(cmd, "--dry-run") {
		t.Errorf("expected '--dry-run', got %s", cmd)
	}
}

func TestRemoveCLICommand_WithRepoPath(t *testing.T) {
	opts := &RemoveOptions{All: true, RepoPath: "/some/repo"}
	cmd := RemoveCLICommand(opts)
	if !strings.HasSuffix(cmd, " /some/repo") {
		t.Errorf("expected command to end with repo path, got %s", cmd)
	}
	if !strings.Contains(cmd, "--all") {
		t.Errorf("expected '--all', got %s", cmd)
	}
}

func TestResolveFilters_StaleFilter(t *testing.T) {
	now := time.Now()
	worktrees := []git.Worktree{
		{Path: "/old", Branch: "refs/heads/feature/old", LastCommitDate: now.Add(-60 * 24 * time.Hour)},
		{Path: "/new", Branch: "refs/heads/feature/new", LastCommitDate: now.Add(-5 * 24 * time.Hour)},
	}
	opts := &RemoveOptions{Stale: 30 * 24 * time.Hour}
	result := ResolveFilters(worktrees, opts, nil, "", nil)
	if len(result) != 1 {
		t.Fatalf("expected 1 worktree, got %d", len(result))
	}
	if result[0].Path != "/old" {
		t.Errorf("expected /old, got %s", result[0].Path)
	}
}

func TestResolveFilters_MergedFilter(t *testing.T) {
	worktrees := []git.Worktree{
		{Path: "/merged", Branch: "refs/heads/feature/merged"},
		{Path: "/unmerged", Branch: "refs/heads/feature/unmerged"},
	}
	// Mock: only "feature/merged" is merged
	isMerged := func(branch string) bool {
		return branch == "feature/merged"
	}
	opts := &RemoveOptions{Merged: true}
	result := ResolveFilters(worktrees, opts, nil, "", isMerged)
	if len(result) != 1 {
		t.Fatalf("expected 1 worktree, got %d", len(result))
	}
	if result[0].Path != "/merged" {
		t.Errorf("expected /merged, got %s", result[0].Path)
	}
}

func TestResolveFilters_AllFilter(t *testing.T) {
	worktrees := []git.Worktree{
		{Path: "/a", Branch: "refs/heads/feature/a"},
		{Path: "/b", Branch: "refs/heads/feature/b"},
	}
	opts := &RemoveOptions{All: true}
	result := ResolveFilters(worktrees, opts, nil, "", nil)
	if len(result) != 2 {
		t.Fatalf("expected 2 worktrees, got %d", len(result))
	}
}

func TestResolveFilters_ProtectedExclusion(t *testing.T) {
	worktrees := []git.Worktree{
		{Path: "/main", Branch: "refs/heads/main"},
		{Path: "/feature", Branch: "refs/heads/feature/x"},
	}
	opts := &RemoveOptions{All: true}
	result := ResolveFilters(worktrees, opts, nil, "", nil)
	// main is protected and should be excluded
	if len(result) != 1 {
		t.Fatalf("expected 1 worktree, got %d", len(result))
	}
	if result[0].Path != "/feature" {
		t.Errorf("expected /feature, got %s", result[0].Path)
	}
}

func TestResolveFilters_ProtectsNonStandardDefaultBranch(t *testing.T) {
	worktrees := []git.Worktree{
		{Path: "/production", Branch: "refs/heads/production"},
		{Path: "/feature", Branch: "refs/heads/feature/x"},
	}
	opts := &RemoveOptions{All: true}
	// Default branch is "production" (non-standard) — it must be excluded.
	result := ResolveFilters(worktrees, opts, nil, "production", nil)
	if len(result) != 1 {
		t.Fatalf("expected 1 worktree, got %d", len(result))
	}
	if result[0].Path != "/feature" {
		t.Errorf("the default-branch worktree must be protected; got %s", result[0].Path)
	}
}

func TestCheckMerged_DefaultBranchNeverMergedIntoItself(t *testing.T) {
	// The self case must short-circuit before touching the runner, so a nil
	// runner here proves git is never invoked for the default branch.
	checker := CheckMerged(nil, "/repo", "production")
	if checker("production") {
		t.Error("the default branch must never report as merged into itself")
	}
}

func TestResolveFilters_CustomProtectedBranches(t *testing.T) {
	worktrees := []git.Worktree{
		{Path: "/staging", Branch: "refs/heads/staging"},
		{Path: "/feature", Branch: "refs/heads/feature/x"},
	}
	opts := &RemoveOptions{All: true}
	result := ResolveFilters(worktrees, opts, []string{"staging"}, "", nil)
	if len(result) != 1 {
		t.Fatalf("expected 1 worktree, got %d", len(result))
	}
	if result[0].Path != "/feature" {
		t.Errorf("expected /feature, got %s", result[0].Path)
	}
}

func TestResolveFilters_CombinedORLogic(t *testing.T) {
	now := time.Now()
	worktrees := []git.Worktree{
		{Path: "/old", Branch: "refs/heads/feature/old", LastCommitDate: now.Add(-60 * 24 * time.Hour)},
		{Path: "/merged", Branch: "refs/heads/feature/merged", LastCommitDate: now.Add(-1 * 24 * time.Hour)},
		{Path: "/active", Branch: "refs/heads/feature/active", LastCommitDate: now.Add(-1 * 24 * time.Hour)},
	}
	isMerged := func(branch string) bool {
		return branch == "feature/merged"
	}
	opts := &RemoveOptions{Stale: 30 * 24 * time.Hour, Merged: true}
	result := ResolveFilters(worktrees, opts, nil, "", isMerged)
	if len(result) != 2 {
		t.Fatalf("expected 2 worktrees, got %d", len(result))
	}
	paths := map[string]bool{}
	for _, wt := range result {
		paths[wt.Path] = true
	}
	if !paths["/old"] || !paths["/merged"] {
		t.Errorf("expected /old and /merged, got %v", paths)
	}
}

func TestResolveFilters_ExcludesBareWorktrees(t *testing.T) {
	worktrees := []git.Worktree{
		{Path: "/bare", Branch: "", IsBare: true},
		{Path: "/feature", Branch: "refs/heads/feature/x"},
	}
	opts := &RemoveOptions{All: true}
	result := ResolveFilters(worktrees, opts, nil, "", nil)
	if len(result) != 1 {
		t.Fatalf("expected 1 worktree, got %d", len(result))
	}
	if result[0].Path != "/feature" {
		t.Errorf("expected /feature, got %s", result[0].Path)
	}
}

func TestResolveFilters_IncludesLockedWorktrees(t *testing.T) {
	worktrees := []git.Worktree{
		{Path: "/bare", IsBare: true},
		{Path: "/locked", Branch: "refs/heads/feature/locked", IsLocked: true},
		{Path: "/clean", Branch: "refs/heads/feature/clean"},
	}
	opts := &RemoveOptions{All: true}
	result := ResolveFilters(worktrees, opts, nil, "", nil)

	paths := make(map[string]bool)
	for _, wt := range result {
		paths[wt.Path] = true
	}

	if !paths["/locked"] {
		t.Error("locked worktree should be included in --all filter results")
	}
	if !paths["/clean"] {
		t.Error("clean worktree should be included")
	}
	if paths["/bare"] {
		t.Error("bare worktree should still be excluded")
	}
}
