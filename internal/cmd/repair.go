package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/user/kb/config"
	"github.com/user/kb/internal/adapters"
	"github.com/user/kb/internal/adapters/confluence"
	"github.com/user/kb/internal/adapters/file"
	"github.com/user/kb/internal/store"
)

var (
	flagRepairDryRun      bool
	flagRepairRemoveStale bool
)

var repairCmd = &cobra.Command{
	Use:   "repair",
	Short: "Find and re-ingest documents that are missing chunks",
	Long: `Repair scans the store for documents that have no chunks (e.g. due to a
previous embedding failure) and re-ingests them.

Documents whose source is no longer configured are reported as stale.
Use --remove-stale to delete them from the store.`,
	RunE: runRepair,
}

func init() {
	repairCmd.Flags().BoolVar(&flagRepairDryRun, "dry-run", false, "list orphaned documents without making any changes")
	repairCmd.Flags().BoolVar(&flagRepairRemoveStale, "remove-stale", false, "delete orphaned documents with no matching source config")
}

func runRepair(cmd *cobra.Command, args []string) error {
	st, err := store.NewSQLite(cfg.DB.Path)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer st.Close()

	ctx := cmd.Context()

	orphans, err := st.GetOrphanedDocuments(ctx)
	if err != nil {
		return fmt.Errorf("query orphaned documents: %w", err)
	}

	if len(orphans) == 0 {
		fmt.Println("No orphaned documents found.")
		return nil
	}

	fmt.Printf("Found %d document(s) with missing chunks.\n", len(orphans))

	// Partition orphans into resolvable (source in config) and stale (no source).
	type repairGroup struct {
		src  config.SourceConfig
		docs []adapters.DocumentMeta
	}
	groupMap := map[string]*repairGroup{} // keyed by source identity string
	var stale []adapters.DocumentMeta

	for _, doc := range orphans {
		src, ok := findRepairSource(cfg.Sources, doc.ID)
		if !ok {
			stale = append(stale, doc)
			continue
		}
		key := repairSourceKey(src)
		if _, exists := groupMap[key]; !exists {
			groupMap[key] = &repairGroup{src: src}
		}
		groupMap[key].docs = append(groupMap[key].docs, doc)
	}

	if flagRepairDryRun {
		if len(groupMap) > 0 {
			fmt.Println("\nResolvable (would be re-ingested):")
			for key, group := range groupMap {
				fmt.Printf("  [%s — %d document(s)]\n", key, len(group.docs))
				for _, doc := range group.docs {
					fmt.Printf("    %s  %s\n", doc.ID, doc.Title)
				}
			}
		}
		if len(stale) > 0 {
			fmt.Printf("\nStale — no matching source config (%d document(s)):\n", len(stale))
			for _, doc := range stale {
				fmt.Printf("  %s  %s\n", doc.ID, doc.Title)
			}
			fmt.Println("\n  Use --remove-stale to delete stale documents from the store.")
		}
		return nil
	}

	// Re-ingest resolvable orphans.
	ing, _, err := newIngester(cfg)
	if err != nil {
		return err
	}

	var totalRepaired, totalErrors int
	for _, group := range groupMap {
		adapter, err := buildRepairAdapter(group.src)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not build adapter for %s: %v\n", repairSourceKey(group.src), err)
			totalErrors += len(group.docs)
			continue
		}
		fmt.Printf("\nRepairing %d document(s) from %s...\n", len(group.docs), repairSourceKey(group.src))
		stats, err := ing.RepairDocuments(ctx, group.docs, adapter, progressPrinter(repairSourceKey(group.src)))
		if err != nil {
			fmt.Fprintf(os.Stderr, "error during repair of %s: %v\n", repairSourceKey(group.src), err)
		}
		totalRepaired += stats.Ingested
		totalErrors += stats.Errors
	}

	// Handle stale orphans.
	if len(stale) > 0 {
		if flagRepairRemoveStale {
			fmt.Printf("\nRemoving %d stale document(s) with no matching source...\n", len(stale))
			removed := 0
			for _, doc := range stale {
				if err := st.DeleteDocument(ctx, doc.ID); err != nil {
					fmt.Fprintf(os.Stderr, "  ✗ failed to delete %s: %v\n", doc.ID, err)
					totalErrors++
				} else {
					fmt.Printf("  - deleted  %s\n", doc.Title)
					removed++
				}
			}
			fmt.Printf("removed=%d\n", removed)
		} else {
			fmt.Printf("\n%d stale document(s) have no matching source in config (use --remove-stale to delete):\n", len(stale))
			for _, doc := range stale {
				fmt.Printf("  %s  %s\n", doc.ID, doc.Title)
			}
		}
	}

	fmt.Printf("\nrepaired=%d errors=%d\n", totalRepaired, totalErrors)
	return nil
}

// findRepairSource returns the config.SourceConfig whose scope covers docID.
func findRepairSource(sources []config.SourceConfig, docID string) (config.SourceConfig, bool) {
	for _, src := range sources {
		switch src.Type {
		case "confluence":
			if strings.HasPrefix(docID, "confluence://"+src.Space+"/") {
				return src, true
			}
		case "file":
			abs := src.Path
			if len(abs) > 0 && abs[len(abs)-1] != '/' {
				abs += "/"
			}
			if strings.HasPrefix(docID, "file://"+abs) {
				return src, true
			}
		}
	}
	return config.SourceConfig{}, false
}

// repairSourceKey returns a human-readable identifier for a source config.
func repairSourceKey(src config.SourceConfig) string {
	switch src.Type {
	case "confluence":
		return "confluence:" + src.Space
	case "file":
		return "file:" + filepath.Base(src.Path)
	}
	return src.Type
}

// buildRepairAdapter creates an adapters.Source for the given source config.
func buildRepairAdapter(src config.SourceConfig) (adapters.Source, error) {
	switch src.Type {
	case "confluence":
		return confluence.New(cfg.Confluence, src.Space, src.PageID), nil
	case "file":
		opts, err := buildFileOptions(cfg)
		if err != nil {
			return nil, fmt.Errorf("vision options: %w", err)
		}
		return file.New(src.Path, src.Recursive, src.Extensions, opts), nil
	}
	return nil, fmt.Errorf("unknown source type %q", src.Type)
}
