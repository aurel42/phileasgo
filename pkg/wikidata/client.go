package wikidata

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"phileasgo/pkg/request"
)

const (
	sparqlEndpoint = "https://query.wikidata.org/sparql"
	apiEndpoint    = "https://www.wikidata.org/w/api.php"
)

// Client handles SPARQL queries.
type Client struct {
	request        *request.Client
	APIEndpoint    string
	SPARQLEndpoint string
	Logger         *slog.Logger
}

// NewClient creates a new Wikidata client.
func NewClient(r *request.Client, logger *slog.Logger) *Client {
	return &Client{
		request:        r,
		APIEndpoint:    apiEndpoint,
		SPARQLEndpoint: sparqlEndpoint,
		Logger:         logger,
	}
}

// QuerySPARQL executes a SPARQL query and parses the result into Articles.
// It returns the list of articles found and the raw JSON response.
func (c *Client) QuerySPARQL(ctx context.Context, query, cacheKey string) ([]Article, string, error) {
	u, err := url.Parse(c.SPARQLEndpoint)
	if err != nil {
		return nil, "", err
	}

	q := u.Query()
	q.Add("query", query)
	q.Add("format", "json")
	u.RawQuery = q.Encode()

	headers := map[string]string{
		"Accept": "application/sparql-results+json",
	}

	body, err := c.request.GetWithHeaders(ctx, u.String(), headers, cacheKey)
	if err != nil {
		return nil, "", err
	}

	// Parse Response
	var result sparqlResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, "", fmt.Errorf("failed to decode json: %w", err)
	}

	return parseBindings(result), string(body), nil
}

// GetEntityClaims fetches specific property claims (e.g. P31, P279) for an entity.
// It returns a list of target QIDs and the English label of the entity.
func (c *Client) GetEntityClaims(ctx context.Context, id, property string) (targets []string, label string, err error) {
	u, _ := url.Parse(c.APIEndpoint)

	q := u.Query()
	q.Add("action", "wbgetentities")
	q.Add("format", "json")
	q.Add("ids", id)
	q.Add("props", "claims|labels")
	q.Add("languages", "en")
	u.RawQuery = q.Encode()

	// Entity claims are currently uncached in requester (cacheKey = "")
	body, err := c.request.Get(ctx, u.String(), "")
	if err != nil {
		return nil, "", err
	}

	// Parse
	var result wrapperEntityResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, "", fmt.Errorf("failed to decode json: %w", err)
	}

	ent, ok := result.Entities[id]
	if !ok {
		return nil, "", fmt.Errorf("entity %s not found in response", id)
	}

	// Extract Label
	label = ""
	if lbl, ok := ent.Labels["en"]; ok {
		label = lbl.Value
	}

	// Extract Claims
	// targets is already defined as return value
	claims, ok := ent.Claims[property]
	if ok {
		for _, claim := range claims {
			if datavalue, ok := claim.Mainsnak["datavalue"].(map[string]interface{}); ok {
				if val, ok := datavalue["value"].(map[string]interface{}); ok {
					if idVal, ok := val["id"].(string); ok {
						targets = append(targets, idVal)
					}
				}
			}
		}
	}

	return targets, label, nil
}

// GetEntityClaimsBatch fetches specific property claims for multiple entities in chunks.
// It returns a map of ID -> List of Target IDs, and a map of ID -> Label.
func (c *Client) GetEntityClaimsBatch(ctx context.Context, ids []string, property string) (claims map[string][]string, labels map[string]string, err error) {
	claims = make(map[string][]string)
	labels = make(map[string]string)

	// Wikidata allows max 50 IDs per request
	const batchSize = 50
	for i := 0; i < len(ids); i += batchSize {
		end := i + batchSize
		if end > len(ids) {
			end = len(ids)
		}
		chunk := ids[i:end]

		u, _ := url.Parse(c.APIEndpoint)
		q := u.Query()
		q.Add("action", "wbgetentities")
		q.Add("format", "json")
		q.Add("ids", strings.Join(chunk, "|"))
		q.Add("props", "claims|labels")
		q.Add("languages", "en")
		u.RawQuery = q.Encode()

		body, errReq := c.request.Get(ctx, u.String(), "")
		if errReq != nil {
			return nil, nil, errReq
		}

		var result wrapperEntityResponse
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, nil, fmt.Errorf("failed to decode json: %w", err)
		}

		for id, ent := range result.Entities {
			// Label
			if lbl, ok := ent.Labels["en"]; ok {
				labels[id] = lbl.Value
			}

			// Claims
			if propClaims, ok := ent.Claims[property]; ok {
				var targets []string
				for _, claim := range propClaims {
					if datavalue, ok := claim.Mainsnak["datavalue"].(map[string]interface{}); ok {
						if val, ok := datavalue["value"].(map[string]interface{}); ok {
							if idVal, ok := val["id"].(string); ok {
								targets = append(targets, idVal)
							}
						}
					}
				}
				claims[id] = targets
			}
		}
	}

	return claims, labels, nil
}

