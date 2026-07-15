package store_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/user/kb/internal/adapters"
	"github.com/user/kb/internal/store"
)

func newTestStore(t *testing.T) store.Store {
	t.Helper()
	s, err := store.NewSQLite(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestUpsertAndGetDocument(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	doc := adapters.Document{
		ID:          "file:///tmp/test.md",
		Title:       "Test Doc",
		Content:     "hello world",
		ContentHash: "abc123",
		SourceType:  "file",
		Metadata:    map[string]string{"path": "/tmp/test.md"},
		IngestedAt:  time.Now().UTC(),
	}
	if err := s.UpsertDocument(ctx, doc); err != nil {
		t.Fatalf("UpsertDocument: %v", err)
	}
	got, err := s.GetDocument(ctx, doc.ID)
	if err != nil {
		t.Fatalf("GetDocument: %v", err)
	}
	if got.Title != doc.Title {
		t.Errorf("title = %q, want %q", got.Title, doc.Title)
	}
	if got.ContentHash != doc.ContentHash {
		t.Errorf("hash = %q, want %q", got.ContentHash, doc.ContentHash)
	}
}

func TestGetDocumentNotFound(t *testing.T) {
	s := newTestStore(t)
	got, err := s.GetDocument(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}

func TestDeleteDocumentCascadesToChunks(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	doc := adapters.Document{
		ID: "file:///tmp/cascade.md", Title: "Cascade", Content: "x",
		ContentHash: "h1", SourceType: "file",
		Metadata: map[string]string{}, IngestedAt: time.Now().UTC(),
	}
	_ = s.UpsertDocument(ctx, doc)
	chunk := store.Chunk{
		ID: "chunk-1", DocumentID: doc.ID, Content: "x", ChunkIndex: 0,
		Embedding: make([]float32, 3072),
	}
	_ = s.SaveChunks(ctx, []store.Chunk{chunk})

	if err := s.DeleteDocument(ctx, doc.ID); err != nil {
		t.Fatalf("DeleteDocument: %v", err)
	}
	// document should be gone
	got, _ := s.GetDocument(ctx, doc.ID)
	if got != nil {
		t.Errorf("document still exists after delete")
	}
}

func TestGetAllDocumentIDs(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	for _, id := range []string{"file:///a.md", "file:///b.md", "confluence://ENG/1"} {
		src := "file"
		if id[:4] == "conf" {
			src = "confluence"
		}
		_ = s.UpsertDocument(ctx, adapters.Document{
			ID: id, Title: id, Content: "x", ContentHash: "h",
			SourceType: src, Metadata: map[string]string{}, IngestedAt: time.Now().UTC(),
		})
	}
	ids, err := s.GetAllDocumentIDs(ctx, "file")
	if err != nil {
		t.Fatalf("GetAllDocumentIDs: %v", err)
	}
	if len(ids) != 2 {
		t.Errorf("got %d file IDs, want 2", len(ids))
	}
}
