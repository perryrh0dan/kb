package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/user/kb/config"
)

func TestLoadDefaults(t *testing.T) {
	// No config file, no env vars — should return defaults
	t.Setenv("KB_OPENAI_API_KEY", "")
	t.Setenv("KB_DB_PATH", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Embedder.Provider != "openai" {
		t.Errorf("default provider = %q, want %q", cfg.Embedder.Provider, "openai")
	}
	if cfg.Embedder.Model != "text-embedding-3-large" {
		t.Errorf("default model = %q, want %q", cfg.Embedder.Model, "text-embedding-3-large")
	}
	if cfg.Chunker.ChunkSize != 512 {
		t.Errorf("default chunk_size = %d, want 512", cfg.Chunker.ChunkSize)
	}
	if cfg.Chunker.ChunkOverlap != 50 {
		t.Errorf("default chunk_overlap = %d, want 50", cfg.Chunker.ChunkOverlap)
	}
}

func TestEnvVarOverride(t *testing.T) {
	t.Setenv("KB_OPENAI_API_KEY", "sk-test-key")
	t.Setenv("KB_DB_PATH", "/tmp/test.db")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.OpenAI.APIKey != "sk-test-key" {
		t.Errorf("api_key = %q, want %q", cfg.OpenAI.APIKey, "sk-test-key")
	}
	if cfg.DB.Path != "/tmp/test.db" {
		t.Errorf("db.path = %q, want %q", cfg.DB.Path, "/tmp/test.db")
	}
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	content := `
openai:
  api_key: "sk-from-file"
chunker:
  chunk_size: 256
  chunk_overlap: 25
`
	if err := os.WriteFile(cfgFile, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.LoadFrom(cfgFile)
	if err != nil {
		t.Fatalf("LoadFrom() error: %v", err)
	}
	if cfg.OpenAI.APIKey != "sk-from-file" {
		t.Errorf("api_key = %q, want %q", cfg.OpenAI.APIKey, "sk-from-file")
	}
	if cfg.Chunker.ChunkSize != 256 {
		t.Errorf("chunk_size = %d, want 256", cfg.Chunker.ChunkSize)
	}
}
