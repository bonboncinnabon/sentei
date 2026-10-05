package creator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/integration"
)

func TestScanArtifacts(t *testing.T) {
	tests := []struct {
		name      string
		dirs      []string
		integs    []integration.Integration
		wantCount int
	}{
		{name: "finds artifacts", dirs: []string{".code-review-graph"}, integs: []integration.Integration{{Name: "code-review-graph", Teardown: integration.TeardownSpec{Dirs: []string{".code-review-graph/"}}}}, wantCount: 1},
		{name: "no artifacts", integs: []integration.Integration{{Name: "code-review-graph", Teardown: integration.TeardownSpec{Dirs: []string{".code-review-graph/"}}}}},
		{name: "multiple artifacts", dirs: []string{".code-review-graph", ".cocoindex_code"}, integs: []integration.Integration{
			{Name: "code-review-graph", Teardown: integration.TeardownSpec{Dirs: []string{".code-review-graph/"}}},
			{Name: "cocoindex-code", Teardown: integration.TeardownSpec{Dirs: []string{".cocoindex_code/"}}},
		}, wantCount: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wtDir := t.TempDir()
			for _, dir := range tt.dirs {
				if err := os.MkdirAll(filepath.Join(wtDir, dir), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if artifacts := ScanArtifacts(wtDir, tt.integs); len(artifacts) != tt.wantCount {
				t.Errorf("artifact count = %d, want %d", len(artifacts), tt.wantCount)
			}
		})
	}
}
