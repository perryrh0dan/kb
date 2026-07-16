// internal/adapters/file/file_test.go
package file_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/user/kb/internal/adapters/file"
)

func TestFileAdapterFindsMarkdown(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.md"), []byte("# Hello\nworld"), 0644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("plain text"), 0644)
	os.WriteFile(filepath.Join(dir, "c.go"), []byte("package main"), 0644)

	src := file.New(dir, false, []string{"md", "txt"}, file.Options{})
	ch, err := src.Documents(context.Background())
	if err != nil {
		t.Fatalf("Documents: %v", err)
	}
	var docs []string
	for d := range ch {
		docs = append(docs, d.ID)
	}
	if len(docs) != 2 {
		t.Errorf("got %d docs, want 2: %v", len(docs), docs)
	}
}

func TestFileAdapterRecursive(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	os.MkdirAll(sub, 0755)
	os.WriteFile(filepath.Join(dir, "root.md"), []byte("root"), 0644)
	os.WriteFile(filepath.Join(sub, "child.md"), []byte("child"), 0644)

	src := file.New(dir, true, []string{"md"}, file.Options{})
	ch, err := src.Documents(context.Background())
	if err != nil {
		t.Fatalf("Documents: %v", err)
	}
	var count int
	for range ch {
		count++
	}
	if count != 2 {
		t.Errorf("got %d docs, want 2", count)
	}
}

func TestFileAdapterDocumentFields(t *testing.T) {
	dir := t.TempDir()
	content := []byte("# My Doc\n\nSome content here.")
	os.WriteFile(filepath.Join(dir, "test.md"), content, 0644)

	src := file.New(dir, false, []string{"md"}, file.Options{})
	ch, _ := src.Documents(context.Background())
	doc := <-ch

	if doc.SourceType != "file" {
		t.Errorf("source_type = %q, want %q", doc.SourceType, "file")
	}
	if doc.ContentHash == "" {
		t.Error("ContentHash should not be empty")
	}
	if doc.Content != string(content) {
		t.Errorf("content mismatch")
	}
	expectedID := "file://" + filepath.Join(dir, "test.md")
	if doc.ID != expectedID {
		t.Errorf("ID = %q, want %q", doc.ID, expectedID)
	}
}
