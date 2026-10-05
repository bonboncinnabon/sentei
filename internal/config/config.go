package config

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bonboncinnabon/sentei/internal/worktreefile"
	"gopkg.in/yaml.v3"
)

// RepoRootResolver resolves the git common dir for a repository path.
// This mirrors git.CommandRunner but is defined here to avoid a circular
// dependency between config and git packages.
type RepoRootResolver interface {
	Run(dir string, args ...string) (string, error)
}

//go:embed defaults/ecosystems.yaml
var defaultEcosystemsYAML []byte

func loadEmbeddedDefaults() (*Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(defaultEcosystemsYAML, &cfg); err != nil {
		return nil, fmt.Errorf("parsing embedded defaults: %w", err)
	}
	return &cfg, nil
}

// loadFile reads and parses a YAML config file. Returns (nil, nil) if the file
// does not exist.
func loadFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading config file %s: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file %s: %w", path, err)
	}
	return &cfg, nil
}

// mergeEcosystems performs a keyed merge of overlay into base, using name as
// the key. Existing entries are updated field-by-field (only non-zero overlay
// fields replace base values). New entries are appended. Source is set on all
// changed or new entries.
func mergeEcosystems(base, overlay []EcosystemConfig, overlaySource string) []EcosystemConfig {
	if len(overlay) == 0 {
		return base
	}

	result := make([]EcosystemConfig, len(base))
	copy(result, base)

	index := make(map[string]int, len(result))
	for i, e := range result {
		index[e.Name] = i
	}

	for _, over := range overlay {
		i, exists := index[over.Name]
		if !exists {
			over.Source = overlaySource
			result = append(result, over)
			index[over.Name] = len(result) - 1
			continue
		}

		e := result[i]
		if over.Enabled != nil {
			e.Enabled = over.Enabled
		}
		if len(over.Detect.Files) > 0 {
			e.Detect.Files = over.Detect.Files
		}
		if over.Install.Command != "" {
			e.Install.Command = over.Install.Command
		}
		if over.Install.WorkspaceDetect != "" {
			e.Install.WorkspaceDetect = over.Install.WorkspaceDetect
		}
		if over.Install.WorkspaceInstall != "" {
			e.Install.WorkspaceInstall = over.Install.WorkspaceInstall
		}
		if over.Install.Parallel != nil {
			e.Install.Parallel = over.Install.Parallel
		}
		if len(over.EnvFiles) > 0 {
			e.EnvFiles = over.EnvFiles
		}
		if len(over.PostInstall) > 0 {
			e.PostInstall = over.PostInstall
		}
		e.Source = overlaySource
		result[i] = e
	}

	return result
}

// mergeConfigs merges overlay on top of base, returning a new Config. Scalar
// lists (ProtectedBranches, IntegrationsEnabled) are replaced entirely when
// the overlay provides them. Note: an empty list (e.g. `protected_branches: []`)
// in the overlay will NOT clear the base list — only a non-empty overlay replaces.
// This is intentional: YAML unmarshalling produces a nil slice for `[]`, which is
// indistinguishable from an absent field.
func mergeConfigs(base, overlay *Config, overlaySource string) *Config {
	result := &Config{
		Ecosystems:          mergeEcosystems(base.Ecosystems, overlay.Ecosystems, overlaySource),
		ProtectedBranches:   base.ProtectedBranches,
		IntegrationsEnabled: base.IntegrationsEnabled,
		WorktreeFiles:       base.WorktreeFiles,
	}
	if len(overlay.ProtectedBranches) > 0 {
		result.ProtectedBranches = overlay.ProtectedBranches
	}
	if len(overlay.IntegrationsEnabled) > 0 {
		result.IntegrationsEnabled = overlay.IntegrationsEnabled
	}
	if len(overlay.WorktreeFiles) > 0 {
		result.WorktreeFiles = overlay.WorktreeFiles
	}
	return result
}

