package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	_ "github.com/mattn/go-sqlite3"
)

type queryVector struct {
	id         string
	sourceType string
	vector     []byte
}

func main() {
	sourcePath := flag.String("source", filepath.Join(os.Getenv("HOME"), ".kb", "kb.db"), "source database")
	workPath := flag.String("work-db", "", "benchmark database; defaults to a temporary file")
	queryCount := flag.Int("queries", 20, "number of stored embeddings to use as benchmark queries")
	limit := flag.Int("limit", 10, "nearest-neighbor result count")
	reuse := flag.Bool("reuse", false, "reuse an existing work database instead of copying the source")
	flag.Parse()

	if *queryCount < 1 || *limit < 1 {
		log.Fatal("queries and limit must be positive")
	}

	ctx := context.Background()
	source, err := sql.Open("sqlite3", *sourcePath+"?_journal_mode=WAL&_busy_timeout=30000")
	if err != nil {
		log.Fatal(err)
	}
	defer source.Close()

	if err := source.PingContext(ctx); err != nil {
		log.Fatalf("open source database: %v", err)
	}

	work := *workPath
	if work == "" {
		f, err := os.CreateTemp("", "kb-vec0-benchmark-*.db")
		if err != nil {
			log.Fatal(err)
		}
		work = f.Name()
		f.Close()
		os.Remove(work)
	}
	if _, err := os.Stat(work); err == nil && !*reuse {
		log.Fatalf("work database already exists: %s", work)
	}

	if !*reuse {
		if _, err := source.ExecContext(ctx, "VACUUM INTO ?", work); err != nil {
			log.Fatalf("copy source database with VACUUM INTO: %v", err)
		}
	}

	vecdb, err := sql.Open("sqlite3", work+"?_journal_mode=WAL&_busy_timeout=30000")
	if err != nil {
		log.Fatal(err)
	}
	defer vecdb.Close()
	if err := vecdb.PingContext(ctx); err != nil {
		log.Fatalf("open benchmark database: %v", err)
	}

	if err := createVectorTable(ctx, vecdb); err != nil {
		log.Fatal(err)
	}
	start := time.Now()
	if _, err := vecdb.ExecContext(ctx, `
		INSERT INTO chunk_vectors(chunk_id, source_type, embedding)
		SELECT c.id, d.source_type, c.embedding
		FROM chunks c
		JOIN documents d ON d.id = c.document_id
		WHERE c.embedding IS NOT NULL`); err != nil {
		log.Fatalf("backfill vec0 table: %v", err)
	}
	backfillDuration := time.Since(start)

	queries, err := loadQueries(ctx, vecdb, *queryCount)
	if err != nil {
		log.Fatal(err)
	}

	baselineDuration, vec0Duration, overlap, err := benchmark(ctx, vecdb, queries, *limit)
	if err != nil {
		log.Fatal(err)
	}

	var vectorCount int
	if err := vecdb.QueryRowContext(ctx, "SELECT count(*) FROM chunk_vectors").Scan(&vectorCount); err != nil {
		log.Fatal(err)
	}
	stat, err := os.Stat(work)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("source=%s\n", *sourcePath)
	fmt.Printf("work_db=%s\n", work)
	fmt.Printf("vec0_vectors=%d\n", vectorCount)
	fmt.Printf("work_db_size=%d MB\n", stat.Size()/(1024*1024))
	fmt.Printf("backfill=%s\n", backfillDuration.Round(time.Millisecond))
	fmt.Printf("queries=%d limit=%d\n", len(queries), *limit)
	fmt.Printf("baseline_total=%s average=%s\n", baselineDuration.Round(time.Millisecond), (baselineDuration / time.Duration(len(queries))).Round(time.Microsecond))
	fmt.Printf("vec0_total=%s average=%s\n", vec0Duration.Round(time.Millisecond), (vec0Duration / time.Duration(len(queries))).Round(time.Microsecond))
	fmt.Printf("top_k_overlap=%.2f\n", overlap)
	fmt.Println("The work database is retained for inspection and can be deleted manually.")
}

func createVectorTable(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, "DROP TABLE IF EXISTS chunk_vectors"); err != nil {
		return err
	}
	_, err := db.ExecContext(ctx, `
		CREATE VIRTUAL TABLE chunk_vectors USING vec0(
			chunk_id TEXT PRIMARY KEY,
			source_type TEXT PARTITION KEY,
			embedding FLOAT[3072] distance_metric=cosine
		)`)
	return err
}

func loadQueries(ctx context.Context, db *sql.DB, limit int) ([]queryVector, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT c.id, d.source_type, c.embedding
		FROM chunks c
		JOIN documents d ON d.id = c.document_id
		WHERE c.embedding IS NOT NULL
		ORDER BY c.id
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var queries []queryVector
	for rows.Next() {
		var q queryVector
		if err := rows.Scan(&q.id, &q.sourceType, &q.vector); err != nil {
			return nil, err
		}
		queries = append(queries, q)
	}
	return queries, rows.Err()
}

func benchmark(ctx context.Context, db *sql.DB, queries []queryVector, limit int) (time.Duration, time.Duration, float64, error) {
	var baselineTotal, vec0Total time.Duration
	var overlapTotal float64

	for _, q := range queries {
		start := time.Now()
		baseline, err := baselineSearch(ctx, db, q.vector, q.sourceType, limit)
		baselineTotal += time.Since(start)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("baseline query %s: %w", q.id, err)
		}

		start = time.Now()
		indexed, err := vec0Search(ctx, db, q.vector, q.sourceType, limit)
		vec0Total += time.Since(start)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("vec0 query %s: %w", q.id, err)
		}
		overlapTotal += overlapRatio(baseline, indexed)
	}

	if len(queries) == 0 {
		return baselineTotal, vec0Total, 1, nil
	}
	return baselineTotal, vec0Total, overlapTotal / float64(len(queries)), nil
}

func baselineSearch(ctx context.Context, db *sql.DB, vector []byte, sourceType string, limit int) (map[string]struct{}, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT c.id
		FROM chunks c
		JOIN documents d ON d.id = c.document_id
		WHERE c.embedding IS NOT NULL
		  AND d.source_type = ?
		ORDER BY vec_distance_cosine(c.embedding, ?)
		LIMIT ?`, sourceType, vector, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanIDs(rows)
}

func vec0Search(ctx context.Context, db *sql.DB, vector []byte, sourceType string, limit int) (map[string]struct{}, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT chunk_id
		FROM chunk_vectors
		WHERE embedding MATCH ?
		  AND k = ?
		  AND source_type = ?`, vector, limit, sourceType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanIDs(rows)
}

func scanIDs(rows *sql.Rows) (map[string]struct{}, error) {
	ids := make(map[string]struct{})
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids[id] = struct{}{}
	}
	return ids, rows.Err()
}

func overlapRatio(left, right map[string]struct{}) float64 {
	if len(left) == 0 {
		if len(right) == 0 {
			return 1
		}
		return 0
	}
	common := 0
	for id := range left {
		if _, ok := right[id]; ok {
			common++
		}
	}
	return float64(common) / float64(len(left))
}

func init() {
	sqlite_vec.Auto()
}
