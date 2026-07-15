package config

// EmbedderConfig holds configuration for the embedding provider.
type EmbedderConfig struct {
	Provider string `mapstructure:"provider"`
	Model    string `mapstructure:"model"`
}
