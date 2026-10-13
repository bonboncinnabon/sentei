package ecosystem

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/config"
)

// boolPtr is a helper to get a pointer to a bool literal.
func boolPtr(b bool) *bool { return &b }

// minimalEcosystems returns three ecosystems in priority order: pnpm, npm, go.
func minimalEcosystems() []config.EcosystemConfig {
	return []config.EcosystemConfig{
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
	}
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

	ecosystems := minimalEcosystems()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			createFiles(t, dir, tc.files)

			got := Detect(ecosystems, dir)

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

	got := Detect([]config.EcosystemConfig{
		{
			Name:    "go",
			Enabled: boolPtr(false),
			Detect:  config.DetectConfig{Files: []string{"go.mod"}},
			Install: config.InstallConfig{Command: "go mod download"},
		},
	}, dir)
	if len(got) != 0 {
		t.Errorf("expected no ecosystems for disabled entry, got %v", namesOf(got))
	}
}

func TestDetect_GlobPattern(t *testing.T) {
	dir := t.TempDir()
	createFiles(t, dir, []string{"MyApp.sln"})

	got := Detect([]config.EcosystemConfig{
		{
			Name:    "dotnet",
			Detect:  config.DetectConfig{Files: []string{"*.sln", "*.csproj"}},
			Install: config.InstallConfig{Command: "dotnet restore"},
		},
	}, dir)
	if len(got) != 1 || got[0].Name != "dotnet" {
		t.Errorf("expected [dotnet], got %v", namesOf(got))
	}
}

// namesOf extracts the names from a slice of ecosystems.
func namesOf(ecosystems []config.EcosystemConfig) []string {
	names := make([]string, len(ecosystems))
	for i, e := range ecosystems {
		names[i] = e.Name
	}
	return names
}

func TestDetect_GlobCharactersInDirectoryPath(t *testing.T) {
	ecosystems := []config.EcosystemConfig{
		{Name: "go", Detect: config.DetectConfig{Files: []string{"go.mod"}}},
		{Name: "dotnet", Detect: config.DetectConfig{Files: []string{"*.csproj"}}},
		{Name: "pnpm", Detect: config.DetectConfig{Files: []string{"./pnpm-lock.yaml"}}},
		{Name: "absent", Detect: config.DetectConfig{Files: []string{"*.sln"}}},
	}

	// Worktree directories take their name from the branch, and "[", "*" and
	// "?" are all legal there.
	for _, name := range []string{"fix[1]", "release-*", "what?", "plain"} {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), name)
			if err := os.Mkdir(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			createFiles(t, dir, []string{"go.mod", "app.csproj", "pnpm-lock.yaml"})

			detected := Detect(ecosystems, dir)
			if want := []string{"go", "dotnet", "pnpm"}; !slices.Equal(namesOf(detected), want) {
				t.Errorf("Detect(%q) = %v, want %v", name, namesOf(detected), want)
			}
		})
	}
}
