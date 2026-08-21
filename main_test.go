package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveGraphRejectsRegularFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "partial.kuzu")
	if err := os.WriteFile(path, []byte("partial"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, err := resolveGraph(path, nil)
	if err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("resolveGraph() error = %v, want a directory diagnostic", err)
	}
}
