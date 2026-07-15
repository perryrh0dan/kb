# kb — Knowledge Base CLI & MCP Server: Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a single Go binary `kb` that ingests documents from files and Confluence, embeds them with OpenAI, stores them in SQLite with vector search, and exposes search via CLI and MCP server.

**Architecture:** Monolithic single binary using Cobra for CLI and the official MCP Go SDK for MCP server (stdio). Internal interfaces (`Source`, `Embedder`) allow new adapters and providers without touching existing code. SQLite + sqlite-vec for local vector storage.

**Tech Stack:** Go 1.22+, Cobra, Viper, modelcontextprotocol/go-sdk, sashabaranov/go-openai, mattn/go-sqlite3 (CGO), asg017/sqlite-vec-go-bindings/cgo, pkoukk/tiktoken-go, google/uuid

## Global Constraints

- Go module path: `github.com/user/kb` (adjust to actual username)
- CGO required: sqlite-vec uses CGO; build with `CGO_ENABLED=1`
- Default config path: `~/.kb/config.yaml`
- Default DB path: `~/.kb/kb.db`
- Embedder: `text-embedding-3-large`, 3072 dimensions
- Default chunk size: 512 tokens, overlap: 50 tokens
- All tests use `go test ./...`
- Commits after every task

---

### Task 1: Go Module + Project Scaffold

**Files:**
- Create: `go.mod`
- Create: `go.sum` (generated)
- Create: `cmd/kb/main.go`
- Create: `internal/adapters/adapter.go`
- Create: `internal/embedder/embedder.go`
- Create: `internal/store/store.go`
- Create: `internal/chunker/chunker.go`
- Create: `internal/ingest/ingest.go`
- Create: `internal/mcp/server.go`
- Create: `config/config.go`

**Interfaces:**
- Produces:
  - `adapters.Document` struct
  - `adapters.Source` interface
  - `embedder.Embedder` interface
  - `store.Store` interface
  - All subsequent tasks depend on these

- [ ] **Step 1: Initialize Go module**

```bash
cd /root/workspace/kb
go mod init github.com/user/kb
```

- [ ] **Step 2: Add all dependencies**

```bash
go get github.com/spf13/cobra@latest
go get github.com/spf13/viper@latest
go get github.com/modelcontextprotocol/go-sdk@latest
go get github.com/sashabaranov/go-openai@latest
go get github.com/mattn/go-sqlite3@latest
go get github.com/asg017/sqlite-vec-go-bindings/cgo@latest
go get github.com/pkoukk/tiktoken-go@latest
go get github.com/google/uuid@latest
go get gopkg.in/yaml.v3@latest
```

- [ ] **Step 3: Create `internal/adapters/adapter.go`**

```go
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
```

- [ ] **Step 4: Create `internal/embedder/embedder.go`**

```go
package embedder

import (
	"context"
	"fmt"

	"github.com/user/kb/config"
)

// Embedder converts text slices into float32 vectors.
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	Dimensions() int
	ModelName() string
}

// New creates an Embedder based on the provider in cfg.
func New(cfg config.EmbedderConfig) (Embedder, error) {
	switch cfg.Provider {
	case "openai":
		// imported in Task 5
		return nil, fmt.Errorf("openai embedder: not yet registered (import openai subpackage)")
	default:
		return nil, fmt.Errorf("unknown embedder provider: %q", cfg.Provider)
	}
}
```

- [ ] **Step 5: Create `internal/store/store.go`**

```go
package store

import (
	"context"

	"github.com/user/kb/internal/adapters"
)

// Chunk is a piece of a document with its embedding.
type Chunk struct {
	ID          string
	DocumentID  string
	Content     string
	ChunkIndex  int
	Embedding   []float32
}

// SearchResult is returned by similarity search.
type SearchResult struct {
	Score      float64
	Chunk      Chunk
	Document   adapters.Document
}

// Store persists documents and chunks and provides similarity search.
type Store interface {
	// Document operations
	GetDocument(ctx context.Context, id string) (*adapters.Document, error)
	UpsertDocument(ctx context.Context, doc adapters.Document) error
	DeleteDocument(ctx context.Context, id string) error
	GetAllDocumentIDs(ctx context.Context, sourceType string) ([]string, error)

	// Chunk operations
	SaveChunks(ctx context.Context, chunks []Chunk) error
	DeleteChunks(ctx context.Context, documentID string) error

	// Search
	Search(ctx context.Context, embedding []float32, limit int, minScore float64, sourceFilter string) ([]SearchResult, error)

	// Stats
	Stats(ctx context.Context) (map[string]interface{}, error)

	Close() error
}
```

- [ ] **Step 6: Create `internal/chunker/chunker.go` (stub)**

```go
package chunker

// Chunker splits text into overlapping chunks.
type Chunker struct {
	ChunkSize    int
	ChunkOverlap int
}

// New creates a Chunker with the given token limits.
func New(chunkSize, chunkOverlap int) *Chunker {
	return &Chunker{ChunkSize: chunkSize, ChunkOverlap: chunkOverlap}
}

// Split splits text into chunks. Implemented in Task 3.
func (c *Chunker) Split(text string) ([]string, error) {
	return []string{text}, nil // placeholder
}
```

- [ ] **Step 7: Create `internal/ingest/ingest.go` (stub)**

```go
package ingest

// Ingester orchestrates the ingest pipeline.
// Implemented in Task 6.
type Ingester struct{}
```

- [ ] **Step 8: Create `internal/mcp/server.go` (stub)**

```go
package mcp

// Server is the MCP server. Implemented in Task 8.
type Server struct{}
```

- [ ] **Step 9: Create `cmd/kb/main.go`**

```go
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "kb: not yet implemented")
	os.Exit(1)
}
```

- [ ] **Step 10: Verify it compiles**

```bash
CGO_ENABLED=1 go build ./...
```

Expected: no errors (warnings about unused imports are fine at this stage)

- [ ] **Step 11: Commit**

```bash
git init
git add .
git commit -m "feat: initial project scaffold with interfaces"
```

---

### Task 2: Config System

**Files:**
- Create: `config/config.go`
- Create: `config/config_test.go`

**Interfaces:**
- Produces:
  - `config.Config` struct
  - `config.EmbedderConfig` struct
  - `config.ChunkerConfig` struct
  - `config.SourceConfig` struct (for registered sources)
  - `config.Load() (*Config, error)`
  - `config.InitDefault() error` (writes `~/.kb/config.yaml` with defaults)

- [ ] **Step 1: Write failing tests**

```go
// config/config_test.go
package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/user/kb/config"
)

func TestLoadDefaults(t *testing.T) {
	// No config file, no env vars — should return defaults
	t.Setenv("KB_OPENAI_API_KEY", "")
	t.Setenv("KB_DB_PATH", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Embedder.Provider != "openai" {
		t.Errorf("default provider = %q, want %q", cfg.Embedder.Provider, "openai")
	}
	if cfg.Embedder.Model != "text-embedding-3-large" {
		t.Errorf("default model = %q, want %q", cfg.Embedder.Model, "text-embedding-3-large")
	}
	if cfg.Chunker.ChunkSize != 512 {
		t.Errorf("default chunk_size = %d, want 512", cfg.Chunker.ChunkSize)
	}
	if cfg.Chunker.ChunkOverlap != 50 {
		t.Errorf("default chunk_overlap = %d, want 50", cfg.Chunker.ChunkOverlap)
	}
}

func TestEnvVarOverride(t *testing.T) {
	t.Setenv("KB_OPENAI_API_KEY", "sk-test-key")
	t.Setenv("KB_DB_PATH", "/tmp/test.db")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.OpenAI.APIKey != "sk-test-key" {
		t.Errorf("api_key = %q, want %q", cfg.OpenAI.APIKey, "sk-test-key")
	}
	if cfg.DB.Path != "/tmp/test.db" {
		t.Errorf("db.path = %q, want %q", cfg.DB.Path, "/tmp/test.db")
	}
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	content := `
openai:
  api_key: "sk-from-file"
