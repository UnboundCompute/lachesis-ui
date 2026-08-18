// Package subsystem turns raw file paths into the human-centric grouping the
// UI navigates by. The code graph has no notion of a "subsystem" — that is a
// developer's mental model, so we derive it here from directory structure and
// keep it entirely UI-side. This is deliberately a heuristic, not ground truth;
// the UI labels it as such.
package subsystem

import (
	"path"
	"sort"
	"strings"
)

// Of returns the subsystem label for a repo-relative (or absolute) file path.
// The label is the most meaningful directory segment: for a conventional tree
// the top-level package/dir ("lib", "src/http", "planner") reads as the
// subsystem. We collapse ubiquitous container dirs (src, lib, pkg, internal) to
// their child so "src/http/..." groups as "http", not "src".
func Of(file string) string {
	p := normalize(file)
	if p == "" {
		return "(root)"
	}
	segs := strings.Split(p, "/")
	if len(segs) == 1 {
		return "(root)" // a top-level file
	}
	i := 0
	for i < len(segs)-1 && isContainer(segs[i]) {
		i++
	}
	if i >= len(segs)-1 {
		// Everything up to the file was a container dir; use the last dir.
		return segs[len(segs)-2]
	}
	return segs[i]
}

// Module returns a finer-grained label than Of: the module a symbol lives in.
// For a flat source root (curl's lib/http.c, lib/ftp.c) each translation unit is
// its own module, so we key on the file stem ("http", "ftp"); for a nested tree
// (lib/vtls/openssl.c, pkg/api/routes.py) the enclosing directory is the module
// ("vtls", "api"). This is what makes "who calls this" read as "30 from HTTP,
// 25 from FTP" instead of one undifferentiated pile.
func Module(file string) string {
	p := normalize(file)
	if p == "" {
		return "(root)"
	}
	segs := strings.Split(p, "/")
	if len(segs) == 1 {
		return stem(segs[0]) // a top-level file is its own module
	}
	parent := segs[len(segs)-2]
	if isFlatRoot(parent) {
		return stem(segs[len(segs)-1]) // flat area: the file is the module
	}
	return parent
}

// stem strips the directory-less filename down to its base identity: no
// extension, and no leading path noise.
func stem(name string) string {
	base := name
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}
	if i := strings.LastIndex(base, "."); i > 0 {
		base = base[:i]
	}
	return base
}

// isFlatRoot reports whether a directory conventionally holds a flat pile of
// modules (so the file, not the dir, names the module).
func isFlatRoot(seg string) bool {
	switch seg {
	case "lib", "src", "pkg", "source", "sources":
		return true
	}
	return false
}

// Group buckets items by subsystem, preserving input order within each bucket
// and returning buckets sorted by descending size then name.
type Bucket[T any] struct {
	Label string
	Items []T
}

// GroupBy buckets items by a subsystem label (coarse: Of) derived from each
// item's path.
func GroupBy[T any](items []T, pathOf func(T) string) []Bucket[T] {
	return groupWith(items, pathOf, Of)
}

// GroupByModule buckets items by module (fine: Module) — the right grain for a
// symbol's neighborhood, where "who calls this" wants per-module counts.
func GroupByModule[T any](items []T, pathOf func(T) string) []Bucket[T] {
	return groupWith(items, pathOf, Module)
}

func groupWith[T any](items []T, pathOf func(T) string, label func(string) string) []Bucket[T] {
	order := []string{}
	byLabel := map[string][]T{}
	for _, it := range items {
		lbl := label(pathOf(it))
		if _, seen := byLabel[lbl]; !seen {
			order = append(order, lbl)
		}
		byLabel[lbl] = append(byLabel[lbl], it)
	}
	out := make([]Bucket[T], 0, len(order))
	for _, lbl := range order {
		out = append(out, Bucket[T]{Label: lbl, Items: byLabel[lbl]})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if len(out[i].Items) != len(out[j].Items) {
			return len(out[i].Items) > len(out[j].Items)
		}
		return out[i].Label < out[j].Label
	})
	return out
}

// Describe returns a short plain-language gloss for a subsystem label when we
// recognize a conventional name, else "". These are hints, never asserted facts.
func Describe(label string) string {
	switch strings.ToLower(label) {
	case "lib", "src":
		return "core library"
	case "cli", "cmd":
		return "command-line entry points"
	case "internal":
		return "internal packages"
	case "test", "tests":
		return "test suite"
	case "vquic", "vtls", "vssh":
		return "pluggable backends"
	case "planner":
		return "finding ranking"
	case "nav":
		return "graph navigation"
	case "core":
		return "engine core"
	case "frontends", "frontend":
		return "language parsers"
	}
	return ""
}

var containerDirs = map[string]bool{
	"src": true, "lib": false, "pkg": true, "internal": false,
}

// isContainer reports whether a dir is a generic container we skip past so the
// child dir names the subsystem. "lib" and "internal" are intentionally kept —
// in C ("lib/...") and Go ("internal/...") they are meaningful roots.
func isContainer(seg string) bool { return containerDirs[seg] }

func normalize(file string) string {
	p := strings.TrimSpace(file)
	p = strings.ReplaceAll(p, "\\", "/")
	// Absolute paths from some tools; keep only the portion after a repo-ish
	// marker if present, else the cleaned path's tail is still grouped sanely
	// by its own dirs.
	p = path.Clean(p)
	p = strings.TrimPrefix(p, "/")
	return p
}
