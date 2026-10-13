package cmd

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/bonboncinnabon/sentei/internal/config"
	"github.com/bonboncinnabon/sentei/internal/ecosystem"
)

// RunEcosystems lists the ecosystems sentei can detect, with the configuration
// that applies to the given repository.
func RunEcosystems(args []string) error {
	fs := flag.NewFlagSet("ecosystems", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: sentei ecosystems [repo]\n\nList the ecosystems sentei can detect and how each one installs dependencies.")
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	repoPath, err := repoPathArg(fs)
	if err != nil {
		return err
	}
	if repoPath == "" {
		repoPath = "."
	}

	cfg, err := config.LoadConfig(repoPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
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
	return nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
