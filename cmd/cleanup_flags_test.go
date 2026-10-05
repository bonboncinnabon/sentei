package cmd

import (
	"strings"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/cleanup"
)

func TestParseCleanupFlags_ModeOnly(t *testing.T) {
	opts, err := ParseCleanupFlags([]string{"--mode", "safe"})
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
	opts, err := ParseCleanupFlags([]string{"--mode", "aggressive"})
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
	opts, err := ParseCleanupFlags([]string{"--mode", "aggressive", "--force"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !opts.Force {
		t.Error("expected Force=true when --force is passed")
	}
}

func TestParseCleanupFlags_DryRun(t *testing.T) {
	opts, err := ParseCleanupFlags([]string{"--mode", "safe", "--dry-run"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !opts.DryRun {
		t.Error("expected DryRun=true")
	}
}

func TestParseCleanupFlags_InvalidMode(t *testing.T) {
	_, err := ParseCleanupFlags([]string{"--mode", "invalid"})
	if err == nil {
		t.Fatal("expected error for invalid mode")
	}
	if !strings.Contains(err.Error(), "invalid value for --mode") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestParseCleanupFlags_NoFlags(t *testing.T) {
	opts, err := ParseCleanupFlags([]string{})
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

func TestCleanupCLICommand_SafeMode(t *testing.T) {
	opts := &cleanup.Options{Mode: cleanup.ModeSafe}
	cmd := CleanupCLICommand(opts)
	if !strings.Contains(cmd, "sentei cleanup") {
		t.Errorf("expected 'sentei cleanup' prefix, got %s", cmd)
	}
	if !strings.Contains(cmd, "--mode safe") {
		t.Errorf("expected '--mode safe', got %s", cmd)
	}
}

func TestCleanupCLICommand_AggressiveDryRun(t *testing.T) {
	opts := &cleanup.Options{Mode: cleanup.ModeAggressive, DryRun: true}
	cmd := CleanupCLICommand(opts)
	if !strings.Contains(cmd, "--mode aggressive") {
		t.Errorf("expected '--mode aggressive', got %s", cmd)
	}
	if !strings.Contains(cmd, "--dry-run") {
		t.Errorf("expected '--dry-run', got %s", cmd)
	}
}
