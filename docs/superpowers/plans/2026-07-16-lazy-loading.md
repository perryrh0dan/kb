# Lazy Loading Source Interface Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the single-phase `Source.Documents()` interface with a two-phase `Scan()`/`Load()` interface so that expensive content extraction (Vision API, full page body fetch) only happens when the hash check confirms a document has actually changed.

**Architecture:** `Source` interface gains `Scan()` which streams cheap `DocumentMeta` (ID + hash + metadata, no content), and `Load()` which fetches full content on demand. The ingestor calls `Scan()` for all documents, checks hashes, and only calls `Load()` for documents that have changed. `Documents()` is removed entirely. File adapter computes hash from raw bytes in `Scan()` and calls `extractPDFContent()` only in `Load()`. Confluence adapter computes hash from `pageID:version.createdAt` in `Scan()` and fetches body only in `Load()`.

**Tech Stack:** Go stdlib only — no new dependencies.

## Global Constraints

- Go module path: `github.com/user/kb`
- Build: `CGO_ENABLED=1 go build -o kb .`
- All tests must pass: `CGO_ENABLED=1 go test ./...`
- `Documents()` method removed from `Source` interface — no backward compat
- `DocumentMeta` is a new struct embedded in `Document`
- `Document.Content` is always populated after `Load()` returns
- Confluence hash: `SHA256(pageID + ":" + version.createdAt)` — no body fetch needed in Scan
- File hash: `SHA256(rawBytes)` — already the case, `Scan()` just reads bytes
- Ingestor calls `Load()` only when `existing.ContentHash != meta.ContentHash`

---

### Task 1: New Source interface + DocumentMeta struct

**Files:**
- Modify: `internal/adapters/adapter.go`

**Interfaces:**
- Produces:
  - `adapters.DocumentMeta` struct
  - `adapters.Document` struct (embeds `DocumentMeta`, adds `Content string`)
  - `adapters.Source` interface with `Scan()`, `Load()`, `ScopePrefix()`
  - `Documents()` removed

- [ ] **Step 1: Replace `internal/adapters/adapter.go`**

```go
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
```

- [ ] **Step 2: Verify the file compiles in isolation**

```bash
go build ./internal/adapters/... 2>&1
```

Expected: errors from other packages that reference the old interface — that's expected. The `adapters` package itself must compile cleanly.

- [ ] **Step 3: Commit**

```bash
git add internal/adapters/adapter.go
git commit -m "refactor(adapters): two-phase Source interface — Scan()+Load() replaces Documents()"
```

---

### Task 2: File adapter — Scan() + Load()

**Files:**
- Modify: `internal/adapters/file/file.go`
- Modify: `internal/adapters/file/file_test.go`

**Interfaces:**
- Consumes: `adapters.DocumentMeta`, `adapters.Document`, `adapters.Source` (Task 1)
- Produces:
  - `(*fileSource).Scan(ctx) (<-chan adapters.DocumentMeta, error)`
  - `(*fileSource).Load(ctx, meta adapters.DocumentMeta) (adapters.Document, error)`
  - `Documents()` removed

- [ ] **Step 1: Write failing tests for new interface**

Replace `internal/adapters/file/file_test.go`:

