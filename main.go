package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/log/v2"

	"github.com/bonboncinnabon/sentei/cmd"
	"github.com/bonboncinnabon/sentei/internal/cleanup"
	"github.com/bonboncinnabon/sentei/internal/cli"
	"github.com/bonboncinnabon/sentei/internal/config"
	"github.com/bonboncinnabon/sentei/internal/dryrun"
	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/integration"
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

	r.Register(&cli.Command{
		Name: "ecosystems",
		Type: cli.Output,
		RunCLI: func(args []string) error {
			cmd.RunEcosystems(args)
			return nil
		},
	})

	r.Register(&cli.Command{
		Name: "integrations",
		Type: cli.Output,
		RunCLI: func(args []string) error {
			cmd.RunIntegrations()
			return nil
		},
	})

	r.Register(&cli.Command{
		Name: "clone",
		Type: cli.Decision,
		RunCLI: func(args []string) error {
			return cmd.RunClone(args)
		},
	})

	r.Register(&cli.Command{
		Name: "create",
		Type: cli.Decision,
		RunCLI: func(args []string) error {
			return cmd.RunCreate(args)
		},
	})

	r.Register(&cli.Command{
		Name:        "cleanup",
		Type:        cli.Decision,
		Destructive: true,
		RunCLI: func(args []string) error {
			return cmd.RunCleanup(args)
		},
	})

	r.Register(&cli.Command{
		Name:        "migrate",
		Type:        cli.Decision,
		Destructive: true,
		RunCLI: func(args []string) error {
			return cmd.RunMigrate(args)
		},
	})

	r.Register(&cli.Command{
		Name:        "remove",
		Type:        cli.Decision,
		Destructive: true,
		RunCLI: func(args []string) error {
			return cmd.RunRemove(args)
		},
	})

	return r
}

// runCommand runs a CLI command, exiting 1 on a real error but treating a
// -h/--help request (flag.ErrHelp) as success — the flag package has already
// printed usage, so a "help requested" line and non-zero exit are wrong.
func runCommand(run func([]string) error, args []string) {
	if err := run(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		log.Error(err)
		os.Exit(1)
	}
}

// exitOnFlagError handles a flag-parse error: -h/--help (flag.ErrHelp) exits 0
// since usage was already printed; any other error exits 1 with a message.
func exitOnFlagError(err error) {
	if err == nil {
		return
	}
	if errors.Is(err, flag.ErrHelp) {
		os.Exit(0)
	}
	log.Error(err)
	os.Exit(1)
}

func main() {
	// CLI errors are immediate feedback, not a log stream: no timestamps.
	log.SetReportTimestamp(false)

	registry := buildRegistry()

	result, err := registry.Dispatch(os.Args[1:])
	if err != nil {
		log.Error(err)
		if cli.IsUnknownCommand(err) {
			fmt.Fprint(os.Stderr, registry.UsageString())
		}
		os.Exit(1)
	}

	// Dispatch to registered commands.
	if result.Command != nil {
		switch result.Command.Type {
		case cli.Output:
			runCommand(result.Command.RunCLI, result.Args)
			return

		case cli.Decision:
			if result.NonInteractive || result.Yes {
				args := result.Args
				// cleanup and remove have their own --force semantics that the
				// global flag extractor consumed. Re-inject it so one --force
				// both passes the destructive gate and reaches the command.
				// Prepend it: the flag parser stops at the first positional
				// (the repo path), so appending would be silently ignored.
				if result.Force && (result.Command.Name == "cleanup" || result.Command.Name == "remove") {
					args = append([]string{"--force"}, result.Args...)
				}
				runCommand(result.Command.RunCLI, args)
				return
			}
			// Interactive mode: parse flags and launch TUI at the appropriate view.
			launchInteractiveDecision(*result)
			return
		}
	}

	// Root: no command specified — handle global flags and launch TUI.
	runRoot(result.Args)
}

