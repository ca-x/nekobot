package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// WebSearchTool searches the web using Brave Search API.
type WebSearchTool struct {
	apiKey     string
	maxResults int
}

// NewWebSearchTool creates a new web search tool.
func NewWebSearchTool(apiKey string, maxResults int) *WebSearchTool {
	if maxResults <= 0 || maxResults > 10 {
		maxResults = 5
	}
	return &WebSearchTool{
		apiKey:     apiKey,
		maxResults: maxResults,
	}
}

func (t *WebSearchTool) Name() string {
	return "web_search"
}

func (t *WebSearchTool) Description() string {
	return "Search the web for current information using Brave Search. Returns titles, URLs, and descriptions from search results. Use this to find recent information, news, documentation, or any web content."
}

func (t *WebSearchTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Search query",
			},
			"count": map[string]interface{}{
				"type":        "integer",
				"description": "Number of results to return (1-10, default: 5)",
				"minimum":     1,
				"maximum":     10,
			},
		},
		"required": []string{"query"},
	}
}

func (t *WebSearchTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	if t.apiKey == "" {
		return "", fmt.Errorf("web search API key not configured (BRAVE_API_KEY)")
	}

	query, ok := args["query"].(string)
	if !ok {
		return "", fmt.Errorf("query parameter is required and must be a string")
	}

	if query == "" {
		return "", fmt.Errorf("query cannot be empty")
	}

	count := t.maxResults
	if c, ok := args["count"].(float64); ok {
		if int(c) >= 1 && int(c) <= 10 {
			count = int(c)
		}
	}

	// Build Brave Search API URL
	searchURL := fmt.Sprintf("https://api.search.brave.com/res/v1/web/search?q=%s&count=%d",
		url.QueryEscape(query), count)

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Subscription-Token", t.apiKey)

	// Execute request
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("search API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	// Parse response
	var searchResp struct {
		Web struct {
			Results []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
			} `json:"results"`
		} `json:"web"`
	}

	if err := json.Unmarshal(body, &searchResp); err != nil {
		return "", fmt.Errorf("failed to parse search response: %w", err)
	}

	results := searchResp.Web.Results
	if len(results) == 0 {
		return fmt.Sprintf("No results found for query: %s", query), nil
	}

	// Format results
	var output strings.Builder
	output.WriteString(fmt.Sprintf("Search results for: %s\n\n", query))

	for i, result := range results {
		if i >= count {
			break
		}

		output.WriteString(fmt.Sprintf("%d. %s\n", i+1, result.Title))
		output.WriteString(fmt.Sprintf("   URL: %s\n", result.URL))
		if result.Description != "" {
			output.WriteString(fmt.Sprintf("   %s\n", result.Description))
		}
		output.WriteString("\n")
	}

	return output.String(), nil
}
