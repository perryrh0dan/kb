package file

import (
	"context"
	"io/fs"
	"log/slog"
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
			content, err := os.ReadFile(p)
			if err != nil {
				log.Warn("failed to read file", "path", p, "error", err)
				return nil
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