chunker:
  chunk_size: 256
  chunk_overlap: 25
`
	if err := os.WriteFile(cfgFile, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.LoadFrom(cfgFile)
	if err != nil {
		t.Fatalf("LoadFrom() error: %v", err)
	}
	if cfg.OpenAI.APIKey != "sk-from-file" {
		t.Errorf("api_key = %q, want %q", cfg.OpenAI.APIKey, "sk-from-file")
	}
	if cfg.Chunker.ChunkSize != 256 {
		t.Errorf("chunk_size = %d, want 256", cfg.Chunker.ChunkSize)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./config/... -v
```

Expected: FAIL — `config` package not implemented yet

- [ ] **Step 3: Implement `config/config.go`**

```go
package config

import (
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type OpenAIConfig struct {
	APIKey string `mapstructure:"api_key"`
}

type ConfluenceConfig struct {
	BaseURL   string `mapstructure:"base_url"`
	Username  string `mapstructure:"username"`
	APIToken  string `mapstructure:"api_token"`
	PAT       string `mapstructure:"pat"`
}

type DBConfig struct {
	Path string `mapstructure:"path"`
}

type EmbedderConfig struct {
	Provider string `mapstructure:"provider"`
	Model    string `mapstructure:"model"`
}

type ChunkerConfig struct {
	ChunkSize    int `mapstructure:"chunk_size"`
	ChunkOverlap int `mapstructure:"chunk_overlap"`
}

type SourceConfig struct {
	Type       string   `mapstructure:"type"`
	Path       string   `mapstructure:"path,omitempty"`
	Recursive  bool     `mapstructure:"recursive,omitempty"`
	Extensions []string `mapstructure:"extensions,omitempty"`
	Space      string   `mapstructure:"space,omitempty"`
	PageID     string   `mapstructure:"page_id,omitempty"`
}

type Config struct {
	OpenAI     OpenAIConfig     `mapstructure:"openai"`
	Confluence ConfluenceConfig `mapstructure:"confluence"`
	DB         DBConfig         `mapstructure:"db"`
	Embedder   EmbedderConfig   `mapstructure:"embedder"`
	Chunker    ChunkerConfig    `mapstructure:"chunker"`
	Sources    []SourceConfig   `mapstructure:"sources"`
}

func defaultConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".kb", "config.yaml")
}

func newViper() *viper.Viper {
	v := viper.New()
	v.SetDefault("embedder.provider", "openai")
	v.SetDefault("embedder.model", "text-embedding-3-large")
	v.SetDefault("chunker.chunk_size", 512)
	v.SetDefault("chunker.chunk_overlap", 50)
	v.SetDefault("db.path", filepath.Join(mustHomeDir(), ".kb", "kb.db"))

	v.SetEnvPrefix("KB")
	v.BindEnv("openai.api_key", "KB_OPENAI_API_KEY")
	v.BindEnv("confluence.api_token", "KB_CONFLUENCE_API_TOKEN")
	v.BindEnv("confluence.pat", "KB_CONFLUENCE_PAT")
	v.BindEnv("db.path", "KB_DB_PATH")

	return v
}

func mustHomeDir() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return h
}

// Load reads config from the default path (~/.kb/config.yaml) with env-var overrides.
func Load() (*Config, error) {
	return LoadFrom(defaultConfigPath())
}

// LoadFrom reads config from the given file path with env-var overrides.
func LoadFrom(path string) (*Config, error) {
	v := newViper()
	v.SetConfigFile(path)

	if err := v.ReadInConfig(); err != nil {
		if !os.IsNotExist(err) {
			// File exists but couldn't be read
			if _, statErr := os.Stat(path); statErr == nil {
				return nil, err
			}
			// File doesn't exist — use defaults + env vars only
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// InitDefault writes a default config file to ~/.kb/config.yaml.
func InitDefault() error {
	path := defaultConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		return nil // already exists
	}
	content := `# kb configuration

openai:
  api_key: ""  # or set KB_OPENAI_API_KEY env var

confluence:
  base_url: ""
  username: ""       # Cloud: Confluence username/email
  api_token: ""      # Cloud: API token (or KB_CONFLUENCE_API_TOKEN)
  pat: ""            # Data Center: Personal Access Token (or KB_CONFLUENCE_PAT)

db:
  path: ~/.kb/kb.db  # or set KB_DB_PATH env var

embedder:
  provider: openai
  model: text-embedding-3-large

chunker:
  chunk_size: 512
  chunk_overlap: 50

# sources are auto-registered when you run: kb ingest file <path> / kb ingest confluence --space <KEY>
sources: []
`
	return os.WriteFile(path, []byte(content), 0600)
}

// Save writes the config back to disk (used to register sources).
func Save(cfg *Config) error {
	path := defaultConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	v := newViper()
	v.Set("openai", cfg.OpenAI)
	v.Set("confluence", cfg.Confluence)
	v.Set("db", cfg.DB)
	v.Set("embedder", cfg.Embedder)
	v.Set("chunker", cfg.Chunker)
	v.Set("sources", cfg.Sources)
	return v.WriteConfigAs(path)
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./config/... -v
```

Expected: PASS for all three tests

- [ ] **Step 5: Commit**

```bash
git add config/
git commit -m "feat: config system with YAML + env-var loading"
```

---

### Task 3: Recursive Character Chunker

**Files:**
- Create: `internal/chunker/chunker.go`
- Create: `internal/chunker/chunker_test.go`

**Interfaces:**
- Consumes: nothing
- Produces:
  - `chunker.New(chunkSize, chunkOverlap int) *Chunker`
  - `(*Chunker).Split(text string) ([]string, error)`

- [ ] **Step 1: Write failing tests**

```go
// internal/chunker/chunker_test.go
package chunker_test

import (
	"strings"
	"testing"

	"github.com/user/kb/internal/chunker"
)

func TestSplitShortText(t *testing.T) {
	c := chunker.New(512, 50)
	chunks, err := c.Split("Hello world")
	if err != nil {
		t.Fatalf("Split() error: %v", err)
	}
	if len(chunks) != 1 {
		t.Errorf("got %d chunks, want 1", len(chunks))
	}
	if chunks[0] != "Hello world" {
		t.Errorf("chunk = %q, want %q", chunks[0], "Hello world")
	}
}

func TestSplitRespectsParagraphs(t *testing.T) {
	// Build text with clear paragraph breaks
	para := strings.Repeat("word ", 100) // ~100 tokens per paragraph
	text := para + "\n\n" + para + "\n\n" + para
	c := chunker.New(150, 20)
	chunks, err := c.Split(text)
	if err != nil {
		t.Fatalf("Split() error: %v", err)
	}
	// Should produce multiple chunks
	if len(chunks) < 2 {
		t.Errorf("got %d chunks, want >= 2", len(chunks))
	}
}

func TestOverlapIsApplied(t *testing.T) {
	// 60-word sentences separated by newlines
	sentence := strings.Repeat("word ", 60)
	text := sentence + "\n" + sentence + "\n" + sentence
	c := chunker.New(100, 30)
	chunks, err := c.Split(text)
	if err != nil {
		t.Fatalf("Split() error: %v", err)
	}
	if len(chunks) < 2 {
		t.Fatalf("need at least 2 chunks to test overlap, got %d", len(chunks))
	}
	// The end of chunk[0] should appear at the beginning of chunk[1]
	end0 := chunks[0][len(chunks[0])-20:]
	if !strings.Contains(chunks[1], end0[:10]) {
		t.Errorf("overlap not found: end of chunk[0] not in chunk[1]")
	}
}

func TestEmptyText(t *testing.T) {
	c := chunker.New(512, 50)
	chunks, err := c.Split("")
	if err != nil {
		t.Fatalf("Split() error: %v", err)
	}
	if len(chunks) != 0 {
		t.Errorf("got %d chunks for empty text, want 0", len(chunks))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/chunker/... -v
```