```go
package file_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/user/kb/internal/adapters"
	"github.com/user/kb/internal/adapters/file"
)

func TestFileAdapterScanFindsMarkdown(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.md"), []byte("# Hello\nworld"), 0644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("plain text"), 0644)
	os.WriteFile(filepath.Join(dir, "c.go"), []byte("package main"), 0644)

	src := file.New(dir, false, []string{"md", "txt"}, file.Options{})
	ch, err := src.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	var metas []adapters.DocumentMeta
	for m := range ch {
		metas = append(metas, m)
	}
	if len(metas) != 2 {
		t.Errorf("got %d metas, want 2", len(metas))
	}
}

func TestFileAdapterScanRecursive(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	os.MkdirAll(sub, 0755)
	os.WriteFile(filepath.Join(dir, "root.md"), []byte("root"), 0644)
	os.WriteFile(filepath.Join(sub, "child.md"), []byte("child"), 0644)

	src := file.New(dir, true, []string{"md"}, file.Options{})
	ch, err := src.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	var count int
	for range ch {
		count++
	}
	if count != 2 {
		t.Errorf("got %d metas, want 2", count)
	}
}

func TestFileAdapterScanMetaFields(t *testing.T) {
	dir := t.TempDir()
	content := []byte("# My Doc\n\nSome content here.")
	os.WriteFile(filepath.Join(dir, "test.md"), content, 0644)

	src := file.New(dir, false, []string{"md"}, file.Options{})
	ch, _ := src.Scan(context.Background())
	meta := <-ch

	if meta.SourceType != "file" {
		t.Errorf("source_type = %q, want %q", meta.SourceType, "file")
	}
	if meta.ContentHash == "" {
		t.Error("ContentHash should not be empty")
	}
	expectedID := "file://" + filepath.Join(dir, "test.md")
	if meta.ID != expectedID {
		t.Errorf("ID = %q, want %q", meta.ID, expectedID)
	}
}

func TestFileAdapterLoadReturnsContent(t *testing.T) {
	dir := t.TempDir()
	content := []byte("# My Doc\n\nSome content here.")
	os.WriteFile(filepath.Join(dir, "test.md"), content, 0644)

	src := file.New(dir, false, []string{"md"}, file.Options{})
	ch, _ := src.Scan(context.Background())
	meta := <-ch

	doc, err := src.Load(context.Background(), meta)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if doc.Content != string(content) {
		t.Errorf("content mismatch: got %q, want %q", doc.Content, string(content))
	}
	if doc.ID != meta.ID {
		t.Errorf("Load returned wrong ID: got %q, want %q", doc.ID, meta.ID)
	}
}

func TestFileAdapterLoadHashMatchesScan(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.md"), []byte("hello"), 0644)

	src := file.New(dir, false, []string{"md"}, file.Options{})
	ch, _ := src.Scan(context.Background())
	meta := <-ch

	doc, err := src.Load(context.Background(), meta)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// Hash in Load result must match hash from Scan
	if doc.ContentHash != meta.ContentHash {
		t.Errorf("ContentHash mismatch: scan=%q load=%q", meta.ContentHash, doc.ContentHash)
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
CGO_ENABLED=1 go test ./internal/adapters/file/... -run "TestFileAdapterScan|TestFileAdapterLoad" -v 2>&1 | grep -E "FAIL|does not implement"
```

Expected: build/compile error — `Scan` and `Load` not yet implemented

- [ ] **Step 3: Implement new `file.go`**

Replace the entire `internal/adapters/file/file.go`:

