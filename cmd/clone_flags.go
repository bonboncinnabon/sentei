package cmd

import (
	"flag"
	"fmt"

	"github.com/bonboncinnabon/sentei/internal/cli"
)

// CloneOptions holds the parsed flags for the clone command.
type CloneOptions struct {
	URL  string
	Name string
}

// ParseCloneFlags parses clone-specific flags and returns CloneOptions.
func ParseCloneFlags(args []string) (*CloneOptions, error) {
	fs := flag.NewFlagSet("clone", flag.ContinueOnError)
	url := fs.String("url", "", "Git repository URL to clone")
	name := fs.String("name", "", "Directory name for the cloned repo (derived from URL if empty)")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	return &CloneOptions{
		URL:  *url,
		Name: *name,
	}, nil
}

// ValidateCloneForNonInteractive checks that all required flags are present
// for non-interactive execution.
func ValidateCloneForNonInteractive(opts *CloneOptions) error {
	if opts.URL == "" {
		return fmt.Errorf("missing required flag: --url")
	}
	return nil
}

// CloneCLICommand generates the equivalent CLI command string from options.
func CloneCLICommand(opts *CloneOptions) string {
	flags := make(map[string]string)
	if opts.URL != "" {
		flags["url"] = opts.URL
	}
	if opts.Name != "" {
		flags["name"] = opts.Name
	}
	return cli.BuildFlagString("sentei clone", flags)
}
