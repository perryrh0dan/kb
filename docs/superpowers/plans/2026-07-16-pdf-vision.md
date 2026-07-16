# PDF Vision Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend PDF ingestion to extract embedded raster images and vector graphics (via rsvg-convert), send them to GPT-4o Vision, and append the visual description to the page text before chunking.

**Architecture:** Three-task pipeline. Task 1 adds `VisionConfig` to config and an `Options` struct to the file adapter. Task 2 implements visual extraction (raster images from HTML, SVG rendering via rsvg-convert subprocess) and GPT-4o Vision calling in `pdf_vision.go`. Task 3 wires everything together: `extractPDFContent` replaces `extractPDFText` in `pdf.go`, and `cmd_ingest.go` passes the OpenAI client through to the file adapter.

**Tech Stack:** go-fitz (already present), `os/exec` for rsvg-convert subprocess, `sashabaranov/go-openai` (already present) for Vision API calls, `encoding/base64`, `regexp`.

## Global Constraints

- Go module path: `github.com/user/kb`
- Build: `CGO_ENABLED=1 go build -o kb .`
- All existing tests must pass: `CGO_ENABLED=1 go test ./...`
- No new external Go dependencies — use only stdlib + already-present modules
- `rsvg-convert` is an optional system tool — if absent, SVGs are skipped with WARN log, no error
- Vision default: `enabled: false` — no API calls unless explicitly enabled
- Vision model: `gpt-4o` (default, configurable)
- Vision DPI: `150` (default, configurable) — used for SVG rendering via rsvg-convert
- Max images per page sent to GPT-4o: `10` (hardcoded guard against runaway costs)
- On Vision API error: WARN log, continue without visual summary (text content preserved)
- `file.New()` signature changes to accept `file.Options` as last parameter — all callers updated

---

### Task 1: VisionConfig + file.Options struct

**Files:**
- Modify: `config/config.go`
- Modify: `internal/adapters/file/file.go`
- Modify: `internal/adapters/file/file_test.go`

**Interfaces:**
- Produces:
  - `config.VisionConfig` struct with fields `Enabled bool`, `Model string`, `DPI float64`
  - `config.Config.Vision VisionConfig` field
  - `file.Options` struct with field `Vision *VisionOptions`
  - `file.VisionOptions` struct with fields `Config config.VisionConfig`, `Client *openai.Client`
  - `file.New(path string, recursive bool, extensions []string, opts file.Options) adapters.Source` — updated signature
  - `file.New` with empty `file.Options{}` behaves identically to the old signature

- [ ] **Step 1: Add `VisionConfig` to `config/config.go`**

Add after `ChunkerConfig`:

```go
type VisionConfig struct {
	Enabled bool    `mapstructure:"enabled" yaml:"enabled"`
	Model   string  `mapstructure:"model"   yaml:"model"`
	DPI     float64 `mapstructure:"dpi"     yaml:"dpi"`
}
```

Add `Vision VisionConfig` field to `Config` struct:

```go
type Config struct {
	OpenAI     OpenAIConfig     `mapstructure:"openai"      yaml:"openai"`
	Confluence ConfluenceConfig `mapstructure:"confluence"  yaml:"confluence"`
	DB         DBConfig         `mapstructure:"db"          yaml:"db"`
	Embedder   EmbedderConfig   `mapstructure:"embedder"    yaml:"embedder"`
	Chunker    ChunkerConfig    `mapstructure:"chunker"     yaml:"chunker"`
	Vision     VisionConfig     `mapstructure:"vision"      yaml:"vision"`
	Sources    []SourceConfig   `mapstructure:"sources"     yaml:"sources"`
}
```

Add defaults to `newViper()`:

```go
v.SetDefault("vision.enabled", false)
v.SetDefault("vision.model", "gpt-4o")
v.SetDefault("vision.dpi", 150.0)
```

Add vision section to `InitDefault()` template string, after the chunker section:

```yaml
vision:
  enabled: false  # true to describe PDF images via GPT-4o Vision (requires api_key)
  model: gpt-4o
  dpi: 150        # resolution for SVG rendering (72-300)
```

