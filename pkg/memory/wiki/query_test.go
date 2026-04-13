package wiki

import (
	"path/filepath"
	"testing"
	"time"
)

func TestQueryManagerSearchFindsRelevantPages(t *testing.T) {
	wikiDir := filepath.Join(t.TempDir(), "wiki")
	now := time.Date(2026, 4, 13, 12, 0, 0, 0, time.UTC)

	if err := SavePage(filepath.Join(wikiDir, "concepts", "llm-wiki.md"), &Page{
		Title:   "LLM Wiki",
		Created: now,
		Updated: now,
		Type:    PageTypeConcept,
		Tags:    []string{"memory"},
		Body:    "Structured knowledge compilation beats fragment recall.\n",
	}); err != nil {
		t.Fatalf("SavePage failed: %v", err)
	}
	if err := SavePage(filepath.Join(wikiDir, "concepts", "qmd.md"), &Page{
		Title:   "QMD",
		Created: now,
		Updated: now,
		Type:    PageTypeConcept,
		Body:    "Session export and collection sync.\n",
	}); err != nil {
		t.Fatalf("SavePage failed: %v", err)
	}

	manager := NewQueryManager(wikiDir)
	results, err := manager.Search("knowledge", 5)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Page.Title != "LLM Wiki" {
		t.Fatalf("expected LLM Wiki result, got %q", results[0].Page.Title)
	}
}
