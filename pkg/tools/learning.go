package tools

import (
	"context"
	"fmt"
	"strings"

	"nekobot/pkg/memory"
)

// LearningTool records durable learnings for future prompt context.
type LearningTool struct {
	manager *memory.LearningsManager
}

// NewLearningTool creates a new learning tool.
func NewLearningTool(manager *memory.LearningsManager) *LearningTool {
	return &LearningTool{manager: manager}
}

// Name returns the tool name.
func (t *LearningTool) Name() string {
	return "learning"
}

// Description returns the tool description.
func (t *LearningTool) Description() string {
	return "Record durable learnings into append-only JSONL storage for future context compression."
}

// Parameters returns the tool parameter schema.
func (t *LearningTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"content": map[string]interface{}{
				"type":        "string",
				"description": "The learning content to record.",
			},
			"category": map[string]interface{}{
				"type":        "string",
				"description": "Optional learning category.",
			},
			"confidence": map[string]interface{}{
				"type":        "number",
				"description": "Confidence score between 0 and 1.",
			},
			"source": map[string]interface{}{
				"type":        "string",
				"description": "Optional source label.",
			},
			"metadata": map[string]interface{}{
				"type":        "object",
				"description": "Optional metadata payload.",
			},
		},
		"required": []string{"content"},
	}
}

// Execute stores a learning entry.
func (t *LearningTool) Execute(ctx context.Context, params map[string]interface{}) (string, error) {
	_ = ctx
	if t == nil || t.manager == nil {
		return "", fmt.Errorf("learning system not initialized")
	}

	content, ok := params["content"].(string)
	if !ok || strings.TrimSpace(content) == "" {
		return "", fmt.Errorf("content parameter is required")
	}

	entry := memory.LearningEntry{
		Content: strings.TrimSpace(content),
	}

	if category, ok := params["category"].(string); ok {
		entry.Category = strings.TrimSpace(category)
	}
	if confidence, ok := params["confidence"].(float64); ok {
		entry.Confidence = confidence
	}
	if source, ok := params["source"].(string); ok {
		entry.Source = strings.TrimSpace(source)
	}
	if metadata, ok := params["metadata"].(map[string]interface{}); ok {
		entry.Metadata = metadata
	}

	if err := t.manager.Add(entry); err != nil {
		return "", fmt.Errorf("record learning: %w", err)
	}

	return "Successfully recorded learning", nil
}
