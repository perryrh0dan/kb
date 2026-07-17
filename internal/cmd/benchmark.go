package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/user/kb/internal/embedder"
	"github.com/user/kb/internal/store"
)

var (
	flagBenchmarkIterations int
	flagBenchmarkLimit      int
	flagBenchmarkMinScore   float64
	flagBenchmarkSource     string
)

var benchmarkCmd = &cobra.Command{
	Use:   "benchmark <query>",
	Short: "Measure local vector-search performance",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runBenchmark,
}

func init() {
	benchmarkCmd.Flags().IntVar(&flagBenchmarkIterations, "iterations", 20, "number of local search iterations")
	benchmarkCmd.Flags().IntVar(&flagBenchmarkLimit, "limit", 10, "number of results per search")
	benchmarkCmd.Flags().Float64Var(&flagBenchmarkMinScore, "min-score", 0, "minimum similarity score (0-1)")
	benchmarkCmd.Flags().StringVar(&flagBenchmarkSource, "source", "", "filter by source type: file|confluence")
}

func runBenchmark(cmd *cobra.Command, args []string) error {
	if flagBenchmarkIterations < 1 || flagBenchmarkLimit < 1 {
		return fmt.Errorf("iterations and limit must be positive")
	}

	st, err := store.NewSQLite(cfg.DB.Path)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer st.Close()

	emb, err := embedder.New(cfg.Embedder, cfg.Providers)
	if err != nil {
		return fmt.Errorf("create embedder: %w", err)
	}

	query := strings.Join(args, " ")
	embedStart := time.Now()
	vecs, err := emb.Embed(cmd.Context(), []string{query})
	if err != nil {
		return fmt.Errorf("embed query: %w", err)
	}
	embedDuration := time.Since(embedStart)

	var total time.Duration
	resultCount := 0
	for i := 0; i < flagBenchmarkIterations; i++ {
		start := time.Now()
		results, err := st.Search(cmd.Context(), vecs[0], flagBenchmarkLimit, flagBenchmarkMinScore, flagBenchmarkSource)
		total += time.Since(start)
		if err != nil {
			return fmt.Errorf("search iteration %d: %w", i+1, err)
		}
		resultCount = len(results)
	}

	fmt.Printf("query_embedding=%s (excluded from search timing)\n", embedDuration.Round(time.Millisecond))
	fmt.Printf("iterations=%d limit=%d source=%q results=%d\n", flagBenchmarkIterations, flagBenchmarkLimit, flagBenchmarkSource, resultCount)
	fmt.Printf("search_total=%s average=%s\n", total.Round(time.Millisecond), (total / time.Duration(flagBenchmarkIterations)).Round(time.Microsecond))
	return nil
}
