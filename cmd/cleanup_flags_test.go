package cmd

import (
	"strings"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/cleanup"
)

func TestParseCleanupFlags_ModeOnly(t *testing.T) {
	opts, _, err := ParseCleanupFlags([]string{"--mode", "safe"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.Mode != cleanup.ModeSafe {
		t.Errorf("expected mode=safe, got %s", opts.Mode)
	}
	if opts.DryRun {
		t.Error("expected DryRun=false")
	}
}

func TestParseCleanupFlags_Aggressive(t *testing.T) {
	opts, _, err := ParseCleanupFlags([]string{"--mode", "aggressive"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.Mode != cleanup.ModeAggressive {
		t.Errorf("expected mode=aggressive, got %s", opts.Mode)
	}
}

func TestParseCleanupFlags_Force(t *testing.T) {
	// The dispatcher re-injects the global --force here ("one --force does both"),
	// so it must reach cleanup.Options.Force.
	opts, _, err := ParseCleanupFlags([]string{"--mode", "aggressive", "--force"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !opts.Force {
		t.Error("expected Force=true when --force is passed")
	}
}

func TestParseCleanupFlags_DryRun(t *testing.T) {
	opts, _, err := ParseCleanupFlags([]string{"--mode", "safe", "--dry-run"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !opts.DryRun {
		t.Error("expected DryRun=true")
	}
}

func TestParseCleanupFlags_InvalidMode(t *testing.T) {
	_, _, err := ParseCleanupFlags([]string{"--mode", "invalid"})
	if err == nil {
		t.Fatal("expected error for invalid mode")
	}
	if !strings.Contains(err.Error(), "invalid value for --mode") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestParseCleanupFlags_NoFlags(t *testing.T) {
	opts, _, err := ParseCleanupFlags([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.Mode != "" {
		t.Errorf("expected empty mode, got %s", opts.Mode)
	}
}

func TestValidateCleanupForNonInteractive_MissingMode(t *testing.T) {
	opts := &cleanup.Options{}
	err := ValidateCleanupForNonInteractive(opts)
	if err == nil {
		t.Fatal("expected error for missing mode")
	}
	if !strings.Contains(err.Error(), "missing required flag: --mode") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidateCleanupForNonInteractive_Valid(t *testing.T) {
	opts := &cleanup.Options{Mode: cleanup.ModeSafe}
	err := ValidateCleanupForNonInteractive(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseCleanupFlags_RepoPath(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"no args", nil, "."},
		{"flags only", []string{"--mode", "safe", "--dry-run"}, "."},
		{"positional only", []string{"/some/repo"}, "/some/repo"},
		{"flags then positional", []string{"--mode", "safe", "/some/repo"}, "/some/repo"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, got, err := ParseCleanupFlags(tt.args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("repo path for %v = %q, want %q", tt.args, got, tt.want)
			}
		})
	}
}
