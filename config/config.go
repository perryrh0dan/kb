package config

import (
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type OpenAIConfig struct {
	APIKey string `mapstructure:"api_key"`
}

type ConfluenceConfig struct {
	BaseURL  string `mapstructure:"base_url"`
	Username string `mapstructure:"username"`
	APIToken string `mapstructure:"api_token"`
	PAT      string `mapstructure:"pat"`
}

type DBConfig struct {
	Path string `mapstructure:"path"`
}

type EmbedderConfig struct {
	Provider string `mapstructure:"provider"`
	Model    string `mapstructure:"model"`
}

type ChunkerConfig struct {
	ChunkSize    int `mapstructure:"chunk_size"`
	ChunkOverlap int `mapstructure:"chunk_overlap"`
}

type SourceConfig struct {
	Type       string   `mapstructure:"type"`
	Path       string   `mapstructure:"path,omitempty"`
	Recursive  bool     `mapstructure:"recursive,omitempty"`
	Extensions []string `mapstructure:"extensions,omitempty"`
	Space      string   `mapstructure:"space,omitempty"`
	PageID     string   `mapstructure:"page_id,omitempty"`
}

type Config struct {
	OpenAI     OpenAIConfig     `mapstructure:"openai"`
	Confluence ConfluenceConfig `mapstructure:"confluence"`
	DB         DBConfig         `mapstructure:"db"`
	Embedder   EmbedderConfig   `mapstructure:"embedder"`
	Chunker    ChunkerConfig    `mapstructure:"chunker"`
	Sources    []SourceConfig   `mapstructure:"sources"`
}

func defaultConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".kb", "config.yaml")
}

func newViper() *viper.Viper {
	v := viper.New()
	v.SetDefault("embedder.provider", "openai")
	v.SetDefault("embedder.model", "text-embedding-3-large")
	v.SetDefault("chunker.chunk_size", 512)
	v.SetDefault("chunker.chunk_overlap", 50)
	v.SetDefault("db.path", filepath.Join(mustHomeDir(), ".kb", "kb.db"))

	v.SetEnvPrefix("KB")
	v.BindEnv("openai.api_key", "KB_OPENAI_API_KEY")       //nolint:errcheck
	v.BindEnv("confluence.api_token", "KB_CONFLUENCE_API_TOKEN") //nolint:errcheck
	v.BindEnv("confluence.pat", "KB_CONFLUENCE_PAT")        //nolint:errcheck
	v.BindEnv("db.path", "KB_DB_PATH")                      //nolint:errcheck

	return v
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

openai:
  api_key: ""  # or set KB_OPENAI_API_KEY env var

confluence:
  base_url: ""
  username: ""       # Cloud: Confluence username/email
  api_token: ""      # Cloud: API token (or KB_CONFLUENCE_API_TOKEN)
  pat: ""            # Data Center: Personal Access Token (or KB_CONFLUENCE_PAT)

db:
  path: ~/.kb/kb.db  # or set KB_DB_PATH env var

embedder:
  provider: openai
  model: text-embedding-3-large

chunker:
  chunk_size: 512
  chunk_overlap: 50

# sources are auto-registered when you run: kb ingest file <path> / kb ingest confluence --space <KEY>
sources: []
`
	return os.WriteFile(path, []byte(content), 0600)
}

// Save writes the config back to disk (used to register sources).
func Save(cfg *Config) error {
	path := defaultConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	v := newViper()
	v.Set("openai", cfg.OpenAI)
	v.Set("confluence", cfg.Confluence)
	v.Set("db", cfg.DB)
	v.Set("embedder", cfg.Embedder)
	v.Set("chunker", cfg.Chunker)
	v.Set("sources", cfg.Sources)
	return v.WriteConfigAs(path)
}