Expected: FAIL

- [ ] **Step 3: Implement `internal/chunker/chunker.go`**

```go
package chunker

import (
	"strings"

	tiktoken "github.com/pkoukk/tiktoken-go"
)

// separators tried in order; fall back to next if chunk still too large
var separators = []string{"\n\n", "\n", ". ", " ", ""}

type Chunker struct {
	ChunkSize    int
	ChunkOverlap int
	enc          *tiktoken.Tiktoken
}

func New(chunkSize, chunkOverlap int) *Chunker {
	enc, err := tiktoken.GetEncoding("cl100k_base")
	if err != nil {
		// fallback: approximate by words
		enc = nil
	}
	return &Chunker{ChunkSize: chunkSize, ChunkOverlap: chunkOverlap, enc: enc}
}

func (c *Chunker) tokenCount(s string) int {
	if c.enc != nil {
		return len(c.enc.Encode(s, nil, nil))
	}
	// fallback: word count as approximation
	return len(strings.Fields(s))
}

// Split splits text into overlapping chunks using recursive character splitting.
func (c *Chunker) Split(text string) ([]string, error) {
	if strings.TrimSpace(text) == "" {
		return nil, nil
	}
	chunks := c.splitRecursive(text, separators)
	return c.mergeWithOverlap(chunks), nil
}

func (c *Chunker) splitRecursive(text string, seps []string) []string {
	if len(seps) == 0 || c.tokenCount(text) <= c.ChunkSize {
		return []string{text}
	}
	sep := seps[0]
	rest := seps[1:]

	var parts []string
	if sep == "" {
		// character-level split
		runes := []rune(text)
		for i := 0; i < len(runes); i += c.ChunkSize {
			end := i + c.ChunkSize
			if end > len(runes) {
				end = len(runes)
			}
			parts = append(parts, string(runes[i:end]))
		}
		return parts
	}

	segments := strings.Split(text, sep)
	var result []string
	for _, seg := range segments {
		if seg == "" {
			continue
		}
		if c.tokenCount(seg) <= c.ChunkSize {
			result = append(result, seg)
		} else {
			result = append(result, c.splitRecursive(seg, rest)...)
		}
	}
	return result
}

// mergeWithOverlap joins small segments into chunks of ChunkSize with ChunkOverlap.
func (c *Chunker) mergeWithOverlap(parts []string) []string {
	if len(parts) == 0 {
		return nil
	}
	var chunks []string
	var current []string
	currentTokens := 0

	flush := func() {
		if len(current) == 0 {
			return
		}
		chunks = append(chunks, strings.Join(current, " "))
	}

	for _, part := range parts {
		pt := c.tokenCount(part)
		if currentTokens+pt > c.ChunkSize && len(current) > 0 {
			flush()
			// keep overlap: drop parts from front until we're within overlap budget
			for len(current) > 0 && currentTokens > c.ChunkOverlap {
				removed := c.tokenCount(current[0])
				current = current[1:]
				currentTokens -= removed
			}
		}
		current = append(current, part)
		currentTokens += pt
	}
	flush()
	return chunks
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/chunker/... -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/chunker/
git commit -m "feat: recursive character splitter with token-based overlap"
```

---

### Task 4: SQLite Store

**Files:**
- Create: `internal/store/sqlite.go`
- Create: `internal/store/migrations/001_init.sql`
- Create: `internal/store/sqlite_test.go`

**Interfaces:**
- Consumes:
  - `adapters.Document`
  - `store.Store` interface (from Task 1)
  - `store.Chunk` struct (from Task 1)
  - `store.SearchResult` struct (from Task 1)
- Produces:
  - `store.NewSQLite(dbPath string) (Store, error)`

- [ ] **Step 1: Create migration SQL**

```sql
-- internal/store/migrations/001_init.sql
CREATE TABLE IF NOT EXISTS documents (
    id           TEXT PRIMARY KEY,
    title        TEXT NOT NULL,
    source_type  TEXT NOT NULL,
    content_hash TEXT NOT NULL,
    metadata     TEXT NOT NULL DEFAULT '{}',
    ingested_at  DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS chunks (
    id          TEXT PRIMARY KEY,
    document_id TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    content     TEXT NOT NULL,
    chunk_index INTEGER NOT NULL,
    embedding   F32_BLOB(3072)
);

CREATE INDEX IF NOT EXISTS idx_chunks_document_id ON chunks(document_id);
```

- [ ] **Step 2: Write failing tests**

```go
// internal/store/sqlite_test.go
package store_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/user/kb/internal/adapters"
	"github.com/user/kb/internal/store"
)

func newTestStore(t *testing.T) store.Store {
	t.Helper()
	s, err := store.NewSQLite(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestUpsertAndGetDocument(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	doc := adapters.Document{
		ID:          "file:///tmp/test.md",
		Title:       "Test Doc",
		Content:     "hello world",
		ContentHash: "abc123",
		SourceType:  "file",
		Metadata:    map[string]string{"path": "/tmp/test.md"},
		IngestedAt:  time.Now().UTC(),
	}
	if err := s.UpsertDocument(ctx, doc); err != nil {
		t.Fatalf("UpsertDocument: %v", err)
	}
	got, err := s.GetDocument(ctx, doc.ID)
	if err != nil {
		t.Fatalf("GetDocument: %v", err)
	}
	if got.Title != doc.Title {
		t.Errorf("title = %q, want %q", got.Title, doc.Title)
	}
	if got.ContentHash != doc.ContentHash {
		t.Errorf("hash = %q, want %q", got.ContentHash, doc.ContentHash)
	}
}

func TestGetDocumentNotFound(t *testing.T) {
	s := newTestStore(t)
	got, err := s.GetDocument(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}

func TestDeleteDocumentCascadesToChunks(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	doc := adapters.Document{
		ID: "file:///tmp/cascade.md", Title: "Cascade", Content: "x",
		ContentHash: "h1", SourceType: "file",
		Metadata: map[string]string{}, IngestedAt: time.Now().UTC(),
	}
	_ = s.UpsertDocument(ctx, doc)
	chunk := store.Chunk{
		ID: "chunk-1", DocumentID: doc.ID, Content: "x", ChunkIndex: 0,
		Embedding: make([]float32, 3072),
	}
	_ = s.SaveChunks(ctx, []store.Chunk{chunk})

	if err := s.DeleteDocument(ctx, doc.ID); err != nil {
		t.Fatalf("DeleteDocument: %v", err)
	}
	// document should be gone
	got, _ := s.GetDocument(ctx, doc.ID)
	if got != nil {
		t.Errorf("document still exists after delete")
	}
}

func TestGetAllDocumentIDs(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	for _, id := range []string{"file:///a.md", "file:///b.md", "confluence://ENG/1"} {
		src := "file"
		if id[:4] == "conf" {
			src = "confluence"
		}
		_ = s.UpsertDocument(ctx, adapters.Document{
			ID: id, Title: id, Content: "x", ContentHash: "h",
			SourceType: src, Metadata: map[string]string{}, IngestedAt: time.Now().UTC(),
		})
	}
	ids, err := s.GetAllDocumentIDs(ctx, "file")
	if err != nil {
		t.Fatalf("GetAllDocumentIDs: %v", err)
	}
	if len(ids) != 2 {
		t.Errorf("got %d file IDs, want 2", len(ids))
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

```bash
CGO_ENABLED=1 go test ./internal/store/... -v
```

Expected: FAIL — `store.NewSQLite` not implemented

- [ ] **Step 4: Implement `internal/store/sqlite.go`**

```go
package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/user/kb/internal/adapters"
	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	_ "github.com/mattn/go-sqlite3"
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
		`INSERT INTO chunks (id, document_id, content, chunk_index, embedding) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, ch := range chunks {
		if ch.ID == "" {
			ch.ID = uuid.New().String()
		}
		embBytes := sqlite_vec.SerializeFloat32(ch.Embedding)
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
	embBytes := sqlite_vec.SerializeFloat32(embedding)

	query := `
		SELECT c.id, c.document_id, c.content, c.chunk_index,
		       (1 - vec_distance_cosine(c.embedding, ?)) AS score,
		       d.title, d.source_type, d.content_hash, d.metadata, d.ingested_at
		FROM chunks c
		JOIN documents d ON c.document_id = d.id
		WHERE c.embedding IS NOT NULL`

	args := []interface{}{embBytes}
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
		if r.Score >= minScore {
			results = append(results, r)
		}
	}
	return results, rows.Err()
}

func (s *sqliteStore) Stats(ctx context.Context) (map[string]interface{}, error) {
	stats := map[string]interface{}{}
	row := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM documents`)
	var docCount int
	row.Scan(&docCount)
	stats["document_count"] = docCount

	row = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM chunks`)
	var chunkCount int
	row.Scan(&chunkCount)
	stats["chunk_count"] = chunkCount

	rows, _ := s.db.QueryContext(ctx,
		`SELECT source_type, COUNT(*), MAX(ingested_at) FROM documents GROUP BY source_type`)
	if rows != nil {
		defer rows.Close()
		type srcStat struct {
			Count     int    `json:"count"`
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
```

