package mcp

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGraphsDirHonorsInstallRoot(t *testing.T) {
	t.Setenv("LACHESIS_GRAPHS_DIR", "")
	t.Setenv("LACHESIS_HOME", "/tmp/lachesis-install")
	if got, want := GraphsDir(), filepath.Join("/tmp/lachesis-install", "graphs"); got != want {
		t.Fatalf("GraphsDir() = %q, want %q", got, want)
	}
}

func TestGraphsDirExplicitOverrideWins(t *testing.T) {
	t.Setenv("LACHESIS_HOME", "/tmp/lachesis-install")
	t.Setenv("LACHESIS_GRAPHS_DIR", "/tmp/custom-graphs")
	if got, want := GraphsDir(), "/tmp/custom-graphs"; got != want {
		t.Fatalf("GraphsDir() = %q, want %q", got, want)
	}
}

func TestDiscoverPythonHonorsInstallRoot(t *testing.T) {
	root := t.TempDir()
	venvPython := filepath.Join(root, "venv", "bin", "python")
	if err := os.MkdirAll(filepath.Dir(venvPython), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(venvPython, []byte(""), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LACHESIS_PYTHON", "")
	t.Setenv("LACHESIS_HOME", root)
	if got := DiscoverPython(); got != venvPython {
		t.Fatalf("DiscoverPython() = %q, want %q", got, venvPython)
	}
}

func TestListGraphsSkipsNonStoreArtifacts(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "valid.kuzu"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "partial.kuzu"), []byte("incomplete"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LACHESIS_GRAPHS_DIR", root)
	graphs := ListGraphs()
	if len(graphs) != 1 || graphs[0].Name != "valid" {
		t.Fatalf("ListGraphs() = %#v, want only valid.kuzu", graphs)
	}
}

func TestStartupTimeoutIsConfigurable(t *testing.T) {
	t.Setenv("LACHESIS_UI_STARTUP_TIMEOUT", "17s")
	if got, want := startupTimeout(), 17*time.Second; got != want {
		t.Fatalf("startupTimeout() = %s, want %s", got, want)
	}
	t.Setenv("LACHESIS_UI_STARTUP_TIMEOUT", "not-a-duration")
	if got := startupTimeout(); got != defaultStartupTimeout {
		t.Fatalf("invalid timeout = %s, want default %s", got, defaultStartupTimeout)
	}
}
