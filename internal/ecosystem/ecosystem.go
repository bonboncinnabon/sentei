package ecosystem

import (
	"os"
	"path/filepath"

	"github.com/bonboncinnabon/sentei/internal/config"
)

// Ecosystem is a detected or registered ecosystem instance.
type Ecosystem struct {
	Name   string
	Config config.EcosystemConfig
}

// Registry holds a list of ecosystem configurations and provides detection.
// The order of ecosystems in the list is preserved, giving callers control
// over priority.
type Registry struct {
	ecosystems []config.EcosystemConfig
}

// NewRegistry creates a Registry from the given ecosystem configs.
func NewRegistry(cfg []config.EcosystemConfig) *Registry {
	return &Registry{ecosystems: cfg}
}

// Detect returns all enabled ecosystems whose detection files are present in
// dir. Order matches the registry order, so the caller controls priority.
func (r *Registry) Detect(dir string) ([]Ecosystem, error) {
	var detected []Ecosystem
	for _, eco := range r.ecosystems {
		if !eco.IsEnabled() {
			continue
		}
		if matchesAny(dir, eco.Detect.Files) {
			detected = append(detected, Ecosystem{Name: eco.Name, Config: eco})
		}
	}
	return detected, nil
}

// All returns every ecosystem in the registry regardless of enabled state.
func (r *Registry) All() []Ecosystem {
	result := make([]Ecosystem, len(r.ecosystems))
	for i, eco := range r.ecosystems {
		result[i] = Ecosystem{Name: eco.Name, Config: eco}
	}
	return result
}

// matchesAny reports whether any of the given glob patterns matches at least
// one regular file inside dir.
func matchesAny(dir string, patterns []string) bool {
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(dir, pattern))
		if err != nil {
			continue
		}
		for _, match := range matches {
			info, err := os.Stat(match)
			if err == nil && !info.IsDir() {
				return true
			}
		}
	}
	return false
}
