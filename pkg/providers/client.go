// Package providers provides a unified interface for interacting with multiple LLM providers.
package providers

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
)

// Client provides a high-level interface for making LLM API calls.
type Client struct {
	adaptor Adaptor
	info    *RelayInfo
}

// NewClient creates a new provider client.
func NewClient(providerName string, info *RelayInfo) (*Client, error) {
	adaptor, err := GetAdaptor(providerName)
	if err != nil {
		return nil, err
	}

	if err := adaptor.Init(info); err != nil {
		return nil, fmt.Errorf("initializing adaptor: %w", err)
	}

	return &Client{
		adaptor: adaptor,
		info:    info,
	}, nil
}

// Chat performs a non-streaming chat completion request.
func (c *Client) Chat(ctx context.Context, req *UnifiedRequest) (*UnifiedResponse, error) {
	// Convert request
	reqBody, err := c.adaptor.ConvertRequest(req, c.info)
	if err != nil {
		return nil, fmt.Errorf("converting request: %w", err)
	}

	// Get request URL
	url, err := c.adaptor.GetRequestURL(c.info)
	if err != nil {
		return nil, fmt.Errorf("getting request URL: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("creating HTTP request: %w", err)
	}

	// Setup headers
	if err := c.adaptor.SetupRequestHeader(httpReq, c.info); err != nil {
		return nil, fmt.Errorf("setting up request headers: %w", err)
	}

	// Execute request
	respBody, err := c.adaptor.DoRequest(ctx, httpReq)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}

	// Parse response
	resp, err := c.adaptor.DoResponse(respBody, c.info)
	if err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	return resp, nil
}

// ChatStream performs a streaming chat completion request.
func (c *Client) ChatStream(ctx context.Context, req *UnifiedRequest, handler StreamHandler) error {
	// Enable streaming in request
	req.Stream = true

	// Convert request
	reqBody, err := c.adaptor.ConvertRequest(req, c.info)
	if err != nil {
		return fmt.Errorf("converting request: %w", err)
	}

	// Update RelayInfo to indicate streaming
	c.info.Stream = true

	// Get request URL
	url, err := c.adaptor.GetRequestURL(c.info)
	if err != nil {
		return fmt.Errorf("getting request URL: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("creating HTTP request: %w", err)
	}

	// Setup headers
	if err := c.adaptor.SetupRequestHeader(httpReq, c.info); err != nil {
		return fmt.Errorf("setting up request headers: %w", err)
	}

	// Execute streaming request
	client := &http.Client{Timeout: 0}
	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP error: %d", resp.StatusCode)
	}

	// Process stream
	return c.adaptor.DoStreamResponse(ctx, resp.Body, handler, c.info)
}

// GetModelList returns a list of available models for this provider.
func (c *Client) GetModelList() ([]string, error) {
	return c.adaptor.GetModelList()
}

// SimpleStreamHandler is a basic implementation of StreamHandler.
type SimpleStreamHandler struct {
	OnChunkFunc    func(*UnifiedStreamChunk) error
	OnErrorFunc    func(error)
	OnCompleteFunc func(*UnifiedUsage)
}

// OnChunk implements StreamHandler.
func (h *SimpleStreamHandler) OnChunk(chunk *UnifiedStreamChunk) error {
	if h.OnChunkFunc != nil {
		return h.OnChunkFunc(chunk)
	}
	return nil
}

// OnError implements StreamHandler.
func (h *SimpleStreamHandler) OnError(err error) {
	if h.OnErrorFunc != nil {
		h.OnErrorFunc(err)
	}
}

// OnComplete implements StreamHandler.
func (h *SimpleStreamHandler) OnComplete(usage *UnifiedUsage) {
	if h.OnCompleteFunc != nil {
		h.OnCompleteFunc(usage)
	}
}
