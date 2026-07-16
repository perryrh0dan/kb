package openai

import (
	oai "github.com/sashabaranov/go-openai"
	"github.com/user/kb/config"
)

type openAIProvider struct {
	client *oai.Client
}

// New creates an OpenAI provider using the standard OpenAI API endpoint.
func New(cfg config.ProviderConfig) (*openAIProvider, error) {
	oaiCfg := oai.DefaultConfig(cfg.APIKey)
	return &openAIProvider{client: oai.NewClientWithConfig(oaiCfg)}, nil
}

func (p *openAIProvider) Client() *oai.Client { return p.client }
func (p *openAIProvider) Name() string        { return "openai" }
