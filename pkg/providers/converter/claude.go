package converter

import (
	"encoding/json"
	"fmt"
	"strings"

	"nekobot/pkg/providers"
)

// ClaudeConverter handles conversion between Claude (Anthropic) API format and unified format.
type ClaudeConverter struct {
	BaseConverter
}

// NewClaudeConverter creates a new Claude format converter.
func NewClaudeConverter() *ClaudeConverter {
	return &ClaudeConverter{}
}

// claudeThinkingConfig represents the thinking configuration in Claude API.
type claudeThinkingConfig struct {
	Type         string `json:"type"`
	BudgetTokens int    `json:"budget_tokens"`
}

// claudeRequest represents the Claude API request format.
type claudeRequest struct {
	Model       string                   `json:"model"`
	Messages    []claudeMessage          `json:"messages"`
	System      string                   `json:"system,omitempty"`
	MaxTokens   int                      `json:"max_tokens"`
	Temperature float64                  `json:"temperature,omitempty"`
	TopP        float64                  `json:"top_p,omitempty"`
	Stream      bool                     `json:"stream,omitempty"`
	Tools       []map[string]interface{} `json:"tools,omitempty"`
	ToolChoice  interface{}              `json:"tool_choice,omitempty"`
	Thinking    *claudeThinkingConfig    `json:"thinking,omitempty"`
}

// claudeMessage represents a single message in Claude format.
type claudeMessage struct {
	Role    string                   `json:"role"` // "user" or "assistant"
	Content []map[string]interface{} `json:"content"`
}

// claudeResponse represents the Claude API response format.
type claudeResponse struct {
	ID           string                   `json:"id"`
	Type         string                   `json:"type"`
	Role         string                   `json:"role"`
	Content      []map[string]interface{} `json:"content"`
	Model        string                   `json:"model"`
	StopReason   string                   `json:"stop_reason"`
	StopSequence string                   `json:"stop_sequence,omitempty"`
	Usage        struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// claudeStreamChunk represents a streaming event in Claude format.
type claudeStreamChunk struct {
	Type  string                 `json:"type"`
	Index int                    `json:"index,omitempty"`
	Delta map[string]interface{} `json:"delta,omitempty"`

	// For message_start event
	Message *claudeResponse `json:"message,omitempty"`

	// For content_block_start event
	ContentBlock map[string]interface{} `json:"content_block,omitempty"`

	// For message_delta event
	Usage struct {
		OutputTokens int `json:"output_tokens"`
	} `json:"usage,omitempty"`
}

// ToProviderRequest converts a UnifiedRequest to Claude format.
func (c *ClaudeConverter) ToProviderRequest(unified *providers.UnifiedRequest) (interface{}, error) {
	// Extract and merge system messages
	systemMsgs, conversationMsgs := c.ExtractSystemMessages(unified.Messages)
	systemText := c.MergeSystemMessages(systemMsgs)

	req := claudeRequest{
		Model:       unified.Model,
		System:      systemText,
		MaxTokens:   unified.MaxTokens,
		Temperature: unified.Temperature,
		TopP:        unified.TopP,
		Stream:      unified.Stream,
		ToolChoice:  unified.ToolChoice,
	}

	// Default max_tokens if not set (required by Claude)
	if req.MaxTokens == 0 {
		req.MaxTokens = 4096
	}

	// Convert messages - Claude uses a content blocks structure
	req.Messages = make([]claudeMessage, 0, len(conversationMsgs))
	for _, msg := range conversationMsgs {
		// Claude only supports "user" and "assistant" roles in messages
		// System messages go in the system parameter
		// Tool messages are represented as content blocks in user messages
		role := msg.Role
		if role == "tool" {
			role = "user"
		}

		claudeMsg := claudeMessage{
			Role:    role,
			Content: make([]map[string]interface{}, 0),
		}

		// Add text content
		if msg.Content != "" {
			if msg.ToolCallID != "" {
				// Tool result
				claudeMsg.Content = append(claudeMsg.Content, map[string]interface{}{
					"type":        "tool_result",
					"tool_use_id": msg.ToolCallID,
					"content":     msg.Content,
				})
			} else {
				// Regular text
				claudeMsg.Content = append(claudeMsg.Content, map[string]interface{}{
					"type": "text",
					"text": msg.Content,
				})
			}
		}

		// Add tool calls
		for _, tc := range msg.ToolCalls {
			claudeMsg.Content = append(claudeMsg.Content, map[string]interface{}{
				"type":  "tool_use",
				"id":    tc.ID,
				"name":  tc.Name,
				"input": tc.Arguments,
			})
		}

		req.Messages = append(req.Messages, claudeMsg)
	}

	// Convert tools
	if len(unified.Tools) > 0 {
		req.Tools = make([]map[string]interface{}, len(unified.Tools))
		for i, tool := range unified.Tools {
			req.Tools[i] = map[string]interface{}{
				"name":         tool.Name,
				"description":  tool.Description,
				"input_schema": tool.Parameters,
			}
		}
	}

	// Apply extended thinking if configured via Extra
	if unified.Extra != nil {
		if enabled, ok := unified.Extra["extended_thinking"].(bool); ok && enabled {
			budget := 10000 // default budget
			if b, ok := unified.Extra["thinking_budget"].(int); ok && b > 0 {
				budget = b
			}
			req.Thinking = &claudeThinkingConfig{
				Type:         "enabled",
				BudgetTokens: budget,
			}
			// Claude requires temperature=1 when thinking is enabled
			req.Temperature = 0
			req.TopP = 0
		}
	}

	return req, nil
}

// FromProviderResponse converts a Claude response to UnifiedResponse.
func (c *ClaudeConverter) FromProviderResponse(providerResp interface{}) (*providers.UnifiedResponse, error) {
	// Re-marshal and unmarshal to convert to claudeResponse type
	data, err := json.Marshal(providerResp)
	if err != nil {
		return nil, fmt.Errorf("marshaling provider response: %w", err)
	}

	var resp claudeResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("unmarshaling to Claude format: %w", err)
	}

	unified := &providers.UnifiedResponse{
		ID:    resp.ID,
		Model: resp.Model,
		Usage: &providers.UnifiedUsage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		},
	}

	// Convert stop_reason to finish_reason
	switch resp.StopReason {
	case "end_turn":
		unified.FinishReason = "stop"
	case "tool_use":
		unified.FinishReason = "tool_calls"
	case "max_tokens":
		unified.FinishReason = "length"
	default:
		unified.FinishReason = resp.StopReason
	}

	// Parse content blocks
	for _, block := range resp.Content {
		blockType, _ := block["type"].(string)

		switch blockType {
		case "thinking":
			if thinking, ok := block["thinking"].(string); ok {
				unified.Thinking += thinking
			}

		case "text":
			if text, ok := block["text"].(string); ok {
				unified.Content += text
			}

		case "tool_use":
			id, _ := block["id"].(string)
			name, _ := block["name"].(string)
			input, _ := block["input"].(map[string]interface{})

			unified.ToolCalls = append(unified.ToolCalls, providers.UnifiedToolCall{
				ID:        id,
				Type:      "function",
				Name:      name,
				Arguments: input,
			})
		}
	}

	return unified, nil
}

