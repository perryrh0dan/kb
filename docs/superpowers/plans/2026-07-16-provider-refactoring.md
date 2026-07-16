# Provider Refactoring Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Introduce a unified `internal/provider` package with `openai/` and `azure/` sub-packages, replacing the scattered `OpenAIConfig` usage, so both embedder and vision can be routed through either OpenAI or Azure OpenAI with a single config change.

**Architecture:** Three tasks. Task 1 restructures config: `OpenAIConfig` → `ProvidersConfig` with `OpenAI ProviderConfig` and `Azure AzureProviderConfig`. Task 2 creates `internal/provider` package with `Provider` interface, `openai.New()`, and `azure.New()` — both exposing a `*oai.Client`. Task 3 wires everything: embedder factory, vision client builder, and all CLI commands updated to use `cfg.Providers` instead of `cfg.OpenAI`.

**Tech Stack:** Go stdlib, `sashabaranov/go-openai` (already present — `DefaultConfig`, `DefaultAzureConfig`), `spf13/viper` (config).

## Global Constraints

- Go module path: `github.com/user/kb`
- Build: `CGO_ENABLED=1 go build -o kb .`
- All tests must pass: `CGO_ENABLED=1 go test ./...`
- No new external Go dependencies
- No backward compatibility with old config keys (`openai.api_key` → `providers.openai.api_key`)
- Provider names: `"openai"` (default) and `"azure"` (exact strings)
- Env vars: `KB_OPENAI_API_KEY` → `providers.openai.api_key`; `KB_AZURE_API_KEY`, `KB_AZURE_BASE_URL`, `KB_AZURE_API_VERSION` — new
- Default embedder provider: `"openai"`, default vision provider: `"openai"`
- `VisionConfig` gets a `Provider string` field (default `"openai"`)
- Azure API version default: `"2024-02-15-preview"`

---

### Task 1: Config restructuring — ProvidersConfig replaces OpenAIConfig

**Files:**
- Modify: `config/config.go`
- Modify: `config/config_test.go`

**Interfaces:**
- Produces:
  - `config.ProviderConfig` struct: `APIKey string`
  - `config.AzureProviderConfig` struct: `APIKey string`, `BaseURL string`, `APIVersion string`
  - `config.ProvidersConfig` struct: `OpenAI ProviderConfig`, `Azure AzureProviderConfig`
  - `config.VisionConfig.Provider string` (new field, default `"openai"`)
  - `config.Config.Providers ProvidersConfig` (replaces `config.Config.OpenAI OpenAIConfig`)
  - `config.OpenAIConfig` type removed
  - Viper bindings: `KB_OPENAI_API_KEY→providers.openai.api_key`, `KB_AZURE_API_KEY→providers.azure.api_key`, `KB_AZURE_BASE_URL→providers.azure.base_url`, `KB_AZURE_API_VERSION→providers.azure.api_version`

- [ ] **Step 1: Replace config structs in `config/config.go`**

Replace the `OpenAIConfig` type and update `Config` struct. Full new version of the relevant section:

```go
// ProviderConfig holds credentials for a standard OpenAI-compatible endpoint.
type ProviderConfig struct {
	APIKey string `mapstructure:"api_key" yaml:"api_key"`
}

// AzureProviderConfig holds credentials and endpoint info for Azure OpenAI.
type AzureProviderConfig struct {
	APIKey     string `mapstructure:"api_key"     yaml:"api_key"`
	BaseURL    string `mapstructure:"base_url"    yaml:"base_url"`
	APIVersion string `mapstructure:"api_version" yaml:"api_version"`
}

// ProvidersConfig holds configuration for all supported LLM/embedding providers.
type ProvidersConfig struct {
	OpenAI ProviderConfig      `mapstructure:"openai" yaml:"openai"`
	Azure  AzureProviderConfig `mapstructure:"azure"  yaml:"azure"`
}
```

Update `VisionConfig`:
```go
type VisionConfig struct {
	Enabled  bool    `mapstructure:"enabled"   yaml:"enabled"`
	Provider string  `mapstructure:"provider"  yaml:"provider"`
	Model    string  `mapstructure:"model"     yaml:"model"`
	DPI      float64 `mapstructure:"dpi"       yaml:"dpi"`
}
```