```go
package file

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	oai "github.com/sashabaranov/go-openai"
	"github.com/user/kb/config"
	"github.com/user/kb/internal/adapters"
	"github.com/user/kb/internal/store"
)

// VisionOptions configures GPT-4o Vision analysis for PDF images.
type VisionOptions struct {
	Config config.VisionConfig
	Client *oai.Client
}

// Options configures optional capabilities of the file adapter.
type Options struct {
	Vision *VisionOptions // nil = Vision disabled
}

type fileSource struct {
	path       string
	recursive  bool
	extensions map[string]bool
	opts       Options
}

// New creates a file Source. extensions should be like []string{"md","txt","pdf"}.
func New(path string, recursive bool, extensions []string, opts Options) adapters.Source {
	exts := make(map[string]bool, len(extensions))
	for _, e := range extensions {
		exts[strings.ToLower(strings.TrimPrefix(e, "."))] = true
	}
	return &fileSource{path: path, recursive: recursive, extensions: exts, opts: opts}
}

// ScopePrefix returns the document ID prefix for this file source.
func (f *fileSource) ScopePrefix() string {
	abs, err := filepath.Abs(f.path)
	if err != nil {
		abs = f.path
	}
	if len(abs) > 0 && abs[len(abs)-1] != '/' {
		abs += "/"
	}
	return "file://" + abs
}

// Scan walks the source directory and streams DocumentMeta for each matching file.
// It reads raw file bytes to compute the ContentHash but does NOT parse PDFs or
// call Vision — those happen in Load() only when the hash has changed.
func (f *fileSource) Scan(ctx context.Context) (<-chan adapters.DocumentMeta, error) {
	log := slog.Default()
	ch := make(chan adapters.DocumentMeta)
	go func() {
		defer close(ch)
		_ = filepath.WalkDir(f.path, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				log.Warn("skipping unreadable path", "path", p, "error", err)
				return nil
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

			rawBytes, err := os.ReadFile(p)
			if err != nil {
				log.Warn("failed to read file", "path", p, "error", err)
				return nil
			}

			absPath, err := filepath.Abs(p)
			if err != nil {
				log.Warn("failed to resolve absolute path", "path", p, "error", err)
				return nil
			}

			info, _ := d.Info()
			modTime := time.Time{}
			if info != nil {
				modTime = info.ModTime()
			}

			meta := adapters.DocumentMeta{
				ID:          "file://" + absPath,
				Title:       filepath.Base(p),
				ContentHash: store.ContentHash(string(rawBytes)),
				SourceType:  "file",
				Metadata: map[string]string{
					"path":     absPath,
					"filename": filepath.Base(p),
					"modified": modTime.UTC().Format(time.RFC3339),
				},
				IngestedAt: time.Now().UTC(),
			}
			log.Debug("scanned file", "path", absPath)
			select {
			case ch <- meta:
			case <-ctx.Done():
				return ctx.Err()
			}
			return nil
		})
	}()
	return ch, nil
}

// Load extracts the full content for the file identified by meta.
// For PDFs this includes text extraction and optional Vision analysis.
// This is the expensive operation — only called when the hash has changed.
func (f *fileSource) Load(ctx context.Context, meta adapters.DocumentMeta) (adapters.Document, error) {
	log := slog.Default()
	absPath := strings.TrimPrefix(meta.ID, "file://")

	rawBytes, err := os.ReadFile(absPath)
	if err != nil {
		return adapters.Document{}, fmt.Errorf("read file %s: %w", absPath, err)
	}

	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(absPath), "."))
	var content string

	if ext == "pdf" {
		log.Debug("loading PDF with content extraction", "path", absPath)
		text, err := extractPDFContent(ctx, absPath, f.opts)
		if err != nil {
			return adapters.Document{}, fmt.Errorf("extract pdf content %s: %w", absPath, err)
		}
		content = text
	} else {
		content = string(rawBytes)
	}

	return adapters.Document{
		DocumentMeta: meta,
		Content:      content,
	}, nil
}
```

- [ ] **Step 4: Run tests**

```bash
CGO_ENABLED=1 go test ./internal/adapters/file/... -v 2>&1 | grep -E "^(=== RUN|--- PASS|--- FAIL|ok|FAIL)"
```

Expected: all PASS (5 new tests + existing PDF/vision tests)

- [ ] **Step 5: Commit**

```bash
git add internal/adapters/file/file.go internal/adapters/file/file_test.go
git commit -m "refactor(file): implement Scan()+Load() two-phase interface

Scan() reads raw bytes for hash only — no PDF parsing, no Vision calls.
Load() does full extraction (text + Vision) only when called by ingestor
after hash mismatch. PDFs with Vision now skip GPT-4o on unchanged files."
```

---

### Task 3: Confluence adapter — Scan() + Load()

**Files:**
- Modify: `internal/adapters/confluence/confluence.go`
- Modify: `internal/adapters/confluence/confluence_test.go`

**Interfaces:**
- Consumes: `adapters.DocumentMeta`, `adapters.Document`, `adapters.Source` (Task 1)
- Produces:
  - `(*confluenceSource).Scan(ctx) (<-chan adapters.DocumentMeta, error)`
  - `(*confluenceSource).Load(ctx, meta adapters.DocumentMeta) (adapters.Document, error)`
  - `Documents()` removed
  - Hash strategy: `SHA256(pageID + ":" + version.createdAt)` — no body needed

- [ ] **Step 1: Write failing tests**

Replace `internal/adapters/confluence/confluence_test.go`:

