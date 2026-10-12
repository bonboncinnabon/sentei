package tui

import (
	"strings"
	"testing"
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
		{"installer dump", "uv tool install x\n" + strings.Repeat("+ pkg==1\n", 80) + "error: executable already exists: tool", 3, "… 80 more lines — ? for full output"},
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

func TestErrorPeek_SanitizesChildProcessOutput(t *testing.T) {
	raw := "tool init && tool index: \x1b[?25l\x1b[36m⠋\x1b[0m Indexing...\r\x1b[2K⠙ Indexing...\r\nIndexing failed: No module named 'sentence_transformers'"
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