- [ ] **Step 2: Add `Options` and `VisionOptions` to `internal/adapters/file/file.go`**

Add at the top of the file (after imports):

```go
import (
    oai "github.com/sashabaranov/go-openai"
    "github.com/user/kb/config"
    // ... existing imports
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
```

Update `fileSource` struct:

```go
type fileSource struct {
	path       string
	recursive  bool
	extensions map[string]bool
	opts       Options
}
```

Update `New()` signature:

```go
func New(path string, recursive bool, extensions []string, opts Options) adapters.Source {
	exts := make(map[string]bool, len(extensions))
	for _, e := range extensions {
		exts[strings.ToLower(strings.TrimPrefix(e, "."))] = true
	}
	return &fileSource{path: path, recursive: recursive, extensions: exts, opts: opts}
}
```

The `Documents` method body stays identical for now — `opts` is stored but not yet used (Task 2 wires it in).

- [ ] **Step 3: Update all callers of `file.New()` to pass `file.Options{}`**

In `cmd_ingest.go`, update line `s := file.New(src.Path, src.Recursive, exts)`:
```go
s := file.New(src.Path, src.Recursive, exts, file.Options{})
```

And line `src := file.New(path, flagRecursive, exts)`:
```go
src := file.New(path, flagRecursive, exts, file.Options{})
```

In `internal/adapters/file/file_test.go`, update all three `file.New(...)` calls:
```go
// TestFileAdapterFindsMarkdown
src := file.New(dir, false, []string{"md", "txt"}, file.Options{})

// TestFileAdapterRecursive
src := file.New(dir, true, []string{"md"}, file.Options{})

// TestFileAdapterDocumentFields
src := file.New(dir, false, []string{"md"}, file.Options{})
```

- [ ] **Step 4: Build and run tests**

```bash
CGO_ENABLED=1 go build -o kb .
CGO_ENABLED=1 go test ./... -v 2>&1 | grep -E "^(ok|FAIL|---)"
```

Expected: all packages `ok`, no FAIL

- [ ] **Step 5: Commit**

```bash
git add config/config.go internal/adapters/file/file.go internal/adapters/file/file_test.go cmd_ingest.go
git commit -m "feat(vision): add VisionConfig + file.Options struct, update file.New() signature"
```

---

### Task 2: Visual extraction + GPT-4o Vision call

**Files:**
- Create: `internal/adapters/file/pdf_vision.go`
- Create: `internal/adapters/file/pdf_vision_test.go`

**Interfaces:**
- Consumes:
  - `file.Options` (from Task 1)
  - `file.VisionOptions` (from Task 1)
  - `config.VisionConfig` (from Task 1)
- Produces:
  - `extractRasterImages(html string) [][]byte` — extracts base64-decoded JPEG/PNG bytes from `<img src="data:image/...;base64,...">` tags in HTML
  - `hasMeaningfulPaths(svg string) bool` — returns true if SVG contains `<path>` elements that are NOT font glyphs (id does not start with "font_")
  - `renderSVG(svg string, dpi float64) ([]byte, error)` — renders SVG via rsvg-convert subprocess; returns nil bytes + nil error if rsvg-convert not in PATH
  - `rsvgAvailable() bool` — checks once if rsvg-convert is in PATH (cached via sync.Once)
  - `describeVisuals(ctx context.Context, client *oai.Client, model string, images [][]byte) (string, error)` — sends up to 10 images to GPT-4o Vision, returns description

- [ ] **Step 1: Write failing tests in `internal/adapters/file/pdf_vision_test.go`**