```go
package confluence_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/user/kb/config"
	"github.com/user/kb/internal/adapters/confluence"
)

func mockConfluenceServer() *httptest.Server {
	mux := http.NewServeMux()
	// Scan endpoint — no body needed, just metadata
	mux.HandleFunc("/wiki/api/v2/spaces/ENG/pages", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"results": []map[string]interface{}{
				{
					"id":      "12345",
					"title":   "Test Page",
					"version": map[string]interface{}{"createdAt": "2026-01-01T00:00:00Z"},
					"_links":  map[string]interface{}{"webui": "/display/ENG/Test+Page"},
				},
			},
			"_links": map[string]interface{}{},
		}
		json.NewEncoder(w).Encode(resp)
	})
	// Load endpoint — single page with body
	mux.HandleFunc("/wiki/api/v2/pages/12345", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"id":    "12345",
			"title": "Test Page",
			"body": map[string]interface{}{
				"storage": map[string]interface{}{
					"value": "<p>Hello <strong>world</strong></p>",
				},
			},
			"version": map[string]interface{}{"createdAt": "2026-01-01T00:00:00Z"},
			"_links":  map[string]interface{}{"webui": "/display/ENG/Test+Page"},
		}
		json.NewEncoder(w).Encode(resp)
	})
	return httptest.NewServer(mux)
}

func TestConfluenceScanFetchesPageMeta(t *testing.T) {
	srv := mockConfluenceServer()
	defer srv.Close()

	cfg := config.ConfluenceConfig{BaseURL: srv.URL, Username: "u", APIToken: "t"}
	src := confluence.New(cfg, "ENG", "")
	ch, err := src.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	var metas []string
	for m := range ch {
		metas = append(metas, m.ID)
	}
	if len(metas) != 1 {
		t.Errorf("got %d metas, want 1", len(metas))
	}
	if metas[0] != "confluence://ENG/12345" {
		t.Errorf("ID = %q, want %q", metas[0], "confluence://ENG/12345")
	}
}

func TestConfluenceScanHashDeterministic(t *testing.T) {
	srv := mockConfluenceServer()
	defer srv.Close()

	cfg := config.ConfluenceConfig{BaseURL: srv.URL, Username: "u", APIToken: "t"}
	src := confluence.New(cfg, "ENG", "")

	ch1, _ := src.Scan(context.Background())
	meta1 := <-ch1

	ch2, _ := src.Scan(context.Background())
	meta2 := <-ch2

	if meta1.ContentHash != meta2.ContentHash {
		t.Errorf("hash not deterministic: %q != %q", meta1.ContentHash, meta2.ContentHash)
	}
	if meta1.ContentHash == "" {
		t.Error("ContentHash should not be empty")
	}
}

func TestConfluenceLoadReturnsContent(t *testing.T) {
	srv := mockConfluenceServer()
	defer srv.Close()

	cfg := config.ConfluenceConfig{BaseURL: srv.URL, Username: "u", APIToken: "t"}
	src := confluence.New(cfg, "ENG", "")

	ch, _ := src.Scan(context.Background())
	meta := <-ch

	doc, err := src.Load(context.Background(), meta)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if doc.Content == "" {
		t.Error("content should not be empty after Load")
	}
	if strings.Contains(doc.Content, "<") {
		t.Errorf("HTML not stripped: %q", doc.Content[:20])
	}
}

func TestConfluencePATAuth(t *testing.T) {
	var receivedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		resp := map[string]interface{}{
			"results": []map[string]interface{}{},
			"_links":  map[string]interface{}{},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	cfg := config.ConfluenceConfig{BaseURL: srv.URL, PAT: "my-pat-token"}
	src := confluence.New(cfg, "ENG", "")
	ch, err := src.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	for range ch {}

	if receivedAuth != "Bearer my-pat-token" {
		t.Errorf("Authorization header = %q, want %q", receivedAuth, "Bearer my-pat-token")
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
go test ./internal/adapters/confluence/... -v 2>&1 | grep -E "FAIL|does not implement"
```

Expected: compile error — Scan/Load not implemented

- [ ] **Step 3: Implement new `confluence.go`**

Replace the entire `internal/adapters/confluence/confluence.go`:

