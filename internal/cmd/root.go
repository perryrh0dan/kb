package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/user/kb/config"
)

var rootCmd = &cobra.Command{
	Use:   "kb",
	Short: "kb — private knowledge base CLI and MCP server",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		_, err := initLogger(false)
		return err
	},
}

var cfg *config.Config
var verbose bool
var logLevel string

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable debug logging (shorthand for --log-level debug)")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "warn", "log level: debug, info, warn, error")
	rootCmd.AddCommand(ingestCmd)
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(configCmd)
}

// Execute is the entry point called from main.
func Execute() error {
	return rootCmd.Execute()
}

func initConfig() {
	var err error
	cfg, err = config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		os.Exit(1)
	}
}

// initLogger configures slog.Default(). When toFile is true, logs go to
// ~/.kb/kb.log instead of stderr (used by MCP server to keep stdio clean).
// Returns a cleanup func that must be called when the process exits.
func initLogger(toFile bool) (func(), error) {
	level := slog.LevelWarn
	if verbose {
		level = slog.LevelDebug
	} else {
		switch logLevel {
		case "debug":
			level = slog.LevelDebug
		case "info":
			level = slog.LevelInfo
		case "warn":
			level = slog.LevelWarn
		case "error":
			level = slog.LevelError
		default:
			return func() {}, fmt.Errorf("invalid log level %q: must be debug, info, warn, or error", logLevel)
		}
	}

	opts := &slog.HandlerOptions{Level: level}

	if toFile {
		home, err := os.UserHomeDir()
		if err != nil {
			return func() {}, fmt.Errorf("resolve home dir: %w", err)
		}
		logPath := filepath.Join(home, ".kb", "kb.log")
		if err := os.MkdirAll(filepath.Dir(logPath), 0700); err != nil {
			return func() {}, fmt.Errorf("create log dir: %w", err)
		}
		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return func() {}, fmt.Errorf("open log file: %w", err)
		}
		slog.SetDefault(slog.New(slog.NewTextHandler(f, opts)))
		return func() { f.Close() }, nil
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, opts)))
	return func() {}, nil
}