```go
package file

import (
	"strings"
	"testing"
)

func TestExtractRasterImages_JPEG(t *testing.T) {
	// Minimal valid base64-encoded 1x1 white JPEG
	jpeg1x1 := "/9j/4AAQSkZJRgABAQEASABIAAD/2wBDAAgGBgcGBQgHBwcJCQgKDBQNDAsLDBkSEw8UHRofHh0aHBwgJC4nICIsIxwcKDcpLDAxNDQ0Hyc5PTgyPC4zNDL/2wBDAQkJCQwLDBgNDRgyIRwhMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjL/wAARCAABAAEDASIAAhEBAxEB/8QAFgABAQEAAAAAAAAAAAAAAAAABgUEA/8QAIRAAAQMEAgMAAAAAAAAAAAAAAQIDBAAFERIhMUH/xAAUAQEAAAAAAAAAAAAAAAAAAAAA/8QAFBEBAAAAAAAAAAAAAAAAAAAAAP/aAAwDAQACEQMRAD8Amw2fa5VyuzMaJFU6+6cJSgZJ/KUpSlKUpSlKUpT/2Q=="
	html := `<div><img style="position:absolute" src="data:image/jpeg;base64,` + jpeg1x1 + `"><p>some text</p></div>`

	images := extractRasterImages(html)
	if len(images) != 1 {
		t.Fatalf("got %d images, want 1", len(images))
	}
	if len(images[0]) == 0 {
		t.Error("decoded image bytes should not be empty")
	}
}

func TestExtractRasterImages_None(t *testing.T) {
	html := `<div><p>just text, no images</p></div>`
	images := extractRasterImages(html)
	if len(images) != 0 {
		t.Errorf("got %d images, want 0", len(images))
	}
}

func TestExtractRasterImages_MultipleFormats(t *testing.T) {
	html := `<div>
		<img src="data:image/jpeg;base64,/9j/abc123">
		<img src="data:image/png;base64,iVBORw0KGgo=">
	</div>`
	images := extractRasterImages(html)
	if len(images) != 2 {
		t.Errorf("got %d images, want 2", len(images))
	}
}

func TestHasMeaningfulPaths_FontOnly(t *testing.T) {
	// SVG with only font glyph paths (id starts with "font_")
	svg := `<svg><defs><path id="font_1_23" d="M 0 0 L 10 10"/></defs><use xlink:href="#font_1_23"/></svg>`
	if hasMeaningfulPaths(svg) {
		t.Error("SVG with only font glyphs should return false")
	}
}

func TestHasMeaningfulPaths_WithShapes(t *testing.T) {
	// SVG with a real shape path (no font_ id)
	svg := `<svg><path d="M 0 0 H 100 V 100 Z" fill="#ff0000"/></svg>`
	if !hasMeaningfulPaths(svg) {
		t.Error("SVG with shape paths should return true")
	}
}

func TestHasMeaningfulPaths_Empty(t *testing.T) {
	svg := `<svg><text>hello</text></svg>`
	if hasMeaningfulPaths(svg) {
		t.Error("SVG with no paths should return false")
	}
}

func TestRenderSVG_NoRsvg(t *testing.T) {
	// Force rsvg-convert unavailable by calling renderSVGWithBinary with a bad path
	svg := `<svg width="10" height="10"><rect width="10" height="10" fill="red"/></svg>`
	bytes, err := renderSVGWithBinary("nonexistent-rsvg-convert-binary", svg, 96)
	if err != nil {
		t.Fatalf("expected nil error when binary missing, got: %v", err)
	}
	if bytes != nil {
		t.Error("expected nil bytes when binary missing")
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
CGO_ENABLED=1 go test ./internal/adapters/file/... -run "TestExtract|TestHas|TestRender" -v 2>&1 | grep -E "^(=== RUN|--- PASS|--- FAIL|FAIL)"
```

Expected: FAIL — functions not yet defined

- [ ] **Step 3: Implement `internal/adapters/file/pdf_vision.go`**

```go
package file

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"os/exec"
	"regexp"
	"strings"
	"sync"

	oai "github.com/sashabaranov/go-openai"
)

const maxImagesPerPage = 10

const visionPrompt = `Describe ONLY the visual elements on this image: diagrams, charts, graphs, illustrations, tables with visual structure, and embedded photographs. Do NOT transcribe or repeat any text that appears — the text content is already captured separately. Focus exclusively on what the visual communicates that plain text cannot: spatial relationships, data trends, color coding, layout structure, and visual patterns.`

// imgTagRe matches <img src="data:image/TYPE;base64,DATA"> tags in HTML.
var imgTagRe = regexp.MustCompile(`<img\s[^>]*src="data:image/([a-zA-Z+]+);base64,([^"]+)"`)

