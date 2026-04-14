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

func TestWikiQueryToolSupportsTypeAndTagFilters(t *testing.T) {
	workspace := t.TempDir()
	now := time.Date(2026, 4, 13, 12, 0, 0, 0, time.UTC)

	if err := wiki.SavePage(filepath.Join(workspace, "wiki", "concepts", "llm-wiki.md"), &wiki.Page{
		Title:   "LLM Wiki",
		Created: now,
		Updated: now,
		Type:    wiki.PageTypeConcept,
		Tags:    []string{"memory"},
		Aliases: []string{"knowledge base"},
		Body:    "Structured knowledge improves recall.\n",
	}); err != nil {
		t.Fatalf("SavePage failed: %v", err)
	}
	if err := wiki.SavePage(filepath.Join(workspace, "wiki", "references", "wechat-api.md"), &wiki.Page{
		Title:   "WeChat API",
		Created: now,
		Updated: now,
		Type:    wiki.PageTypeSummary,
		Tags:    []string{"wechat"},
		Body:    "Protocol reference.\n",
	}); err != nil {
		t.Fatalf("SavePage failed: %v", err)
	}

	tool := NewWikiQueryTool(workspace)
	out, err := tool.Execute(context.Background(), map[string]interface{}{
		"query": "knowledge base",
		"type":  "concept",
		"tag":   "memory",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !strings.Contains(out, "LLM Wiki") || strings.Contains(out, "WeChat API") {
		t.Fatalf("expected filtered result set, got %q", out)
	}
}
