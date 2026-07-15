package ingest

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/user/kb/internal/adapters"
	"github.com/user/kb/internal/chunker"
	"github.com/user/kb/internal/embedder"
	"github.com/user/kb/internal/store"
)

// IngestStats summarises a single ingest run.
type IngestStats struct {
	Ingested int
	Skipped  int
	Pruned   int
	Errors   int
}

// Ingester orchestrates Adapter → Chunker → Embedder → Store.
type Ingester struct {
	store    store.Store
	chunker  *chunker.Chunker
	embedder embedder.Embedder
}

// New creates an Ingester.
func New(st store.Store, c *chunker.Chunker, emb embedder.Embedder) *Ingester {
	return &Ingester{store: st, chunker: c, embedder: emb}
}

// Run ingests all documents from src. force=true skips hash check.
func (ing *Ingester) Run(ctx context.Context, src adapters.Source, sourceType string, force bool) (IngestStats, error) {
	var stats IngestStats

	knownIDs, err := ing.store.GetAllDocumentIDs(ctx, sourceType)
	if err != nil {
		return stats, fmt.Errorf("get known ids: %w", err)
	}
	known := make(map[string]bool, len(knownIDs))
	for _, id := range knownIDs {
		known[id] = true
	}
	seen := make(map[string]bool)

	docCh, err := src.Documents(ctx)
	if err != nil {
		return stats, fmt.Errorf("open source: %w", err)
	}

	for doc := range docCh {
		if ctx.Err() != nil {
			break
		}
		seen[doc.ID] = true

		if !force {
			existing, err := ing.store.GetDocument(ctx, doc.ID)
			if err == nil && existing != nil && existing.ContentHash == doc.ContentHash {
				stats.Skipped++
				continue
			}
		}

		chunks, err := ing.chunker.Split(doc.Content)
		if err != nil {
			stats.Errors++
			continue
		}
		if len(chunks) == 0 {
			chunks = []string{doc.Content}
		}

		// 1. Delete old chunks FIRST
		if err := ing.store.DeleteChunks(ctx, doc.ID); err != nil {
			stats.Errors++
			continue
		}

		// 2. Upsert document SECOND
		if err := ing.store.UpsertDocument(ctx, doc); err != nil {
			stats.Errors++
			continue
		}

		// 3. Embed THIRD
		embeddings, err := ing.embedder.Embed(ctx, chunks)
		if err != nil {
			stats.Errors++
			continue
		}

		// 4. Save chunks LAST
		storeChunks := make([]store.Chunk, len(chunks))
		for i, text := range chunks {
			storeChunks[i] = store.Chunk{
				ID:         uuid.New().String(),
				DocumentID: doc.ID,
				Content:    text,
				ChunkIndex: i,
				Embedding:  embeddings[i],
			}
		}
		if err := ing.store.SaveChunks(ctx, storeChunks); err != nil {
			stats.Errors++
			continue
		}
		stats.Ingested++
	}

	// Phase 2: prune documents no longer in source
	for id := range known {
		if !seen[id] {
			if err := ing.store.DeleteDocument(ctx, id); err != nil {
				stats.Errors++
			} else {
				stats.Pruned++
			}
		}
	}

	return stats, nil
}
