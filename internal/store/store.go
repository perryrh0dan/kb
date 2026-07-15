package store

import (
	"context"

	"github.com/user/kb/internal/adapters"
)

// Chunk is a piece of a document with its embedding.
type Chunk struct {
	ID         string
	DocumentID string
	Content    string
	ChunkIndex int
	Embedding  []float32
}

// SearchResult is returned by similarity search.
type SearchResult struct {
	Score    float64
	Chunk    Chunk
	Document adapters.Document
}

// Store persists documents and chunks and provides similarity search.
type Store interface {
	// Document operations
	GetDocument(ctx context.Context, id string) (*adapters.Document, error)
	UpsertDocument(ctx context.Context, doc adapters.Document) error
	DeleteDocument(ctx context.Context, id string) error
	// GetAllDocumentIDs returns IDs of all documents whose ID starts with idPrefix.
	// Use the source's ScopePrefix() as the prefix to limit pruning to that scope.
	GetAllDocumentIDs(ctx context.Context, idPrefix string) ([]string, error)

	// Chunk operations
	SaveChunks(ctx context.Context, chunks []Chunk) error
	DeleteChunks(ctx context.Context, documentID string) error
	GetChunks(ctx context.Context, documentID string) ([]Chunk, error)

	// Search
	Search(ctx context.Context, embedding []float32, limit int, minScore float64, sourceFilter string) ([]SearchResult, error)

	// Stats
	Stats(ctx context.Context) (map[string]interface{}, error)

	Close() error
}
