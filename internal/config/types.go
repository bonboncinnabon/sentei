package config

import "github.com/bonboncinnabon/sentei/internal/worktreefile"

// Config is the top-level configuration for sentei.
type Config struct {
	Ecosystems          []EcosystemConfig   `yaml:"ecosystems"`
	ProtectedBranches   []string            `yaml:"protected_branches"`
	IntegrationsEnabled []string            `yaml:"integrations_enabled"`
	WorktreeFiles       []worktreefile.Rule `yaml:"worktree_files"`
}

// EcosystemConfig describes how to detect and install a language/tool ecosystem.
type EcosystemConfig struct {
	Name        string        `yaml:"name"`
	Enabled     *bool         `yaml:"enabled,omitempty"`
	Detect      DetectConfig  `yaml:"detect"`
	Install     InstallConfig `yaml:"install"`
	EnvFiles    []string      `yaml:"env_files"`
	PostInstall []string      `yaml:"post_install"`
	Source      string        `yaml:"-"` // "embedded", "global", or "per-repo"
}

// IsEnabled reports whether the ecosystem is active. An absent Enabled field
// is treated as true.
func (e *EcosystemConfig) IsEnabled() bool {
	return e.Enabled == nil || *e.Enabled
}

// DetectConfig holds the file patterns used to detect an ecosystem.
type DetectConfig struct {
	Files []string `yaml:"files"`
}

// InstallConfig describes how to install an ecosystem's dependencies. The
// command runs once, at the root of the new worktree.
type InstallConfig struct {
	Command string `yaml:"command"`
}
