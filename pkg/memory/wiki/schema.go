package wiki

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var schemaSectionPattern = regexp.MustCompile(`(?m)^##\s+(.+)$`)

// SchemaManager loads and validates wiki conventions from SCHEMA.md.
type SchemaManager struct {
	wikiDir string
}

// NewSchemaManager creates a schema manager for one wiki directory.
func NewSchemaManager(wikiDir string) *SchemaManager {
	return &SchemaManager{wikiDir: wikiDir}
}

// Path returns the schema file path.
func (m *SchemaManager) Path() string {
	return filepath.Join(m.wikiDir, "SCHEMA.md")
}

// Load reads and parses SCHEMA.md into a lightweight config model.
func (m *SchemaManager) Load() (*WikiConfig, error) {
	data, err := os.ReadFile(m.Path())
	if err != nil {
		return nil, fmt.Errorf("read wiki schema: %w", err)
	}
	return ParseSchema(string(data))
}

// ParseSchema parses SCHEMA.md content into a WikiConfig.
func ParseSchema(content string) (*WikiConfig, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, fmt.Errorf("parse wiki schema: content is empty")
	}

	config := &WikiConfig{
		MinOutLinks: 2,
		SplitLines:  200,
	}

	sections := splitSchemaSections(content)
	config.Domain = firstBulletValue(sections["Domain"])
	config.TagTaxonomy = bulletValues(sections["Tag Taxonomy"])
	config.ArchivePolicy = lastBulletValue(sections["Page Thresholds"])
	if rules := bulletValues(sections["Rules"]); len(rules) > 0 {
		for _, rule := range rules {
			lower := strings.ToLower(rule)
			if strings.Contains(lower, "minimum") && strings.Contains(lower, "outbound links") {
				if strings.Contains(lower, "2") {
					config.MinOutLinks = 2
				}
			}
		}
	}
	if thresholds := bulletValues(sections["Page Thresholds"]); len(thresholds) > 0 {
		for _, threshold := range thresholds {
			lower := strings.ToLower(threshold)
			if strings.Contains(lower, "split page") && strings.Contains(lower, "200") {
				config.SplitLines = 200
			}
		}
	}
	return config, nil
}

// IsValidTag reports whether the tag is allowed by schema taxonomy.
func (c *WikiConfig) IsValidTag(tag string) bool {
	tag = strings.TrimSpace(strings.ToLower(tag))
	if tag == "" {
		return false
	}
	if len(c.TagTaxonomy) == 0 {
		return true
	}
	for _, allowed := range c.TagTaxonomy {
		if strings.TrimSpace(strings.ToLower(allowed)) == tag {
			return true
		}
	}
	return false
}

func splitSchemaSections(content string) map[string]string {
	matches := schemaSectionPattern.FindAllStringSubmatchIndex(content, -1)
	sections := make(map[string]string, len(matches))
	for i, match := range matches {
		titleStart, titleEnd := match[2], match[3]
		bodyStart := match[1]
		bodyEnd := len(content)
		if i+1 < len(matches) {
			bodyEnd = matches[i+1][0]
		}
		title := strings.TrimSpace(content[titleStart:titleEnd])
		body := strings.TrimSpace(content[bodyStart:bodyEnd])
		sections[title] = body
	}
	return sections
}

func bulletValues(section string) []string {
	lines := strings.Split(section, "\n")
	values := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "- ") {
			continue
		}
		values = append(values, strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")))
	}
	return values
}

func firstBulletValue(section string) string {
	values := bulletValues(section)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func lastBulletValue(section string) string {
	values := bulletValues(section)
	if len(values) == 0 {
		return ""
	}
	return values[len(values)-1]
}
