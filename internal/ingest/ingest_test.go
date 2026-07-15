// internal/ingest/ingest_test.go
package ingest_test

import (
	"context"
	"testing"
	"time"

	"github.com/user/kb/internal/adapters"
	"github.com/user/kb/internal/chunker"
	"github.com/user/kb/internal/embedder"
	"github.com/user/kb/internal/ingest"
	"github.com/user/kb/internal/store"
)

// stubSource emits a fixed list of documents.
type stubSource struct{ docs []adapters.Document }

func (s *stubSource) Documents(ctx context.Context) (<-chan adapters.Document, error) {
	ch := make(chan adapters.Document, len(s.docs))
	for _, d := range s.docs {
		ch <- d
	}
	close(ch)
	return ch, nil
}

// stubEmbedder returns zero vectors.
type stubEmbedder struct{ dims int }

func (e *stubEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	vecs := make([][]float32, len(texts))
	for i := range vecs {
		vecs[i] = make([]float32, e.dims)
	}
	return vecs, nil
}
func (e *stubEmbedder) Dimensions() int   { return e.dims }
func (e *stubEmbedder) ModelName() string { return "stub" }

func makeDoc(id, content string) adapters.Document {
	return adapters.Document{
		ID: id, Title: id, Content: content,
		ContentHash: store.ContentHash(content),
		SourceType:  "file",
		Metadata:    map[string]string{},
		IngestedAt:  time.Now().UTC(),
	}
}

func TestIngestNewDocument(t *testing.T) {
	st, _ := store.NewSQLite(t.TempDir() + "/test.db")
	defer st.Close()

	c := chunker.New(512, 50)
	emb := &stubEmbedder{dims: 3072}
	ing := ingest.New(st, c, emb)

	src := &stubSource{docs: []adapters.Document{makeDoc("file:///a.md", "hello world")}}
	stats, err := ing.Run(context.Background(), src, "file", false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if stats.Ingested != 1 {
		t.Errorf("ingested = %d, want 1", stats.Ingested)
	}
}

func TestIngestSkipsUnchanged(t *testing.T) {
	st, _ := store.NewSQLite(t.TempDir() + "/test.db")
	defer st.Close()

	c := chunker.New(512, 50)
	emb := &stubEmbedder{dims: 3072}
	ing := ingest.New(st, c, emb)
	doc := makeDoc("file:///a.md", "hello world")

	// First ingest
	ing.Run(context.Background(), &stubSource{docs: []adapters.Document{doc}}, "file", false)
	// Second ingest — same content
	stats, _ := ing.Run(context.Background(), &stubSource{docs: []adapters.Document{doc}}, "file", false)
	if stats.Skipped != 1 {
		t.Errorf("skipped = %d, want 1", stats.Skipped)
	}
}

func TestIngestPrunesDeletedDocuments(t *testing.T) {
	st, _ := store.NewSQLite(t.TempDir() + "/test.db")
	defer st.Close()

	c := chunker.New(512, 50)
	emb := &stubEmbedder{dims: 3072}
	ing := ingest.New(st, c, emb)

	// Ingest two docs
	ing.Run(context.Background(), &stubSource{docs: []adapters.Document{
		makeDoc("file:///a.md", "aaa"),
		makeDoc("file:///b.md", "bbb"),
	}}, "file", false)

	// Second ingest — only one doc remains in source
	stats, _ := ing.Run(context.Background(), &stubSource{docs: []adapters.Document{
		makeDoc("file:///a.md", "aaa"),
	}}, "file", false)

	if stats.Pruned != 1 {
		t.Errorf("pruned = %d, want 1", stats.Pruned)
	}
	// b.md should be gone
	doc, _ := st.GetDocument(context.Background(), "file:///b.md")
	if doc != nil {
		t.Errorf("file:///b.md still exists after prune")
	}
}

func TestIngestForceReindex(t *testing.T) {
	st, err := store.NewSQLite(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}
	defer st.Close()
	c := chunker.New(512, 50)
	emb := &stubEmbedder{dims: 3072}
	ing := ingest.New(st, c, emb)
	doc := makeDoc("file:///a.md", "hello world")

	// First ingest
	ing.Run(context.Background(), &stubSource{docs: []adapters.Document{doc}}, "file", false)
	// Second ingest with same content but force=true
	stats, err := ing.Run(context.Background(), &stubSource{docs: []adapters.Document{doc}}, "file", true)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if stats.Ingested != 1 {
		t.Errorf("force=true: ingested = %d, want 1", stats.Ingested)
	}
	if stats.Skipped != 0 {
		t.Errorf("force=true: skipped = %d, want 0", stats.Skipped)
	}
}

func TestIngestZeroChunkFallback(t *testing.T) {
	st, err := store.NewSQLite(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}
	defer st.Close()
	c := chunker.New(512, 50)
	emb := &stubEmbedder{dims: 3072}
	ing := ingest.New(st, c, emb)
	// Use empty content to trigger the zero-chunk path
	doc := makeDoc("file:///empty.md", "")
	stats, err := ing.Run(context.Background(), &stubSource{docs: []adapters.Document{doc}}, "file", false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// Empty content → chunker returns nil → fallback to []string{""} → 1 chunk embedded
	// Document should be ingested (not error)
	if stats.Errors != 0 {
		t.Errorf("errors = %d, want 0", stats.Errors)
	}
	if stats.Ingested != 1 {
		t.Errorf("ingested = %d, want 1", stats.Ingested)
	}
}

// Ensure the stubEmbedder satisfies the embedder.Embedder interface at compile time.
var _ embedder.Embedder = (*stubEmbedder)(nil)