- [ ] **Step 5: Run tests**

```bash
CGO_ENABLED=1 go test ./internal/store/... -v
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/store/
git commit -m "feat: SQLite store with sqlite-vec vector search"
```

---

### Task 5: OpenAI Embedder

**Files:**
- Create: `internal/embedder/openai/openai.go`
- Modify: `internal/embedder/embedder.go` (update factory to import openai subpackage)
- Create: `internal/embedder/openai/openai_test.go`

**Interfaces:**
- Consumes:
  - `embedder.Embedder` interface (from Task 1)
  - `config.EmbedderConfig` (from Task 2)
  - `config.OpenAIConfig` (from Task 2)
- Produces:
  - `openai.New(embedCfg config.EmbedderConfig, oaiCfg config.OpenAIConfig) (embedder.Embedder, error)`

- [ ] **Step 1: Write failing test (uses mock HTTP)**

```go
// internal/embedder/openai/openai_test.go
package openai_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/user/kb/config"
	oaiemb "github.com/user/kb/internal/embedder/openai"
)

func TestEmbed(t *testing.T) {
	// Mock OpenAI API
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"object": "list",
			"data": []map[string]interface{}{
				{"object": "embedding", "index": 0, "embedding": make([]float64, 3072)},
				{"object": "embedding", "index": 1, "embedding": make([]float64, 3072)},
			},
			"model": "text-embedding-3-large",
			"usage": map[string]int{"prompt_tokens": 10, "total_tokens": 10},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	emb, err := oaiemb.NewWithBaseURL(
		config.EmbedderConfig{Provider: "openai", Model: "text-embedding-3-large"},
		config.OpenAIConfig{APIKey: "sk-test"},
		srv.URL+"/v1",
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	texts := []string{"hello world", "foo bar"}
	vecs, err := emb.Embed(context.Background(), texts)
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(vecs) != 2 {
		t.Errorf("got %d vectors, want 2", len(vecs))
	}
	if len(vecs[0]) != 3072 {
		t.Errorf("got %d dims, want 3072", len(vecs[0]))
	}
}

func TestDimensions(t *testing.T) {
	emb, _ := oaiemb.NewWithBaseURL(
		config.EmbedderConfig{Provider: "openai", Model: "text-embedding-3-large"},
		config.OpenAIConfig{APIKey: "sk-test"},
		"http://localhost",
	)
	if emb.Dimensions() != 3072 {
		t.Errorf("Dimensions() = %d, want 3072", emb.Dimensions())
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
CGO_ENABLED=1 go test ./internal/embedder/... -v
```

Expected: FAIL

- [ ] **Step 3: Implement `internal/embedder/openai/openai.go`**

```go
package openai

import (
	"context"
	"fmt"

	oai "github.com/sashabaranov/go-openai"
	"github.com/user/kb/config"
)

const batchSize = 100

type openAIEmbedder struct {
	client *oai.Client
	model  oai.EmbeddingModel
	dims   int
}

// New creates an OpenAI embedder using the default OpenAI base URL.
func New(embedCfg config.EmbedderConfig, oaiCfg config.OpenAIConfig) (*openAIEmbedder, error) {
	return NewWithBaseURL(embedCfg, oaiCfg, "")
}

// NewWithBaseURL creates an OpenAI embedder with a custom base URL (for testing).
func NewWithBaseURL(embedCfg config.EmbedderConfig, oaiCfg config.OpenAIConfig, baseURL string) (*openAIEmbedder, error) {
	cfg := oai.DefaultConfig(oaiCfg.APIKey)
	if baseURL != "" {
		cfg.BaseURL = baseURL
	}
	client := oai.NewClientWithConfig(cfg)
	return &openAIEmbedder{
		client: client,
		model:  oai.EmbeddingModel(embedCfg.Model),
		dims:   3072,
	}, nil
}

func (e *openAIEmbedder) Dimensions() int    { return e.dims }
func (e *openAIEmbedder) ModelName() string  { return string(e.model) }

func (e *openAIEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	var results [][]float32
	for i := 0; i < len(texts); i += batchSize {
		end := i + batchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch := texts[i:end]
		resp, err := e.client.CreateEmbeddings(ctx, oai.EmbeddingRequest{
			Input: batch,
			Model: e.model,
		})
		if err != nil {
			return nil, fmt.Errorf("openai embed batch [%d:%d]: %w", i, end, err)
		}
		for _, d := range resp.Data {
			results = append(results, d.Embedding)
		}
	}
	return results, nil
}
```

- [ ] **Step 4: Update embedder factory to use openai package**

```go
// internal/embedder/embedder.go
package embedder

import (
	"context"
	"fmt"

	"github.com/user/kb/config"
	oaiemb "github.com/user/kb/internal/embedder/openai"
)

type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	Dimensions() int
	ModelName() string
}

func New(embedCfg config.EmbedderConfig, oaiCfg config.OpenAIConfig) (Embedder, error) {
	switch embedCfg.Provider {
	case "openai":
		return oaiemb.New(embedCfg, oaiCfg)
	default:
		return nil, fmt.Errorf("unknown embedder provider: %q", embedCfg.Provider)
	}
}
```

- [ ] **Step 5: Run tests**

```bash
CGO_ENABLED=1 go test ./internal/embedder/... -v
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/embedder/
git commit -m "feat: OpenAI embedder with batching and factory pattern"
```

---

### Task 6: File Adapter

**Files:**
- Create: `internal/adapters/file/file.go`
- Create: `internal/adapters/file/file_test.go`

**Interfaces:**
- Consumes:
  - `adapters.Source` interface (from Task 1)
  - `adapters.Document` struct (from Task 1)
  - `store.ContentHash` function (from Task 4)
- Produces:
  - `file.New(path string, recursive bool, extensions []string) adapters.Source`

- [ ] **Step 1: Write failing tests**