```go
package confluence

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
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
// pageID is optional; if set, only that page is scanned/loaded.
func New(cfg config.ConfluenceConfig, space, pageID string) adapters.Source {
	return &confluenceSource{
		cfg:    cfg,
		space:  space,
		pageID: pageID,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// ScopePrefix returns the document ID prefix for pruning.
func (c *confluenceSource) ScopePrefix() string {
	if c.pageID != "" {
		return fmt.Sprintf("confluence://%s/%s", c.space, c.pageID)
	}
	return fmt.Sprintf("confluence://%s/", c.space)
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

// pageMeta is used for Scan — no body, just identity and version info.
type pageMeta struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Version struct {
		CreatedAt string `json:"createdAt"`
	} `json:"version"`
	Links struct {
		WebUI string `json:"webui"`
	} `json:"_links"`
}

type pagesMetaResponse struct {
	Results []pageMeta `json:"results"`
	Links   struct {
		Next string `json:"next"`
	} `json:"_links"`
}

// pageBody is used for Load — includes the storage body.
type pageBody struct {
	pageMeta
	Body struct {
		Storage struct {
			Value string `json:"value"`
		} `json:"storage"`
	} `json:"body"`
}

// Scan fetches page metadata (no body) and streams DocumentMeta.
// ContentHash is computed from pageID + ":" + version.createdAt —
// deterministic and changes only when the page is actually updated.
func (c *confluenceSource) Scan(ctx context.Context) (<-chan adapters.DocumentMeta, error) {
	log := slog.Default()
	ch := make(chan adapters.DocumentMeta)
	go func() {
		defer close(ch)
		// For a single page, use the pages/{id} endpoint (no body-format needed for meta)
		url := fmt.Sprintf("%s/wiki/api/v2/spaces/%s/pages?limit=50", c.cfg.BaseURL, c.space)
		if c.pageID != "" {
			url = fmt.Sprintf("%s/wiki/api/v2/pages/%s", c.cfg.BaseURL, c.pageID)
		}
		for url != "" {
			resp, err := c.doRequest(ctx, url)
			if err != nil {
				log.Warn("confluence HTTP request failed", "url", url, "error", err)
				return
			}
			if resp.StatusCode >= 400 {
				log.Warn("confluence HTTP error", "url", url, "status", resp.StatusCode)
				resp.Body.Close()
				return
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			var pr pagesMetaResponse
			if c.pageID != "" {
				var single pageMeta
				if err := json.Unmarshal(body, &single); err != nil {
					log.Warn("failed to parse single page meta", "page_id", c.pageID, "error", err)
					return
				}
				pr.Results = []pageMeta{single}
			} else {
				if err := json.Unmarshal(body, &pr); err != nil {
					log.Warn("failed to parse pages meta response", "url", url, "error", err)
					return
				}
			}

			for _, page := range pr.Results {
				log.Debug("scanned confluence page", "id", page.ID, "title", page.Title)
				// Hash from pageID + version timestamp — deterministic, no body needed
				hashInput := page.ID + ":" + page.Version.CreatedAt
				meta := adapters.DocumentMeta{
					ID:          fmt.Sprintf("confluence://%s/%s", c.space, page.ID),
					Title:       page.Title,
					ContentHash: store.ContentHash(hashInput),
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
				case ch <- meta:
				case <-ctx.Done():
					return
				}
			}

			if pr.Links.Next != "" && c.pageID == "" {
				url = c.cfg.BaseURL + pr.Links.Next
			} else {
				url = ""
			}
		}
	}()
	return ch, nil
}

// Load fetches the full page body and returns a Document with stripped HTML content.
// This is the expensive operation — only called when the hash has changed.
func (c *confluenceSource) Load(ctx context.Context, meta adapters.DocumentMeta) (adapters.Document, error) {
	log := slog.Default()
	// Extract page ID from the document ID: "confluence://SPACE/PAGEID"
	parts := strings.Split(meta.ID, "/")
	pageID := parts[len(parts)-1]

	url := fmt.Sprintf("%s/wiki/api/v2/pages/%s?body-format=storage", c.cfg.BaseURL, pageID)
	resp, err := c.doRequest(ctx, url)
	if err != nil {
		return adapters.Document{}, fmt.Errorf("fetch page %s: %w", pageID, err)
	}
	if resp.StatusCode >= 400 {
		resp.Body.Close()
		return adapters.Document{}, fmt.Errorf("fetch page %s: HTTP %d", pageID, resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	var page pageBody
	if err := json.Unmarshal(body, &page); err != nil {
		return adapters.Document{}, fmt.Errorf("parse page %s: %w", pageID, err)
	}

	content := stripHTML(page.Body.Storage.Value)
	log.Debug("loaded confluence page", "id", meta.ID, "content_len", len(content))

	return adapters.Document{
		DocumentMeta: meta,
		Content:      content,
	}, nil
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/adapters/confluence/... -v 2>&1 | grep -E "^(=== RUN|--- PASS|--- FAIL|ok|FAIL)"
```

Expected: all PASS (4 tests)

- [ ] **Step 5: Commit**

