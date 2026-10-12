package tui

import (
	"slices"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/config"
)

// The "Copy environment files" row lists the files of every detected
// ecosystem. Unchecking an ecosystem's install row used to drop its env files
// while the env row stayed checked.
func TestBuildCreatorOptions_EnvFilesDoNotDependOnInstallSelection(t *testing.T) {
	m := createOptionsModel()
	m.create.ecosystems = []config.EcosystemConfig{{Name: "node", EnvFiles: []string{".env", ".env.local"}}}
	m.create.ecoEnabled = map[string]bool{"node": false}
	m.create.copyEnvFiles = true

	opts := m.buildCreatorOptions()

	if len(opts.Ecosystems) != 0 {
		t.Errorf("no install was selected, got ecosystems %v", opts.Ecosystems)
	}
	if !opts.CopyEnvFiles || !slices.Equal(opts.EnvFiles, []string{".env", ".env.local"}) {
		t.Errorf("CopyEnvFiles=%v EnvFiles=%v, want the detected ecosystem's env files", opts.CopyEnvFiles, opts.EnvFiles)
	}
}
