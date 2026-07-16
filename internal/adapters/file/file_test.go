package file_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/user/kb/internal/adapters"
	"github.com/user/kb/internal/adapters/file"
)

func TestFileAdapterScanFindsMarkdown(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.md"), []byte("# Hello\nworld"), 0644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("plain text"), 0644)
	os.WriteFile(filepath.Join(dir, "c.go"), []byte("package main"), 0644)

	src := file.New(dir, false, []string{"md", "txt"}, file.Options{})
	ch, err := src.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	var metas []adapters.DocumentMeta
	for m := range ch {
		metas = append(metas, m)
	}
	if len(metas) != 2 {
		t.Errorf("got %d metas, want 2", len(metas))
	}
}

func TestFileAdapterScanRecursive(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	os.MkdirAll(sub, 0755)
	os.WriteFile(filepath.Join(dir, "root.md"), []byte("root"), 0644)
	os.WriteFile(filepath.Join(sub, "child.md"), []byte("child"), 0644)

	src := file.New(dir, true, []string{"md"}, file.Options{})
	ch, err := src.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	var count int
	for range ch {
		count++
	}
	if count != 2 {
		t.Errorf("got %d metas, want 2", count)
	}
}

func TestFileAdapterScanMetaFields(t *testing.T) {
	dir := t.TempDir()
	content := []byte("# My Doc\n\nSome content here.")
	os.WriteFile(filepath.Join(dir, "test.md"), content, 0644)

	src := file.New(dir, false, []string{"md"}, file.Options{})
	ch, _ := src.Scan(context.Background())
	meta := <-ch

	if meta.SourceType != "file" {
		t.Errorf("source_type = %q, want %q", meta.SourceType, "file")
	}
	if meta.ContentHash == "" {
		t.Error("ContentHash should not be empty")
	}
	expectedID := "file://" + filepath.Join(dir, "test.md")
	if meta.ID != expectedID {
		t.Errorf("ID = %q, want %q", meta.ID, expectedID)
	}
}

func TestFileAdapterLoadReturnsContent(t *testing.T) {
	dir := t.TempDir()
	content := []byte("# My Doc\n\nSome content here.")
	os.WriteFile(filepath.Join(dir, "test.md"), content, 0644)

	src := file.New(dir, false, []string{"md"}, file.Options{})
	ch, _ := src.Scan(context.Background())
	meta := <-ch

	doc, err := src.Load(context.Background(), meta)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if doc.Content != string(content) {
		t.Errorf("content mismatch: got %q, want %q", doc.Content, string(content))
	}
	if doc.ID != meta.ID {
		t.Errorf("Load returned wrong ID: got %q, want %q", doc.ID, meta.ID)
	}
}

func TestFileAdapterLoadHashMatchesScan(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.md"), []byte("hello"), 0644)

	src := file.New(dir, false, []string{"md"}, file.Options{})
	ch, _ := src.Scan(context.Background())
	meta := <-ch

	doc, err := src.Load(context.Background(), meta)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// Hash in Load result must match hash from Scan
	if doc.ContentHash != meta.ContentHash {
		t.Errorf("ContentHash mismatch: scan=%q load=%q", meta.ContentHash, doc.ContentHash)
	}
}
