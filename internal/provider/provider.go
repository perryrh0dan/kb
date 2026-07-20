package provider

import (
	"fmt"

	oai "github.com/sashabaranov/go-openai"
	"github.com/user/kb/internal/config"
	azprovider "github.com/user/kb/internal/provider/azure"
	oauthopenai "github.com/user/kb/internal/provider/oauthopenai"
	oaiprovider "github.com/user/kb/internal/provider/openai"
)

// Provider wraps a configured API client for use by embedder and vision.
type Provider interface {
	// Client returns the underlying go-openai client.
	// Both OpenAI and Azure use the same client type; the difference is
	// in how the client was configured (DefaultConfig vs DefaultAzureConfig).
	Client() *oai.Client

	// Name returns the provider identifier: "openai", "azure", or "oauth_openai".
	Name() string
}

// New creates a Provider for the given name using the credentials in cfg.
// name must be "openai", "azure", or "oauth_openai". An empty or unrecognised name defaults to "openai".
func New(name string, cfg config.ProvidersConfig) (Provider, error) {
	switch name {
	case "azure":
		return azprovider.New(cfg.Azure)
	case "oauth_openai":
		return oauthopenai.New(cfg.OAuthOpenAI)
	case "openai", "":
		return oaiprovider.New(cfg.OpenAI)
	default:
		return nil, fmt.Errorf("unknown provider %q: must be \"openai\", \"azure\", or \"oauth_openai\"", name)
	}
}
