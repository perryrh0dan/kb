package ingest

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

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

// ProgressAction describes what happened to a document during ingest.
type ProgressAction string

const (
	ActionIngested ProgressAction = "ingested"
	ActionSkipped  ProgressAction = "skipped"
	ActionError    ProgressAction = "error"
	ActionPruned   ProgressAction = "pruned"
)

// ProgressEvent is emitted once per document processed during an ingest run.
type ProgressEvent struct {
	Action ProgressAction
	Title  string // document title or short identifier
	Total  int    // running count of all processed documents so far (incl. skipped/errors)
}

// ProgressFunc is called once per document during RunWithProgress.
// It is always called synchronously in the ingest goroutine.
// A nil ProgressFunc is valid — no progress events are emitted.
type ProgressFunc func(ProgressEvent)

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
// No progress events are emitted; use RunWithProgress for live feedback.
func (ing *Ingester) Run(ctx context.Context, src adapters.Source, sourceType string, force bool) (IngestStats, error) {
	return ing.RunWithProgress(ctx, src, sourceType, force, nil)
}

// RunWithProgress ingests all documents from src, calling progress for each
// document processed. progress may be nil.
func (ing *Ingester) RunWithProgress(ctx context.Context, src adapters.Source, sourceType string, force bool, progress ProgressFunc) (IngestStats, error) {
	log := slog.Default()
	var stats IngestStats
	var total int

	emit := func(action ProgressAction, title string) {
		if progress != nil {
			progress(ProgressEvent{Action: action, Title: title, Total: total})
		}
	}

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
				total++
				emit(ActionSkipped, meta.Title)
				continue
			}
		}

		doc, err := src.Load(ctx, meta)
		if err != nil {
			log.Warn("load failed", "id", meta.ID, "error", err)
			stats.Errors++
			total++
			emit(ActionError, meta.Title)
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
			if strings.TrimSpace(doc.Content) == "" {
				log.Debug("skipping document with empty content", "id", doc.ID)
				stats.Skipped++
				total++
				emit(ActionSkipped, doc.Title)
				continue
			}
			chunks = []string{doc.Content}
		}

		// 1. Embed FIRST — if this fails the document hash is NOT written to the DB,
		// so the next ingest run will retry rather than skipping with a stale hash.
		embeddings, err := ing.embedder.Embed(ctx, chunks)
		if err != nil {
			log.Warn("embedding failed", "id", doc.ID, "chunks", len(chunks), "error", err)
			stats.Errors++
			total++
			emit(ActionError, doc.Title)
			continue
		}

		// 2. Delete old chunks SECOND (only after we know embedding succeeded)
		if err := ing.store.DeleteChunks(ctx, doc.ID); err != nil {
			log.Warn("failed to delete old chunks", "id", doc.ID, "error", err)
			stats.Errors++
			continue
		}

		// 3. Upsert document THIRD — hash written only after successful embed
		if err := ing.store.UpsertDocument(ctx, doc); err != nil {
			log.Warn("failed to upsert document", "id", doc.ID, "error", err)
			stats.Errors++
			continue
		}

		// 4. Save chunks LAST — skip any that had no embedding (empty content)
		var storeChunks []store.Chunk
		for i, text := range chunks {
			if embeddings[i] == nil {
				log.Debug("skipping empty chunk", "id", doc.ID, "chunk_index", i)
				continue
			}
			storeChunks = append(storeChunks, store.Chunk{
				ID:         uuid.New().String(),
				DocumentID: doc.ID,
				Content:    text,
				ChunkIndex: i,
				Embedding:  embeddings[i],
			})
		}
		if err := ing.store.SaveChunks(ctx, storeChunks); err != nil {
			log.Warn("failed to save chunks", "id", doc.ID, "error", err)
			stats.Errors++
			continue
		}
		log.Info("document ingested", "id", doc.ID, "chunks", len(chunks))
		stats.Ingested++
		total++
		emit(ActionIngested, doc.Title)
	}

	// Phase 2: prune documents no longer in source
	for id := range known {
		if !seen[id] {
			log.Debug("pruning deleted document", "id", id)
			if err := ing.store.DeleteDocument(ctx, id); err != nil {
				log.Warn("failed to prune document", "id", id, "error", id)
				stats.Errors++
			} else {
				log.Info("document pruned", "id", id)
				stats.Pruned++
				total++
				emit(ActionPruned, id)
			}
		}
	}

	return stats, nil
}

// RepairDocuments re-ingests a pre-supplied list of documents from src.
// Unlike Run, it skips the Scan phase and always forces re-embedding.
// Use this to recover documents that are in the store but have no chunks.
func (ing *Ingester) RepairDocuments(
	ctx context.Context,
	docs []adapters.DocumentMeta,
	src adapters.Source,
	progress ProgressFunc,
) (IngestStats, error) {
	log := slog.Default()
	var stats IngestStats
	var total int

	emit := func(action ProgressAction, title string) {
		if progress != nil {
			progress(ProgressEvent{Action: action, Title: title, Total: total})
		}
	}

	for _, meta := range docs {
		if ctx.Err() != nil {
			break
		}

		doc, err := src.Load(ctx, meta)
		if err != nil {
			log.Warn("repair: load failed", "id", meta.ID, "error", err)
			stats.Errors++
			total++
			emit(ActionError, meta.Title)
			continue
		}

		chunks, err := ing.chunker.Split(doc.Content)
		if err != nil {
			log.Warn("repair: chunker failed", "id", doc.ID, "error", err)
			stats.Errors++
			total++
			emit(ActionError, doc.Title)
			continue
		}
		if len(chunks) == 0 {
			if strings.TrimSpace(doc.Content) == "" {
				log.Debug("repair: skipping document with empty content", "id", doc.ID)
				stats.Skipped++
				total++
				emit(ActionSkipped, doc.Title)
				continue
			}
			chunks = []string{doc.Content}
		}

		embeddings, err := ing.embedder.Embed(ctx, chunks)
		if err != nil {
			log.Warn("repair: embedding failed", "id", doc.ID, "chunks", len(chunks), "error", err)
			stats.Errors++
			total++
			emit(ActionError, doc.Title)
			continue
		}

		if err := ing.store.DeleteChunks(ctx, doc.ID); err != nil {
			log.Warn("repair: failed to delete old chunks", "id", doc.ID, "error", err)
			stats.Errors++
			total++
			emit(ActionError, doc.Title)
			continue
		}

		if err := ing.store.UpsertDocument(ctx, doc); err != nil {
			log.Warn("repair: failed to upsert document", "id", doc.ID, "error", err)
			stats.Errors++
			total++
			emit(ActionError, doc.Title)
			continue
		}

		var storeChunks []store.Chunk
		for i, text := range chunks {
			if embeddings[i] == nil {
				continue
			}
			storeChunks = append(storeChunks, store.Chunk{
				ID:         uuid.New().String(),
				DocumentID: doc.ID,
				Content:    text,
				ChunkIndex: i,
				Embedding:  embeddings[i],
			})
		}
		if err := ing.store.SaveChunks(ctx, storeChunks); err != nil {
			log.Warn("repair: failed to save chunks", "id", doc.ID, "error", err)
			stats.Errors++
			total++
			emit(ActionError, doc.Title)
			continue
		}

		log.Info("repair: document repaired", "id", doc.ID, "chunks", len(storeChunks))
		stats.Ingested++
		total++
		emit(ActionIngested, doc.Title)
	}

	return stats, nil
}
