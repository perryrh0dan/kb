# kb — Private Knowledge Base CLI & MCP Server

`kb` is a single Go binary that ingests documents from local files and Confluence, stores them as vector embeddings in a local SQLite database, and exposes search via CLI or as an MCP server for use with OpenCode and other AI tools.

## Features

- Ingest Markdown, plain text, and PDF files recursively from local directories
- Ingest pages from Confluence Cloud (API token) or Confluence Data Center (PAT)
- Semantic search powered by OpenAI `text-embedding-3-large`
- Incremental sync with hash-based change detection and pruning of deleted documents
- MCP stdio server with `search_knowledge_base`, `list_sources`, and `get_document` tools
- Single binary, no external services required (SQLite with sqlite-vec `vec0` KNN search)

## Installation

### Prerequisites

- Go 1.21+
- CGO enabled (required for SQLite)
- An OpenAI API key

### Build from source

```bash
git clone https://github.com/user/kb
cd kb
CGO_ENABLED=1 go build -o kb .
# Optionally move to PATH
mv kb /usr/local/bin/kb
```

## Quick Start

### 1. Initialize configuration

```bash
kb config init
```

This creates `~/.kb/config.yaml` with default settings. Edit it to add your OpenAI API key and any sources.

### Benchmark search

Measure local vector-search latency without changing the database. Query embedding is
performed once and reported separately from the repeated local search timing:

```bash
kb benchmark "Self Service Prozess" --iterations 20 --limit 10
```


### 2. Ingest local files

```bash
kb ingest file ./docs/
```

Ingests all `.md`, `.txt`, and `.pdf` files recursively. Subsequent runs only process changed or new files.

### 3. Search the knowledge base

```bash
kb search "query"
```

Returns the top matching document chunks with source, score, and content preview.

### 4. Check status

```bash
kb status
```

Shows all registered sources, document counts, and database statistics.

## OpenCode MCP Integration

To use `kb` as an MCP server with OpenCode, add the following to your `opencode.json`:

```json
{
  "mcpServers": {
    "kb": {
      "command": "/path/to/kb",
      "args": ["serve"]
    }
  }
}
```

Replace `/path/to/kb` with the absolute path to the `kb` binary. OpenCode will start the MCP server automatically and expose three tools:

| Tool | Description |
|---|---|
| `search_knowledge_base` | Semantic search over all ingested documents |
| `list_sources` | List all registered ingest sources |
| `get_document` | Retrieve a specific document by ID |

## Environment Variables

| Variable | Description | Default |
|---|---|---|
| `KB_OPENAI_API_KEY` | OpenAI API key for generating embeddings | — |
| `KB_DB_PATH` | Path to the SQLite database file | `~/.kb/kb.db` |
| `KB_CONFLUENCE_API_TOKEN` | Confluence Cloud API token (email:token format) | — |
| `KB_CONFLUENCE_PAT` | Confluence Data Center personal access token | — |
| `KB_OAUTH_OPENAI_ENDPOINT` | oauth_openai gateway URL | — |
| `KB_OAUTH_OPENAI_TOKEN_URL` | OAuth2 token endpoint URL | — |
| `KB_OAUTH_OPENAI_CLIENT_ID` | OAuth2 client ID | — |
| `KB_OAUTH_OPENAI_CLIENT_SECRET` | OAuth2 client secret | — |
| `KB_OAUTH_OPENAI_SCOPE` | OAuth2 scope | — |

Environment variables override values set in the config file.

