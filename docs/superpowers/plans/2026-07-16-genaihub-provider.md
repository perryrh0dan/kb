# GenAI Hub Provider Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `genai_hub` provider to `kb` that authenticates via OAuth2 client credentials (Azure AD) and injects both `Authorization: Bearer` and `api-key` headers on every request.

**Architecture:** New package `internal/provider/genaihub` with a `tokenTransport` (http.RoundTripper) that auto-fetches and refreshes OAuth2 tokens, and a `genAIHubProvider` that wires that transport into a `go-openai` client. Config gains a `GenAIHubProviderConfig` struct and 7 env var bindings. The existing `azure` and `openai` providers are untouched.

**Tech Stack:** Go, `golang.org/x/oauth2/clientcredentials` (already in go.sum), `github.com/sashabaranov/go-openai`, `github.com/spf13/viper`, `net/http/httptest` for tests.

## Global Constraints

- Go module: `github.com/user/kb`
- `golang.org/x/oauth2` is already an indirect dep — promote to direct with `go get golang.org/x/oauth2`
- CGO_ENABLED=1 required to build (sqlite-vec)
- All new env vars use the `KB_` prefix
- Default `api_version`: `"2024-02-15-preview"`
- Provider `Name()` must return exact string `"genai_hub"`
- No changes to `openai` or `azure` providers

---

### Task 1: Config — add `GenAIHubProviderConfig` and env bindings

**Files:**
- Modify: `config/config.go`

**Interfaces:**
- Produces: `config.GenAIHubProviderConfig` struct, `config.ProvidersConfig.GenAIHub` field

- [ ] **Step 1: Add the new config struct and field**

In `config/config.go`, after the `AzureProviderConfig` struct, add:

```go
// GenAIHubProviderConfig holds credentials for the KFW GenAI Hub gateway.
// Authentication uses OAuth2 client credentials (Azure AD) plus an API key header.
type GenAIHubProviderConfig struct {
	Endpoint     string `mapstructure:"endpoint"       yaml:"endpoint"`
	APIKey       string `mapstructure:"api_key"        yaml:"api_key"`
	ClientID     string `mapstructure:"client_id"      yaml:"client_id"`
	ClientSecret string `mapstructure:"client_secret"  yaml:"client_secret"`
	TenantID     string `mapstructure:"tenant_id"      yaml:"tenant_id"`
	Scope        string `mapstructure:"scope"          yaml:"scope"`
	APIVersion   string `mapstructure:"api_version"    yaml:"api_version"`
}
```

Update `ProvidersConfig`:

```go
type ProvidersConfig struct {
	OpenAI   ProviderConfig         `mapstructure:"openai"    yaml:"openai"`
	Azure    AzureProviderConfig    `mapstructure:"azure"     yaml:"azure"`
	GenAIHub GenAIHubProviderConfig `mapstructure:"genai_hub" yaml:"genai_hub"`
}
```

- [ ] **Step 2: Add default and env var bindings in `newViper()`**

Inside `newViper()`, after the existing Azure default/bindings, add:

```go
v.SetDefault("providers.genai_hub.api_version", "2024-02-15-preview")

v.BindEnv("providers.genai_hub.endpoint",      "KB_GENAI_HUB_ENDPOINT")       //nolint:errcheck
v.BindEnv("providers.genai_hub.api_key",       "KB_GENAI_HUB_API_KEY")        //nolint:errcheck
v.BindEnv("providers.genai_hub.client_id",     "KB_GENAI_HUB_CLIENT_ID")      //nolint:errcheck
v.BindEnv("providers.genai_hub.client_secret", "KB_GENAI_HUB_CLIENT_SECRET")  //nolint:errcheck
v.BindEnv("providers.genai_hub.tenant_id",     "KB_GENAI_HUB_TENANT_ID")      //nolint:errcheck
v.BindEnv("providers.genai_hub.scope",         "KB_GENAI_HUB_SCOPE")          //nolint:errcheck
v.BindEnv("providers.genai_hub.api_version",   "KB_GENAI_HUB_API_VERSION")    //nolint:errcheck
```

- [ ] **Step 3: Add commented block to `InitDefault()` template**

In the `content` string inside `InitDefault()`, after the `azure:` block, add:

