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
					Width  int    `json:"width"`
					Height int    `json:"height"`
				} `json:"thumbnail"`
			} `json:"pages"`
		} `json:"query"`
	}
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", fmt.Errorf("failed to decode json: %w", err)
	}

	for _, page := range apiResp.Query.Pages {
		if page.Thumbnail.Source != "" {
			// 1. Check filename/extension patterns
			if isUnwantedImage(page.Thumbnail.Source) {
				continue
			}
			// 2. Check Aspect Ratio (Reject vertical portraits if Height > 1.2 * Width)
			// Landmarks are rarely vertical portraits (except towers, but usually they are 1:2 not 3:4 or taller like portraits?)
			// Actually 1.2 is reasonable for a portrait photo. A tower might be taller.
			// However, most "Portrait of a Mayor" images are vertical.
			// Let's stick to the plan: Height > Width * 1.3 to be safe against towers.
			if page.Thumbnail.Width > 0 && float64(page.Thumbnail.Height) > float64(page.Thumbnail.Width)*1.3 {
				continue
			}

			return page.Thumbnail.Source, nil
		}
	}

	// Fallback: Get first content image that isn't unwanted
	return c.getFirstContentImage(ctx, title, lang, endpoint)
}

// getFirstContentImage fetches the first non-SVG image from the article's content.
func (c *Client) getFirstContentImage(ctx context.Context, title, lang, endpoint string) (string, error) {
	// Query for images list
	u, _ := url.Parse(endpoint)
	q := u.Query()
	q.Add("action", "query")
	q.Add("prop", "images")
	q.Add("imlimit", "15") // Increased limit to find a good one
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

	// Find first valid image
	var imageTitle string
	for _, page := range imgResp.Query.Pages {
		for _, img := range page.Images {
			if isUnwantedImage(img.Title) {
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
	q.Add("iiprop", "url|size") // Request size too
	q.Add("iiurlwidth", "800")  // Request thumbnail at 800px width
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
					Width    int    `json:"width"`
					Height   int    `json:"height"`
				} `json:"imageinfo"`
			} `json:"pages"`
		} `json:"query"`
	}
	if err := json.Unmarshal(body, &infoResp); err != nil {
		return "", fmt.Errorf("failed to decode imageinfo json: %w", err)
	}

	for _, page := range infoResp.Query.Pages {
		if len(page.ImageInfo) > 0 {
			info := page.ImageInfo[0]
			// Check aspect ratio regarding original dimensions
			if info.Width > 0 && float64(info.Height) > float64(info.Width)*1.3 {
				return "", nil // Reject portrait aspect ratio
			}

			// Prefer thumbnail URL if available
			if info.ThumbURL != "" {
				return info.ThumbURL, nil
			}
			return info.URL, nil
		}
	}

	return "", nil
}

// isUnwantedImage checks if a filename or URL represents a vector graphic, icon, map, or other unwanted type.
func isUnwantedImage(name string) bool {
	lower := strings.ToLower(name)

	// 1. Unwanted Extensions
	badExtensions := []string{".svg", ".svg.png", ".gif", ".tif", ".ogv", ".webm"}
	for _, ext := range badExtensions {
		if strings.HasSuffix(lower, ext) || strings.Contains(lower, ext) {
			return true
		}
	}

	// 2. Unwanted Keywords in Filename
	badKeywords := []string{
		"logo", "icon", "flag", "coat of arms", "wappen", "insignia",
		"map", "locator", "plan", "diagram", "chart", "graph",
		"stub", "placeholder", "missing",
		"collage", "montage", // often composite images that look bad as thumb
		"signature", "restored", // historically restored documents?
	}
	for _, kw := range badKeywords {
		// Check for word boundaries roughly (simple contains is safer for now)
		// e.g. "Flag_of_..."
		if strings.Contains(lower, kw) {

			// Exception: "map" might trigger on "maple" (rare but possible).
			// But usually filenames are "Map_of_X".
			// Let's be slightly more specific for "map" if needed, but "locator" covers many maps.
			if kw == "map" && strings.Contains(lower, "maple") {
				continue
			}
			return true
		}
	}

	return false
}
