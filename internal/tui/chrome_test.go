package tui

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
	"charm.land/lipgloss/v2"

	"github.com/bonboncinnabon/sentei/internal/git"
)

func TestViewTitle_Standard(t *testing.T) {
	got := viewTitle("Removing worktrees")
	if !strings.Contains(got, "sentei ─ Removing worktrees") {
		t.Errorf("viewTitle = %q, want it to contain the prefixed title", got)
	}
	if !strings.HasPrefix(stripANSI(got), "  ") {
		t.Errorf("expected 2-space left padding, got %q", got)
	}
}

func TestViewTitle_Empty(t *testing.T) {
	got := stripANSI(viewTitle(""))
	if got != "  sentei ─" {
		t.Errorf("viewTitle(\"\") = %q, want %q", got, "  sentei ─")
	}
}

func TestViewSeparator_StandardWidth(t *testing.T) {
	got := stripANSI(viewSeparator(80))
	want := "  " + strings.Repeat("┄", 76)
	if got != want {
		t.Errorf("viewSeparator(80) = %d chars, want 2-space pad + 76 dashes", len([]rune(got)))
	}
}

func TestViewSeparator_NarrowWidthFallsBack(t *testing.T) {
	for _, w := range []int{4, 0} {
		got := stripANSI(viewSeparator(w))
		want := "  " + strings.Repeat("┄", 36)
		if got != want {
			t.Errorf("viewSeparator(%d) = %q, want 40-width fallback", w, got)
		}
	}
}

func TestViewFooter_Multiple(t *testing.T) {
	got := stripANSI(viewFooter(80, confirmationFooter))
	want := "  enter confirm · esc back · q quit"
	if got != want {
		t.Errorf("viewFooter = %q, want %q", got, want)
	}
}

func TestViewFooter_Single(t *testing.T) {
	got := stripANSI(viewFooter(80, quitOnlyFooter))
	if got != "  q quit" {
		t.Errorf("viewFooter = %q, want %q", got, "  q quit")
	}
}

func TestViewFooter_None(t *testing.T) {
	if got := viewFooter(80, nil); got != "" {
		t.Errorf("viewFooter(nil) = %q, want empty", got)
	}
}

func TestViewFooter_NarrowWidthTruncates(t *testing.T) {
	got := stripANSI(viewFooter(24, listFooter))
	if len([]rune(got)) > 24 {
		t.Errorf("footer wider than budget: %q (%d runes)", got, len([]rune(got)))
	}
	if !strings.Contains(got, "…") {
		t.Errorf("truncated footer must end with ellipsis, got %q", got)
	}
}

// Drift guard: every binding a view advertises in its footer must also appear
// in that view's help sections, keyed by the rendered key label.
func TestFooterBindingsAppearInSections(t *testing.T) {
	cases := []struct {
		name     string
		footer   []key.Binding
		sections []keySection
	}{
		{"menu", menuFooter, append(menuSections, helpGlobalSection)},
		{"list", listFooter, append(listSections, helpGlobalSection)},
		{"confirm", confirmFooter, confirmSections},
		{"options", optionsFooter, append(optionsSections, helpGlobalSection)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inSections := make(map[string]bool)
			for _, s := range tc.sections {
				for _, b := range s.bindings {
					inSections[b.Help().Key] = true
				}
			}
			// Combined presentation labels (j/k) and global quit are covered
			// by section variants; compare on the primary token.
			for _, b := range tc.footer {
				label := b.Help().Key
				found := inSections[label]
				if !found {
					for k := range inSections {
						if strings.Contains(k, label) || strings.Contains(label, k) {
							found = true
							break
						}
					}
				}
				if !found {
					t.Errorf("footer binding %q has no counterpart in help sections", label)
				}
			}
		})
	}
}

func TestTruncateWithEllipsis(t *testing.T) {
	cases := []struct {
		name  string
		in    string
		width int
		want  string
	}{
		{"fits unchanged", "short", 10, "short"},
		{"exact fit unchanged", "exact", 5, "exact"},
		{"long truncated", "/very/long/worktree/path", 10, "/very/lon…"},
		{"width one", "abc", 1, "…"},
		{"width zero", "abc", 0, ""},
		{"multibyte runes", "wörk/trée-päth", 6, "wörk/…"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := truncateWithEllipsis(tc.in, tc.width); got != tc.want {
				t.Errorf("truncateWithEllipsis(%q, %d) = %q, want %q", tc.in, tc.width, got, tc.want)
			}
		})
	}
}

