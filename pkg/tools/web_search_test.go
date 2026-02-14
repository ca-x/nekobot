package tools

import (
	"context"
	"strings"
	"testing"
)

func TestNewWebSearchTool_DuckDuckGoOnly(t *testing.T) {
	tool := NewWebSearchTool(WebSearchToolOptions{
		BraveAPIKey:       "",
		DuckDuckGoEnabled: true,
	})
	if tool == nil {
		t.Fatal("expected tool, got nil")
	}
	if got := tool.ProviderSummary(); got != "duckduckgo" {
		t.Fatalf("expected duckduckgo provider, got %s", got)
	}
}

func TestNewWebSearchTool_None(t *testing.T) {
	tool := NewWebSearchTool(WebSearchToolOptions{
		BraveAPIKey:       "",
		DuckDuckGoEnabled: false,
	})
	if tool != nil {
		t.Fatal("expected nil tool when no providers enabled")
	}
}

func TestDuckDuckGoExtractResults(t *testing.T) {
	p := NewDuckDuckGoSearchProvider()

	html := `
<div class="results">
  <a class="result__a" href="/l/?kh=-1&uddg=https%3A%2F%2Fexample.com%2Fa">Example A</a>
  <a class="result__snippet">Snippet A</a>
  <a class="result__a" href="https://example.com/b">Example B</a>
  <a class="result__snippet">Snippet B</a>
</div>`

	out := p.extractResults(html, 2, "demo query")
	if !strings.Contains(out, "Results for: demo query (via DuckDuckGo)") {
		t.Fatalf("missing header: %s", out)
	}
	if !strings.Contains(out, "https://example.com/a") {
		t.Fatalf("missing decoded URL: %s", out)
	}
	if !strings.Contains(out, "Snippet A") {
		t.Fatalf("missing snippet: %s", out)
	}
}

func TestWebSearchTool_MissingQuery(t *testing.T) {
	tool := NewWebSearchTool(WebSearchToolOptions{
		DuckDuckGoEnabled: true,
	})
	_, err := tool.Execute(context.Background(), map[string]interface{}{})
	if err == nil || !strings.Contains(err.Error(), "query") {
		t.Fatalf("expected query error, got %v", err)
	}
}
