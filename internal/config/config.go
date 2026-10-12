package config

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/worktreefile"
	"gopkg.in/yaml.v3"
)

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
		if len(over.EnvFiles) > 0 {
			e.EnvFiles = over.EnvFiles
		}
		e.Source = overlaySource
		result[i] = e
	}

	return result
}

// mergeConfigs merges overlay on top of base, returning a new Config. The
// worktree_files list is replaced entirely when the overlay provides one; an
// empty overlay list does not clear the base list.
func mergeConfigs(base, overlay *Config, overlaySource string) *Config {
	result := &Config{
		Ecosystems:    mergeEcosystems(base.Ecosystems, overlay.Ecosystems, overlaySource),
		WorktreeFiles: base.WorktreeFiles,
	}
	if len(overlay.WorktreeFiles) > 0 {
		result.WorktreeFiles = overlay.WorktreeFiles
	}
	return result
}

// validate checks the config for structural errors.
func validate(cfg *Config) error {
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

// repoRoot returns the directory whose .sentei.yaml belongs to the repository
// at repoPath: the parent of git's common dir, which is the container of a
// .bare layout or the top of a regular working tree. A plain bare repository
// is its own root, because its parent may hold unrelated repositories. Outside
// a repository the root is repoPath.
func repoRoot(runner git.CommandRunner, repoPath string) string {
	commonDir, err := git.CommonDir(runner, repoPath)
	if err != nil || commonDir == filepath.Clean(repoPath) {
		return repoPath
	}
	return filepath.Dir(commonDir)
}

// LoadOption configures optional behaviour of LoadConfig.
type LoadOption func(*loadOptions)

type loadOptions struct {
	runner git.CommandRunner
}

// WithRunner supplies the git command runner used to find the repo root.
func WithRunner(r git.CommandRunner) LoadOption {
	return func(o *loadOptions) { o.runner = r }
}

// LoadConfig is the public API for loading sentei configuration. It:
//  1. Loads the embedded defaults (Source="embedded").
//  2. Merges the global config (~/.config/sentei/config.yaml).
//  3. Resolves the repo root and merges .sentei.yaml from it.
//  4. Validates the result.
func LoadConfig(repoPath string, opts ...LoadOption) (*Config, error) {
	lo := loadOptions{runner: &git.GitRunner{}}
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

	repoCfg, err := loadFile(filepath.Join(repoRoot(lo.runner, repoPath), ".sentei.yaml"))
	if err != nil {
		return nil, fmt.Errorf("loading repo config: %w", err)
	}
	if repoCfg != nil {
		cfg = mergeConfigs(cfg, repoCfg, "per-repo")
	}

	if err := validate(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
