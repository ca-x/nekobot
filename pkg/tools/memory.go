package tools

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"
	"nekobot/pkg/logger"
	"nekobot/pkg/memory"
)

// MemoryToolOptions configures memory tool behavior.
type MemoryToolOptions struct {
	DefaultTopK   int
	MaxTopK       int
	SearchPolicy  string
	IncludeScores bool
}

// MemoryTool provides memory search and storage capabilities.
type MemoryTool struct {
	log     *logger.Logger
	manager memory.SearchManager
	options MemoryToolOptions
}

// NewMemoryTool creates a new memory tool.
func NewMemoryTool(log *logger.Logger, manager memory.SearchManager, opts MemoryToolOptions) *MemoryTool {
	if opts.DefaultTopK <= 0 {
		opts.DefaultTopK = 5
	}
	if opts.MaxTopK <= 0 || opts.MaxTopK < opts.DefaultTopK {
		opts.MaxTopK = opts.DefaultTopK
	}
	policy := strings.TrimSpace(strings.ToLower(opts.SearchPolicy))
	if policy == "" {
		policy = "hybrid"
	}
	opts.SearchPolicy = policy

	return &MemoryTool{
		log:     log,
		manager: manager,
		options: opts,
	}
}

// Name returns the tool name.
func (t *MemoryTool) Name() string {
	return "memory"
}

// Description returns the tool description.
func (t *MemoryTool) Description() string {
	return `Search and manage long-term memory using vector embeddings. Use this tool to:
- Search for relevant information from past conversations
- Store important facts, preferences, or context
- Retrieve user preferences and learned information

The memory system uses semantic search to find relevant content.`
}

// Parameters returns the tool parameters schema.
func (t *MemoryTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"search", "add", "status"},
				"description": "Action to perform: search, add, or status",
			},
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Search query or text to add (required for search and add)",
			},
			"type": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"fact", "preference", "context", "conversation"},
				"description": "Type of memory when adding (default: context)",
			},
			"max_results": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of search results (default: 5)",
			},
			"min_score": map[string]interface{}{
				"type":        "number",
				"description": "Minimum similarity score between 0 and 1.",
			},
		},
		"required": []string{"action"},
	}
}

// Execute executes the memory tool.
func (t *MemoryTool) Execute(ctx context.Context, params map[string]interface{}) (string, error) {
	if t.manager == nil {
		return "", fmt.Errorf("memory system not initialized")
	}

	if !t.manager.IsEnabled() {
		return "Memory system is currently disabled", nil
	}

	action, ok := params["action"].(string)
	if !ok {
		return "", fmt.Errorf("action parameter is required")
	}

	switch action {
	case "search":
		return t.search(ctx, params)
	case "add":
		return t.add(ctx, params)
	case "status":
		return t.status(ctx)
	default:
		return "", fmt.Errorf("unknown action: %s", action)
	}
}

// search searches memory for relevant content.
func (t *MemoryTool) search(ctx context.Context, params map[string]interface{}) (string, error) {
	query, ok := params["query"].(string)
	if !ok || query == "" {
		return "", fmt.Errorf("query parameter is required for search")
	}

	maxResults := 5
	if mr, ok := params["max_results"].(float64); ok {
		maxResults = int(mr)
	}
	if maxResults <= 0 {
		maxResults = t.options.DefaultTopK
	}
	if maxResults > t.options.MaxTopK {
		maxResults = t.options.MaxTopK
	}

	searchOpts := memory.DefaultSearchOptions()
	searchOpts.Limit = maxResults
	searchOpts.Hybrid = t.options.SearchPolicy != "vector"
	if minScore, ok := params["min_score"].(float64); ok && minScore >= 0 && minScore <= 1 {
		searchOpts.MinScore = minScore
	}

	t.log.Info("Searching memory",
		zap.String("query", query),
		zap.Int("max_results", maxResults),
		zap.String("search_policy", t.options.SearchPolicy))

	results, err := t.manager.Search(ctx, query, searchOpts)
	if err != nil {
		return "", fmt.Errorf("failed to search memory: %w", err)
	}

	if len(results) == 0 {
		return "No relevant memories found for the query", nil
	}

	return formatMemorySearchResults(results, t.options.IncludeScores), nil
}

// add adds new content to memory.
func (t *MemoryTool) add(ctx context.Context, params map[string]interface{}) (string, error) {
	text, ok := params["query"].(string)
	if !ok || text == "" {
		return "", fmt.Errorf("query parameter (text to add) is required")
	}

	// Get memory type
	memType := memory.TypeContext
	if typeStr, ok := params["type"].(string); ok {
		switch typeStr {
		case "fact":
			memType = memory.TypeFact
		case "preference":
			memType = memory.TypePreference
		case "context":
			memType = memory.TypeContext
		case "conversation":
			memType = memory.TypeConversation
		}
	}

	t.log.Info("Adding to memory",
		zap.String("type", string(memType)),
		zap.Int("length", len(text)))

	metadata := memory.Metadata{
		Importance: 0.5, // Default importance
	}

	if err := t.manager.Add(ctx, text, memory.SourceSession, memType, metadata); err != nil {
		return "", fmt.Errorf("failed to add to memory: %w", err)
	}

	return fmt.Sprintf("Successfully added to memory as %s", memType), nil
}

// status returns the memory system status.
func (t *MemoryTool) status(ctx context.Context) (string, error) {
	if t.manager.IsEnabled() {
		status := t.manager.Status()
		backend, _ := status["backend"].(string)
		if backend == "" {
			backend = "builtin"
		}
		return fmt.Sprintf("Memory system is enabled and operational (backend: %s)", backend), nil
	}
	return "Memory system is disabled", nil
}

func formatMemorySearchResults(results []*memory.SearchResult, includeScores bool) string {
	var sb strings.Builder
	sb.WriteString("# Relevant Memory\n\n")

	for i, result := range results {
		if includeScores {
			sb.WriteString(fmt.Sprintf("## Memory %d (score: %.2f)\n\n", i+1, result.Score))
		} else {
			sb.WriteString(fmt.Sprintf("## Memory %d\n\n", i+1))
		}
		sb.WriteString(result.Text)
		sb.WriteString("\n\n")
		if result.Metadata.FilePath != "" {
			sb.WriteString(fmt.Sprintf("*Source: %s*\n\n", result.Metadata.FilePath))
		}
		sb.WriteString("---\n\n")
	}

	return sb.String()
}
