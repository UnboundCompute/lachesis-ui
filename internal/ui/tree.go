package ui

import (
	"fmt"
	"path"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/UnboundCompute/lachesis-ui/internal/mcp"
)

// treeNode is one row in the source tree: a folder or a file. Folders are
// collapsed by default and load their children lazily the first time they are
// expanded, so opening a large repo shows a handful of top-level folders rather
// than a wall of files.
type treeNode struct {
	entry    mcp.FolderEntry
	depth    int
	parent   *treeNode
	expanded bool
	loaded   bool // children have been fetched
	loading  bool // a fetch is in flight
	children []*treeNode
}

// treeModel is the nested source tree with a per-file symbol outline. Unlike a
// file-manager "cd into a folder" list, the whole tree stays on screen and
// folders expand in place — matching the design's collapsible outline.
type treeModel struct {
	root      string // absolute source root
	cwd       string // enclosing folder of the selection, for the breadcrumb
	rootNodes []*treeNode
	flat      []*treeNode // visible rows, recomputed on every expand/collapse
	selNode   *treeNode
	off       int // scroll offset into flat

	outlineFile string
	decls       []mcp.Decl
	outlineNote string
	loadedOnce  bool
}

func newTree() treeModel { return treeModel{} }

func (m *treeModel) loaded() bool { return m.loadedOnce }

// open roots the tree at the repo root and loads its immediate children. The
// path argument is accepted for symmetry with the navigation message but the
// tree always roots at the repo root — nesting, not re-rooting, is how you move.
func (m *treeModel) open(a *App, _ string) tea.Cmd {
	if a.root == "" {
		return nil // overview hasn't resolved the root yet; a later press retries
	}
	m.root = a.root
	m.cwd = a.root
	m.rootNodes = nil
	m.flat = nil
	m.selNode = nil
	m.off = 0
	m.outlineFile, m.decls, m.outlineNote = "", nil, ""
	m.loadedOnce = true
	return loadFolderCmd(a.client, a.root)
}

func (m *treeModel) onFolder(msg folderLoadedMsg) {
	kids := m.buildNodes(msg.entries)
	if msg.path == m.root {
		for _, n := range kids {
			n.depth = 0
			n.parent = nil
		}
		m.rootNodes = kids
		m.rebuildFlat()
		if m.selNode == nil && len(m.flat) > 0 {
			m.selNode = m.flat[0]
		}
		m.syncCwd()
		return
	}
	node := m.findByPath(m.rootNodes, msg.path)
	if node == nil {
		return // children arrived for a node we no longer hold
	}
	for _, n := range kids {
		n.depth = node.depth + 1
		n.parent = node
	}
	node.children = kids
	node.loaded = true
	node.loading = false
	node.expanded = true
	m.rebuildFlat()
	m.syncCwd()
}

func (m *treeModel) onOutline(msg outlineLoadedMsg) {
	if msg.file != m.currentFilePath() {
		return // a stale outline for a file we've since moved off
	}
	m.outlineFile = msg.file
	m.decls = msg.decls
	m.outlineNote = msg.note
}

func (m *treeModel) update(a *App, msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "up", "k":
		return m.move(a, -1)
	case "down", "j":
		return m.move(a, +1)
	case "right", "l":
		// Expand a folder in place; on a file, nothing (its outline is already shown).
		if n := m.selNode; n != nil && n.entry.IsDir {
			return m.expand(a, n)
		}
		return nil
	case "enter":
		n := m.selNode
		if n == nil {
			return nil
		}
		if n.entry.IsDir {
			// Toggle: a second enter collapses what the first opened.
			if n.expanded {
				n.expanded = false
				m.rebuildFlat()
				return nil
			}
			return m.expand(a, n)
		}
		// On a file: jump into its first symbol's neighborhood.
		if len(m.decls) > 0 && m.outlineFile == n.entry.Path {
			name := m.decls[0].Name
			return func() tea.Msg { return gotoNeighborhoodMsg{name: name} }
		}
		return nil
	case "left", "h":
		return m.collapseOrParent(a)
	}
	return nil
}

