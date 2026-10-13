package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// portalTrigger records which key opened the portal so the same key toggles
// it closed while the other key can switch the content.
type portalTrigger int

const (
	portalClosed  portalTrigger = iota
	portalHelp                  // opened via F1
	portalDetails               // opened via ?
)

// portalMargin is the gap, in cells, between the portal box and each
// terminal edge.
const portalMargin = 2

// DetailPortal is a scrollable read-only overlay composited over the active
// view: the shared home for contextual details (?) and global help (F1).
// It is a sub-model on Model, not a standalone tea.Model.
type DetailPortal struct {
	trigger      portalTrigger
	title        string
	viewport     viewport.Model
	contentLines int
	width        int // terminal width
	height       int // terminal height
}

func (p DetailPortal) Visible() bool { return p.trigger != portalClosed }

// Open shows the portal with the given content, resetting scroll to the top.
// The box hugs short content instead of filling the terminal.
func (p DetailPortal) Open(trigger portalTrigger, title, content string) DetailPortal {
	p.trigger = trigger
	p.title = title
	p.contentLines = strings.Count(content, "\n") + 1
	p.viewport = viewport.New(viewport.WithWidth(p.contentWidth()), viewport.WithHeight(p.fitHeight()))
	// The portal scrolls vertically only; keyless bindings disable the
	// viewport's default h/l horizontal scrolling.
	p.viewport.KeyMap.Left = key.NewBinding()
	p.viewport.KeyMap.Right = key.NewBinding()
	p.viewport.SetContent(content)
	p.viewport.GotoTop()
	return p
}

// Refresh replaces live detail content without disorienting a user who is
// already reading it. SetYOffset clamps the previous position when the new
// content is shorter.
func (p DetailPortal) Refresh(title, content string) DetailPortal {
	offset := p.viewport.YOffset()
	p.title = title
	p.contentLines = strings.Count(content, "\n") + 1
	p.viewport.SetWidth(p.contentWidth())
	p.viewport.SetHeight(p.fitHeight())
	p.viewport.SetContent(content)
	p.viewport.SetYOffset(offset)
	return p
}

// fitHeight is the viewport height: the content's own height, capped by the
// terminal budget.
func (p DetailPortal) fitHeight() int {
	return max(min(p.contentHeight(), p.contentLines), 1)
}

func (p DetailPortal) Close() DetailPortal {
	p.trigger = portalClosed
	return p
}

// SetSize records the terminal dimensions and refits the viewport.
func (p DetailPortal) SetSize(width, height int) DetailPortal {
	p.width = width
	p.height = height
	p.viewport.SetWidth(p.contentWidth())
	p.viewport.SetHeight(p.fitHeight())
	return p
}

// contentWidth is the viewport width inside margins, border, and padding.
func (p DetailPortal) contentWidth() int {
	if p.width <= 0 {
		return 20
	}
	return max(p.width-2*portalMargin-4, 1)
}

// contentHeight is the viewport height inside margins, border, title,
// separator, and hint lines.
func (p DetailPortal) contentHeight() int {
	if p.height <= 0 {
		return 3
	}
	return max(p.height-2*portalMargin-2-3, 1)
}

// Update routes scroll keys to the viewport. Dismiss and quit are handled
// by the caller so trigger semantics stay in one place.
func (p DetailPortal) Update(msg tea.Msg) (DetailPortal, tea.Cmd) {
	var cmd tea.Cmd
	p.viewport, cmd = p.viewport.Update(msg)
	return p, cmd
}

// View renders the portal box composited centered over the background view.
func (p DetailPortal) View(background string) string {
	if p.width <= 0 || p.height <= 0 {
		return background
	}
	if p.height < 7 {
		lines := []string{truncateWithEllipsis("  "+styleTitle.Render(p.title), p.width)}
		if p.height > 1 {
			lines = append(lines, truncateWithEllipsis(p.viewport.View(), p.width))
		}
		if p.height > 2 {
			lines = append(lines, truncateWithEllipsis(viewFooter(p.width, portalFooterStatic), p.width))
		}
		return compositeOverlay(strings.Join(lines[:min(len(lines), p.height)], "\n"), background)
	}
	var b strings.Builder

	// The brand is already on screen behind the box; the portal carries
	// just its own name.
	b.WriteString(truncateWithEllipsis("  "+styleTitle.Render(p.title), p.contentWidth()))
	b.WriteString("\n")
	b.WriteString(viewSeparator(p.contentWidth()))
	b.WriteString("\n")
	b.WriteString(p.viewport.View())
	b.WriteString("\n")

	// Scroll hints are dead weight when everything fits: offer j/k and
	// the more-marker only when the viewport actually scrolls.
	footer := portalFooterStatic
	if p.viewport.TotalLineCount() > p.viewport.VisibleLineCount() {
		footer = portalFooter
	}
	hintLine := viewFooter(p.contentWidth(), footer)
	if !p.viewport.AtBottom() {
		hintLine += styleDim.Render("  ↓ more")
	}
	b.WriteString(truncateWithEllipsis(hintLine, p.contentWidth()))

	// lipgloss v2 Width spans the whole block including border and padding,
	// so the box adds its full frame (2 border + 2 padding) to the content.
	box := stylePortalBox.Width(p.contentWidth() + 4).Render(b.String())
	// A one-cell space margin covers the background characters adjacent to
	// the border, which otherwise read as clipped artifacts.
	box = lipgloss.NewStyle().Padding(0, 1).Render(box)
	return compositeOverlay(box, background)
}

// updatePortalKeys handles key input while the portal is open: quit passes
// through, esc dismisses, F1/? toggle or switch content, everything else
// scrolls the viewport.
func (m Model) updatePortalKeys(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, keys.Back):
		m.portal = m.portal.Close()
		return m, nil

	case key.Matches(msg, keys.GlobalHelp):
		if m.portal.trigger == portalHelp {
			m.portal = m.portal.Close()
		} else {
			title, content := m.helpContent()
			m.portal = m.portal.Open(portalHelp, title, content)
		}
		return m, nil

	case key.Matches(msg, keys.Info):
		if m.portal.trigger == portalDetails {
			m.portal = m.portal.Close()
		} else if title, content := m.detailContent(); content != "" {
			m.portal = m.portal.Open(portalDetails, title, content)
		} else {
			m.portal = m.portal.Close()
		}
		return m, nil

	default:
		var cmd tea.Cmd
		m.portal, cmd = m.portal.Update(msg)
		return m, cmd
	}
}
