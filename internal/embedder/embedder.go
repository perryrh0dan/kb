package embedder

import (
	"context"
	"fmt"

	"github.com/user/kb/internal/config"
	oaiemb "github.com/user/kb/internal/embedder/openai"
	"github.com/user/kb/internal/provider"
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
