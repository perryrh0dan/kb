package confluence_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/user/kb/config"
	"github.com/user/kb/internal/adapters/confluence"
)

func mockConfluenceServer() *httptest.Server {
	mux := http.NewServeMux()
	// v2 pages endpoint
	mux.HandleFunc("/wiki/api/v2/spaces/ENG/pages", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"results": []map[string]interface{}{
				{
					"id":    "12345",
					"title": "Test Page",
					"body": map[string]interface{}{
						"storage": map[string]interface{}{
							"value": "<p>Hello <strong>world</strong></p>",
						},
					},
					"version": map[string]interface{}{"createdAt": "2026-01-01T00:00:00Z"},
					"_links":  map[string]interface{}{"webui": "/display/ENG/Test+Page"},
				},
			},
			"_links": map[string]interface{}{}, // no next page
		}
		json.NewEncoder(w).Encode(resp)
	})
	return httptest.NewServer(mux)
}

func TestConfluenceAdapterFetchesPages(t *testing.T) {
	srv := mockConfluenceServer()
	defer srv.Close()

	cfg := config.ConfluenceConfig{
		BaseURL:  srv.URL,
		Username: "user@example.com",
		APIToken: "token123",
	}
	src := confluence.New(cfg, "ENG", "")
	ch, err := src.Documents(context.Background())
	if err != nil {
		t.Fatalf("Documents: %v", err)
	}
	var docs []string
	for d := range ch {
		docs = append(docs, d.ID)
	}
	if len(docs) != 1 {
		t.Errorf("got %d docs, want 1", len(docs))
	}
	if docs[0] != "confluence://ENG/12345" {
		t.Errorf("ID = %q, want %q", docs[0], "confluence://ENG/12345")
	}
}

func TestConfluenceHTMLStripped(t *testing.T) {
	srv := mockConfluenceServer()
	defer srv.Close()

	cfg := config.ConfluenceConfig{BaseURL: srv.URL, Username: "u", APIToken: "t"}
	src := confluence.New(cfg, "ENG", "")
	ch, _ := src.Documents(context.Background())
	doc := <-ch

	if doc.Content == "" {
		t.Error("content should not be empty")
	}
	// HTML tags should be stripped
	if len(doc.Content) > 0 && doc.Content[0] == '<' {
		t.Errorf("content still contains HTML: %q", doc.Content[:20])
	}
}

func TestConfluencePATAuth(t *testing.T) {
	var receivedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		resp := map[string]interface{}{
			"results": []map[string]interface{}{},
			"_links":  map[string]interface{}{},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	cfg := config.ConfluenceConfig{
		BaseURL: srv.URL,
		PAT:     "my-pat-token",
	}
	src := confluence.New(cfg, "ENG", "")
	ch, err := src.Documents(context.Background())
	if err != nil {
		t.Fatalf("Documents: %v", err)
	}
	for range ch {} // drain

	if receivedAuth != "Bearer my-pat-token" {
		t.Errorf("Authorization header = %q, want %q", receivedAuth, "Bearer my-pat-token")
	}
}
