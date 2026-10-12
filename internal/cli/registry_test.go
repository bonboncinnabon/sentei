package cli

import (
	"errors"
	"testing"
)

func newTestRegistry() *Registry {
	r := NewRegistry()

	r.Register(&Command{
		Name: "ecosystems",
		Type: Output,
		RunCLI: func(args []string) error {
			return nil
		},
	})

	r.Register(&Command{
		Name: "cleanup",
		Type: Decision,
		RunCLI: func(args []string) error {
			return nil
		},
	})

	r.Register(&Command{
		Name: "create",
		Type: Decision,
		RunCLI: func(args []string) error {
			return nil
		},
	})

	return r
}

func TestDispatch_Root(t *testing.T) {
	r := newTestRegistry()

	result, err := r.Dispatch(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsRoot {
		t.Error("expected IsRoot=true for nil args")
	}

	result, err = r.Dispatch([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsRoot {
		t.Error("expected IsRoot=true for empty args")
	}
}

func TestDispatch_RootWithFlags(t *testing.T) {
	r := newTestRegistry()

	result, err := r.Dispatch([]string{"--version"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsRoot {
		t.Error("expected IsRoot=true when first arg is a flag")
	}
	if len(result.Args) != 1 || result.Args[0] != "--version" {
		t.Errorf("expected args=[--version], got %v", result.Args)
	}
}

func TestDispatch_KnownOutputCommand(t *testing.T) {
	r := newTestRegistry()

	result, err := r.Dispatch([]string{"ecosystems"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsRoot {
		t.Error("expected IsRoot=false for known command")
	}
	if result.Command.Name != "ecosystems" {
		t.Errorf("expected command=ecosystems, got %s", result.Command.Name)
	}
	if result.Command.Type != Output {
		t.Error("expected Output type")
	}
}

func TestDispatch_KnownDecisionCommand(t *testing.T) {
	r := newTestRegistry()

	result, err := r.Dispatch([]string{"create", "--branch", "foo"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Command.Name != "create" {
		t.Errorf("expected command=create, got %s", result.Command.Name)
	}
	if result.Command.Type != Decision {
		t.Error("expected Decision type")
	}
	if len(result.Args) != 2 || result.Args[0] != "--branch" || result.Args[1] != "foo" {
		t.Errorf("expected remaining args=[--branch foo], got %v", result.Args)
	}
}

func TestDispatch_UnknownCommand(t *testing.T) {
	r := newTestRegistry()

	_, err := r.Dispatch([]string{"foobar"})
	if !errors.Is(err, ErrUnknownCommand) {
		t.Errorf("expected ErrUnknownCommand, got %v", err)
	}
}

func TestDispatch_NonInteractiveFlag(t *testing.T) {
	r := newTestRegistry()

	result, err := r.Dispatch([]string{"create", "--non-interactive", "--branch", "foo"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.NonInteractive {
		t.Error("expected NonInteractive=true")
	}
	if len(result.Args) != 2 || result.Args[0] != "--branch" || result.Args[1] != "foo" {
		t.Errorf("--non-interactive should be stripped from args, got %v", result.Args)
	}
}

func TestDispatch_ForceFlag(t *testing.T) {
	r := newTestRegistry()

	result, err := r.Dispatch([]string{"cleanup", "--force", "--non-interactive"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Force {
		t.Error("expected Force=true")
	}
	if !result.NonInteractive {
		t.Error("expected NonInteractive=true")
	}
}

// --non-interactive used to demand --force for cleanup, remove and migrate.
// That same --force switches off each command's own safety (the at-risk gate,
// branch -d versus -D), so the gate taught scripts and agents to discard work.
func TestDispatch_NonInteractiveDoesNotRequireForce(t *testing.T) {
	r := newTestRegistry()

	for _, command := range []string{"cleanup", "create"} {
		result, err := r.Dispatch([]string{command, "--non-interactive"})
		if err != nil {
			t.Fatalf("%s --non-interactive: unexpected error: %v", command, err)
		}
		if !result.NonInteractive || result.Force {
			t.Errorf("%s: NonInteractive=%v Force=%v, want true and false", command, result.NonInteractive, result.Force)
		}
	}
}

// `sentei <repo path>` is the documented way to open another repository and
// what the TUI relaunches itself with; it used to fail as an unknown command.
func TestDispatch_DirectoryIsRootWithRepoPath(t *testing.T) {
	r := newTestRegistry()
	dir := t.TempDir()

	result, err := r.Dispatch([]string{dir, "--dry-run"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsRoot || result.Command != nil {
		t.Errorf("a directory argument must dispatch to the root command, got %+v", result)
	}
	if len(result.Args) != 2 || result.Args[0] != dir {
		t.Errorf("args = %v, want the path kept for the root command", result.Args)
	}
}

func TestLookup(t *testing.T) {
	r := newTestRegistry()

	if cmd := r.Lookup("ecosystems"); cmd == nil {
		t.Error("expected to find ecosystems")
	}
	if cmd := r.Lookup("nope"); cmd != nil {
		t.Error("expected nil for unknown command")
	}
}

func TestCommandNames(t *testing.T) {
	r := newTestRegistry()

	names := r.CommandNames()
	if len(names) != 3 {
		t.Errorf("expected 3 names, got %d", len(names))
	}

	nameSet := make(map[string]bool)
	for _, n := range names {
		nameSet[n] = true
	}
	for _, want := range []string{"ecosystems", "cleanup", "create"} {
		if !nameSet[want] {
			t.Errorf("expected %q in command names", want)
		}
	}
}
