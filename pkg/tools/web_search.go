package tools

import (
	"context"
	"fmt"
	"strings"
)

// WebSearchToolOptions controls provider selection.
type WebSearchToolOptions struct {
	BraveAPIKey          string
	BraveMaxResults      int
	DuckDuckGoEnabled    bool
	DuckDuckGoMaxResults int
}

// WebSearchTool searches the web using a primary provider with optional fallback.
type WebSearchTool struct {
	primary    SearchProvider
	fallback   SearchProvider
	maxResults int
}

type webSearchArgs struct {
	Query string `json:"query"`
	Count *int   `json:"count,omitempty"`
}

type webSearchParameterSchema struct {
	Type       string               `json:"type"`
	Properties webSearchSchemaProps `json:"properties"`
	Required   []string             `json:"required,omitempty"`
}

type webSearchSchemaProps struct {
	Query ParamSchema `json:"query"`
	Count ParamSchema `json:"count"`
}

// NewWebSearchTool creates a web-search tool with provider fallback.
// Priority: Brave (if API key present) -> DuckDuckGo (if enabled).
func NewWebSearchTool(opts WebSearchToolOptions) *WebSearchTool {
	primary, fallback, maxResults := BuildSearchProviders(opts)
	if primary == nil {
		return nil
	}

	return &WebSearchTool{
		primary:    primary,
		fallback:   fallback,
		maxResults: maxResults,
	}
}

// ProviderSummary returns the active provider route for logging/diagnostics.
func (t *WebSearchTool) ProviderSummary() string {
	if t == nil || t.primary == nil {
		return "none"
	}
	if t.fallback == nil {
		return t.primary.Name()
	}
	return fmt.Sprintf("%s -> %s", t.primary.Name(), t.fallback.Name())
}

func (t *WebSearchTool) Name() string { return "web_search" }

func (t *WebSearchTool) Description() string {
	return "Search the web for current information. Uses Brave Search when configured, and can fallback to DuckDuckGo HTML search."
}

func (t *WebSearchTool) Parameters() map[string]interface{} {
	return MustSchemaMap(webSearchParameterSchema{
		Type: "object",
		Properties: webSearchSchemaProps{
			Query: ParamSchema{
				Type:        "string",
				Description: "Search query",
			},
			Count: ParamSchema{
				Type:        "integer",
				Description: "Number of results to return (1-10, default: 5)",
				Minimum:     intPtr(1),
				Maximum:     intPtr(10),
			},
		},
		Required: []string{"query"},
	})
}

func (t *WebSearchTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	if t == nil || t.primary == nil {
		return "", fmt.Errorf("no web search provider configured")
	}

	parsed, err := DecodeArgs[webSearchArgs](args)
	if err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	query := strings.TrimSpace(parsed.Query)
	if query == "" {
		return "", fmt.Errorf("query cannot be empty")
	}

	count := t.maxResults
	if parsed.Count != nil && *parsed.Count >= 1 && *parsed.Count <= 10 {
		count = *parsed.Count
	}

	result, err := t.primary.Search(ctx, query, count)
	if err == nil {
		return result, nil
	}

	if t.fallback != nil {
		fallbackResult, fallbackErr := t.fallback.Search(ctx, query, count)
		if fallbackErr == nil {
			return fallbackResult, nil
		}
		return "", fmt.Errorf("search failed (%s: %v; fallback %s: %v)", t.primary.Name(), err, t.fallback.Name(), fallbackErr)
	}

	return "", fmt.Errorf("search failed (%s): %w", t.primary.Name(), err)
}
