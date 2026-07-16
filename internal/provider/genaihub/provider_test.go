package genaihub_test

import (
	"testing"

	"github.com/user/kb/config"
	genaihub "github.com/user/kb/internal/provider/genaihub"
)

var validCfg = config.GenAIHubProviderConfig{
	Endpoint:     "https://api.genai-hub.example.com",
	APIKey:       "test-key",
	ClientID:     "client-id",
	ClientSecret: "client-secret",
	TenantID:     "tenant-id",
	Scope:        "api://scope/.default",
	APIVersion:   "2024-02-15-preview",
}

func TestNew_ValidConfig(t *testing.T) {
	p, err := genaihub.New(validCfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if p.Name() != "genai_hub" {
		t.Errorf("Name() = %q, want %q", p.Name(), "genai_hub")
	}
	if p.Client() == nil {
		t.Error("Client() should not be nil")
	}
}

func TestNew_MissingEndpoint(t *testing.T) {
	cfg := validCfg
	cfg.Endpoint = ""
	_, err := genaihub.New(cfg)
	if err == nil {
		t.Error("expected error for missing endpoint")
	}
}

func TestNew_MissingClientID(t *testing.T) {
	cfg := validCfg
	cfg.ClientID = ""
	_, err := genaihub.New(cfg)
	if err == nil {
		t.Error("expected error for missing client_id")
	}
}

func TestNew_MissingClientSecret(t *testing.T) {
	cfg := validCfg
	cfg.ClientSecret = ""
	_, err := genaihub.New(cfg)
	if err == nil {
		t.Error("expected error for missing client_secret")
	}
}

func TestNew_MissingTenantID(t *testing.T) {
	cfg := validCfg
	cfg.TenantID = ""
	_, err := genaihub.New(cfg)
	if err == nil {
		t.Error("expected error for missing tenant_id")
	}
}

func TestNew_MissingScope(t *testing.T) {
	cfg := validCfg
	cfg.Scope = ""
	_, err := genaihub.New(cfg)
	if err == nil {
		t.Error("expected error for missing scope")
	}
}
