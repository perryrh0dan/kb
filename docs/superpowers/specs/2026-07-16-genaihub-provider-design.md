# Design: GenAI Hub Provider

**Date:** 2026-07-16  
**Status:** Approved

## Summary

Add a `genai_hub` provider to `kb` that authenticates with the KFW GenAI Hub gateway using OAuth2 client credentials flow (Azure AD), sending both an `Authorization: Bearer <token>` header and an `api-key` header on every request. The existing `openai` and `azure` providers are untouched.

---

## Background

The KFW GenAI Hub is an Azure-hosted OpenAI-compatible gateway that requires two forms of authentication on each API call:

1. **OAuth2 Bearer token** — obtained via Azure AD client credentials flow (`client_id`, `client_secret`, `tenant_id`, `scope`)
2. **API key** — sent as the `api-key` request header

`golang.org/x/oauth2/clientcredentials` is already an indirect dependency and handles token caching and auto-refresh.

---

## Config

### New struct in `config/config.go`

```go
type GenAIHubProviderConfig struct {
    Endpoint     string `mapstructure:"endpoint"      yaml:"endpoint"`
    APIKey       string `mapstructure:"api_key"       yaml:"api_key"`
    ClientID     string `mapstructure:"client_id"     yaml:"client_id"`
    ClientSecret string `mapstructure:"client_secret" yaml:"client_secret"`
    TenantID     string `mapstructure:"tenant_id"     yaml:"tenant_id"`
    Scope        string `mapstructure:"scope"         yaml:"scope"`
    APIVersion   string `mapstructure:"api_version"   yaml:"api_version"`
}
```

### Updated `ProvidersConfig`

```go
type ProvidersConfig struct {
    OpenAI   ProviderConfig         `mapstructure:"openai"    yaml:"openai"`
    Azure    AzureProviderConfig    `mapstructure:"azure"     yaml:"azure"`
    GenAIHub GenAIHubProviderConfig `mapstructure:"genai_hub" yaml:"genai_hub"`
}
```

### Env var bindings (all prefixed `KB_`)

| Env var                    | Config field                          |
|----------------------------|---------------------------------------|
| `KB_GENAI_HUB_ENDPOINT`    | `providers.genai_hub.endpoint`        |
| `KB_GENAI_HUB_API_KEY`     | `providers.genai_hub.api_key`         |
| `KB_GENAI_HUB_CLIENT_ID`   | `providers.genai_hub.client_id`       |
| `KB_GENAI_HUB_CLIENT_SECRET` | `providers.genai_hub.client_secret` |
| `KB_GENAI_HUB_TENANT_ID`   | `providers.genai_hub.tenant_id`       |
| `KB_GENAI_HUB_SCOPE`       | `providers.genai_hub.scope`           |
| `KB_GENAI_HUB_API_VERSION` | `providers.genai_hub.api_version`     |

Default value for `api_version`: `"2024-02-15-preview"` (same as Azure).

---

## Architecture

### New package: `internal/provider/genaihub`

Two files:

#### `transport.go`

Implements `http.RoundTripper`. Wraps an inner transport (defaults to `http.DefaultTransport`) and injects auth headers on every request:

- Calls `tokenSource.Token()` — auto-refreshed by `golang.org/x/oauth2`
- Sets `Authorization: Bearer <token>`
- Sets `api-key: <api_key>` (if non-empty)

```
tokenTransport{
    tokenSource oauth2.TokenSource  // from clientcredentials.Config.TokenSource()
    apiKey      string
    inner       http.RoundTripper   // nil → http.DefaultTransport
}
```

#### `provider.go`

`New(cfg config.GenAIHubProviderConfig) (*genAIHubProvider, error)`

1. Validates: `endpoint`, `client_id`, `client_secret`, `tenant_id`, `scope` must be non-empty
2. Builds `clientcredentials.Config` with `TokenURL = https://login.microsoftonline.com/<TenantID>/oauth2/v2.0/token`
3. Creates `tokenTransport`
4. Builds `oai.ClientConfig`:
   - `BaseURL` = `cfg.Endpoint`
   - `APIType` = `oai.APITypeAzure`
   - `APIVersion` = cfg.APIVersion (default `"2024-02-15-preview"`)
   - `HTTPClient` = `&http.Client{Transport: tokenTransport}`
5. Returns provider wrapping `oai.NewClientWithConfig(oaiCfg)`

Satisfies the existing `Provider` interface:
- `Client() *oai.Client`
- `Name() string` → `"genai_hub"`

### Updated `internal/provider/provider.go`

Adds `"genai_hub"` case:
```go
case "genai_hub":
    return genaihub.New(cfg.GenAIHub)
```

### Updated `config/config.go`

- Adds `GenAIHubProviderConfig` struct and field on `ProvidersConfig`
- Adds 7 new `BindEnv` calls
- Adds default for `providers.genai_hub.api_version`
- Adds commented `genai_hub:` block to `InitDefault()` template

---

## Testing

### `internal/provider/genaihub/transport_test.go`

Uses `httptest.NewServer` for both a mock token endpoint and a mock API endpoint. Verifies:
- `Authorization: Bearer <token>` header is present
- `api-key: <key>` header is present
- Token is fetched from the token endpoint (not hardcoded)

### `internal/provider/genaihub/provider_test.go`

Unit tests for `New()`:
- Returns error when `endpoint` is empty
- Returns error when `client_id` is empty
- Returns error when `client_secret` is empty
- Returns error when `tenant_id` is empty
- Returns error when `scope` is empty
- Returns valid provider for valid config
- `Name()` returns `"genai_hub"`
- `Client()` is non-nil

---

## Non-Goals

- No changes to the `azure` or `openai` providers
- No new dependencies (oauth2 already present as indirect; promoted to direct)
- No changes to CLI commands or MCP server logic
