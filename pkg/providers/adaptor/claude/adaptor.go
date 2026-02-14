// Package claude provides the Claude (Anthropic) API adaptor implementation.
package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"nekobot/pkg/providers"
	"nekobot/pkg/providers/converter"
	"nekobot/pkg/providers/streaming"
)

// Adaptor implements the providers.Adaptor interface for Claude API.
type Adaptor struct {
	converter  *converter.ClaudeConverter
	httpClient *http.Client
}

// New creates a new Claude adaptor instance.
func New() *Adaptor {
	return &Adaptor{
		converter: converter.NewClaudeConverter(),
		httpClient: &http.Client{
			Timeout: 0, // No timeout, we handle it per-request
		},
	}
}

// Init initializes the adaptor with the given RelayInfo.
func (a *Adaptor) Init(info *providers.RelayInfo) error {
	if info.APIKey == "" {
		return fmt.Errorf("API key is required for Claude")
	}

	if info.APIBase == "" {
		info.APIBase = "https://api.anthropic.com/v1"
	}

	// Setup HTTP client with proxy if provided
	client, err := providers.NewHTTPClientWithProxy(info.Proxy)
	if err != nil {
		return fmt.Errorf("setting up proxy: %w", err)
	}
	a.httpClient = client

	return nil
}

// GetRequestURL returns the full URL for the API request.
func (a *Adaptor) GetRequestURL(info *providers.RelayInfo) (string, error) {
	baseURL := info.APIBase
	if baseURL == "" {
		baseURL = "https://api.anthropic.com/v1"
	}

	// Claude messages endpoint
	return baseURL + "/messages", nil
}

// SetupRequestHeader sets up HTTP headers for the request.
func (a *Adaptor) SetupRequestHeader(req *http.Request, info *providers.RelayInfo) error {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", info.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	// Add custom headers if provided
	for key, value := range info.Headers {
		req.Header.Set(key, value)
	}

	return nil
}

// ConvertRequest converts a UnifiedRequest to provider-specific format.
func (a *Adaptor) ConvertRequest(unified *providers.UnifiedRequest, info *providers.RelayInfo) ([]byte, error) {
	// Use converter to transform to Claude format
	providerReq, err := a.converter.ToProviderRequest(unified)
	if err != nil {
		return nil, fmt.Errorf("converting request: %w", err)
	}

	// Marshal to JSON
	data, err := json.Marshal(providerReq)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	return data, nil
}

// DoRequest performs the HTTP request and returns the raw response body.
func (a *Adaptor) DoRequest(ctx context.Context, req *http.Request) ([]byte, error) {
	// Create a new context with timeout if specified
	if deadline, ok := ctx.Deadline(); ok {
		timeout := time.Until(deadline)
		client := &http.Client{Timeout: timeout}
		resp, err := client.Do(req.WithContext(ctx))
		if err != nil {
			return nil, fmt.Errorf("executing request: %w", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("reading response body: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			return nil, parseError(resp.StatusCode, body)
		}

		return body, nil
	}

	// No timeout specified, use default client
	resp, err := a.httpClient.Do(req.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, parseError(resp.StatusCode, body)
	}

	return body, nil
}

// DoResponse parses the provider-specific response into UnifiedResponse.
func (a *Adaptor) DoResponse(body []byte, info *providers.RelayInfo) (*providers.UnifiedResponse, error) {
	// Unmarshal the response
	var providerResp interface{}
	if err := json.Unmarshal(body, &providerResp); err != nil {
		return nil, fmt.Errorf("unmarshaling response: %w", err)
	}

	// Use converter to transform to unified format
	unified, err := a.converter.FromProviderResponse(providerResp)
	if err != nil {
		return nil, fmt.Errorf("converting response: %w", err)
	}

	return unified, nil
}

// DoStreamResponse handles streaming responses.
func (a *Adaptor) DoStreamResponse(ctx context.Context, reader io.Reader, handler providers.StreamHandler, info *providers.RelayInfo) error {
	// Create stream processor for SSE format (Claude uses SSE)
	processor := streaming.NewStreamProcessor(ctx, reader, streaming.FormatSSE)

	// Set timeout if provided
	if info.Timeout > 0 {
		processor.SetTimeout(time.Duration(info.Timeout) * time.Second)
	}

	// Accumulate usage information across chunks
	var totalUsage *providers.UnifiedUsage

	// Process each chunk
	err := processor.ProcessStream(func(chunk []byte) error {
		// Convert chunk to unified format
		unified, err := a.converter.FromProviderStreamChunk(chunk)
		if err != nil {
			// Some chunks might not be valid (e.g., ping events), skip them
			return nil
		}

		if unified == nil {
			// Stream termination marker
			return nil
		}

		// Accumulate usage
		if unified.Usage != nil {
			if totalUsage == nil {
				totalUsage = unified.Usage
			} else {
				totalUsage.CompletionTokens += unified.Usage.CompletionTokens
				totalUsage.PromptTokens = unified.Usage.PromptTokens
				totalUsage.TotalTokens = totalUsage.PromptTokens + totalUsage.CompletionTokens
			}
		}

		// Call handler
		if err := handler.OnChunk(unified); err != nil {
			return fmt.Errorf("handler error: %w", err)
		}

		return nil
	})

	if err != nil {
		handler.OnError(err)
		return err
	}

	handler.OnComplete(totalUsage)
	return nil
}

// GetModelList returns a list of available models for this provider.
func (a *Adaptor) GetModelList() ([]string, error) {
	// Return a static list of Claude models
	return []string{
		"claude-opus-4-6",
		"claude-sonnet-4-5-20250929",
		"claude-sonnet-4-20250514",
		"claude-sonnet-3-7-20250219",
		"claude-3-7-sonnet-20250219",
		"claude-3-5-sonnet-20241022",
		"claude-3-5-sonnet-20240620",
		"claude-3-opus-20240229",
		"claude-3-sonnet-20240229",
		"claude-3-haiku-20240307",
	}, nil
}

// parseError parses a Claude API error response.
func parseError(statusCode int, body []byte) error {
	var errResp struct {
		Type  string `json:"type"`
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &errResp); err != nil {
		return &providers.ErrorResponse{
			StatusCode: statusCode,
			Message:    string(body),
		}
	}

	return &providers.ErrorResponse{
		StatusCode: statusCode,
		Message:    errResp.Error.Message,
		Type:       errResp.Error.Type,
	}
}

// init registers the Claude adaptor with the global registry.
func init() {
	providers.Register("claude", func() providers.Adaptor {
		return New()
	})
	// Also register under "anthropic" alias
	providers.Register("anthropic", func() providers.Adaptor {
		return New()
	})
}
