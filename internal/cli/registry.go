package cli

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
)

// CommandType classifies commands as output (read-only, always CLI) or decision
// (requires user choices, defaults to TUI).
type CommandType int

const (
	Output   CommandType = iota // Read-only commands that print to stdout and exit.
	Decision                    // Commands that require choices; default to TUI.
)

// Command defines a registered CLI command with its type and handlers.
type Command struct {
	Name string
	Type CommandType

	// RunCLI executes the command in non-interactive mode.
	// For output commands, this is the only execution path.
	// For decision commands, this runs when --non-interactive is provided.
	RunCLI func(args []string) error
}

// Registry holds registered commands and dispatches based on os.Args.
type Registry struct {
	commands map[string]*Command
}

// NewRegistry creates an empty command registry.
func NewRegistry() *Registry {
	return &Registry{commands: make(map[string]*Command)}
}

// Register adds a command to the registry.
func (r *Registry) Register(cmd *Command) {
	r.commands[cmd.Name] = cmd
}

// DispatchResult tells the caller what action to take after dispatch.
type DispatchResult struct {
	// Command is the matched command. It is nil when no command was named:
	// the root flags apply and the TUI menu opens.
	Command *Command

	// Args are the remaining arguments after the command name.
	Args []string

	// NonInteractive is true when --non-interactive was provided: run the CLI
	// path instead of the TUI. Each command's own safeties stay in effect.
	NonInteractive bool

	// Force is true when --force was provided. It is the per-command override
	// (remove at-risk worktrees, force-delete unmerged branches) and nothing
	// else: requiring it for every non-interactive run made scripts and
	// agents switch those safeties off just to get past the gate.
	Force bool
	// Yes is true when --yes/-y was provided. It is an alias for
	// --non-interactive.
	Yes bool
}

var ErrUnknownCommand = errors.New("unknown command")

// IsUnknownCommand returns true if the error wraps ErrUnknownCommand.
func IsUnknownCommand(err error) bool {
	return errors.Is(err, ErrUnknownCommand)
}

// Dispatch parses the command name from args and returns a DispatchResult.
// It extracts --non-interactive, --yes and --force from the args before returning.
func (r *Registry) Dispatch(args []string) (*DispatchResult, error) {
	if len(args) == 0 {
		return &DispatchResult{}, nil
	}

	name := args[0]

	// Check if the first arg looks like a flag (not a command).
	if strings.HasPrefix(name, "-") {
		return &DispatchResult{Args: args}, nil
	}

	cmd, ok := r.commands[name]
	if !ok {
		// A directory is the `sentei <repo path>` form: the documented way to
		// open another repository, and what the TUI relaunches itself with
		// after create, clone and migrate.
		if info, err := os.Stat(name); err == nil && info.IsDir() {
			return &DispatchResult{Args: args}, nil
		}
		return nil, fmt.Errorf("%w: %s", ErrUnknownCommand, name)
	}

	remaining := args[1:]
	nonInteractive, force, yes, remaining := extractGlobalFlags(remaining)

	return &DispatchResult{
		Command:        cmd,
		Args:           remaining,
		NonInteractive: nonInteractive,
		Force:          force,
		Yes:            yes,
	}, nil
}

// CommandNames returns all registered command names.
func (r *Registry) CommandNames() []string {
	names := make([]string, 0, len(r.commands))
	for name := range r.commands {
		names = append(names, name)
	}
	return names
}

// UsageString returns a formatted help string listing all commands.
func (r *Registry) UsageString() string {
	var b strings.Builder
	b.WriteString("Usage: sentei [command] [options]\n\n")
	b.WriteString("Commands:\n")

	names := r.CommandNames()
	sort.Strings(names)
	for _, name := range names {
		cmd := r.commands[name]
		label := "output"
		if cmd.Type == Decision {
			label = "interactive"
		}
		fmt.Fprintf(&b, "  %-14s (%s)\n", cmd.Name, label)
	}

	b.WriteString("\nGlobal flags:\n")
	b.WriteString("  --non-interactive  run without the TUI; command safeties stay active\n")
	b.WriteString("  --yes, -y          same as --non-interactive\n")
	b.WriteString("  --force            override a command's safeties: remove at-risk worktrees, force-delete unmerged branches\n")
	b.WriteString("\nRun 'sentei <command> --help' for command-specific options.\n")
	return b.String()
}

// BuildFlagString constructs a CLI command string from a base command and flags.
// Boolean flags (value == "true") are rendered as --flag (no value).
// Flags are sorted by key for deterministic output.
func BuildFlagString(base string, flags map[string]string) string {
	flagKeys := make([]string, 0, len(flags))
	for k := range flags {
		flagKeys = append(flagKeys, k)
	}
	sort.Strings(flagKeys)

	result := base
	for _, k := range flagKeys {
		v := flags[k]
		if v == "true" {
			result += " --" + k
		} else {
			result += " --" + k + " " + v
		}
	}
	return result
}

// extractGlobalFlags pulls --non-interactive and --force from the args slice,
// returning the flag values and the remaining args. This uses a simple scan
// rather than flag.FlagSet to avoid conflicting with command-specific flags.
func extractGlobalFlags(args []string) (nonInteractive bool, force bool, yes bool, remaining []string) {
	for _, arg := range args {
		switch arg {
		case "--non-interactive":
			nonInteractive = true
		case "--force":
			force = true
		case "--yes", "-y":
			yes = true
		default:
			remaining = append(remaining, arg)
		}
	}
	return
}
