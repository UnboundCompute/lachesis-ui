// Package ui is the Bubbletea front end: a persistent, keyboard-driven screen
// over the lachesis graph, organized around how a developer thinks (subsystems,
// files, a symbol's neighborhood) rather than raw graph primitives.
package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/UnboundCompute/lachesis-ui/internal/mcp"
)

type view int

const (
	viewOverview view = iota
	viewTree
	viewNeighborhood
)

// App is the root Bubbletea model.
type App struct {
	client *mcp.Client
	graph  string // display name of the loaded graph
	root   string // absolute source root

	width, height int
	ready         bool // graph loaded + overview in hand
	spinner       spinner.Model
	err           error
	statusHint    string

	view      view
	overview  overviewModel
	tree      treeModel
	neigh     neighModel
	neighInit bool // whether the neighborhood has ever been loaded

	searching bool
	search    textinput.Model
	results   searchModel
}

// New builds the root model around a connected client.
func New(client *mcp.Client, graph string) App {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = stCyan

	ti := textinput.New()
	ti.Prompt = stCyanB.Render("/ ")
	ti.Placeholder = "symbol name…"
	ti.CharLimit = 80

	return App{
		client:   client,
		graph:    graph,
		spinner:  sp,
		search:   ti,
		overview: newOverview(),
		tree:     newTree(),
		neigh:    newNeigh(),
		results:  newSearch(),
	}
}

func (a App) Init() tea.Cmd {
	// The first Hubs call blocks until the engine finishes loading the graph,
	// which gives us the warm-load wait for free while the spinner runs.
	return tea.Batch(a.spinner.Tick, loadOverviewCmd(a.client))
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
		return a, nil

	case spinner.TickMsg:
		if a.ready {
			return a, nil
		}
		var cmd tea.Cmd
		a.spinner, cmd = a.spinner.Update(msg)
		return a, cmd

	case errMsg:
		a.err = msg.err
		a.ready = true // stop the spinner; show the error
		return a, nil

	case tea.KeyMsg:
		return a.handleKey(msg)

	// ---- cross-screen navigation --------------------------------------
	case gotoOverviewMsg:
		a.view = viewOverview
		a.err = nil
		return a, nil
	case gotoTreeMsg:
		a.view = viewTree
		a.err = nil
		return a, a.tree.open(&a, msg.path)
	case gotoNeighborhoodMsg:
		a.view = viewNeighborhood
		a.neighInit = true
		a.err = nil
		return a, loadNeighborhoodCmd(a.client, msg.name, a.root)

	// ---- data arrivals: route to the owning screen --------------------
	case overviewLoadedMsg:
		a.ready = true
		a.root = msg.root
		a.overview.onLoaded(msg)
		// If the user already jumped to the tree while the graph was still
		// loading, its open() bailed for want of a root — open it now.
		if a.view == viewTree && !a.tree.loaded() {
			return a, a.tree.open(&a, "")
		}
		return a, nil
	case folderLoadedMsg:
		a.tree.onFolder(msg)
		return a, nil
	case outlineLoadedMsg:
		a.tree.onOutline(msg)
		return a, nil
	case neighborhoodLoadedMsg:
		a.neigh.onLoaded(msg)
		return a, nil
	case searchResultsMsg:
		a.results.onResults(msg)
		return a, nil
	}
	return a, nil
}

func (a App) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Search overlay owns the keyboard while open.
	if a.searching {
		switch msg.String() {
		case "esc":
			a.searching = false
			a.search.Blur()
			return a, nil
		case "enter":
			q := strings.TrimSpace(a.search.Value())
			if q == "" {
				a.searching = false
				return a, nil
			}
			return a, searchCmd(a.client, q)
		case "up", "down":
			a.results.move(keyDelta(msg.String()))
			return a, nil
		case "tab":
			// Enter accepts the highlighted result → its neighborhood.
			if sel, ok := a.results.selected(); ok {
				a.searching = false
				a.search.Blur()
				return a, func() tea.Msg { return gotoNeighborhoodMsg{name: sel.Name} }
			}
			return a, nil
		}
		// ctrl+enter / right-arrow accept too; otherwise feed the text box.
		if msg.String() == "right" {
			if sel, ok := a.results.selected(); ok {
				a.searching = false
				a.search.Blur()
				return a, func() tea.Msg { return gotoNeighborhoodMsg{name: sel.Name} }
			}
		}
		var cmd tea.Cmd
		a.search, cmd = a.search.Update(msg)
		return a, cmd
	}

	switch msg.String() {
	case "ctrl+c", "q":
		return a, tea.Quit
	case "/":
		a.searching = true
		a.search.SetValue("")
		a.search.Focus()
		a.results.clear()
		return a, textinput.Blink
	case "1", "o":
		a.view = viewOverview
		return a, nil
	case "2", "t":
		a.view = viewTree
		if !a.tree.loaded() {
			return a, a.tree.open(&a, "")
		}
		return a, nil
	case "3":
		if a.neighInit {
			a.view = viewNeighborhood
		}
		return a, nil
	}

	if !a.ready {
		return a, nil
	}
	// Delegate to the active screen.
	switch a.view {
	case viewOverview:
		return a, a.overview.update(&a, msg)
	case viewTree:
		return a, a.tree.update(&a, msg)
	case viewNeighborhood:
		return a, a.neigh.update(&a, msg)
	}
	return a, nil
}

