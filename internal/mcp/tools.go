package mcp

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Version is stamped into the client handshake and the --version flag. Release
// builds replace it with the tag version via -ldflags; source builds retain the
// current development default.
var Version = "0.1.0"

// ---- shared row shapes (as the engine emits them) -------------------------

// Symbol is a function/method node as returned by callers/callees.
type Symbol struct {
	NodeID   string `json:"node_id"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	Via      string `json:"via"`      // direct | indirect(context|fn-pointer|may_invoke)
	Resolved bool   `json:"resolved"` // false = unresolved function-pointer slot
}

// Hub is a centrality-ranked node from hubs.
type Hub struct {
	NodeID string   `json:"node_id"`
	Name   string   `json:"name"`
	Handle string   `json:"handle"`
	File   string   `json:"file"`
	Line   int      `json:"line"`
	FanIn  int      `json:"fan_in"`
	FanOut int      `json:"fan_out"`
	Degree int      `json:"degree"`
	Flags  []string `json:"flags"` // exported | dispatch_target | callback
}

// ---- hubs -----------------------------------------------------------------

type hubsResult struct {
	Move   string `json:"move"`
	Count  int    `json:"count"`
	Ranked []Hub  `json:"ranked"`
}

// Hubs returns the top-n highest-degree nodes: the subsystem spine.
func (c *Client) Hubs(n int) ([]Hub, error) {
	raw, err := c.Call("hubs", map[string]any{"n": n, "limit": n})
	if err != nil {
		return nil, err
	}
	var res hubsResult
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, err
	}
	return res.Ranked, nil
}

// ---- callers / callees ----------------------------------------------------

type callersResult struct {
	Callers []Symbol `json:"callers"`
	Of      string   `json:"of"`
}

type calleesResult struct {
	Callees []Symbol `json:"callees"`
	Of      string   `json:"of"`
}

// Callers returns who reaches this symbol (direct + indirect dispatch).
func (c *Client) Callers(name string, limit int) ([]Symbol, error) {
	raw, err := c.Call("callers", map[string]any{"name": name, "limit": limit})
	if err != nil {
		return nil, err
	}
	var res callersResult
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, err
	}
	return res.Callers, nil
}

// Callees returns what this symbol uses (in-repo, direct + indirect).
func (c *Client) Callees(name string, limit int) ([]Symbol, error) {
	raw, err := c.Call("callees", map[string]any{"name": name, "limit": limit})
	if err != nil {
		return nil, err
	}
	var res calleesResult
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, err
	}
	return res.Callees, nil
}

// ---- open_folder (L0 folder graph) ----------------------------------------

type folderNode struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"` // folder | file
	Label      string `json:"label"`
	Properties struct {
		File     string `json:"file"`
		Path     string `json:"path"`
		Basename string `json:"basename"`
	} `json:"properties"`
	Location struct {
		StartLine int `json:"start_line"`
	} `json:"location"`
}

type folderResult struct {
	Manifest struct {
		Root   string `json:"root"`
		Counts struct {
			Nodes int `json:"nodes"`
			Edges int `json:"edges"`
			Files int `json:"files"`
		} `json:"counts"`
	} `json:"manifest"`
	Nodes []folderNode `json:"nodes"`
	Edges []struct {
		Source string `json:"source"`
		Target string `json:"target"`
		Kind   string `json:"kind"`
	} `json:"edges"`
}

// FolderEntry is a flattened folder or file under a root.
type FolderEntry struct {
	IsDir bool
	Label string // basename
	Path  string // repo-relative path (folder path, or file path)
}

// OpenFolder lists the folders and files directly under root, sorted
// dirs-first then alphabetically. Repo-relative paths where the engine
// provides them.
func (c *Client) OpenFolder(root string) ([]FolderEntry, error) {
	raw, err := c.Call("open_folder", map[string]any{"root": root})
	if err != nil {
		return nil, err
	}
	var res folderResult
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, err
	}

	// The engine returns the entire subtree. Reduce it to the direct children
	// of the requested folder; for the repository root, keep nodes with no
	// folder parent. DECLARES edges are intentionally ignored here.
	rootID := "nav:folder:" + strings.Trim(strings.ReplaceAll(root, "\\", "/"), "/")
	parentOf := map[string]string{}
	for _, e := range res.Edges {
		if e.Kind == "CONTAINS" {
			parentOf[e.Target] = e.Source
		}
	}

	var out []FolderEntry
	for _, n := range res.Nodes {
		parent, hasParent := parentOf[n.ID]
		if root == "" {
			if hasParent {
				continue
			}
		} else if parent != rootID {
			continue
		}
		switch n.Kind {
		case "file":
			out = append(out, FolderEntry{IsDir: false, Label: n.Label, Path: n.Properties.File})
		case "folder":
			if n.Properties.Path != "" {
				out = append(out, FolderEntry{IsDir: true, Label: n.Properties.Basename, Path: n.Properties.Path})
			}
		}
	}
	return out, nil
}

// ---- open_file (L1 file graph: declarations) ------------------------------

type fileGraphNode struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"` // function | method | type | import | ...
	Label      string `json:"label"`
	Properties struct {
		File      string `json:"file"`
		Name      string `json:"name"`
		Specifier string `json:"specifier"`
	} `json:"properties"`
	Location struct {
		StartLine int `json:"start_line"`
		EndLine   int `json:"end_line"`
	} `json:"location"`
}

type fileGraphResult struct {
	Nodes []fileGraphNode `json:"nodes"`
}

// Decl is one top-level declaration in a file (the symbol outline).
type Decl struct {
	NodeID    string
	Kind      string
	Name      string
	StartLine int
}

