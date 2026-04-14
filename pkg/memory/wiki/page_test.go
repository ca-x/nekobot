package wiki

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRenderAndParsePageRoundTrip(t *testing.T) {
	now := time.Date(2026, 4, 13, 12, 0, 0, 0, time.UTC)
	page := &Page{
		Title:      "llm-wiki",
		Created:    now,
		Updated:    now,
		Type:       PageTypeConcept,
		Tags:       []string{"memory", "wiki"},
		Sources:    []string{"spec"},
		Aliases:    []string{"llm knowledge base"},
		Confidence: "high",
		Summary:    "Structured wiki protocol for LLM-oriented knowledge bases.",
		Body:       "See also [[prompt-memory]].\n",
	}

	rendered, err := RenderPage(page)
	if err != nil {
		t.Fatalf("RenderPage failed: %v", err)
	}
	if !strings.Contains(rendered, "title: llm-wiki") {
		t.Fatalf("expected title in rendered page, got %q", rendered)
	}

	parsed, err := ParsePage(filepath.Join(t.TempDir(), "llm-wiki.md"), []byte(rendered))
	if err != nil {
		t.Fatalf("ParsePage failed: %v", err)
	}
	if parsed.Title != "llm-wiki" {
		t.Fatalf("expected title llm-wiki, got %q", parsed.Title)
	}
	if parsed.Type != PageTypeConcept {
		t.Fatalf("expected type concept, got %q", parsed.Type)
	}
	if parsed.Confidence != "high" {
		t.Fatalf("expected confidence high, got %q", parsed.Confidence)
	}
	if len(parsed.Aliases) != 1 || parsed.Aliases[0] != "llm knowledge base" {
		t.Fatalf("expected aliases to round-trip, got %+v", parsed.Aliases)
	}
	if parsed.Summary != "Structured wiki protocol for LLM-oriented knowledge bases." {
		t.Fatalf("expected summary to round-trip, got %q", parsed.Summary)
	}
	if len(parsed.OutLinks) != 1 || parsed.OutLinks[0] != "prompt-memory" {
		t.Fatalf("expected one wikilink, got %+v", parsed.OutLinks)
	}
}
