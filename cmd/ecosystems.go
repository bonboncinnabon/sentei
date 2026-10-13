package cmd

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/bonboncinnabon/sentei/internal/config"
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

	fmt.Printf("Ecosystems (%d registered)\n\n", len(cfg.Ecosystems))
	fmt.Printf("  %-14s %-24s %-30s %-10s %s\n", "NAME", "DETECT FILES", "INSTALL", "SOURCE", "STATUS")

	for _, eco := range cfg.Ecosystems {
		files := strings.Join(eco.Detect.Files, ", ")
		status := "enabled"
		if !eco.IsEnabled() {
			status = "disabled"
		}
		fmt.Printf("  %-14s %-24s %-30s %-10s %s\n",
			eco.Name,
			truncate(files, 22),
			truncate(eco.Install.Command, 28),
			eco.Source,
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
