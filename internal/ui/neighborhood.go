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
	name       string
	root       string
	callers    []mcp.Symbol
	callees    []mcp.Symbol
	body       mcp.Body
	hasBody    bool
	facts      mcp.NavigationFacts
	factErrors map[string]error

	callerGroups []subsystem.Bucket[mcp.Symbol]
	calleeGroups []subsystem.Bucket[mcp.Symbol]

	pane      int // 0 = reached-by, 1 = uses
	sel       [2]int
	history   []string
	historyAt int
	fullBody  bool
	bodyOff   int
	loading   bool
}

func newNeigh() neighModel { return neighModel{historyAt: -1} }

func (m *neighModel) pushHistory(name string) {
	if name == "" || (m.historyAt >= 0 && m.historyAt < len(m.history) && m.history[m.historyAt] == name) {
		return
	}
	if m.historyAt+1 < len(m.history) {
		m.history = m.history[:m.historyAt+1]
	}
	m.history = append(m.history, name)
	m.historyAt = len(m.history) - 1
}

func (m *neighModel) beginLoad(name string) {
	m.name = name
	m.callers, m.callees = nil, nil
	m.callerGroups, m.calleeGroups = nil, nil
	m.body, m.hasBody = mcp.Body{}, false
	m.facts, m.factErrors = mcp.NavigationFacts{}, nil
	m.loading = true
}

func (m *neighModel) onLoaded(msg neighborhoodLoadedMsg) {
	m.name = msg.name
	m.root = msg.root
	m.callers = msg.callers
	m.callees = msg.callees
	m.body = msg.body
	m.hasBody = msg.hasBody
	m.facts = msg.facts
	m.factErrors = msg.factErrors
	key := func(s mcp.Symbol) string { return relPath(msg.root, s.File) }
	m.callerGroups = subsystem.GroupByModule(msg.callers, key)
	m.calleeGroups = subsystem.GroupByModule(msg.callees, key)
	m.pane = 0
	m.sel = [2]int{0, 0}
	m.fullBody = false
	m.bodyOff = 0
	m.loading = false
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
		if m.fullBody && m.hasBody {
			if m.bodyOff > 0 {
				m.bodyOff--
			}
			return nil
		}
		if m.sel[m.pane] > 0 {
			m.sel[m.pane]--
		}
	case "down", "j":
		if m.fullBody && m.hasBody {
			m.bodyOff++
			return nil
		}
		if m.sel[m.pane] < m.flatCount(m.pane)-1 {
			m.sel[m.pane]++
		}
	case "enter":
		if sym, ok := m.selected(); ok {
			name := sym.Name
			return func() tea.Msg { return gotoNeighborhoodMsg{name: name} }
		}
	case "[":
		if m.historyAt > 0 {
			m.historyAt--
			name := m.history[m.historyAt]
			m.beginLoad(name)
			return loadNeighborhoodCmd(a.client, name, a.root)
		}
	case "]":
		if m.historyAt+1 < len(m.history) {
			m.historyAt++
			name := m.history[m.historyAt]
			m.beginLoad(name)
			return loadNeighborhoodCmd(a.client, name, a.root)
		}
	case "b":
		m.fullBody = !m.fullBody
		m.bodyOff = 0
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
	leftW := (a.width - 2) * 30 / 100
	centerW := (a.width - 2) * 40 / 100
	rightW := a.width - leftW - centerW - 2
	if leftW < 22 {
		leftW = 22
	}
	if centerW < 28 {
		centerW = 28
	}
	if rightW < 22 {
		rightW = 22
	}
	left := m.viewGroupPane("◀ REACHED BY", m.callerGroups, 0, leftW, h)
	focus := m.viewFocus(centerW, h)
	right := m.viewGroupPane("USES ▶", m.calleeGroups, 1, rightW, h)
	sep := lipgloss.NewStyle().Foreground(colRule).Render(strings.TrimSuffix(strings.Repeat("│\n", h), "\n"))
	return lipgloss.JoinHorizontal(lipgloss.Top, left, sep, focus, sep, right)
}