// validate checks the config for structural errors and warns about unknown
// integration names. knownIntegrationNames is the set of recognised names.
func validate(cfg *Config, knownIntegrationNames []string) error {
	if err := worktreefile.ValidateRules(cfg.WorktreeFiles); err != nil {
		return err
	}
	for i, e := range cfg.Ecosystems {
		if !e.IsEnabled() {
			continue
		}
		if e.Name == "" {
			return fmt.Errorf("ecosystems[%d]: name is required", i)
		}
		if len(e.Detect.Files) == 0 {
			return fmt.Errorf("ecosystem %q: detect.files must not be empty", e.Name)
		}
	}
	known := make(map[string]struct{}, len(knownIntegrationNames))
	for _, n := range knownIntegrationNames {
		known[n] = struct{}{}
	}
	for _, name := range cfg.IntegrationsEnabled {
		if _, ok := known[name]; !ok {
			fmt.Fprintf(os.Stderr, "warning: unknown integration %q in integrations_enabled\n", name)
		}
	}
	return nil
}

// globalConfigPath returns the path to the global sentei config file, honouring
// XDG_CONFIG_HOME and defaulting to ~/.config.
func globalConfigPath() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "sentei", "config.yaml")
}

// resolveRepoRoot resolves a git working directory to the root of the bare
// repository (the directory containing the worktrees). It falls back to
// repoPath on any error. When runner is nil, it falls back to exec.Command
// directly (for use in contexts where a runner is not yet available).
func resolveRepoRoot(repoPath string, runner RepoRootResolver) string {
	var commonDir string
	if runner != nil {
		out, err := runner.Run(repoPath, "rev-parse", "--git-common-dir")
		if err != nil {
			return repoPath
		}
		commonDir = strings.TrimSpace(out)
	} else {
		out, err := exec.Command("git", "-C", repoPath, "rev-parse", "--git-common-dir").Output()
		if err != nil {
			return repoPath
		}
		commonDir = strings.TrimSpace(string(out))
	}
	if filepath.IsAbs(commonDir) {
		return filepath.Dir(commonDir)
	}
	return filepath.Dir(filepath.Join(repoPath, commonDir))
}

// LoadOption configures optional behaviour of LoadConfig.
type LoadOption func(*loadOptions)

type loadOptions struct {
	runner                RepoRootResolver
	knownIntegrationNames []string
}

// WithRunner supplies a git command runner for resolving the repo root.
func WithRunner(r RepoRootResolver) LoadOption {
	return func(o *loadOptions) { o.runner = r }
}

// WithKnownIntegrations supplies the set of valid integration names for
// config validation. When empty, integration name validation is skipped.
func WithKnownIntegrations(names []string) LoadOption {
	return func(o *loadOptions) { o.knownIntegrationNames = names }
}

// LoadConfig is the public API for loading sentei configuration. It:
//  1. Loads the embedded defaults (Source="embedded").
//  2. Merges the global config (~/.config/sentei/config.yaml).
//  3. Resolves the repo root and merges .sentei.yaml from it.
//  4. Validates the result.
func LoadConfig(repoPath string, opts ...LoadOption) (*Config, error) {
	var lo loadOptions
	for _, opt := range opts {
		opt(&lo)
	}

	cfg, err := loadEmbeddedDefaults()
	if err != nil {
		return nil, err
	}
	for i := range cfg.Ecosystems {
		cfg.Ecosystems[i].Source = "embedded"
	}

	globalPath := globalConfigPath()
	globalCfg, err := loadFile(globalPath)
	if err != nil {
		return nil, fmt.Errorf("loading global config: %w", err)
	}
	if globalCfg != nil {
		cfg = mergeConfigs(cfg, globalCfg, "global")
	}

	repoRoot := resolveRepoRoot(repoPath, lo.runner)
	repoCfg, err := loadFile(filepath.Join(repoRoot, ".sentei.yaml"))
	if err != nil {
		return nil, fmt.Errorf("loading repo config: %w", err)
	}
	if repoCfg != nil {
		cfg = mergeConfigs(cfg, repoCfg, "per-repo")
	}

	if err := validate(cfg, lo.knownIntegrationNames); err != nil {
		return nil, err
	}
	return cfg, nil
}
