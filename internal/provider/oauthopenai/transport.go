package oauthopenai

import (
	"fmt"
	"net/http"

	"golang.org/x/oauth2"
)

// tokenTransport is an http.RoundTripper that injects an OAuth2 Bearer token
// and optionally an api-key header on every outbound request.
type tokenTransport struct {
	tokenSource oauth2.TokenSource
	apiKey      string
	inner       http.RoundTripper
}

// NewTokenTransport creates a tokenTransport.
// inner may be nil — http.DefaultTransport is used in that case.
func NewTokenTransport(ts oauth2.TokenSource, apiKey string, inner http.RoundTripper) *tokenTransport {
	if inner == nil {
		inner = http.DefaultTransport
	}
	return &tokenTransport{tokenSource: ts, apiKey: apiKey, inner: inner}
}

func (t *tokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	tok, err := t.tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("oauthopenai: fetch oauth2 token: %w", err)
	}

	r2 := req.Clone(req.Context())
	r2.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	if t.apiKey != "" {
		r2.Header.Set("api-key", t.apiKey)
	}
	return t.inner.RoundTrip(r2)
}
