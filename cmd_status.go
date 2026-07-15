package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/user/kb/config"
	"github.com/user/kb/internal/store"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show knowledge base statistics",
	RunE:  runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
	st, err := store.NewSQLite(cfg.DB.Path)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer st.Close()

	stats, err := st.Stats(context.Background())
	if err != nil {
		return err
	}
	b, _ := json.MarshalIndent(stats, "", "  ")
	fmt.Println(string(b))
	return nil
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create default config file at ~/.kb/config.yaml",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := config.InitDefault(); err != nil {
			return err
		}
		fmt.Println("Config created at ~/.kb/config.yaml")
		return nil
	},
}

func init() {
	configCmd.AddCommand(configInitCmd)
}
