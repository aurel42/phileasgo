package gemini

import (
	"math/rand"

	"google.golang.org/genai"
)

// resolveModel returns the target model name and configuration for the given intent.
func (c *Client) resolveModel(intent string) (string, *genai.GenerateContentConfig) {
	// Identify configured model name
	targetModel := c.modelName // Default

	// Check if intent maps to a profile
	if profileModel, ok := c.profiles[intent]; ok && profileModel != "" {
		targetModel = profileModel
	}

	// Default configuration
	config := &genai.GenerateContentConfig{}

	// Enable Google Search for narration tasks (Text generation)
	// Note: Google Search is currently incompatible with JSON mode (used by dynamic_config).
	if intent == "essay" || intent == "narration" {
		config.Tools = []*genai.Tool{
			{
				GoogleSearch: &genai.GoogleSearch{},
			},
		}

		// Apply temperature with bell curve (normal distribution)
		if c.temperatureBase > 0 {
			temp := sampleTemperature(c.temperatureBase, c.temperatureJitter)
			config.Temperature = &temp
		}
	}

	return targetModel, config
}

// sampleTemperature samples from a normal distribution centered on base.
// Uses jitter as the approximate range (±jitter), with σ = jitter/2.
// Result is clamped to [base-jitter, base+jitter] and minimum 0.1.
func sampleTemperature(base, jitter float32) float32 {
	if jitter <= 0 {
		return base
	}

	// Sample from normal distribution: μ = base, σ = jitter/2
	sigma := float64(jitter) / 2.0
	sample := float64(base) + rand.NormFloat64()*sigma

	// Clamp to [base-jitter, base+jitter]
	minTemp := float64(base) - float64(jitter)
	maxTemp := float64(base) + float64(jitter)
	if sample < minTemp {
		sample = minTemp
	}
	if sample > maxTemp {
		sample = maxTemp
	}

	// Ensure minimum positive temperature
	if sample < 0.1 {
		sample = 0.1
	}

	return float32(sample)
}
