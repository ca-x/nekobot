package converter

import (
	"encoding/json"
	"fmt"
	"strings"

	"nekobot/pkg/providers"
)

// GeminiConverter handles conversion between Google Gemini API format and unified format.
type GeminiConverter struct {
	BaseConverter
}

// NewGeminiConverter creates a new Gemini format converter.
func NewGeminiConverter() *GeminiConverter {
	return &GeminiConverter{}
}

// geminiRequest represents the Gemini API request format.
type geminiRequest struct {
	Contents          []geminiContent         `json:"contents"`
	SystemInstruction *geminiContent          `json:"systemInstruction,omitempty"`
	Tools             []geminiTool            `json:"tools,omitempty"`
	GenerationConfig  *geminiGenerationConfig `json:"generationConfig,omitempty"`
}

// geminiContent represents a content message in Gemini format.
type geminiContent struct {
	Role  string       `json:"role"` // "user" or "model"
	Parts []geminiPart `json:"parts"`
}

// geminiPart represents a content part (text, function call, or function response).
type geminiPart map[string]interface{}

// geminiTool represents a tool definition in Gemini format.
type geminiTool struct {
	FunctionDeclarations []geminiFunctionDeclaration `json:"functionDeclarations"`
}

// geminiFunctionDeclaration represents a function declaration in Gemini format.
type geminiFunctionDeclaration struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// geminiGenerationConfig represents generation configuration.
type geminiGenerationConfig struct {
	Temperature     float64 `json:"temperature,omitempty"`
	TopP            float64 `json:"topP,omitempty"`
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
}

// geminiResponse represents the Gemini API response format.
type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []geminiPart `json:"parts"`
			Role  string       `json:"role"`
		} `json:"content"`
		FinishReason  string `json:"finishReason"`
		Index         int    `json:"index"`
		SafetyRatings []struct {
			Category    string `json:"category"`
			Probability string `json:"probability"`
		} `json:"safetyRatings"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
}

// geminiStreamChunk represents a streaming chunk in Gemini format.
type geminiStreamChunk struct {
	Candidates []struct {
		Content struct {
			Parts []geminiPart `json:"parts"`
			Role  string       `json:"role"`
		} `json:"content"`
		FinishReason string `json:"finishReason,omitempty"`
		Index        int    `json:"index"`
	} `json:"candidates"`
	UsageMetadata *struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata,omitempty"`
}

// ToProviderRequest converts a UnifiedRequest to Gemini format.
func (c *GeminiConverter) ToProviderRequest(unified *providers.UnifiedRequest) (interface{}, error) {
	// Extract and merge system messages
	systemMsgs, conversationMsgs := c.ExtractSystemMessages(unified.Messages)
	systemText := c.MergeSystemMessages(systemMsgs)

	req := geminiRequest{}

	// Add system instruction if present
	if systemText != "" {
		req.SystemInstruction = &geminiContent{
			Role: "user",
			Parts: []geminiPart{
				{"text": systemText},
			},
		}
	}

	// Convert messages
	req.Contents = make([]geminiContent, 0, len(conversationMsgs))
	for _, msg := range conversationMsgs {
		// Convert role: "assistant" -> "model", "tool" -> "user"
		role := msg.Role
		if role == "assistant" {
			role = "model"
		} else if role == "tool" {
			role = "user"
		}

		content := geminiContent{
			Role:  role,
			Parts: make([]geminiPart, 0),
		}

		// Add text content
		if msg.Content != "" {
			if msg.ToolCallID != "" {
				// Function response
				content.Parts = append(content.Parts, geminiPart{
					"functionResponse": map[string]interface{}{
						"name": msg.Name,
						"response": map[string]interface{}{
							"content": msg.Content,
						},
					},
				})
			} else {
				// Regular text
				content.Parts = append(content.Parts, geminiPart{
					"text": msg.Content,
				})
			}
		}

		// Add function calls
		for _, tc := range msg.ToolCalls {
			content.Parts = append(content.Parts, geminiPart{
				"functionCall": map[string]interface{}{
					"name": tc.Name,
					"args": tc.Arguments,
				},
			})
		}

		req.Contents = append(req.Contents, content)
	}

	// Convert tools
	if len(unified.Tools) > 0 {
		functionDecls := make([]geminiFunctionDeclaration, len(unified.Tools))
		for i, tool := range unified.Tools {
			functionDecls[i] = geminiFunctionDeclaration{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.Parameters,
			}
		}
		req.Tools = []geminiTool{
			{FunctionDeclarations: functionDecls},
		}
	}

	// Generation config
	if unified.Temperature > 0 || unified.TopP > 0 || unified.MaxTokens > 0 {
		req.GenerationConfig = &geminiGenerationConfig{
			Temperature:     unified.Temperature,
			TopP:            unified.TopP,
			MaxOutputTokens: unified.MaxTokens,
		}
	}

	return req, nil
}