```go
// internal/adapters/file/file_test.go
package file_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/user/kb/internal/adapters/file"
)

func TestFileAdapterFindsMarkdown(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.md"), []byte("# Hello\nworld"), 0644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("plain text"), 0644)
	os.WriteFile(filepath.Join(dir, "c.go"), []byte("package main"), 0644)

	src := file.New(dir, false, []string{"md", "txt"})
	ch, err := src.Documents(context.Background())
	if err != nil {
		t.Fatalf("Documents: %v", err)
	}
	var docs []string
	for d := range ch {
		docs = append(docs, d.ID)
	}
	if len(docs) != 2 {
		t.Errorf("got %d docs, want 2: %v", len(docs), docs)
	}
}

func TestFileAdapterRecursive(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	os.MkdirAll(sub, 0755)
	os.WriteFile(filepath.Join(dir, "root.md"), []byte("root"), 0644)
	os.WriteFile(filepath.Join(sub, "child.md"), []byte("child"), 0644)

	src := file.New(dir, true, []string{"md"})
	ch, err := src.Documents(context.Background())
	if err != nil {
		t.Fatalf("Documents: %v", err)
	}
	var count int
	for range ch {
		count++
	}
	if count != 2 {
		t.Errorf("got %d docs, want 2", count)
	}
}

func TestFileAdapterDocumentFields(t *testing.T) {
	dir := t.TempDir()
	content := []byte("# My Doc\n\nSome content here.")
	os.WriteFile(filepath.Join(dir, "test.md"), content, 0644)

	src := file.New(dir, false, []string{"md"})
	ch, _ := src.Documents(context.Background())
	doc := <-ch

	if doc.SourceType != "file" {
		t.Errorf("source_type = %q, want %q", doc.SourceType, "file")
	}
	if doc.ContentHash == "" {
		t.Error("ContentHash should not be empty")
	}
	if doc.Content != string(content) {
		t.Errorf("content mismatch")
	}
	expectedID := "file://" + filepath.Join(dir, "test.md")
	if doc.ID != expectedID {
		t.Errorf("ID = %q, want %q", doc.ID, expectedID)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/adapters/file/... -v
```

Expected: FAIL

- [ ] **Step 3: Implement `internal/adapters/file/file.go`**

```go
package file

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/user/kb/internal/adapters"
	"github.com/user/kb/internal/store"
)

type fileSource struct {
	path       string
	recursive  bool
	extensions map[string]bool
}

// New creates a file Source. extensions should be like []string{"md","txt","pdf"}.
func New(path string, recursive bool, extensions []string) adapters.Source {
	exts := make(map[string]bool, len(extensions))
	for _, e := range extensions {
		exts[strings.ToLower(strings.TrimPrefix(e, "."))] = true
	}
	return &fileSource{path: path, recursive: recursive, extensions: exts}
}

func (f *fileSource) Documents(ctx context.Context) (<-chan adapters.Document, error) {
	ch := make(chan adapters.Document)
	go func() {
		defer close(ch)
		_ = filepath.WalkDir(f.path, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil // skip unreadable
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if d.IsDir() {
				if !f.recursive && p != f.path {
					return filepath.SkipDir
				}
				return nil
			}
			ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(p), "."))
			if !f.extensions[ext] {
				return nil
			}
			content, err := os.ReadFile(p)
			if err != nil {
				return nil
			}
			info, _ := d.Info()
			modTime := time.Time{}
			if info != nil {
				modTime = info.ModTime()
			}
			absPath, _ := filepath.Abs(p)
			doc := adapters.Document{
				ID:          "file://" + absPath,
				Title:       filepath.Base(p),
				Content:     string(content),
				ContentHash: store.ContentHash(string(content)),
				SourceType:  "file",
				Metadata: map[string]string{
					"path":     absPath,
					"filename": filepath.Base(p),
					"modified": modTime.UTC().Format(time.RFC3339),
				},
				IngestedAt: time.Now().UTC(),
			}
			select {
			case ch <- doc:
			case <-ctx.Done():
				return ctx.Err()
			}
			return nil
		})
	}()
	return ch, nil
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/adapters/file/... -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/adapters/file/
git commit -m "feat: file adapter with recursive walk and extension filtering"
```

---

### Task 7: Confluence Adapter

**Files:**
- Create: `internal/adapters/confluence/confluence.go`
- Create: `internal/adapters/confluence/confluence_test.go`

**Interfaces:**
- Consumes:
  - `adapters.Source` interface
  - `adapters.Document` struct
  - `store.ContentHash`
  - `config.ConfluenceConfig`
- Produces:
  - `confluence.New(cfg config.ConfluenceConfig, space string, pageID string) adapters.Source`

- [ ] **Step 1: Write failing tests (mock HTTP server)**

```go
// internal/adapters/confluence/confluence_test.go
package confluence_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/user/kb/config"
	"github.com/user/kb/internal/adapters/confluence"
)

func mockConfluenceServer() *httptest.Server {
	mux := http.NewServeMux()
	// v2 pages endpoint
	mux.HandleFunc("/wiki/api/v2/spaces/ENG/pages", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"results": []map[string]interface{}{
				{
					"id":    "12345",
					"title": "Test Page",
					"body": map[string]interface{}{
						"storage": map[string]interface{}{
							"value": "<p>Hello <strong>world</strong></p>",
						},
					},
					"version": map[string]interface{}{"createdAt": "2026-01-01T00:00:00Z"},
					"_links":  map[string]interface{}{"webui": "/display/ENG/Test+Page"},
				},
			},
			"_links": map[string]interface{}{}, // no next page
		}
		json.NewEncoder(w).Encode(resp)
	})
	return httptest.NewServer(mux)
}

func TestConfluenceAdapterFetchesPages(t *testing.T) {
	srv := mockConfluenceServer()
	defer srv.Close()

	cfg := config.ConfluenceConfig{
		BaseURL:  srv.URL,
		Username: "user@example.com",
		APIToken: "token123",
	}
	src := confluence.New(cfg, "ENG", "")
	ch, err := src.Documents(context.Background())
	if err != nil {
		t.Fatalf("Documents: %v", err)
	}
	var docs []string
	for d := range ch {
		docs = append(docs, d.ID)
	}
	if len(docs) != 1 {
		t.Errorf("got %d docs, want 1", len(docs))
	}
	if docs[0] != "confluence://ENG/12345" {
		t.Errorf("ID = %q, want %q", docs[0], "confluence://ENG/12345")
	}
}

func TestConfluenceHTMLStripped(t *testing.T) {
	srv := mockConfluenceServer()
	defer srv.Close()

	cfg := config.ConfluenceConfig{BaseURL: srv.URL, Username: "u", APIToken: "t"}
	src := confluence.New(cfg, "ENG", "")
	ch, _ := src.Documents(context.Background())
	doc := <-ch

	if doc.Content == "" {
		t.Error("content should not be empty")
	}
	// HTML tags should be stripped
	if len(doc.Content) > 0 && doc.Content[0] == '<' {
		t.Errorf("content still contains HTML: %q", doc.Content[:20])
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/adapters/confluence/... -v
```

Expected: FAIL

- [ ] **Step 3: Implement `internal/adapters/confluence/confluence.go`**

