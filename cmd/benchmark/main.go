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
	id     string
	vector []byte
}

func main() {
	sourcePath := flag.String("source", filepath.Join(os.Getenv("HOME"), ".kb", "kb.db"), "source database")
	workPath := flag.String("work-db", "", "benchmark database; defaults to a temporary copy")
	queryCount := flag.Int("queries", 20, "number of stored embeddings to use as benchmark queries")
	limit := flag.Int("limit", 10, "search result count")
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
		f, err := os.CreateTemp("", "kb-benchmark-*.db")
		if err != nil {
			log.Fatal(err)
		}
		work = f.Name()
		f.Close()
		os.Remove(work)
	}
	if _, err := os.Stat(work); err == nil {
		log.Fatalf("work database already exists: %s", work)
	}
	if _, err := source.ExecContext(ctx, "VACUUM INTO ?", work); err != nil {
		log.Fatalf("copy source database with VACUUM INTO: %v", err)
	}

	db, err := sql.Open("sqlite3", work+"?_journal_mode=WAL&_busy_timeout=30000")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("open benchmark database: %v", err)
	}

	queries, err := loadQueries(ctx, db, *queryCount)
	if err != nil {
		log.Fatal(err)
	}

	start := time.Now()
	for _, query := range queries {
		if _, err := search(ctx, db, query.vector, *limit); err != nil {
			log.Fatalf("search %s: %v", query.id, err)
		}
	}
	duration := time.Since(start)

	stat, err := os.Stat(work)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("source=%s\n", *sourcePath)
	fmt.Printf("work_db=%s\n", work)
	fmt.Printf("queries=%d limit=%d\n", len(queries), *limit)
	fmt.Printf("total=%s average=%s\n", duration.Round(time.Millisecond), (duration / time.Duration(len(queries))).Round(time.Microsecond))
	fmt.Printf("work_db_size=%d MB\n", stat.Size()/(1024*1024))
	fmt.Println("The work database is retained for inspection and can be deleted manually.")
}

func loadQueries(ctx context.Context, db *sql.DB, limit int) ([]queryVector, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, embedding
		FROM chunks
		WHERE embedding IS NOT NULL
		ORDER BY id
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var queries []queryVector
	for rows.Next() {
		var query queryVector
		if err := rows.Scan(&query.id, &query.vector); err != nil {
			return nil, err
		}
		queries = append(queries, query)
	}
	return queries, rows.Err()
}

func search(ctx context.Context, db *sql.DB, vector []byte, limit int) (int, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT c.id
		FROM chunks c
		WHERE c.embedding IS NOT NULL
		ORDER BY vec_distance_cosine(c.embedding, ?)
		LIMIT ?`, vector, limit)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return 0, err
		}
		count++
	}
	return count, rows.Err()
}

func init() {
	sqlite_vec.Auto()
}