```bash
git add internal/adapters/confluence/confluence.go internal/adapters/confluence/confluence_test.go
git commit -m "refactor(confluence): implement Scan()+Load() two-phase interface

Scan() fetches page metadata without body — hash from pageID:version.createdAt.
Load() fetches full body only when hash mismatch detected by ingestor.
Eliminates unnecessary body fetches for unchanged Confluence pages."
```

---

### Task 4: Ingestor — use Scan() + conditional Load()

**Files:**
- Modify: `internal/ingest/ingest.go`
- Modify: `internal/ingest/ingest_test.go`

**Interfaces:**
- Consumes:
  - `adapters.Source.Scan()` (Task 1)
  - `adapters.Source.Load()` (Task 1)
  - `adapters.DocumentMeta` (Task 1)
  - `adapters.Document` (Task 1)
  - `store.Store` (unchanged)

- [ ] **Step 1: Update `internal/ingest/ingest_test.go`**

`stubSource` must implement the new `Scan()`/`Load()` interface:

```go
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

// stubSource implements adapters.Source for testing.
type stubSource struct {
	docs   []adapters.Document
	prefix string
}

func (s *stubSource) Scan(ctx context.Context) (<-chan adapters.DocumentMeta, error) {
	ch := make(chan adapters.DocumentMeta, len(s.docs))
	for _, d := range s.docs {
		ch <- d.DocumentMeta
	}
	close(ch)
	return ch, nil
}

func (s *stubSource) Load(_ context.Context, meta adapters.DocumentMeta) (adapters.Document, error) {
	for _, d := range s.docs {
		if d.ID == meta.ID {
			return d, nil
		}
	}
	return adapters.Document{}, fmt.Errorf("stubSource: document not found: %s", meta.ID)
}

func (s *stubSource) ScopePrefix() string {
	if s.prefix != "" {
		return s.prefix
	}
	return "file:///"
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
		DocumentMeta: adapters.DocumentMeta{
			ID:          id,
			Title:       id,
			ContentHash: store.ContentHash(content),
			SourceType:  "file",
			Metadata:    map[string]string{},
			IngestedAt:  time.Now().UTC(),
		},
		Content: content,
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

	ing.Run(context.Background(), &stubSource{docs: []adapters.Document{doc}}, "file", false)
	stats, _ := ing.Run(context.Background(), &stubSource{docs: []adapters.Document{doc}}, "file", false)
	if stats.Skipped != 1 {
		t.Errorf("skipped = %d, want 1", stats.Skipped)
	}
}

func TestIngestLoadNotCalledOnSkip(t *testing.T) {
	st, _ := store.NewSQLite(t.TempDir() + "/test.db")
	defer st.Close()

	c := chunker.New(512, 50)
	emb := &stubEmbedder{dims: 3072}
	ing := ingest.New(st, c, emb)
	doc := makeDoc("file:///a.md", "hello world")

	// First ingest to populate DB
	ing.Run(context.Background(), &stubSource{docs: []adapters.Document{doc}}, "file", false)

	// Second ingest with a tracking source that counts Load() calls
	ts := &trackingSource{stubSource: &stubSource{docs: []adapters.Document{doc}}}
	ing.Run(context.Background(), ts, "file", false)

	if ts.loadCalls != 0 {
		t.Errorf("Load() called %d times on unchanged document, want 0", ts.loadCalls)
	}
}

// trackingSource wraps stubSource and counts Load() calls.
type trackingSource struct {
	*stubSource
	loadCalls int
}

func (ts *trackingSource) Load(ctx context.Context, meta adapters.DocumentMeta) (adapters.Document, error) {
	ts.loadCalls++
	return ts.stubSource.Load(ctx, meta)
}

func TestIngestPrunesDeletedDocuments(t *testing.T) {
	st, _ := store.NewSQLite(t.TempDir() + "/test.db")
	defer st.Close()

	c := chunker.New(512, 50)
	emb := &stubEmbedder{dims: 3072}
	ing := ingest.New(st, c, emb)

	scope := "file:///"

	ing.Run(context.Background(), &stubSource{
		docs:   []adapters.Document{makeDoc("file:///a.md", "aaa"), makeDoc("file:///b.md", "bbb")},
		prefix: scope,
	}, "file", false)

	stats, _ := ing.Run(context.Background(), &stubSource{
		docs:   []adapters.Document{makeDoc("file:///a.md", "aaa")},
		prefix: scope,
	}, "file", false)

	if stats.Pruned != 1 {
		t.Errorf("pruned = %d, want 1", stats.Pruned)
	}
	doc, _ := st.GetDocument(context.Background(), "file:///b.md")
	if doc != nil {
		t.Errorf("file:///b.md still exists after prune")
	}
}

func TestIngestScopedPruning(t *testing.T) {
	st, err := store.NewSQLite(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}
	defer st.Close()

	c := chunker.New(512, 50)
	emb := &stubEmbedder{dims: 3072}
	ing := ingest.New(st, c, emb)

	security := &stubSource{
		docs:   []adapters.Document{makeDoc("file:///docs/security/auth.md", "auth content")},
		prefix: "file:///docs/security/",
	}
	if _, err := ing.Run(context.Background(), security, "file", false); err != nil {
		t.Fatalf("first ingest: %v", err)
	}

	k8s := &stubSource{
		docs:   []adapters.Document{makeDoc("file:///docs/k8s/deploy.md", "deploy content")},
		prefix: "file:///docs/k8s/",
	}
	stats, err := ing.Run(context.Background(), k8s, "file", false)
	if err != nil {
		t.Fatalf("second ingest: %v", err)
	}
	if stats.Pruned != 0 {
		t.Errorf("pruned = %d, want 0", stats.Pruned)
	}
	doc, _ := st.GetDocument(context.Background(), "file:///docs/security/auth.md")
	if doc == nil {
		t.Errorf("security doc was wrongly pruned")
	}
}

func TestIngestForceReindex(t *testing.T) {
	st, err := store.NewSQLite(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}
	defer st.Close()
	c := chunker.New(512, 50)
	emb := &stubEmbedder{dims: 3072}
	ing := ingest.New(st, c, emb)
	doc := makeDoc("file:///a.md", "hello world")

	ing.Run(context.Background(), &stubSource{docs: []adapters.Document{doc}}, "file", false)
	stats, err := ing.Run(context.Background(), &stubSource{docs: []adapters.Document{doc}}, "file", true)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if stats.Ingested != 1 {
		t.Errorf("force=true: ingested = %d, want 1", stats.Ingested)
	}
	if stats.Skipped != 0 {
		t.Errorf("force=true: skipped = %d, want 0", stats.Skipped)
	}
}

func TestIngestZeroChunkFallback(t *testing.T) {
	st, err := store.NewSQLite(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}
	defer st.Close()
	c := chunker.New(512, 50)
	emb := &stubEmbedder{dims: 3072}
	ing := ingest.New(st, c, emb)
	doc := makeDoc("file:///empty.md", "")
	stats, err := ing.Run(context.Background(), &stubSource{docs: []adapters.Document{doc}}, "file", false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if stats.Errors != 0 {
		t.Errorf("errors = %d, want 0", stats.Errors)
	}
	if stats.Ingested != 1 {
		t.Errorf("ingested = %d, want 1", stats.Ingested)
	}
}

// Ensure the stubEmbedder satisfies the embedder.Embedder interface at compile time.
var _ embedder.Embedder = (*stubEmbedder)(nil)
```