Update `Config`:
```go
type Config struct {
	Providers  ProvidersConfig  `mapstructure:"providers"   yaml:"providers"`
	Confluence ConfluenceConfig `mapstructure:"confluence"  yaml:"confluence"`
	DB         DBConfig         `mapstructure:"db"          yaml:"db"`
	Embedder   EmbedderConfig   `mapstructure:"embedder"    yaml:"embedder"`
	Chunker    ChunkerConfig    `mapstructure:"chunker"     yaml:"chunker"`
	Vision     VisionConfig     `mapstructure:"vision"      yaml:"vision"`
	Sources    []SourceConfig   `mapstructure:"sources"     yaml:"sources"`
}
```

- [ ] **Step 2: Update `newViper()` in `config/config.go`**

Replace the viper setup block:

```go
func newViper() *viper.Viper {
	v := viper.New()
	v.SetDefault("embedder.provider", "openai")
	v.SetDefault("embedder.model", "text-embedding-3-large")
	v.SetDefault("chunker.chunk_size", 512)
	v.SetDefault("chunker.chunk_overlap", 50)
	v.SetDefault("db.path", filepath.Join(mustHomeDir(), ".kb", "kb.db"))
	v.SetDefault("vision.enabled", false)
	v.SetDefault("vision.provider", "openai")
	v.SetDefault("vision.model", "gpt-4o")
	v.SetDefault("vision.dpi", 150.0)
	v.SetDefault("providers.azure.api_version", "2024-02-15-preview")

	v.SetEnvPrefix("KB")
	v.BindEnv("providers.openai.api_key",    "KB_OPENAI_API_KEY")        //nolint:errcheck
	v.BindEnv("providers.azure.api_key",     "KB_AZURE_API_KEY")         //nolint:errcheck
	v.BindEnv("providers.azure.base_url",    "KB_AZURE_BASE_URL")        //nolint:errcheck
	v.BindEnv("providers.azure.api_version", "KB_AZURE_API_VERSION")     //nolint:errcheck
	v.BindEnv("confluence.api_token",        "KB_CONFLUENCE_API_TOKEN")  //nolint:errcheck
	v.BindEnv("confluence.pat",              "KB_CONFLUENCE_PAT")        //nolint:errcheck
	v.BindEnv("db.path",                     "KB_DB_PATH")               //nolint:errcheck

	return v
}
```

- [ ] **Step 3: Update `InitDefault()` YAML template**

Replace the `openai:` section with `providers:`:

```go
content := `# kb configuration

providers:
  openai:
    api_key: ""  # or set KB_OPENAI_API_KEY env var

  azure:         # optional — only fill in if using Azure OpenAI
    api_key: ""  # or set KB_AZURE_API_KEY env var
    base_url: "" # e.g. https://my-resource.openai.azure.com/
    api_version: "2024-02-15-preview"  # or set KB_AZURE_API_VERSION

confluence:
  base_url: ""
  username: ""       # Cloud: Confluence username/email
  api_token: ""      # Cloud: API token (or KB_CONFLUENCE_API_TOKEN)
  pat: ""            # Data Center: Personal Access Token (or KB_CONFLUENCE_PAT)

db:
  path: ~/.kb/kb.db  # or set KB_DB_PATH env var

embedder:
  provider: openai   # "openai" | "azure"
  model: text-embedding-3-large

chunker:
  chunk_size: 512
  chunk_overlap: 50

vision:
  enabled: false     # true to describe PDF images via Vision model
  provider: openai   # "openai" | "azure"
  model: gpt-4o      # for Azure: use the deployment name
  dpi: 150

# sources are auto-registered when you run: kb ingest file <path> / kb ingest confluence --space <KEY>
sources: []
`
```

- [ ] **Step 4: Update config tests**

In `config/config_test.go`, update all tests to use the new key paths:

```go
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
```

- [ ] **Step 5: Run config tests**

```bash
go test ./config/... -v 2>&1 | grep -E "^(=== RUN|--- PASS|--- FAIL|ok|FAIL)"
```

