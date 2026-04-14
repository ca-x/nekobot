package wiki

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// IndexManager manages wiki/index.md.
type IndexManager struct {
	wikiDir string
}

// NewIndexManager creates an index manager for one wiki directory.
func NewIndexManager(wikiDir string) *IndexManager {
	return &IndexManager{wikiDir: wikiDir}
}

// Path returns the index file path.
func (m *IndexManager) Path() string {
	return filepath.Join(m.wikiDir, "index.md")
}

// Write rewrites wiki/index.md with the provided entries.
func (m *IndexManager) Write(entries []IndexEntry) error {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Type == entries[j].Type {
			return strings.ToLower(entries[i].Title) < strings.ToLower(entries[j].Title)
		}
		return entries[i].Type < entries[j].Type
	})

	var b strings.Builder
	b.WriteString("# Wiki Index\n\n")
	if len(entries) == 0 {
		b.WriteString("_No wiki pages yet._\n")
	} else {
		currentType := PageType("")
		for _, entry := range entries {
			if entry.Type != currentType {
				currentType = entry.Type
				if b.Len() > 0 {
					b.WriteString("\n")
				}
				_, _ = fmt.Fprintf(&b, "## %s\n\n", strings.Title(string(entry.Type)))
			}
			summary := strings.TrimSpace(entry.Summary)
			if summary == "" {
				summary = "No summary yet."
			}
			_, _ = fmt.Fprintf(&b, "- [[%s]] — %s\n", entry.Title, summary)
		}
	}
	b.WriteString("\n")

	if err := os.MkdirAll(m.wikiDir, 0o755); err != nil {
		return fmt.Errorf("write wiki index: create wiki directory: %w", err)
	}
	if err := os.WriteFile(m.Path(), []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("write wiki index: %w", err)
	}
	return nil
}

// Rebuild scans wiki pages and rewrites index.md.
func (m *IndexManager) Rebuild() ([]IndexEntry, error) {
	var entries []IndexEntry
	err := filepath.Walk(m.wikiDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			base := filepath.Base(path)
			if strings.HasPrefix(base, "_") && path != m.wikiDir {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".md" {
			return nil
		}
		base := filepath.Base(path)
		if base == "SCHEMA.md" || base == "index.md" || base == "log.md" {
			return nil
		}

		page, err := LoadPage(path)
		if err != nil {
			return err
		}
		entries = append(entries, IndexEntry{
			Title:   page.Title,
			Path:    path,
			Type:    page.Type,
			Summary: summarizePage(page),
			Updated: chooseTimestamp(page.Updated, info.ModTime()),
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("rebuild wiki index: %w", err)
	}
	if err := m.Write(entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func summarizeBody(body string) string {
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(strings.TrimPrefix(line, "#"))
		if trimmed == "" {
			continue
		}
		if len(trimmed) > 120 {
			return trimmed[:117] + "..."
		}
		return trimmed
	}
	return ""
}

func summarizePage(page *Page) string {
	if page == nil {
		return ""
	}
	if summary := strings.TrimSpace(page.Summary); summary != "" {
		if len(summary) > 120 {
			return summary[:117] + "..."
		}
		return summary
	}
	return summarizeBody(page.Body)
}

func chooseTimestamp(primary, fallback time.Time) time.Time {
	if !primary.IsZero() {
		return primary
	}
	return fallback
}
