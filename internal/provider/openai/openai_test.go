package openai_test

import (
	"testing"

	"github.com/user/kb/config"
	oaiprovider "github.com/user/kb/internal/provider/openai"
)

func TestNewOpenAIProvider(t *testing.T) {
	cfg := config.ProviderConfig{APIKey: "sk-test"}
	p, err := oaiprovider.New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if p.Name() != "openai" {
		t.Errorf("Name() = %q, want %q", p.Name(), "openai")
	}
	if p.Client() == nil {
		t.Error("Client() should not be nil")
	}
}
