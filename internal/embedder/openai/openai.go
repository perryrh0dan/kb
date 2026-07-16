package openai

import (
	"context"
	"fmt"
	"log/slog"

	oai "github.com/sashabaranov/go-openai"
	"github.com/user/kb/config"
)

const batchSize = 100

type openAIEmbedder struct {
	client *oai.Client
	model  oai.EmbeddingModel
	dims   int
}

// NewWithClient creates an embedder using an already-configured oai.Client.
// This is the primary constructor used by the provider system.
func NewWithClient(client *oai.Client, embedCfg config.EmbedderConfig) (*openAIEmbedder, error) {
	return &openAIEmbedder{
		client: client,
		model:  oai.EmbeddingModel(embedCfg.Model),
		dims:   3072,
	}, nil
}

// NewWithBaseURL creates an embedder with a custom base URL. Used in tests only.
func NewWithBaseURL(embedCfg config.EmbedderConfig, apiKey string, baseURL string) (*openAIEmbedder, error) {
	cfg := oai.DefaultConfig(apiKey)
	if baseURL != "" {
		cfg.BaseURL = baseURL
	}
	return &openAIEmbedder{
		client: oai.NewClientWithConfig(cfg),
		model:  oai.EmbeddingModel(embedCfg.Model),
		dims:   3072,
	}, nil
}

func (e *openAIEmbedder) Dimensions() int   { return e.dims }
func (e *openAIEmbedder) ModelName() string { return string(e.model) }

func (e *openAIEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	log := slog.Default()
	var results [][]float32
	for i := 0; i < len(texts); i += batchSize {
		end := i + batchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch := texts[i:end]
		log.Debug("embedding batch", "batch_start", i, "batch_end", end, "model", e.model)
		resp, err := e.client.CreateEmbeddings(ctx, oai.EmbeddingRequest{
			Input: batch,
			Model: e.model,
		})
		if err != nil {
			log.Warn("embedding batch failed", "batch_start", i, "batch_end", end, "error", err)
			return nil, fmt.Errorf("embed batch [%d:%d]: %w", i, end, err)
		}
		for _, d := range resp.Data {
			results = append(results, d.Embedding)
		}
	}
	return results, nil
}
