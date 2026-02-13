package converter

import (
	"encoding/json"
	"fmt"
	"strings"

	"nekobot/pkg/providers"
)

// OpenAIConverter handles conversion between OpenAI API format and unified format.
type OpenAIConverter struct {
	BaseConverter
}

// NewOpenAIConverter creates a new OpenAI format converter.
func NewOpenAIConverter() *OpenAIConverter {
	return &OpenAIConverter{}
}

// openAIRequest represents the OpenAI API request format.
type openAIRequest struct {
	Model       string                   `json:"model"`
	Messages    []openAIMessage          `json:"messages"`
	MaxTokens   int                      `json:"max_tokens,omitempty"`
	Temperature float64                  `json:"temperature,omitempty"`
	TopP        float64                  `json:"top_p,omitempty"`
	Stream      bool                     `json:"stream,omitempty"`
	Tools       []map[string]interface{} `json:"tools,omitempty"`
	ToolChoice  interface{}              `json:"tool_choice,omitempty"`
	User        string                   `json:"user,omitempty"`
}

// openAIMessage represents a single message in OpenAI format.
type openAIMessage struct {
	Role       string           `json:"role"`
	Content    string           `json:"content,omitempty"`
	Name       string           `json:"name,omitempty"`
	ToolCalls  []openAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

// openAIToolCall represents a tool call in OpenAI format.
type openAIToolCall struct {
	ID       string             `json:"id"`
	Type     string             `json:"type"`
	Function openAIFunctionCall `json:"function"`
}

// openAIFunctionCall represents the function details in a tool call.
type openAIFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// openAIResponse represents the OpenAI API response format.
type openAIResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role      string           `json:"role"`
			Content   string           `json:"content"`
			ToolCalls []openAIToolCall `json:"tool_calls,omitempty"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *providers.UnifiedUsage `json:"usage,omitempty"`
}

// openAIStreamChunk represents a streaming chunk in OpenAI format.
type openAIStreamChunk struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Role      string           `json:"role,omitempty"`
			Content   string           `json:"content,omitempty"`
			ToolCalls []openAIToolCall `json:"tool_calls,omitempty"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason,omitempty"`
	} `json:"choices"`
	Usage *providers.UnifiedUsage `json:"usage,omitempty"`
}

// ToProviderRequest converts a UnifiedRequest to OpenAI format.
func (c *OpenAIConverter) ToProviderRequest(unified *providers.UnifiedRequest) (interface{}, error) {
	req := openAIRequest{
		Model:       unified.Model,
		MaxTokens:   unified.MaxTokens,
		Temperature: unified.Temperature,
		TopP:        unified.TopP,
		Stream:      unified.Stream,
		ToolChoice:  unified.ToolChoice,
		User:        unified.User,
	}

	// Convert messages
	req.Messages = make([]openAIMessage, len(unified.Messages))
	for i, msg := range unified.Messages {
		oaiMsg := openAIMessage{
			Role:       msg.Role,
			Content:    msg.Content,
			Name:       msg.Name,
			ToolCallID: msg.ToolCallID,
		}

		// Convert tool calls
		if len(msg.ToolCalls) > 0 {
			oaiMsg.ToolCalls = make([]openAIToolCall, len(msg.ToolCalls))
			for j, tc := range msg.ToolCalls {
				// Marshal arguments to JSON string
				argsJSON, err := json.Marshal(tc.Arguments)
				if err != nil {
					return nil, fmt.Errorf("marshaling tool call arguments: %w", err)
				}

				oaiMsg.ToolCalls[j] = openAIToolCall{
					ID:   tc.ID,
					Type: tc.Type,
					Function: openAIFunctionCall{
						Name:      tc.Name,
						Arguments: string(argsJSON),
					},
				}
			}
		}

		req.Messages[i] = oaiMsg
	}

	// Convert tools
	if len(unified.Tools) > 0 {
		req.Tools = c.ConvertToolsToOpenAIFormat(unified.Tools)
	}

	return req, nil
}

