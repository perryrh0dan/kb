package oauthopenai_test

import (
	"testing"

	"github.com/user/kb/internal/config"
	"github.com/user/kb/internal/provider/oauthopenai"
)

var validCfg = config.OAuthOpenAIProviderConfig{
	Endpoint:     "https://api.example.com",
	APIKey:       "test-key",
	TokenURL:     "https://login.microsoftonline.com/tenant-id/oauth2/v2.0/token",
	ClientID:     "client-id",
	ClientSecret: "client-secret",
	Scope:        "api://scope/.default",
	APIVersion:   "2024-02-15-preview",
	Routing:      "azure",
}

func TestNew_ValidConfig(t *testing.T) {
	p, err := oauthopenai.New(validCfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if p.Name() != "oauth_openai" {
		t.Errorf("Name() = %q, want %q", p.Name(), "oauth_openai")
	}
	if p.Client() == nil {
		t.Error("Client() should not be nil")
	}
}

func TestNew_OpenAIRouting(t *testing.T) {
	cfg := validCfg
	cfg.Routing = "openai"
	p, err := oauthopenai.New(cfg)
	if err != nil {
		t.Fatalf("New with openai routing: %v", err)
	}
	if p.Client() == nil {
		t.Error("Client() should not be nil")
	}
}

func TestNew_InvalidRouting(t *testing.T) {
	cfg := validCfg
	cfg.Routing = "invalid"
	_, err := oauthopenai.New(cfg)
	if err == nil {
		t.Error("expected error for invalid routing")
	}
}

func TestNew_MissingEndpoint(t *testing.T) {
	cfg := validCfg
	cfg.Endpoint = ""
	_, err := oauthopenai.New(cfg)
	if err == nil {
		t.Error("expected error for missing endpoint")
	}
}

func TestNew_MissingTokenURL(t *testing.T) {
	cfg := validCfg
	cfg.TokenURL = ""
	_, err := oauthopenai.New(cfg)
	if err == nil {
		t.Error("expected error for missing token_url")
	}
}

func TestNew_MissingClientID(t *testing.T) {
	cfg := validCfg
	cfg.ClientID = ""
	_, err := oauthopenai.New(cfg)
	if err == nil {
		t.Error("expected error for missing client_id")
	}
}

func TestNew_MissingClientSecret(t *testing.T) {
	cfg := validCfg
	cfg.ClientSecret = ""
	_, err := oauthopenai.New(cfg)
	if err == nil {
		t.Error("expected error for missing client_secret")
	}
}

func TestNew_MissingScope(t *testing.T) {
	cfg := validCfg
	cfg.Scope = ""
	_, err := oauthopenai.New(cfg)
	if err == nil {
		t.Error("expected error for missing scope")
	}
}
