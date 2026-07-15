package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/user/kb/config"
)

var rootCmd = &cobra.Command{
	Use:   "kb",
	Short: "kb — private knowledge base CLI and MCP server",
}

var cfg *config.Config

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.AddCommand(ingestCmd)
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(configCmd)
}

func initConfig() {
	var err error
	cfg, err = config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		os.Exit(1)
	}
}
