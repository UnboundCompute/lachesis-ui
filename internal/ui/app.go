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
	"github.com/charmbracelet/x/ansi"

	"github.com/UnboundCompute/lachesis-ui/internal/mcp"
)

type view int

const (
	viewOverview view = iota
	viewTree
	viewNeighborhood
	viewFindings
	viewFindingDetail
	viewReaches
	viewSkeleton
	viewToolResult
	viewScan
	viewHubs
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

	view       view
	returnView view
	overview   overviewModel
	tree       treeModel
	neigh      neighModel
	neighInit  bool // whether the neighborhood has ever been loaded
	findings   findingsModel
	toolName   string
	toolBody   string
	toolErr    error
	scanData   map[string]any

	searching  bool
	search     textinput.Model
	results    searchModel
	help       bool
	palette    bool
	paletteSel int
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
		findings: newFindings(),
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
		a.neigh.pushHistory(msg.name)
		a.neigh.beginLoad(msg.name)
		return a, loadNeighborhoodCmd(a.client, msg.name, a.root)
	case gotoSkeletonMsg:
		a.findings.active = msg.candidate
		a.findings.skeleton = nil
		a.view = viewSkeleton
		return a, loadSkeletonCmd(a.client, msg.candidate)

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
		if next := a.tree.onFolder(msg); next != "" {
			return a, loadFolderCmd(a.client, next)
		}
		return a, a.tree.maybeLoadOutline(&a)
	case outlineLoadedMsg:
		a.tree.onOutline(msg)
		return a, nil
	case sourceLoadedMsg:
		a.tree.onSource(msg)
		return a, nil
	case neighborhoodLoadedMsg:
		a.neigh.onLoaded(msg)
		return a, nil
	case searchResultsMsg:
		a.results.onResults(msg)
		return a, nil
	case candidatesLoadedMsg:
		a.findings.rows = msg.rows
		a.view = viewFindings
		a.ready = true
		return a, nil
	case candidateDetailLoadedMsg:
		a.findings.active, a.findings.detail, a.view = msg.candidate, msg.data, viewFindingDetail
		return a, nil
	case reachesLoadedMsg:
		a.findings.active, a.findings.path, a.view = msg.candidate, msg.data, viewReaches
		return a, nil
	case skeletonLoadedMsg:
		a.findings.active, a.findings.skeleton, a.view = msg.candidate, msg.data, viewSkeleton
		return a, nil
	case toolResultMsg:
		if msg.name == "review" {
			if msg.err != nil {
				a.statusHint = "review could not be saved: " + msg.err.Error()
			} else {
				a.statusHint = "review decision saved for this session"
			}
			return a, nil
		}
		a.toolName, a.toolBody, a.toolErr, a.view = msg.name, msg.body, msg.err, viewToolResult
		return a, nil
	case scanLoadedMsg:
		a.scanData, a.view = msg.data, viewScan
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
				a.returnView = a.view
				return a, func() tea.Msg { return gotoNeighborhoodMsg{name: sel.Name} }
			}
			return a, nil
		}
		// ctrl+enter / right-arrow accept too; otherwise feed the text box.
		if msg.String() == "right" {
			if sel, ok := a.results.selected(); ok {
				a.searching = false
				a.search.Blur()
				a.returnView = a.view
				return a, func() tea.Msg { return gotoNeighborhoodMsg{name: sel.Name} }
			}
		}
		var cmd tea.Cmd
		a.search, cmd = a.search.Update(msg)
		return a, cmd
	}
	if a.help {
		if msg.String() == "?" || msg.String() == "esc" {
			a.help = false
		}
		return a, nil
	}
	if a.palette {
		switch msg.String() {
		case "esc":
			a.palette = false
			return a, nil
		case "up", "k":
			if a.paletteSel > 0 {
				a.paletteSel--
			}
			return a, nil
		case "down", "j":
			if a.paletteSel < 5 {
				a.paletteSel++
			}
			return a, nil
		case "enter":
			a.palette = false
			if a.paletteSel == 0 {
				if a.findings.active.Source == "" || a.findings.active.Sink == "" {
					a.statusHint = "choose an evidence row first"
					return a, nil
				}
				a.view = viewReaches
				return a, loadReachesCmd(a.client, a.findings.active)
			}
			if a.paletteSel == 1 {
				if a.findings.active.Sink == "" {
					a.statusHint = "choose an evidence row first"
					return a, nil
				}
				a.returnView = a.view
				return a, loadToolCmd(a.client, "sources_of", map[string]any{"sink": a.findings.active.Sink})
			}
			if a.paletteSel == 2 {
				if a.neigh.name == "" {
					a.statusHint = "open a symbol first, then flow from its neighborhood"
					return a, nil
				}
				a.returnView = a.view
				return a, loadToolCmd(a.client, "flow", map[string]any{"seed": a.neigh.name, "limit": 200})
			}
			if a.paletteSel == 3 {
				a.returnView = a.view
				return a, loadScanCmd(a.client)
			}
			if a.paletteSel == 4 {
				a.returnView = a.view
				return a, loadCandidatesCmd(a.client)
			}
			a.statusHint = "graph switching is not exposed by the current client session"
			return a, nil
		}
		return a, nil
	}
	if a.statusHint != "" {
		a.statusHint = ""
	}

	switch msg.String() {
	case "?":
		a.help = true
		return a, nil
	case ":":
		a.palette = true
		a.paletteSel = 0
		return a, nil
	case "f":
		a.returnView = a.view
		return a, loadScanCmd(a.client)
	case "h":
		a.returnView = a.view
		a.view = viewHubs
		return a, nil
	case "tab":
		a.returnView = a.view
		return a, loadCandidatesCmd(a.client)
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
	case "esc":
		if a.view == viewFindingDetail || a.view == viewReaches || a.view == viewSkeleton {
			a.view = viewFindings
			return a, nil
		}
		if a.view == viewFindings {
			a.view = a.returnView
			return a, nil
		}
		if a.view == viewScan || a.view == viewToolResult {
			a.view = a.returnView
			return a, nil
		}
		if a.view == viewNeighborhood && a.returnView != viewOverview {
			a.view = a.returnView
			a.err = nil
			return a, nil
		}
		if a.view != viewOverview {
			a.view = viewOverview
			a.err = nil
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
		if msg.String() == "r" || msg.String() == "s" {
			a.findings.active = mcp.Candidate{Entrypoint: a.neigh.name, Source: a.neigh.name, Sink: a.neigh.name}
			a.view = viewSkeleton
			return a, loadSkeletonCmd(a.client, a.findings.active)
		}
		return a, a.neigh.update(&a, msg)
	case viewFindings:
		return a, a.findings.update(&a, msg)
	case viewFindingDetail:
		switch msg.String() {
		case "o", "c", "e":
			name := or(a.findings.active.Entrypoint, a.findings.active.Source)
			if name == "" {
				a.statusHint = "this evidence row has no source symbol"
				return a, nil
			}
			a.returnView = viewFindingDetail
			a.view = viewNeighborhood
			a.neighInit = true
			a.neigh.pushHistory(name)
			a.neigh.beginLoad(name)
			return a, loadNeighborhoodCmd(a.client, name, a.root)
		case "r":
			a.view = viewReaches
			return a, loadReachesCmd(a.client, a.findings.active)
		case "s":
			a.view = viewSkeleton
			return a, loadSkeletonCmd(a.client, a.findings.active)
		case "k":
			if a.findings.active.ID == "" {
				a.statusHint = "this evidence row has no review id"
				return a, nil
			}
			a.statusHint = "saving review decision for this session"
			return a, reviewCandidateCmd(a.client, a.findings.active.ID, "confirmed")
		}
	case viewReaches:
		if msg.String() == "y" {
			a.statusHint = "path copying is not exposed by the current terminal client"
			return a, nil
		}
		if msg.String() == "s" {
			a.view = viewSkeleton
			return a, loadSkeletonCmd(a.client, a.findings.active)
		}
	case viewToolResult:
		if msg.String() == "esc" {
			a.view = a.returnView
		}
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
		case viewFindings:
			body = a.findings.list(&a, bodyHeight)
		case viewFindingDetail:
			body = a.findings.detailView(&a, bodyHeight)
		case viewReaches:
			body = a.findings.reachesView(&a, bodyHeight)
		case viewSkeleton:
			body = a.findings.skeletonView(&a, bodyHeight)
		case viewToolResult:
			body = a.toolView(bodyHeight)
		case viewScan:
			body = scanView(a.scanData, a.width, bodyHeight)
		case viewHubs:
			body = hubsView(a.overview.hubs, a.width, bodyHeight)
		}
	}
	body = lipgloss.NewStyle().Height(bodyHeight).MaxHeight(bodyHeight).Render(body)

	out := lipgloss.JoinVertical(lipgloss.Left, header, body, status)
	if a.searching {
		out = a.overlaySearch(out)
	}
	if a.help {
		out = a.overlayHelp(out)
	}
	if a.palette {
		out = a.overlayPalette(out)
	}
	return stApp.Width(a.width).Height(a.height).Render(out)
}

