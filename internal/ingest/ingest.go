package ingest

import (
	"context"
	"fmt"
	"log/slog"

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
	log := slog.Default()
	var stats IngestStats

	// Use the source's scope prefix to limit pruning to only documents that
	// belong to this specific source (e.g. one directory, one Confluence space).
	// This prevents ingesting ./docs/k8s/ from deleting docs from ./docs/security/.
	scopePrefix := src.ScopePrefix()
	log.Debug("ingest run started", "source_type", sourceType, "scope_prefix", scopePrefix, "force", force)

	knownIDs, err := ing.store.GetAllDocumentIDs(ctx, scopePrefix)
	if err != nil {
		return stats, fmt.Errorf("get known ids: %w", err)
	}
	known := make(map[string]bool, len(knownIDs))
	for _, id := range knownIDs {
		known[id] = true
	}
	seen := make(map[string]bool)

	metaCh, err := src.Scan(ctx)
	if err != nil {
		return stats, fmt.Errorf("open source: %w", err)
	}

	for meta := range metaCh {
		if ctx.Err() != nil {
			break
		}
		seen[meta.ID] = true

		if !force {
			existing, err := ing.store.GetDocument(ctx, meta.ID)
			if err == nil && existing != nil && existing.ContentHash == meta.ContentHash {
				log.Debug("document unchanged, skipping", "id", meta.ID)
				stats.Skipped++
				continue
			}
		}

		doc, err := src.Load(ctx, meta)
		if err != nil {
			log.Warn("load failed", "id", meta.ID, "error", err)
			stats.Errors++
			continue
		}

		log.Debug("ingesting document", "id", doc.ID, "title", doc.Title)

		chunks, err := ing.chunker.Split(doc.Content)
		if err != nil {
			log.Warn("chunker failed", "id", doc.ID, "error", err)
			stats.Errors++
			continue
		}
		if len(chunks) == 0 {
			chunks = []string{doc.Content}
		}

		// 1. Delete old chunks FIRST
		if err := ing.store.DeleteChunks(ctx, doc.ID); err != nil {
			log.Warn("failed to delete old chunks", "id", doc.ID, "error", err)
			stats.Errors++
			continue
		}

		// 2. Upsert document SECOND
		if err := ing.store.UpsertDocument(ctx, doc); err != nil {
			log.Warn("failed to upsert document", "id", doc.ID, "error", err)
			stats.Errors++
			continue
		}

		// 3. Embed THIRD
		embeddings, err := ing.embedder.Embed(ctx, chunks)
		if err != nil {
			log.Warn("embedding failed", "id", doc.ID, "chunks", len(chunks), "error", err)
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
			log.Warn("failed to save chunks", "id", doc.ID, "error", err)
			stats.Errors++
			continue
		}
		log.Info("document ingested", "id", doc.ID, "chunks", len(chunks))
		stats.Ingested++
	}

	// Phase 2: prune documents no longer in source
	for id := range known {
		if !seen[id] {
			log.Debug("pruning deleted document", "id", id)
			if err := ing.store.DeleteDocument(ctx, id); err != nil {
				log.Warn("failed to prune document", "id", id, "error", err)
				stats.Errors++
			} else {
				log.Info("document pruned", "id", id)
				stats.Pruned++
			}
		}
	}

	return stats, nil
}