// FromProviderResponse converts an OpenAI response to UnifiedResponse.
func (c *OpenAIConverter) FromProviderResponse(providerResp interface{}) (*providers.UnifiedResponse, error) {
	// Re-marshal and unmarshal to convert to openAIResponse type
	data, err := json.Marshal(providerResp)
	if err != nil {
		return nil, fmt.Errorf("marshaling provider response: %w", err)
	}

	var resp openAIResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("unmarshaling to OpenAI format: %w", err)
	}

	if len(resp.Choices) == 0 {
		return &providers.UnifiedResponse{
			ID:           resp.ID,
			Model:        resp.Model,
			Content:      "",
			FinishReason: "stop",
			Usage:        resp.Usage,
		}, nil
	}

	choice := resp.Choices[0]
	unified := &providers.UnifiedResponse{
		ID:           resp.ID,
		Model:        resp.Model,
		Content:      choice.Message.Content,
		FinishReason: choice.FinishReason,
		Usage:        resp.Usage,
	}

	// Convert tool calls
	if len(choice.Message.ToolCalls) > 0 {
		unified.ToolCalls = make([]providers.UnifiedToolCall, len(choice.Message.ToolCalls))
		for i, tc := range choice.Message.ToolCalls {
			var args map[string]interface{}
			if tc.Function.Arguments != "" {
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
					args = map[string]interface{}{"raw": tc.Function.Arguments}
				}
			}

			unified.ToolCalls[i] = providers.UnifiedToolCall{
				ID:        tc.ID,
				Type:      tc.Type,
				Name:      tc.Function.Name,
				Arguments: args,
			}
		}
	}

	return unified, nil
}

// FromProviderStreamChunk converts an OpenAI streaming chunk to UnifiedStreamChunk.
func (c *OpenAIConverter) FromProviderStreamChunk(rawChunk []byte) (*providers.UnifiedStreamChunk, error) {
	// OpenAI uses SSE format: "data: {json}\n\n"
	// Strip "data: " prefix if present
	line := string(rawChunk)
	line = strings.TrimSpace(line)

	if strings.HasPrefix(line, "data: ") {
		line = strings.TrimPrefix(line, "data: ")
		line = strings.TrimSpace(line)
	}

	// Check for stream termination
	if line == "[DONE]" {
		return nil, nil
	}

	var chunk openAIStreamChunk
	if err := json.Unmarshal([]byte(line), &chunk); err != nil {
		return nil, fmt.Errorf("unmarshaling stream chunk: %w", err)
	}

	if len(chunk.Choices) == 0 {
		return &providers.UnifiedStreamChunk{
			ID:    chunk.ID,
			Model: chunk.Model,
			Usage: chunk.Usage,
		}, nil
	}

	choice := chunk.Choices[0]
	unified := &providers.UnifiedStreamChunk{
		ID:           chunk.ID,
		Model:        chunk.Model,
		FinishReason: choice.FinishReason,
		Usage:        chunk.Usage,
		Delta: providers.UnifiedDelta{
			Role:    choice.Delta.Role,
			Content: choice.Delta.Content,
		},
	}

	// Convert tool calls in delta
	if len(choice.Delta.ToolCalls) > 0 {
		unified.Delta.ToolCalls = make([]providers.UnifiedToolCall, len(choice.Delta.ToolCalls))
		for i, tc := range choice.Delta.ToolCalls {
			var args map[string]interface{}
			if tc.Function.Arguments != "" {
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
					// For deltas, arguments might be partial, so store as raw if invalid
					args = map[string]interface{}{"raw": tc.Function.Arguments}
				}
			}

			unified.Delta.ToolCalls[i] = providers.UnifiedToolCall{
				ID:        tc.ID,
				Type:      tc.Type,
				Name:      tc.Function.Name,
				Arguments: args,
			}
		}
	}

	return unified, nil
}
