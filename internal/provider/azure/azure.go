package azure

import (
	"fmt"

	oai "github.com/sashabaranov/go-openai"
	"github.com/user/kb/config"
)

type azureProvider struct {
	client *oai.Client
}

// New creates an Azure OpenAI provider.
// cfg.BaseURL is required — it is the Azure OpenAI resource endpoint,
// e.g. "https://my-resource.openai.azure.com/".
// cfg.APIVersion defaults to "2024-02-15-preview" if empty.
func New(cfg config.AzureProviderConfig) (*azureProvider, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("azure provider requires base_url (e.g. https://my-resource.openai.azure.com/)")
	}
	azCfg := oai.DefaultAzureConfig(cfg.APIKey, cfg.BaseURL)
	if cfg.APIVersion != "" {
		azCfg.APIVersion = cfg.APIVersion
	}
	return &azureProvider{client: oai.NewClientWithConfig(azCfg)}, nil
}

func (p *azureProvider) Client() *oai.Client { return p.client }
func (p *azureProvider) Name() string        { return "azure" }
