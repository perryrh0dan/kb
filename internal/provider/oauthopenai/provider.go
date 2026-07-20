package oauthopenai

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"

	oai "github.com/sashabaranov/go-openai"
	"golang.org/x/oauth2/clientcredentials"

	"github.com/user/kb/internal/config"
)

type oauthOpenAIProvider struct {
	client *oai.Client
}

// New creates an OAuthOpenAI provider that authenticates via OAuth2 client
// credentials and injects both Authorization: Bearer and api-key headers.
// cfg.Routing controls the API path convention:
//   - "azure" (default): uses Azure deployment paths (/openai/deployments/{model}/...)
//   - "openai": uses standard OpenAI paths (/v1/embeddings)
func New(cfg config.OAuthOpenAIProviderConfig) (*oauthOpenAIProvider, error) {
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("oauth_openai provider requires endpoint")
	}
	if cfg.TokenURL == "" {
		return nil, fmt.Errorf("oauth_openai provider requires token_url")
	}
	if cfg.ClientID == "" {
		return nil, fmt.Errorf("oauth_openai provider requires client_id")
	}
	if cfg.ClientSecret == "" {
		return nil, fmt.Errorf("oauth_openai provider requires client_secret")
	}
	if cfg.Scope == "" {
		return nil, fmt.Errorf("oauth_openai provider requires scope")
	}

	ccCfg := &clientcredentials.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		TokenURL:     cfg.TokenURL,
		Scopes:       []string{cfg.Scope},
	}
	tokenSource := ccCfg.TokenSource(context.Background())

	// Clone DefaultTransport to inherit system proxy settings and connection-pool
	// defaults, then apply any TLS overrides on top.
	base := http.DefaultTransport.(*http.Transport).Clone()
	if cfg.TLSInsecureSkipVerify || cfg.TLSCACertFile != "" {
		tlsCfg := &tls.Config{}
		if cfg.TLSInsecureSkipVerify {
			tlsCfg.InsecureSkipVerify = true //nolint:gosec
		}
		if cfg.TLSCACertFile != "" {
			pem, err := os.ReadFile(cfg.TLSCACertFile)
			if err != nil {
				return nil, fmt.Errorf("oauth_openai: read tls_ca_cert_file %q: %w", cfg.TLSCACertFile, err)
			}
			pool := x509.NewCertPool()
			if !pool.AppendCertsFromPEM(pem) {
				return nil, fmt.Errorf("oauth_openai: no valid PEM certificates found in %q", cfg.TLSCACertFile)
			}
			tlsCfg.RootCAs = pool
		}
		base.TLSClientConfig = tlsCfg
	}
	inner := base

	transport := NewTokenTransport(tokenSource, cfg.APIKey, inner)

	apiVersion := cfg.APIVersion
	if apiVersion == "" {
		apiVersion = "2024-02-15-preview"
	}

	var oaiCfg oai.ClientConfig
	routing := cfg.Routing
	if routing == "" {
		routing = "azure"
	}
	switch routing {
	case "azure":
		oaiCfg = oai.DefaultAzureConfig("", cfg.Endpoint)
		oaiCfg.APIVersion = apiVersion
	case "openai":
		oaiCfg = oai.DefaultConfig("")
		oaiCfg.BaseURL = cfg.Endpoint
	default:
		return nil, fmt.Errorf("oauth_openai: unknown routing %q: must be \"azure\" or \"openai\"", routing)
	}
	oaiCfg.HTTPClient = &http.Client{Transport: transport}

	return &oauthOpenAIProvider{client: oai.NewClientWithConfig(oaiCfg)}, nil
}

func (p *oauthOpenAIProvider) Client() *oai.Client { return p.client }
func (p *oauthOpenAIProvider) Name() string        { return "oauth_openai" }
