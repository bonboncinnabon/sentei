package config

import (
	"os"
	"path/filepath"
	"testing"
)

// writeGlobalConfig writes base/sentei/config.yaml adding one ecosystem.
func writeGlobalConfig(t *testing.T, base, ecosystem string) {
	t.Helper()
	dir := filepath.Join(base, "sentei")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "ecosystems:\n  - name: " + ecosystem + "\n    detect:\n      files: [marker]\n"
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadConfig_GlobalConfigIsNeverRelativeToWorkingDirectory(t *testing.T) {
	// A file exactly where a relative ".config" or "rel" base would resolve.
	cwd := t.TempDir()
	writeGlobalConfig(t, filepath.Join(cwd, ".config"), "from-working-directory")
	writeGlobalConfig(t, filepath.Join(cwd, "rel"), "from-working-directory")
	t.Chdir(cwd)

	home := t.TempDir()
	writeGlobalConfig(t, filepath.Join(home, ".config"), "from-home")

	tests := []struct {
		name, home, xdg string
		wantHome        bool
	}{
		{name: "HOME and XDG_CONFIG_HOME unset", home: "", xdg: ""},
		{name: "relative HOME", home: ".", xdg: ""},
		{name: "relative XDG_CONFIG_HOME without HOME", home: "", xdg: "rel"},
		{name: "relative XDG_CONFIG_HOME falls back to HOME", home: home, xdg: "rel", wantHome: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HOME", tt.home)
			t.Setenv("XDG_CONFIG_HOME", tt.xdg)

			cfg, err := LoadConfig(t.TempDir())
			if err != nil {
				t.Fatalf("LoadConfig() error: %v", err)
			}
			if hasEcosystem(cfg, "from-working-directory") {
				t.Error("the global config was read relative to the working directory")
			}
			if got := hasEcosystem(cfg, "from-home"); got != tt.wantHome {
				t.Errorf("config from HOME loaded = %v, want %v", got, tt.wantHome)
			}
		})
	}
}