```go
package confluence

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/user/kb/config"
	"github.com/user/kb/internal/adapters"
	"github.com/user/kb/internal/store"
)

var htmlTagRe = regexp.MustCompile(`<[^>]+>`)

func stripHTML(s string) string {
	s = htmlTagRe.ReplaceAllString(s, " ")
	s = strings.Join(strings.Fields(s), " ")
	return s
}

type confluenceSource struct {
	cfg    config.ConfluenceConfig
	space  string
	pageID string
	client *http.Client
}

// New creates a Confluence Source.
// pageID is optional; if set, only that page and its children are fetched.
func New(cfg config.ConfluenceConfig, space, pageID string) adapters.Source {
	return &confluenceSource{cfg: cfg, space: space, pageID: pageID, client: &http.Client{Timeout: 30 * time.Second}}
}

func (c *confluenceSource) doRequest(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if c.cfg.PAT != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.PAT)
	} else {
		req.SetBasicAuth(c.cfg.Username, c.cfg.APIToken)
	}
	req.Header.Set("Accept", "application/json")
	return c.client.Do(req)
}

type pageResult struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Body  struct {
		Storage struct {
			Value string `json:"value"`
		} `json:"storage"`
	} `json:"body"`
	Version struct {
		CreatedAt string `json:"createdAt"`
	} `json:"version"`
	Links struct {
		WebUI string `json:"webui"`
	} `json:"_links"`
}

type pagesResponse struct {
	Results []pageResult `json:"results"`
	Links   struct {
		Next string `json:"next"`
	} `json:"_links"`
}

func (c *confluenceSource) Documents(ctx context.Context) (<-chan adapters.Document, error) {
	ch := make(chan adapters.Document)
	go func() {
		defer close(ch)
		url := fmt.Sprintf("%s/wiki/api/v2/spaces/%s/pages?body-format=storage&limit=50", c.cfg.BaseURL, c.space)
		if c.pageID != "" {
			url = fmt.Sprintf("%s/wiki/api/v2/pages/%s?body-format=storage", c.cfg.BaseURL, c.pageID)
		}
		for url != "" {
			resp, err := c.doRequest(ctx, url)
			if err != nil || resp.StatusCode >= 400 {
				return
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			var pr pagesResponse
			if c.pageID != "" {
				// single page response
				var single pageResult
				json.Unmarshal(body, &single)
				pr.Results = []pageResult{single}
			} else {
				json.Unmarshal(body, &pr)
			}

			for _, page := range pr.Results {
				content := stripHTML(page.Body.Storage.Value)
				doc := adapters.Document{
					ID:          fmt.Sprintf("confluence://%s/%s", c.space, page.ID),
					Title:       page.Title,
					Content:     content,
					ContentHash: store.ContentHash(content),
					SourceType:  "confluence",
					Metadata: map[string]string{
						"url":        c.cfg.BaseURL + "/wiki" + page.Links.WebUI,
						"space":      c.space,
						"page_id":    page.ID,
						"updated_at": page.Version.CreatedAt,
					},
					IngestedAt: time.Now().UTC(),
				}
				select {
				case ch <- doc:
				case <-ctx.Done():
					return
				}
			}

			// pagination
			if pr.Links.Next != "" && c.pageID == "" {
				url = c.cfg.BaseURL + pr.Links.Next
			} else {
				url = ""
			}
		}
	}()
	return ch, nil
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/adapters/confluence/... -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/adapters/confluence/
git commit -m "feat: Confluence adapter with Cloud (API token) and DC (PAT) auth"
```

---

### Task 8: Ingest Orchestrator

**Files:**
- Create: `internal/ingest/ingest.go`
- Create: `internal/ingest/ingest_test.go`

**Interfaces:**
- Consumes:
  - `adapters.Source` interface
  - `chunker.New`, `(*Chunker).Split`
  - `embedder.Embedder`
  - `store.Store`
  - `store.Chunk`
  - `store.ContentHash`
- Produces:
  - `ingest.New(store store.Store, chunker *chunker.Chunker, embedder embedder.Embedder) *Ingester`
  - `(*Ingester).Run(ctx context.Context, src adapters.Source, sourceType string, force bool) (IngestStats, error)`

- [ ] **Step 1: Write failing tests**

```go
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
type stubSource struct{ docs []adapters.Document }

func (s *stubSource) Documents(ctx context.Context) (<-chan adapters.Document, error) {
	ch := make(chan adapters.Document, len(s.docs))
	for _, d := range s.docs {
		ch <- d
	}
	close(ch)
	return ch, nil
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

	// Ingest two docs
	ing.Run(context.Background(), &stubSource{docs: []adapters.Document{
		makeDoc("file:///a.md", "aaa"),
		makeDoc("file:///b.md", "bbb"),
	}}, "file", false)

	// Second ingest — only one doc remains in source
	stats, _ := ing.Run(context.Background(), &stubSource{docs: []adapters.Document{
		makeDoc("file:///a.md", "aaa"),
	}}, "file", false)

	if stats.Pruned != 1 {
		t.Errorf("pruned = %d, want 1", stats.Pruned)
	}
	// b.md should be gone
	doc, _ := st.GetDocument(context.Background(), "file:///b.md")
	if doc != nil {
		t.Errorf("file:///b.md still exists after prune")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
CGO_ENABLED=1 go test ./internal/ingest/... -v
```

Expected: FAIL

- [ ] **Step 3: Implement `internal/ingest/ingest.go`**

```go
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
	store   store.Store
	chunker *chunker.Chunker
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

		embeddings, err := ing.embedder.Embed(ctx, chunks)
		if err != nil {
			stats.Errors++
			continue
		}

		if err := ing.store.DeleteChunks(ctx, doc.ID); err != nil {
			stats.Errors++
			continue
		}
		if err := ing.store.UpsertDocument(ctx, doc); err != nil {
			stats.Errors++
			continue
		}

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
			_ = ing.store.DeleteDocument(ctx, id)
			stats.Pruned++
		}
	}

	return stats, nil
}
```

- [ ] **Step 4: Run tests**

```bash
CGO_ENABLED=1 go test ./internal/ingest/... -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ingest/
git commit -m "feat: ingest orchestrator with incremental sync and pruning"
```

---

### Task 9: CLI Commands

**Files:**
- Create: `cmd_root.go`
- Create: `cmd_ingest.go`
- Create: `cmd_search.go`
- Create: `cmd_serve.go`
- Create: `cmd_status.go`
- Modify: `cmd/kb/main.go`

**Interfaces:**
- Consumes: all previous tasks
- Produces: working `kb` binary

- [ ] **Step 1: Create `cmd_root.go`**

```go
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/user/kb/config"
)

var rootCmd = &cobra.Command{
	Use:   "kb",
	Short: "kb — private knowledge base CLI and MCP server",
}

var cfg *config.Config

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.AddCommand(ingestCmd)
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(configCmd)
}

func initConfig() {
	var err error
	cfg, err = config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 2: Create `cmd_ingest.go`**

```go
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/user/kb/config"
	"github.com/user/kb/internal/adapters/confluence"
	"github.com/user/kb/internal/adapters/file"
	"github.com/user/kb/internal/chunker"
	"github.com/user/kb/internal/embedder"
	"github.com/user/kb/internal/ingest"
	"github.com/user/kb/internal/store"
)

var ingestCmd = &cobra.Command{
	Use:   "ingest",
	Short: "Ingest documents into the knowledge base",
	RunE:  runIngestAll,
}

var ingestFileCmd = &cobra.Command{
	Use:   "file <path>",
	Short: "Ingest local files",
	Args:  cobra.ExactArgs(1),
	RunE:  runIngestFile,
}

var ingestConfluenceCmd = &cobra.Command{
	Use:   "confluence",
	Short: "Ingest Confluence pages",
	RunE:  runIngestConfluence,
}

var (
	flagRecursive bool
	flagExt       string
	flagForce     bool
	flagSpace     string
	flagPageID    string
)

func init() {
	ingestFileCmd.Flags().BoolVar(&flagRecursive, "recursive", false, "walk subdirectories")
	ingestFileCmd.Flags().StringVar(&flagExt, "ext", "md,txt,pdf", "comma-separated file extensions")
	ingestFileCmd.Flags().BoolVar(&flagForce, "force", false, "force full re-index")
	ingestConfluenceCmd.Flags().StringVar(&flagSpace, "space", "", "Confluence space key (required)")
	ingestConfluenceCmd.Flags().StringVar(&flagPageID, "page", "", "scope to a single page ID")
	ingestConfluenceCmd.Flags().BoolVar(&flagForce, "force", false, "force full re-index")
	ingestConfluenceCmd.MarkFlagRequired("space")
	ingestCmd.AddCommand(ingestFileCmd, ingestConfluenceCmd)
}

func newIngester(cfg *config.Config) (*ingest.Ingester, store.Store, error) {
	st, err := store.NewSQLite(cfg.DB.Path)
	if err != nil {
		return nil, nil, fmt.Errorf("open store: %w", err)
	}
	emb, err := embedder.New(cfg.Embedder, cfg.OpenAI)
	if err != nil {
		return nil, nil, fmt.Errorf("create embedder: %w", err)
	}
	c := chunker.New(cfg.Chunker.ChunkSize, cfg.Chunker.ChunkOverlap)
	return ingest.New(st, c, emb), st, nil
}

