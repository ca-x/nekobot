// Package converter provides format conversion between different LLM API formats.
// Each provider may have its own request/response format, and these converters
// translate between the provider format and our unified internal format.
package converter

import (
	"nekobot/pkg/providers"
)

// FormatConverter defines the interface for converting between provider-specific
// formats and the unified format.
type FormatConverter interface {
	// ToProviderRequest converts a UnifiedRequest to provider-specific format.
	// Returns the provider-specific request structure.
	ToProviderRequest(unified *providers.UnifiedRequest) (interface{}, error)

	// FromProviderResponse converts a provider-specific response to UnifiedResponse.
	// The input is the unmarshaled provider response structure.
	FromProviderResponse(providerResp interface{}) (*providers.UnifiedResponse, error)

	// FromProviderStreamChunk converts a provider-specific streaming chunk to UnifiedStreamChunk.
	// The input is typically a line from an SSE stream or JSON-lines format.
	FromProviderStreamChunk(rawChunk []byte) (*providers.UnifiedStreamChunk, error)
}

// BaseConverter provides common functionality for format converters.
// Provider-specific converters can embed this to reuse common logic.
type BaseConverter struct{}

// ExtractSystemMessages separates system messages from the conversation history.
// Some providers (like Claude) handle system messages differently.
func (b *BaseConverter) ExtractSystemMessages(messages []providers.UnifiedMessage) ([]providers.UnifiedMessage, []providers.UnifiedMessage) {
	var system, conversation []providers.UnifiedMessage

	for _, msg := range messages {
		if msg.Role == "system" {
			system = append(system, msg)
		} else {
			conversation = append(conversation, msg)
		}
	}

	return system, conversation
}

// MergeSystemMessages combines multiple system messages into one.
func (b *BaseConverter) MergeSystemMessages(messages []providers.UnifiedMessage) string {
	var combined string
	for i, msg := range messages {
		if i > 0 {
			combined += "\n\n"
		}
		combined += msg.Content
	}
	return combined
}

// ConvertToolsToOpenAIFormat converts unified tools to OpenAI tool format.
// This format is widely compatible with many providers.
func (b *BaseConverter) ConvertToolsToOpenAIFormat(tools []providers.UnifiedTool) []map[string]interface{} {
	result := make([]map[string]interface{}, len(tools))
	for i, tool := range tools {
		result[i] = map[string]interface{}{
			"type": tool.Type,
			"function": map[string]interface{}{
				"name":        tool.Name,
				"description": tool.Description,
				"parameters":  tool.Parameters,
			},
		}
	}
	return result
}
