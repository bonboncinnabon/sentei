package cmd

import (
	"flag"
	"fmt"
	"strings"

	"github.com/bonboncinnabon/sentei/internal/cli"
)

// CreateOptions holds parsed flags for the create command.
type CreateOptions struct {
	Branch     string
	Base       string
	Ecosystems []string
	MergeBase  bool
	CopyEnv    bool
	RepoPath   string // positional arg: path to bare repo
}

// ParseCreateFlags parses create-specific flags and returns CreateOptions.
func ParseCreateFlags(args []string) (*CreateOptions, error) {
	fs := flag.NewFlagSet("create", flag.ContinueOnError)
	branch := fs.String("branch", "", "Branch name for the new worktree")
	base := fs.String("base", "", "Base branch to create from")
	ecosystems := fs.String("ecosystems", "", "Comma-separated list of ecosystems to install")
	mergeBase := fs.Bool("merge-base", false, "Merge base branch into the new worktree")
	copyEnv := fs.Bool("copy-env", false, "Copy environment files from source worktree")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	opts := &CreateOptions{
		Branch:    *branch,
		Base:      *base,
		MergeBase: *mergeBase,
		CopyEnv:   *copyEnv,
	}

	if *ecosystems != "" {
		opts.Ecosystems = strings.Split(*ecosystems, ",")
	}

	if fs.NArg() > 0 {
		opts.RepoPath = fs.Arg(0)
	}

	return opts, nil
}

// ValidateCreateForNonInteractive checks that all required flags are present
// for non-interactive execution.
func ValidateCreateForNonInteractive(opts *CreateOptions) error {
	if opts.Branch == "" {
		return fmt.Errorf("missing required flag: --branch")
	}
	if opts.Base == "" {
		return fmt.Errorf("missing required flag: --base")
	}
	return nil
}

// CreateCLICommand generates the equivalent CLI command string from options.
func CreateCLICommand(opts *CreateOptions) string {
	flags := make(map[string]string)
	if opts.Branch != "" {
		flags["branch"] = opts.Branch
	}
	if opts.Base != "" {
		flags["base"] = opts.Base
	}
	if len(opts.Ecosystems) > 0 {
		flags["ecosystems"] = strings.Join(opts.Ecosystems, ",")
	}
	if opts.MergeBase {
		flags["merge-base"] = "true"
	}
	if opts.CopyEnv {
		flags["copy-env"] = "true"
	}
	cmd := cli.BuildFlagString("sentei create", flags)
	if opts.RepoPath != "" {
		cmd += " " + opts.RepoPath
	}
	return cmd
}