// FromProviderResponse converts a Gemini response to UnifiedResponse.
func (c *GeminiConverter) FromProviderResponse(providerResp interface{}) (*providers.UnifiedResponse, error) {
	// Re-marshal and unmarshal to convert to geminiResponse type
	data, err := json.Marshal(providerResp)
	if err != nil {
		return nil, fmt.Errorf("marshaling provider response: %w", err)
	}

	var resp geminiResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("unmarshaling to Gemini format: %w", err)
	}

	unified := &providers.UnifiedResponse{
		Usage: &providers.UnifiedUsage{
			PromptTokens:     resp.UsageMetadata.PromptTokenCount,
			CompletionTokens: resp.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      resp.UsageMetadata.TotalTokenCount,
		},
	}

	if len(resp.Candidates) == 0 {
		unified.FinishReason = "stop"
		return unified, nil
	}

	candidate := resp.Candidates[0]

	// Convert finish_reason
	switch candidate.FinishReason {
	case "STOP":
		unified.FinishReason = "stop"
	case "MAX_TOKENS":
		unified.FinishReason = "length"
	case "SAFETY":
		unified.FinishReason = "content_filter"
	case "RECITATION":
		unified.FinishReason = "content_filter"
	default:
		unified.FinishReason = strings.ToLower(candidate.FinishReason)
	}

	// Parse parts
	for _, part := range candidate.Content.Parts {
		if text, ok := part["text"].(string); ok {
			unified.Content += text
		}

		if functionCall, ok := part["functionCall"].(map[string]interface{}); ok {
			name, _ := functionCall["name"].(string)
			args, _ := functionCall["args"].(map[string]interface{})

			unified.ToolCalls = append(unified.ToolCalls, providers.UnifiedToolCall{
				ID:        name, // Gemini doesn't provide IDs, use name as ID
				Type:      "function",
				Name:      name,
				Arguments: args,
			})
		}
	}

	// If we have function calls, set finish_reason to tool_calls
	if len(unified.ToolCalls) > 0 {
		unified.FinishReason = "tool_calls"
	}

	return unified, nil
}

// FromProviderStreamChunk converts a Gemini streaming chunk to UnifiedStreamChunk.
func (c *GeminiConverter) FromProviderStreamChunk(rawChunk []byte) (*providers.UnifiedStreamChunk, error) {
	// Gemini uses JSON-lines format for streaming
	line := string(rawChunk)
	line = strings.TrimSpace(line)

	if line == "" {
		return nil, nil
	}

	var chunk geminiStreamChunk
	if err := json.Unmarshal([]byte(line), &chunk); err != nil {
		return nil, fmt.Errorf("unmarshaling stream chunk: %w", err)
	}

	unified := &providers.UnifiedStreamChunk{}

	if chunk.UsageMetadata != nil {
		unified.Usage = &providers.UnifiedUsage{
			PromptTokens:     chunk.UsageMetadata.PromptTokenCount,
			CompletionTokens: chunk.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      chunk.UsageMetadata.TotalTokenCount,
		}
	}

	if len(chunk.Candidates) == 0 {
		return unified, nil
	}

	candidate := chunk.Candidates[0]

	// Convert finish_reason
	if candidate.FinishReason != "" {
		switch candidate.FinishReason {
		case "STOP":
			unified.FinishReason = "stop"
		case "MAX_TOKENS":
			unified.FinishReason = "length"
		case "SAFETY":
			unified.FinishReason = "content_filter"
		case "RECITATION":
			unified.FinishReason = "content_filter"
		default:
			unified.FinishReason = strings.ToLower(candidate.FinishReason)
		}
	}

	// Parse parts for delta
	for _, part := range candidate.Content.Parts {
		if text, ok := part["text"].(string); ok {
			unified.Delta.Content += text
		}

		if functionCall, ok := part["functionCall"].(map[string]interface{}); ok {
			name, _ := functionCall["name"].(string)
			args, _ := functionCall["args"].(map[string]interface{})

			unified.Delta.ToolCalls = append(unified.Delta.ToolCalls, providers.UnifiedToolCall{
				ID:        name,
				Type:      "function",
				Name:      name,
				Arguments: args,
			})
		}
	}

	// If we have function calls, set finish_reason to tool_calls
	if len(unified.Delta.ToolCalls) > 0 && unified.FinishReason == "" {
		unified.FinishReason = "tool_calls"
	}

	return unified, nil
}
