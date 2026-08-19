package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/UnboundCompute/lachesis-ui/internal/mcp"
)

func navigationFixture() App {
	a := New(nil, "demo-graph")
	a.width, a.height, a.ready = 130, 33, true
	hubs := []mcp.Hub{
		{Name: "main", File: "cli/main.py", Line: 62, Degree: 40, Flags: []string{"exported"}},
		{Name: "ensureGraph", File: "core/graph.py", Line: 41, Degree: 30, Flags: []string{"dispatch_target"}},
		{Name: "rank", File: "planner/rank.py", Line: 12, Degree: 20},
		{Name: "parse", File: "frontends/python.py", Line: 9, Degree: 10},
	}
	a.overview.onLoaded(overviewLoadedMsg{hubs: hubs, dirs: []mcp.FolderEntry{
		{IsDir: true, Label: "cli", Path: "cli"},
		{IsDir: true, Label: "core", Path: "core"},
		{IsDir: true, Label: "planner", Path: "planner"},
		{IsDir: true, Label: "frontends", Path: "frontends"},
	}})
	return a
}

func TestNavigationScreensMatchDesignedStructure(t *testing.T) {
	a := navigationFixture()
	assertFrame(t, a.View(), "AREAS — what lives where", "STARTING POINTS — where execution enters", "GOOD PLACES TO START")

	file := &treeNode{entry: mcp.FolderEntry{Label: "main.py", Path: "cli/main.py"}, depth: 1}
	dir := &treeNode{entry: mcp.FolderEntry{IsDir: true, Label: "cli", Path: "cli"}, expanded: true, loaded: true, children: []*treeNode{file}}
	file.parent = dir
	a.view = viewTree
	a.tree = treeModel{rootNodes: []*treeNode{dir}, flat: []*treeNode{dir, file}, selNode: file, outlineFile: file.entry.Path,
		decls: []mcp.Decl{{Kind: "function", Name: "command_scan", StartLine: 62}}, imports: []string{"indexer", "planner"}, loadedOnce: true, pane: 1}
	assertFrame(t, a.View(), "SOURCE TREE", "SYMBOLS IN THIS FILE", "imports from")

	a.view = viewNeighborhood
	a.neigh.onLoaded(neighborhoodLoadedMsg{name: "ensureGraph", callers: []mcp.Symbol{{Name: "main", File: "cli/main.py", Line: 62}},
		callees: []mcp.Symbol{{Name: "load", File: "core/store.py", Line: 41}}, hasBody: true,
		body: mcp.Body{File: "core/graph.py", StartLine: 41, EndLine: 44, Source: "def ensureGraph():\n    return load()"}})
	assertFrame(t, a.View(), "◀ CALLED FROM", "● SELECTED SYMBOL", "CALLS ▶")
}

func TestNavigationFlowKeys(t *testing.T) {
	a := navigationFixture()
	model, _ := a.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	a = model.(App)
	if a.view != viewTree {
		t.Fatalf("t should open tree, got %v", a.view)
	}
	model, _ = a.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	a = model.(App)
	if a.view != viewOverview {
		t.Fatalf("esc should return to overview, got %v", a.view)
	}
}

func TestNavigationFrameFitsEightyColumnTerminal(t *testing.T) {
	a := navigationFixture()
	a.width, a.height = 80, 24
	rendered := a.View()
	if got := lipgloss.Height(rendered); got != 24 {
		t.Errorf("frame height = %d, want 24", got)
	}
	if got := lipgloss.Width(rendered); got != 80 {
		t.Errorf("frame width = %d, want 80", got)
	}
}

func TestNeighborhoodFullBodyScrollsLongFunctions(t *testing.T) {
	a := navigationFixture()
	a.neigh.onLoaded(neighborhoodLoadedMsg{
		name: "long", hasBody: true,
		body: mcp.Body{File: "lib/long.c", StartLine: 1, EndLine: 200, Source: strings.Repeat("statement();\n", 200)},
	})
	if a.neigh.update(&a, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}}) != nil {
		t.Fatal("body toggle unexpectedly returned a command")
	}
	a.neigh.update(&a, tea.KeyMsg{Type: tea.KeyDown})
	if a.neigh.bodyOff != 1 {
		t.Fatalf("full-body down offset = %d, want 1", a.neigh.bodyOff)
	}
	if !strings.Contains(ansi.Strip(a.neigh.view(&a, 20)), "scroll") {
		t.Fatal("long full body did not show its scroll affordance")
	}
}

func TestHelpOverlayIsDiscoverable(t *testing.T) {
	a := navigationFixture()
	model, _ := a.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	a = model.(App)
	if !a.help || !strings.Contains(ansi.Strip(a.View()), "How to explore this code") {
		t.Fatal("? did not open the help overlay")
	}
	model, _ = a.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	a = model.(App)
	if a.help {
		t.Fatal("esc did not close the help overlay")
	}
}

func TestOverviewSubsystemPathExpandsNestedTree(t *testing.T) {
	m := treeModel{root: "", requestedPath: "src/http"}
	if next := m.onFolder(folderLoadedMsg{path: "", entries: []mcp.FolderEntry{{IsDir: true, Label: "src", Path: "src"}}}); next != "src" {
		t.Fatalf("first expansion = %q, want src", next)
	}
	if next := m.onFolder(folderLoadedMsg{path: "src", entries: []mcp.FolderEntry{{IsDir: true, Label: "http", Path: "src/http"}}}); next != "src/http" {
		t.Fatalf("second expansion = %q, want src/http", next)
	}
	if next := m.onFolder(folderLoadedMsg{path: "src/http", entries: []mcp.FolderEntry{{Label: "client.go", Path: "src/http/client.go"}}}); next != "" {
		t.Fatalf("unexpected further expansion %q", next)
	}
	if m.selNode == nil || m.selNode.entry.Path != "src/http" || !m.selNode.expanded {
		t.Fatalf("requested subsystem was not selected and expanded: %#v", m.selNode)
	}
}

func assertFrame(t *testing.T, rendered string, want ...string) {
	t.Helper()
	plain := ansi.Strip(rendered)
	for _, s := range want {
		if !strings.Contains(plain, s) {
			t.Errorf("render missing %q", s)
		}
	}
	if got := lipgloss.Height(rendered); got != 33 {
		t.Errorf("frame height = %d, want 33", got)
	}
	if got := lipgloss.Width(rendered); got != 130 {
		t.Errorf("frame width = %d, want 130", got)
	}
}