Note: add `"fmt"` to the imports.

- [ ] **Step 2: Run tests to confirm they fail**

```bash
CGO_ENABLED=1 go test ./internal/ingest/... -v 2>&1 | grep -E "FAIL|does not implement|undefined"
```

Expected: compile errors

- [ ] **Step 3: Implement new `ingest.go`**

Replace `internal/ingest/ingest.go`:

```go
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

// Run ingests all documents from src using the two-phase Scan/Load approach:
// 1. Scan() streams cheap DocumentMeta for all documents.
// 2. Hash-check against DB — skip if unchanged (Load() never called for skipped docs).
// 3. Load() called only for changed/new documents — triggers expensive ops (Vision API etc).
// force=true skips the hash check and calls Load() for every document.
func (ing *Ingester) Run(ctx context.Context, src adapters.Source, sourceType string, force bool) (IngestStats, error) {
	log := slog.Default()
	var stats IngestStats

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
		return stats, fmt.Errorf("scan source: %w", err)
	}

	for meta := range metaCh {
		if ctx.Err() != nil {
			break
		}
		seen[meta.ID] = true

		// Phase 1: hash check — skip Load() entirely if unchanged
		if !force {
			existing, err := ing.store.GetDocument(ctx, meta.ID)
			if err == nil && existing != nil && existing.ContentHash == meta.ContentHash {
				log.Debug("document unchanged, skipping", "id", meta.ID)
				stats.Skipped++
				continue
			}
		}

		// Phase 2: Load() — expensive content extraction happens here
		log.Debug("loading document content", "id", meta.ID, "title", meta.Title)
		doc, err := src.Load(ctx, meta)
		if err != nil {
			log.Warn("failed to load document", "id", meta.ID, "error", err)
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
		storedDoc := adapters.Document{
			DocumentMeta: meta, // use meta for ContentHash (stable)
			Content:      doc.Content,
		}
		if err := ing.store.UpsertDocument(ctx, storedDoc); err != nil {
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

	// Phase 3: prune documents no longer in source
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
```

