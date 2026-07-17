package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/user/kb/config"
)

// TestLoadDefaults — check vision.provider default
func TestLoadDefaults(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("KB_OPENAI_API_KEY", "")
	t.Setenv("KB_DB_PATH", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Embedder.Provider != "openai" {
		t.Errorf("default embedder provider = %q, want %q", cfg.Embedder.Provider, "openai")
	}
	if cfg.Vision.Provider != "openai" {
		t.Errorf("default vision provider = %q, want %q", cfg.Vision.Provider, "openai")
	}
	if cfg.Chunker.ChunkSize != 512 {
		t.Errorf("default chunk_size = %d, want 512", cfg.Chunker.ChunkSize)
	}
	if cfg.Chunker.ChunkOverlap != 50 {
		t.Errorf("default chunk_overlap = %d, want 50", cfg.Chunker.ChunkOverlap)
	}
}

// TestEnvVarOverride — check new env var names
func TestEnvVarOverride(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("KB_OPENAI_API_KEY", "sk-test-key")
	t.Setenv("KB_DB_PATH", "/tmp/test.db")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Providers.OpenAI.APIKey != "sk-test-key" {
		t.Errorf("providers.openai.api_key = %q, want %q", cfg.Providers.OpenAI.APIKey, "sk-test-key")
	}
	if cfg.DB.Path != "/tmp/test.db" {
		t.Errorf("db.path = %q, want %q", cfg.DB.Path, "/tmp/test.db")
	}
}

// TestLoadFromFile — test providers section in YAML
func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	content := `
providers:
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
	if cfg.Providers.OpenAI.APIKey != "sk-from-file" {
		t.Errorf("providers.openai.api_key = %q, want %q", cfg.Providers.OpenAI.APIKey, "sk-from-file")
	}
	if cfg.Chunker.ChunkSize != 256 {
		t.Errorf("chunk_size = %d, want 256", cfg.Chunker.ChunkSize)
	}
}

func TestOAuthOpenAIEnvVars(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("KB_OAUTH_OPENAI_ENDPOINT", "https://api.example.com")
	t.Setenv("KB_OAUTH_OPENAI_TOKEN_URL", "https://idp.example.com/token")
	t.Setenv("KB_OAUTH_OPENAI_CLIENT_ID", "my-client")
	t.Setenv("KB_OAUTH_OPENAI_ROUTING", "openai")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Providers.OAuthOpenAI.Endpoint != "https://api.example.com" {
		t.Errorf("OAuthOpenAI.Endpoint = %q, want %q", cfg.Providers.OAuthOpenAI.Endpoint, "https://api.example.com")
	}
	if cfg.Providers.OAuthOpenAI.TokenURL != "https://idp.example.com/token" {
		t.Errorf("OAuthOpenAI.TokenURL = %q, want %q", cfg.Providers.OAuthOpenAI.TokenURL, "https://idp.example.com/token")
	}
	if cfg.Providers.OAuthOpenAI.ClientID != "my-client" {
		t.Errorf("OAuthOpenAI.ClientID = %q, want %q", cfg.Providers.OAuthOpenAI.ClientID, "my-client")
	}
	if cfg.Providers.OAuthOpenAI.Routing != "openai" {
		t.Errorf("OAuthOpenAI.Routing = %q, want %q", cfg.Providers.OAuthOpenAI.Routing, "openai")
	}
}

func TestConfigPathEnvOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace", "config.yaml")
	t.Setenv("KB_CONFIG_PATH", path)

	if got := config.Path(); got != path {
		t.Fatalf("Path() = %q, want %q", got, path)
	}
}

func TestLoadAndSaveUseConfigPathEnvOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace", "config.yaml")
	t.Setenv("KB_CONFIG_PATH", path)

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("embedder:\n  model: custom-model\n"), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Embedder.Model != "custom-model" {
		t.Fatalf("Embedder.Model = %q, want custom-model", cfg.Embedder.Model)
	}

	cfg.Embedder.Model = "saved-model"
	if err := config.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "model: saved-model") {
		t.Fatalf("saved config did not use KB_CONFIG_PATH: %s", content)
	}
}

// TestSaveRoundTrip — update to use Providers
func TestSaveRoundTrip(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	cfg := &config.Config{}
	cfg.Providers.OpenAI.APIKey = "sk-roundtrip"
	cfg.Chunker.ChunkSize = 256
	cfg.Chunker.ChunkOverlap = 25
	cfg.DB.Path = "/tmp/test.db"
	if err := config.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Providers.OpenAI.APIKey != "sk-roundtrip" {
		t.Errorf("providers.openai.api_key = %q, want %q", loaded.Providers.OpenAI.APIKey, "sk-roundtrip")
	}
	if loaded.Chunker.ChunkSize != 256 {
		t.Errorf("chunk_size = %d, want 256", loaded.Chunker.ChunkSize)
	}
}
