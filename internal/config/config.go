package config

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/worktreefile"
	"gopkg.in/yaml.v3"
)

//go:embed defaults/ecosystems.yaml
var defaultEcosystemsYAML []byte

// retiredKeys were read by earlier versions and no longer do anything. A file
// that still carries them loads; every other unknown key is an error, so a
// misspelled key cannot silently leave a default in place.
// ponytail: matched by name at any depth; match on position too if a retired
// name is ever reused for a live key.
var retiredKeys = []string{
	"protected_branches", "integrations_enabled", "post_install",
	"workspace_detect", "workspace_install", "parallel",
}

// unknownKey matches the message yaml.v3 produces for a key with no field.
var unknownKey = regexp.MustCompile(`^(line \d+): field (\S+) not found in type \S+$`)

// parse decodes one YAML config document, rejecting keys sentei does not know.
func parse(data []byte) (*Config, error) {
	var cfg Config
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	err := decoder.Decode(&cfg)

	// A type error lists every problem; the rest of the document is decoded.
	var typeErr *yaml.TypeError
	switch {
	case err == nil, errors.Is(err, io.EOF): // io.EOF: nothing but comments
		return &cfg, nil
	case !errors.As(err, &typeErr):
		return nil, err
	}
	var problems []string
	for _, problem := range typeErr.Errors {
		match := unknownKey.FindStringSubmatch(problem)
		switch {
		case match == nil:
			problems = append(problems, problem)
		case !slices.Contains(retiredKeys, match[2]):
			problems = append(problems, fmt.Sprintf("%s: unknown key %q (check the spelling or remove it)", match[1], match[2]))
		}
	}
	if len(problems) == 0 {
		return &cfg, nil
	}
	return nil, errors.New(strings.Join(problems, "; "))
}

func loadEmbeddedDefaults() (*Config, error) {
	cfg, err := parse(defaultEcosystemsYAML)
	if err != nil {
		return nil, fmt.Errorf("parsing embedded defaults: %w", err)
	}
	return cfg, nil
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
	cfg, err := parse(data)
	if err != nil {
		return nil, fmt.Errorf("parsing config file %s: %w", path, err)
	}
	return cfg, nil
}

// mergeEcosystems performs a keyed merge of overlay into base, using name as
// the key. Existing entries are updated field by field with the keys the
// overlay names: a list the overlay sets replaces the base list, and an
// explicit empty list (`env_files: []`) clears it, while an absent key leaves
// the base value alone. install.command has no such form: an empty string is
// indistinguishable from an absent key, so it keeps the base command. New
// entries are appended. Source is set on all changed or new entries.
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
		if over.Detect.Files != nil {
			e.Detect.Files = over.Detect.Files
		}
		if over.Install.Command != "" {
			e.Install.Command = over.Install.Command
		}
		if over.EnvFiles != nil {
			e.EnvFiles = over.EnvFiles
		}
		e.Source = overlaySource
		result[i] = e
	}

	return result
}

// mergeConfigs merges overlay on top of base, returning a new Config. The
// worktree_files list is replaced entirely when the overlay names the key, so
// `worktree_files: []` clears the base list.
func mergeConfigs(base, overlay *Config, overlaySource string) *Config {
	result := &Config{
		Ecosystems:    mergeEcosystems(base.Ecosystems, overlay.Ecosystems, overlaySource),
		WorktreeFiles: base.WorktreeFiles,
	}
	if overlay.WorktreeFiles != nil {
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
			return fmt.Errorf("ecosystem %q: detect.files must not be empty; list at least one file, or set enabled: false to turn the ecosystem off", e.Name)
		}
	}
	return nil
}

// globalConfigPath returns the path to the global sentei config file:
// $XDG_CONFIG_HOME/sentei/config.yaml, defaulting to ~/.config. It returns ""
// when neither variable names an absolute directory (the XDG specification
// says to ignore a relative value), so the path is never resolved against the
// working directory.
func globalConfigPath() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if !filepath.IsAbs(base) {
		home, _ := os.UserHomeDir()
		if !filepath.IsAbs(home) {
			return ""
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "sentei", "config.yaml")
}

// repoRoot returns the directory whose .sentei.yaml belongs to the repository
// at repoPath. For a common dir named .git or .bare, or one a .git pointer
// file sits beside, that is its parent: the top of a regular working tree, or
// the container of a bare layout. Any other common dir is a plain bare
// repository and is its own root, reached from itself or from one of its
// worktrees, because its parent may hold unrelated repositories. Outside a
// repository the root is repoPath.
func repoRoot(runner git.CommandRunner, repoPath string) string {
	commonDir, err := git.CommonDir(runner, repoPath)
	if err != nil {
		return repoPath
	}
	parent := filepath.Dir(commonDir)
	switch filepath.Base(commonDir) {
	case ".git", ".bare":
		return parent
	}
	if pointer, err := os.Lstat(filepath.Join(parent, ".git")); err == nil && pointer.Mode().IsRegular() {
		return parent
	}
	return commonDir
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

	if globalPath := globalConfigPath(); globalPath != "" {
		globalCfg, err := loadFile(globalPath)
		if err != nil {
			return nil, fmt.Errorf("loading global config: %w", err)
		}
		if globalCfg != nil {
			cfg = mergeConfigs(cfg, globalCfg, "global")
		}
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
