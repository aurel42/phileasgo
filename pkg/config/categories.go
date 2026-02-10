package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// CategoriesConfig holds category configuration loaded from JSON/YAML.
type CategoriesConfig struct {
	Categories        map[string]Category `json:"categories" yaml:"categories"`
	IgnoredCategories map[string]string   `json:"ignored_categories" yaml:"ignored_categories"`
	MergeDistance     map[string]float64  `json:"merge_distance" yaml:"merge_distance"`
	CategoryGroups    map[string][]string `json:"category_groups" yaml:"category_groups"`

	// Internal lookup for O(1) group checking
	GroupLookup map[string]string
}

// CategoryLookup maps QIDs to Category Names.
type CategoryLookup map[string]string

// Category holds specific configuration for a category.
type Category struct {
	Weight       float64           `json:"weight" yaml:"weight"`
	Size         string            `json:"size" yaml:"size"`
	Icon         string            `json:"icon" yaml:"icon"`
	IconArtistic string            `json:"icon_artistic" yaml:"icon_artistic"`
	SitelinksMin int               `json:"sitelinks_min" yaml:"sitelinks_min"`
	QIDs         map[string]string `json:"qids" yaml:"qids"`
	Preground    bool              `json:"preground" yaml:"preground"` // Enable Sonar pregrounding for this category
}

// BuildLookup creates a map of QID -> Category Name for fast lookups.
func (c *CategoriesConfig) BuildLookup() CategoryLookup {
	lookup := make(CategoryLookup)
	for catName, catData := range c.Categories {
		for qid := range catData.QIDs {
			lookup[qid] = catName
		}
	}
	return lookup
}

// LoadCategories loads the category configuration from a YAML/JSON file.
func LoadCategories(path string) (*CategoriesConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read categories file: %w", err)
	}

	var cfg CategoriesConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse categories file: %w", err)
	}

	// Normalize keys to lowercase for easier lookup if needed,
	// or we can handle case-insensitivity during lookup.
	// For now, we keep as is but note that lookups should be lowercased.
	// Actually, let's normalize the map keys to lowercase right here to avoid repeated ToLower later.
	normalizedCats := make(map[string]Category)
	for k, v := range cfg.Categories {
		normalizedCats[strings.ToLower(k)] = v
	}
	cfg.Categories = normalizedCats

	// Build Group Lookup
	cfg.GroupLookup = make(map[string]string)
	for groupName, cats := range cfg.CategoryGroups {
		for _, c := range cats {
			cfg.GroupLookup[strings.ToLower(c)] = groupName
		}
	}

	return &cfg, nil
}

// GetWeight returns the weight for a category (default 1.0).
func (c *CategoriesConfig) GetWeight(category string) float64 {
	if cat, ok := c.Categories[strings.ToLower(category)]; ok {
		if cat.Weight != 0 {
			return cat.Weight
		}
		return 1.0
	}
	return 1.0
}

// GetSize returns the size for a category (default "M").
func (c *CategoriesConfig) GetSize(category string) string {
	if cat, ok := c.Categories[strings.ToLower(category)]; ok {
		if cat.Size != "" {
			return cat.Size
		}
		return "M"
	}
	return "M"
}

// GetMergeDistance returns the merge distance in meters for a given size (default 500m).
func (c *CategoriesConfig) GetMergeDistance(size string) float64 {
	if dist, ok := c.MergeDistance[size]; ok {
		return dist
	}
	return 500.0 // Default fallback
}

// GetGroup returns the group name for a category, or empty string if none.
func (c *CategoriesConfig) GetGroup(category string) string {
	if group, ok := c.GroupLookup[strings.ToLower(category)]; ok {
		return group
	}
	return ""
}

// ShouldPreground returns true if the category has pregrounding enabled.
func (c *CategoriesConfig) ShouldPreground(category string) bool {
	if cat, ok := c.Categories[strings.ToLower(category)]; ok {
		return cat.Preground
	}
	return false
}
