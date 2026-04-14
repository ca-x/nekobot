package wiki

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// LintManager checks wiki health against schema and file structure.
type LintManager struct {
	wikiDir string
	index   *IndexManager
	schema  *SchemaManager
}

// NewLintManager creates a lint manager for one wiki directory.
func NewLintManager(wikiDir string) *LintManager {
	return &LintManager{
		wikiDir: wikiDir,
		index:   NewIndexManager(wikiDir),
		schema:  NewSchemaManager(wikiDir),
	}
}

// Run executes a lightweight wiki health check.
func (m *LintManager) Run() (*LintResult, error) {
	config, err := m.schema.Load()
	if err != nil {
		return nil, err
	}
	entries, err := m.index.Rebuild()
	if err != nil {
		return nil, err
	}

	result := &LintResult{}
	knownPages := make(map[string]string, len(entries))
	indexTitles := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		knownPages[strings.ToLower(entry.Title)] = entry.Path
		indexTitles[strings.ToLower(entry.Title)] = struct{}{}
	}

	for _, entry := range entries {
		page, err := LoadPage(entry.Path)
		if err != nil {
			return nil, fmt.Errorf("lint wiki page %s: %w", entry.Path, err)
		}
		if _, ok := indexTitles[strings.ToLower(page.Title)]; !ok {
			result.MissingIndex = append(result.MissingIndex, page.Title)
		}

		if config.SplitLines > 0 && countContentLines(page.Body) > config.SplitLines {
			result.OversizedPages = append(result.OversizedPages, page.Title)
		}
		if len(page.OutLinks) < config.MinOutLinks {
			result.OrphanPages = append(result.OrphanPages, page.Title)
		}
		for _, link := range page.OutLinks {
			if _, ok := knownPages[strings.ToLower(link)]; !ok {
				result.BrokenLinks = append(result.BrokenLinks, LinkIssue{
					SourcePage: page.Title,
					TargetLink: link,
				})
			}
		}
		invalidTags := make([]string, 0)
		for _, tag := range page.Tags {
			if !config.IsValidTag(tag) {
				invalidTags = append(invalidTags, tag)
			}
		}
		if len(invalidTags) > 0 {
			result.TagViolations = append(result.TagViolations, TagViolation{
				Page: page.Title,
				Tags: invalidTags,
			})
		}
	}

	sort.Strings(result.MissingIndex)
	sort.Strings(result.OversizedPages)
	sort.Strings(result.OrphanPages)
	result.TotalIssues = len(result.BrokenLinks) + len(result.MissingIndex) + len(result.OversizedPages) + len(result.OrphanPages) + len(result.TagViolations)
	return result, nil
}

func countContentLines(body string) int {
	if strings.TrimSpace(body) == "" {
		return 0
	}
	return len(strings.Split(strings.TrimRight(body, "\n"), "\n"))
}

// DefaultSchemaPath returns the SCHEMA.md path for one workspace wiki.
func DefaultSchemaPath(workspace string) string {
	return filepath.Join(DefaultWikiDir(workspace), "SCHEMA.md")
}
