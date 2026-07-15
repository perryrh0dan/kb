//go:build integration

package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func buildBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "kb")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	return bin
}

func TestIngestAndSearch(t *testing.T) {
	if os.Getenv("KB_OPENAI_API_KEY") == "" {
		t.Skip("KB_OPENAI_API_KEY not set")
	}
	bin := buildBinary(t)
	dir := t.TempDir()

	// Write a test document
	doc := filepath.Join(dir, "test.md")
	os.WriteFile(doc, []byte("# Kubernetes\n\nKubernetes is a container orchestration platform."), 0644)

	dbPath := filepath.Join(t.TempDir(), "test.db")

	// Ingest
	cmd := exec.Command(bin, "ingest", "file", dir)
	cmd.Env = append(os.Environ(), "KB_DB_PATH="+dbPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ingest failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "ingested=1") {
		t.Errorf("expected ingested=1 in output: %s", out)
	}

	// Search
	cmd = exec.Command(bin, "search", "container orchestration")
	cmd.Env = append(os.Environ(), "KB_DB_PATH="+dbPath)
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("search failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "Kubernetes") {
		t.Errorf("expected Kubernetes in search results: %s", out)
	}
}
