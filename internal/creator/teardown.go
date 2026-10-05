package creator

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/bonboncinnabon/sentei/internal/integration"
)

// ArtifactInfo describes the artifact directories found for an integration.
type ArtifactInfo struct {
	IntegrationName string
	Dirs            []string
}

// ScanArtifacts checks which integrations have artifact directories present in wtPath.
func ScanArtifacts(wtPath string, integrations []integration.Integration) []ArtifactInfo {
	var found []ArtifactInfo

	for _, integ := range integrations {
		var presentDirs []string
		for _, dir := range integ.Teardown.Dirs {
			cleanDir := strings.TrimSuffix(dir, "/")
			fullPath := filepath.Join(wtPath, cleanDir)
			if info, err := os.Stat(fullPath); err == nil && info.IsDir() {
				presentDirs = append(presentDirs, dir)
			}
		}
		if len(presentDirs) > 0 {
			found = append(found, ArtifactInfo{
				IntegrationName: integ.Name,
				Dirs:            presentDirs,
			})
		}
	}

	return found
}
