package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/UnboundCompute/lachesis-ui/internal/mcp"
	"github.com/UnboundCompute/lachesis-ui/internal/subsystem"
)

// neighModel is a symbol's neighborhood, grouped the way a developer reads it:
// who reaches it and what it uses, bucketed by subsystem — never a raw dump.
type neighModel struct {
	name    string
	root    string
	callers []mcp.Symbol
	callees []mcp.Symbol
	body    mcp.Body
	hasBody bool

	callerGroups []subsystem.Bucket[mcp.Symbol]
	calleeGroups []subsystem.Bucket[mcp.Symbol]

	pane int // 0 = reached-by, 1 = uses
	sel  [2]int
}

func newNeigh() neighModel { return neighModel{} }

func (m *neighModel) onLoaded(msg neighborhoodLoadedMsg) {
	m.name = msg.name
	m.root = msg.root
	m.callers = msg.callers
	m.callees = msg.callees
	m.body = msg.body
	m.hasBody = msg.hasBody
	key := func(s mcp.Symbol) string { return relPath(msg.root, s.File) }
	m.callerGroups = subsystem.GroupByModule(msg.callers, key)
	m.calleeGroups = subsystem.GroupByModule(msg.callees, key)
	m.pane = 0
	m.sel = [2]int{0, 0}
}

func (m *neighModel) update(a *App, msg tea.KeyMsg) tea.Cmd {
	m.root = a.root
	switch msg.String() {
	case "tab":
		m.pane = 1 - m.pane
	case "left", "h":
		m.pane = 0
	case "right", "l":
		m.pane = 1
	case "up", "k":
		if m.sel[m.pane] > 0 {
			m.sel[m.pane]--
		}
	case "down", "j":
		if m.sel[m.pane] < m.flatCount(m.pane)-1 {
			m.sel[m.pane]++
		}
	case "enter":
		if sym, ok := m.selected(); ok {
			name := sym.Name
			return loadNeighborhoodCmd(a.client, name, a.root)
		}
	}
	return nil
}

func (m *neighModel) selected() (mcp.Symbol, bool) {
	flat := m.flatten(m.pane)
	i := m.sel[m.pane]
	if i < 0 || i >= len(flat) {
		return mcp.Symbol{}, false
	}
	return flat[i], true
}

func (m *neighModel) flatten(pane int) []mcp.Symbol {
	groups := m.callerGroups
	if pane == 1 {
		groups = m.calleeGroups
	}
	var out []mcp.Symbol
	for _, g := range groups {
		out = append(out, g.Items...)
	}
	return out
}

func (m *neighModel) flatCount(pane int) int { return len(m.flatten(pane)) }

func (m *neighModel) view(a *App, h int) string {
	m.root = a.root
	var b strings.Builder

	// Focus header + body peek.
	fmt.Fprintln(&b, stBright.Render(m.name)+"  "+
		stDim.Render(fmt.Sprintf("reached by %d · uses %d", len(m.callers), len(m.callees))))
	peek := m.viewBody(a.width - 2)
	fmt.Fprintln(&b, stPanel.Width(a.width-2).Render(peek))

	// Two grouped panes.
	usedH := lipgloss.Height(b.String())
	paneH := h - usedH - 1
	if paneH < 3 {
		paneH = 3
	}
	colW := (a.width - 3) / 2
	if colW < 20 {
		colW = 20
	}
	left := m.viewGroupPane("◀ REACHED BY", m.callerGroups, 0, colW, paneH)
	right := m.viewGroupPane("USES ▶", m.calleeGroups, 1, colW, paneH)
	sep := lipgloss.NewStyle().Foreground(colBorder).Render(strings.Repeat("│\n", paneH))
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, left, sep, right))
	return b.String()
}

func (m *neighModel) viewBody(w int) string {
	if !m.hasBody {
		return stFainter.Render("no source available for this symbol")
	}
	lines := strings.Split(m.body.Source, "\n")
	limit := 7
	var b strings.Builder
	loc := fmt.Sprintf("%s:%d–%d", relPath(m.root, m.body.File), m.body.StartLine, m.body.EndLine)
	fmt.Fprintln(&b, stFainter.Render(loc))
	for i, ln := range lines {
		if i >= limit {
			fmt.Fprint(&b, stFainter.Render("  …"))
			break
		}
		fmt.Fprintln(&b, stDim.Render(clip(ln, w-2)))
	}
	return b.String()
}

func (m *neighModel) viewGroupPane(title string, groups []subsystem.Bucket[mcp.Symbol], pane, w, h int) string {
	var b strings.Builder
	active := m.pane == pane
	head := stColHead.Render(title)
	if active {
		head = stCyanB.Render(title)
	}
	fmt.Fprintln(&b, head)

	rows := h - 1
	// Build flat display with group headers; track selectable symbol index.
	type line struct {
		text       string
		selectable bool
		symIndex   int
	}
	var disp []line
	symIdx := 0
	for _, g := range groups {
		label := g.Label
		if gloss := subsystem.Describe(label); gloss != "" {
			label = label + " · " + gloss
		}
		disp = append(disp, line{text: stDim.Render(fmt.Sprintf("%s · %d", label, len(g.Items)))})
		for _, s := range g.Items {
			via := ""
			if !strings.HasPrefix(s.Via, "direct") {
				via = stFainter.Render("  ⟿")
			}
			row := stFg.Render(padRight(clip(s.Name, w-8), w-8)) + via
			disp = append(disp, line{text: row, selectable: true, symIndex: symIdx})
			symIdx++
		}
	}
	if len(disp) == 0 {
		fmt.Fprintln(&b, stFainter.Render("(none)"))
		return lipgloss.NewStyle().Width(w).Render(b.String())
	}

	// Scroll so the selected symbol stays visible.
	selLine := 0
	for i, l := range disp {
		if l.selectable && l.symIndex == m.sel[pane] {
			selLine = i
			break
		}
	}
	off := 0
	if selLine >= rows {
		off = selLine - rows + 1
	}
	for i := off; i < len(disp) && i < off+rows; i++ {
		l := disp[i]
		if l.selectable {
			selected := active && l.symIndex == m.sel[pane]
			fmt.Fprintln(&b, selRule(selected)+l.text)
		} else {
			fmt.Fprintln(&b, "  "+l.text)
		}
	}
	return lipgloss.NewStyle().Width(w).Render(b.String())
}

func clip(s string, w int) string {
	if w < 1 {
		w = 1
	}
	if len(s) <= w {
		return s
	}
	if w <= 1 {
		return s[:w]
	}
	return s[:w-1] + "…"
}