// pathTagRe matches <path ...> elements in SVG.
var pathTagRe = regexp.MustCompile(`<path\s([^>]*)>|<path\s([^/]*)/?>`)

// fontIDRe matches path ids that are font glyphs (id="font_...").
var fontIDRe = regexp.MustCompile(`\bid="font_`)

var (
	rsvgOnce      sync.Once
	rsvgAvailFlag bool
)

// rsvgAvailable returns true if rsvg-convert is found in PATH. Result is cached.
func rsvgAvailable() bool {
	rsvgOnce.Do(func() {
		_, err := exec.LookPath("rsvg-convert")
		rsvgAvailFlag = err == nil
	})
	return rsvgAvailFlag
}

// extractRasterImages parses HTML output from go-fitz and returns decoded image bytes
// for every <img src="data:image/TYPE;base64,..."> tag found.
func extractRasterImages(html string) [][]byte {
	matches := imgTagRe.FindAllStringSubmatch(html, -1)
	var images [][]byte
	for _, m := range matches {
		// m[2] is the base64 payload
		decoded, err := base64.StdEncoding.DecodeString(m[2])
		if err != nil {
			// Some base64 in HTML may use URL-safe encoding
			decoded, err = base64.URLEncoding.DecodeString(m[2])
			if err != nil {
				slog.Default().Warn("failed to decode base64 image from PDF HTML", "error", err)
				continue
			}
		}
		images = append(images, decoded)
	}
	return images
}

// hasMeaningfulPaths returns true if the SVG contains <path> elements
// that are NOT font glyphs (i.e. id does not start with "font_").
// This distinguishes real vector graphics from MuPDF's text-glyph paths.
func hasMeaningfulPaths(svg string) bool {
	matches := pathTagRe.FindAllString(svg, -1)
	for _, m := range matches {
		if !fontIDRe.MatchString(m) {
			return true
		}
	}
	return false
}

// renderSVGWithBinary renders an SVG string to PNG bytes using the given binary path.
// Returns (nil, nil) if the binary does not exist or is not executable.
// Returns (nil, err) only on unexpected execution errors.
func renderSVGWithBinary(binary, svg string, dpi float64) ([]byte, error) {
	_, err := exec.LookPath(binary)
	if err != nil {
		return nil, nil // binary not available — skip silently
	}
	cmd := exec.Command(binary,
		"--format", "png",
		"--dpi-x", fmt.Sprintf("%.0f", dpi),
		"--dpi-y", fmt.Sprintf("%.0f", dpi),
		"-",
	)
	cmd.Stdin = strings.NewReader(svg)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("rsvg-convert: %w (stderr: %s)", err, stderr.String())
	}
	return out.Bytes(), nil
}

// renderSVG renders an SVG string to PNG using rsvg-convert.
// Returns (nil, nil) if rsvg-convert is not available.
func renderSVG(svg string, dpi float64) ([]byte, error) {
	if !rsvgAvailable() {
		return nil, nil
	}
	return renderSVGWithBinary("rsvg-convert", svg, dpi)
}

