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
	Short: "Start MCP server (stdio) — logs written to ~/.kb/kb.log",
	RunE:  runServe,
}

func runServe(cmd *cobra.Command, args []string) error {
	cleanup, err := initLogger(true)
	if err != nil {
		return err
	}
	defer cleanup()

	st, err := store.NewSQLite(cfg.DB.Path)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer st.Close()

	emb, err := embedder.New(cfg.Embedder, cfg.Providers)
	if err != nil {
		return fmt.Errorf("create embedder: %w", err)
	}

	srv := mcpserver.New(st, emb)
	return srv.Run(cmd.Context())
}