// Truncation measures what the terminal draws. Counting runes cut a styled
// string inside its escape sequence and let wide runes through at twice the
// budget.
func TestTruncateWithEllipsis_MeasuresDisplayWidth(t *testing.T) {
	styled := styleWarning.Render("uncommitted changes") + ", " + styleStatusLocked.Render("locked: on a USB disk")
	got := truncateWithEllipsis(styled, 12)
	if plain := stripANSI(got); plain != "uncommitted…" {
		t.Errorf("styled text truncated to %q, want %q", plain, "uncommitted…")
	}
	if width := lipgloss.Width(got); width > 12 {
		t.Errorf("styled text is %d cells wide after truncating to 12", width)
	}

	wide := truncateWithEllipsis("界界界界界", 6)
	if wide != "界界…" || lipgloss.Width(wide) > 6 {
		t.Errorf("wide runes truncated to %q (%d cells), want %q within 6 cells", wide, lipgloss.Width(wide), "界界…")
	}
}

// The worktree details portal truncates a status that is already styled.
func TestWorktreeDetails_StyledStatusFitsThePortal(t *testing.T) {
	m := NewModel([]git.Worktree{{
		Path: "/work/a", Branch: "refs/heads/a",
		HasUncommittedChanges: true, HasUntrackedFiles: true, IsLocked: true,
		LockReason: "kept on an external disk that is not always mounted",
	}}, nil, "/repo")
	m.view = listView
	m.portal = m.portal.SetSize(50, 24)

	_, content := m.detailContent()
	for _, line := range strings.Split(content, "\n") {
		if width := lipgloss.Width(line); width > m.portal.contentWidth() {
			t.Errorf("detail line is %d cells, portal content is %d: %q", width, m.portal.contentWidth(), stripANSI(line))
		}
	}
	if !strings.Contains(stripANSI(content), "uncommitted changes") {
		t.Errorf("status row lost its text:\n%s", stripANSI(content))
	}
}

func TestViewStatLine_Standard(t *testing.T) {
	got := stripANSI(viewStatLine(WindowStats{Done: 10, Active: 3, Pending: 17, Showing: 6, Total: 30}, indicatorActiveFallback))
	want := "    ✦ 10 done  ✻ 3 active  · 17 pending  showing 6 of 30"
	if got != want {
		t.Errorf("viewStatLine = %q, want %q", got, want)
	}
}

func TestViewStatLine_WithFailures(t *testing.T) {
	got := stripANSI(viewStatLine(WindowStats{Done: 1, Failed: 2, Showing: 3, Total: 3}, indicatorActiveFallback))
	if !strings.Contains(got, "✗ 2 failed") {
		t.Errorf("expected failed legend, got %q", got)
	}
}

func TestViewStatLine_ZeroCountsOmitted(t *testing.T) {
	got := stripANSI(viewStatLine(WindowStats{Done: 5, Showing: 5, Total: 5}, indicatorActiveFallback))
	for _, absent := range []string{"active", "pending", "failed"} {
		if strings.Contains(got, absent) {
			t.Errorf("zero-count category %q should be omitted, got %q", absent, got)
		}
	}
}

func TestRenderProgressBar_Halfway(t *testing.T) {
	got := stripANSI(renderProgressBar(10, 20, 20))
	want := "  " + strings.Repeat("█", 10) + strings.Repeat("░", 10) + " 50%"
	if got != want {
		t.Errorf("renderProgressBar(10,20) = %q, want %q", got, want)
	}
}

func TestRenderProgressBar_DoneExceedsTotal_ClampsNoPanic(t *testing.T) {
	got := stripANSI(renderProgressBar(3, 1, 20))
	want := "  " + strings.Repeat("█", 20) + " 100%"
	if got != want {
		t.Errorf("renderProgressBar(3,1) = %q, want clamped 100%%", got)
	}
}

func TestRenderProgressBar_ZeroTotal(t *testing.T) {
	got := stripANSI(renderProgressBar(0, 0, 20))
	want := "  " + strings.Repeat("░", 20) + " 0%"
	if got != want {
		t.Errorf("renderProgressBar(0,0) = %q, want empty bar at 0%%", got)
	}
}

// stripANSI removes escape sequences so assertions hold regardless of the
// test terminal's color profile.
func stripANSI(s string) string {
	var b strings.Builder
	inEscape := false
	for _, r := range s {
		switch {
		case inEscape:
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEscape = false
			}
		case r == '\x1b':
			inEscape = true
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
