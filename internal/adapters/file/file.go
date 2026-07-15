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