Expected: all PASS (other packages will fail to build — that's expected until Task 3)

- [ ] **Step 6: Commit**

```bash
git add config/
git commit -m "refactor(config): ProvidersConfig replaces OpenAIConfig; VisionConfig gets Provider field"
```

---

### Task 2: internal/provider package with openai/ and azure/

**Files:**
- Create: `internal/provider/provider.go`
- Create: `internal/provider/openai/openai.go`
- Create: `internal/provider/openai/openai_test.go`
- Create: `internal/provider/azure/azure.go`
- Create: `internal/provider/azure/azure_test.go`

**Interfaces:**
- Consumes: `config.ProvidersConfig`, `config.ProviderConfig`, `config.AzureProviderConfig` (from Task 1)
- Produces:
  - `provider.Provider` interface: `Client() *oai.Client`, `Name() string`
  - `provider.New(name string, cfg config.ProvidersConfig) (provider.Provider, error)`
  - `openai.New(cfg config.ProviderConfig) (provider.Provider, error)`
  - `azure.New(cfg config.AzureProviderConfig) (provider.Provider, error)`

- [ ] **Step 1: Write failing tests**

`internal/provider/openai/openai_test.go`:
```go
package openai_test

import (
	"testing"

	"github.com/user/kb/config"
	oaiprovider "github.com/user/kb/internal/provider/openai"
)

func TestNewOpenAIProvider(t *testing.T) {
	cfg := config.ProviderConfig{APIKey: "sk-test"}
	p, err := oaiprovider.New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if p.Name() != "openai" {
		t.Errorf("Name() = %q, want %q", p.Name(), "openai")
	}
	if p.Client() == nil {
		t.Error("Client() should not be nil")
	}
}
```

`internal/provider/azure/azure_test.go`:
```go
package azure_test

import (
	"testing"

	"github.com/user/kb/config"
	azprovider "github.com/user/kb/internal/provider/azure"
)

func TestNewAzureProvider(t *testing.T) {
	cfg := config.AzureProviderConfig{
		APIKey:     "azure-test-key",
		BaseURL:    "https://my-resource.openai.azure.com/",
		APIVersion: "2024-02-15-preview",
	}
	p, err := azprovider.New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if p.Name() != "azure" {
		t.Errorf("Name() = %q, want %q", p.Name(), "azure")
	}
	if p.Client() == nil {
		t.Error("Client() should not be nil")
	}
}

func TestNewAzureProvider_MissingBaseURL(t *testing.T) {
	cfg := config.AzureProviderConfig{APIKey: "key"}
	_, err := azprovider.New(cfg)
	if err == nil {
		t.Error("expected error for missing base_url, got nil")
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
go test ./internal/provider/... -v 2>&1 | grep -E "FAIL|no Go files"
```

Expected: build error — packages don't exist yet

- [ ] **Step 3: Implement `internal/provider/provider.go`**

```go
package provider

import (
	"fmt"

	oai "github.com/sashabaranov/go-openai"
	"github.com/user/kb/config"
	azprovider "github.com/user/kb/internal/provider/azure"
	oaiprovider "github.com/user/kb/internal/provider/openai"
)

// Provider wraps a configured API client for use by embedder and vision.
type Provider interface {
	// Client returns the underlying go-openai client.
	// Both OpenAI and Azure use the same client type; the difference is
	// in how the client was configured (DefaultConfig vs DefaultAzureConfig).
	Client() *oai.Client

	// Name returns the provider identifier: "openai" or "azure".
	Name() string
}

// New creates a Provider for the given name using the credentials in cfg.
// name must be "openai" or "azure". An empty or unrecognised name defaults to "openai".
func New(name string, cfg config.ProvidersConfig) (Provider, error) {
	switch name {
	case "azure":
		return azprovider.New(cfg.Azure)
	case "openai", "":
		return oaiprovider.New(cfg.OpenAI)
	default:
		return nil, fmt.Errorf("unknown provider %q: must be \"openai\" or \"azure\"", name)
	}
}
```

- [ ] **Step 4: Implement `internal/provider/openai/openai.go`**

```go
package openai

import (
	oai "github.com/sashabaranov/go-openai"
	"github.com/user/kb/config"
)

type openAIProvider struct {
	client *oai.Client
}

// New creates an OpenAI provider using the standard OpenAI API endpoint.
func New(cfg config.ProviderConfig) (*openAIProvider, error) {
	oaiCfg := oai.DefaultConfig(cfg.APIKey)
	return &openAIProvider{client: oai.NewClientWithConfig(oaiCfg)}, nil
}

func (p *openAIProvider) Client() *oai.Client { return p.client }
func (p *openAIProvider) Name() string        { return "openai" }
```

- [ ] **Step 5: Implement `internal/provider/azure/azure.go`**

```go
package azure

import (
	"fmt"

	oai "github.com/sashabaranov/go-openai"
	"github.com/user/kb/config"
)

type azureProvider struct {
	client *oai.Client
}

// New creates an Azure OpenAI provider.
// cfg.BaseURL is required — it is the Azure OpenAI resource endpoint,
// e.g. "https://my-resource.openai.azure.com/".
// cfg.APIVersion defaults to "2024-02-15-preview" if empty.
func New(cfg config.AzureProviderConfig) (*azureProvider, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("azure provider requires base_url (e.g. https://my-resource.openai.azure.com/)")
	}
	azCfg := oai.DefaultAzureConfig(cfg.APIKey, cfg.BaseURL)
	if cfg.APIVersion != "" {
		azCfg.APIVersion = cfg.APIVersion
	}
	return &azureProvider{client: oai.NewClientWithConfig(azCfg)}, nil
}

func (p *azureProvider) Client() *oai.Client { return p.client }
func (p *azureProvider) Name() string        { return "azure" }
```

- [ ] **Step 6: Run provider tests**

```bash
go test ./internal/provider/... -v 2>&1 | grep -E "^(=== RUN|--- PASS|--- FAIL|ok|FAIL)"
```

Expected: all PASS (3 tests)

- [ ] **Step 7: Commit**

```bash
git add internal/provider/
git commit -m "feat(provider): add internal/provider package with openai/ and azure/ implementations"
```

---

### Task 3: Wire provider into embedder, vision, and CLI commands

**Files:**
- Modify: `internal/embedder/embedder.go`
- Modify: `internal/embedder/openai/openai.go`
- Modify: `internal/embedder/openai/openai_test.go`
- Modify: `internal/adapters/file/file.go`
- Modify: `cmd_ingest.go`
- Modify: `cmd_search.go`
- Modify: `cmd_serve.go`

**Interfaces:**
- Consumes:
  - `provider.New(name string, cfg config.ProvidersConfig) (provider.Provider, error)` (Task 2)
  - `provider.Provider.Client() *oai.Client` (Task 2)
  - `config.ProvidersConfig` (Task 1)
  - `config.VisionConfig.Provider` (Task 1)
- Produces: working `kb` binary where all OpenAI/Azure calls go through the provider system

- [ ] **Step 1: Update `internal/embedder/openai/openai.go`**

Add `NewWithClient` constructor, keep `New` and `NewWithBaseURL` for backward compat in tests:

```go
package openai

import (
	"context"
	"fmt"
	"log/slog"

	oai "github.com/sashabaranov/go-openai"
	"github.com/user/kb/config"
)

const batchSize = 100

type openAIEmbedder struct {
	client *oai.Client
	model  oai.EmbeddingModel
	dims   int
}

// NewWithClient creates an embedder using an already-configured oai.Client.
// This is the primary constructor used by the provider system.
func NewWithClient(client *oai.Client, embedCfg config.EmbedderConfig) (*openAIEmbedder, error) {
	return &openAIEmbedder{
		client: client,
		model:  oai.EmbeddingModel(embedCfg.Model),
		dims:   3072,
	}, nil
}

// NewWithBaseURL creates an embedder with a custom base URL. Used in tests only.
func NewWithBaseURL(embedCfg config.EmbedderConfig, apiKey string, baseURL string) (*openAIEmbedder, error) {
	cfg := oai.DefaultConfig(apiKey)
	if baseURL != "" {
		cfg.BaseURL = baseURL
	}
	return &openAIEmbedder{
		client: oai.NewClientWithConfig(cfg),
		model:  oai.EmbeddingModel(embedCfg.Model),
		dims:   3072,
	}, nil
}

func (e *openAIEmbedder) Dimensions() int   { return e.dims }
func (e *openAIEmbedder) ModelName() string { return string(e.model) }

func (e *openAIEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	log := slog.Default()
	var results [][]float32
	for i := 0; i < len(texts); i += batchSize {
		end := i + batchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch := texts[i:end]
		log.Debug("embedding batch", "batch_start", i, "batch_end", end, "model", e.model)
		resp, err := e.client.CreateEmbeddings(ctx, oai.EmbeddingRequest{
			Input: batch,
			Model: e.model,
		})
		if err != nil {
			log.Warn("embedding batch failed", "batch_start", i, "batch_end", end, "error", err)
			return nil, fmt.Errorf("embed batch [%d:%d]: %w", i, end, err)
		}
		for _, d := range resp.Data {
			results = append(results, d.Embedding)
		}
	}
	return results, nil
}
```

- [ ] **Step 2: Update `internal/embedder/openai/openai_test.go`**

The test uses `NewWithBaseURL` — update to pass `apiKey` instead of `config.OpenAIConfig`:

```go
package openai_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/user/kb/config"
	oaiemb "github.com/user/kb/internal/embedder/openai"
)

func TestEmbed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"object": "list",
			"data": []map[string]interface{}{
				{"object": "embedding", "index": 0, "embedding": make([]float64, 3072)},
				{"object": "embedding", "index": 1, "embedding": make([]float64, 3072)},
			},
			"model": "text-embedding-3-large",
			"usage": map[string]int{"prompt_tokens": 10, "total_tokens": 10},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	emb, err := oaiemb.NewWithBaseURL(
		config.EmbedderConfig{Provider: "openai", Model: "text-embedding-3-large"},
		"sk-test",
		srv.URL+"/v1",
	)
	if err != nil {
		t.Fatalf("NewWithBaseURL: %v", err)
	}

	texts := []string{"hello world", "foo bar"}
	vecs, err := emb.Embed(context.Background(), texts)
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(vecs) != 2 {
		t.Errorf("got %d vectors, want 2", len(vecs))
	}
	if len(vecs[0]) != 3072 {
		t.Errorf("got %d dims, want 3072", len(vecs[0]))
	}
}

func TestDimensions(t *testing.T) {
	emb, err := oaiemb.NewWithBaseURL(
		config.EmbedderConfig{Provider: "openai", Model: "text-embedding-3-large"},
		"sk-test",
		"http://localhost",
	)
	if err != nil {
		t.Fatalf("NewWithBaseURL: %v", err)
	}
	if emb.Dimensions() != 3072 {
		t.Errorf("Dimensions() = %d, want 3072", emb.Dimensions())
	}
}
```

- [ ] **Step 3: Update `internal/embedder/embedder.go`**

```go
package embedder

import (
	"context"
	"fmt"

	"github.com/user/kb/config"
	"github.com/user/kb/internal/provider"
	oaiemb "github.com/user/kb/internal/embedder/openai"
)

// Embedder converts text slices into float32 vectors.
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	Dimensions() int
	ModelName() string
}

// New creates an Embedder. The provider is determined by embedCfg.Provider
// ("openai" or "azure"). Credentials come from providersCfg.
func New(embedCfg config.EmbedderConfig, providersCfg config.ProvidersConfig) (Embedder, error) {
	prov, err := provider.New(embedCfg.Provider, providersCfg)
	if err != nil {
		return nil, fmt.Errorf("embedder provider: %w", err)
	}
	return oaiemb.NewWithClient(prov.Client(), embedCfg)
}
```

- [ ] **Step 4: Update `internal/adapters/file/file.go`**

Remove the direct `oai` import — `VisionOptions.Client` stays `*oai.Client` (the provider returns one), but we no longer need to import `go-openai` directly in `file.go`. Update the import block:

```go
import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	oai "github.com/sashabaranov/go-openai"
	"github.com/user/kb/config"
	"github.com/user/kb/internal/adapters"
	"github.com/user/kb/internal/store"
)
```

The `VisionOptions` struct stays identical — `Client *oai.Client` — no change needed there. The import of `oai` stays because `VisionOptions` references `*oai.Client`.

- [ ] **Step 5: Update `cmd_ingest.go`**

Replace `buildFileOptions` and update `newIngester`:

```go
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/user/kb/config"
	"github.com/user/kb/internal/adapters/confluence"
	"github.com/user/kb/internal/adapters/file"
	"github.com/user/kb/internal/chunker"
	"github.com/user/kb/internal/embedder"
	"github.com/user/kb/internal/ingest"
	"github.com/user/kb/internal/provider"
	"github.com/user/kb/internal/store"
)
```

Replace `buildFileOptions`:
```go
func buildFileOptions(cfg *config.Config) (file.Options, error) {
	if !cfg.Vision.Enabled {
		return file.Options{}, nil
	}
	prov, err := provider.New(cfg.Vision.Provider, cfg.Providers)
	if err != nil {
		return file.Options{}, fmt.Errorf("vision provider: %w", err)
	}
	return file.Options{
		Vision: &file.VisionOptions{
			Config: cfg.Vision,
			Client: prov.Client(),
		},
	}, nil
}
```

Update `newIngester`:
```go
func newIngester(cfg *config.Config) (*ingest.Ingester, store.Store, error) {
	st, err := store.NewSQLite(cfg.DB.Path)
	if err != nil {
		return nil, nil, fmt.Errorf("open store: %w", err)
	}
	emb, err := embedder.New(cfg.Embedder, cfg.Providers)
	if err != nil {
		return nil, nil, fmt.Errorf("create embedder: %w", err)
	}
	c := chunker.New(cfg.Chunker.ChunkSize, cfg.Chunker.ChunkOverlap)
	return ingest.New(st, c, emb), st, nil
}
```

Update `runIngestFile` and `runIngestAll` to use new `buildFileOptions` signature (returns error):

```go
func runIngestFile(cmd *cobra.Command, args []string) error {
	// ...existing source registration...
	
	opts, err := buildFileOptions(cfg)
	if err != nil {
		return err
	}

	ing, st, err := newIngester(cfg)
	if err != nil {
		return err
	}
	defer st.Close()

	ctx := cmd.Context()
	src := file.New(path, flagRecursive, exts, opts)
	// ...rest unchanged...
}
```

Update `runSource` similarly for the `file` case:
```go
case "file":
	// ...exts setup...
	opts, err := buildFileOptions(cfg)
	if err != nil {
		return err
	}
	s := file.New(src.Path, src.Recursive, exts, opts)
	// ...rest unchanged...
```

Remove the `oai` import — it's no longer needed in `cmd_ingest.go`.

- [ ] **Step 6: Update `cmd_search.go`**

Replace line 40:
```go
// old:
emb, err := embedder.New(cfg.Embedder, cfg.OpenAI)
// new:
emb, err := embedder.New(cfg.Embedder, cfg.Providers)
```

Update import: remove `oai "github.com/sashabaranov/go-openai"` if present, keep `embedder` import.

- [ ] **Step 7: Update `cmd_serve.go`**

Replace line 31:
```go
// old:
emb, err := embedder.New(cfg.Embedder, cfg.OpenAI)
// new:
emb, err := embedder.New(cfg.Embedder, cfg.Providers)
```

- [ ] **Step 8: Build**

```bash
CGO_ENABLED=1 go build -o kb . 2>&1
```

Expected: clean

- [ ] **Step 9: Run all tests**

```bash
CGO_ENABLED=1 go test ./... -v 2>&1 | grep -E "^(ok|FAIL|--- FAIL)"
```

Expected: all packages `ok`

- [ ] **Step 10: Smoke test CLI**

```bash
./kb --help
./kb config init --help
```

Expected: no errors, help text shows

- [ ] **Step 11: Commit**

```bash
git add internal/embedder/ internal/adapters/file/file.go cmd_ingest.go cmd_search.go cmd_serve.go
git commit -m "refactor(provider): wire provider system into embedder, vision, and CLI

- embedder.New() now takes config.ProvidersConfig instead of config.OpenAIConfig
- embedder/openai: add NewWithClient() for provider-injected construction
- cmd_ingest: buildFileOptions() uses provider.New() for vision client
- cmd_search, cmd_serve: embedder.New() uses cfg.Providers
- Remove direct oai.DefaultConfig usage from CLI commands"
```

---

## Self-Review

### Spec Coverage

| Requirement | Task |
|---|---|
| `internal/provider` package | Task 2 |
| `internal/provider/openai/openai.go` | Task 2 |
| `internal/provider/azure/azure.go` | Task 2 |
| `provider.Provider` interface with `Client()` and `Name()` | Task 2 |
| `provider.New(name, cfg)` factory | Task 2 |
| Azure requires `base_url` — error if missing | Task 2 |
| Azure default `api_version: 2024-02-15-preview` | Task 1 (viper default) |
| `ProvidersConfig` with `OpenAI` and `Azure` sub-structs | Task 1 |
| `VisionConfig.Provider string` | Task 1 |
| `KB_OPENAI_API_KEY` maps to `providers.openai.api_key` | Task 1 |
| `KB_AZURE_API_KEY`, `KB_AZURE_BASE_URL`, `KB_AZURE_API_VERSION` | Task 1 |
| `embedder.New()` uses `ProvidersConfig` | Task 3 |
| `buildFileOptions()` uses `provider.New()` | Task 3 |
| All CLI commands updated | Task 3 |
| All tests pass | Task 3 |
| No backward compat with old `openai.api_key` config key | Task 1 (no migration code) |
