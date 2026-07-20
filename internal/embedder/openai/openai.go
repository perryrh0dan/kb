package openai

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	oai "github.com/sashabaranov/go-openai"
	"github.com/user/kb/internal/config"
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

	// Filter out empty/whitespace-only strings before sending to the API.
	// Track original indices so we can reconstruct a full-length result slice.
	type indexedText struct {
		orig int
		text string
	}
	var nonempty []indexedText
	for i, t := range texts {
		if strings.TrimSpace(t) != "" {
			nonempty = append(nonempty, indexedText{i, t})
		}
	}

	results := make([][]float32, len(texts)) // nil entries for empty inputs

	for batchStart := 0; batchStart < len(nonempty); batchStart += batchSize {
		batchEnd := batchStart + batchSize
		if batchEnd > len(nonempty) {
			batchEnd = len(nonempty)
		}
		batch := nonempty[batchStart:batchEnd]
		strs := make([]string, len(batch))
		for j, it := range batch {
			strs[j] = it.text
		}
		log.Debug("embedding batch", "batch_start", batchStart, "batch_end", batchEnd, "model", e.model)
		resp, err := e.client.CreateEmbeddings(ctx, oai.EmbeddingRequest{
			Input: strs,
			Model: e.model,
		})
		if err != nil {
			log.Warn("embedding batch failed", "batch_start", batchStart, "batch_end", batchEnd, "error", err)
			return nil, fmt.Errorf("embed batch [%d:%d]: %w", batchStart, batchEnd, err)
		}
		for j, d := range resp.Data {
			results[batch[j].orig] = d.Embedding
		}
	}
	return results, nil
}
