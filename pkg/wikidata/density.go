package wikidata

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// LanguageConfig holds density and average word length for a specific language.
type LanguageConfig struct {
	Density    float64 `yaml:"density"`
	AvgWordLen float64 `yaml:"avg_word_len"`
}

// DensityManager handles language-specific information density and word count estimation.
type DensityManager struct {
	languages map[string]LanguageConfig
}

// NewDensityManager loads the language configuration from a YAML file.
func NewDensityManager(configPath string) (*DensityManager, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read language config: %w", err)
	}

	var configs map[string]LanguageConfig
	if err := yaml.Unmarshal(data, &configs); err != nil {
		return nil, fmt.Errorf("failed to parse language config: %w", err)
	}

	return &DensityManager{languages: configs}, nil
}

// ExtractLang extracts the language code from a Wikipedia URL.
// Example: "https://zh.wikipedia.org/wiki/..." -> "zh"
func (m *DensityManager) ExtractLang(url string) string {
	if !strings.Contains(url, "wikipedia.org") {
		return "en" // Default
	}
	parts := strings.Split(url, "://")
	if len(parts) < 2 {
		return "en"
	}
	subparts := strings.Split(parts[1], ".")
	if len(subparts) < 1 {
		return "en"
	}
	return subparts[0]
}

// GetConfig returns the configuration for a given language code.
func (m *DensityManager) GetConfig(lang string) LanguageConfig {
	if cfg, ok := m.languages[lang]; ok {
		return cfg
	}
	// Default to English-like if unknown
	return LanguageConfig{
		Density:    1.0,
		AvgWordLen: 5.1,
	}
}

// GetAdjustedLength returns the English-equivalent character count.
func (m *DensityManager) GetAdjustedLength(rawLen int, url string) int {
	lang := m.ExtractLang(url)
	cfg := m.GetConfig(lang)
	return int(float64(rawLen) * cfg.Density)
}

// EstimateWordCount returns the estimated English-equivalent word count.
func (m *DensityManager) EstimateWordCount(rawLen int, url string) int {
	lang := m.ExtractLang(url)
	cfg := m.GetConfig(lang)

	// Information depth in English-equivalent characters
	infoDepthChars := float64(rawLen) * cfg.Density

	// Convert to word count using English average word length (5.1)
	// because MaxWords is historically based on English word counts.
	return int(infoDepthChars / 5.1)
}

// EstimateWordCountTarget returns the estimated word count in a target language.
// Useful for calculating MaxWords for a specific output language.
func (m *DensityManager) EstimateWordCountTarget(rawLen int, sourceURL, targetLang string) int {
	sourceLang := m.ExtractLang(sourceURL)
	sourceCfg := m.GetConfig(sourceLang)
	targetCfg := m.GetConfig(targetLang)

	// infoDepth is "intrinsic content units"
	infoDepth := float64(rawLen) * sourceCfg.Density

	// To get words in target lang: infoDepth / avgWordLenTarget
	// We divide by targetCfg.AvgWordLen because that's how many bytes/chars
	// a "word" takes in that language on average.
	if targetCfg.AvgWordLen <= 0 {
		return int(infoDepth / 5.1)
	}

	return int(infoDepth / targetCfg.AvgWordLen)
}
