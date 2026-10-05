package cmd

import (
	"strings"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/integration"
)

func TestDetectStatus(t *testing.T) {
	tests := []struct {
		name   string
		detect integration.DetectSpec
		want   string
	}{
		{"command succeeds", integration.DetectSpec{Command: "git --version"}, "installed"},
		{"command fails but binary exists", integration.DetectSpec{Command: "git --no-such-flag-zzz", BinaryName: "git"}, "installed"},
		{"binary exists", integration.DetectSpec{BinaryName: "git"}, "installed"},
		{"nothing detectable", integration.DetectSpec{Command: "no-such-binary-zzz --version", BinaryName: "no-such-binary-zzz"}, "not found"},
		{"empty spec", integration.DetectSpec{}, "not found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectStatus(integration.Integration{Detect: tt.detect})
			if got != tt.want {
				t.Errorf("detectStatus(%+v) = %q, want %q", tt.detect, got, tt.want)
			}
		})
	}
}

func TestRunIntegrations_ListsAllIntegrations(t *testing.T) {
	out := captureStdout(t, func() {
		RunIntegrations()
	})

	if !strings.Contains(out, "Integrations (") {
		t.Errorf("expected integrations header, got:\n%s", out)
	}
	for _, integ := range integration.All() {
		if !strings.Contains(out, integ.Name) {
			t.Errorf("expected integration %q to be listed, got:\n%s", integ.Name, out)
		}
		if !strings.Contains(out, integ.URL) {
			t.Errorf("expected URL %q to be listed, got:\n%s", integ.URL, out)
		}
	}
}
