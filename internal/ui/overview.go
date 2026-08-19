package ui

import (
	"fmt"
	"path"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

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
		rel := relPath(msg.root, h.File)
		lbl := subsystem.Of(rel)
		counts[lbl]++
		if _, ok := dirFor[lbl]; !ok {
			dirFor[lbl] = path.Dir(rel)
			if dirFor[lbl] == "." {
				dirFor[lbl] = ""
			}
		}
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
	if len(m.subs) > 4 {
		m.subs = m.subs[:4]
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
	contentW := a.width - 4
	if contentW < 40 {
		contentW = 40
	}

	// SUBSYSTEMS — two columns of bordered cards, matching the map layout.
	fmt.Fprintln(&b, stColHead.Render("SUBSYSTEMS — what lives where"))
	cardW := (contentW - 2) / 2
	for i := 0; i < len(m.subs); i += 2 {
		left := m.subsystemCard(i, cardW)
		right := ""
		if i+1 < len(m.subs) {
			right = m.subsystemCard(i+1, cardW)
		}
		fmt.Fprintln(&b, lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right))
	}
	if len(m.subs) == 0 {
		fmt.Fprintln(&b, stFainter.Render("no subsystem data in this graph"))
	}
	fmt.Fprintln(&b, stFainter.Render("graph limitation: per-subsystem file/function totals are unavailable"))
	fmt.Fprintln(&b)

	// ENTRY POINTS — graph-flagged roots rendered as compact pills.
	fmt.Fprintln(&b, stColHead.Render("ENTRY POINTS — how execution gets in"))
	if len(m.entries) == 0 {
		fmt.Fprintln(&b, stFainter.Render("graph has no exported, dispatch-target, or callback flags"))
	} else {
		for i := 0; i < len(m.entries); i += 3 {
			var line strings.Builder
			fmt.Fprint(&line, stDim.Render("graph roots  "))
			for j := i; j < len(m.entries) && j < i+3; j++ {
				e := m.entries[j]
				pill := lipgloss.NewStyle().Foreground(colBlue).Padding(0, 1).Render(e.Name)
				fmt.Fprint(&line, pill+" ")
			}
			fmt.Fprintln(&b, line.String())
		}
	}
	fmt.Fprintln(&b)

	// Hubs provides ranked landmarks, but not an ordered execution path. Keep
	// that graph limitation visible rather than drawing false call arrows.
	fmt.Fprintln(&b, stGreenB.Render("start here")+stDim.Render(" — highest-degree landmarks"))
	var landmarks strings.Builder
	for i, node := range m.spine {
		selected := m.rowIndexOfHub(i) == m.sel
		label := stCyan.Render(node.Name)
		if selected {
			label = stSelected.Render(node.Name)
		}
		if i > 0 {
			fmt.Fprint(&landmarks, stFainter.Render("  ·  "))
		}
		fmt.Fprint(&landmarks, label)
	}
	fmt.Fprintln(&b, wrapANSI(landmarks.String(), contentW))
	fmt.Fprintln(&b, stFainter.Render("graph limitation: hubs does not provide an ordered execution spine"))

	return lipgloss.NewStyle().Padding(1, 2).Width(a.width).Height(h).Render(b.String())
}

func (m *overviewModel) subsystemCard(i, w int) string {
	s := m.subs[i]
	selected := m.rowIndexOfSub(i) == m.sel
	location := s.dir
	if location != "" {
		location += "/"
	}
	title := stBright.Render(clip(strings.TrimSpace(s.label+" "+location), max(8, w-18)))
	title += "  " + stDim.Render(fmt.Sprintf("%d hubs", s.hubCount))
	gloss := s.gloss
	if gloss == "" {
		gloss = "graph-ranked subsystem"
	}
	style := lipgloss.NewStyle().Width(w).Height(2).Border(lipgloss.RoundedBorder()).BorderForeground(colRule).Padding(0, 1)
	if selected {
		style = style.BorderLeftForeground(colCyan)
	}
	return style.Render(title + "\n" + stDim.Render(clip(gloss, w-4)))
}

// wrapANSI is intentionally conservative: Lip Gloss measures styled strings,
// so it wraps only at landmark separators while preserving escape sequences.
func wrapANSI(s string, w int) string {
	if lipgloss.Width(s) <= w {
		return s
	}
	return ansi.Truncate(s, w, "…")
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