// GetEntitiesBatch fetches labels and specific claims for multiple entities in one request.
func (c *Client) GetEntitiesBatch(ctx context.Context, ids []string) (map[string]EntityMetadata, error) {
	if len(ids) == 0 {
		return make(map[string]EntityMetadata), nil
	}

	// Sort IDs to ensure consistent caching, as map iteration order is random.
	// Work on a copy to avoid side effects.
	sortedIDs := make([]string, len(ids))
	copy(sortedIDs, ids)
	sort.Strings(sortedIDs)

	resultMap := make(map[string]EntityMetadata)

	// Wikidata allows max 50 IDs per request
	const batchSize = 50
	for i := 0; i < len(sortedIDs); i += batchSize {
		end := i + batchSize
		if end > len(sortedIDs) {
			end = len(sortedIDs)
		}
		chunk := sortedIDs[i:end]
		idStr := strings.Join(chunk, "|")

		// Create stable cache key
		hash := md5.Sum([]byte(idStr))
		cacheKey := fmt.Sprintf("wd_batch_%s", hex.EncodeToString(hash[:]))

		u, _ := url.Parse(c.APIEndpoint)
		q := u.Query()
		q.Add("action", "wbgetentities")
		q.Add("format", "json")
		q.Add("ids", idStr)
		q.Add("props", "claims|labels")
		q.Add("languages", "en")
		u.RawQuery = q.Encode()

		// Request with cache key
		body, err := c.request.Get(ctx, u.String(), cacheKey)
		if err != nil {
			return nil, err
		}

		var result wrapperEntityResponse
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, fmt.Errorf("failed to decode json: %w", err)
		}

		for id, ent := range result.Entities {
			data := EntityMetadata{
				Labels: make(map[string]string),
				Claims: make(map[string][]string),
			}

			for lang, lbl := range ent.Labels {
				data.Labels[lang] = lbl.Value
			}

			for prop, claims := range ent.Claims {
				var targets []string
				for _, claim := range claims {
					if datavalue, ok := claim.Mainsnak["datavalue"].(map[string]interface{}); ok {
						if val, ok := datavalue["value"].(map[string]interface{}); ok {
							if idVal, ok := val["id"].(string); ok {
								targets = append(targets, idVal)
							}
						}
					}
				}
				data.Claims[prop] = targets
			}
			resultMap[id] = data
		}
	}

	return resultMap, nil
}

// FallbackData contains raw labels and sitelinks for rescue operations.
type FallbackData struct {
	Labels    map[string]string // Lang -> Value
	Sitelinks map[string]string // Site -> Title (e.g. "enwiki" -> "Title")
}

// FetchFallbackData gets all labels and sitelinks for a batch of IDs (no language filter).
func (c *Client) FetchFallbackData(ctx context.Context, ids []string) (map[string]FallbackData, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	resultMap := make(map[string]FallbackData)

	const batchSize = 50
	for i := 0; i < len(ids); i += batchSize {
		end := i + batchSize
		if end > len(ids) {
			end = len(ids)
		}
		chunk := ids[i:end]
		idStr := strings.Join(chunk, "|")

		u, _ := url.Parse(c.APIEndpoint)
		q := u.Query()
		q.Add("action", "wbgetentities")
		q.Add("format", "json")
		q.Add("ids", idStr)
		q.Add("props", "labels|sitelinks")
		// No languages param = fetch all
		u.RawQuery = q.Encode()

		// Do not cache fallback data as it's a rescue operation for "bad" cached data
		body, err := c.request.Get(ctx, u.String(), "")
		if err != nil {
			return nil, err
		}

		// We need a slightly richer wrapper to capture all sitelinks/labels
		// wrapperEntityResponse is defined strictly?
		// Let's redefine a local struct for this specific parsing if needed,
		// or check if wrapperEntityResponse is sufficient.
		// wrapperEntityResponse uses map[string]struct{Value string} for labels, which is fine.
		// It doesn't map Sitelinks. We need to add Sitelinks to the wrapper or use a new one.
		// Since wrapperEntityResponse is private and defined at the bottom, let's verify it first.
		// Wait, I can't verify it in this tool call.
		// I'll assume I need to extend the wrapperEntityResponse or create a compatible one.
		// To be safe, I'll define a local struct inside this method or just extend the file's struct if possible.
		// I cannot modify the struct definition easily here without a separate replacement.

		// Better approach: Unmarshal into a map[string]interface{} or a specific struct here.
		type extendedEntity struct {
			Labels map[string]struct {
				Value string `json:"value"`
			} `json:"labels"`
			Sitelinks map[string]struct {
				Site  string `json:"site"`
				Title string `json:"title"`
			} `json:"sitelinks"`
		}
		type extendedResponse struct {
			Entities map[string]extendedEntity `json:"entities"`
		}

		var extRes extendedResponse
		if err := json.Unmarshal(body, &extRes); err != nil {
			return nil, fmt.Errorf("failed to decode json: %w", err)
		}

		for id, ent := range extRes.Entities {
			fd := FallbackData{
				Labels:    make(map[string]string),
				Sitelinks: make(map[string]string),
			}
			for lang, lbl := range ent.Labels {
				fd.Labels[lang] = lbl.Value
			}
			for site, sl := range ent.Sitelinks {
				fd.Sitelinks[site] = sl.Title
			}
			resultMap[id] = fd
		}
	}

	return resultMap, nil
}