func runIngestAll(cmd *cobra.Command, args []string) error {
	if len(cfg.Sources) == 0 {
		fmt.Println("No sources configured. Use `kb ingest file <path>` or `kb ingest confluence --space <KEY>` to add sources.")
		return nil
	}
	ing, st, err := newIngester(cfg)
	if err != nil {
		return err
	}
	defer st.Close()

	for _, src := range cfg.Sources {
		if err := runSource(ing, src, false); err != nil {
			fmt.Fprintf(os.Stderr, "error ingesting %s: %v\n", src.Type, err)
		}
	}
	return nil
}

func runSource(ing *ingest.Ingester, src config.SourceConfig, force bool) error {
	ctx := context.Background()
	switch src.Type {
	case "file":
		exts := strings.Split(src.Extensions[0], ",")
		if len(src.Extensions) > 1 {
			exts = src.Extensions
		}
		s := file.New(src.Path, src.Recursive, exts)
		stats, err := ing.Run(ctx, s, "file", force)
		if err != nil {
			return err
		}
		fmt.Printf("file %s: ingested=%d skipped=%d pruned=%d errors=%d\n",
			src.Path, stats.Ingested, stats.Skipped, stats.Pruned, stats.Errors)
	case "confluence":
		s := confluence.New(cfg.Confluence, src.Space, src.PageID)
		stats, err := ing.Run(ctx, s, "confluence", force)
		if err != nil {
			return err
		}
		fmt.Printf("confluence %s: ingested=%d skipped=%d pruned=%d errors=%d\n",
			src.Space, stats.Ingested, stats.Skipped, stats.Pruned, stats.Errors)
	}
	return nil
}

func runIngestFile(cmd *cobra.Command, args []string) error {
	path := args[0]
	exts := strings.Split(flagExt, ",")

	// Register/update in config
	registerSource(config.SourceConfig{
		Type: "file", Path: path, Recursive: flagRecursive, Extensions: exts,
	})

	ing, st, err := newIngester(cfg)
	if err != nil {
		return err
	}
	defer st.Close()

	ctx := context.Background()
	src := file.New(path, flagRecursive, exts)
	stats, err := ing.Run(ctx, src, "file", flagForce)
	if err != nil {
		return err
	}
	fmt.Printf("ingested=%d skipped=%d pruned=%d errors=%d\n",
		stats.Ingested, stats.Skipped, stats.Pruned, stats.Errors)
	return nil
}

func runIngestConfluence(cmd *cobra.Command, args []string) error {
	registerSource(config.SourceConfig{
		Type: "confluence", Space: flagSpace, PageID: flagPageID,
	})

	ing, st, err := newIngester(cfg)
	if err != nil {
		return err
	}
	defer st.Close()

	ctx := context.Background()
	src := confluence.New(cfg.Confluence, flagSpace, flagPageID)
	stats, err := ing.Run(ctx, src, "confluence", flagForce)
	if err != nil {
		return err
	}
	fmt.Printf("ingested=%d skipped=%d pruned=%d errors=%d\n",
		stats.Ingested, stats.Skipped, stats.Pruned, stats.Errors)
	return nil
}

// registerSource upserts a source in config and saves it.
func registerSource(src config.SourceConfig) {
	for i, s := range cfg.Sources {
		if s.Type == src.Type && s.Path == src.Path && s.Space == src.Space {
			cfg.Sources[i] = src
			config.Save(cfg)
			return
		}
	}
	cfg.Sources = append(cfg.Sources, src)
	config.Save(cfg)
}
```

- [ ] **Step 3: Create `cmd_search.go`**

```go
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/user/kb/internal/embedder"
	"github.com/user/kb/internal/store"
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search the knowledge base",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runSearch,
}

var (
	flagLimit     int
	flagMinScore  float64
	flagSource    string
)

func init() {
	searchCmd.Flags().IntVar(&flagLimit, "limit", 10, "number of results")
	searchCmd.Flags().Float64Var(&flagMinScore, "min-score", 0.0, "minimum similarity score (0-1)")
	searchCmd.Flags().StringVar(&flagSource, "source", "", "filter by source type: file|confluence")
}

func runSearch(cmd *cobra.Command, args []string) error {
	query := strings.Join(args, " ")

	st, err := store.NewSQLite(cfg.DB.Path)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer st.Close()

	emb, err := embedder.New(cfg.Embedder, cfg.OpenAI)
	if err != nil {
		return fmt.Errorf("create embedder: %w", err)
	}

	ctx := context.Background()
	vecs, err := emb.Embed(ctx, []string{query})
	if err != nil {
		return fmt.Errorf("embed query: %w", err)
	}

	results, err := st.Search(ctx, vecs[0], flagLimit, flagMinScore, flagSource)
	if err != nil {
		return fmt.Errorf("search: %w", err)
	}

	if len(results) == 0 {
		fmt.Println("No results found.")
		return nil
	}

	sep := strings.Repeat("─", 60)
	for _, r := range results {
		fmt.Printf("\nScore  %-12s %s\n", r.Document.SourceType, r.Document.Title)
		fmt.Println(sep)
		if url, ok := r.Document.Metadata["url"]; ok {
			fmt.Printf("  URL:      %s\n", url)
		}
		if path, ok := r.Document.Metadata["path"]; ok {
			fmt.Printf("  Path:     %s\n", path)
		}
		if author, ok := r.Document.Metadata["author"]; ok && author != "" {
			fmt.Printf("  Author:   %s\n", author)
		}
		if updated, ok := r.Document.Metadata["updated_at"]; ok && updated != "" {
			fmt.Printf("  Updated:  %s\n", updated)
		}
		fmt.Printf("  Score:    %.3f\n", r.Score)
		fmt.Printf("\n  %q\n", truncate(r.Chunk.Content, 300))
	}
	fmt.Println()
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
```

- [ ] **Step 4: Create `cmd_serve.go`**

(stub — full MCP implementation in Task 10)

```go
package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start MCP server (stdio)",
	RunE:  runServe,
}

func runServe(cmd *cobra.Command, args []string) error {
	fmt.Println("MCP server: implemented in Task 10")
	return nil
}
```

- [ ] **Step 5: Create `cmd_status.go`**

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/user/kb/internal/store"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show knowledge base statistics",
	RunE:  runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
	st, err := store.NewSQLite(cfg.DB.Path)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer st.Close()

	stats, err := st.Stats(context.Background())
	if err != nil {
		return err
	}
	b, _ := json.MarshalIndent(stats, "", "  ")
	fmt.Println(string(b))
	return nil
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create default config file at ~/.kb/config.yaml",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := config.InitDefault(); err != nil {
			return err
		}
		fmt.Println("Config created at ~/.kb/config.yaml")
		return nil
	},
}

func init() {
	configCmd.AddCommand(configInitCmd)
}
```

- [ ] **Step 6: Update `cmd/kb/main.go`**

```go
package main

import (
	"fmt"
	"os"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

Note: `rootCmd` and all `cmd_*.go` files must be in `package main` at the repo root (not inside `cmd/kb/`). Move all cmd files to root package.

- [ ] **Step 7: Build and smoke test**

```bash
CGO_ENABLED=1 go build -o kb .
./kb --help
./kb search --help
./kb ingest --help
```

Expected: help text for all commands with correct flags

- [ ] **Step 8: Commit**

```bash
git add cmd_root.go cmd_ingest.go cmd_search.go cmd_serve.go cmd_status.go cmd/kb/main.go
git commit -m "feat: CLI commands - ingest, search, serve, status, config"
```

---

### Task 10: MCP Server

**Files:**
- Modify: `internal/mcp/server.go`
- Modify: `cmd_serve.go`

**Interfaces:**
- Consumes:
  - `store.Store` (Search, Stats, GetDocument)
  - `embedder.Embedder`
  - `modelcontextprotocol/go-sdk` server API
- Produces:
  - MCP tools: `search_knowledge_base`, `list_sources`, `get_document`

- [ ] **Step 1: Implement `internal/mcp/server.go`**

```go
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/user/kb/internal/embedder"
	"github.com/user/kb/internal/store"
)