- [ ] **Step 4: Fix store.UpsertDocument call**

The `store.UpsertDocument` currently takes `adapters.Document`. Check the signature:

```bash
grep -n "UpsertDocument" /root/workspace/kb/internal/store/store.go
```

If it takes `adapters.Document`, and `adapters.Document` now embeds `DocumentMeta`, the upsert needs the full `adapters.Document`. Verify `sqlite.go` scans `doc.ID`, `doc.Title`, etc. — these come from embedded `DocumentMeta`. The build will catch any issues.

- [ ] **Step 5: Run tests**

```bash
CGO_ENABLED=1 go test ./internal/ingest/... -v 2>&1 | grep -E "^(=== RUN|--- PASS|--- FAIL|ok|FAIL)"
```

Expected: all PASS (8 tests including new `TestIngestLoadNotCalledOnSkip`)

- [ ] **Step 6: Build**

```bash
CGO_ENABLED=1 go build -o kb . 2>&1
```

Expected: errors from `store/sqlite.go` if it references old `Document` fields — fix those in the next step.

- [ ] **Step 7: Fix store/sqlite.go if needed**

If build fails because `sqlite.go` references `doc.Content` or similar fields that moved to the embedded struct, update the field accesses. With embedding, `doc.ID` still works (Go promotes fields), so most accesses should be fine. Check with:

```bash
CGO_ENABLED=1 go build ./internal/store/... 2>&1
```

- [ ] **Step 8: Full build and test**

```bash
CGO_ENABLED=1 go build -o kb . && CGO_ENABLED=1 go test ./... 2>&1 | grep -E "^(ok|FAIL)"
```

Expected: all `ok`

- [ ] **Step 9: Commit**

```bash
git add internal/ingest/ingest.go internal/ingest/ingest_test.go
git commit -m "refactor(ingest): use Scan()+Load() — Load() skipped for unchanged docs

The ingestor now calls Scan() to get cheap DocumentMeta for all docs,
checks the hash, and only calls Load() when the document has changed.
New test TestIngestLoadNotCalledOnSkip verifies Load() is never invoked
for unchanged documents — eliminating Vision API calls on re-ingest."
```

---

## Self-Review

### Spec Coverage

| Requirement | Task |
|---|---|
| `DocumentMeta` struct with ID, Title, ContentHash, SourceType, Metadata, IngestedAt | Task 1 |
| `Document` embeds `DocumentMeta`, adds `Content string` | Task 1 |
| `Source.Scan()` → `<-chan DocumentMeta` | Task 1 |
| `Source.Load()` → `Document` | Task 1 |
| `Documents()` removed | Task 1 |
| File `Scan()` reads raw bytes, computes hash, no PDF parse | Task 2 |
| File `Load()` calls `extractPDFContent()` including Vision | Task 2 |
| Confluence `Scan()` uses `SHA256(pageID:version.createdAt)` | Task 3 |
| Confluence `Load()` fetches body, strips HTML | Task 3 |
| Ingestor calls `Scan()` first | Task 4 |
| Ingestor skips `Load()` when hash matches | Task 4 |
| `TestIngestLoadNotCalledOnSkip` verifies no Load on skip | Task 4 |
| All existing ingest tests still pass | Task 4 |
| force=true still works (bypasses hash check, always calls Load) | Task 4 |
| Pruning still works | Task 4 |
| Scoped pruning still works | Task 4 |