// SearchEntities searches for items in Wikidata by name/label.
func (c *Client) SearchEntities(ctx context.Context, query string) ([]SearchResult, error) {
	u, _ := url.Parse(c.APIEndpoint)
	q := u.Query()
	q.Add("action", "wbsearchentities")
	q.Add("search", query)
	q.Add("language", "en")
	q.Add("format", "json")
	q.Add("type", "item")
	q.Add("limit", "5")
	u.RawQuery = q.Encode()

	body, err := c.request.Get(ctx, u.String(), "")
	if err != nil {
		return nil, err
	}

	var result struct {
		Search []SearchResult `json:"search"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode json: %w", err)
	}

	return result.Search, nil
}

type SearchResult struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

// -- Internal parsing structs --

type wrapperEntityResponse struct {
	Entities map[string]struct {
		Labels map[string]struct {
			Value string `json:"value"`
		} `json:"labels"`
		Claims map[string][]struct {
			Mainsnak map[string]interface{} `json:"mainsnak"`
		} `json:"claims"`
	} `json:"entities"`
}

// -- Internal parsing structs --

type sparqlResponse struct {
	Results struct {
		Bindings []map[string]sparqlValue `json:"bindings"`
	} `json:"results"`
}

type sparqlValue struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

func parseBindings(resp sparqlResponse) []Article {
	var articles []Article

	for _, b := range resp.Results.Bindings {
		lat, _ := strconv.ParseFloat(val(b, "lat"), 64)
		lon, _ := strconv.ParseFloat(val(b, "lon"), 64)

		itemURI := val(b, "item")
		qid := ""
		if parts := strings.Split(itemURI, "/"); len(parts) > 0 {
			qid = parts[len(parts)-1]
		}

		if qid == "" {
			continue
		}

		sitelinks, _ := strconv.Atoi(val(b, "sitelinks"))

		// Optional Dimensions
		area := parseFloatPtr(val(b, "area"))
		height := parseFloatPtr(val(b, "height"))
		length := parseFloatPtr(val(b, "length"))
		width := parseFloatPtr(val(b, "width"))

		// Instances
		instStr := val(b, "instances")
		var instances []string
		if instStr != "" {
			for _, uri := range strings.Split(instStr, ",") {
				if parts := strings.Split(uri, "/"); len(parts) > 0 {
					instances = append(instances, parts[len(parts)-1])
				}
			}
		}

		// Parse Local Titles (lang:title|lang:title)
		localTitlesStr := val(b, "local_titles")
		localTitles := make(map[string]string)
		if localTitlesStr != "" {
			for _, pair := range strings.Split(localTitlesStr, "|") {
				// Each pair is "lang:title"
				parts := strings.SplitN(pair, ":", 2)
				if len(parts) == 2 {
					// Verify Title Namespace (Reject Category:, File:, etc or their localized equivalents if possible)
					// Simple heuristic for English/Universal namespaces
					t := parts[1]
					if strings.HasPrefix(t, "Category:") ||
						strings.HasPrefix(t, "File:") ||
						strings.HasPrefix(t, "Template:") ||
						strings.HasPrefix(t, "User:") ||
						strings.HasPrefix(t, "Talk:") {
						continue
					}
					localTitles[parts[0]] = parts[1]
				}
			}
		}

		articles = append(articles, Article{
			QID:         qid,
			LocalTitles: localTitles,
			TitleEn:     val(b, "title_en_val"),
			TitleUser:   val(b, "title_user_val"),
			Label:       val(b, "itemLabel"),
			Lat:         lat,
			Lon:         lon,
			Sitelinks:   sitelinks,
			Instances:   instances,
			Area:        area,
			Height:      height,
			Length:      length,
			Width:       width,
		})
	}
	return articles
}

func val(binding map[string]sparqlValue, key string) string {
	if v, ok := binding[key]; ok {
		return v.Value
	}
	return ""
}

func parseFloatPtr(s string) *float64 {
	if s == "" {
		return nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &f
}