// describeVisuals sends up to maxImagesPerPage PNG/JPEG images to GPT-4o Vision
// and returns a plain-text description of the visual content.
// Images must be raw PNG or JPEG bytes.
func describeVisuals(ctx context.Context, client *oai.Client, model string, images [][]byte) (string, error) {
	if len(images) == 0 {
		return "", nil
	}
	// Cap at maxImagesPerPage to control cost
	if len(images) > maxImagesPerPage {
		slog.Default().Warn("truncating images sent to vision model", "total", len(images), "limit", maxImagesPerPage)
		images = images[:maxImagesPerPage]
	}

	// Build multi-part message: one image_url part per image
	parts := []oai.ChatMessagePart{
		{
			Type: oai.ChatMessagePartTypeText,
			Text: visionPrompt,
		},
	}
	for _, imgBytes := range images {
		// Detect MIME type from magic bytes
		mime := "image/png"
		if len(imgBytes) >= 3 && imgBytes[0] == 0xFF && imgBytes[1] == 0xD8 {
			mime = "image/jpeg"
		}
		encoded := base64.StdEncoding.EncodeToString(imgBytes)
		dataURL := fmt.Sprintf("data:%s;base64,%s", mime, encoded)
		parts = append(parts, oai.ChatMessagePart{
			Type: oai.ChatMessagePartTypeImageURL,
			ImageURL: &oai.ChatMessageImageURL{
				URL:    dataURL,
				Detail: oai.ImageURLDetailHigh,
			},
		})
	}

	resp, err := client.CreateChatCompletion(ctx, oai.ChatCompletionRequest{
		Model: model,
		Messages: []oai.ChatCompletionMessage{
			{
				Role:         oai.ChatMessageRoleUser,
				MultiContent: parts,
			},
		},
		MaxTokens: 1024,
	})
	if err != nil {
		return "", fmt.Errorf("vision API call: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("vision API returned no choices")
	}
	return strings.TrimSpace(resp.Choices[0].Message.Content), nil
}
```

- [ ] **Step 4: Run tests**

```bash
CGO_ENABLED=1 go test ./internal/adapters/file/... -run "TestExtract|TestHas|TestRender" -v 2>&1 | grep -E "^(=== RUN|--- PASS|--- FAIL|FAIL|ok)"
```

Expected: all PASS

- [ ] **Step 5: Build**

```bash
CGO_ENABLED=1 go build -o kb . 2>&1
```

Expected: clean

- [ ] **Step 6: Commit**

```bash
git add internal/adapters/file/pdf_vision.go internal/adapters/file/pdf_vision_test.go
git commit -m "feat(vision): visual extraction (raster+SVG) and GPT-4o Vision call"
```

---

### Task 3: Wire Vision into PDF extraction + CLI

**Files:**
- Modify: `internal/adapters/file/pdf.go`
- Modify: `internal/adapters/file/file.go`
- Modify: `internal/adapters/file/pdf_test.go`
- Modify: `cmd_ingest.go`

**Interfaces:**
- Consumes:
  - `extractRasterImages(html string) [][]byte` (Task 2)
  - `hasMeaningfulPaths(svg string) bool` (Task 2)
  - `renderSVG(svg string, dpi float64) ([]byte, error)` (Task 2)
  - `describeVisuals(ctx, client, model, images) (string, error)` (Task 2)
  - `file.Options` / `file.VisionOptions` (Task 1)
  - `config.VisionConfig` (Task 1)
- Produces:
  - `extractPDFContent(ctx, path string, opts Options) (string, error)` — replaces `extractPDFText`; returns full content string with optional visual summaries appended per page

- [ ] **Step 1: Replace `extractPDFText` with `extractPDFContent` in `internal/adapters/file/pdf.go`**

Replace the entire file:

```go
package file

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gen2brain/go-fitz"
)

// errNoContent is returned when a PDF has no text and no visual content was extracted.
var errNoContent = errors.New("pdf contains no extractable content")

