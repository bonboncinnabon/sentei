package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/bonboncinnabon/sentei/internal/git"
	"github.com/bonboncinnabon/sentei/internal/repo"
)

func TestPortal_FitsNarrowAndEmergencyTerminals(t *testing.T) {
	for _, width := range []int{20, 32} {
		for height := 1; height <= 11; height++ {
			var p DetailPortal
			p = p.SetSize(width, height)
			p = p.Open(portalDetails, "界 details with a long title", strings.Repeat("long 界🙂 detail line\n", 30))
			background := strings.TrimSuffix(strings.Repeat(strings.Repeat(" ", width)+"\n", height), "\n")
			view := p.View(background)
			if got := lipgloss.Height(view); got > height {
				t.Fatalf("%dx%d portal height=%d:\n%s", width, height, got, stripANSI(view))
			}
			for row, line := range strings.Split(view, "\n") {
				if got := lipgloss.Width(line); got > width {
					t.Fatalf("%dx%d portal row %d width=%d:\n%s", width, height, row+1, got, stripANSI(view))
				}
			}
		}
	}
}

func portalTestModel() Model {
	m := NewMenuModel(nil, nil, "/repo", nil, repo.ContextBareRepo)
	m.width, m.height = 100, 26
	m.portal = m.portal.SetSize(100, 32)
	return m
}

func keyF1() tea.KeyPressMsg  { return tea.KeyPressMsg{Code: tea.KeyF1} }
func keyEsc() tea.KeyPressMsg { return tea.KeyPressMsg{Code: tea.KeyEsc} }

func TestPortal_OpenCloseLifecycle(t *testing.T) {
	var p DetailPortal
	p = p.SetSize(80, 24)
	if p.Visible() {
		t.Fatal("portal must start hidden")
	}

	p = p.Open(portalHelp, "Help — Test", "line1\nline2")
	if !p.Visible() {
		t.Fatal("portal must be visible after Open")
	}
	if p.viewport.YOffset() != 0 {
		t.Error("scroll position must reset on open")
	}

	p = p.Close()
	if p.Visible() {
		t.Error("portal must hide after Close")
	}
}

func TestPortal_ScrollResetsOnReopen(t *testing.T) {
	var p DetailPortal
	p = p.SetSize(80, 12)
	p = p.Open(portalHelp, "T", strings.Repeat("line\n", 50))
	p.viewport.ScrollDown(10)
	if p.viewport.YOffset() == 0 {
		t.Fatal("precondition: viewport scrolled")
	}

	p = p.Open(portalHelp, "T", strings.Repeat("line\n", 50))
	if p.viewport.YOffset() != 0 {
		t.Errorf("scroll must reset on reopen, got offset %d", p.viewport.YOffset())
	}
}

func TestPortal_RefreshPreservesOrClampsScroll(t *testing.T) {
	var p DetailPortal
	p = p.SetSize(40, 10)
	p = p.Open(portalDetails, "Details", strings.Repeat("line\n", 50))
	p.viewport.ScrollDown(8)
	offset := p.viewport.YOffset()

	p = p.Refresh("Updated details", strings.Repeat("updated\n", 50))
	if got := p.viewport.YOffset(); got != offset {
		t.Fatalf("same-size refresh moved offset from %d to %d", offset, got)
	}

	p = p.Refresh("Updated details", "one line")
	if got := p.viewport.YOffset(); got != 0 {
		t.Fatalf("short refresh did not clamp offset, got %d", got)
	}
}

func TestPortal_SizingStandardTerminal(t *testing.T) {
	var p DetailPortal
	p = p.SetSize(80, 24)
	if got := p.contentWidth(); got != 80-2*portalMargin-4 {
		t.Errorf("content width = %d, want %d", got, 80-2*portalMargin-4)
	}
	if got := p.contentHeight(); got != 24-2*portalMargin-5 {
		t.Errorf("content height = %d, want %d", got, 24-2*portalMargin-5)
	}
}

func TestPortal_ResizeWhileOpen(t *testing.T) {
	m := portalTestModel()
	updated, _ := m.Update(keyF1())
	m = updated.(Model)
	if !m.portal.Visible() {
		t.Fatal("precondition: portal open")
	}

	updated, _ = m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	m = updated.(Model)
	if m.portal.viewport.Width() != m.portal.contentWidth() {
		t.Errorf("viewport width %d not refit to %d", m.portal.viewport.Width(), m.portal.contentWidth())
	}
}

