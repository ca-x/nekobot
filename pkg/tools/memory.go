package tools

import (
	"context"
	"fmt"

	"nekobot/pkg/logger"
	"nekobot/pkg/memory"
)

// MemoryTool provides memory search and storage capabilities.
type MemoryTool struct {
	log     *logger.Logger
	manager *memory.Manager
}

// NewMemoryTool creates a new memory tool.
func NewMemoryTool(log *logger.Logger, manager *memory.Manager) *MemoryTool {
	return &MemoryTool{
		log:     log,
		manager: manager,
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

	t.log.Info("Searching memory",
		logger.String("query", query),
		logger.Int("max_results", maxResults))

	context, err := t.manager.GetRelevantContext(ctx, query, maxResults)
	if err != nil {
		return "", fmt.Errorf("failed to search memory: %w", err)
	}

	if context == "" {
		return "No relevant memories found for the query", nil
	}

	return context, nil
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
		logger.String("type", string(memType)),
		logger.Int("length", len(text)))

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
		return "Memory system is enabled and operational", nil
	}
	return "Memory system is disabled", nil
}
