package cleanup

import (
	"slices"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/testutil/mock"
)

func TestPruneRemoteRefs(t *testing.T) {
	tests := []struct {
		name        string
		dryRun      bool
		pruneOutput string
		want        []string
	}{
		{
			name:        "no stale refs",
			pruneOutput: "",
		},
		{
			name:        "some stale refs",
			pruneOutput: "Pruning origin\nURL: git@github.com:Org/repo.git\n * [would prune] origin/feature/old\n * [would prune] origin/fix/done",
			want:        []string{"origin/feature/old", "origin/fix/done"},
		},
		{
			name:        "dry run reports the same refs",
			dryRun:      true,
			pruneOutput: "Pruning origin\nURL: git@github.com:Org/repo.git\n * [would prune] origin/feature/old",
			want:        []string{"origin/feature/old"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &mock.Runner{Responses: map[string]mock.Response{
				"/repo:[remote]":                        {Output: "origin"},
				"/repo:[remote prune origin --dry-run]": {Output: tt.pruneOutput},
				"/repo:[fetch --prune origin]":          {Output: ""},
			}}
			opts := Options{DryRun: tt.dryRun}
			events := collectEvents(t)

			stale, err := PruneRemoteRefs(runner, "/repo", opts, events.Emit)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !slices.Equal(stale, tt.want) {
				t.Errorf("stale = %v, want %v", stale, tt.want)
			}
		})
	}
}

// The bracketed label is translated by git; the bullet and the ref name are
// not. A dangling-symref notice has no bullet and is not a pruned ref.
func TestParseStaleRefs_DoesNotDependOnTheLabelText(t *testing.T) {
	output := "Pruning origin\n" +
		"URL: git@github.com:Org/repo.git\n" +
		" * [würde veralteten Branch entfernen] origin/feature/old\n" +
		" origin/HEAD will become dangling!"

	if got, want := parseStaleRefs(output), []string{"origin/feature/old"}; !slices.Equal(got, want) {
		t.Errorf("parseStaleRefs = %v, want %v", got, want)
	}
}

func TestPruneRemoteRefs_NoRemotes(t *testing.T) {
	tests := []struct {
		name         string
		remoteOutput string
	}{
		{name: "no remotes at all", remoteOutput: ""},
		{name: "remote exists but not origin", remoteOutput: "upstream"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &mock.Runner{Responses: map[string]mock.Response{
				"/repo:[remote]": {Output: tt.remoteOutput},
			}}
			events := collectEvents(t)

			stale, err := PruneRemoteRefs(runner, "/repo", Options{}, events.Emit)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(stale) != 0 {
				t.Errorf("stale = %v, want none", stale)
			}

			for _, call := range runner.Calls {
				if call == "/repo:[remote prune origin --dry-run]" {
					t.Error("should not call remote prune when origin doesn't exist")
				}
			}
		})
	}
}

func TestPruneRemoteRefs_DryRunDoesNotFetch(t *testing.T) {
	runner := &mock.Runner{Responses: map[string]mock.Response{
		"/repo:[remote]":                        {Output: "origin"},
		"/repo:[remote prune origin --dry-run]": {Output: " * [would prune] origin/old"},
	}}
	opts := Options{DryRun: true}
	events := collectEvents(t)

	_, err := PruneRemoteRefs(runner, "/repo", opts, events.Emit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, call := range runner.Calls {
		if call == "/repo:[fetch --prune origin]" {
			t.Error("dry-run should not call fetch --prune")
		}
	}
}