func TestPortal_PasteDoesNotEditHiddenInput(t *testing.T) {
	m := createBranchModel()
	updated, _ := m.Update(keyF1())
	m = updated.(Model)
	if !m.portal.Visible() {
		t.Fatal("precondition: help portal must be open")
	}

	updated, _ = m.Update(tea.PasteMsg{Content: "hidden-change"})
	m = updated.(Model)

	if !m.portal.Visible() {
		t.Fatal("paste must not close the portal")
	}
	if got := m.create.branchInput.Value(); got != "" {
		t.Errorf("hidden branch input changed to %q", got)
	}
}

func TestPortal_ChromeTitleHintsAndScrollIndicator(t *testing.T) {
	var p DetailPortal
	p = p.SetSize(80, 14)
	p = p.Open(portalDetails, "Worktree details", strings.Repeat("content line\n", 40))

	view := stripANSI(p.View("background"))
	for _, want := range []string{"Worktree details", "esc close · j/k scroll", "↓"} {
		if !strings.Contains(view, want) {
			t.Errorf("portal chrome missing %q:\n%s", want, view)
		}
	}

	p = p.Open(portalDetails, "Short", "one line")
	view = stripANSI(p.View("background"))
	if strings.Contains(view, "↓") {
		t.Error("scroll indicator must hide when content fits")
	}
}

func TestPortal_CompositesOverBackground(t *testing.T) {
	m := portalTestModel()
	updated, _ := m.Update(keyF1())
	m = updated.(Model)

	view := stripANSI(m.View().Content)
	if !strings.Contains(view, "Help — Menu") {
		t.Errorf("expected portal content in composite view:\n%s", view)
	}
	// The canvas keeps the background's full height: menu lines outside the
	// box remain part of the output (per-cell visibility is covered by the
	// compositeOverlay unit tests).
	if got, bg := len(strings.Split(view, "\n")), len(strings.Split(m.viewContent(), "\n")); got < bg {
		t.Errorf("composite lost background rows: %d < %d", got, bg)
	}
}

func TestPortal_InterceptsNavigationPassesQuit(t *testing.T) {
	m := portalTestModel()
	cursorBefore := m.menuCursor
	updated, _ := m.Update(keyF1())
	m = updated.(Model)

	// j scrolls the portal, never the menu.
	updated, _ = m.Update(keyRune('j'))
	m = updated.(Model)
	if m.menuCursor != cursorBefore {
		t.Error("navigation keys must not reach the background view")
	}
	if !m.portal.Visible() {
		t.Error("portal must stay open on scroll keys")
	}

	// q quits even with the portal open.
	_, cmd := m.Update(keyRune('q'))
	if cmd == nil {
		t.Fatal("expected quit Cmd")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("q must quit through the portal")
	}
}

func TestPortal_EscCloses(t *testing.T) {
	m := portalTestModel()
	updated, _ := m.Update(keyF1())
	m = updated.(Model)

	updated, _ = m.Update(keyEsc())
	m = updated.(Model)
	if m.portal.Visible() {
		t.Error("esc must close the portal")
	}
	if m.view != menuView {
		t.Error("background view must be unchanged after close")
	}
}

func TestHelp_F1TogglesAndIsContextual(t *testing.T) {
	m := portalTestModel()

	updated, _ := m.Update(keyF1())
	m = updated.(Model)
	if !m.portal.Visible() || m.portal.title != "Help — Menu" {
		t.Fatalf("expected menu help, got visible=%v title=%q", m.portal.Visible(), m.portal.title)
	}

	updated, _ = m.Update(keyF1())
	m = updated.(Model)
	if m.portal.Visible() {
		t.Error("F1 must toggle help closed")
	}

	// Different view, different content.
	m.view = listView
	updated, _ = m.Update(keyF1())
	m = updated.(Model)
	if m.portal.title != "Help — Worktree List" {
		t.Errorf("expected contextual title, got %q", m.portal.title)
	}
}

