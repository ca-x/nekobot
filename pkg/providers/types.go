// Package providers implements a unified provider architecture for LLM APIs.
// It uses an adaptor pattern to abstract away provider-specific details and
// provides format converters for seamless translation between different API formats.
package providers

import (
	"context"
	"io"
	"net/http"
)

// UnifiedRequest represents a provider-agnostic request structure.
// All provider-specific formats are converted to/from this unified format.
type UnifiedRequest struct {
	Model       string                 `json:"model"`
	Messages    []UnifiedMessage       `json:"messages"`
	MaxTokens   int                    `json:"max_tokens,omitempty"`
	Temperature float64                `json:"temperature,omitempty"`
	TopP        float64                `json:"top_p,omitempty"`
	Stream      bool                   `json:"stream,omitempty"`
	Tools       []UnifiedTool          `json:"tools,omitempty"`
	ToolChoice  interface{}            `json:"tool_choice,omitempty"`
	User        string                 `json:"user,omitempty"`
	Extra       map[string]interface{} `json:"-"` // Provider-specific extras
}

// UnifiedMessage represents a single message in the conversation.
type UnifiedMessage struct {
	Role       string                 `json:"role"` // "system", "user", "assistant", "tool"
	Content    string                 `json:"content,omitempty"`
	Name       string                 `json:"name,omitempty"`
	ToolCalls  []UnifiedToolCall      `json:"tool_calls,omitempty"`
	ToolCallID string                 `json:"tool_call_id,omitempty"`
	Metadata   map[string]interface{} `json:"-"` // Provider-specific metadata
}

// UnifiedToolCall represents a tool invocation by the LLM.
type UnifiedToolCall struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type,omitempty"` // Usually "function"
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// UnifiedTool represents a tool definition for the LLM.
type UnifiedTool struct {
	Type        string                 `json:"type"` // Usually "function"
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters"` // JSON Schema
}

// UnifiedResponse represents a provider-agnostic response structure.
type UnifiedResponse struct {
	ID           string              `json:"id"`
	Model        string              `json:"model"`
	Content      string              `json:"content"`
	Thinking     string              `json:"thinking,omitempty"`  // Model's internal reasoning (extended thinking)
	ToolCalls    []UnifiedToolCall   `json:"tool_calls,omitempty"`
	FinishReason string              `json:"finish_reason"` // "stop", "length", "tool_calls", etc.
	Usage        *UnifiedUsage       `json:"usage,omitempty"`
	Extra        map[string]interface{} `json:"-"` // Provider-specific extras
}

// UnifiedStreamChunk represents a single chunk in a streaming response.
type UnifiedStreamChunk struct {
	ID           string            `json:"id"`
	Model        string            `json:"model"`
	Delta        UnifiedDelta      `json:"delta"`
	FinishReason string            `json:"finish_reason,omitempty"`
	Usage        *UnifiedUsage     `json:"usage,omitempty"`
	Extra        map[string]interface{} `json:"-"`
}

// UnifiedDelta represents the incremental changes in a streaming chunk.
type UnifiedDelta struct {
	Role      string            `json:"role,omitempty"`
	Content   string            `json:"content,omitempty"`
	Thinking  string            `json:"thinking,omitempty"` // Incremental thinking content
	ToolCalls []UnifiedToolCall `json:"tool_calls,omitempty"`
}

// UnifiedUsage represents token usage information.
type UnifiedUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// RelayInfo contains metadata about the current request being processed.
// It's used to pass context through the adaptor pipeline.
type RelayInfo struct {
	RequestID     string                 // Unique request identifier
	ProviderName  string                 // Name of the provider (e.g., "openai", "claude")
	APIKey        string                 // API key for authentication
	APIBase       string                 // Base URL for API endpoints
	Model         string                 // Model identifier
	Stream        bool                   // Whether streaming is enabled
	MaxRetries    int                    // Maximum retry attempts
	Timeout       int                    // Timeout in seconds
	Proxy         string                 // HTTP proxy URL
	Headers       map[string]string      // Additional HTTP headers
	Metadata      map[string]interface{} // Additional metadata
}

// StreamHandler is a callback interface for processing streaming responses.
type StreamHandler interface {
	// OnChunk is called for each chunk received in the stream.
	OnChunk(chunk *UnifiedStreamChunk) error

	// OnError is called when an error occurs during streaming.
	OnError(err error)

	// OnComplete is called when the stream completes successfully.
	OnComplete(usage *UnifiedUsage)
}

// ErrorResponse represents a provider error response.
type ErrorResponse struct {
	StatusCode int                    `json:"status_code"`
	Message    string                 `json:"message"`
	Type       string                 `json:"type,omitempty"`
	Code       string                 `json:"code,omitempty"`
	Extra      map[string]interface{} `json:"-"`
}

// Error implements the error interface.
func (e *ErrorResponse) Error() string {
	if e.Code != "" {
		return e.Code + ": " + e.Message
	}
	return e.Message
}

// Adaptor defines the interface that all provider adaptors must implement.
// Each provider (OpenAI, Claude, Gemini, etc.) has its own adaptor implementation.
type Adaptor interface {
	// Init initializes the adaptor with the given RelayInfo.
	Init(info *RelayInfo) error

	// GetRequestURL returns the full URL for the API request.
	GetRequestURL(info *RelayInfo) (string, error)

	// SetupRequestHeader sets up HTTP headers for the request.
	SetupRequestHeader(req *http.Request, info *RelayInfo) error

	// ConvertRequest converts a UnifiedRequest to provider-specific format.
	// Returns the marshaled JSON bytes ready to send.
	ConvertRequest(unified *UnifiedRequest, info *RelayInfo) ([]byte, error)

	// DoRequest performs the HTTP request and returns the raw response body.
	// This is used for non-streaming requests.
	DoRequest(ctx context.Context, req *http.Request) ([]byte, error)

	// DoResponse parses the provider-specific response into UnifiedResponse.
	DoResponse(body []byte, info *RelayInfo) (*UnifiedResponse, error)

	// DoStreamResponse handles streaming responses.
	// It reads from the response body and calls the handler for each chunk.
	DoStreamResponse(ctx context.Context, reader io.Reader, handler StreamHandler, info *RelayInfo) error

	// GetModelList returns a list of available models for this provider (optional).
	GetModelList() ([]string, error)
}