func (m *neighModel) viewFocus(w, h int) string {
	var b strings.Builder
	fmt.Fprintln(&b, stCyanB.Render("● FOCUS"))
	fmt.Fprintln(&b, stBright.Render(clip(m.name, w-4)))
	if m.loading {
		fmt.Fprintln(&b, stFainter.Render("loading neighborhood…"))
		return lipgloss.NewStyle().Width(w).Height(h).Background(colFocus).Padding(0, 1).Render(b.String())
	}
	if !m.hasBody {
		fmt.Fprintln(&b, stFainter.Render("no source available for this symbol"))
		return lipgloss.NewStyle().Width(w).Height(h).Background(colFocus).Padding(0, 1).Render(b.String())
	}
	loc := fmt.Sprintf("%s:%d–%d", relPath(m.root, m.body.File), m.body.StartLine, m.body.EndLine)
	fmt.Fprintln(&b, stDim.Render(loc))
	fmt.Fprintln(&b)
	lines := strings.Split(m.body.Source, "\n")
	limit := h - 11
	if limit < 3 {
		limit = 3
	}
	if !m.fullBody && limit > 7 {
		limit = 7
	}
	if m.bodyOff > len(lines)-limit {
		m.bodyOff = max(0, len(lines)-limit)
	}
	var code strings.Builder
	start := 0
	if m.fullBody {
		start = m.bodyOff
	}
	end := min(len(lines), start+limit)
	for i := start; i < end; i++ {
		ln := lines[i]
		lineNo := stFainter.Render(fmt.Sprintf("%-4d", m.body.StartLine+i))
		fmt.Fprintln(&code, lineNo+stFg.Render(clip(ln, w-13)))
	}
	if end < len(lines) {
		fmt.Fprintln(&code, stFainter.Render(fmt.Sprintf("… lines %d–%d of %d · ↑↓ scroll", start+1, end, len(lines))))
	}
	if m.body.Truncated {
		fmt.Fprintln(&code, stAmber.Render("graph body response was truncated; source checkout may contain the complete function"))
	}
	fmt.Fprintln(&b, stPanel.Width(max(12, w-4)).Render(strings.TrimSuffix(code.String(), "\n")))
	fmt.Fprintln(&b)
	reaches := m.factBadge("reaches from here", "reaches", m.facts.Reaches)
	guards := m.factBadge("guards", "guards", m.facts.Guards)
	points := m.factBadge("points-to", "points-to", m.facts.PointsTo)
	factsLine := reaches + "  " + guards + "  " + points
	if lipgloss.Width(factsLine) <= w-4 {
		fmt.Fprintln(&b, factsLine)
	} else {
		fmt.Fprintln(&b, reaches)
		fmt.Fprintln(&b, guards+"  "+points)
	}
	if missing := m.missingFacts(); missing != "" {
		fmt.Fprintln(&b, stFainter.Render(clip("graph data unavailable: "+missing, w-4)))
	}
	return lipgloss.NewStyle().Width(w).Height(h).Background(colFocus).Padding(0, 1).Render(b.String())
}

func (m *neighModel) factBadge(label, key string, value int) string {
	if _, missing := m.factErrors[key]; missing {
		return stDim.Render(label+" ") + stAmber.Render("n/a")
	}
	return stDim.Render(label+" ") + stFg.Render(fmt.Sprintf("%d", value))
}

func (m *neighModel) missingFacts() string {
	var missing []string
	for _, key := range []string{"reaches", "guards", "points-to"} {
		if _, ok := m.factErrors[key]; ok {
			missing = append(missing, key)
		}
	}
	return strings.Join(missing, ", ")
}

func (m *neighModel) viewGroupPane(title string, groups []subsystem.Bucket[mcp.Symbol], pane, w, h int) string {
	var b strings.Builder
	active := m.pane == pane
	head := stColHead.Render(title)
	if active {
		head = stCyanB.Render(title)
	}
	fmt.Fprintln(&b, head)

	rows := h - 2
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
		gloss := subsystem.Describe(label)
		if gloss == "" {
			gloss = label
		}
		disp = append(disp, line{text: stDim.Render(fmt.Sprintf("from %s · %d", gloss, len(g.Items)))})
		shown := g.Items
		if len(shown) > 4 {
			shown = shown[:4]
		}
		for _, s := range shown {
			via := ""
			if !strings.HasPrefix(s.Via, "direct") {
				via = stFainter.Render("  ⟿")
			}
			location := ""
			if s.File != "" {
				location = stBlue.Render(clip(relHandle(m.root, s.File, s.Line), 14))
			}
			nameW := w - lipgloss.Width(location) - lipgloss.Width(via) - 5
			row := stFg.Render(padRight(clip(s.Name, nameW), nameW)) + " " + location + via
			disp = append(disp, line{text: row, selectable: true, symIndex: symIdx})
			symIdx++
		}
		if len(g.Items) > len(shown) {
			disp = append(disp, line{text: stCyan.Render(fmt.Sprintf("show all %d →", len(g.Items)))})
			for _, s := range g.Items[len(shown):] {
				location := ""
				if s.File != "" {
					location = stBlue.Render(clip(relHandle(m.root, s.File, s.Line), 14))
				}
				nameW := w - lipgloss.Width(location) - 5
				row := stFg.Render(padRight(clip(s.Name, nameW), nameW)) + " " + location
				disp = append(disp, line{text: row, selectable: true, symIndex: symIdx})
				symIdx++
			}
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
	return lipgloss.NewStyle().Width(w).Height(h).Padding(0, 1).Render(b.String())
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