func (m *treeModel) move(a *App, delta int) tea.Cmd {
	i := m.selIndex()
	if i < 0 {
		if len(m.flat) > 0 {
			m.selNode = m.flat[0]
		}
		m.syncCwd()
		return m.maybeLoadOutline(a)
	}
	j := i + delta
	if j < 0 {
		j = 0
	}
	if j >= len(m.flat) {
		j = len(m.flat) - 1
	}
	m.selNode = m.flat[j]
	m.syncCwd()
	return m.maybeLoadOutline(a)
}

// expand opens a folder, lazily fetching its children the first time.
func (m *treeModel) expand(a *App, n *treeNode) tea.Cmd {
	if !n.entry.IsDir || n.loading {
		return nil
	}
	if !n.loaded {
		n.loading = true
		return loadFolderCmd(a.client, n.entry.Path)
	}
	n.expanded = true
	m.rebuildFlat()
	m.syncCwd()
	return nil
}

// collapseOrParent closes an open folder, or jumps the selection to its parent —
// the natural "back out one level" gesture in a nested tree.
func (m *treeModel) collapseOrParent(a *App) tea.Cmd {
	n := m.selNode
	if n == nil {
		return nil
	}
	if n.entry.IsDir && n.expanded {
		n.expanded = false
		m.rebuildFlat()
		m.syncCwd()
		return nil
	}
	if n.parent != nil {
		m.selNode = n.parent
		m.syncCwd()
		return m.maybeLoadOutline(a)
	}
	return nil
}

func (m *treeModel) maybeLoadOutline(a *App) tea.Cmd {
	fp := m.currentFilePath()
	if fp == "" || fp == m.outlineFile {
		return nil
	}
	return loadOutlineCmd(a.client, fp)
}

func (m *treeModel) currentFilePath() string {
	if m.selNode == nil || m.selNode.entry.IsDir {
		return ""
	}
	return m.selNode.entry.Path
}

// syncCwd keeps the breadcrumb pointed at the folder that contains the selection.
func (m *treeModel) syncCwd() {
	switch {
	case m.selNode == nil:
		m.cwd = m.root
	case m.selNode.entry.IsDir:
		m.cwd = m.selNode.entry.Path
	case m.selNode.parent != nil:
		m.cwd = m.selNode.parent.entry.Path
	default:
		m.cwd = m.root
	}
}

func (m *treeModel) buildNodes(entries []mcp.FolderEntry) []*treeNode {
	sorted := sortEntries(entries)
	out := make([]*treeNode, 0, len(sorted))
	for _, e := range sorted {
		out = append(out, &treeNode{entry: e})
	}
	return out
}

func (m *treeModel) findByPath(nodes []*treeNode, p string) *treeNode {
	for _, n := range nodes {
		if n.entry.Path == p {
			return n
		}
		if n.entry.IsDir && len(n.children) > 0 {
			if hit := m.findByPath(n.children, p); hit != nil {
				return hit
			}
		}
	}
	return nil
}

// rebuildFlat recomputes the visible-row list: a node shows if every ancestor is
// expanded. Selection is tracked by pointer, so it survives the rebuild.
func (m *treeModel) rebuildFlat() {
	m.flat = m.flat[:0]
	var walk func(nodes []*treeNode)
	walk = func(nodes []*treeNode) {
		for _, n := range nodes {
			m.flat = append(m.flat, n)
			if n.entry.IsDir && n.expanded {
				walk(n.children)
			}
		}
	}
	walk(m.rootNodes)
}

func (m *treeModel) selIndex() int {
	for i, n := range m.flat {
		if n == m.selNode {
			return i
		}
	}
	return -1
}

func (m *treeModel) view(a *App, h int) string {
	leftW := 44
	rightW := a.width - leftW - 3
	if rightW < 20 {
		rightW = 20
	}
	left := m.viewTreePane(leftW, h)
	right := m.viewOutlinePane(a, rightW, h)
	sep := lipgloss.NewStyle().Foreground(colBorder).Render(strings.Repeat("│\n", h))
	return lipgloss.JoinHorizontal(lipgloss.Top, left, sep, right)
}