// OpenFile returns the declaration outline for a repo-relative file path.
func (c *Client) OpenFile(file string) ([]Decl, []string, error) {
	raw, err := c.Call("open_file", map[string]any{"file": file})
	if err != nil {
		return nil, nil, err
	}
	var res fileGraphResult
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, nil, err
	}
	var out []Decl
	var imports []string
	for _, n := range res.Nodes {
		switch n.Kind {
		case "function", "method", "type", "struct", "class", "enum":
			name := n.Properties.Name
			if name == "" {
				name = n.Label
			}
			out = append(out, Decl{
				NodeID:    n.ID,
				Kind:      n.Kind,
				Name:      name,
				StartLine: n.Location.StartLine,
			})
		case "module", "file":
			if n.Properties.Specifier != "" {
				imports = append(imports, n.Properties.Specifier)
			}
		}
	}
	return out, imports, nil
}

// ---- read_body ------------------------------------------------------------

type readBodyResult struct {
	NodeID    string `json:"node_id"`
	Name      string `json:"name"`
	File      string `json:"file"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Source    string `json:"body"` // the engine names the source span "body"
	Via       string `json:"via"`  // offsets | body_nodes
	Truncated bool   `json:"truncated"`
}

// Body is a function's source span.
type Body struct {
	Name      string
	File      string
	StartLine int
	EndLine   int
	Source    string
	Truncated bool
}

// ReadBody returns the source of a symbol by name.
func (c *Client) ReadBody(name string, maxChars int) (Body, error) {
	raw, err := c.Call("read_body", map[string]any{"name": name, "max_chars": maxChars})
	if err != nil {
		return Body{}, err
	}
	var res readBodyResult
	if err := json.Unmarshal(raw, &res); err != nil {
		return Body{}, err
	}
	return Body{
		Name:      res.Name,
		File:      res.File,
		StartLine: res.StartLine,
		EndLine:   res.EndLine,
		Source:    res.Source,
		Truncated: res.Truncated,
	}, nil
}

// NavigationFacts are the compact graph-backed counters shown beside a
// neighborhood's source preview.
type NavigationFacts struct {
	Reaches  int
	Guards   int
	PointsTo int
}

// Facts derives the three counters used by the navigation design. A failure is
// returned per fact so the UI can mark missing graph capability without hiding
// the caller/callee neighborhood that did load successfully.
func (c *Client) Facts(name string) (NavigationFacts, map[string]error) {
	facts := NavigationFacts{}
	errs := map[string]error{}

	if raw, err := c.Call("flow", map[string]any{"seed": name, "limit": 200}); err != nil {
		errs["reaches"] = err
	} else {
		var res struct {
			Error    string `json:"error"`
			Manifest struct {
				Reached int `json:"reached"`
			} `json:"manifest"`
		}
		if err := json.Unmarshal(raw, &res); err != nil {
			errs["reaches"] = err
		} else if res.Error != "" {
			errs["reaches"] = fmt.Errorf("%s", res.Error)
		} else {
			facts.Reaches = res.Manifest.Reached
		}
	}

	if raw, err := c.Call("guards", map[string]any{"fn": name}); err != nil {
		errs["guards"] = err
	} else {
		var res struct {
			Error string `json:"error"`
			Guard struct {
				Conditions    int `json:"conditions"`
				ShortCircuits int `json:"short_circuits"`
				Throws        int `json:"throws"`
			} `json:"guard_signal"`
		}
		if err := json.Unmarshal(raw, &res); err != nil {
			errs["guards"] = err
		} else if res.Error != "" {
			errs["guards"] = fmt.Errorf("%s", res.Error)
		} else {
			facts.Guards = res.Guard.Conditions + res.Guard.ShortCircuits + res.Guard.Throws
		}
	}

	if raw, err := c.Call("points_to", map[string]any{"value": name}); err != nil {
		errs["points-to"] = err
	} else {
		var res struct {
			Error string            `json:"error"`
			Nodes []json.RawMessage `json:"nodes"`
		}
		if err := json.Unmarshal(raw, &res); err != nil {
			errs["points-to"] = err
		} else if res.Error != "" {
			errs["points-to"] = fmt.Errorf("%s", res.Error)
		} else if len(res.Nodes) > 0 {
			facts.PointsTo = len(res.Nodes) - 1
		}
	}
	return facts, errs
}

// ---- search ---------------------------------------------------------------

type searchResult struct {
	Total   int  `json:"total"`
	HasMore bool `json:"has_more"`
	Hits    []struct {
		NodeID    string `json:"node_id"`
		Name      string `json:"name"`
		Kind      string `json:"kind"`
		File      string `json:"file"`
		Line      int    `json:"line"`
		Exported  bool   `json:"exported"`
		Signature string `json:"signature"`
	} `json:"hits"`
}

// Match is a search hit.
type Match struct {
	NodeID    string
	Name      string
	File      string
	Line      int
	Kind      string
	Exported  bool
	Signature string
}

// Search resolves a fuzzy name to candidate symbols. Returns the hits plus the
// true total (search pages), so the UI can show "showing N of M".
func (c *Client) Search(name string, limit int) ([]Match, int, error) {
	raw, err := c.Call("search", map[string]any{"name": name, "limit": limit})
	if err != nil {
		return nil, 0, err
	}
	var res searchResult
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, 0, err
	}
	out := make([]Match, 0, len(res.Hits))
	for _, m := range res.Hits {
		out = append(out, Match{
			NodeID: m.NodeID, Name: m.Name, File: m.File, Line: m.Line,
			Kind: m.Kind, Exported: m.Exported, Signature: m.Signature,
		})
	}
	return out, res.Total, nil
}
