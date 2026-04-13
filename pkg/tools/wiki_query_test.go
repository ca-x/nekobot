package tools

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"nekobot/pkg/memory/wiki"
)

func TestWikiQueryToolSearchesWorkspaceWiki(t *testing.T) {
	workspace := t.TempDir()
	wikiDir := filepath.Join(workspace, "wiki", "concepts")
	now := time.Date(2026, 4, 13, 12, 0, 0, 0, time.UTC)

	if err := wiki.SavePage(filepath.Join(wikiDir, "llm-wiki.md"), &wiki.Page{
		Title:   "LLM Wiki",
		Created: now,
		Updated: now,
		Type:    wiki.PageTypeConcept,
		Body:    "Structured knowledge improves recall.\n",
	}); err != nil {
		t.Fatalf("SavePage failed: %v", err)
	}

	tool := NewWikiQueryTool(workspace)
	out, err := tool.Execute(context.Background(), map[string]interface{}{
		"query": "knowledge",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !strings.Contains(out, "LLM Wiki") {
		t.Fatalf("expected result to include LLM Wiki, got %q", out)
	}
}
