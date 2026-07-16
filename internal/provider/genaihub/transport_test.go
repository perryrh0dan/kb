package genaihub_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/oauth2"

	genaihub "github.com/user/kb/internal/provider/genaihub"
)

// staticTokenSource returns a fixed token — no real HTTP call needed.
type staticTokenSource struct{ tok *oauth2.Token }

func (s *staticTokenSource) Token() (*oauth2.Token, error) { return s.tok, nil }

func TestTokenTransport_InjectsHeaders(t *testing.T) {
	var gotAuth, gotKey string
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotKey = r.Header.Get("api-key")
		w.WriteHeader(http.StatusOK)
	}))
	defer apiSrv.Close()

	ts := &staticTokenSource{tok: &oauth2.Token{AccessToken: "test-bearer-token"}}
	transport := genaihub.NewTokenTransport(ts, "my-api-key", nil)

	client := &http.Client{Transport: transport}
	resp, err := client.Get(apiSrv.URL)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()

	if gotAuth != "Bearer test-bearer-token" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer test-bearer-token")
	}
	if gotKey != "my-api-key" {
		t.Errorf("api-key = %q, want %q", gotKey, "my-api-key")
	}
}

func TestTokenTransport_NoAPIKey(t *testing.T) {
	var gotKey string
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("api-key")
		w.WriteHeader(http.StatusOK)
	}))
	defer apiSrv.Close()

	ts := &staticTokenSource{tok: &oauth2.Token{AccessToken: "tok"}}
	transport := genaihub.NewTokenTransport(ts, "", nil)

	client := &http.Client{Transport: transport}
	resp, err := client.Get(apiSrv.URL)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()

	if gotKey != "" {
		t.Errorf("api-key header should be absent, got %q", gotKey)
	}
}
