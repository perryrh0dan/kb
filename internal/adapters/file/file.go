package file

import (
	"context"
	"errors"
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
// Only docs whose IDs start with this prefix are pruned during ingest.
func (f *fileSource) ScopePrefix() string {
	abs, err := filepath.Abs(f.path)
	if err != nil {
		abs = f.path
	}
	// Ensure trailing slash so "file:///docs/k8s/" doesn't match "file:///docs/k8s2/"
	if len(abs) > 0 && abs[len(abs)-1] != '/' {
		abs += "/"
	}
	return "file://" + abs
}

func (f *fileSource) Documents(ctx context.Context) (<-chan adapters.Document, error) {
	log := slog.Default()
	ch := make(chan adapters.Document)
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
			// Read raw file bytes first — used for the content hash.
			// The hash is always computed over the original file bytes so it is
			// deterministic and independent of non-deterministic outputs (e.g.
			// GPT-4o Vision summaries that vary between runs).
			rawBytes, err := os.ReadFile(p)
			if err != nil {
				log.Warn("failed to read file", "path", p, "error", err)
				return nil
			}

			// Extract content — PDFs need text extraction and optional vision analysis.
			var content string
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
			} else {
				content = string(rawBytes)
			}
			info, _ := d.Info()
			modTime := time.Time{}
			if info != nil {
				modTime = info.ModTime()
			}
			absPath, err := filepath.Abs(p)
			if err != nil {
				log.Warn("failed to resolve absolute path", "path", p, "error", err)
				return nil
			}
			doc := adapters.Document{
				ID:          "file://" + absPath,
				Title:       filepath.Base(p),
				Content:     content,
				ContentHash: store.ContentHash(string(rawBytes)),
				SourceType:  "file",
				Metadata: map[string]string{
					"path":     absPath,
					"filename": filepath.Base(p),
					"modified": modTime.UTC().Format(time.RFC3339),
				},
				IngestedAt: time.Now().UTC(),
			}
			log.Debug("found file", "path", absPath)
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
