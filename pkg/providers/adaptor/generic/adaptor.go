// Package generic provides a generic adaptor for OpenAI-compatible APIs.
// This is used for providers like OpenRouter, vLLM, Groq, etc. that follow
// the OpenAI API format but with different base URLs.
package generic

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

// Adaptor implements the providers.Adaptor interface for generic OpenAI-compatible APIs.
type Adaptor struct {
	converter  *converter.OpenAIConverter
	httpClient *http.Client
}

// New creates a new generic adaptor instance.
func New() *Adaptor {
	return &Adaptor{
		converter: converter.NewOpenAIConverter(),
		httpClient: &http.Client{
			Timeout: 0, // No timeout, we handle it per-request
		},
	}
}

// Init initializes the adaptor with the given RelayInfo.
func (a *Adaptor) Init(info *providers.RelayInfo) error {
	if info.APIBase == "" {
		return fmt.Errorf("API base URL is required for generic provider")
	}

	// Setup HTTP client with proxy if provided
	client, err := providers.NewHTTPClientWithProxy(info.Proxy)
	if err != nil {
		return fmt.Errorf("setting up proxy: %w", err)
	}
	a.httpClient = client

	// API key is optional for some self-hosted services
	return nil
}

// GetRequestURL returns the full URL for the API request.
func (a *Adaptor) GetRequestURL(info *providers.RelayInfo) (string, error) {
	baseURL := info.APIBase
	if baseURL == "" {
		return "", fmt.Errorf("API base URL is required")
	}

	// Generic OpenAI-compatible chat completions endpoint
	return baseURL + "/chat/completions", nil
}

// SetupRequestHeader sets up HTTP headers for the request.
func (a *Adaptor) SetupRequestHeader(req *http.Request, info *providers.RelayInfo) error {
	req.Header.Set("Content-Type", "application/json")

	// Add authorization header if API key is provided
	if info.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+info.APIKey)
	}

	// Add custom headers if provided
	for key, value := range info.Headers {
		req.Header.Set(key, value)
	}

	return nil
}

// ConvertRequest converts a UnifiedRequest to provider-specific format.
func (a *Adaptor) ConvertRequest(unified *providers.UnifiedRequest, info *providers.RelayInfo) ([]byte, error) {
	// Use OpenAI converter since this is OpenAI-compatible
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

	// Use OpenAI converter since this is OpenAI-compatible
	unified, err := a.converter.FromProviderResponse(providerResp)
	if err != nil {
		return nil, fmt.Errorf("converting response: %w", err)
	}

	return unified, nil
}

// DoStreamResponse handles streaming responses.
func (a *Adaptor) DoStreamResponse(ctx context.Context, reader io.Reader, handler providers.StreamHandler, info *providers.RelayInfo) error {
	// Create stream processor for SSE format (OpenAI-compatible uses SSE)
	processor := streaming.NewStreamProcessor(ctx, reader, streaming.FormatSSE)

	// Set timeout if provided
	if info.Timeout > 0 {
		processor.SetTimeout(time.Duration(info.Timeout) * time.Second)
	}

	// Process each chunk
	err := processor.ProcessStream(func(chunk []byte) error {
		// Convert chunk to unified format
		unified, err := a.converter.FromProviderStreamChunk(chunk)
		if err != nil {
			// Some chunks might not be valid (e.g., keep-alive), skip them
			return nil
		}

		if unified == nil {
			// Stream termination marker
			return nil
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

	handler.OnComplete(nil)
	return nil
}

// GetModelList returns a list of available models for this provider.
func (a *Adaptor) GetModelList() ([]string, error) {
	// Cannot determine models for generic provider without API call
	return []string{}, nil
}

// parseError parses an OpenAI-compatible API error response.
func parseError(statusCode int, body []byte) error {
	var errResp struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
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
		Code:       errResp.Error.Code,
	}
}

// init registers common aliases for OpenAI-compatible providers.
func init() {
	// Register generic adaptor under various provider names
	factory := func() providers.Adaptor {
		return New()
	}

	providers.Register("generic", factory)
	providers.Register("openrouter", factory)
	providers.Register("groq", factory)
	providers.Register("vllm", factory)
	providers.Register("together", factory)
	providers.Register("perplexity", factory)
	providers.Register("deepseek", factory)
	providers.Register("moonshot", factory)
	providers.Register("zhipu", factory)
	providers.Register("nvidia", factory)
}
