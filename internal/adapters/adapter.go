package adapters

import (
	"context"
	"time"
)

// Document represents a document from any source.
type Document struct {
	ID          string            // Source URI e.g. "file:///abs/path/doc.md"
	Title       string
	Content     string
	ContentHash string            // SHA256 hex of Content
	SourceType  string            // "file" | "confluence"
	Metadata    map[string]string // author, url, updated_at, etc.
	IngestedAt  time.Time
}

// Source is the interface all data source adapters must implement.
type Source interface {
	// Documents streams all documents from this source.
	// The channel is closed when all documents have been sent or ctx is cancelled.
	Documents(ctx context.Context) (<-chan Document, error)
}
