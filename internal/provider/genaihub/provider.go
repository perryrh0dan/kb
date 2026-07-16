package genaihub

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"

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

	// Build TLS configuration.
	tlsCfg := &tls.Config{}
	if cfg.TLSInsecureSkipVerify {
		tlsCfg.InsecureSkipVerify = true //nolint:gosec
	}
	if cfg.TLSCACertFile != "" {
		pem, err := os.ReadFile(cfg.TLSCACertFile)
		if err != nil {
			return nil, fmt.Errorf("genai_hub: read tls_ca_cert_file %q: %w", cfg.TLSCACertFile, err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("genai_hub: no valid PEM certificates found in %q", cfg.TLSCACertFile)
		}
		tlsCfg.RootCAs = pool
	}
	inner := &http.Transport{TLSClientConfig: tlsCfg}

	transport := NewTokenTransport(tokenSource, cfg.APIKey, inner)

	apiVersion := cfg.APIVersion
	if apiVersion == "" {
		apiVersion = "2024-02-15-preview"
	}

	// Use Azure-style config so go-openai routes to the correct deployment path:
	// /openai/deployments/{model}/embeddings?api-version=...
	// The empty string as apiKey avoids go-openai adding its own api-key header
	// (our tokenTransport already injects it from cfg.APIKey).
	oaiCfg := oai.DefaultAzureConfig("", cfg.Endpoint)
	oaiCfg.APIVersion = apiVersion
	oaiCfg.HTTPClient = &http.Client{Transport: transport}

	return &genAIHubProvider{client: oai.NewClientWithConfig(oaiCfg)}, nil
}

func (p *genAIHubProvider) Client() *oai.Client { return p.client }
func (p *genAIHubProvider) Name() string        { return "genai_hub" }
