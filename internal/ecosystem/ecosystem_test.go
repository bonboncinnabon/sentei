package ecosystem

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/config"
)

// boolPtr is a helper to get a pointer to a bool literal.
func boolPtr(b bool) *bool { return &b }

// minimalRegistry returns a Registry with three ecosystems in priority order:
// pnpm, npm, go.
func minimalRegistry() *Registry {
	return NewRegistry([]config.EcosystemConfig{
		{
			Name:    "pnpm",
			Detect:  config.DetectConfig{Files: []string{"pnpm-lock.yaml"}},
			Install: config.InstallConfig{Command: "pnpm install"},
		},
		{
			Name:    "npm",
			Detect:  config.DetectConfig{Files: []string{"package-lock.json"}},
			Install: config.InstallConfig{Command: "npm install"},
		},
		{
			Name:    "go",
			Detect:  config.DetectConfig{Files: []string{"go.mod"}},
			Install: config.InstallConfig{Command: "go mod download"},
		},
	})
}

// createFiles creates the given file names (with empty content) inside dir.
func createFiles(t *testing.T, dir string, names []string) {
	t.Helper()
	for _, name := range names {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte{}, 0600); err != nil {
			t.Fatalf("creating test file %s: %v", path, err)
		}
	}
}

func TestDetect(t *testing.T) {
	tests := []struct {
		name      string
		files     []string
		wantNames []string
	}{
		{
			name:      "go project",
			files:     []string{"go.mod"},
			wantNames: []string{"go"},
		},
		{
			name:      "pnpm project",
			files:     []string{"pnpm-lock.yaml", "package.json"},
			wantNames: []string{"pnpm"},
		},
		{
			name:      "multi-language project",
			files:     []string{"go.mod", "package-lock.json"},
			wantNames: []string{"npm", "go"},
		},
		{
			name:      "no ecosystem",
			files:     []string{"README.md"},
			wantNames: nil,
		},
		{
			name:      "pnpm wins over npm when both present",
			files:     []string{"pnpm-lock.yaml", "package-lock.json"},
			wantNames: []string{"pnpm", "npm"},
		},
	}

	reg := minimalRegistry()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			createFiles(t, dir, tc.files)

			got, err := reg.Detect(dir)
			if err != nil {
				t.Fatalf("Detect returned unexpected error: %v", err)
			}

			if len(got) != len(tc.wantNames) {
				t.Fatalf("got %d ecosystems %v, want %d %v", len(got), namesOf(got), len(tc.wantNames), tc.wantNames)
			}
			for i, want := range tc.wantNames {
				if got[i].Name != want {
					t.Errorf("ecosystems[%d]: got %q, want %q", i, got[i].Name, want)
				}
			}
		})
	}
}

func TestDetect_DisabledEcosystem(t *testing.T) {
	dir := t.TempDir()
	createFiles(t, dir, []string{"go.mod"})

	reg := NewRegistry([]config.EcosystemConfig{
		{
			Name:    "go",
			Enabled: boolPtr(false),
			Detect:  config.DetectConfig{Files: []string{"go.mod"}},
			Install: config.InstallConfig{Command: "go mod download"},
		},
	})

	got, err := reg.Detect(dir)
	if err != nil {
		t.Fatalf("Detect returned unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected no ecosystems for disabled entry, got %v", namesOf(got))
	}
}

func TestDetect_GlobPattern(t *testing.T) {
	dir := t.TempDir()
	createFiles(t, dir, []string{"MyApp.sln"})

	reg := NewRegistry([]config.EcosystemConfig{
		{
			Name:    "dotnet",
			Detect:  config.DetectConfig{Files: []string{"*.sln", "*.csproj"}},
			Install: config.InstallConfig{Command: "dotnet restore"},
		},
	})

	got, err := reg.Detect(dir)
	if err != nil {
		t.Fatalf("Detect returned unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Name != "dotnet" {
		t.Errorf("expected [dotnet], got %v", namesOf(got))
	}
}

func TestRegistryAll(t *testing.T) {
	reg := minimalRegistry()
	all := reg.All()
	if len(all) != 3 {
		t.Errorf("All() returned %d ecosystems, want 3", len(all))
	}
	wantOrder := []string{"pnpm", "npm", "go"}
	for i, want := range wantOrder {
		if all[i].Name != want {
			t.Errorf("All()[%d]: got %q, want %q", i, all[i].Name, want)
		}
	}
}

// namesOf extracts the Name field from a slice of Ecosystem values.
func namesOf(ecosystems []Ecosystem) []string {
	names := make([]string, len(ecosystems))
	for i, e := range ecosystems {
		names[i] = e.Name
	}
	return names
}
