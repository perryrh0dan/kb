package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

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

	// TLS options — use when the hub endpoint uses a private/corporate CA.
	TLSInsecureSkipVerify bool   `mapstructure:"tls_insecure_skip_verify" yaml:"tls_insecure_skip_verify,omitempty"`
	TLSCACertFile         string `mapstructure:"tls_ca_cert_file"         yaml:"tls_ca_cert_file,omitempty"`
}

// ProvidersConfig holds configuration for all supported LLM/embedding providers.
type ProvidersConfig struct {
	OpenAI   ProviderConfig         `mapstructure:"openai"    yaml:"openai"`
	Azure    AzureProviderConfig    `mapstructure:"azure"     yaml:"azure"`
	GenAIHub GenAIHubProviderConfig `mapstructure:"genai_hub" yaml:"genai_hub"`
}

type ConfluenceConfig struct {
	BaseURL  string `mapstructure:"base_url"  yaml:"base_url"`
	Username string `mapstructure:"username"  yaml:"username"`
	APIToken string `mapstructure:"api_token" yaml:"api_token"`
	PAT      string `mapstructure:"pat"       yaml:"pat"`
}

type DBConfig struct {
	Path string `mapstructure:"path" yaml:"path"`
}

type EmbedderConfig struct {
	Provider string `mapstructure:"provider" yaml:"provider"`
	Model    string `mapstructure:"model"    yaml:"model"`
}

type ChunkerConfig struct {
	ChunkSize    int `mapstructure:"chunk_size"    yaml:"chunk_size"`
	ChunkOverlap int `mapstructure:"chunk_overlap" yaml:"chunk_overlap"`
}

type VisionConfig struct {
	Enabled  bool    `mapstructure:"enabled"   yaml:"enabled"`
	Provider string  `mapstructure:"provider"  yaml:"provider"`
	Model    string  `mapstructure:"model"     yaml:"model"`
	DPI      float64 `mapstructure:"dpi"       yaml:"dpi"`
}

type SourceConfig struct {
	Type       string   `mapstructure:"type"                 yaml:"type"`
	Path       string   `mapstructure:"path,omitempty"       yaml:"path,omitempty"`
	Recursive  bool     `mapstructure:"recursive,omitempty"  yaml:"recursive,omitempty"`
	Extensions []string `mapstructure:"extensions,omitempty" yaml:"extensions,omitempty"`
	Space      string   `mapstructure:"space,omitempty"      yaml:"space,omitempty"`
	PageID     string   `mapstructure:"page_id,omitempty"    yaml:"page_id,omitempty"`
}

type Config struct {
	Providers  ProvidersConfig  `mapstructure:"providers"   yaml:"providers"`
	Confluence ConfluenceConfig `mapstructure:"confluence"  yaml:"confluence"`
	DB         DBConfig         `mapstructure:"db"          yaml:"db"`
	Embedder   EmbedderConfig   `mapstructure:"embedder"    yaml:"embedder"`
	Chunker    ChunkerConfig    `mapstructure:"chunker"     yaml:"chunker"`
	Vision     VisionConfig     `mapstructure:"vision"      yaml:"vision"`
	Sources    []SourceConfig   `mapstructure:"sources"     yaml:"sources"`
}

func defaultConfigPath() string {
	return filepath.Join(mustHomeDir(), ".kb", "config.yaml")
}

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

	v.SetDefault("providers.genai_hub.api_version", "2024-02-15-preview")

	v.BindEnv("providers.genai_hub.endpoint",      "KB_GENAI_HUB_ENDPOINT")       //nolint:errcheck
	v.BindEnv("providers.genai_hub.api_key",       "KB_GENAI_HUB_API_KEY")        //nolint:errcheck
	v.BindEnv("providers.genai_hub.client_id",     "KB_GENAI_HUB_CLIENT_ID")      //nolint:errcheck
	v.BindEnv("providers.genai_hub.client_secret", "KB_GENAI_HUB_CLIENT_SECRET")  //nolint:errcheck
	v.BindEnv("providers.genai_hub.tenant_id",     "KB_GENAI_HUB_TENANT_ID")      //nolint:errcheck
	v.BindEnv("providers.genai_hub.scope",                   "KB_GENAI_HUB_SCOPE")                    //nolint:errcheck
	v.BindEnv("providers.genai_hub.api_version",             "KB_GENAI_HUB_API_VERSION")              //nolint:errcheck
	v.BindEnv("providers.genai_hub.tls_insecure_skip_verify","KB_GENAI_HUB_TLS_INSECURE_SKIP_VERIFY") //nolint:errcheck
	v.BindEnv("providers.genai_hub.tls_ca_cert_file",        "KB_GENAI_HUB_TLS_CA_CERT_FILE")         //nolint:errcheck

	v.BindEnv("confluence.api_token",        "KB_CONFLUENCE_API_TOKEN")  //nolint:errcheck
	v.BindEnv("confluence.pat",              "KB_CONFLUENCE_PAT")        //nolint:errcheck
	v.BindEnv("db.path",                     "KB_DB_PATH")               //nolint:errcheck

	return v
}

// expandHome replaces a leading ~/ with the current user's home directory.
func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func mustHomeDir() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return h
}

// Load reads config from the default path (~/.kb/config.yaml) with env-var overrides.
func Load() (*Config, error) {
	return LoadFrom(defaultConfigPath())
}

// LoadFrom reads config from the given file path with env-var overrides.
func LoadFrom(path string) (*Config, error) {
	v := newViper()
	v.SetConfigFile(path)

	if err := v.ReadInConfig(); err != nil {
		if !os.IsNotExist(err) {
			// File exists but couldn't be read
			if _, statErr := os.Stat(path); statErr == nil {
				return nil, err
			}
			// File doesn't exist — use defaults + env vars only
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	cfg.DB.Path = expandHome(cfg.DB.Path)
	return &cfg, nil
}

// InitDefault writes a default config file to ~/.kb/config.yaml.
func InitDefault() error {
	path := defaultConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		return nil // already exists
	}
	content := `# kb configuration

providers:
  openai:
    api_key: ""  # or set KB_OPENAI_API_KEY env var

  azure:         # optional — only fill in if using Azure OpenAI
    api_key: ""  # or set KB_AZURE_API_KEY env var
    base_url: "" # e.g. https://my-resource.openai.azure.com/
    api_version: "2024-02-15-preview"  # or set KB_AZURE_API_VERSION

  genai_hub:      # optional — KFW GenAI Hub gateway (OAuth2 + API key)
    endpoint: ""       # e.g. https://api.genai-hub.example.com  (KB_GENAI_HUB_ENDPOINT)
    api_key: ""        # KB_GENAI_HUB_API_KEY
    client_id: ""      # KB_GENAI_HUB_CLIENT_ID
    client_secret: ""  # KB_GENAI_HUB_CLIENT_SECRET
    tenant_id: ""      # KB_GENAI_HUB_TENANT_ID
    scope: ""          # e.g. api://d6c63b5b-.../.default  (KB_GENAI_HUB_SCOPE)
    api_version: "2024-02-15-preview"  # KB_GENAI_HUB_API_VERSION
    # tls_insecure_skip_verify: false  # set true to skip TLS cert validation (not for production)
    # tls_ca_cert_file: ""             # path to PEM CA cert for private/corporate CAs (KB_GENAI_HUB_TLS_CA_CERT_FILE)

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
	return os.WriteFile(path, []byte(content), 0600)
}

// Save writes the config back to disk using yaml tags for correct key names.
func Save(cfg *Config) error {
	path := defaultConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	b, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0600)
}