> See the [OAuth OpenAI Provider](#oauth-openai-provider) section for the full list of `KB_OAUTH_OPENAI_*` variables.

## Confluence Integration

### Confluence Cloud

```bash
# Set credentials
export KB_CONFLUENCE_API_TOKEN="user@example.com:ATATT..."

# Ingest a space
kb ingest confluence --space ENG --url https://yourorg.atlassian.net

# Ingest a specific page and its children
kb ingest confluence --space ENG --url https://yourorg.atlassian.net --page 12345
```

### Confluence Data Center (PAT)

```bash
# Set PAT
export KB_CONFLUENCE_PAT="your-personal-access-token"

# Ingest a space
kb ingest confluence --space ENG --url https://confluence.yourcompany.com
```

The adapter automatically detects Cloud vs. Data Center based on the URL and the auth token provided.

## Search Options

```bash
kb search "kubernetes deployment" --limit 10 --min-score 0.7 --source ./docs/
```

| Flag | Description | Default |
|---|---|---|
| `--limit` | Maximum number of results to return | `5` |
| `--min-score` | Minimum similarity score (0.0–1.0) | `0.0` |
| `--source` | Filter results to a specific source path | — |

## Ingest Options

```bash
kb ingest file ./docs/ --ext .md --force
```

| Flag | Description |
|---|---|
| `--ext` | Only ingest files with this extension (e.g. `.md`) |
| `--force` | Force re-index of all documents, even unchanged ones |

## Running Tests

### Unit tests

```bash
CGO_ENABLED=1 go test ./...
```

### Integration tests (requires `KB_OPENAI_API_KEY`)

```bash
export KB_OPENAI_API_KEY="sk-..."
CGO_ENABLED=1 go test -tags integration ./...
```

## Configuration File

`kb config init` creates a YAML config at `~/.kb/config.yaml`:

```yaml
db_path: ~/.kb/kb.db
openai_api_key: ""   # or set KB_OPENAI_API_KEY
sources: []
```

Sources are automatically registered when you run `kb ingest file` or `kb ingest confluence` and stored in the config for subsequent incremental syncs.

## OAuth OpenAI Provider

The `oauth_openai` provider supports any OpenAI-compatible API endpoint protected by OAuth2 client credentials. It fetches a Bearer token from a configurable `token_url` (any OAuth2-compliant identity provider) and optionally sends an additional `api-key` header. This works with Azure OpenAI gateways, corporate proxies, or any endpoint that follows the client credentials flow.

Configure via `~/.kb/config.yaml` or environment variables:

```yaml
providers:
  oauth_openai:
    endpoint: "https://api.my-gateway.example.com"
    api_key: ""             # KB_OAUTH_OPENAI_API_KEY (optional — sent as api-key header)
    token_url: "https://login.microsoftonline.com/YOUR_TENANT_ID/oauth2/v2.0/token"  # KB_OAUTH_OPENAI_TOKEN_URL
    client_id: ""           # KB_OAUTH_OPENAI_CLIENT_ID
    client_secret: ""       # KB_OAUTH_OPENAI_CLIENT_SECRET
    scope: "api://your-app-id/.default"  # KB_OAUTH_OPENAI_SCOPE
    api_version: "2024-02-15-preview"    # KB_OAUTH_OPENAI_API_VERSION
    routing: "azure"        # "azure" (deployment paths) or "openai" (/v1 paths)  (KB_OAUTH_OPENAI_ROUTING)
    # tls_insecure_skip_verify: false
    # tls_ca_cert_file: ""  # path to PEM CA cert (KB_OAUTH_OPENAI_TLS_CA_CERT_FILE)

embedder:
  provider: oauth_openai
  model: your-deployment-name   # deployment name for azure routing, model name for openai routing
```

The `routing` field controls the API path convention:
- `"azure"` (default): Azure deployment paths (`/openai/deployments/{model}/embeddings?api-version=...`)
- `"openai"`: Standard OpenAI paths (`/v1/embeddings`)

| Env var | Config field |
|---|---|
| `KB_OAUTH_OPENAI_ENDPOINT` | `providers.oauth_openai.endpoint` |
| `KB_OAUTH_OPENAI_API_KEY` | `providers.oauth_openai.api_key` |
| `KB_OAUTH_OPENAI_TOKEN_URL` | `providers.oauth_openai.token_url` |
| `KB_OAUTH_OPENAI_CLIENT_ID` | `providers.oauth_openai.client_id` |
| `KB_OAUTH_OPENAI_CLIENT_SECRET` | `providers.oauth_openai.client_secret` |
| `KB_OAUTH_OPENAI_SCOPE` | `providers.oauth_openai.scope` |
| `KB_OAUTH_OPENAI_API_VERSION` | `providers.oauth_openai.api_version` |
| `KB_OAUTH_OPENAI_ROUTING` | `providers.oauth_openai.routing` |
| `KB_OAUTH_OPENAI_TLS_INSECURE_SKIP_VERIFY` | `providers.oauth_openai.tls_insecure_skip_verify` |
| `KB_OAUTH_OPENAI_TLS_CA_CERT_FILE` | `providers.oauth_openai.tls_ca_cert_file` |

## License

MIT