func (m *treeModel) viewTreePane(w, h int) string {
	var b strings.Builder
	fmt.Fprintln(&b, stColHead.Render("SOURCE TREE")+stDim.Render("  "+relPath(m.root, m.cwd)))
	rows := h - 2
	m.clampScroll(rows)
	if len(m.flat) == 0 {
		fmt.Fprintln(&b, stFainter.Render("  loading…"))
	}
	for i := m.off; i < len(m.flat) && i < m.off+rows; i++ {
		n := m.flat[i]
		selected := n == m.selNode
		indent := strings.Repeat("  ", n.depth)
		var icon, name, badge string
		if n.entry.IsDir {
			arrow := "▸"
			if n.expanded {
				arrow = "▾"
			}
			icon = stAmber.Render(arrow + " ")
			name = stFg.Render(n.entry.Label + "/")
			switch {
			case n.loaded:
				badge = "  " + stFainter.Render(fmt.Sprintf("%d", len(n.children)))
			case n.loading:
				badge = "  " + stFainter.Render("…")
			}
		} else {
			icon = stBlue.Render("▪ ")
			if selected {
				name = stBright.Render(n.entry.Label)
			} else {
				name = stFg.Render(n.entry.Label)
			}
		}
		fmt.Fprintln(&b, selRule(selected)+indent+icon+name+badge)
	}
	return lipgloss.NewStyle().Width(w).Render(b.String())
}

func (m *treeModel) viewOutlinePane(a *App, w, h int) string {
	var b strings.Builder
	fp := m.currentFilePath()
	if fp == "" {
		fmt.Fprintln(&b, stDim.Render("a folder — ")+stFainter.Render("→ expand, ← collapse"))
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, stFainter.Render("select a file to see the symbols it defines"))
		return lipgloss.NewStyle().Width(w).Render(b.String())
	}
	fmt.Fprintln(&b, stBright.Render(path.Base(fp))+"  "+stDim.Render(relPath(m.root, fp)))
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, stColHead.Render("WHAT IT DEFINES")+stDim.Render("  — symbols, not lines"))
	if len(m.decls) == 0 {
		note := m.outlineNote
		if note == "" {
			note = "loading…"
		}
		fmt.Fprintln(&b, stFainter.Render(wrap(note, w-2)))
		return lipgloss.NewStyle().Width(w).Render(b.String())
	}
	for _, d := range m.decls {
		kind := stMag.Render(padRight(shortKind(d.Kind), 3))
		fmt.Fprintln(&b, "  "+kind+" "+stFg.Render(padRight(d.Name, 28))+
			stDim.Render(fmt.Sprintf(":%d", d.StartLine)))
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, stFainter.Render("<enter> neighborhood of first symbol"))
	return lipgloss.NewStyle().Width(w).Render(b.String())
}

func (m *treeModel) clampScroll(rows int) {
	if rows < 1 {
		rows = 1
	}
	sel := m.selIndex()
	if sel < 0 {
		return
	}
	if sel < m.off {
		m.off = sel
	}
	if sel >= m.off+rows {
		m.off = sel - rows + 1
	}
	if m.off < 0 {
		m.off = 0
	}
}

func sortEntries(in []mcp.FolderEntry) []mcp.FolderEntry {
	out := make([]mcp.FolderEntry, len(in))
	copy(out, in)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].IsDir != out[j].IsDir {
			return out[i].IsDir // dirs first
		}
		return strings.ToLower(out[i].Label) < strings.ToLower(out[j].Label)
	})
	return out
}

func shortKind(k string) string {
	switch k {
	case "function", "method":
		return "fn"
	case "type", "struct", "class", "enum":
		return "ty"
	default:
		if len(k) >= 2 {
			return k[:2]
		}
		return k
	}
}

func wrap(s string, w int) string {
	if w < 8 {
		w = 8
	}
	words := strings.Fields(s)
	var b strings.Builder
	ln := 0
	for _, wd := range words {
		if ln+len(wd)+1 > w {
			b.WriteByte('\n')
			ln = 0
		}
		if ln > 0 {
			b.WriteByte(' ')
			ln++
		}
		b.WriteString(wd)
		ln += len(wd)
	}
	return b.String()
}
