package cmd

import (
	"flag"
	"fmt"
)

// MigrateOptions holds the parsed flags for the migrate command.
type MigrateOptions struct {
	DeleteBackup bool
	RepoPath     string
}

// ParseMigrateFlags parses migrate-specific flags and returns MigrateOptions.
// The repo path is taken as a positional argument.
func ParseMigrateFlags(args []string) (*MigrateOptions, error) {
	fs := flag.NewFlagSet("migrate", flag.ContinueOnError)
	deleteBackup := fs.Bool("delete-backup", false, "Delete the backup after successful migration")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	opts := &MigrateOptions{
		DeleteBackup: *deleteBackup,
	}

	repoPath, err := repoPathArg(fs)
	if err != nil {
		return nil, err
	}
	opts.RepoPath = repoPath

	return opts, nil
}

// ValidateMigrateForNonInteractive checks that all required arguments are present
// for non-interactive execution.
func ValidateMigrateForNonInteractive(opts *MigrateOptions) error {
	if opts.RepoPath == "" {
		return fmt.Errorf("repo path required (positional argument)")
	}
	return nil
}

// MigrateCLICommand generates the equivalent CLI command string from options.
func MigrateCLICommand(opts *MigrateOptions) string {
	cmd := "sentei migrate"
	if opts.DeleteBackup {
		cmd += " --delete-backup"
	}
	if opts.RepoPath != "" {
		cmd += " " + opts.RepoPath
	}
	return cmd
}
