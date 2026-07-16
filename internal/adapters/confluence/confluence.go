package confluence

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/user/kb/config"
	"github.com/user/kb/internal/adapters"
	"github.com/user/kb/internal/store"
)

var htmlTagRe = regexp.MustCompile(`<[^>]+>`)

func stripHTML(s string) string {
	s = htmlTagRe.ReplaceAllString(s, " ")
	s = strings.Join(strings.Fields(s), " ")
	return s
}

type confluenceSource struct {
	cfg    config.ConfluenceConfig
	space  string
	pageID string
	client *http.Client
}

// New creates a Confluence Source.
// pageID is optional; if set, only that page is scanned/loaded.
func New(cfg config.ConfluenceConfig, space, pageID string) adapters.Source {
	transport := http.DefaultTransport
	if cfg.TLSInsecureSkipVerify {
		slog.Warn("confluence: TLS certificate verification disabled — do not use in production")
		transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
		}
	}
	return &confluenceSource{
		cfg:    cfg,
		space:  space,
		pageID: pageID,
		client: &http.Client{Timeout: 30 * time.Second, Transport: transport},
	}
}

// ScopePrefix returns the document ID prefix for pruning.
func (c *confluenceSource) ScopePrefix() string {
	if c.pageID != "" {
		return fmt.Sprintf("confluence://%s/%s", c.space, c.pageID)
	}
	return fmt.Sprintf("confluence://%s/", c.space)
}

func (c *confluenceSource) doRequest(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if c.cfg.PAT != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.PAT)
	} else {
		req.SetBasicAuth(c.cfg.Username, c.cfg.APIToken)
	}
	req.Header.Set("Accept", "application/json")
	return c.client.Do(req)
}

// pageMeta is used for Scan — no body, just identity and version info.
// Handles both v1 (version.when) and v2 (version.createdAt) response shapes.
type pageMeta struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Version struct {
		CreatedAt string `json:"createdAt"` // v2
		When      string `json:"when"`      // v1
	} `json:"version"`
	Links struct {
		WebUI string `json:"webui"`
	} `json:"_links"`
}

// versionTS returns whichever timestamp field is populated.
func (p pageMeta) versionTS() string {
	if p.Version.CreatedAt != "" {
		return p.Version.CreatedAt
	}
	return p.Version.When
}

type pagesMetaResponse struct {
	Results []pageMeta `json:"results"`
	Links   struct {
		Next string `json:"next"`
	} `json:"_links"`
}

// pageBody is used for Load — includes the storage body.
type pageBody struct {
	pageMeta
	Body struct {
		Storage struct {
			Value string `json:"value"`
		} `json:"storage"`
	} `json:"body"`
}

// isV1 returns true when the config requests the v1 REST API.
func (c *confluenceSource) isV1() bool {
	return c.cfg.APIVersion == "v1"
}

// scanURL returns the initial page-listing URL for Scan.
func (c *confluenceSource) scanURL() string {
	if c.pageID != "" {
		if c.isV1() {
			return fmt.Sprintf("%s/rest/api/content/%s?expand=version", c.cfg.BaseURL, c.pageID)
		}
		return fmt.Sprintf("%s/wiki/api/v2/pages/%s", c.cfg.BaseURL, c.pageID)
	}
	if c.isV1() {
		return fmt.Sprintf("%s/rest/api/space/%s/content/page?limit=50&expand=version", c.cfg.BaseURL, c.space)
	}
	return fmt.Sprintf("%s/wiki/api/v2/spaces/%s/pages?limit=50", c.cfg.BaseURL, c.space)
}

// loadURL returns the URL to fetch a page body by ID.
func (c *confluenceSource) loadURL(pageID string) string {
	if c.isV1() {
		return fmt.Sprintf("%s/rest/api/content/%s?expand=body.storage", c.cfg.BaseURL, pageID)
	}
	return fmt.Sprintf("%s/wiki/api/v2/pages/%s?body-format=storage", c.cfg.BaseURL, pageID)
}

