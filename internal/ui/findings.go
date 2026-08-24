package ui

import (
	"fmt"
	"github.com/UnboundCompute/lachesis-ui/internal/mcp"
	tea "github.com/charmbracelet/bubbletea"
	"sort"
	"strings"
)

type findingsModel struct {
	rows     []mcp.Candidate
	sel      int
	detail   map[string]any
	path     map[string]any
	skeleton map[string]any
	active   mcp.Candidate
	scroll   int
}

func newFindings() findingsModel { return findingsModel{} }
func (m *findingsModel) update(a *App, msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "up", "k":
		if m.sel > 0 {
			m.sel--
		}
	case "down", "j":
		if m.sel < len(m.rows)-1 {
			m.sel++
		}
	case "enter":
		if len(m.rows) > 0 {
			m.active = m.rows[m.sel]
			a.view = viewFindingDetail
			return loadCandidateDetailCmd(a.client, m.active)
		}
	case "r":
		if len(m.rows) > 0 {
			m.active = m.rows[m.sel]
			a.view = viewReaches
			return loadReachesCmd(a.client, m.active)
		}
	case "s":
		if len(m.rows) > 0 {
			m.active = m.rows[m.sel]
			a.view = viewSkeleton
			return loadSkeletonCmd(a.client, m.active)
		}
	}
	return nil
}
func (m findingsModel) list(a *App, h int) string {
	var b strings.Builder
	fmt.Fprintln(&b, stColHead.Render(fmt.Sprintf("CANDIDATES — evidence to review [%d]", len(m.rows))))
	fmt.Fprintln(&b, stDim.Render("These are pointers, not safety verdicts. Select one to inspect the evidence."))
	fmt.Fprintln(&b, stFainter.Render("rank   entry point                 sink                       kind       location        guard"))
	for i, c := range m.rows {
		line := fmt.Sprintf("%-6.2f %-27s %-26s %-10s %-15s %s", c.Rank, c.Entrypoint, c.Sink, c.Kind, fmt.Sprintf("%s:%d", c.File, c.Line), c.Guard)
		if i == m.sel {
			line = selRule(true) + stSelected.Render(line)
		} else {
			line = selRule(false) + line
		}
		fmt.Fprintln(&b, line)
	}
	if len(m.rows) == 0 {
		fmt.Fprintln(&b, stFainter.Render("no catalog candidates returned — the graph may not have an Atropos catalog"))
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, stAmber.Render("honest limitation: the graph reports observations and coverage frontiers; it does not prove exploitability"))
	return padView(b.String(), a.width, h)
}
func (m findingsModel) detailView(a *App, h int) string {
	c := m.active
	var b strings.Builder
	fmt.Fprintln(&b, stDim.Render("Candidates / ")+stCyanB.Render(c.Source+" → "+c.Sink))
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, stBright.Render(fmt.Sprintf("%.2f  %s → %s", c.Rank, c.Source, c.Sink)))
	fmt.Fprintln(&b, stDim.Render("candidate evidence capsule  ·  neutral, deterministic observations"))
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, stColHead.Render("PATH"))
	fmt.Fprintf(&b, "  source   %s\n", stGreen.Render(c.Source))
	fmt.Fprintf(&b, "  sink     %s\n", stRed.Render(c.Sink))
	fmt.Fprintf(&b, "  location %s\n", stBlue.Render(fmt.Sprintf("%s:%d", c.File, c.Line)))
	fmt.Fprintf(&b, "  guard    %s\n", stAmber.Render(or(c.Guard, "not reported")))
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, stAmber.Render("PROVE OR KILL"))
	fmt.Fprintln(&b, stDim.Render("Read the source and path before deciding. This UI never labels a candidate safe or unsafe."))
	if len(m.detail) > 0 {
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, stColHead.Render("EVIDENCE FIELDS"))
		keys := make([]string, 0, len(m.detail))
		for k := range m.detail {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			v := fmt.Sprint(m.detail[k])
			if len(v) > 120 {
				v = v[:120] + "…"
			}
			fmt.Fprintf(&b, "  %-18s %s\n", k, stFg.Render(v))
		}
	}
	return padView(b.String(), a.width, h)
}
func (m findingsModel) reachesView(a *App, h int) string {
	var b strings.Builder
	fmt.Fprintln(&b, stDim.Render("Candidates / ")+stCyanB.Render("reaches — witness path"))
	fmt.Fprintln(&b, stDim.Render("A labelled path from source to sink. Missing hops are shown as unavailable evidence."))
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "◆ %s  %s\n", stGreen.Render("source"), m.active.Source)
	fmt.Fprintln(&b, stFainter.Render("│  value-flow edges and call seams returned by the graph"))
	fmt.Fprintf(&b, "◆ %s  %s\n", stRed.Render("sink"), m.active.Sink)
	if len(m.path) == 0 {
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, stAmber.Render("witness data unavailable — this tool may not resolve this pair"))
	} else {
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, stGreen.Render("witness returned"))
	}
	return padView(b.String(), a.width, h)
}
func (m findingsModel) skeletonView(a *App, h int) string {
	var b strings.Builder
	fmt.Fprintln(&b, stDim.Render("Skeleton / ")+stCyanB.Render(m.active.Entrypoint))
	fmt.Fprintln(&b, stDim.Render("A control skeleton keeps branches and catalogued sinks while eliding unrelated code."))
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, stColHead.Render("CONTROL + SINK EVENTS"))
	if len(m.skeleton) == 0 {
		fmt.Fprintln(&b, stAmber.Render("skeleton data unavailable"))
	} else {
		for _, k := range []string{"function", "events", "sinks", "guards"} {
			if v, ok := m.skeleton[k]; ok {
				fmt.Fprintf(&b, "%-10s %v\n", k, v)
			}
		}
	}
	return padView(b.String(), a.width, h)
}
func padView(s string, w, h int) string { return strings.TrimRight(s, "\n") }
func or(a, b string) string {
	if a == "" {
		return b
	}
	return a
}

func (a *App) findingsKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "tab":
		a.view = viewOverview
		return nil
	case "esc":
		a.view = viewOverview
		return nil
	}
	return a.findings.update(a, msg)
}
