package mcp

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// DiscoverPython finds an interpreter that can import the lachesis engine.
//
// Order: explicit override ($LACHESIS_PYTHON), the standard venv the engine
// installs into (~/.lachesis/venv), then whatever `python3`/`python` is on
// PATH. We do not verify the import here — Spawn surfaces a clear error if the
// chosen interpreter cannot serve.
func DiscoverPython() string {
	if p := os.Getenv("LACHESIS_PYTHON"); p != "" {
		return p
	}
	installRoot := os.Getenv("LACHESIS_HOME")
	if installRoot == "" {
		if home, err := os.UserHomeDir(); err == nil {
			installRoot = filepath.Join(home, ".lachesis")
		}
	}
	if installRoot != "" {
		venv := filepath.Join(installRoot, "venv", "bin", "python")
		if fileExists(venv) {
			return venv
		}
	}
	for _, name := range []string{"python3", "python"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return "python3"
}

// GraphsDir is where the engine writes prebuilt graph stores.
func GraphsDir() string {
	if d := os.Getenv("LACHESIS_GRAPHS_DIR"); d != "" {
		return d
	}
	if root := os.Getenv("LACHESIS_HOME"); root != "" {
		return filepath.Join(root, "graphs")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".lachesis/graphs"
	}
	return filepath.Join(home, ".lachesis", "graphs")
}

// Graph is a discovered prebuilt graph store.
type Graph struct {
	Name string // display name, e.g. "curl"
	Path string // absolute path to the .kuzu store
}

// ListGraphs enumerates *.kuzu stores in the graphs dir, newest first.
// Enriched overlays (*.kuzu.enriched) are folded out; only base stores show.
func ListGraphs() []Graph {
	dir := GraphsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	type withTime struct {
		g   Graph
		mod int64
	}
	var found []withTime
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".kuzu") {
			continue // skips ".kuzu.enriched" and everything else
		}
		if !e.IsDir() {
			continue // ignore partial files and other non-store artifacts
		}
		info, err := e.Info()
		var mod int64
		if err == nil {
			mod = info.ModTime().UnixNano()
		}
		found = append(found, withTime{
			g:   Graph{Name: strings.TrimSuffix(name, ".kuzu"), Path: filepath.Join(dir, name)},
			mod: mod,
		})
	}
	sort.Slice(found, func(i, j int) bool { return found[i].mod > found[j].mod })
	out := make([]Graph, len(found))
	for i, f := range found {
		out[i] = f.g
	}
	return out
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
