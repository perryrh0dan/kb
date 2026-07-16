package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

type OpenAIConfig struct {
	APIKey string `mapstructure:"api_key" yaml:"api_key"`
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

type SourceConfig struct {
	Type       string   `mapstructure:"type"                 yaml:"type"`
	Path       string   `mapstructure:"path,omitempty"       yaml:"path,omitempty"`
	Recursive  bool     `mapstructure:"recursive,omitempty"  yaml:"recursive,omitempty"`
	Extensions []string `mapstructure:"extensions,omitempty" yaml:"extensions,omitempty"`
	Space      string   `mapstructure:"space,omitempty"      yaml:"space,omitempty"`
	PageID     string   `mapstructure:"page_id,omitempty"    yaml:"page_id,omitempty"`
}

type Config struct {
	OpenAI     OpenAIConfig     `mapstructure:"openai"      yaml:"openai"`
	Confluence ConfluenceConfig `mapstructure:"confluence"  yaml:"confluence"`
	DB         DBConfig         `mapstructure:"db"          yaml:"db"`
	Embedder   EmbedderConfig   `mapstructure:"embedder"    yaml:"embedder"`
	Chunker    ChunkerConfig    `mapstructure:"chunker"     yaml:"chunker"`
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

	v.SetEnvPrefix("KB")
	v.BindEnv("openai.api_key", "KB_OPENAI_API_KEY")            //nolint:errcheck
	v.BindEnv("confluence.api_token", "KB_CONFLUENCE_API_TOKEN") //nolint:errcheck
	v.BindEnv("confluence.pat", "KB_CONFLUENCE_PAT")            //nolint:errcheck
	v.BindEnv("db.path", "KB_DB_PATH")                          //nolint:errcheck

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