type Server struct {
	store   store.Store
	embedder embedder.Embedder
}

func New(st store.Store, emb embedder.Embedder) *Server {
	return &Server{store: st, embedder: emb}
}

// Run starts the MCP server on stdio and blocks until ctx is cancelled.
func (s *Server) Run(ctx context.Context) error {
	srv := mcp.NewServer("kb", "1.0.0", nil)

	// Tool: search_knowledge_base
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "search_knowledge_base",
		Description: "Search the private knowledge base using semantic similarity.",
		InputSchema: mcp.MustParseInputSchema(`{
			"type": "object",
			"properties": {
				"query":      {"type": "string", "description": "Search query"},
				"limit":      {"type": "integer", "description": "Max results (default 10)", "default": 10},
				"min_score":  {"type": "number", "description": "Minimum similarity score 0-1 (default 0)"},
				"source":     {"type": "string", "description": "Filter by source: file|confluence"}
			},
			"required": ["query"]
		}`),
	}, func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[searchArgs]) (*mcp.CallToolResult, error) {
		args := params.Arguments
		if args.Limit == 0 {
			args.Limit = 10
		}
		vecs, err := s.embedder.Embed(ctx, []string{args.Query})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("embed error: %v", err)), nil
		}
		results, err := s.store.Search(ctx, vecs[0], args.Limit, args.MinScore, args.Source)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("search error: %v", err)), nil
		}
		b, _ := json.Marshal(results)
		return mcp.NewToolResultText(string(b)), nil
	})

	// Tool: list_sources
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_sources",
		Description: "List all ingested sources with document and chunk counts.",
		InputSchema: mcp.MustParseInputSchema(`{"type":"object","properties":{}}`),
	}, func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[struct{}]) (*mcp.CallToolResult, error) {
		stats, err := s.store.Stats(ctx)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		b, _ := json.Marshal(stats)
		return mcp.NewToolResultText(string(b)), nil
	})

	// Tool: get_document
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_document",
		Description: "Retrieve the full content and metadata of a document by its ID.",
		InputSchema: mcp.MustParseInputSchema(`{
			"type": "object",
			"properties": {
				"document_id": {"type": "string", "description": "Document ID (source URI)"}
			},
			"required": ["document_id"]
		}`),
	}, func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[getDocArgs]) (*mcp.CallToolResult, error) {
		doc, err := s.store.GetDocument(ctx, params.Arguments.DocumentID)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if doc == nil {
			return mcp.NewToolResultError("document not found"), nil
		}
		b, _ := json.Marshal(doc)
		return mcp.NewToolResultText(string(b)), nil
	})

	transport := mcp.NewStdioTransport()
	_, err := srv.Connect(ctx, transport)
	return err
}

type searchArgs struct {
	Query    string  `json:"query"`
	Limit    int     `json:"limit"`
	MinScore float64 `json:"min_score"`
	Source   string  `json:"source"`
}

type getDocArgs struct {
	DocumentID string `json:"document_id"`
}
```

- [ ] **Step 2: Update `cmd_serve.go`**

```go
package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/user/kb/internal/embedder"
	mcpserver "github.com/user/kb/internal/mcp"
	"github.com/user/kb/internal/store"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start MCP server (stdio)",
	RunE:  runServe,
}

func runServe(cmd *cobra.Command, args []string) error {
	st, err := store.NewSQLite(cfg.DB.Path)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer st.Close()

	emb, err := embedder.New(cfg.Embedder, cfg.OpenAI)
	if err != nil {
		return fmt.Errorf("create embedder: %w", err)
	}

	srv := mcpserver.New(st, emb)
	return srv.Run(cmd.Context())
}
```

- [ ] **Step 3: Build**

```bash
CGO_ENABLED=1 go build -o kb .
```

Expected: no errors

- [ ] **Step 4: Smoke test MCP server**

```bash
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}' | ./kb serve
```

Expected: JSON response with server capabilities and tool list

- [ ] **Step 5: Commit**

```bash
git add internal/mcp/ cmd_serve.go
git commit -m "feat: MCP server with search_knowledge_base, list_sources, get_document tools"
```

---

### Task 11: Integration Test & OpenCode MCP Config

**Files:**
- Create: `integration_test.go`
- Create: `README.md`

**Interfaces:**
- Consumes: entire built binary

- [ ] **Step 1: Write integration test**

```go
// integration_test.go
//go:build integration

package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func buildBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "kb")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	return bin
}

func TestIngestAndSearch(t *testing.T) {
	if os.Getenv("KB_OPENAI_API_KEY") == "" {
		t.Skip("KB_OPENAI_API_KEY not set")
	}
	bin := buildBinary(t)
	dir := t.TempDir()

	// Write a test document
	doc := filepath.Join(dir, "test.md")
	os.WriteFile(doc, []byte("# Kubernetes\n\nKubernetes is a container orchestration platform."), 0644)

	dbPath := filepath.Join(t.TempDir(), "test.db")

	// Ingest
	cmd := exec.Command(bin, "ingest", "file", dir)
	cmd.Env = append(os.Environ(), "KB_DB_PATH="+dbPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ingest failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "ingested=1") {
		t.Errorf("expected ingested=1 in output: %s", out)
	}

	// Search
	cmd = exec.Command(bin, "search", "container orchestration")
	cmd.Env = append(os.Environ(), "KB_DB_PATH="+dbPath)
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("search failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "Kubernetes") {
		t.Errorf("expected Kubernetes in search results: %s", out)
	}
}
```

- [ ] **Step 2: Run unit tests (all)**

```bash
CGO_ENABLED=1 go test ./... -v
```

Expected: PASS (integration test skipped without build tag)

- [ ] **Step 3: Create README.md**

Create `README.md` with:
- Installation: `CGO_ENABLED=1 go build -o kb .`
- Quick start: `kb config init`, `kb ingest file ./docs/`, `kb search "query"`
- OpenCode MCP config snippet:
  ```json
  {
    "mcpServers": {
      "kb": {
        "command": "/path/to/kb",
        "args": ["serve"]
      }
    }
  }
  ```
- Environment variables table
- Adding Confluence source example

- [ ] **Step 4: Final commit**

```bash
git add integration_test.go README.md
git commit -m "feat: integration test + README with OpenCode MCP setup"
```

---

## Self-Review

### Spec Coverage

| Spec Requirement | Task |
|---|---|
| Single Go binary | Task 1, 9 |
| File adapter (md, txt, pdf, recursive, --ext) | Task 6, 9 |
| Confluence adapter (Cloud + PAT) | Task 7, 9 |
| Recursive Character Splitter | Task 3 |
| OpenAI text-embedding-3-large | Task 5 |
| Embedder interface + factory (extensible) | Task 1, 5 |
| SQLite + sqlite-vec storage | Task 4 |
| Incremental sync (hash-based) | Task 8 |
| Full re-index (--force) | Task 8, 9 |
| Prune deleted documents | Task 8 |
| `kb ingest` (all sources) | Task 9 |
| `kb ingest file` (register + sync) | Task 9 |
| `kb ingest confluence` (register + sync) | Task 9 |
| `kb search` (rich output, --limit, --min-score, --source) | Task 9 |
| `kb serve` (MCP stdio) | Task 10 |
| `kb status` | Task 9 |
| `kb config init` | Task 9 |
| MCP: search_knowledge_base | Task 10 |
| MCP: list_sources | Task 10 |
| MCP: get_document | Task 10 |
| Config YAML + env-var override | Task 2 |
| Sources auto-registered in config | Task 9 |
| Duplicate source update (not append) | Task 9 |
| Cloud vs DC auth detection (PAT vs token) | Task 7 |

All spec requirements covered. ✓
