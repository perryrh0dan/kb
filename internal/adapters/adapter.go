package adapters

import (
	"context"
	"time"
)

// DocumentMeta contains everything that can be determined cheaply —
// without fetching document body or running Vision analysis.
// ContentHash is always computed from source bytes (file: raw file bytes;
// confluence: pageID + ":" + version.createdAt) so it is deterministic
// and does not require full content extraction.
type DocumentMeta struct {
	ID          string            // Source URI e.g. "file:///abs/path/doc.md"
	Title       string
	ContentHash string            // SHA256 hex — deterministic, computed without body
	SourceType  string            // "file" | "confluence"
	Metadata    map[string]string // author, url, updated_at, etc.
	IngestedAt  time.Time
}

// Document is DocumentMeta plus the extracted content.
// Content is only populated after Source.Load() is called.
type Document struct {
	DocumentMeta
	Content string
}

// Source is the interface all data source adapters must implement.
type Source interface {
	// Scan streams DocumentMeta for all documents in this source.
	// This must be cheap: no body fetching, no Vision API calls.
	// The channel is closed when all metadata has been sent or ctx is cancelled.
	Scan(ctx context.Context) (<-chan DocumentMeta, error)

	// Load fetches the full content for a document identified by meta.
	// This may be expensive (Vision API, HTTP body fetch).
	// Only called by the ingestor when the hash check fails.
	Load(ctx context.Context, meta DocumentMeta) (Document, error)

	// ScopePrefix returns the document ID prefix for pruning.
	// Only documents whose IDs start with this prefix are considered for pruning.
	ScopePrefix() string
}
