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

// treeModel is the source tree with a per-file symbol outline.
type treeModel struct {
	root    string // absolute source root (floor for ascending)
	cwd     string // current folder root passed to open_folder
	entries []mcp.FolderEntry
	sel     int
	off     int // scroll offset

	outlineFile string
	decls       []mcp.Decl
	outlineNote string
	loadedOnce  bool
}

func newTree() treeModel { return treeModel{} }

func (m *treeModel) loaded() bool { return m.loadedOnce }

// open (re)roots the tree. path "" opens the repo root.
func (m *treeModel) open(a *App, p string) tea.Cmd {
	m.root = a.root
	if p == "" {
		p = a.root
	}
	m.cwd = p
	m.sel, m.off = 0, 0
	m.outlineFile, m.decls, m.outlineNote = "", nil, ""
	m.loadedOnce = true
	return loadFolderCmd(a.client, p)
}

func (m *treeModel) onFolder(msg folderLoadedMsg) {
	m.cwd = msg.path
	m.entries = sortEntries(msg.entries)
	m.sel, m.off = 0, 0
	m.outlineFile, m.decls, m.outlineNote = "", nil, ""
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
		if m.sel > 0 {
			m.sel--
		}
		return m.maybeLoadOutline(a)
	case "down", "j":
		if m.sel < len(m.entries)-1 {
			m.sel++
		}
		return m.maybeLoadOutline(a)
	case "right", "enter", "l":
		if m.sel < 0 || m.sel >= len(m.entries) {
			return nil
		}
		e := m.entries[m.sel]
		if e.IsDir {
			return loadFolderCmd(a.client, e.Path)
		}
		// On a file: jump into its first symbol's neighborhood if we have one.
		if len(m.decls) > 0 && m.outlineFile == e.Path {
			name := m.decls[0].Name
			return func() tea.Msg { return gotoNeighborhoodMsg{name: name} }
		}
		return nil
	case "left", "h":
		return m.ascend(a)
	}
	return nil
}

func (m *treeModel) ascend(a *App) tea.Cmd {
	if m.cwd == m.root || m.cwd == "" {
		return nil
	}
	parent := path.Dir(m.cwd)
	if parent == "." || parent == "/" {
		parent = m.root
	}
	return loadFolderCmd(a.client, parent)
}

func (m *treeModel) maybeLoadOutline(a *App) tea.Cmd {
	fp := m.currentFilePath()
	if fp == "" || fp == m.outlineFile {
		return nil
	}
	return loadOutlineCmd(a.client, fp)
}

func (m *treeModel) currentFilePath() string {
	if m.sel < 0 || m.sel >= len(m.entries) {
		return ""
	}
	e := m.entries[m.sel]
	if e.IsDir {
		return ""
	}
	return e.Path
}

func (m *treeModel) view(a *App, h int) string {
	leftW := 40
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
	for i := m.off; i < len(m.entries) && i < m.off+rows; i++ {
		e := m.entries[i]
		selected := i == m.sel
		var icon, name string
		if e.IsDir {
			icon = stAmber.Render("▸ ")
			name = stFg.Render(e.Label + "/")
		} else {
			icon = stBlue.Render("▪ ")
			name = stFg.Render(e.Label)
			if selected {
				name = stBright.Render(e.Label)
			}
		}
		fmt.Fprintln(&b, selRule(selected)+icon+name)
	}
	return lipgloss.NewStyle().Width(w).Render(b.String())
}

func (m *treeModel) viewOutlinePane(a *App, w, h int) string {
	var b strings.Builder
	fp := m.currentFilePath()
	if fp == "" {
		fmt.Fprintln(&b, stDim.Render("a folder — ")+stFainter.Render("→ to open, ← to go up"))
		return lipgloss.NewStyle().Width(w).Render(b.String())
	}
	fmt.Fprintln(&b, stBright.Render(path.Base(fp))+"  "+stDim.Render(fp))
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
	return lipgloss.NewStyle().Width(w).Render(b.String())
}

func (m *treeModel) clampScroll(rows int) {
	if rows < 1 {
		rows = 1
	}
	if m.sel < m.off {
		m.off = m.sel
	}
	if m.sel >= m.off+rows {
		m.off = m.sel - rows + 1
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
