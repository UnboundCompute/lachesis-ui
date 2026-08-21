// Command lachesis-ui is a persistent, keyboard-driven terminal UI over the
// lachesis code-property-graph engine. It spawns the engine over MCP and
// navigates the graph the way a developer reads a codebase.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/UnboundCompute/lachesis-ui/internal/mcp"
	"github.com/UnboundCompute/lachesis-ui/internal/ui"
)

func main() {
	var (
		graphFlag  = flag.String("graph", "", "path to a prebuilt .kuzu graph store (default: newest in ~/.lachesis/graphs)")
		pythonFlag = flag.String("python", "", "interpreter that can run the engine (default: auto-discover)")
		listFlag   = flag.Bool("list", false, "list discovered graphs and exit")
		versionF   = flag.Bool("version", false, "print version and exit")
	)
	flag.Usage = usage
	flag.Parse()

	if *versionF {
		fmt.Printf("lachesis-ui %s\n", mcp.Version)
		return
	}

	if *listFlag {
		listGraphs()
		return
	}

	graphPath, graphName, err := resolveGraph(*graphFlag, flag.Args())
	if err != nil {
		fatal(err.Error())
	}

	python := *pythonFlag
	if python == "" {
		python = mcp.DiscoverPython()
	}

	fmt.Fprintf(os.Stderr, "lachesis-ui: loading %s via %s …\n", graphName, python)
	client, err := mcp.Spawn(python, graphPath)
	if err != nil {
		fatal(fmt.Sprintf("could not start the engine: %v\n\n"+
			"The UI needs the lachesis engine (Python). Install it, or point at it:\n"+
			"  pip install lachesis-cpg   # or your project's install\n"+
			"  lachesis-ui --python /path/to/python\n"+
			"  LACHESIS_PYTHON=/path/to/python lachesis-ui", err))
	}
	defer client.Close()

	p := tea.NewProgram(ui.New(client, graphName), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fatal(err.Error())
	}
}

// resolveGraph picks the graph store: explicit --graph, a positional path, or
// the newest one in the graphs dir.
func resolveGraph(flagVal string, args []string) (path, name string, err error) {
	candidate := flagVal
	if candidate == "" && len(args) > 0 {
		candidate = args[0]
	}
	if candidate != "" {
		abs, aerr := filepath.Abs(candidate)
		if aerr != nil {
			return "", "", aerr
		}
		if _, serr := os.Stat(abs); serr != nil {
			return "", "", fmt.Errorf("graph not found: %s", candidate)
		}
		return abs, graphNameOf(abs), nil
	}
	graphs := mcp.ListGraphs()
	if len(graphs) == 0 {
		return "", "", fmt.Errorf("no graphs found in %s\n\nBuild one first:\n  lachesis index <source_dir>\nor pass a path:\n  lachesis-ui --graph <path.kuzu>", mcp.GraphsDir())
	}
	return graphs[0].Path, graphs[0].Name, nil
}

func graphNameOf(p string) string {
	base := filepath.Base(p)
	base = strings.TrimSuffix(base, ".kuzu")
	return base
}

func listGraphs() {
	graphs := mcp.ListGraphs()
	if len(graphs) == 0 {
		fmt.Printf("no graphs in %s\n", mcp.GraphsDir())
		return
	}
	fmt.Printf("graphs in %s:\n", mcp.GraphsDir())
	for _, g := range graphs {
		fmt.Printf("  %-24s %s\n", g.Name, g.Path)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `lachesis-ui — a terminal UI for navigating a code graph

Usage:
  lachesis-ui [flags] [graph.kuzu]

Flags:
  --graph PATH     prebuilt .kuzu store (default: newest in ~/.lachesis/graphs)
  --python PATH    interpreter that runs the engine (default: auto-discover)
  --list           list discovered graphs and exit
  --version        print version and exit

Keys (in-app):
  1/o overview   2/t tree   3 neighborhood   / search   q quit
`)
}

func fatal(msg string) {
	fmt.Fprintf(os.Stderr, "lachesis-ui: %s\n", msg)
	os.Exit(1)
}
