package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/user/kb/config"
	"github.com/user/kb/internal/adapters/confluence"
	"github.com/user/kb/internal/adapters/file"
	"github.com/user/kb/internal/chunker"
	"github.com/user/kb/internal/embedder"
	"github.com/user/kb/internal/ingest"
	"github.com/user/kb/internal/provider"
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
	flagFileForce bool
	flagConfForce bool
	flagAllForce  bool
	flagSpace     string
	flagPageID    string
)

func init() {
	ingestFileCmd.Flags().BoolVar(&flagRecursive, "recursive", false, "walk subdirectories")
	ingestFileCmd.Flags().StringVar(&flagExt, "ext", "md,txt,pdf", "comma-separated file extensions")
	ingestFileCmd.Flags().BoolVar(&flagFileForce, "force", false, "force full re-index")
	ingestConfluenceCmd.Flags().StringVar(&flagSpace, "space", "", "Confluence space key (required)")
	ingestConfluenceCmd.Flags().StringVar(&flagPageID, "page", "", "scope to a single page ID")
	ingestConfluenceCmd.Flags().BoolVar(&flagConfForce, "force", false, "force full re-index")
	ingestConfluenceCmd.MarkFlagRequired("space")
	ingestCmd.Flags().BoolVar(&flagAllForce, "force", false, "force full re-index for all sources")
	ingestCmd.AddCommand(ingestFileCmd, ingestConfluenceCmd)
}

func newIngester(cfg *config.Config) (*ingest.Ingester, store.Store, error) {
	st, err := store.NewSQLite(cfg.DB.Path)
	if err != nil {
		return nil, nil, fmt.Errorf("open store: %w", err)
	}
	emb, err := embedder.New(cfg.Embedder, cfg.Providers)
	if err != nil {
		return nil, nil, fmt.Errorf("create embedder: %w", err)
	}
	c := chunker.New(cfg.Chunker.ChunkSize, cfg.Chunker.ChunkOverlap)
	return ingest.New(st, c, emb), st, nil
}

func buildFileOptions(cfg *config.Config) (file.Options, error) {
	if !cfg.Vision.Enabled {
		return file.Options{}, nil
	}
	prov, err := provider.New(cfg.Vision.Provider, cfg.Providers)
	if err != nil {
		return file.Options{}, fmt.Errorf("vision provider: %w", err)
	}
	return file.Options{
		Vision: &file.VisionOptions{
			Config: cfg.Vision,
			Client: prov.Client(),
		},
	}, nil
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
		if err := runSource(cmd, ing, src, flagAllForce); err != nil {
			fmt.Fprintf(os.Stderr, "error ingesting %s: %v\n", src.Type, err)
		}
	}
	return nil
}

func runSource(cmd *cobra.Command, ing *ingest.Ingester, src config.SourceConfig, force bool) error {
	ctx := cmd.Context()
	switch src.Type {
	case "file":
		exts := []string{"md", "txt", "pdf"} // default
		if len(src.Extensions) > 0 {
			if len(src.Extensions) == 1 {
				exts = strings.Split(src.Extensions[0], ",")
			} else {
				exts = src.Extensions
			}
		}
		opts, err := buildFileOptions(cfg)
		if err != nil {
			return err
		}
		s := file.New(src.Path, src.Recursive, exts, opts)
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
	absPath, err := filepath.Abs(args[0])
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}
	exts := strings.Split(flagExt, ",")

	// Register/update in config — always store absolute path
	registerSource(config.SourceConfig{
		Type: "file", Path: absPath, Recursive: flagRecursive, Extensions: exts,
	})

	ing, st, err := newIngester(cfg)
	if err != nil {
		return err
	}
	defer st.Close()

	ctx := cmd.Context()
	opts, err := buildFileOptions(cfg)
	if err != nil {
		return err
	}
	src := file.New(absPath, flagRecursive, exts, opts)
	stats, err := ing.Run(ctx, src, "file", flagFileForce)
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

	ctx := cmd.Context()
	src := confluence.New(cfg.Confluence, flagSpace, flagPageID)
	stats, err := ing.Run(ctx, src, "confluence", flagConfForce)
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
			if err := config.Save(cfg); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not save config: %v\n", err)
			}
			return
		}
	}
	cfg.Sources = append(cfg.Sources, src)
	if err := config.Save(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not save config: %v\n", err)
	}
}
