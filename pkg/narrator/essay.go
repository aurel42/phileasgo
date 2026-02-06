package narrator

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"sync"

	"gopkg.in/yaml.v3"

	"phileasgo/pkg/llm/prompts"
	"phileasgo/pkg/prompt"
)

// EssayTopic represents a single essay topic definition.
type EssayTopic struct {
	ID          string `yaml:"id"`
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	MaxWords    int    `yaml:"max_words"`
	Icon        string `yaml:"icon"`
}

// EssayConfig holds the list of defined essay topics.
type EssayConfig struct {
	Topics []EssayTopic `yaml:"topics"`
}

// EssayHandler manages the selection and prompting of regional essays.
type EssayHandler struct {
	topics        []EssayTopic
	availablePool []string // IDs of topics available in the current rotation cycle
	mu            sync.Mutex
	prompts       *prompts.Manager
}

// NewEssayHandler creates a new EssayHandler by loading topics from the config file.
func NewEssayHandler(configPath string, prompts *prompts.Manager) (*EssayHandler, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read essays config: %w", err)
	}

	var cfg EssayConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse essays config: %w", err)
	}

	return &EssayHandler{
		topics:        cfg.Topics,
		availablePool: make([]string, 0),
		prompts:       prompts,
	}, nil
}

// SelectTopic selects a random topic from the rotation pool.
// It guarantees that all topics are played once before any repeat.
func (h *EssayHandler) SelectTopic() (*EssayTopic, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if len(h.topics) == 0 {
		return nil, fmt.Errorf("no essay topics available")
	}

	// Refill if empty
	if len(h.availablePool) == 0 {
		h.availablePool = make([]string, len(h.topics))
		for i, t := range h.topics {
			h.availablePool[i] = t.ID
		}
		slog.Info("EssayHandler: Topic pool exhausted. Starting new rotation cycle.", "topics", len(h.topics))
	}

	// Pick random index
	idx := rand.Intn(len(h.availablePool))
	selectedID := h.availablePool[idx]

	// Swap with last and shrink to remove (O(1))
	h.availablePool[idx] = h.availablePool[len(h.availablePool)-1]
	h.availablePool = h.availablePool[:len(h.availablePool)-1]

	// Find the topic object
	for _, t := range h.topics {
		if t.ID == selectedID {
			// Return a copy
			selected := t
			return &selected, nil
		}
	}

	return nil, fmt.Errorf("topic %s not found in rotation", selectedID)
}

func (h *EssayHandler) BuildPrompt(ctx context.Context, topic *EssayTopic, pd *prompt.Data) (string, error) {
	// Prepare template data
	// We merge the Topic specific fields into the prompt data
	(*pd)["TopicName"] = topic.Name
	(*pd)["TopicDescription"] = topic.Description
	(*pd)["MaxWords"] = topic.MaxWords

	return h.prompts.Render("narrator/essay.tmpl", pd)
}
