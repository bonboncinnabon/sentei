package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/bonboncinnabon/sentei/cmd"
	"github.com/bonboncinnabon/sentei/internal/cleanup"
	"github.com/bonboncinnabon/sentei/internal/cli"
	"github.com/bonboncinnabon/sentei/internal/config"
	"github.com/bonboncinnabon/sentei/internal/dryrun"
	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/playground"
	"github.com/bonboncinnabon/sentei/internal/repo"
	"github.com/bonboncinnabon/sentei/internal/tui"
	"github.com/bonboncinnabon/sentei/internal/worktree"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func buildRegistry() *cli.Registry {
	r := cli.NewRegistry()
	r.Register(&cli.Command{Name: "ecosystems", Type: cli.Output, RunCLI: cmd.RunEcosystems})
	r.Register(&cli.Command{Name: "clone", Type: cli.Decision, RunCLI: cmd.RunClone})
	r.Register(&cli.Command{Name: "create", Type: cli.Decision, RunCLI: cmd.RunCreate})
	r.Register(&cli.Command{Name: "cleanup", Type: cli.Decision, RunCLI: cmd.RunCleanup})
	r.Register(&cli.Command{Name: "migrate", Type: cli.Decision, RunCLI: cmd.RunMigrate})
	r.Register(&cli.Command{Name: "remove", Type: cli.Decision, RunCLI: cmd.RunRemove})
	return r
}

func main() {
	registry := buildRegistry()
	err := run(registry, os.Args[1:])
	// -h/--help is success: the flag package has already printed usage.
	if err == nil || errors.Is(err, flag.ErrHelp) {
		return
	}
	fmt.Fprintln(os.Stderr, "Error:", err)
	if cli.IsUnknownCommand(err) {
		fmt.Fprint(os.Stderr, registry.UsageString())
	}
	os.Exit(1)
}

// run dispatches the command line. The code below it returns its error instead
// of calling os.Exit: an exit would skip deferred cleanup, such as removing the
// playground's temp repo.
func run(registry *cli.Registry, args []string) error {
	result, err := registry.Dispatch(args)
	if err != nil {
		return err
	}

	// No command: handle the root flags and launch the TUI.
	if result.Command == nil {
		return runRoot(result.Args)
	}

	if result.Command.Type == cli.Decision && !result.NonInteractive && !result.Yes {
		return launchInteractiveDecision(*result)
	}

	args = result.Args
	// cleanup and remove have their own --force semantics, and the global flag
	// extractor consumed the flag. Re-inject it so it reaches the command.
	// Prepend it: the flag parser stops at the first positional (the repo
	// path), so appending would be silently ignored.
	if result.Force && (result.Command.Name == "cleanup" || result.Command.Name == "remove") {
		args = append([]string{"--force"}, args...)
	}
	return result.Command.RunCLI(args)
}

// decisionOpts holds one interactive decision command's parsed flags and the
// repository it targets. Exactly one of the option pointers is set.
type decisionOpts struct {
	repoPath string
	cleanup  *cleanup.Options
	create   *cmd.CreateOptions
	clone    *cmd.CloneOptions
	remove   *cmd.RemoveOptions
	migrate  *cmd.MigrateOptions
}

// parseDecision parses a decision command's flags before any repository
// detection, so the positional repo path decides which repository the TUI
// opens. Detecting from the current directory first made
// `sentei cleanup /path/repoB` act on the repository in the cwd.
func parseDecision(name string, args []string) (decisionOpts, error) {
	parsed := decisionOpts{repoPath: "."}
	var err error
	switch name {
	case "cleanup":
		parsed.cleanup, parsed.repoPath, err = cmd.ParseCleanupFlags(args)
	case "create":
		if parsed.create, err = cmd.ParseCreateFlags(args); err == nil && parsed.create.RepoPath != "" {
			parsed.repoPath = parsed.create.RepoPath
		}
	case "clone":
		parsed.clone, err = cmd.ParseCloneFlags(args)
	case "remove":
		if parsed.remove, err = cmd.ParseRemoveFlags(args); err == nil && parsed.remove.RepoPath != "" {
			parsed.repoPath = parsed.remove.RepoPath
		}
	case "migrate":
		if parsed.migrate, err = cmd.ParseMigrateFlags(args); err == nil && parsed.migrate.RepoPath != "" {
			parsed.repoPath = parsed.migrate.RepoPath
		}
	}
	if err != nil {
		return decisionOpts{}, err
	}
	absPath, err := filepath.Abs(parsed.repoPath)
	if err != nil {
		return decisionOpts{}, fmt.Errorf("resolving repository path %q: %w", parsed.repoPath, err)
	}
	parsed.repoPath = absPath
	return parsed, nil
}

