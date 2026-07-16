package azure_test

import (
	"testing"

	"github.com/user/kb/config"
	azprovider "github.com/user/kb/internal/provider/azure"
)

func TestNewAzureProvider(t *testing.T) {
	cfg := config.AzureProviderConfig{
		APIKey:     "azure-test-key",
		BaseURL:    "https://my-resource.openai.azure.com/",
		APIVersion: "2024-02-15-preview",
	}
	p, err := azprovider.New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if p.Name() != "azure" {
		t.Errorf("Name() = %q, want %q", p.Name(), "azure")
	}
	if p.Client() == nil {
		t.Error("Client() should not be nil")
	}
}

func TestNewAzureProvider_MissingBaseURL(t *testing.T) {
	cfg := config.AzureProviderConfig{APIKey: "key"}
	_, err := azprovider.New(cfg)
	if err == nil {
		t.Error("expected error for missing base_url, got nil")
	}
}
