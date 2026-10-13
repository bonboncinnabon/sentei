package cmd

import (
	"flag"
	"fmt"

	"github.com/bonboncinnabon/sentei/internal/cleanup"
)

// ParseCleanupFlags parses the cleanup flags and the optional positional
// repository path, which defaults to the current directory. It returns an
// error for an invalid mode or an argument after the path.
func ParseCleanupFlags(args []string) (*cleanup.Options, string, error) {
	fs := flag.NewFlagSet("cleanup", flag.ContinueOnError)
	mode := fs.String("mode", "", "Cleanup mode: safe or aggressive")
	force := fs.Bool("force", false, "Force-delete unmerged branches (aggressive mode)")
	dryRun := fs.Bool("dry-run", false, "Show what would be done without making changes")

	if err := fs.Parse(args); err != nil {
		return nil, "", err
	}
	repoPath, err := repoPathArg(fs)
	if err != nil {
		return nil, "", err
	}
	if repoPath == "" {
		repoPath = "."
	}

	opts := &cleanup.Options{
		Force:  *force,
		DryRun: *dryRun,
	}

	if *mode != "" {
		m := cleanup.Mode(*mode)
		if m != cleanup.ModeSafe && m != cleanup.ModeAggressive {
			return nil, "", fmt.Errorf("invalid value for --mode: must be 'safe' or 'aggressive'")
		}
		opts.Mode = m
	}

	return opts, repoPath, nil
}

// ValidateCleanupForNonInteractive checks that all required flags are present
// for non-interactive execution.
func ValidateCleanupForNonInteractive(opts *cleanup.Options) error {
	if opts.Mode == "" {
		return fmt.Errorf("missing required flag: --mode (safe|aggressive)")
	}
	return nil
}
