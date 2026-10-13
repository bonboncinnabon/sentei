package config

import "github.com/bonboncinnabon/sentei/internal/worktreefile"

// Config is the top-level configuration for sentei. A key with no field here
// is an error when a file is loaded, except the retired keys listed in
// retiredKeys.
type Config struct {
	Ecosystems    []EcosystemConfig   `yaml:"ecosystems"`
	WorktreeFiles []worktreefile.Rule `yaml:"worktree_files"`
}

// EcosystemConfig describes how to detect and install a language/tool ecosystem.
type EcosystemConfig struct {
	Name     string        `yaml:"name"`
	Enabled  *bool         `yaml:"enabled,omitempty"`
	Detect   DetectConfig  `yaml:"detect"`
	Install  InstallConfig `yaml:"install"`
	EnvFiles []string      `yaml:"env_files"`
	Source   string        `yaml:"-"` // "embedded", "global", or "per-repo"
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

	// cleared marks a file that writes `command: ""`, which turns an inherited
	// install off. Command alone cannot tell that from a file without the key,
	// which inherits.
	cleared bool
}
