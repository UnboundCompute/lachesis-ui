package ui

import (
	"path"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/UnboundCompute/lachesis-ui/internal/mcp"
)

// ---- cross-screen navigation messages (handled by the App) ----------------

type gotoOverviewMsg struct{}
type gotoTreeMsg struct{ path string } // "" = repo root
type gotoNeighborhoodMsg struct{ name string }

type errMsg struct{ err error }

func (e errMsg) Error() string { return e.err.Error() }

// ---- per-screen data messages ---------------------------------------------

type overviewLoadedMsg struct {
	root  string // absolute source root, for open_folder
	hubs  []mcp.Hub
	dirs  []mcp.FolderEntry // top-level subsystems (dirs under root)
	files int
}

type folderLoadedMsg struct {
	path    string
	entries []mcp.FolderEntry
}

type outlineLoadedMsg struct {
	file  string
	decls []mcp.Decl
	note  string // set when no symbol index is available for this file
}

type neighborhoodLoadedMsg struct {
	name    string
	root    string // absolute source root, so files can be shown/grouped relative
	callers []mcp.Symbol
	callees []mcp.Symbol
	body    mcp.Body
	hasBody bool
}

type searchResultsMsg struct {
	query string
	hits  []mcp.Match
	total int
}

// ---- commands (client calls, off the UI goroutine) ------------------------

func loadOverviewCmd(c *mcp.Client) tea.Cmd {
	return func() tea.Msg {
		hubs, err := c.Hubs(40)
		if err != nil {
			return errMsg{err}
		}
		root := commonDir(hubs)
		var dirs []mcp.FolderEntry
		var files int
		if root != "" {
			entries, err := c.OpenFolder(root)
			if err == nil {
				for _, e := range entries {
					if e.IsDir {
						dirs = append(dirs, e)
					} else {
						files++
					}
				}
			}
		}
		return overviewLoadedMsg{root: root, hubs: hubs, dirs: dirs, files: files}
	}
}

func loadFolderCmd(c *mcp.Client, p string) tea.Cmd {
	return func() tea.Msg {
		entries, err := c.OpenFolder(p)
		if err != nil {
			return errMsg{err}
		}
		return folderLoadedMsg{path: p, entries: entries}
	}
}

func loadOutlineCmd(c *mcp.Client, file string) tea.Cmd {
	return func() tea.Msg {
		decls, err := c.OpenFile(file)
		if err != nil {
			return errMsg{err}
		}
		note := ""
		if len(decls) == 0 {
			note = "this file declares no functions, methods, or types — it may hold only macros, data, or comments"
		}
		return outlineLoadedMsg{file: file, decls: decls, note: note}
	}
}

func loadNeighborhoodCmd(c *mcp.Client, name, root string) tea.Cmd {
	return func() tea.Msg {
		callers, err := c.Callers(name, 500)
		if err != nil {
			return errMsg{err}
		}
		callees, err := c.Callees(name, 500)
		if err != nil {
			return errMsg{err}
		}
		body, err := c.ReadBody(name, 1600)
		hasBody := err == nil && strings.TrimSpace(body.Source) != ""
		return neighborhoodLoadedMsg{
			name: name, root: root, callers: callers, callees: callees, body: body, hasBody: hasBody,
		}
	}
}

func searchCmd(c *mcp.Client, query string) tea.Cmd {
	return func() tea.Msg {
		hits, total, err := c.Search(query, 40)
		if err != nil {
			return errMsg{err}
		}
		return searchResultsMsg{query: query, hits: hits, total: total}
	}
}

// commonDir returns the longest shared directory prefix across hub file paths —
// a good-enough repo root to seed open_folder.
func commonDir(hubs []mcp.Hub) string {
	var paths []string
	for _, h := range hubs {
		if h.File != "" {
			paths = append(paths, path.Dir(h.File))
		}
	}
	if len(paths) == 0 {
		return ""
	}
	prefix := paths[0]
	for _, p := range paths[1:] {
		prefix = sharedPrefix(prefix, p)
		if prefix == "" || prefix == "/" {
			break
		}
	}
	return prefix
}

func sharedPrefix(a, b string) string {
	as := strings.Split(a, "/")
	bs := strings.Split(b, "/")
	n := len(as)
	if len(bs) < n {
		n = len(bs)
	}
	var out []string
	for i := 0; i < n; i++ {
		if as[i] != bs[i] {
			break
		}
		out = append(out, as[i])
	}
	return strings.Join(out, "/")
}

// relPath renders an absolute path relative to root for display.
func relPath(root, p string) string {
	if root == "" {
		return p
	}
	if strings.HasPrefix(p, root+"/") {
		return strings.TrimPrefix(p, root+"/")
	}
	return p
}
