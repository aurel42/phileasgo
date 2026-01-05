package prompts

import (
	"bytes"
	"fmt"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// Manager handles loading and rendering of prompt templates.
type Manager struct {
	root *template.Template
	dir  string
}

// NewManager creates a new prompt manager loading templates from the specified directory.
func NewManager(dir string) (*Manager, error) {
	m := &Manager{
		dir: dir,
	}
	m.root = template.New("root").Funcs(template.FuncMap{
		"category":  m.categoryFunc,
		"interests": interestsFunc,
		"maybe":     maybeFunc,
		"pick":      pickFunc,
	})

	if err := m.loadCommon(dir); err != nil {
		return nil, fmt.Errorf("loading common templates: %w", err)
	}

	if err := m.loadTemplates(dir); err != nil {
		return nil, fmt.Errorf("loading templates: %w", err)
	}

	return m, nil
}

func (m *Manager) loadCommon(dir string) error {
	commonDir := filepath.Join(dir, "common")
	return filepath.Walk(commonDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}

		if info.IsDir() || !strings.HasSuffix(path, ".tmpl") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		if _, err = m.root.Parse(string(content)); err != nil {
			return fmt.Errorf("parsing %s: %w", path, err)
		}
		return nil
	})
}

func (m *Manager) loadTemplates(dir string) error {
	return filepath.Walk(dir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() || !strings.HasSuffix(path, ".tmpl") {
			return nil
		}

		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		name := filepath.ToSlash(rel)
		if strings.HasPrefix(name, "common/") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		if _, err = m.root.New(name).Parse(string(content)); err != nil {
			return fmt.Errorf("parsing %s: %w", path, err)
		}
		return nil
	})
}

// Render executes the named template with the provided data.
func (m *Manager) Render(name string, data any) (string, error) {
	var buf bytes.Buffer
	if err := m.root.ExecuteTemplate(&buf, name, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (m *Manager) categoryFunc(name string, data any) (string, error) {
	if name == "" {
		return "", nil
	}

	// Try to find a template named "category/<name>.tmpl"
	tmplName := "category/" + strings.ToLower(name) + ".tmpl"
	t := m.root.Lookup(tmplName)
	if t == nil {
		// Silently ignore missing category templates
		return "", nil
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// interestsFunc shuffles interests and returns them as a comma-separated string.
// It also randomly excludes 2 topics from the list to add variety.
func interestsFunc(interests []string) string {
	if len(interests) == 0 {
		return ""
	}
	// Make a copy to avoid modifying the original slice
	shuffled := make([]string, len(interests))
	copy(shuffled, interests)
	rand.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})
	// Randomly exclude 2 topics if we have at least 4 interests (leaves at least 2)
	const excludeCount = 2
	if len(shuffled) >= excludeCount+2 {
		shuffled = shuffled[:len(shuffled)-excludeCount]
	}
	return strings.Join(shuffled, ", ")
}

// maybeFunc includes content with a given probability (0-100).
// Usage: {{maybe 50 "This text appears 50% of the time"}}
// Re-rolls on each template render.
func maybeFunc(percent int, content string) string {
	if percent <= 0 {
		return ""
	}
	if percent >= 100 {
		return content
	}
	if rand.Intn(100) < percent {
		return content
	}
	return ""
}

// pickFunc selects one random option from a list separated by "|||".
// Usage: {{pick "Option A|||Option B|||Option C"}}
// Re-rolls on each template render.
func pickFunc(options string) string {
	parts := strings.Split(options, "|||")
	if len(parts) == 0 {
		return ""
	}
	// Trim whitespace from each option
	for i, p := range parts {
		parts[i] = strings.TrimSpace(p)
	}
	return parts[rand.Intn(len(parts))]
}
