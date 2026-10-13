package ecosystem

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"github.com/bonboncinnabon/sentei/internal/config"
)

// Detect returns the enabled ecosystems whose detection files are present in
// dir. The order of ecosystems is kept, so the caller controls priority.
func Detect(ecosystems []config.EcosystemConfig, dir string) []config.EcosystemConfig {
	var detected []config.EcosystemConfig
	for _, eco := range ecosystems {
		if eco.IsEnabled() && matchesAny(dir, eco.Detect.Files) {
			detected = append(detected, eco)
		}
	}
	return detected
}

// matchesAny reports whether any of the given glob patterns matches at least
// one regular file inside dir. Patterns are matched within dir as a file
// system root, so glob characters in dir itself ("fix[1]") are literal and a
// pattern cannot reach outside dir.
func matchesAny(dir string, patterns []string) bool {
	root := os.DirFS(dir)
	for _, pattern := range patterns {
		matches, err := fs.Glob(root, path.Clean(filepath.ToSlash(pattern)))
		if err != nil {
			continue
		}
		for _, match := range matches {
			info, err := fs.Stat(root, match)
			if err == nil && !info.IsDir() {
				return true
			}
		}
	}
	return false
}
