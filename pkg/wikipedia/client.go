package wikipedia

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"phileasgo/pkg/request"
)

// Client handles Wikipedia API interactions.
type Client struct {
	request     *request.Client
	APIEndpoint string // Optional override for testing
}

// NewClient creates a new Wikipedia client.
func NewClient(r *request.Client) *Client {
	return &Client{request: r}
}

// GetArticleLengths fetches the length (in bytes) of multiple articles in a specific language.
// Returns a map of Title -> Length.
func (c *Client) GetArticleLengths(ctx context.Context, titles []string, lang string) (map[string]int, error) {
	if len(titles) == 0 {
		return make(map[string]int), nil
	}
	if lang == "" {
		lang = "en"
	}

	var endpoint string
	if c.APIEndpoint != "" {
		endpoint = c.APIEndpoint
	} else {
		endpoint = fmt.Sprintf("https://%s.wikipedia.org/w/api.php", lang)
	}
	u, _ := url.Parse(endpoint)

	// API limits: 50 titles per request typically
	const batchSize = 50
	result := make(map[string]int)

	for i := 0; i < len(titles); i += batchSize {
		end := i + batchSize
		if end > len(titles) {
			end = len(titles)
		}
		batch := titles[i:end]

		q := u.Query()
		q.Add("action", "query")
		q.Add("prop", "info")
		q.Add("titles", strings.Join(batch, "|"))
		q.Add("format", "json")
		q.Add("redirects", "1") // Follow redirects
		u.RawQuery = q.Encode()

		body, err := c.request.Get(ctx, u.String(), "")
		if err != nil {
			// Log warning and continue? Or fail?
			// For enrichment, partial failure is acceptable but here we return error.
			return nil, err
		}

		var apiResp response
		if err := json.Unmarshal(body, &apiResp); err != nil {
			return nil, fmt.Errorf("failed to decode json: %w", err)
		}

		for _, page := range apiResp.Query.Pages {
			// page.Title is the normalized title (after redirects)
			// We prioritize mapping the normalized title.
			result[page.Title] = page.Length

			// If it was a redirect, the original title might be lost in this mapping if we only use page.Title.
			// The caller might expect inputs map to outputs.
			// `redirects=1` provides a <redirects> list in response mapping "from" -> "to".
			// We should technically handle this to map original requested titles to lengths.
			// BUT for our use case (scoring), we likely use the normalized title for display anyway.

			// IMPORTANT: If page is missing (invalid title), "missing" field is present. Length is 0 or undefined.
			// struct parsing will give 0 for Length if missing.
		}

		// Map redirects to ensure original titles point to the length too?
		for _, r := range apiResp.Query.Redirects {
			if length, ok := result[r.To]; ok {
				result[r.From] = length
			}
		}
	}

	return result, nil
}

// GetArticleContent fetches the extract text for a single article.
func (c *Client) GetArticleContent(ctx context.Context, title, lang string) (string, error) {
	if lang == "" {
		lang = "en"
	}

	var endpoint string
	if c.APIEndpoint != "" {
		endpoint = c.APIEndpoint
	} else {
		endpoint = fmt.Sprintf("https://%s.wikipedia.org/w/api.php", lang)
	}

	u, _ := url.Parse(endpoint)
	q := u.Query()
	q.Add("action", "query")
	q.Add("prop", "extracts")
	q.Add("explaintext", "1")
	q.Add("titles", title)
	q.Add("format", "json")
	q.Add("redirects", "1")
	u.RawQuery = q.Encode()

	body, err := c.request.Get(ctx, u.String(), "")
	if err != nil {
		return "", err
	}

	var apiResp struct {
		Query struct {
			Pages map[string]struct {
				Extract string `json:"extract"`
			} `json:"pages"`
		} `json:"query"`
	}
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", fmt.Errorf("failed to decode json: %w", err)
	}

	for _, page := range apiResp.Query.Pages {
		return page.Extract, nil
	}

	return "", fmt.Errorf("article not found: %s", title)
}

type response struct {
	Query struct {
		Pages map[string]struct {
			PageID  int    `json:"pageid"`
			Title   string `json:"title"`
			Length  int    `json:"length"`
			Missing string `json:"missing,omitempty"`
		} `json:"pages"`
		Redirects []struct {
			From string `json:"from"`
			To   string `json:"to"`
		} `json:"redirects"`
	} `json:"query"`
}

// GetThumbnail fetches the thumbnail URL for a Wikipedia article.
func (c *Client) GetThumbnail(ctx context.Context, title, lang string) (string, error) {
	if lang == "" {
		lang = "en"
	}

	var endpoint string
	if c.APIEndpoint != "" {
		endpoint = c.APIEndpoint
	} else {
		endpoint = fmt.Sprintf("https://%s.wikipedia.org/w/api.php", lang)
	}

	u, _ := url.Parse(endpoint)
	q := u.Query()
	q.Add("action", "query")
	q.Add("prop", "pageimages")
	q.Add("piprop", "thumbnail")
	q.Add("pithumbsize", "800")
	q.Add("titles", title)
	q.Add("format", "json")
	q.Add("redirects", "1")
	u.RawQuery = q.Encode()

	body, err := c.request.Get(ctx, u.String(), "")
	if err != nil {
		return "", err
	}

	var apiResp struct {
		Query struct {
			Pages map[string]struct {
				Thumbnail struct {
					Source string `json:"source"`
				} `json:"thumbnail"`
			} `json:"pages"`
		} `json:"query"`
	}
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", fmt.Errorf("failed to decode json: %w", err)
	}

	for _, page := range apiResp.Query.Pages {
		if page.Thumbnail.Source != "" {
			return page.Thumbnail.Source, nil
		}
	}

	return "", nil // No thumbnail available
}
