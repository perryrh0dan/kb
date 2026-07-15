package embedder

import (
	"context"
	"fmt"

	"github.com/user/kb/config"
)

// Embedder converts text slices into float32 vectors.
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	Dimensions() int
	ModelName() string
}

// New creates an Embedder based on the provider in cfg.
func New(cfg config.EmbedderConfig) (Embedder, error) {
	switch cfg.Provider {
	case "openai":
		// imported in Task 5
		return nil, fmt.Errorf("openai embedder: not yet registered (import openai subpackage)")
	default:
		return nil, fmt.Errorf("unknown embedder provider: %q", cfg.Provider)
	}
}
