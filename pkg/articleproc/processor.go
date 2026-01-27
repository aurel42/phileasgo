package articleproc

import (
	"io"
	"strings"

	"phileasgo/pkg/llm"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// Info contains the cleaned prose and metadata.
type Info struct {
	Prose      string
	WordCount  int
	IsReliable bool
}

// ExtractProse parses the Wikipedia HTML and extracts main body paragraphs.
func ExtractProse(r io.Reader) (*Info, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, err
	}

	var output []string
	var totalWords int

	// 1. Find the mw-parser-output div which contains the actual article content
	parserOutput := findParserOutput(doc)
	if parserOutput == nil {
		// Fallback to body if we can't find the specific Wikipedia class
		parserOutput = findBody(doc)
	}

	if parserOutput != nil {
		// 2. Traverse children and extract prose
		// terminal sections usually precede structural noise like reference lists or navboxes.
		shouldStop := false
		for c := parserOutput.FirstChild; c != nil; c = c.NextSibling {
			if shouldStop {
				break
			}

			if c.Type == html.ElementNode {
				// Truly language-agnostic terminal signals:
				// 1. If we hit a known terminal container (reflist, navbox, etc.)
				if isStructuralNoise(c) {
					shouldStop = true
					continue
				}

				// 2. Stop at terminal headers (only if immediately followed by noise)
				if c.DataAtom == atom.H2 || c.DataAtom == atom.H3 {
					if isFollowedByStructuralNoise(c) {
						shouldStop = true
						continue
					}
				}

				// Only collect paragraphs
				if c.DataAtom == atom.P {
					text := cleanParagraph(c)
					if text != "" {
						// PHILEAS: Wrap prose at the source to ensure logs (and prompts) are readable.
						wrappedText := llm.WordWrap(text, 80)
						output = append(output, wrappedText)
						totalWords += countWords(text)
					}
				}
			}
		}
	}

	prose := strings.Join(output, "\n\n")
	return &Info{
		Prose:      prose,
		WordCount:  totalWords,
		IsReliable: totalWords > 20, // Arbitrary threshold for "actual content"
	}, nil
}

func findParserOutput(n *html.Node) *html.Node {
	if n.Type == html.ElementNode && n.DataAtom == atom.Div {
		for _, a := range n.Attr {
			if a.Key == "class" && strings.Contains(a.Val, "mw-parser-output") {
				return n
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if res := findParserOutput(c); res != nil {
			return res
		}
	}
	return nil
}

func findBody(n *html.Node) *html.Node {
	if n.Type == html.ElementNode && n.DataAtom == atom.Body {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if res := findBody(c); res != nil {
			return res
		}
	}
	return nil
}

func cleanParagraph(p *html.Node) string {
	var b strings.Builder
	traverseParagraph(p, &b)
	return strings.TrimSpace(b.String())
}

func traverseParagraph(n *html.Node, b *strings.Builder) {
	if n.Type == html.TextNode {
		b.WriteString(n.Data)
		return
	}

	if n.Type == html.ElementNode {
		// Skip unwanted elements inside paragraphs
		// - <sup> for citations [1][2]
		// - <style>, <script>
		// - .mw-empty-elt
		if n.DataAtom == atom.Sup || n.DataAtom == atom.Style || n.DataAtom == atom.Script {
			return
		}
		for _, a := range n.Attr {
			if a.Key == "class" && (strings.Contains(a.Val, "mw-empty-elt") || strings.Contains(a.Val, "reference")) {
				return
			}
		}
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		traverseParagraph(c, b)
	}
}

func countWords(s string) int {
	return len(strings.Fields(s))
}

func isStructuralNoise(n *html.Node) bool {
	if n.Type != html.ElementNode {
		return false
	}
	for _, a := range n.Attr {
		if a.Key == "class" {
			val := strings.ToLower(a.Val)
			// Navigation boxes, reference lists, and galleries are almost always terminal.
			if strings.Contains(val, "reflist") ||
				strings.Contains(val, "references") ||
				strings.Contains(val, "navbox") ||
				strings.Contains(val, "asbox") || // Stub notice
				strings.Contains(val, "catlinks") {
				return true
			}
		}
	}
	return false
}

func isFollowedByStructuralNoise(n *html.Node) bool {
	// A header is terminal if it's followed immediately by noisy containers.
	// We allow for one empty text node or similar in between.
	limit := 1
	for next := n.NextSibling; next != nil && limit > 0; next = next.NextSibling {
		if next.Type == html.ElementNode {
			if isStructuralNoise(next) {
				return true
			}
			limit--
		}
	}
	return false
}
