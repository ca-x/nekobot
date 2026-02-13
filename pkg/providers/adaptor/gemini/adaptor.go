// Package gemini provides the Google Gemini API adaptor implementation.
package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"nekobot/pkg/providers"
	"nekobot/pkg/providers/converter"
	"nekobot/pkg/providers/streaming"
)

// Adaptor implements the providers.Adaptor interface for Gemini API.
type Adaptor struct {
	converter  *converter.GeminiConverter
	httpClient *http.Client
}

// New creates a new Gemini adaptor instance.
func New() *Adaptor {
	return &Adaptor{
		converter: converter.NewGeminiConverter(),
		httpClient: &http.Client{
			Timeout: 0, // No timeout, we handle it per-request
		},
	}
}

// Init initializes the adaptor with the given RelayInfo.
func (a *Adaptor) Init(info *providers.RelayInfo) error {
	if info.APIKey == "" {
		return fmt.Errorf("API key is required for Gemini")
	}

	if info.APIBase == "" {
		info.APIBase = "https://generativelanguage.googleapis.com/v1beta"
	}

	return nil
}

// GetRequestURL returns the full URL for the API request.
func (a *Adaptor) GetRequestURL(info *providers.RelayInfo) (string, error) {
	baseURL := info.APIBase
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com/v1beta"
	}

	model := info.Model
	if model == "" {
		return "", fmt.Errorf("model is required for Gemini")
	}

	// Remove provider prefix if present (e.g., "google/gemini-pro" -> "gemini-pro")
	if idx := strings.Index(model, "/"); idx != -1 {
		model = model[idx+1:]
	}

	// Construct URL based on streaming or not
	if info.Stream {
		return fmt.Sprintf("%s/models/%s:streamGenerateContent?key=%s", baseURL, model, info.APIKey), nil
	}

	return fmt.Sprintf("%s/models/%s:generateContent?key=%s", baseURL, model, info.APIKey), nil
}

// SetupRequestHeader sets up HTTP headers for the request.
func (a *Adaptor) SetupRequestHeader(req *http.Request, info *providers.RelayInfo) error {
	req.Header.Set("Content-Type", "application/json")

	// Gemini uses API key in URL, but we also set it in header for consistency
	req.Header.Set("x-goog-api-key", info.APIKey)

	// Add custom headers if provided
	for key, value := range info.Headers {
		req.Header.Set(key, value)
	}

	return nil
}

// ConvertRequest converts a UnifiedRequest to provider-specific format.
func (a *Adaptor) ConvertRequest(unified *providers.UnifiedRequest, info *providers.RelayInfo) ([]byte, error) {
	// Use converter to transform to Gemini format
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
	// Create stream processor for JSON-lines format (Gemini uses JSON-lines)
	processor := streaming.NewStreamProcessor(ctx, reader, streaming.FormatJSONLines)

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
			// Some chunks might not be valid, skip them
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
				// Update with latest usage info
				totalUsage = unified.Usage
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
	// Return a static list of Gemini models
	return []string{
		"gemini-2.0-flash-exp",
		"gemini-2.0-flash-001",
		"gemini-1.5-pro-latest",
		"gemini-1.5-pro-001",
		"gemini-1.5-flash-latest",
		"gemini-1.5-flash-001",
		"gemini-1.0-pro",
		"gemini-1.0-pro-vision",
	}, nil
}

// parseError parses a Gemini API error response.
func parseError(statusCode int, body []byte) error {
	var errResp struct {
		Error struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Status  string `json:"status"`
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
		Type:       errResp.Error.Status,
		Code:       fmt.Sprintf("%d", errResp.Error.Code),
	}
}

// init registers the Gemini adaptor with the global registry.
func init() {
	providers.Register("gemini", func() providers.Adaptor {
		return New()
	})
	// Also register under "google" alias
	providers.Register("google", func() providers.Adaptor {
		return New()
	})
}
