package cleanup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/testutil/mock"
)

func copyFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("reading fixture %s: %v", name, err)
	}
	tmp := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}
	return tmp
}

func TestDedupConfig(t *testing.T) {
	tests := []struct {
		name            string
		fixture         string
		wantRemoved     int
		wantLinesBefore int
		wantLinesAfter  int
	}{
		{
			name:            "already clean",
			fixture:         "clean.gitconfig",
			wantRemoved:     0,
			wantLinesBefore: 10,
			wantLinesAfter:  10,
		},
		{
			name:            "removes exact duplicates",
			fixture:         "bloated.gitconfig",
			wantRemoved:     4,
			wantLinesBefore: 24,
			wantLinesAfter:  20,
		},
		{
			name:            "preserves multi-valued keys",
			fixture:         "multi-value.gitconfig",
			wantRemoved:     0,
			wantLinesBefore: 10,
			wantLinesAfter:  10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := copyFixture(t, tt.fixture)
			ec := collectEvents(t)

			result, err := DedupConfig(path, Options{}, ec.Emit)
			if err != nil {
				t.Fatalf("DedupConfig error: %v", err)
			}

			if result.Before != tt.wantLinesBefore {
				t.Errorf("Before = %d, want %d", result.Before, tt.wantLinesBefore)
			}
			if result.After != tt.wantLinesAfter {
				t.Errorf("After = %d, want %d", result.After, tt.wantLinesAfter)
			}
			if result.Removed != tt.wantRemoved {
				t.Errorf("Removed = %d, want %d", result.Removed, tt.wantRemoved)
			}
		})
	}
}

func TestDedupConfig_DryRun(t *testing.T) {
	path := copyFixture(t, "bloated.gitconfig")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading original: %v", err)
	}

	ec := collectEvents(t)
	result, err := DedupConfig(path, Options{DryRun: true}, ec.Emit)
	if err != nil {
		t.Fatalf("DedupConfig error: %v", err)
	}

	if result.Removed == 0 {
		t.Error("expected Removed > 0")
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading file after dry run: %v", err)
	}
	if string(after) != string(original) {
		t.Error("file was modified during dry run")
	}
}

func TestDedupConfig_CreatesBackup(t *testing.T) {
	path := copyFixture(t, "bloated.gitconfig")
	ec := collectEvents(t)

	_, err := DedupConfig(path, Options{}, ec.Emit)
	if err != nil {
		t.Fatalf("DedupConfig error: %v", err)
	}

	bakPath := path + ".bak"
	if _, err := os.Stat(bakPath); os.IsNotExist(err) {
		t.Errorf("backup file %s not created", bakPath)
	}
}

func TestPurgeOrphanedBranchConfigs(t *testing.T) {
	tests := []struct {
		name          string
		fixture       string
		existBranches string
		wantRemoved   int
	}{
		{
			name:          "no orphans",
			fixture:       "bloated.gitconfig",
			existBranches: "main\nfeature/old-work\nfix/stale-branch",
			wantRemoved:   0,
		},
		{
			name:          "removes orphaned sections",
			fixture:       "bloated.gitconfig",
			existBranches: "main",
			wantRemoved:   2,
		},
		{
			name:          "all branches gone",
			fixture:       "bloated.gitconfig",
			existBranches: "",
			wantRemoved:   3,
		},
		{
			name:          "special characters in branch names",
			fixture:       "special-chars.gitconfig",
			existBranches: "feature/deep/nested.v2",
			wantRemoved:   2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := copyFixture(t, tt.fixture)
			runner := &mock.Runner{Responses: map[string]mock.Response{
				"/repo:[branch --format=%(refname:short)]": {Output: tt.existBranches},
			}}
			opts := Options{DryRun: false}
			events := collectEvents(t)

			result, err := PurgeOrphanedBranchConfigs(runner, "/repo", path, opts, events.Emit)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Removed != tt.wantRemoved {
				t.Errorf("Removed = %d, want %d", result.Removed, tt.wantRemoved)
			}
		})
	}
}