```yaml
  genai_hub:      # optional — KFW GenAI Hub gateway (OAuth2 + API key)
    endpoint: ""       # e.g. https://api.genai-hub.example.com  (KB_GENAI_HUB_ENDPOINT)
    api_key: ""        # KB_GENAI_HUB_API_KEY
    client_id: ""      # KB_GENAI_HUB_CLIENT_ID
    client_secret: ""  # KB_GENAI_HUB_CLIENT_SECRET
    tenant_id: ""      # KB_GENAI_HUB_TENANT_ID
    scope: ""          # e.g. api://d6c63b5b-.../.default  (KB_GENAI_HUB_SCOPE)
    api_version: "2024-02-15-preview"  # KB_GENAI_HUB_API_VERSION
```

- [ ] **Step 4: Verify config compiles**

```bash
cd /root/workspace/kb
CGO_ENABLED=1 go build ./config/...
```

Expected: no output (success).

- [ ] **Step 5: Commit**

```bash
git add config/config.go
git commit -m "feat(config): add GenAIHubProviderConfig with env bindings"
```

---

### Task 2: Token transport

**Files:**
- Create: `internal/provider/genaihub/transport.go`
- Create: `internal/provider/genaihub/transport_test.go`

**Interfaces:**
- Consumes: `golang.org/x/oauth2.TokenSource`
- Produces: `tokenTransport` struct implementing `http.RoundTripper`; constructor `newTokenTransport(ts oauth2.TokenSource, apiKey string, inner http.RoundTripper) *tokenTransport`

- [ ] **Step 1: Promote oauth2 to a direct dependency**

```bash
cd /root/workspace/kb
go get golang.org/x/oauth2@v0.35.0
```

Expected: go.mod updated with `golang.org/x/oauth2` as a direct require.

- [ ] **Step 2: Write the failing test**

Create `internal/provider/genaihub/transport_test.go`:

```go
package genaihub_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/oauth2"

	genaihub "github.com/user/kb/internal/provider/genaihub"
)

// staticTokenSource returns a fixed token — no real HTTP call needed.
type staticTokenSource struct{ tok *oauth2.Token }

func (s *staticTokenSource) Token() (*oauth2.Token, error) { return s.tok, nil }

func TestTokenTransport_InjectsHeaders(t *testing.T) {
	var gotAuth, gotKey string
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotKey = r.Header.Get("api-key")
		w.WriteHeader(http.StatusOK)
	}))
	defer apiSrv.Close()

	ts := &staticTokenSource{tok: &oauth2.Token{AccessToken: "test-bearer-token"}}
	transport := genaihub.NewTokenTransport(ts, "my-api-key", nil)

	client := &http.Client{Transport: transport}
	resp, err := client.Get(apiSrv.URL)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()

	if gotAuth != "Bearer test-bearer-token" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer test-bearer-token")
	}
	if gotKey != "my-api-key" {
		t.Errorf("api-key = %q, want %q", gotKey, "my-api-key")
	}
}

func TestTokenTransport_NoAPIKey(t *testing.T) {
	var gotKey string
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("api-key")
		w.WriteHeader(http.StatusOK)
	}))
	defer apiSrv.Close()

	ts := &staticTokenSource{tok: &oauth2.Token{AccessToken: "tok"}}
	transport := genaihub.NewTokenTransport(ts, "", nil)

	client := &http.Client{Transport: transport}
	resp, err := client.Get(apiSrv.URL)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()

	if gotKey != "" {
		t.Errorf("api-key header should be absent, got %q", gotKey)
	}
}
```

- [ ] **Step 3: Run test to confirm it fails**

```bash
cd /root/workspace/kb
go test ./internal/provider/genaihub/... 2>&1
```

Expected: compile error — package does not exist yet.

- [ ] **Step 4: Implement transport**

Create `internal/provider/genaihub/transport.go`:

```go
package genaihub

import (
	"fmt"
	"net/http"

	"golang.org/x/oauth2"
)

// tokenTransport is an http.RoundTripper that injects an OAuth2 Bearer token
// and optionally an api-key header on every outbound request.
type tokenTransport struct {
	tokenSource oauth2.TokenSource
	apiKey      string
	inner       http.RoundTripper
}

// NewTokenTransport creates a tokenTransport.
// inner may be nil — http.DefaultTransport is used in that case.
func NewTokenTransport(ts oauth2.TokenSource, apiKey string, inner http.RoundTripper) *tokenTransport {
	if inner == nil {
		inner = http.DefaultTransport
	}
	return &tokenTransport{tokenSource: ts, apiKey: apiKey, inner: inner}
}

func (t *tokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	tok, err := t.tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("genaihub: fetch oauth2 token: %w", err)
	}

	// Clone the request to avoid mutating the original.
	r2 := req.Clone(req.Context())
	r2.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	if t.apiKey != "" {
		r2.Header.Set("api-key", t.apiKey)
	}
	return t.inner.RoundTrip(r2)
}
```

