package cmd

import (
	"flag"
	"fmt"
)

// MigrateOptions holds the parsed flags for the migrate command.
type MigrateOptions struct {
	RepoPath string
}

// ParseMigrateFlags parses migrate-specific flags and returns MigrateOptions.
// The repo path is taken as a positional argument.
func ParseMigrateFlags(args []string) (*MigrateOptions, error) {
	fs := flag.NewFlagSet("migrate", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	repoPath, err := repoPathArg(fs)
	if err != nil {
		return nil, err
	}
	return &MigrateOptions{RepoPath: repoPath}, nil
}

// ValidateMigrateForNonInteractive checks that all required arguments are present
// for non-interactive execution.
func ValidateMigrateForNonInteractive(opts *MigrateOptions) error {
	if opts.RepoPath == "" {
		return fmt.Errorf("repo path required (positional argument)")
	}
	return nil
}
