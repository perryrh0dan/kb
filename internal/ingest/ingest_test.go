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
type stubSource struct {
	docs   []adapters.Document
	prefix string // scope prefix for pruning
}

func (s *stubSource) Documents(ctx context.Context) (<-chan adapters.Document, error) {
	ch := make(chan adapters.Document, len(s.docs))
	for _, d := range s.docs {
		ch <- d
	}
	close(ch)
	return ch, nil
}

func (s *stubSource) ScopePrefix() string {
	if s.prefix != "" {
		return s.prefix
	}
	return "file:///"
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

	// Both docs share the same scope prefix — simulates a single directory scan.
	scope := "file:///"

	// Ingest two docs
	ing.Run(context.Background(), &stubSource{
		docs:   []adapters.Document{makeDoc("file:///a.md", "aaa"), makeDoc("file:///b.md", "bbb")},
		prefix: scope,
	}, "file", false)

	// Second ingest — only one doc remains in source
	stats, _ := ing.Run(context.Background(), &stubSource{
		docs:   []adapters.Document{makeDoc("file:///a.md", "aaa")},
		prefix: scope,
	}, "file", false)

	if stats.Pruned != 1 {
		t.Errorf("pruned = %d, want 1", stats.Pruned)
	}
	// b.md should be gone
	doc, _ := st.GetDocument(context.Background(), "file:///b.md")
	if doc != nil {
		t.Errorf("file:///b.md still exists after prune")
	}
}

// TestIngestScopedPruning verifies that pruning is limited to the source's scope.
// Documents outside the scope must NOT be deleted even if unseen.
func TestIngestScopedPruning(t *testing.T) {
	st, err := store.NewSQLite(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}
	defer st.Close()

	c := chunker.New(512, 50)
	emb := &stubEmbedder{dims: 3072}
	ing := ingest.New(st, c, emb)

	// First: ingest a doc from a different scope (security/)
	security := &stubSource{
		docs:   []adapters.Document{makeDoc("file:///docs/security/auth.md", "auth content")},
		prefix: "file:///docs/security/",
	}
	if _, err := ing.Run(context.Background(), security, "file", false); err != nil {
		t.Fatalf("first ingest: %v", err)
	}

	// Second: ingest from a different scope (k8s/) — should NOT prune security doc
	k8s := &stubSource{
		docs:   []adapters.Document{makeDoc("file:///docs/k8s/deploy.md", "deploy content")},
		prefix: "file:///docs/k8s/",
	}
	stats, err := ing.Run(context.Background(), k8s, "file", false)
	if err != nil {
		t.Fatalf("second ingest: %v", err)
	}

	if stats.Pruned != 0 {
		t.Errorf("pruned = %d, want 0 (security doc should not be pruned)", stats.Pruned)
	}

	// Security doc must still exist
	doc, _ := st.GetDocument(context.Background(), "file:///docs/security/auth.md")
	if doc == nil {
		t.Errorf("security doc was wrongly pruned by k8s ingest")
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
