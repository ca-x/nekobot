package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"nekobot/pkg/memory/wiki"
)

func TestWikiLintToolReportsIssues(t *testing.T) {
	workspace := t.TempDir()
	wikiDir := filepath.Join(workspace, "wiki")
	now := time.Date(2026, 4, 14, 12, 0, 0, 0, time.UTC)
	if err := os.MkdirAll(filepath.Join(wikiDir, "concepts"), 0o755); err != nil {
		t.Fatalf("create wiki concepts dir: %v", err)
	}

	schema := `# Wiki Schema

## Domain
- project: Nekobot

## Rules
- Every wiki page starts with YAML frontmatter.
- Use [[wikilinks]] to link between pages (minimum 2 outbound links per page)

## Tag Taxonomy
- memory
`
	if err := os.WriteFile(filepath.Join(wikiDir, "SCHEMA.md"), []byte(schema), 0o644); err != nil {
		t.Fatalf("write schema: %v", err)
	}
	if err := wiki.SavePage(filepath.Join(wikiDir, "concepts", "alpha.md"), &wiki.Page{
		Title:   "Alpha",
		Created: now,
		Updated: now,
		Type:    wiki.PageTypeConcept,
		Tags:    []string{"bad-tag"},
		Body:    "Alpha with [[Missing Page]].\n",
	}); err != nil {
		t.Fatalf("SavePage failed: %v", err)
	}

	tool := NewWikiLintTool(workspace)
	out, err := tool.Execute(context.Background(), map[string]interface{}{})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !strings.Contains(out, "Broken link") {
		t.Fatalf("expected broken link in lint output, got %q", out)
	}
}
