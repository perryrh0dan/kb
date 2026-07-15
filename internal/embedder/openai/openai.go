package openai

import (
	"context"
	"fmt"

	oai "github.com/sashabaranov/go-openai"
	"github.com/user/kb/config"
)

const batchSize = 100

type openAIEmbedder struct {
	client *oai.Client
	model  oai.EmbeddingModel
	dims   int
}

// New creates an OpenAI embedder using the default OpenAI base URL.
func New(embedCfg config.EmbedderConfig, oaiCfg config.OpenAIConfig) (*openAIEmbedder, error) {
	return NewWithBaseURL(embedCfg, oaiCfg, "")
}

// NewWithBaseURL creates an OpenAI embedder with a custom base URL (for testing).
func NewWithBaseURL(embedCfg config.EmbedderConfig, oaiCfg config.OpenAIConfig, baseURL string) (*openAIEmbedder, error) {
	cfg := oai.DefaultConfig(oaiCfg.APIKey)
	if baseURL != "" {
		cfg.BaseURL = baseURL
	}
	client := oai.NewClientWithConfig(cfg)
	return &openAIEmbedder{
		client: client,
		model:  oai.EmbeddingModel(embedCfg.Model),
		dims:   3072,
	}, nil
}

func (e *openAIEmbedder) Dimensions() int   { return e.dims }
func (e *openAIEmbedder) ModelName() string { return string(e.model) }

func (e *openAIEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	var results [][]float32
	for i := 0; i < len(texts); i += batchSize {
		end := i + batchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch := texts[i:end]
		resp, err := e.client.CreateEmbeddings(ctx, oai.EmbeddingRequest{
			Input: batch,
			Model: e.model,
		})
		if err != nil {
			return nil, fmt.Errorf("openai embed batch [%d:%d]: %w", i, end, err)
		}
		for _, d := range resp.Data {
			results = append(results, d.Embedding)
		}
	}
	return results, nil
}
