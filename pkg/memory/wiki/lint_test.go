package wiki

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLintManagerFlagsBrokenLinksAndTagViolations(t *testing.T) {
	wikiDir := filepath.Join(t.TempDir(), "wiki")
	now := time.Date(2026, 4, 14, 12, 0, 0, 0, time.UTC)

	schema := `# Wiki Schema

## Domain
- project: Nekobot

## Rules
- Every wiki page starts with YAML frontmatter.
- Use [[wikilinks]] to link between pages (minimum 2 outbound links per page)

## Tag Taxonomy
- memory
- research

## Page Thresholds
- Split page: exceeds ~200 lines
`
	if err := SavePage(filepath.Join(wikiDir, "concepts", "alpha.md"), &Page{
		Title:   "Alpha",
		Created: now,
		Updated: now,
		Type:    PageTypeConcept,
		Tags:    []string{"invalid-tag"},
		Body:    "Alpha body with [[Missing Page]].\n",
	}); err != nil {
		t.Fatalf("SavePage failed: %v", err)
	}
	if err := SavePage(filepath.Join(wikiDir, "concepts", "beta.md"), &Page{
		Title:   "Beta",
		Created: now,
		Updated: now,
		Type:    PageTypeConcept,
		Tags:    []string{"memory"},
		Body:    "Beta body with [[Alpha]] and [[Missing Two]].\n",
	}); err != nil {
		t.Fatalf("SavePage failed: %v", err)
	}
	if err := osWriteFile(filepath.Join(wikiDir, "SCHEMA.md"), []byte(schema)); err != nil {
		t.Fatalf("write schema: %v", err)
	}

	result, err := NewLintManager(wikiDir).Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if len(result.BrokenLinks) == 0 {
		t.Fatal("expected broken links")
	}
	if len(result.TagViolations) == 0 {
		t.Fatal("expected tag violations")
	}
	if result.TotalIssues == 0 {
		t.Fatal("expected total issues > 0")
	}
}

func osWriteFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0o644)
}
