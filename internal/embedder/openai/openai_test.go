// internal/embedder/openai/openai_test.go
package openai_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/user/kb/config"
	oaiemb "github.com/user/kb/internal/embedder/openai"
)

func TestEmbed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"object": "list",
			"data": []map[string]interface{}{
				{"object": "embedding", "index": 0, "embedding": make([]float64, 3072)},
				{"object": "embedding", "index": 1, "embedding": make([]float64, 3072)},
			},
			"model": "text-embedding-3-large",
			"usage": map[string]int{"prompt_tokens": 10, "total_tokens": 10},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	emb, err := oaiemb.NewWithBaseURL(
		config.EmbedderConfig{Provider: "openai", Model: "text-embedding-3-large"},
		"sk-test",
		srv.URL+"/v1",
	)
	if err != nil {
		t.Fatalf("NewWithBaseURL: %v", err)
	}

	texts := []string{"hello world", "foo bar"}
	vecs, err := emb.Embed(context.Background(), texts)
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(vecs) != 2 {
		t.Errorf("got %d vectors, want 2", len(vecs))
	}
	if len(vecs[0]) != 3072 {
		t.Errorf("got %d dims, want 3072", len(vecs[0]))
	}
}

func TestDimensions(t *testing.T) {
	emb, err := oaiemb.NewWithBaseURL(
		config.EmbedderConfig{Provider: "openai", Model: "text-embedding-3-large"},
		"sk-test",
		"http://localhost",
	)
	if err != nil {
		t.Fatalf("NewWithBaseURL: %v", err)
	}
	if emb.Dimensions() != 3072 {
		t.Errorf("Dimensions() = %d, want 3072", emb.Dimensions())
	}
}
