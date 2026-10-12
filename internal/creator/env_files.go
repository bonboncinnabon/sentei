package creator

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bonboncinnabon/sentei/internal/fileutil"
)

// uniqueEnvFiles returns the env files to copy: the explicit list plus those
// declared by the ecosystems selected for install.
func uniqueEnvFiles(opts Options) []string {
	seen := map[string]bool{}
	var files []string
	add := func(name string) {
		name = filepath.Clean(strings.TrimSpace(name))
		if name == "." || filepath.IsAbs(name) || strings.HasPrefix(name, ".."+string(filepath.Separator)) || seen[name] {
			return
		}
		seen[name] = true
		files = append(files, name)
	}
	for _, name := range opts.EnvFiles {
		add(name)
	}
	for _, ecosystem := range opts.Ecosystems {
		for _, name := range ecosystem.EnvFiles {
			add(name)
		}
	}
	sort.Strings(files)
	return files
}

func copyPreparedEnvFiles(source, destination string, files []string) (string, error) {
	var copied []string
	for _, name := range files {
		if _, err := os.Stat(filepath.Join(source, name)); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return "", fmt.Errorf("inspecting %s: %w", name, err)
		}
		if err := fileutil.CopyFile(filepath.Join(source, name), filepath.Join(destination, name)); err != nil {
			return "", fmt.Errorf("copying %s: %w", name, err)
		}
		copied = append(copied, name)
	}
	if len(copied) == 0 {
		return "no source files found", nil
	}
	return strings.Join(copied, ", "), nil
}
