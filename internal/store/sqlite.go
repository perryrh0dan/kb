package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"time"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"github.com/user/kb/internal/adapters"
)

//go:embed migrations/001_init.sql
var initSQL string

func init() {
	sqlite_vec.Auto()
}

type sqliteStore struct {
	db *sql.DB
}

// NewSQLite opens (or creates) the SQLite database at dbPath and runs migrations.
func NewSQLite(dbPath string) (Store, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on&_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := db.PingContext(context.Background()); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	if _, err := db.Exec(initSQL); err != nil {
		return nil, fmt.Errorf("run migrations: %w", err)
	}
	return &sqliteStore{db: db}, nil
}

func (s *sqliteStore) Close() error { return s.db.Close() }

func (s *sqliteStore) GetDocument(ctx context.Context, id string) (*adapters.Document, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, title, source_type, content_hash, metadata, ingested_at FROM documents WHERE id = ?`, id)
	var doc adapters.Document
	var metaJSON string
	var ingestedAt string
	err := row.Scan(&doc.ID, &doc.Title, &doc.SourceType, &doc.ContentHash, &metaJSON, &ingestedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	doc.IngestedAt, _ = time.Parse(time.RFC3339, ingestedAt)
	json.Unmarshal([]byte(metaJSON), &doc.Metadata)
	return &doc, nil
}

func (s *sqliteStore) UpsertDocument(ctx context.Context, doc adapters.Document) error {
	meta, _ := json.Marshal(doc.Metadata)
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO documents (id, title, source_type, content_hash, metadata, ingested_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   title=excluded.title, source_type=excluded.source_type,
		   content_hash=excluded.content_hash, metadata=excluded.metadata,
		   ingested_at=excluded.ingested_at`,
		doc.ID, doc.Title, doc.SourceType, doc.ContentHash,
		string(meta), doc.IngestedAt.UTC().Format(time.RFC3339))
	return err
}

func (s *sqliteStore) DeleteDocument(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM documents WHERE id = ?`, id)
	return err
}

func (s *sqliteStore) GetAllDocumentIDs(ctx context.Context, sourceType string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM documents WHERE source_type = ?`, sourceType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *sqliteStore) SaveChunks(ctx context.Context, chunks []Chunk) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx,
		`INSERT OR REPLACE INTO chunks (id, document_id, content, chunk_index, embedding) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, ch := range chunks {
		if ch.ID == "" {
			ch.ID = uuid.New().String()
		}
		embBytes, err := sqlite_vec.SerializeFloat32(ch.Embedding)
		if err != nil {
			return fmt.Errorf("serialize embedding: %w", err)
		}
		if _, err := stmt.ExecContext(ctx, ch.ID, ch.DocumentID, ch.Content, ch.ChunkIndex, embBytes); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *sqliteStore) DeleteChunks(ctx context.Context, documentID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM chunks WHERE document_id = ?`, documentID)
	return err
}

func (s *sqliteStore) Search(ctx context.Context, embedding []float32, limit int, minScore float64, sourceFilter string) ([]SearchResult, error) {
	embBytes, err := sqlite_vec.SerializeFloat32(embedding)
	if err != nil {
		return nil, fmt.Errorf("serialize embedding: %w", err)
	}

	query := `
		SELECT c.id, c.document_id, c.content, c.chunk_index,
		       (1 - vec_distance_cosine(c.embedding, ?)) AS score,
		       d.title, d.source_type, d.content_hash, d.metadata, d.ingested_at
		FROM chunks c
		JOIN documents d ON c.document_id = d.id
		WHERE c.embedding IS NOT NULL
		  AND (1 - vec_distance_cosine(c.embedding, ?)) >= ?`

	args := []interface{}{embBytes, embBytes, minScore}
	if sourceFilter != "" {
		query += " AND d.source_type = ?"
		args = append(args, sourceFilter)
	}
	query += " ORDER BY vec_distance_cosine(c.embedding, ?) ASC LIMIT ?"
	args = append(args, embBytes, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("search query: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		var metaJSON, ingestedAt string
		err := rows.Scan(
			&r.Chunk.ID, &r.Chunk.DocumentID, &r.Chunk.Content, &r.Chunk.ChunkIndex,
			&r.Score,
			&r.Document.Title, &r.Document.SourceType, &r.Document.ContentHash,
			&metaJSON, &ingestedAt,
		)
		if err != nil {
			return nil, err
		}
		r.Document.ID = r.Chunk.DocumentID
		r.Document.IngestedAt, _ = time.Parse(time.RFC3339, ingestedAt)
		json.Unmarshal([]byte(metaJSON), &r.Document.Metadata)
		results = append(results, r)
	}
	return results, rows.Err()
}

func (s *sqliteStore) GetChunks(ctx context.Context, documentID string) ([]Chunk, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, document_id, content, chunk_index FROM chunks WHERE document_id = ? ORDER BY chunk_index`, documentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var chunks []Chunk
	for rows.Next() {
		var ch Chunk
		if err := rows.Scan(&ch.ID, &ch.DocumentID, &ch.Content, &ch.ChunkIndex); err != nil {
			return nil, err
		}
		chunks = append(chunks, ch)
	}
	return chunks, rows.Err()
}

func (s *sqliteStore) Stats(ctx context.Context) (map[string]interface{}, error) {
	stats := map[string]interface{}{}
	row := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM documents`)
	var docCount int
	if err := row.Scan(&docCount); err != nil {
		return nil, err
	}
	stats["document_count"] = docCount

	row = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM chunks`)
	var chunkCount int
	if err := row.Scan(&chunkCount); err != nil {
		return nil, err
	}
	stats["chunk_count"] = chunkCount

	rows, _ := s.db.QueryContext(ctx,
		`SELECT source_type, COUNT(*), MAX(ingested_at) FROM documents GROUP BY source_type`)
	if rows != nil {
		defer rows.Close()
		type srcStat struct {
			Count      int    `json:"count"`
			LastIngest string `json:"last_ingested"`
		}
		bySource := map[string]srcStat{}
		for rows.Next() {
			var st, last string
			var cnt int
			rows.Scan(&st, &cnt, &last)
			bySource[st] = srcStat{Count: cnt, LastIngest: last}
		}
		stats["by_source"] = bySource
	}
	return stats, nil
}

// ContentHash computes SHA256 of s as a hex string.
func ContentHash(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h)
}
