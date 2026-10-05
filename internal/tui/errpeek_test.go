package tui

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/progress"
)

func TestErrorPeekLines_Bounds(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want int
		last string
	}{
		{"single line", "exit status 1", 1, "exit status 1"},
		{"two lines", "ran thing\nerror: nope", 2, "error: nope"},
		{"installer dump", "uv tool install x\n" + strings.Repeat("+ pkg==1\n", 80) + "error: executable already exists: ccc", 3, "… 80 more lines — ? for full output"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := errorPeekLines(tc.in, 120)
			if len(got) != tc.want {
				t.Fatalf("len = %d, want %d (%v)", len(got), tc.want, got)
			}
			if got[len(got)-1] != tc.last {
				t.Errorf("last = %q, want %q", got[len(got)-1], tc.last)
			}
		})
	}
}

func TestErrorPeekLines_TruncatesToWidth(t *testing.T) {
	long := strings.Repeat("x", 300)
	for _, line := range errorPeekLines(long+"\nmid\n"+long, 40) {
		if n := len([]rune(line)); n > 40 {
			t.Errorf("line exceeds width: %d runes", n)
		}
	}
}

func TestErrorPeekLast_OneLine(t *testing.T) {
	got := errorPeekLast("cmd\nnoise\nerror: the actual reason", 60)
	if got != "error: the actual reason" {
		t.Errorf("got %q", got)
	}
}

func TestRenderIntegrationOutcomes_FailureStaysBounded(t *testing.T) {
	dump := "uv tool install cocoindex-code\n"
	for i := range 78 {
		dump += fmt.Sprintf("+ package-%d==1.0.0\n", i)
	}
	dump += "error: executable already exists: ccc"

	groups := []integrationWorktreeOutcomes{{
		worktree: "/wt/feature-wip",
		steps: []integrationStepOutcome{
			{step: "Setup code-review-graph", ev: progress.Event{Status: progress.StepDone}},
			{step: "Install cocoindex-code", ev: progress.Event{Status: progress.StepFailed, Error: errors.New(dump)}},
		},
	}}

	var b strings.Builder
	renderIntegrationOutcomes(&b, groups, 80)
	inline := stripAnsi(b.String())
	if got := len(strings.Split(strings.TrimRight(inline, "\n"), "\n")); got > 6 {
		t.Errorf("inline failure must stay bounded, got %d lines:\n%s", got, inline)
	}
	if !strings.Contains(inline, "error: executable already exists: ccc") {
		t.Errorf("the actual error line must surface:\n%s", inline)
	}
	if !strings.Contains(inline, "? for full output") {
		t.Errorf("elision marker missing:\n%s", inline)
	}

	var full strings.Builder
	renderIntegrationOutcomes(&full, groups, 0)
	if !strings.Contains(stripAnsi(full.String()), "+ package-42==1.0.0") {
		t.Error("portal mode must keep the full output")
	}
}

func TestApplySummary_FailureUnlocksDetailPortal(t *testing.T) {
	m := NewModel([]git.Worktree{}, nil, "/repo")
	m.width = 100
	m.integ.events = []progress.Event{
		{Phase: "/wt/a", Step: "Install x", Status: progress.StepFailed, Error: errors.New("line1\nline2\nerror: boom")},
	}
	title, content := m.integrationSummaryDetailContent()
	if title == "" || !strings.Contains(stripAnsi(content), "error: boom") {
		t.Errorf("failure output must be reachable in the portal, got title=%q", title)
	}
	if !strings.Contains(stripAnsi(m.viewIntegrationSummary()), "? details") {
		t.Error("footer must offer ? when a failure has output")
	}
}

func TestErrorPeek_SanitizesChildProcessOutput(t *testing.T) {
	raw := "ccc init && ccc index: \x1b[?25l\x1b[36m⠋\x1b[0m Indexing...\r\x1b[2K⠙ Indexing...\r\nIndexing failed: No module named 'sentence_transformers'"
	lines := errorPeekLines(raw, 100)
	for _, l := range lines {
		if strings.ContainsAny(l, "\r\x1b") {
			t.Errorf("peek line carries control codes: %q", l)
		}
	}
	// Peek shape: first line, last content line, elision marker.
	if lines[1] != "Indexing failed: No module named 'sentence_transformers'" {
		t.Errorf("content line = %q", lines[1])
	}
}
