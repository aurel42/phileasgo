package wikidata

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"phileasgo/pkg/logging"
	"phileasgo/pkg/request"
)

const (
	sparqlEndpoint = "https://query.wikidata.org/sparql"
	apiEndpoint    = "https://www.wikidata.org/w/api.php"
)

// ClientInterface defines the interface for Wikidata interactions.
type ClientInterface interface {
	QuerySPARQL(ctx context.Context, query, cacheKey string, radiusM int, lat, lon float64) ([]Article, string, error)
	QueryEntities(ctx context.Context, ids []string) ([]Article, string, error)
	GetEntitiesBatch(ctx context.Context, ids []string) (map[string]EntityMetadata, error)
	FetchFallbackData(ctx context.Context, ids, allowedSites []string) (map[string]FallbackData, error)
	GetEntityClaims(ctx context.Context, id, property string) (targets []string, label string, err error)
}

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
// radiusM is the query radius in meters for geodata caching.
func (c *Client) QuerySPARQL(ctx context.Context, query, cacheKey string, radiusM int, lat, lon float64) ([]Article, string, error) {

	// Use POST to avoid URL length limits
	data := url.Values{}
	data.Set("query", query)
	data.Set("format", "json")
	encodedData := data.Encode()

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
		"Accept":       "application/sparql-results+json",
	}

	logging.Trace(c.Logger, "Executing SPARQL Query", "query", query)
	start := time.Now()

	// Use geodata cache (routes to cache_geodata table with radius metadata)
	body, err := c.request.PostWithGeodataCache(ctx, c.SPARQLEndpoint, []byte(encodedData), headers, cacheKey, radiusM, lat, lon)

	duration := time.Since(start)
	logging.Trace(c.Logger, "SPARQL Query Completed", "duration", duration, "cached", err == nil && len(body) > 0)

	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrNetwork, err)
	}

	// Parse Response (Zero-Alloc Streaming)
	articles, _, err := ParseSPARQLStreaming(strings.NewReader(string(body)))
	return articles, string(body), err
}

// QueryEntities fetches specific entities by their QIDs using the same unified schema as tile queries.
func (c *Client) QueryEntities(ctx context.Context, ids []string) ([]Article, string, error) {
	if len(ids) == 0 {
		return nil, "", nil
	}

	var builders []string
	for _, id := range ids {
		builders = append(builders, "wd:"+id)
	}
	valuesClause := strings.Join(builders, " ")

	query := fmt.Sprintf(`SELECT DISTINCT ?item ?lat ?lon ?sitelinks 
            (GROUP_CONCAT(DISTINCT ?instance_of_uri; separator=",") AS ?instances) 
            ?area ?height ?length ?width
        WHERE { 
            VALUES ?item { %s }
            ?item p:P625/psv:P625 [ wikibase:geoLatitude ?lat ; wikibase:geoLongitude ?lon ] . 
            
            OPTIONAL { ?item wdt:P31 ?instance_of_uri . } 
            OPTIONAL { ?item wikibase:sitelinks ?sitelinks . } 
            OPTIONAL { ?item wdt:P2046 ?area . }
            OPTIONAL { ?item wdt:P2048 ?height . }
            OPTIONAL { ?item wdt:P2043 ?length . }
            OPTIONAL { ?item wdt:P2049 ?width . }
            
            FILTER(?sitelinks > 0)
        } 
        GROUP BY ?item ?lat ?lon ?sitelinks ?area ?height ?length ?width`, valuesClause)

	// Since we are querying specific entities, we use a dedicated cache prefix
	sort.Strings(ids)
	hash := md5.Sum([]byte(strings.Join(ids, ",")))
	cacheKey := fmt.Sprintf("wd_entities_%s", hex.EncodeToString(hash[:]))

	return c.QuerySPARQL(ctx, query, cacheKey, 0, 0, 0)
}