func launchInteractiveDecision(result cli.DispatchResult) error {
	parsed, err := parseDecision(result.Command.Name, result.Args)
	if err != nil {
		return err
	}

	runner := &git.GitRunner{}
	context, repoPath := detectRepo(runner, parsed.repoPath)
	model, err := newMenuModel(runner, repoPath, context)
	if err != nil {
		return err
	}

	switch result.Command.Name {
	case "cleanup":
		opts := parsed.cleanup
		if result.Force {
			opts.Force = true // global --force also force-deletes unmerged branches
		}
		if opts.Mode == "" {
			opts.Mode = cleanup.ModeSafe
		}
		model.SetCleanupOpts(opts)

	case "create":
		opts := parsed.create
		model.SetCreateOpts(&tui.CreateOpts{
			Branch:     opts.Branch,
			Base:       opts.Base,
			Ecosystems: opts.Ecosystems,
			MergeBase:  opts.MergeBase,
			CopyEnv:    opts.CopyEnv,
			RepoPath:   opts.RepoPath,
		})

	case "clone":
		opts := parsed.clone
		model.SetCloneOpts(&tui.CloneOpts{
			URL:  opts.URL,
			Name: opts.Name,
		})

	case "remove":
		opts := parsed.remove
		if opts.HasFilter() {
			worktrees, err := git.ListWorktrees(runner, repoPath)
			if err != nil {
				return err
			}
			worktrees = worktree.EnrichWorktrees(runner, worktrees, worktree.DefaultEnrichConcurrency)

			defaultBranch := git.DetectDefaultBranch(runner, repoPath)
			if err := cmd.CheckNamedBranches(worktrees, opts.Branches, defaultBranch); err != nil {
				return err
			}
			var isMerged cmd.MergedChecker
			if opts.Merged {
				isMerged = cmd.CheckMerged(runner, repoPath, defaultBranch)
			}
			filtered := cmd.ResolveFilters(worktrees, opts, defaultBranch, isMerged)

			var paths []string
			for _, wt := range filtered {
				paths = append(paths, wt.Path)
			}
			model.SetRemoveOpts(tui.RemovePreSelection{
				Paths:       paths,
				FilterLabel: cmd.FormatFilterLabel(opts),
				CLICommand:  cmd.RemoveCLICommand(opts),
			})
		}

	case "migrate":
		model.SetMigrateOpts(&tui.MigrateOpts{RepoPath: repoPath})
	}

	return runTUI(model)
}

// detectRepo reports what kind of repository holds path and, for a bare
// layout, the layout's root: path may be the bare directory, the container of
// a .bare layout, or any worktree.
func detectRepo(runner git.CommandRunner, path string) (repo.RepoContext, string) {
	context := repo.DetectContext(runner, path)
	if context == repo.ContextBareRepo {
		path = repo.ResolveBareRoot(runner, path)
	}
	return context, path
}

// newMenuModel builds the menu TUI for a detected repository. A config that
// fails to load stops the launch: a warning would vanish behind the alternate
// screen, and running without the config silently skips what it sets up.
func newMenuModel(runner git.CommandRunner, repoPath string, context repo.RepoContext, opts ...tui.ModelOption) (tui.Model, error) {
	var cfg *config.Config
	if context == repo.ContextBareRepo {
		var err error
		cfg, err = config.LoadConfig(repoPath, config.WithRunner(runner))
		if err != nil {
			return tui.Model{}, fmt.Errorf("loading config: %w; fix the file and run sentei again", err)
		}
	}
	return tui.NewMenuModel(runner, &git.DefaultShellRunner{}, repoPath, cfg, context, opts...), nil
}

// runTUI runs the model to completion and warns when the user quit in the
// middle of a flow that changes the repository.
func runTUI(model tui.Model) error {
	final, err := tea.NewProgram(model).Run()
	if err != nil {
		return fmt.Errorf("failed to run TUI: %w", err)
	}
	if tm, ok := final.(tui.Model); ok {
		if op := tm.InterruptedFlow(); op != "" {
			fmt.Fprintln(os.Stderr, "Warning: quit during "+op+"; check repository state before retrying")
		}
	}
	return nil
}

func runRoot(args []string) error {
	// ExitOnError exits inside Parse, before anything that needs cleanup.
	fs := flag.NewFlagSet("sentei", flag.ExitOnError)
	versionFlag := fs.Bool("version", false, "Print version and exit")
	playgroundFlag := fs.Bool("playground", false, "Launch with a temporary test repo")
	dryRunFlag := fs.Bool("dry-run", false, "Print worktree summary and exit (no interactive TUI)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *versionFlag {
		fmt.Printf("sentei %s (%s, %s)\n", version, commit, date)
		return nil
	}
	// The flag package stops at the first positional, so a flag written after
	// the repo path (`sentei <path> --dry-run`) would be silently dropped.
	if fs.NArg() > 1 {
		return fmt.Errorf("unexpected argument %q after repository path %q: flags must come before the path", fs.Arg(1), fs.Arg(0))
	}

	repoPath := "."
	if fs.NArg() > 0 {
		repoPath = fs.Arg(0)
	}
	if absPath, err := filepath.Abs(repoPath); err == nil {
		repoPath = absPath
	}

	if *playgroundFlag {
		var cleanup func()
		var err error
		repoPath, cleanup, err = playground.Setup()
		if err != nil {
			return fmt.Errorf("failed to set up playground: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Playground repo: %s\n", repoPath)
		defer cleanup()
	}

	runner := &git.GitRunner{}
	context, repoPath := detectRepo(runner, repoPath)

	if *dryRunFlag {
		if context != repo.ContextBareRepo {
			return errors.New("--dry-run requires a bare repository")
		}

		worktrees, err := git.ListWorktrees(runner, repoPath)
		if err != nil {
			return err
		}

		worktrees = worktree.EnrichWorktrees(runner, worktrees, worktree.DefaultEnrichConcurrency)

		var filtered []git.Worktree
		for _, wt := range worktrees {
			if !wt.IsBare {
				filtered = append(filtered, wt)
			}
		}

		if len(filtered) == 0 {
			fmt.Println("No worktrees found (only the main working tree exists).")
			return nil
		}

		return dryrun.Print(filtered, git.DetectDefaultBranch(runner, repoPath), os.Stdout)
	}

	var menuOpts []tui.ModelOption
	if *playgroundFlag {
		menuOpts = append(menuOpts, tui.WithMinProgressDuration(1500*time.Millisecond))
	}
	model, err := newMenuModel(runner, repoPath, context, menuOpts...)
	if err != nil {
		return err
	}
	return runTUI(model)
}
