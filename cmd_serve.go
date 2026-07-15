package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start MCP server (stdio)",
	RunE:  runServe,
}

func runServe(cmd *cobra.Command, args []string) error {
	fmt.Println("MCP server: implemented in Task 10")
	return nil
}