// ParseSPARQLStreaming iterates over the JSON stream to extract bindings without loading the full structure.
// It is exported for use by debugging tools.
func ParseSPARQLStreaming(r io.Reader) ([]Article, string, error) {
	dec := json.NewDecoder(r)

	// We only care about matching "bindings" -> [ ... ]
	// Structure: { ..., "results": { "bindings": [ ... ] } }

	// Fast-forward to "bindings"
	foundBindings := false
	for {
		t, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, "", fmt.Errorf("json stream error: %w", err)
		}

		if s, ok := t.(string); ok && s == "bindings" {
			foundBindings = true
			break
		}
	}

	if !foundBindings {
		// Valid SPARQL response might have empty results, but "bindings" key usually exists.
		// If not found, return empty.
		return []Article{}, "", nil
	}

	// Consume open bracket '['
	if _, err := dec.Token(); err != nil {
		return nil, "", fmt.Errorf("expected array open: %w", err)
	}

	// Stream bindings
	var articles []Article
	seen := make(map[string]bool)

	for dec.More() {
		var b map[string]sparqlValue
		if err := dec.Decode(&b); err != nil {
			return nil, "", fmt.Errorf("failed to decode binding: %w", err)
		}

		// --- Core Parsing Logic (Identical to parseBindings) ---
		lat, _ := strconv.ParseFloat(val(b, "lat"), 64)
		lon, _ := strconv.ParseFloat(val(b, "lon"), 64)

		itemURI := val(b, "item")
		qid := ""
		if idx := strings.LastIndex(itemURI, "/"); idx != -1 && idx < len(itemURI)-1 {
			qid = itemURI[idx+1:]
		} else {
			qid = itemURI
		}

		if qid == "" || seen[qid] {
			continue
		}
		seen[qid] = true

		sitelinks, _ := strconv.Atoi(val(b, "sitelinks"))

		articles = append(articles, Article{
			QID:         qid,
			LocalTitles: parseLocalTitles(val(b, "local_titles")),
			TitleEn:     val(b, "title_en_val"),
			TitleUser:   val(b, "title_user_val"),
			Label:       val(b, "itemLabel"),
			Lat:         lat,
			Lon:         lon,
			Sitelinks:   sitelinks,
			Instances:   parseInstances(val(b, "instances")),
			Area:        parseFloatPtr(val(b, "area")),
			Height:      parseFloatPtr(val(b, "height")),
			Length:      parseFloatPtr(val(b, "length")),
			Width:       parseFloatPtr(val(b, "width")),
		})
	}

	// We don't keep the raw body string anymore for memory reasons, but interface demands it.
	// Returning empty string for raw body to save RAM.
	return articles, "", nil
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
			return nil, nil, fmt.Errorf("%w: %v", ErrNetwork, errReq)
		}

		var result wrapperEntityResponse
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, nil, fmt.Errorf("%w: failed to decode json: %v", ErrParse, err)
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

// FetchFallbackData gets labels and sitelinks for a batch of IDs, optionally filtered by site.
func (c *Client) FetchFallbackData(ctx context.Context, ids, allowedSites []string) (map[string]FallbackData, error) {
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

		// Apply site filter if provided
		if len(allowedSites) > 0 {
			q.Add("sitefilter", strings.Join(allowedSites, "|"))
		}

		// No languages param = fetch all labels (unless we want to limit labels too? Labels are cheap)
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

type sparqlValue struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

func parseInstances(instStr string) []string {
	if instStr == "" {
		return nil
	}
	// Optimization: strings.Split allocates a slice.
	// Since we know the count approx, we could preallocate, or just loop.
	// Low-hanging fruit: The QID extraction inner loop was the big one.
	// Let's use strings.Split for comma (frequent but flat),
	// but strictly optimize the URI splitting for each instance.

	parts := strings.Split(instStr, ",")
	instances := make([]string, 0, len(parts))

	for _, uri := range parts {
		if idx := strings.LastIndex(uri, "/"); idx != -1 && idx < len(uri)-1 {
			instances = append(instances, uri[idx+1:])
		} else {
			instances = append(instances, uri)
		}
	}
	return instances
}

func parseLocalTitles(localTitlesStr string) map[string]string {
	localTitles := make(map[string]string)
	if localTitlesStr == "" {
		return localTitles
	}
	for _, pair := range strings.Split(localTitlesStr, "|") {
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) != 2 {
			continue
		}
		t := parts[1]
		// Reject namespace prefixes
		if strings.HasPrefix(t, "Category:") || strings.HasPrefix(t, "File:") ||
			strings.HasPrefix(t, "Template:") || strings.HasPrefix(t, "User:") ||
			strings.HasPrefix(t, "Talk:") {
			continue
		}
		localTitles[parts[0]] = t
	}
	return localTitles
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
