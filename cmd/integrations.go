package cmd

import (
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/bonboncinnabon/sentei/internal/integration"
)

func RunIntegrations() {
	all := integration.All()

	fmt.Printf("Integrations (%d registered)\n\n", len(all))
	fmt.Printf("  %-22s %-12s %s\n", "NAME", "STATUS", "DESCRIPTION")

	for _, integ := range all {
		status := detectStatus(integ)
		fmt.Printf("  %-22s %-12s %s\n", integ.Name, status, integ.Description)
		fmt.Printf("  %-22s %-12s %s\n", "", "", integ.URL)
	}
}

func detectStatus(integ integration.Integration) string {
	if integ.Detect.Command != "" {
		parts := strings.Fields(integ.Detect.Command)
		if len(parts) > 0 {
			cmd := exec.Command(parts[0], parts[1:]...)
			cmd.Stdout = io.Discard
			cmd.Stderr = io.Discard
			if err := cmd.Run(); err == nil {
				return "installed"
			}
		}
	}
	if integ.Detect.BinaryName != "" {
		if _, err := exec.LookPath(integ.Detect.BinaryName); err == nil {
			return "installed"
		}
	}
	return "not found"
}
