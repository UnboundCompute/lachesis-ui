package ui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/UnboundCompute/lachesis-ui/internal/mcp"
	"github.com/UnboundCompute/lachesis-ui/internal/subsystem"
)

// overviewModel is the dev-centric map: subsystems, entry points, and the spine.
type overviewModel struct {
	root string
	hubs []mcp.Hub

	subs    []subRow  // subsystem cards, hub-density-ranked
	entries []mcp.Hub // exported / dispatch / callback entry points
	spine   []mcp.Hub // highest-degree nodes

	rows []ovRow // flat selectable list (subsystems then spine)
	sel  int
}

type subRow struct {
	label    string
	dir      string // path to open in the tree ("" if unknown)
	hubCount int
	gloss    string
}

type ovRowKind int

const (
	rowSub ovRowKind = iota
	rowHub
)

type ovRow struct {
	kind ovRowKind
	sub  subRow
	hub  mcp.Hub
}

func newOverview() overviewModel { return overviewModel{} }

func (m *overviewModel) onLoaded(msg overviewLoadedMsg) {
	m.root = msg.root
	m.hubs = msg.hubs

	// Hub density per subsystem: a real signal for "what is this built around".
	counts := map[string]int{}
	dirFor := map[string]string{}
	for _, h := range msg.hubs {
		lbl := subsystem.Of(relPath(msg.root, h.File))
		counts[lbl]++
	}
	// Prefer directory paths we actually saw under root for opening the tree.
	for _, d := range msg.dirs {
		lbl := subsystem.Of(d.Path)
		if _, ok := dirFor[lbl]; !ok {
			dirFor[lbl] = d.Path
		}
	}

	m.subs = m.subs[:0]
	for lbl, n := range counts {
		m.subs = append(m.subs, subRow{
			label:    lbl,
			dir:      dirFor[lbl],
			hubCount: n,
			gloss:    subsystem.Describe(lbl),
		})
	}
	sort.SliceStable(m.subs, func(i, j int) bool {
		if m.subs[i].hubCount != m.subs[j].hubCount {
			return m.subs[i].hubCount > m.subs[j].hubCount
		}
		return m.subs[i].label < m.subs[j].label
	})
	if len(m.subs) > 8 {
		m.subs = m.subs[:8]
	}

	// Entry points: nodes the graph flags as reachable from outside.
	m.entries = m.entries[:0]
	for _, h := range msg.hubs {
		if len(h.Flags) > 0 {
			m.entries = append(m.entries, h)
		}
		if len(m.entries) >= 6 {
			break
		}
	}

	// Spine: the highest-degree nodes, the best cold start.
	m.spine = msg.hubs
	if len(m.spine) > 6 {
		m.spine = m.spine[:6]
	}

	m.rebuildRows()
}

func (m *overviewModel) rebuildRows() {
	m.rows = m.rows[:0]
	for _, s := range m.subs {
		m.rows = append(m.rows, ovRow{kind: rowSub, sub: s})
	}
	for _, h := range m.spine {
		m.rows = append(m.rows, ovRow{kind: rowHub, hub: h})
	}
	if m.sel >= len(m.rows) {
		m.sel = 0
	}
}

func (m *overviewModel) update(a *App, msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "up", "k":
		if m.sel > 0 {
			m.sel--
		}
	case "down", "j":
		if m.sel < len(m.rows)-1 {
			m.sel++
		}
	case "enter", "right", "l":
		if m.sel < 0 || m.sel >= len(m.rows) {
			return nil
		}
		row := m.rows[m.sel]
		switch row.kind {
		case rowSub:
			p := row.sub.dir
			return func() tea.Msg { return gotoTreeMsg{path: p} }
		case rowHub:
			name := row.hub.Name
			return func() tea.Msg { return gotoNeighborhoodMsg{name: name} }
		}
	}
	return nil
}

func (m *overviewModel) view(a *App, h int) string {
	var b strings.Builder
	fmt.Fprintln(&b, stFg.Render("a code-property-graph over ")+stBright.Render(a.graph)+
		stDim.Render(" — organized the way a developer reads it, not as a raw graph"))
	fmt.Fprintln(&b)

	// SUBSYSTEMS
	fmt.Fprintln(&b, stColHead.Render("SUBSYSTEMS")+stDim.Render("  — what lives where (ranked by how much of the spine sits inside)"))
	for i, s := range m.subs {
		selected := m.rows[m.sel].kind == rowSub && m.rowIndexOfSub(i) == m.sel
		name := stBright.Render(padRight(s.label, 16))
		count := stDim.Render(fmt.Sprintf("%2d hubs", s.hubCount))
		gloss := ""
		if s.gloss != "" {
			gloss = stDim.Render(" · " + s.gloss)
		}
		fmt.Fprintln(&b, selRule(selected)+name+" "+count+gloss)
	}
	fmt.Fprintln(&b)

	// ENTRY POINTS
	fmt.Fprintln(&b, stColHead.Render("ENTRY POINTS")+stDim.Render("  — how execution gets in"))
	for _, e := range m.entries {
		flags := stMag.Render(strings.Join(e.Flags, " · "))
		fmt.Fprintln(&b, "  "+stFg.Render(padRight(e.Name, 26))+" "+
			stBlue.Render(padRight(relHandle(m.root, e.File, e.Line), 30))+" "+flags)
	}
	fmt.Fprintln(&b)

	// START HERE (spine)
	fmt.Fprintln(&b, stGreenB.Render("start here")+stDim.Render(" — the spine, highest-degree first"))
	for i, hnode := range m.spine {
		selected := m.rowIndexOfHub(i) == m.sel
		deg := stDim.Render(fmt.Sprintf("deg %-4d", hnode.Degree))
		fmt.Fprintln(&b, selRule(selected)+stCyan.Render(padRight(hnode.Name, 26))+" "+deg+" "+
			stFainter.Render(relHandle(m.root, hnode.File, hnode.Line)))
	}
	return b.String()
}

func (m *overviewModel) rowIndexOfSub(i int) int { return i }
func (m *overviewModel) rowIndexOfHub(i int) int { return len(m.subs) + i }

func relHandle(root, file string, line int) string {
	return fmt.Sprintf("%s:%d", relPath(root, file), line)
}

func padRight(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}
