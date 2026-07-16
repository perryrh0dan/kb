package confluence_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/user/kb/config"
	"github.com/user/kb/internal/adapters/confluence"
)

func mockConfluenceServer() *httptest.Server {
	mux := http.NewServeMux()
	// Scan endpoint — no body needed, just metadata
	mux.HandleFunc("/wiki/api/v2/spaces/ENG/pages", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"results": []map[string]interface{}{
				{
					"id":      "12345",
					"title":   "Test Page",
					"version": map[string]interface{}{"createdAt": "2026-01-01T00:00:00Z"},
					"_links":  map[string]interface{}{"webui": "/display/ENG/Test+Page"},
				},
			},
			"_links": map[string]interface{}{},
		}
		json.NewEncoder(w).Encode(resp)
	})
	// Load endpoint — single page with body
	mux.HandleFunc("/wiki/api/v2/pages/12345", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"id":    "12345",
			"title": "Test Page",
			"body": map[string]interface{}{
				"storage": map[string]interface{}{
					"value": "<p>Hello <strong>world</strong></p>",
				},
			},
			"version": map[string]interface{}{"createdAt": "2026-01-01T00:00:00Z"},
			"_links":  map[string]interface{}{"webui": "/display/ENG/Test+Page"},
		}
		json.NewEncoder(w).Encode(resp)
	})
	return httptest.NewServer(mux)
}

func TestConfluenceScanFetchesPageMeta(t *testing.T) {
	srv := mockConfluenceServer()
	defer srv.Close()

	cfg := config.ConfluenceConfig{BaseURL: srv.URL, Username: "u", APIToken: "t"}
	src := confluence.New(cfg, "ENG", "")
	ch, err := src.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	var metas []string
	for m := range ch {
		metas = append(metas, m.ID)
	}
	if len(metas) != 1 {
		t.Errorf("got %d metas, want 1", len(metas))
	}
	if metas[0] != "confluence://ENG/12345" {
		t.Errorf("ID = %q, want %q", metas[0], "confluence://ENG/12345")
	}
}

func TestConfluenceScanHashDeterministic(t *testing.T) {
	srv := mockConfluenceServer()
	defer srv.Close()

	cfg := config.ConfluenceConfig{BaseURL: srv.URL, Username: "u", APIToken: "t"}
	src := confluence.New(cfg, "ENG", "")

	ch1, _ := src.Scan(context.Background())
	meta1 := <-ch1

	ch2, _ := src.Scan(context.Background())
	meta2 := <-ch2

	if meta1.ContentHash != meta2.ContentHash {
		t.Errorf("hash not deterministic: %q != %q", meta1.ContentHash, meta2.ContentHash)
	}
	if meta1.ContentHash == "" {
		t.Error("ContentHash should not be empty")
	}
}

func TestConfluenceLoadReturnsContent(t *testing.T) {
	srv := mockConfluenceServer()
	defer srv.Close()

	cfg := config.ConfluenceConfig{BaseURL: srv.URL, Username: "u", APIToken: "t"}
	src := confluence.New(cfg, "ENG", "")

	ch, _ := src.Scan(context.Background())
	meta := <-ch

	doc, err := src.Load(context.Background(), meta)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if doc.Content == "" {
		t.Error("content should not be empty after Load")
	}
	if strings.Contains(doc.Content, "<") {
		preview := doc.Content
		if len(preview) > 20 {
			preview = preview[:20]
		}
		t.Errorf("HTML not stripped: %q", preview)
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

	cfg := config.ConfluenceConfig{BaseURL: srv.URL, PAT: "my-pat-token"}
	src := confluence.New(cfg, "ENG", "")
	ch, err := src.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	for range ch {}

	if receivedAuth != "Bearer my-pat-token" {
		t.Errorf("Authorization header = %q, want %q", receivedAuth, "Bearer my-pat-token")
	}
}