func (a App) View() string {
	if a.width == 0 {
		return "" // wait for the first WindowSizeMsg
	}
	header := a.renderHeader()
	status := a.renderStatus()

	bodyHeight := a.height - lipgloss.Height(header) - lipgloss.Height(status)
	if bodyHeight < 1 {
		bodyHeight = 1
	}

	var body string
	switch {
	case !a.ready:
		body = a.renderLoading(bodyHeight)
	case a.err != nil:
		body = a.renderError(bodyHeight)
	default:
		switch a.view {
		case viewOverview:
			body = a.overview.view(&a, bodyHeight)
		case viewTree:
			body = a.tree.view(&a, bodyHeight)
		case viewNeighborhood:
			body = a.neigh.view(&a, bodyHeight)
		}
	}
	body = lipgloss.NewStyle().Height(bodyHeight).MaxHeight(bodyHeight).Render(body)

	out := lipgloss.JoinVertical(lipgloss.Left, header, body, status)
	if a.searching {
		out = a.overlaySearch(out)
	}
	return out
}

func (a App) renderHeader() string {
	chip := stModeChip.Render("NAVIGATE")
	var crumb string
	switch a.view {
	case viewOverview:
		crumb = stCyanB.Render("Overview")
	case viewTree:
		crumb = stDim.Render("Overview ") + stFainter.Render("/ ") + stCyanB.Render("tree") +
			breadcrumbTail(a.tree.cwd, a.root)
	case viewNeighborhood:
		crumb = stDim.Render("Overview ") + stFainter.Render("/ ") + stCyanB.Render(a.neigh.name)
	}
	left := chip + "  " + crumb
	right := stDim.Render(a.graph)
	gap := a.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	line := left + strings.Repeat(" ", gap) + right
	rule := stFainter.Render(strings.Repeat("─", a.width))
	return line + "\n" + rule
}

func (a App) renderStatus() string {
	var hints string
	switch a.view {
	case viewOverview:
		hints = key("↑↓", "move") + key("enter", "open") + key("t", "tree") + key("/", "search")
	case viewTree:
		hints = key("↑↓", "move") + key("→", "expand") + key("←", "collapse") + key("enter", "open symbol") + key("/", "search")
	case viewNeighborhood:
		hints = key("↑↓", "move") + key("enter", "re-center") + key("tab", "switch pane") + key("t", "tree")
	}
	mode := stStatusMode.Render(" NAVIGATE ")
	quit := key("q", "quit")
	line := mode + " " + hints
	gap := a.width - lipgloss.Width(line) - lipgloss.Width(quit) - 1
	if gap < 1 {
		gap = 1
	}
	return stStatusBar.Width(a.width).Render(line + strings.Repeat(" ", gap) + quit)
}

func (a App) renderLoading(h int) string {
	msg := fmt.Sprintf("%s  loading graph %s — this happens once per session",
		a.spinner.View(), stCyan.Render(a.graph))
	return center(msg, a.width, h)
}

func (a App) renderError(h int) string {
	msg := stErr.Render("✗ ") + stFg.Render(a.err.Error())
	return center(msg, a.width, h)
}

func (a App) overlaySearch(base string) string {
	panel := a.results.view(&a)
	box := stPanel.Width(min(a.width-4, 90)).Render(
		a.search.View() + "\n" + panel,
	)
	// Simple top-anchored overlay: draw the box near the top over the body.
	overlay := lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center, box,
		lipgloss.WithWhitespaceChars(" "))
	_ = base
	return overlay
}

func keyDelta(s string) int {
	if s == "up" {
		return -1
	}
	return 1
}

func key(k, label string) string {
	return stStatusKey.Render("<"+k+">") + " " + stDim.Render(label) + "  "
}

func breadcrumbTail(cwd, root string) string {
	rel := relPath(root, cwd)
	if rel == "" || rel == cwd {
		return ""
	}
	return stFainter.Render(" / ") + stDim.Render(rel)
}

func center(s string, w, h int) string {
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, s)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
