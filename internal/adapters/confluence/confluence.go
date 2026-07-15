package confluence

import (
	"context"
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
// pageID is optional; if set, only that page is fetched.
func New(cfg config.ConfluenceConfig, space, pageID string) adapters.Source {
	return &confluenceSource{cfg: cfg, space: space, pageID: pageID, client: &http.Client{Timeout: 30 * time.Second}}
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

type pageResult struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Body  struct {
		Storage struct {
			Value string `json:"value"`
		} `json:"storage"`
	} `json:"body"`
	Version struct {
		CreatedAt string `json:"createdAt"`
	} `json:"version"`
	Links struct {
		WebUI string `json:"webui"`
	} `json:"_links"`
}

type pagesResponse struct {
	Results []pageResult `json:"results"`
	Links   struct {
		Next string `json:"next"`
	} `json:"_links"`
}

func (c *confluenceSource) Documents(ctx context.Context) (<-chan adapters.Document, error) {
	log := slog.Default()
	ch := make(chan adapters.Document)
	go func() {
		defer close(ch)
		url := fmt.Sprintf("%s/wiki/api/v2/spaces/%s/pages?body-format=storage&limit=50", c.cfg.BaseURL, c.space)
		if c.pageID != "" {
			url = fmt.Sprintf("%s/wiki/api/v2/pages/%s?body-format=storage", c.cfg.BaseURL, c.pageID)
		}
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
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			var pr pagesResponse
			if c.pageID != "" {
				var single pageResult
				if err := json.Unmarshal(body, &single); err != nil {
					log.Warn("failed to parse single page response", "page_id", c.pageID, "error", err)
					return
				}
				pr.Results = []pageResult{single}
			} else {
				if err := json.Unmarshal(body, &pr); err != nil {
					log.Warn("failed to parse pages response", "url", url, "error", err)
					return
				}
			}

			for _, page := range pr.Results {
				log.Debug("fetched confluence page", "id", page.ID, "title", page.Title)
				content := stripHTML(page.Body.Storage.Value)
				doc := adapters.Document{
					ID:          fmt.Sprintf("confluence://%s/%s", c.space, page.ID),
					Title:       page.Title,
					Content:     content,
					ContentHash: store.ContentHash(content),
					SourceType:  "confluence",
					Metadata: map[string]string{
						"url":        c.cfg.BaseURL + "/wiki" + page.Links.WebUI,
						"space":      c.space,
						"page_id":    page.ID,
						"updated_at": page.Version.CreatedAt,
					},
					IngestedAt: time.Now().UTC(),
				}
				select {
				case ch <- doc:
				case <-ctx.Done():
					return
				}
			}

			// pagination
			if pr.Links.Next != "" && c.pageID == "" {
				url = c.cfg.BaseURL + pr.Links.Next
			} else {
				url = ""
			}
		}
	}()
	return ch, nil
}