// extractPDFContent extracts text from all pages of a PDF.
// If opts.Vision is non-nil and VisionConfig.Enabled is true, it also extracts
// visual elements per page (embedded images + SVG vector graphics) and appends
// a GPT-4o Vision description to each page's text.
// Returns errNoContent if the PDF yields no content at all.
func extractPDFContent(ctx context.Context, path string, opts Options) (string, error) {
	log := slog.Default()

	doc, err := fitz.New(path)
	if err != nil {
		if errors.Is(err, fitz.ErrNeedsPassword) {
			return "", fmt.Errorf("pdf is password-protected: %w", err)
		}
		return "", fmt.Errorf("open pdf: %w", err)
	}
	defer doc.Close()

	visionEnabled := opts.Vision != nil && opts.Vision.Config.Enabled && opts.Vision.Client != nil

	var docBuilder strings.Builder

	for n := 0; n < doc.NumPage(); n++ {
		var pageBuilder strings.Builder

		// --- Text extraction ---
		text, err := doc.Text(n)
		if err != nil {
			log.Warn("failed to extract text from pdf page", "path", path, "page", n, "error", err)
		} else {
			text = strings.TrimSpace(text)
			if text != "" {
				pageBuilder.WriteString(text)
			}
		}

		// --- Visual extraction + Vision ---
		if visionEnabled {
			var pageImages [][]byte

			// 1. Raster images from HTML
			html, err := doc.HTML(n, false)
			if err != nil {
				log.Warn("failed to get HTML from pdf page", "path", path, "page", n, "error", err)
			} else {
				raster := extractRasterImages(html)
				pageImages = append(pageImages, raster...)
				log.Debug("extracted raster images from pdf page", "path", path, "page", n, "count", len(raster))
			}

			// 2. SVG vector graphics
			svg, err := doc.SVG(n)
			if err != nil {
				log.Warn("failed to get SVG from pdf page", "path", path, "page", n, "error", err)
			} else if hasMeaningfulPaths(svg) {
				pngBytes, err := renderSVG(svg, opts.Vision.Config.DPI)
				if err != nil {
					log.Warn("failed to render SVG via rsvg-convert", "path", path, "page", n, "error", err)
				} else if pngBytes != nil {
					pageImages = append(pageImages, pngBytes)
					log.Debug("rendered SVG vector graphics from pdf page", "path", path, "page", n)
				} else {
					log.Debug("rsvg-convert not available, skipping SVG for page", "path", path, "page", n)
				}
			}

			// 3. Vision API call
			if len(pageImages) > 0 {
				summary, err := describeVisuals(ctx, opts.Vision.Client, opts.Vision.Config.Model, pageImages)
				if err != nil {
					log.Warn("vision API failed for pdf page", "path", path, "page", n, "error", err)
				} else if summary != "" {
					if pageBuilder.Len() > 0 {
						pageBuilder.WriteString("\n\n")
					}
					pageBuilder.WriteString("[Visual content: ")
					pageBuilder.WriteString(summary)
					pageBuilder.WriteString("]")
					log.Debug("appended vision summary to pdf page", "path", path, "page", n)
				}
			}
		}

		// Append page content to document
		if pageBuilder.Len() > 0 {
			if docBuilder.Len() > 0 {
				docBuilder.WriteString("\n\n")
			}
			docBuilder.WriteString(pageBuilder.String())
		}
	}

	if docBuilder.Len() == 0 {
		return "", errNoContent
	}
	return docBuilder.String(), nil
}
```

- [ ] **Step 2: Update `internal/adapters/file/file.go` to use `extractPDFContent`**

In the `Documents` method, replace the `ext == "pdf"` block:

```go
// old:
if ext == "pdf" {
    text, err := extractPDFText(p)
    if err != nil {
        if errors.Is(err, errNoText) {
            log.Warn("pdf has no extractable text, skipping", "path", p)
        } else {
            log.Warn("failed to extract pdf text", "path", p, "error", err)
        }
        return nil
    }
    content = text
}

// new:
if ext == "pdf" {
    text, err := extractPDFContent(ctx, p, f.opts)
    if err != nil {
        if errors.Is(err, errNoContent) {
            log.Warn("pdf has no extractable content, skipping", "path", p)
        } else {
            log.Warn("failed to extract pdf content", "path", p, "error", err)
        }
        return nil
    }
    content = text
}
```

Note: `errNoText` is now removed from `pdf.go` (replaced by `errNoContent`) — remove its declaration too.

- [ ] **Step 3: Update `internal/adapters/file/pdf_test.go`**

Update `TestExtractPDFText` to call `extractPDFContent` (vision disabled):

```go
package file

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestExtractPDFContent(t *testing.T) {
	// Vision disabled — behaves like old extractPDFText
	content, err := extractPDFContent(context.Background(), "testdata/sample.pdf", Options{})
	if err != nil {
		t.Fatalf("extractPDFContent: %v", err)
	}
	if !strings.Contains(content, "Hello from PDF") {
		t.Errorf("expected 'Hello from PDF' in content, got: %q", content)
	}
	if !strings.Contains(content, "test content for extraction") {
		t.Errorf("expected 'test content for extraction' in content, got: %q", content)
	}
}

