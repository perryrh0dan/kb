package cmd

import (
	"fmt"
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
	flagLimit    int
	flagMinScore float64
	flagSource   string
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

	emb, err := embedder.New(cfg.Embedder, cfg.Providers)
	if err != nil {
		return fmt.Errorf("create embedder: %w", err)
	}

	ctx := cmd.Context()
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
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}
