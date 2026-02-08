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

		// Use form values for POST body
		form := url.Values{}
		form.Add("action", "query")
		form.Add("prop", "info")
		form.Add("titles", strings.Join(batch, "|"))
		form.Add("format", "json")
		form.Add("redirects", "1")

		headers := map[string]string{
			"Content-Type": "application/x-www-form-urlencoded",
		}

		body, err := c.request.PostWithHeaders(ctx, u.String(), []byte(form.Encode()), headers)
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
		}

		// Map redirects to ensure original titles point to the length too
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

// GetArticleHTML fetches the parsed HTML content for a single article.
func (c *Client) GetArticleHTML(ctx context.Context, title, lang string) (string, error) {
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
	q.Add("action", "parse")
	q.Add("prop", "text")
	q.Add("page", title)
	q.Add("format", "json")
	q.Add("redirects", "1")
	q.Add("disableeditsection", "1")
	u.RawQuery = q.Encode()

	body, err := c.request.Get(ctx, u.String(), "")
	if err != nil {
		return "", err
	}

	var apiResp struct {
		Parse struct {
			Text struct {
				Html string `json:"*"`
			} `json:"text"`
		} `json:"parse"`
		Error struct {
			Code string `json:"code"`
			Info string `json:"info"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", fmt.Errorf("failed to decode json: %w", err)
	}

	if apiResp.Error.Code != "" {
		return "", fmt.Errorf("wikipedia api error: %s - %s", apiResp.Error.Code, apiResp.Error.Info)
	}

	return apiResp.Parse.Text.Html, nil
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

// GetImageFilenames fetches a list of all images associated with the article.
// Returns a slice of filenames (e.g., "File:Example.jpg").
func (c *Client) GetImageFilenames(ctx context.Context, title, lang string) ([]string, error) {
	if lang == "" {
		lang = "en"
	}
	endpoint := c.APIEndpoint
	if endpoint == "" {
		endpoint = fmt.Sprintf("https://%s.wikipedia.org/w/api.php", lang)
	}

	u, _ := url.Parse(endpoint)
	q := u.Query()
	q.Add("action", "query")
	q.Add("prop", "images")
	q.Add("imlimit", "50") // Fetch enough candidates
	q.Add("titles", title)
	q.Add("format", "json")
	q.Add("redirects", "1")
	u.RawQuery = q.Encode()

	body, err := c.request.Get(ctx, u.String(), "")
	if err != nil {
		return nil, err
	}

	var imgResp struct {
		Query struct {
			Pages map[string]struct {
				Images []struct {
					Title string `json:"title"`
				} `json:"images"`
			} `json:"pages"`
		} `json:"query"`
	}
	if err := json.Unmarshal(body, &imgResp); err != nil {
		return nil, fmt.Errorf("failed to decode images json: %w", err)
	}

	var filenames []string
	for _, page := range imgResp.Query.Pages {
		for _, img := range page.Images {
			filenames = append(filenames, img.Title)
		}
	}
	return filenames, nil
}

// IsUnwantedImage checks if a filename or URL represents a vector graphic, icon, map, or other unwanted type.
func IsUnwantedImage(name string) bool {
	lower := strings.ToLower(name)

	// 1. Unwanted Extensions
	badExtensions := []string{".svg", ".svg.png", ".gif", ".tif", ".ogv", ".webm"}
	for _, ext := range badExtensions {
		if strings.HasSuffix(lower, ext) || strings.Contains(lower, ext) {
			return true
		}
	}

	return false
}

// ImageResult holds a filename and its resolved URL.
type ImageResult struct {
	Title string
	URL   string
}

// GetImagesWithURLs fetches all images for an article along with their URLs.
// It uses a generator query to efficiently fetch image info in batch.
func (c *Client) GetImagesWithURLs(ctx context.Context, title, lang string) ([]ImageResult, error) {
	if lang == "" {
		lang = "en"
	}
	endpoint := c.APIEndpoint
	if endpoint == "" {
		endpoint = fmt.Sprintf("https://%s.wikipedia.org/w/api.php", lang)
	}

	u, _ := url.Parse(endpoint)
	q := u.Query()
	q.Add("action", "query")
	q.Add("generator", "images")
	q.Add("titles", title)
	q.Add("gimlimit", "50") // Fetch up to 50 images linked on the page
	q.Add("prop", "imageinfo")
	q.Add("iiprop", "url")
	q.Add("format", "json")
	q.Add("redirects", "1")
	u.RawQuery = q.Encode()

	body, err := c.request.Get(ctx, u.String(), "")
	if err != nil {
		return nil, err
	}

	var apiResp struct {
		Query struct {
			Pages map[string]struct {
				Title     string `json:"title"`
				ImageInfo []struct {
					URL string `json:"url"`
				} `json:"imageinfo"`
			} `json:"pages"`
		} `json:"query"`
	}

	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode json: %w", err)
	}

	var results []ImageResult
	for _, page := range apiResp.Query.Pages {
		// page.Title is "File:..."
		if IsUnwantedImage(page.Title) {
			continue
		}

		url := ""
		if len(page.ImageInfo) > 0 {
			url = page.ImageInfo[0].URL
		}

		if url != "" {
			results = append(results, ImageResult{
				Title: page.Title,
				URL:   url,
			})
		}
	}

	return results, nil
}
