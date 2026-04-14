package wiki

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var wikilinkPattern = regexp.MustCompile(`\[\[([^\]]+)\]\]`)

type pageFrontmatter struct {
	Title          string    `yaml:"title"`
	Created        time.Time `yaml:"created"`
	Updated        time.Time `yaml:"updated"`
	Type           PageType  `yaml:"type"`
	Tags           []string  `yaml:"tags"`
	Sources        []string  `yaml:"sources"`
	Aliases        []string  `yaml:"aliases,omitempty"`
	Confidence     string    `yaml:"confidence,omitempty"`
	Summary        string    `yaml:"summary,omitempty"`
	Contradictions []string  `yaml:"contradictions,omitempty"`
}

// ParsePage parses one markdown wiki page with YAML frontmatter.
func ParsePage(path string, data []byte) (*Page, error) {
	raw := string(data)
	if !strings.HasPrefix(raw, "---\n") {
		return nil, fmt.Errorf("parse wiki page %s: missing YAML frontmatter", path)
	}

	rest := raw[len("---\n"):]
	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		return nil, fmt.Errorf("parse wiki page %s: missing closing frontmatter delimiter", path)
	}

	frontmatterText := rest[:end]
	body := strings.TrimLeft(rest[end+len("\n---\n"):], "\n")

	var frontmatter pageFrontmatter
	if err := yaml.Unmarshal([]byte(frontmatterText), &frontmatter); err != nil {
		return nil, fmt.Errorf("parse wiki page %s frontmatter: %w", path, err)
	}
	if strings.TrimSpace(frontmatter.Title) == "" {
		return nil, fmt.Errorf("parse wiki page %s: title is required", path)
	}

	page := &Page{
		Title:          strings.TrimSpace(frontmatter.Title),
		Created:        frontmatter.Created,
		Updated:        frontmatter.Updated,
		Type:           frontmatter.Type,
		Tags:           append([]string(nil), frontmatter.Tags...),
		Sources:        append([]string(nil), frontmatter.Sources...),
		Aliases:        append([]string(nil), frontmatter.Aliases...),
		Confidence:     strings.TrimSpace(frontmatter.Confidence),
		Summary:        strings.TrimSpace(frontmatter.Summary),
		Contradictions: append([]string(nil), frontmatter.Contradictions...),
		Body:           body,
		FilePath:       path,
		OutLinks:       extractWikiLinks(body),
	}
	return page, nil
}

// LoadPage reads and parses one wiki page from disk.
func LoadPage(path string) (*Page, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read wiki page %s: %w", path, err)
	}
	return ParsePage(path, data)
}

// SavePage renders and writes one wiki page to disk.
func SavePage(path string, page *Page) error {
	if page == nil {
		return fmt.Errorf("save wiki page %s: page is nil", path)
	}
	rendered, err := RenderPage(page)
	if err != nil {
		return fmt.Errorf("save wiki page %s: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("save wiki page %s: create parent directory: %w", path, err)
	}
	if err := os.WriteFile(path, []byte(rendered), 0o644); err != nil {
		return fmt.Errorf("save wiki page %s: %w", path, err)
	}
	return nil
}

// RenderPage renders one wiki page into markdown with YAML frontmatter.
func RenderPage(page *Page) (string, error) {
	if page == nil {
		return "", fmt.Errorf("render wiki page: page is nil")
	}
	if strings.TrimSpace(page.Title) == "" {
		return "", fmt.Errorf("render wiki page: title is required")
	}

	created := page.Created
	if created.IsZero() {
		created = time.Now().UTC()
	}
	updated := page.Updated
	if updated.IsZero() {
		updated = created
	}

	frontmatter := pageFrontmatter{
		Title:          strings.TrimSpace(page.Title),
		Created:        created,
		Updated:        updated,
		Type:           page.Type,
		Tags:           append([]string(nil), page.Tags...),
		Sources:        append([]string(nil), page.Sources...),
		Aliases:        append([]string(nil), page.Aliases...),
		Confidence:     strings.TrimSpace(page.Confidence),
		Summary:        strings.TrimSpace(page.Summary),
		Contradictions: append([]string(nil), page.Contradictions...),
	}
	yamlBytes, err := yaml.Marshal(frontmatter)
	if err != nil {
		return "", fmt.Errorf("render wiki page frontmatter: %w", err)
	}

	body := strings.TrimLeft(page.Body, "\n")
	if body != "" && !strings.HasSuffix(body, "\n") {
		body += "\n"
	}

	return fmt.Sprintf("---\n%s---\n\n%s", string(yamlBytes), body), nil
}

func extractWikiLinks(body string) []string {
	if strings.TrimSpace(body) == "" {
		return nil
	}
	matches := wikilinkPattern.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		return nil
	}
	out := make([]string, 0, len(matches))
	seen := make(map[string]struct{}, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		link := strings.TrimSpace(match[1])
		if link == "" {
			continue
		}
		if _, exists := seen[link]; exists {
			continue
		}
		seen[link] = struct{}{}
		out = append(out, link)
	}
	return out
}
