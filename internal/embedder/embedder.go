package embedder

import (
	"context"
	"fmt"

	"github.com/user/kb/config"
	oaiemb "github.com/user/kb/internal/embedder/openai"
)

// Embedder converts text slices into float32 vectors.
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	Dimensions() int
	ModelName() string
}

// New creates an Embedder based on the provider in cfg.
func New(embedCfg config.EmbedderConfig, oaiCfg config.OpenAIConfig) (Embedder, error) {
	switch embedCfg.Provider {
	case "openai":
		return oaiemb.New(embedCfg, oaiCfg)
	default:
		return nil, fmt.Errorf("unknown embedder provider: %q", embedCfg.Provider)
	}
}