func (a App) renderHeader() string {
	chip := stModeChip.Render("NAVIGATE")
	var left, right string
	switch a.view {
	case viewOverview:
		left = chip + "  " + stCyanB.Render("Overview") + "  " +
			stFg.Render("a code-property-graph, organized the way a developer reads it")
		right = stDim.Render(a.graph)
	case viewTree:
		left = chip + "  " + stDim.Render("Overview") + " " + stFainter.Render("/") + " " + stCyanB.Render("tree")
		right = stFainter.Render("</> filter files")
	case viewNeighborhood:
		left = chip + "  " + stDim.Render("Overview") + " " + stFainter.Render("/") + " " + stCyanB.Render(a.neigh.name)
		if a.neigh.hasBody {
			left += "  " + stBlue.Render(relHandle(a.root, a.neigh.body.File, a.neigh.body.StartLine))
		}
		right = stDim.Render("called from ") + stFg.Render(fmt.Sprintf("%d", len(a.neigh.callers))) +
			stDim.Render(" · calls ") + stFg.Render(fmt.Sprintf("%d", len(a.neigh.callees))) +
			"  " + stFainter.Render("<[> back  <]> fwd")
	default:
		left = chip + "  " + stCyanB.Render("Findings")
		right = stDim.Render("evidence review")
	}
	left = "  " + left
	right += "  "
	availableLeft := a.width - lipgloss.Width(right) - 1
	if availableLeft < 1 {
		availableLeft = 1
	}
	left = ansi.Truncate(left, availableLeft, "…")
	gap := a.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	line := left + strings.Repeat(" ", gap) + right
	rule := lipgloss.NewStyle().Foreground(colRule).Render(strings.Repeat("─", a.width))
	return lipgloss.NewStyle().Height(2).Render(line + "\n" + rule)
}

