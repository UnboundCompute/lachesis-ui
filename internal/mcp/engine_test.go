package mcp

import (
	"os"
	"path/filepath"
	"testing"
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