// FromProviderStreamChunk converts a Claude streaming event to UnifiedStreamChunk.
func (c *ClaudeConverter) FromProviderStreamChunk(rawChunk []byte) (*providers.UnifiedStreamChunk, error) {
	// Claude uses SSE format: "event: {type}\ndata: {json}\n\n"
	// We need to extract the JSON data
	line := string(rawChunk)
	line = strings.TrimSpace(line)

	// Extract data from SSE format
	if strings.HasPrefix(line, "data: ") {
		line = strings.TrimPrefix(line, "data: ")
		line = strings.TrimSpace(line)
	}

	var chunk claudeStreamChunk
	if err := json.Unmarshal([]byte(line), &chunk); err != nil {
		return nil, fmt.Errorf("unmarshaling stream chunk: %w", err)
	}

	unified := &providers.UnifiedStreamChunk{}

	switch chunk.Type {
	case "message_start":
		if chunk.Message != nil {
			unified.ID = chunk.Message.ID
			unified.Model = chunk.Message.Model
		}

	case "content_block_start":
		if chunk.ContentBlock != nil {
			blockType, _ := chunk.ContentBlock["type"].(string)
			if blockType == "tool_use" {
				id, _ := chunk.ContentBlock["id"].(string)
				name, _ := chunk.ContentBlock["name"].(string)
				unified.Delta.ToolCalls = append(unified.Delta.ToolCalls, providers.UnifiedToolCall{
					ID:   id,
					Name: name,
					Type: "function",
				})
			}
		}

	case "content_block_delta":
		if chunk.Delta != nil {
			deltaType, _ := chunk.Delta["type"].(string)

			switch deltaType {
			case "thinking_delta":
				if thinking, ok := chunk.Delta["thinking"].(string); ok {
					unified.Delta.Thinking = thinking
				}
			case "text_delta":
				if text, ok := chunk.Delta["text"].(string); ok {
					unified.Delta.Content = text
				}
			case "input_json_delta":
				// Accumulate partial JSON for tool inputs
				if partialJSON, ok := chunk.Delta["partial_json"].(string); ok {
					// Store partial JSON in a tool call delta
					// Note: In practice, you'd accumulate these until the block ends
					unified.Delta.ToolCalls = append(unified.Delta.ToolCalls, providers.UnifiedToolCall{
						Arguments: map[string]interface{}{"partial_json": partialJSON},
					})
				}
			}
		}

	case "message_delta":
		if chunk.Delta != nil {
			if stopReason, ok := chunk.Delta["stop_reason"].(string); ok {
				switch stopReason {
				case "end_turn":
					unified.FinishReason = "stop"
				case "tool_use":
					unified.FinishReason = "tool_calls"
				case "max_tokens":
					unified.FinishReason = "length"
				default:
					unified.FinishReason = stopReason
				}
			}
		}
		if chunk.Usage.OutputTokens > 0 {
			unified.Usage = &providers.UnifiedUsage{
				CompletionTokens: chunk.Usage.OutputTokens,
			}
		}

	case "message_stop":
		// Stream complete
		return nil, nil
	}

	return unified, nil
}
