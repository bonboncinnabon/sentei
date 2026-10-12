package cmd

import (
	"strings"
	"testing"
)

func TestParseCreateFlags_BranchOnly(t *testing.T) {
	opts, err := ParseCreateFlags([]string{"--branch", "feature/foo"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.Branch != "feature/foo" {
		t.Errorf("expected branch=feature/foo, got %s", opts.Branch)
	}
	if opts.Base != "" {
		t.Errorf("expected empty base, got %s", opts.Base)
	}
	if opts.MergeBase {
		t.Error("expected MergeBase=false by default")
	}
	if opts.CopyEnv {
		t.Error("expected CopyEnv=false by default")
	}
	if len(opts.Ecosystems) != 0 {
		t.Errorf("expected no ecosystems, got %v", opts.Ecosystems)
	}
}

func TestParseCreateFlags_BranchAndBase(t *testing.T) {
	opts, err := ParseCreateFlags([]string{"--branch", "feature/bar", "--base", "develop"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.Branch != "feature/bar" {
		t.Errorf("expected branch=feature/bar, got %s", opts.Branch)
	}
	if opts.Base != "develop" {
		t.Errorf("expected base=develop, got %s", opts.Base)
	}
}

func TestParseCreateFlags_AllFlags(t *testing.T) {
	opts, err := ParseCreateFlags([]string{
		"--branch", "feat/all",
		"--base", "main",
		"--ecosystems", "go,pnpm",
		"--merge-base",
		"--copy-env",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.Branch != "feat/all" {
		t.Errorf("expected branch=feat/all, got %s", opts.Branch)
	}
	if opts.Base != "main" {
		t.Errorf("expected base=main, got %s", opts.Base)
	}
	if !opts.MergeBase {
		t.Error("expected MergeBase=true")
	}
	if !opts.CopyEnv {
		t.Error("expected CopyEnv=true")
	}
	if len(opts.Ecosystems) != 2 || opts.Ecosystems[0] != "go" || opts.Ecosystems[1] != "pnpm" {
		t.Errorf("expected ecosystems=[go pnpm], got %v", opts.Ecosystems)
	}
}

func TestParseCreateFlags_NoFlags(t *testing.T) {
	opts, err := ParseCreateFlags([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.Branch != "" {
		t.Errorf("expected empty branch, got %s", opts.Branch)
	}
	if opts.Base != "" {
		t.Errorf("expected empty base, got %s", opts.Base)
	}
}

func TestParseCreateFlags_EcosystemsSingle(t *testing.T) {
	opts, err := ParseCreateFlags([]string{"--branch", "x", "--ecosystems", "go"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(opts.Ecosystems) != 1 || opts.Ecosystems[0] != "go" {
		t.Errorf("expected ecosystems=[go], got %v", opts.Ecosystems)
	}
}

func TestValidateCreateForNonInteractive_MissingBranch(t *testing.T) {
	opts := &CreateOptions{Base: "main"}
	err := ValidateCreateForNonInteractive(opts)
	if err == nil {
		t.Fatal("expected error for missing branch")
	}
	if !strings.Contains(err.Error(), "--branch") {
		t.Errorf("expected error to mention --branch, got: %v", err)
	}
}

func TestValidateCreateForNonInteractive_MissingBase(t *testing.T) {
	opts := &CreateOptions{Branch: "feat/x"}
	err := ValidateCreateForNonInteractive(opts)
	if err == nil {
		t.Fatal("expected error for missing base")
	}
	if !strings.Contains(err.Error(), "--base") {
		t.Errorf("expected error to mention --base, got: %v", err)
	}
}

func TestValidateCreateForNonInteractive_Valid(t *testing.T) {
	opts := &CreateOptions{Branch: "feat/x", Base: "main"}
	err := ValidateCreateForNonInteractive(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