- [ ] **Step 5: Run tests and confirm they pass**

```bash
cd /root/workspace/kb
go test ./internal/provider/genaihub/... -run TestTokenTransport -v
```

Expected:
```
--- PASS: TestTokenTransport_InjectsHeaders
--- PASS: TestTokenTransport_NoAPIKey
PASS
```

- [ ] **Step 6: Commit**

```bash
git add internal/provider/genaihub/transport.go internal/provider/genaihub/transport_test.go go.mod go.sum
git commit -m "feat(genaihub): add OAuth2 token transport"
```

---

### Task 3: GenAI Hub provider

**Files:**
- Create: `internal/provider/genaihub/provider.go`
- Create: `internal/provider/genaihub/provider_test.go`

**Interfaces:**
- Consumes: `config.GenAIHubProviderConfig`, `genaihub.NewTokenTransport`
- Produces: `genaihub.New(cfg config.GenAIHubProviderConfig) (*genAIHubProvider, error)` — satisfies `provider.Provider` interface (`Client() *oai.Client`, `Name() string`)

- [ ] **Step 1: Write failing tests**

Create `internal/provider/genaihub/provider_test.go`:

```go
package genaihub_test

import (
	"testing"

	"github.com/user/kb/config"
	genaihub "github.com/user/kb/internal/provider/genaihub"
)

var validCfg = config.GenAIHubProviderConfig{
	Endpoint:     "https://api.genai-hub.example.com",
	APIKey:       "test-key",
	ClientID:     "client-id",
	ClientSecret: "client-secret",
	TenantID:     "tenant-id",
	Scope:        "api://scope/.default",
	APIVersion:   "2024-02-15-preview",
}

func TestNew_ValidConfig(t *testing.T) {
	p, err := genaihub.New(validCfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if p.Name() != "genai_hub" {
		t.Errorf("Name() = %q, want %q", p.Name(), "genai_hub")
	}
	if p.Client() == nil {
		t.Error("Client() should not be nil")
	}
}

func TestNew_MissingEndpoint(t *testing.T) {
	cfg := validCfg
	cfg.Endpoint = ""
	_, err := genaihub.New(cfg)
	if err == nil {
		t.Error("expected error for missing endpoint")
	}
}

func TestNew_MissingClientID(t *testing.T) {
	cfg := validCfg
	cfg.ClientID = ""
	_, err := genaihub.New(cfg)
	if err == nil {
		t.Error("expected error for missing client_id")
	}
}

func TestNew_MissingClientSecret(t *testing.T) {
	cfg := validCfg
	cfg.ClientSecret = ""
	_, err := genaihub.New(cfg)
	if err == nil {
		t.Error("expected error for missing client_secret")
	}
}

func TestNew_MissingTenantID(t *testing.T) {
	cfg := validCfg
	cfg.TenantID = ""
	_, err := genaihub.New(cfg)
	if err == nil {
		t.Error("expected error for missing tenant_id")
	}
}

func TestNew_MissingScope(t *testing.T) {
	cfg := validCfg
	cfg.Scope = ""
	_, err := genaihub.New(cfg)
	if err == nil {
		t.Error("expected error for missing scope")
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
cd /root/workspace/kb
go test ./internal/provider/genaihub/... -run TestNew -v 2>&1
```

Expected: compile error — `genaihub.New` undefined.

- [ ] **Step 3: Implement the provider**

Create `internal/provider/genaihub/provider.go`:

