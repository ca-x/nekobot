package wiki

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// QueryManager performs read-only wiki searches.
type QueryManager struct {
	wikiDir string
	index   *IndexManager
}

// NewQueryManager creates a query manager for one wiki directory.
func NewQueryManager(wikiDir string) *QueryManager {
	return &QueryManager{
		wikiDir: wikiDir,
		index:   NewIndexManager(wikiDir),
	}
}

// Search finds relevant wiki pages using simple lexical scoring.
func (m *QueryManager) Search(query string, limit int) ([]SearchResult, error) {
	return m.SearchWithOptions(query, limit, QueryOptions{})
}

// SearchWithOptions finds relevant wiki pages using lexical scoring and optional filters.
func (m *QueryManager) SearchWithOptions(query string, limit int, opts QueryOptions) ([]SearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("wiki search query is required")
	}
	if limit <= 0 {
		limit = 5
	}

	entries, err := m.index.Rebuild()
	if err != nil {
		return nil, err
	}
	lowerQuery := strings.ToLower(query)
	results := make([]SearchResult, 0, len(entries))
	for _, entry := range entries {
		page, err := LoadPage(entry.Path)
		if err != nil {
			return nil, fmt.Errorf("load wiki page for search %s: %w", entry.Path, err)
		}
		if opts.Type != "" && page.Type != opts.Type {
			continue
		}
		if tag := strings.TrimSpace(strings.ToLower(opts.Tag)); tag != "" && !pageHasTag(page, tag) {
			continue
		}
		score := scorePage(lowerQuery, page, entry)
		if score == 0 {
			continue
		}
		results = append(results, SearchResult{
			Page:    *page,
			Summary: entry.Summary,
			Score:   score,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Score == results[j].Score {
			return strings.ToLower(results[i].Page.Title) < strings.ToLower(results[j].Page.Title)
		}
		return results[i].Score > results[j].Score
	})
	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

func scorePage(query string, page *Page, entry IndexEntry) int {
	score := 0
	title := strings.ToLower(page.Title)
	if strings.Contains(title, query) {
		score += 5
	}
	summary := strings.ToLower(entry.Summary)
	if strings.Contains(summary, query) {
		score += 3
	}
	body := strings.ToLower(page.Body)
	if strings.Contains(body, query) {
		score += 2
	}
	for _, tag := range page.Tags {
		if strings.Contains(strings.ToLower(tag), query) {
			score++
		}
	}
	for _, alias := range page.Aliases {
		if strings.Contains(strings.ToLower(alias), query) {
			score += 2
		}
	}
	return score
}

func pageHasTag(page *Page, tag string) bool {
	if page == nil || tag == "" {
		return false
	}
	for _, item := range page.Tags {
		if strings.EqualFold(strings.TrimSpace(item), tag) {
			return true
		}
	}
	return false
}

// DefaultWikiDir returns the default wiki directory inside one workspace.
func DefaultWikiDir(workspace string) string {
	return filepath.Join(workspace, "wiki")
}
