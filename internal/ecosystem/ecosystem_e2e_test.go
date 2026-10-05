package ecosystem_test

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/config"
	"github.com/bonboncinnabon/sentei/internal/ecosystem"
)

func TestDetect_E2E_GoProject(t *testing.T) {
	repoDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoDir, "go.mod"), []byte("module example.com/myapp\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatalf("WriteFile go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("WriteFile main.go: %v", err)
	}

	xdgDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdgDir)

	cfg, err := config.LoadConfig(repoDir)
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}

	reg := ecosystem.NewRegistry(cfg.Ecosystems)
	detected, err := reg.Detect(repoDir)
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}

	if len(detected) != 1 {
		t.Fatalf("expected 1 ecosystem, got %d: %v", len(detected), namesOf(detected))
	}
	if detected[0].Name != "go" {
		t.Errorf("detected[0].Name: got %q, want %q", detected[0].Name, "go")
	}
}

func TestDetect_E2E_PnpmMonorepo(t *testing.T) {
	repoDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(repoDir, "pnpm-lock.yaml"), []byte("lockfileVersion: '6.0'\n"), 0o644); err != nil {
		t.Fatalf("WriteFile pnpm-lock.yaml: %v", err)
	}

	pnpmWorkspace := `packages:
  - "packages/*"
  - "apps/*"
`
	if err := os.WriteFile(filepath.Join(repoDir, "pnpm-workspace.yaml"), []byte(pnpmWorkspace), 0o644); err != nil {
		t.Fatalf("WriteFile pnpm-workspace.yaml: %v", err)
	}

	for _, dir := range []string{"packages/core", "packages/utils", "apps/web"} {
		if err := os.MkdirAll(filepath.Join(repoDir, dir), 0o755); err != nil {
			t.Fatalf("MkdirAll %s: %v", dir, err)
		}
	}

	xdgDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdgDir)

	cfg, err := config.LoadConfig(repoDir)
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}

	reg := ecosystem.NewRegistry(cfg.Ecosystems)
	detected, err := reg.Detect(repoDir)
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}

	var pnpmEco *ecosystem.Ecosystem
	for i := range detected {
		if detected[i].Name == "pnpm" {
			pnpmEco = &detected[i]
			break
		}
	}
	if pnpmEco == nil {
		t.Fatalf("pnpm not detected, got: %v", namesOf(detected))
	}

	workspaceDetect := pnpmEco.Config.Install.WorkspaceDetect
	if workspaceDetect == "" {
		t.Fatal("pnpm WorkspaceDetect is empty")
	}

	workspaces, err := ecosystem.DetectWorkspaces(repoDir, workspaceDetect)
	if err != nil {
		t.Fatalf("DetectWorkspaces() error: %v", err)
	}
	if len(workspaces) != 3 {
		t.Fatalf("expected 3 workspaces, got %d: %v", len(workspaces), workspaces)
	}

	sorted := make([]string, len(workspaces))
	copy(sorted, workspaces)
	sort.Strings(sorted)
	want := []string{"apps/web", "packages/core", "packages/utils"}
	for i, w := range want {
		if sorted[i] != w {
			t.Errorf("workspaces[%d]: got %q, want %q", i, sorted[i], w)
		}
	}
}

func TestDetect_E2E_MultiLanguage(t *testing.T) {
	repoDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(repoDir, "go.mod"), []byte("module example.com/myapp\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatalf("WriteFile go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "package-lock.json"), []byte(`{"name":"myapp","lockfileVersion":3}`), 0o644); err != nil {
		t.Fatalf("WriteFile package-lock.json: %v", err)
	}

	xdgDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdgDir)

	cfg, err := config.LoadConfig(repoDir)
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}

	reg := ecosystem.NewRegistry(cfg.Ecosystems)
	detected, err := reg.Detect(repoDir)
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}

	if len(detected) != 2 {
		t.Fatalf("expected 2 ecosystems, got %d: %v", len(detected), namesOf(detected))
	}

	// npm comes before go in the embedded defaults (npm is index 2, go is index 5)
	names := namesOf(detected)
	hasNpm := false
	hasGo := false
	npmIdx := -1
	goIdx := -1
	for i, n := range names {
		if n == "npm" {
			hasNpm = true
			npmIdx = i
		}
		if n == "go" {
			hasGo = true
			goIdx = i
		}
	}
	if !hasNpm {
		t.Error("npm not detected")
	}
	if !hasGo {
		t.Error("go not detected")
	}
	if hasNpm && hasGo && npmIdx > goIdx {
		t.Errorf("expected npm before go in results (npm index %d, go index %d)", npmIdx, goIdx)
	}
}

func namesOf(ecosystems []ecosystem.Ecosystem) []string {
	names := make([]string, len(ecosystems))
	for i, e := range ecosystems {
		names[i] = e.Name
	}
	return names
}
