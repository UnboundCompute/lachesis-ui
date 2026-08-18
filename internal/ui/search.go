package ui

import (
	"fmt"
	"strings"

	"github.com/UnboundCompute/lachesis-ui/internal/mcp"
)

// searchModel renders results inside the search overlay.
type searchModel struct {
	query string
	hits  []mcp.Match
	total int
	sel   int
}

func newSearch() searchModel { return searchModel{} }

func (m *searchModel) onResults(msg searchResultsMsg) {
	m.query = msg.query
	m.hits = msg.hits
	m.total = msg.total
	m.sel = 0
}

func (m *searchModel) clear() {
	m.hits = nil
	m.total = 0
	m.sel = 0
}

func (m *searchModel) move(d int) {
	m.sel += d
	if m.sel < 0 {
		m.sel = 0
	}
	if m.sel >= len(m.hits) {
		m.sel = len(m.hits) - 1
	}
}

func (m *searchModel) selected() (mcp.Match, bool) {
	if m.sel < 0 || m.sel >= len(m.hits) {
		return mcp.Match{}, false
	}
	return m.hits[m.sel], true
}

func (m *searchModel) view(a *App) string {
	var b strings.Builder
	if len(m.hits) == 0 {
		if m.query == "" {
			fmt.Fprint(&b, stFainter.Render("type a symbol name, ↑↓ to pick, → or tab to open its neighborhood"))
		} else {
			fmt.Fprintf(&b, "%s", stDim.Render(fmt.Sprintf("no matches for %q", m.query)))
		}
		return b.String()
	}
	fmt.Fprintln(&b, stDim.Render(fmt.Sprintf("%d of %d matches", len(m.hits), m.total)))
	for i, h := range m.hits {
		selected := i == m.sel
		kind := stMag.Render(padRight(shortKind(h.Kind), 3))
		name := stFg.Render(padRight(clip(h.Name, 30), 30))
		if selected {
			name = stBright.Render(padRight(clip(h.Name, 30), 30))
		}
		loc := stFainter.Render(relHandle(a.root, h.File, h.Line))
		fmt.Fprintln(&b, selRule(selected)+kind+" "+name+" "+loc)
		if i >= 11 {
			break
		}
	}
	return b.String()
}
