package cmd

import (
	"flag"
	"fmt"

	"github.com/bonboncinnabon/sentei/internal/cleanup"
	"github.com/bonboncinnabon/sentei/internal/cli"
)

// ParseCleanupFlags parses cleanup-specific flags and returns CleanupOptions.
// Returns an error if validation fails (e.g., invalid mode).
func ParseCleanupFlags(args []string) (*cleanup.Options, error) {
	fs := flag.NewFlagSet("cleanup", flag.ContinueOnError)
	mode := fs.String("mode", "", "Cleanup mode: safe or aggressive")
	force := fs.Bool("force", false, "Force-delete unmerged branches (aggressive mode)")
	dryRun := fs.Bool("dry-run", false, "Show what would be done without making changes")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	opts := &cleanup.Options{
		Force:  *force,
		DryRun: *dryRun,
	}

	if *mode != "" {
		m := cleanup.Mode(*mode)
		if m != cleanup.ModeSafe && m != cleanup.ModeAggressive {
			return nil, fmt.Errorf("invalid value for --mode: must be 'safe' or 'aggressive'")
		}
		opts.Mode = m
	}

	return opts, nil
}

// ParseCleanupRepoPath extracts the positional repo path from cleanup args.
func ParseCleanupRepoPath(args []string) string {
	fs := flag.NewFlagSet("cleanup", flag.ContinueOnError)
	fs.String("mode", "", "")
	fs.Bool("force", false, "")
	fs.Bool("dry-run", false, "")
	_ = fs.Parse(args)
	if fs.NArg() > 0 {
		return fs.Arg(0)
	}
	return "."
}

// ValidateCleanupForNonInteractive checks that all required flags are present
// for non-interactive execution.
func ValidateCleanupForNonInteractive(opts *cleanup.Options) error {
	if opts.Mode == "" {
		return fmt.Errorf("missing required flag: --mode (safe|aggressive)")
	}
	return nil
}

// CleanupCLICommand generates the equivalent CLI command string from options.
func CleanupCLICommand(opts *cleanup.Options) string {
	flags := make(map[string]string)
	if opts.Mode != "" {
		flags["mode"] = string(opts.Mode)
	}
	if opts.DryRun {
		flags["dry-run"] = "true"
	}
	return cli.BuildFlagString("sentei cleanup", flags)
}
