package tools

import (
	"context"
	"fmt"
	"strings"

	"nekobot/pkg/memory/wiki"
)

// WikiQueryTool performs read-only searches over workspace wiki pages.
type WikiQueryTool struct {
	manager *wiki.QueryManager
}

// NewWikiQueryTool creates a wiki query tool for one workspace.
func NewWikiQueryTool(workspace string) *WikiQueryTool {
	return &WikiQueryTool{
		manager: wiki.NewQueryManager(wiki.DefaultWikiDir(workspace)),
	}
}

// Name returns the tool name.
func (t *WikiQueryTool) Name() string {
	return "wiki_query"
}

// Description returns the tool description.
func (t *WikiQueryTool) Description() string {
	return "Search the structured LLM Wiki knowledge base stored in workspace/wiki."
}

// Parameters returns the tool parameter schema.
func (t *WikiQueryTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Search query for the workspace wiki.",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of results to return.",
			},
			"type": map[string]interface{}{
				"type":        "string",
				"description": "Optional page type filter (concept, topic, reference, thesis, etc.).",
			},
			"tag": map[string]interface{}{
				"type":        "string",
				"description": "Optional tag filter.",
			},
		},
		"required": []string{"query"},
	}
}

// Execute runs the wiki query tool.
func (t *WikiQueryTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	_ = ctx
	if t == nil || t.manager == nil {
		return "", fmt.Errorf("wiki query tool not initialized")
	}
	query, ok := args["query"].(string)
	if !ok || strings.TrimSpace(query) == "" {
		return "", fmt.Errorf("query parameter is required")
	}

	limit := 5
	if raw, ok := args["limit"].(float64); ok {
		limit = int(raw)
	}
	var opts wiki.QueryOptions
	if rawType, ok := args["type"].(string); ok {
		opts.Type = wiki.PageType(strings.TrimSpace(strings.ToLower(rawType)))
	}
	if rawTag, ok := args["tag"].(string); ok {
		opts.Tag = strings.TrimSpace(rawTag)
	}
	results, err := t.manager.SearchWithOptions(query, limit, opts)
	if err != nil {
		return "", fmt.Errorf("wiki query: %w", err)
	}
	if len(results) == 0 {
		return "No relevant wiki pages found.", nil
	}

	var b strings.Builder
	b.WriteString("# Wiki Query Results\n\n")
	for i, result := range results {
		_, _ = fmt.Fprintf(&b, "## %d. %s\n\n", i+1, result.Page.Title)
		if result.Summary != "" {
			_, _ = fmt.Fprintf(&b, "%s\n\n", result.Summary)
		}
		if result.Page.FilePath != "" {
			_, _ = fmt.Fprintf(&b, "*Path: %s*\n\n", result.Page.FilePath)
		}
	}
	return b.String(), nil
}