func TestExtractPDFContentCorrupted(t *testing.T) {
	_, err := extractPDFContent(context.Background(), "testdata/corrupted.pdf", Options{})
	if err == nil {
		t.Fatal("expected error for corrupted PDF, got nil")
	}
	if errors.Is(err, errNoContent) {
		t.Errorf("expected open error, not errNoContent")
	}
}

func TestExtractPDFContentNotFound(t *testing.T) {
	_, err := extractPDFContent(context.Background(), "testdata/nonexistent.pdf", Options{})
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}
```

- [ ] **Step 4: Update `cmd_ingest.go` to wire Vision into file adapter**

Add a helper to build `file.Options` from config:

```go
func buildFileOptions(cfg *config.Config) file.Options {
	if !cfg.Vision.Enabled {
		return file.Options{}
	}
	oaiCfg := oai.DefaultConfig(cfg.OpenAI.APIKey)
	client := oai.NewClientWithConfig(oaiCfg)
	return file.Options{
		Vision: &file.VisionOptions{
			Config: cfg.Vision,
			Client: client,
		},
	}
}
```

Add import `oai "github.com/sashabaranov/go-openai"` to `cmd_ingest.go`.

Update all four `file.New(...)` calls to use `buildFileOptions(cfg)`:

```go
// in runSource:
s := file.New(src.Path, src.Recursive, exts, buildFileOptions(cfg))

// in runIngestFile:
src := file.New(path, flagRecursive, exts, buildFileOptions(cfg))
```

- [ ] **Step 5: Run all tests**

```bash
CGO_ENABLED=1 go test ./... 2>&1 | grep -E "^(ok|FAIL|---)"
```

Expected: all `ok`, no FAIL

- [ ] **Step 6: Build and smoke test**

```bash
CGO_ENABLED=1 go build -o kb .
./kb --help
```

Expected: clean build, help text shows normally

- [ ] **Step 7: Commit**

```bash
git add internal/adapters/file/pdf.go internal/adapters/file/file.go \
        internal/adapters/file/pdf_test.go cmd_ingest.go
git commit -m "feat(vision): wire GPT-4o Vision into PDF ingestion pipeline

- extractPDFContent() replaces extractPDFText(); processes text + visuals per page
- Raster images extracted from HTML, SVG rendered via rsvg-convert (skipped if absent)  
- Vision summary appended as '[Visual content: ...]' after page text
- Controlled via config: vision.enabled, vision.model, vision.dpi
- Vision errors are non-fatal: WARN log, page text preserved"
```

---

## Self-Review

### Spec Coverage

| Requirement | Task |
|---|---|
| `VisionConfig` with `enabled`, `model`, `dpi` | Task 1 |
| `vision.enabled: false` default (no API calls) | Task 1 |
| `file.Options` struct with `Vision *VisionOptions` | Task 1 |
| `file.New()` accepts `Options` as last param | Task 1 |
| All existing callers updated | Task 1 |
| Raster images extracted from HTML `<img src="data:...">` | Task 2 |
| `hasMeaningfulPaths` filters font glyphs from real vectors | Task 2 |
| `renderSVG` via rsvg-convert subprocess | Task 2 |
| rsvg-convert absent → nil, nil (skip with no error) | Task 2 |
| Max 10 images per page sent to GPT-4o | Task 2 |
| `describeVisuals` builds correct multi-part Vision request | Task 2 |
| Vision prompt instructs GPT-4o not to repeat text | Task 2 |
| JPEG detection via magic bytes (0xFF 0xD8) | Task 2 |
| `extractPDFContent` replaces `extractPDFText` | Task 3 |
| Text + Vision summary concatenated per page | Task 3 |
| Vision errors are non-fatal (WARN + continue) | Task 3 |
| `errNoContent` replaces `errNoText` | Task 3 |
| `buildFileOptions` wires OpenAI client from config | Task 3 |
| All tests pass | Tasks 1-3 |