// pageWebURL returns a browser-friendly URL for a page given its webui link fragment.
func (c *confluenceSource) pageWebURL(webuiFragment string) string {
	if c.isV1() {
		// v1 webui links are already absolute paths like /spaces/SPACE/pages/123/Title
		return c.cfg.BaseURL + webuiFragment
	}
	return c.cfg.BaseURL + "/wiki" + webuiFragment
}

// Scan fetches page metadata (no body) and streams DocumentMeta.
// ContentHash is computed from pageID + ":" + version timestamp —
// deterministic and changes only when the page is actually updated.
func (c *confluenceSource) Scan(ctx context.Context) (<-chan adapters.DocumentMeta, error) {
	log := slog.Default()
	ch := make(chan adapters.DocumentMeta)
	go func() {
		defer close(ch)
		url := c.scanURL()
		for url != "" {
			resp, err := c.doRequest(ctx, url)
			if err != nil {
				log.Warn("confluence HTTP request failed", "url", url, "error", err)
				return
			}
			if resp.StatusCode >= 400 {
				log.Warn("confluence HTTP error", "url", url, "status", resp.StatusCode)
				resp.Body.Close()
				return
			}
			body, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			if readErr != nil {
				log.Warn("confluence: failed to read response body", "url", url, "error", readErr)
				return
			}

			var pr pagesMetaResponse
			if c.pageID != "" {
				var single pageMeta
				if err := json.Unmarshal(body, &single); err != nil {
					log.Warn("failed to parse single page meta", "page_id", c.pageID, "error", err)
					return
				}
				pr.Results = []pageMeta{single}
			} else {
				if err := json.Unmarshal(body, &pr); err != nil {
					log.Warn("failed to parse pages meta response", "url", url, "error", err)
					return
				}
			}

			for _, page := range pr.Results {
				ts := page.versionTS()
				log.Debug("scanned confluence page", "id", page.ID, "title", page.Title)
				meta := adapters.DocumentMeta{
					ID:          fmt.Sprintf("confluence://%s/%s", c.space, page.ID),
					Title:       page.Title,
					ContentHash: store.ContentHash(page.ID + ":" + ts),
					SourceType:  "confluence",
					Metadata: map[string]string{
						"url":        c.pageWebURL(page.Links.WebUI),
						"space":      c.space,
						"page_id":    page.ID,
						"updated_at": ts,
					},
					IngestedAt: time.Now().UTC(),
				}
				select {
				case ch <- meta:
				case <-ctx.Done():
					return
				}
			}

			if pr.Links.Next != "" && c.pageID == "" {
				url = c.cfg.BaseURL + pr.Links.Next
			} else {
				url = ""
			}
		}
	}()
	return ch, nil
}

// Load fetches the full page body and returns a Document with stripped HTML content.
// This is the expensive operation — only called when the hash has changed.
func (c *confluenceSource) Load(ctx context.Context, meta adapters.DocumentMeta) (adapters.Document, error) {
	log := slog.Default()
	// Extract page ID from the document ID: "confluence://SPACE/PAGEID"
	const idPrefix = "confluence://"
	tail := strings.TrimPrefix(meta.ID, idPrefix)
	slashIdx := strings.LastIndex(tail, "/")
	if slashIdx < 0 || slashIdx == len(tail)-1 {
		return adapters.Document{}, fmt.Errorf("malformed confluence document ID: %q", meta.ID)
	}
	pageID := tail[slashIdx+1:]

	url := c.loadURL(pageID)
	resp, err := c.doRequest(ctx, url)
	if err != nil {
		return adapters.Document{}, fmt.Errorf("fetch page %s: %w", pageID, err)
	}
	if resp.StatusCode >= 400 {
		resp.Body.Close()
		return adapters.Document{}, fmt.Errorf("fetch page %s: HTTP %d", pageID, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return adapters.Document{}, fmt.Errorf("read body for page %s: %w", pageID, err)
	}

	var page pageBody
	if err := json.Unmarshal(body, &page); err != nil {
		return adapters.Document{}, fmt.Errorf("parse page %s: %w", pageID, err)
	}

	content := stripHTML(page.Body.Storage.Value)
	log.Debug("loaded confluence page", "id", meta.ID, "content_len", len(content))

	return adapters.Document{
		DocumentMeta: meta,
		Content:      content,
	}, nil
}