func (a App) renderStatus() string {
	if a.statusHint != "" {
		return stStatusBar.Width(a.width).Render(stAmber.Render(" "+a.statusHint) + stDim.Render("  · press esc to dismiss"))
	}
	var hints string
	switch a.view {
	case viewOverview:
		hints = key("enter", "open area") + key("t", "files") + key("/", "find symbol")
	case viewTree:
		hints = key("↑↓", "move/scroll") + key("→", "expand/select symbol") + key("b", "full source") + key("enter", "see symbol map")
	case viewNeighborhood:
		hints = key("enter", "open selected") + key("b", "full body/preview") + key("↑↓", "move/scroll") + key("tab", "switch side") + key("[ ]", "back/forward")
	case viewFindings, viewFindingDetail, viewReaches, viewSkeleton, viewToolResult, viewScan, viewHubs:
		hints = key("enter", "inspect") + key("r", "witness path") + key("s", "skeleton") + key("esc", "back")
	}
	label := "NAVIGATE"
	if a.view == viewTree {
		label = "TREE"
	} else if a.view == viewNeighborhood {
		label = "SYMBOL MAP"
	} else if a.view >= viewFindings {
		label = "FINDINGS"
	}
	mode := stStatusMode.Render(" " + label + " ")
	quit := stDim.Render("<esc> overview  <?> help ")
	if a.view == viewOverview {
		quit = key("q", "quit") + key("?", "help")
	}
	line := mode + " " + hints
	availableLine := a.width - lipgloss.Width(quit) - 1
	if availableLine < 1 {
		availableLine = 1
	}
	line = ansi.Truncate(line, availableLine, "…")
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

func (a App) overlayHelp(base string) string {
	var b strings.Builder
	fmt.Fprintln(&b, stBright.Render("How to explore this code"))
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, stCyanB.Render("↑ ↓ / j k")+"  "+stFg.Render("move through the current list"))
	fmt.Fprintln(&b, stCyanB.Render("enter")+"     "+stFg.Render("open the selected area, file, or symbol"))
	fmt.Fprintln(&b, stCyanB.Render("tab")+"       "+stFg.Render("switch between the two side lists"))
	fmt.Fprintln(&b, stCyanB.Render("b")+"         "+stFg.Render("toggle the full source/body view"))
	fmt.Fprintln(&b, stCyanB.Render("t")+"         "+stFg.Render("open the source tree"))
	fmt.Fprintln(&b, stCyanB.Render("/")+"         "+stFg.Render("find a symbol by name"))
	fmt.Fprintln(&b, stCyanB.Render("f")+"         "+stFg.Render("run a graph scan"))
	fmt.Fprintln(&b, stCyanB.Render("esc")+"       "+stFg.Render("return to the overview"))
	fmt.Fprintln(&b, stCyanB.Render("?")+"         "+stFg.Render("close this help"))
	box := stPanel.Width(min(a.width-6, 72)).Render(b.String())
	_ = base
	return lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center, box, lipgloss.WithWhitespaceChars(" "))
}

func (a App) overlayPalette(base string) string {
	items := []string{"reaches        witness path from a source to a sink", "sources_of     reverse cone into a sink", "flow           forward cone from a value", "scan           rank questions to investigate", "candidates     evidence to review", "load_graph     switch the loaded graph"}
	var b strings.Builder
	fmt.Fprintln(&b, stCyanB.Render(":")+" "+stBright.Render("command palette"))
	for i, item := range items {
		prefix := "  "
		if i == a.paletteSel {
			prefix = selRule(true)
		}
		fmt.Fprintln(&b, prefix+stFg.Render(item))
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, stDim.Render("↑↓ select   enter run   esc close"))
	box := stPanel.Width(min(a.width-8, 72)).Render(b.String())
	_ = base
	return lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center, box, lipgloss.WithWhitespaceChars(" "))
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