func TestHelp_AllViewsProduceContent(t *testing.T) {
	m := portalTestModel()
	views := []viewState{
		menuView, listView, confirmView, progressView, summaryView,
		createBranchView, createOptionsView, createProgressView, createSummaryView,
		repoNameView, repoOptionsView, repoProgressView, repoSummaryView,
		cloneInputView, migrateConfirmView, migrateProgressView, migrateSummaryView,
		migrateNextView, integrationListView, integrationProgressView,
		integrationSummaryView, migrateIntegrationsView, cleanupConfirmView,
		cleanupResultView, createConfirmView, cloneConfirmView,
	}
	for _, v := range views {
		m.view = v
		title, content := m.helpContent()
		if !strings.HasPrefix(title, "Help — ") {
			t.Errorf("view %d: bad help title %q", v, title)
		}
		if !strings.Contains(stripANSI(content), "q / ctrl+c") {
			t.Errorf("view %d: help missing global section:\n%s", v, content)
		}
	}
}

func TestDetails_QuestionMarkOnListView(t *testing.T) {
	wts := []git.Worktree{{
		Path:              "/work/feature-a",
		Branch:            "refs/heads/feature/a",
		LastCommitSubject: "Add feature A",
	}}
	m := NewModel(wts, nil, "/repo")
	m.width, m.height = 100, 26
	m.portal = m.portal.SetSize(100, 32)
	m.view = listView

	updated, _ := m.Update(keyRune('?'))
	m = updated.(Model)
	if !m.portal.Visible() || m.portal.title != "Worktree details" {
		t.Fatalf("expected details portal, got visible=%v title=%q", m.portal.Visible(), m.portal.title)
	}

	view := stripANSI(m.View().Content)
	for _, want := range []string{"feature/a", "/work/feature-a", "Add feature A"} {
		if !strings.Contains(view, want) {
			t.Errorf("details missing %q:\n%s", want, view)
		}
	}

	// ? toggles closed.
	updated, _ = m.Update(keyRune('?'))
	m = updated.(Model)
	if m.portal.Visible() {
		t.Error("? must toggle details closed")
	}
}

func TestDetails_NoOpOnViewWithoutDetails(t *testing.T) {
	m := portalTestModel() // menu view has no detail content
	updated, _ := m.Update(keyRune('?'))
	m = updated.(Model)
	if m.portal.Visible() {
		t.Error("? must be a no-op on views without details")
	}
}

func TestDetails_HelpAndDetailsSwitch(t *testing.T) {
	wts := []git.Worktree{{Path: "/work/a", Branch: "refs/heads/a"}}
	m := NewModel(wts, nil, "/repo")
	m.width, m.height = 100, 26
	m.portal = m.portal.SetSize(100, 32)
	m.view = listView

	// Open help, then ? switches to details.
	updated, _ := m.Update(keyF1())
	m = updated.(Model)
	updated, _ = m.Update(keyRune('?'))
	m = updated.(Model)
	if m.portal.title != "Worktree details" {
		t.Errorf("expected switch to details, got %q", m.portal.title)
	}

	// F1 switches back to help.
	updated, _ = m.Update(keyF1())
	m = updated.(Model)
	if m.portal.title != "Help — Worktree List" {
		t.Errorf("expected switch back to help, got %q", m.portal.title)
	}
}

func TestDetails_IntegrationListKeepsOwnInfoCard(t *testing.T) {
	m := portalTestModel()
	m.view = integrationListView
	m.integ.integrations = nil // empty: info key is a no-op there too

	updated, _ := m.Update(keyRune('?'))
	m = updated.(Model)
	if m.portal.Visible() {
		t.Error("? on the integration list must fall through to the view's own handling")
	}
}

func TestPortal_ScrollHintsOnlyWhenScrollable(t *testing.T) {
	var p DetailPortal
	p = p.SetSize(80, 30)
	p = p.Open(portalDetails, "Short", "one line")
	if view := stripAnsi(p.View("")); strings.Contains(view, "j/k scroll") || strings.Contains(view, "↓ more") {
		t.Errorf("fitting content must not offer scroll keys:\n%s", view)
	}

	long := strings.Repeat("line\n", 80)
	p = p.Open(portalDetails, "Long", long)
	if view := stripAnsi(p.View("")); !strings.Contains(view, "j/k scroll") {
		t.Errorf("scrollable content must offer scroll keys:\n%s", view)
	}
}
