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
// Falls back to the first non-vector content image if no page image is designated.
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

	// First try: pageimages (designated page image)
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
			// Skip if page thumbnail is a vector graphic
			if !isVectorGraphic(page.Thumbnail.Source) {
				return page.Thumbnail.Source, nil
			}
		}
	}

	// Fallback: Get first content image that isn't a vector graphic
	return c.getFirstContentImage(ctx, title, lang, endpoint)
}

// getFirstContentImage fetches the first non-SVG image from the article's content.
func (c *Client) getFirstContentImage(ctx context.Context, title, lang, endpoint string) (string, error) {
	// Query for images list
	u, _ := url.Parse(endpoint)
	q := u.Query()
	q.Add("action", "query")
	q.Add("prop", "images")
	q.Add("imlimit", "10") // Limit to first 10 images
	q.Add("titles", title)
	q.Add("format", "json")
	q.Add("redirects", "1")
	u.RawQuery = q.Encode()

	body, err := c.request.Get(ctx, u.String(), "")
	if err != nil {
		return "", err
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
		return "", fmt.Errorf("failed to decode images json: %w", err)
	}

	// Find first non-SVG image
	var imageTitle string
	for _, page := range imgResp.Query.Pages {
		for _, img := range page.Images {
			// Skip vector graphics and icons
			if isVectorGraphic(img.Title) {
				continue
			}
			imageTitle = img.Title
			break
		}
		if imageTitle != "" {
			break
		}
	}

	if imageTitle == "" {
		return "", nil // No suitable image found
	}

	// Get image URL via imageinfo
	return c.getImageURL(ctx, imageTitle, lang, endpoint)
}

// getImageURL fetches the URL for a specific image file from Wikipedia.
func (c *Client) getImageURL(ctx context.Context, imageTitle, lang, endpoint string) (string, error) {
	u, _ := url.Parse(endpoint)
	q := u.Query()
	q.Add("action", "query")
	q.Add("prop", "imageinfo")
	q.Add("iiprop", "url")
	q.Add("iiurlwidth", "800") // Request thumbnail at 800px width
	q.Add("titles", imageTitle)
	q.Add("format", "json")
	u.RawQuery = q.Encode()

	body, err := c.request.Get(ctx, u.String(), "")
	if err != nil {
		return "", err
	}

	var infoResp struct {
		Query struct {
			Pages map[string]struct {
				ImageInfo []struct {
					ThumbURL string `json:"thumburl"`
					URL      string `json:"url"`
				} `json:"imageinfo"`
			} `json:"pages"`
		} `json:"query"`
	}
	if err := json.Unmarshal(body, &infoResp); err != nil {
		return "", fmt.Errorf("failed to decode imageinfo json: %w", err)
	}

	for _, page := range infoResp.Query.Pages {
		if len(page.ImageInfo) > 0 {
			// Prefer thumbnail URL if available
			if page.ImageInfo[0].ThumbURL != "" {
				return page.ImageInfo[0].ThumbURL, nil
			}
			return page.ImageInfo[0].URL, nil
		}
	}

	return "", nil
}

// isVectorGraphic checks if a filename or URL represents a vector graphic, icon, or map.
func isVectorGraphic(name string) bool {
	lower := strings.ToLower(name)
	// Common vector graphic patterns
	vectorPatterns := []string{".svg", ".svg.png", ".gif"}
	for _, pattern := range vectorPatterns {
		if strings.HasSuffix(lower, pattern) || strings.Contains(lower, pattern) {
			return true
		}
	}
	// Skip map images (e.g., "foo_map.png" or "foo_map_of_bar.png")
	if strings.Contains(lower, "_map.") || strings.Contains(lower, "_map_") {
		return true
	}
	return false
}
