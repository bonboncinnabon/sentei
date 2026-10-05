package cleanup

import (
	"testing"

	"github.com/bonboncinnabon/sentei/internal/testutil/mock"
)

func TestPruneRemoteRefs(t *testing.T) {
	tests := []struct {
		name        string
		dryRun      bool
		pruneOutput string
		wantCount   int
	}{
		{
			name:        "no stale refs",
			pruneOutput: "",
			wantCount:   0,
		},
		{
			name:        "some stale refs",
			pruneOutput: "Pruning origin\nURL: git@github.com:Org/repo.git\n * [would prune] origin/feature/old\n * [would prune] origin/fix/done",
			wantCount:   2,
		},
		{
			name:        "dry run reports count",
			dryRun:      true,
			pruneOutput: "Pruning origin\nURL: git@github.com:Org/repo.git\n * [would prune] origin/feature/old",
			wantCount:   1,
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

			count, err := PruneRemoteRefs(runner, "/repo", opts, events.Emit)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if count != tt.wantCount {
				t.Errorf("count = %d, want %d", count, tt.wantCount)
			}
		})
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

			count, err := PruneRemoteRefs(runner, "/repo", Options{}, events.Emit)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if count != 0 {
				t.Errorf("count = %d, want 0", count)
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