func launchInteractiveDecision(result cli.DispatchResult) {
	repoPath := "."
	if absPath, err := filepath.Abs(repoPath); err == nil {
		repoPath = absPath
	}

	runner := &git.GitRunner{}
	shell := &git.DefaultShellRunner{}

	context := repo.DetectContext(runner, repoPath)
	if context == repo.ContextBareRepo {
		repoPath = repo.ResolveBareRoot(runner, repoPath)
	}

	var cfg *config.Config
	if context == repo.ContextBareRepo {
		var err error
		cfg, err = config.LoadConfig(repoPath,
			config.WithRunner(runner),
			config.WithKnownIntegrations(integration.Names()),
		)
		if err != nil {
			log.Warn("failed to load config", "err", err)
		}
	}

	model := tui.NewMenuModel(runner, shell, repoPath, cfg, context)

	switch result.Command.Name {
	case "cleanup":
		opts, err := cmd.ParseCleanupFlags(result.Args)
		exitOnFlagError(err)
		if result.Force {
			opts.Force = true // global --force also force-deletes unmerged branches
		}
		if opts.Mode == "" {
			opts.Mode = cleanup.ModeSafe
		}
		model.SetCleanupOpts(opts)

	case "create":
		opts, err := cmd.ParseCreateFlags(result.Args)
		exitOnFlagError(err)
		model.SetCreateOpts(&tui.CreateOpts{
			Branch:     opts.Branch,
			Base:       opts.Base,
			Ecosystems: opts.Ecosystems,
			MergeBase:  opts.MergeBase,
			CopyEnv:    opts.CopyEnv,
			RepoPath:   opts.RepoPath,
		})

	case "clone":
		opts, err := cmd.ParseCloneFlags(result.Args)
		exitOnFlagError(err)
		model.SetCloneOpts(&tui.CloneOpts{
			URL:  opts.URL,
			Name: opts.Name,
		})

	case "remove":
		opts, err := cmd.ParseRemoveFlags(result.Args)
		exitOnFlagError(err)
		if opts.Merged || opts.All || opts.Stale > 0 {
			worktrees, err := git.ListWorktrees(runner, repoPath)
			if err != nil {
				log.Error(err)
				os.Exit(1)
			}
			worktrees = worktree.EnrichWorktrees(runner, worktrees, worktree.DefaultEnrichConcurrency)

			defaultBranch := git.DetectDefaultBranch(runner, repoPath)
			var isMerged cmd.MergedChecker
			if opts.Merged {
				isMerged = cmd.CheckMerged(runner, repoPath, defaultBranch)
			}
			filtered := cmd.ResolveFilters(worktrees, opts, nil, defaultBranch, isMerged)

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
		opts, err := cmd.ParseMigrateFlags(result.Args)
		exitOnFlagError(err)
		model.SetMigrateOpts(&tui.MigrateOpts{
			DeleteBackup: opts.DeleteBackup,
			RepoPath:     opts.RepoPath,
		})
	}

	p := tea.NewProgram(model)
	final, err := p.Run()
	if err != nil {
		log.Error("failed to run TUI", "err", err)
		os.Exit(1)
	}
	if tm, ok := final.(tui.Model); ok {
		if op := tm.InterruptedFlow(); op != "" {
			log.Warn("quit during " + op + "; check repository state before retrying")
		}
	}
}

func runRoot(args []string) {
	fs := flag.NewFlagSet("sentei", flag.ExitOnError)
	versionFlag := fs.Bool("version", false, "Print version and exit")
	playgroundFlag := fs.Bool("playground", false, "Launch with a temporary test repo")
	dryRunFlag := fs.Bool("dry-run", false, "Print worktree summary and exit (no interactive TUI)")
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	if *versionFlag {
		fmt.Printf("sentei %s (%s, %s)\n", version, commit, date)
		os.Exit(0)
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
			log.Error("failed to set up playground", "err", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Playground repo: %s\n", repoPath)
		defer cleanup()
	}

	runner := &git.GitRunner{}
	shell := &git.DefaultShellRunner{}

	context := repo.DetectContext(runner, repoPath)
	if context == repo.ContextBareRepo {
		repoPath = repo.ResolveBareRoot(runner, repoPath)
	}

	if *dryRunFlag {
		if context != repo.ContextBareRepo {
			log.Error("--dry-run requires a bare repository")
			os.Exit(1)
		}

		worktrees, err := git.ListWorktrees(runner, repoPath)
		if err != nil {
			log.Error(err)
			os.Exit(1)
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
			os.Exit(0)
		}

		if err := dryrun.Print(filtered, os.Stdout); err != nil {
			log.Error(err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	var cfg *config.Config
	if context == repo.ContextBareRepo {
		var err error
		cfg, err = config.LoadConfig(repoPath,
			config.WithRunner(runner),
			config.WithKnownIntegrations(integration.Names()),
		)
		if err != nil {
			log.Warn("failed to load config", "err", err)
		}
	}

	var menuOpts []tui.ModelOption
	if *playgroundFlag {
		menuOpts = append(menuOpts, tui.WithMinProgressDuration(1500*time.Millisecond))
	}
	model := tui.NewMenuModel(runner, shell, repoPath, cfg, context, menuOpts...)
	p := tea.NewProgram(model)

	final, err := p.Run()
	if err != nil {
		log.Error("failed to run TUI", "err", err)
		os.Exit(1)
	}
	if tm, ok := final.(tui.Model); ok {
		if op := tm.InterruptedFlow(); op != "" {
			log.Warn("quit during " + op + "; check repository state before retrying")
		}
	}
}
