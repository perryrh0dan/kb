package genaihub

import (
	"context"
	"fmt"
	"net/http"

	oai "github.com/sashabaranov/go-openai"
	"golang.org/x/oauth2/clientcredentials"

	"github.com/user/kb/config"
)

type genAIHubProvider struct {
	client *oai.Client
}

// New creates a GenAI Hub provider that authenticates via OAuth2 client
// credentials and injects both Authorization: Bearer and api-key headers.
func New(cfg config.GenAIHubProviderConfig) (*genAIHubProvider, error) {
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("genai_hub provider requires endpoint")
	}
	if cfg.ClientID == "" {
		return nil, fmt.Errorf("genai_hub provider requires client_id")
	}
	if cfg.ClientSecret == "" {
		return nil, fmt.Errorf("genai_hub provider requires client_secret")
	}
	if cfg.TenantID == "" {
		return nil, fmt.Errorf("genai_hub provider requires tenant_id")
	}
	if cfg.Scope == "" {
		return nil, fmt.Errorf("genai_hub provider requires scope")
	}

	tokenURL := fmt.Sprintf(
		"https://login.microsoftonline.com/%s/oauth2/v2.0/token",
		cfg.TenantID,
	)
	ccCfg := &clientcredentials.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		TokenURL:     tokenURL,
		Scopes:       []string{cfg.Scope},
	}
	tokenSource := ccCfg.TokenSource(context.Background())

	transport := NewTokenTransport(tokenSource, cfg.APIKey, nil)

	oaiCfg := oai.DefaultConfig("")
	oaiCfg.BaseURL = cfg.Endpoint
	oaiCfg.HTTPClient = &http.Client{Transport: transport}

	return &genAIHubProvider{client: oai.NewClientWithConfig(oaiCfg)}, nil
}

func (p *genAIHubProvider) Client() *oai.Client { return p.client }
func (p *genAIHubProvider) Name() string        { return "genai_hub" }