```go
package genaihub

import (
	"context"
	"fmt"
	"net/http"

	oai "github.com/sashabaranov/go-openai"
	"golang.org/x/oauth2/clientcredentials"

	"github.com/user/kb/config"
)

type genAIHubProvider struct {
	client *oai.Client
}

// New creates a GenAI Hub provider that authenticates via OAuth2 client
// credentials and injects both Authorization: Bearer and api-key headers.
func New(cfg config.GenAIHubProviderConfig) (*genAIHubProvider, error) {
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("genai_hub provider requires endpoint")
	}
	if cfg.ClientID == "" {
		return nil, fmt.Errorf("genai_hub provider requires client_id")
	}
	if cfg.ClientSecret == "" {
		return nil, fmt.Errorf("genai_hub provider requires client_secret")
	}
	if cfg.TenantID == "" {
		return nil, fmt.Errorf("genai_hub provider requires tenant_id")
	}
	if cfg.Scope == "" {
		return nil, fmt.Errorf("genai_hub provider requires scope")
	}

	tokenURL := fmt.Sprintf(
		"https://login.microsoftonline.com/%s/oauth2/v2.0/token",
		cfg.TenantID,
	)
	ccCfg := &clientcredentials.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		TokenURL:     tokenURL,
		Scopes:       []string{cfg.Scope},
	}
	tokenSource := ccCfg.TokenSource(context.Background())

	transport := NewTokenTransport(tokenSource, cfg.APIKey, nil)

	apiVersion := cfg.APIVersion
	if apiVersion == "" {
		apiVersion = "2024-02-15-preview"
	}

	oaiCfg := oai.DefaultAzureConfig("", cfg.Endpoint)
	oaiCfg.APIVersion = apiVersion
	oaiCfg.HTTPClient = &http.Client{Transport: transport}

	return &genAIHubProvider{client: oai.NewClientWithConfig(oaiCfg)}, nil
}

func (p *genAIHubProvider) Client() *oai.Client { return p.client }
func (p *genAIHubProvider) Name() string        { return "genai_hub" }
```

- [ ] **Step 4: Run all genaihub tests**

```bash
cd /root/workspace/kb
go test ./internal/provider/genaihub/... -v
```

Expected: all 7 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/provider/genaihub/provider.go internal/provider/genaihub/provider_test.go
git commit -m "feat(genaihub): add GenAI Hub provider with OAuth2 client credentials"
```

---

### Task 4: Wire `genai_hub` into the provider registry

**Files:**
- Modify: `internal/provider/provider.go`

**Interfaces:**
- Consumes: `genaihub.New`, `config.ProvidersConfig.GenAIHub`
- Produces: `provider.New("genai_hub", cfg)` works end-to-end

- [ ] **Step 1: Add the `genai_hub` case**

In `internal/provider/provider.go`, add the import and case:

```go
import (
    // existing imports ...
    genaihub "github.com/user/kb/internal/provider/genaihub"
)

func New(name string, cfg config.ProvidersConfig) (Provider, error) {
    switch name {
    case "azure":
        return azprovider.New(cfg.Azure)
    case "genai_hub":
        return genaihub.New(cfg.GenAIHub)
    case "openai", "":
        return oaiprovider.New(cfg.OpenAI)
    default:
        return nil, fmt.Errorf("unknown provider %q: must be \"openai\", \"azure\", or \"genai_hub\"", name)
    }
}
```

- [ ] **Step 2: Full build check**

```bash
cd /root/workspace/kb
CGO_ENABLED=1 go build ./...
```

Expected: no output (success).

- [ ] **Step 3: Run all tests**

```bash
cd /root/workspace/kb
CGO_ENABLED=1 go test ./...
```

Expected: all existing tests pass, new genaihub tests pass.

- [ ] **Step 4: Commit**

```bash
git add internal/provider/provider.go
git commit -m "feat(provider): register genai_hub provider"
```

---

### Task 5: Update README with GenAI Hub configuration

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Add GenAI Hub section to README**

In `README.md`, after the existing Azure OpenAI configuration section (or after the Quick Start section if Azure is not documented), add:

```markdown
## GenAI Hub (KFW Azure OpenAI Gateway)

The GenAI Hub provider authenticates using OAuth2 client credentials (Azure AD)
and also sends an `api-key` header. Configure via `~/.kb/config.yaml` or environment variables:

```yaml
providers:
  genai_hub:
    endpoint: "https://api.genai-hub.example.com"
    api_key: ""             # KB_GENAI_HUB_API_KEY
    client_id: ""           # KB_GENAI_HUB_CLIENT_ID
    client_secret: ""       # KB_GENAI_HUB_CLIENT_SECRET
    tenant_id: ""           # KB_GENAI_HUB_TENANT_ID
    scope: "api://d6c63b5b-.../.default"  # KB_GENAI_HUB_SCOPE
    api_version: "2024-02-15-preview"

embedder:
  provider: genai_hub
  model: text-embedding-3-large   # deployment name on the hub
```

All fields can also be set via environment variables (see table above).
```

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "docs: add GenAI Hub configuration section to README"
```
