package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/bonboncinnabon/sentei/internal/config"
	"github.com/bonboncinnabon/sentei/internal/ecosystem"
)

func RunEcosystems(args []string) {
	repoPath := "."
	if len(args) > 0 {
		repoPath = args[0]
	}

	cfg, err := config.LoadConfig(repoPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	reg := ecosystem.NewRegistry(cfg.Ecosystems)
	all := reg.All()

	fmt.Printf("Ecosystems (%d registered)\n\n", len(all))
	fmt.Printf("  %-14s %-24s %-30s %-10s %s\n", "NAME", "DETECT FILES", "INSTALL", "SOURCE", "STATUS")

	for _, eco := range all {
		files := strings.Join(eco.Config.Detect.Files, ", ")
		status := "enabled"
		if !eco.Config.IsEnabled() {
			status = "disabled"
		}
		source := eco.Config.Source
		fmt.Printf("  %-14s %-24s %-30s %-10s %s\n",
			eco.Name,
			truncate(files, 22),
			truncate(eco.Config.Install.Command, 28),
			source,
			status,
		)
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
