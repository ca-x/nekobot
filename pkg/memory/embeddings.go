package memory

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OpenAIEmbeddingProvider provides embeddings using OpenAI's API.
type OpenAIEmbeddingProvider struct {
	apiKey     string
	model      string
	httpClient *http.Client
	dimension  int
	maxBatch   int
}

// NewOpenAIEmbeddingProvider creates a new OpenAI embedding provider.
func NewOpenAIEmbeddingProvider(apiKey, model string) *OpenAIEmbeddingProvider {
	if model == "" {
		model = "text-embedding-3-small" // Default model
	}

	dimension := 1536 // text-embedding-3-small dimension
	if model == "text-embedding-3-large" {
		dimension = 3072
	} else if model == "text-embedding-ada-002" {
		dimension = 1536
	}

	return &OpenAIEmbeddingProvider{
		apiKey: apiKey,
		model:  model,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		dimension: dimension,
		maxBatch:  2048, // OpenAI limit
	}
}

// Embed generates a single embedding.
func (p *OpenAIEmbeddingProvider) Embed(text string) ([]float32, error) {
	embeddings, err := p.EmbedBatch([]string{text})
	if err != nil {
		return nil, err
	}

	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}

	return embeddings[0], nil
}

// EmbedBatch generates multiple embeddings in one API call.
func (p *OpenAIEmbeddingProvider) EmbedBatch(texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	if len(texts) > p.maxBatch {
		return nil, fmt.Errorf("batch size %d exceeds maximum %d", len(texts), p.maxBatch)
	}

	// Prepare request
	reqBody := map[string]interface{}{
		"model": p.model,
		"input": texts,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", "https://api.openai.com/v1/embeddings", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	// Send request
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
			Index     int       `json:"index"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Extract embeddings in order
	embeddings := make([][]float32, len(texts))
	for _, item := range result.Data {
		if item.Index < len(embeddings) {
			embeddings[item.Index] = item.Embedding
		}
	}

	return embeddings, nil
}

// Dimension returns the dimension of embeddings.
func (p *OpenAIEmbeddingProvider) Dimension() int {
	return p.dimension
}

// MaxBatchSize returns the maximum batch size.
func (p *OpenAIEmbeddingProvider) MaxBatchSize() int {
	return p.maxBatch
}

// SimpleEmbeddingProvider provides simple hash-based embeddings for testing.
// This is NOT suitable for production use - use OpenAI or other real embedding models.
type SimpleEmbeddingProvider struct {
	dimension int
}

// NewSimpleEmbeddingProvider creates a simple embedding provider for testing.
func NewSimpleEmbeddingProvider(dimension int) *SimpleEmbeddingProvider {
	if dimension <= 0 {
		dimension = 384 // Default dimension
	}

	return &SimpleEmbeddingProvider{
		dimension: dimension,
	}
}

// Embed generates a simple hash-based embedding (for testing only).
func (p *SimpleEmbeddingProvider) Embed(text string) ([]float32, error) {
	embeddings, err := p.EmbedBatch([]string{text})
	if err != nil {
		return nil, err
	}
	return embeddings[0], nil
}

// EmbedBatch generates simple hash-based embeddings (for testing only).
func (p *SimpleEmbeddingProvider) EmbedBatch(texts []string) ([][]float32, error) {
	embeddings := make([][]float32, len(texts))

	for i, text := range texts {
		// Simple hash-based embedding (NOT semantic!)
		embedding := make([]float32, p.dimension)

		// Use characters to generate deterministic but meaningless vectors
		for j := 0; j < len(text) && j < p.dimension; j++ {
			embedding[j%p.dimension] += float32(text[j]) / 255.0
		}

		// Normalize
		var norm float32
		for _, val := range embedding {
			norm += val * val
		}
		if norm > 0 {
			norm = float32(1.0 / float32(norm))
			for j := range embedding {
				embedding[j] *= norm
			}
		}

		embeddings[i] = embedding
	}

	return embeddings, nil
}

// Dimension returns the dimension of embeddings.
func (p *SimpleEmbeddingProvider) Dimension() int {
	return p.dimension
}

// MaxBatchSize returns the maximum batch size.
func (p *SimpleEmbeddingProvider) MaxBatchSize() int {
	return 100
}
