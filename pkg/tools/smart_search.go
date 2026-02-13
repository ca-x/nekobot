package tools

import (
	"context"
	"fmt"
	"strings"

	"nekobot/pkg/logger"
)

// SmartSearchTool provides intelligent web search with fallback.
type SmartSearchTool struct {
	log         *logger.Logger
	webSearch   *WebSearchTool
	webFetch    *WebFetchTool
	browser     *BrowserTool
	hasWebAPI   bool
	hasBrowser  bool
}

// NewSmartSearchTool creates a new smart search tool.
func NewSmartSearchTool(
	log *logger.Logger,
	webSearch *WebSearchTool,
	webFetch *WebFetchTool,
	browser *BrowserTool,
) *SmartSearchTool {
	return &SmartSearchTool{
		log:        log,
		webSearch:  webSearch,
		webFetch:   webFetch,
		browser:    browser,
		hasWebAPI:  webSearch != nil,
		hasBrowser: browser != nil,
	}
}

// Name returns the tool name.
func (t *SmartSearchTool) Name() string {
	return "smart_search"
}

// Description returns the tool description.
func (t *SmartSearchTool) Description() string {
	return `Intelligent web search that automatically chooses the best method:
1. Try web_search API first (fast, structured results)
2. Fallback to browser-based search if API unavailable
3. Can fetch and process search result pages

Use this for comprehensive web searches when you need reliable results.`
}

// Parameters returns the tool parameters schema.
func (t *SmartSearchTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Search query",
			},
			"fetch_first": map[string]interface{}{
				"type":        "boolean",
				"description": "Fetch and extract content from the first result (default: false)",
			},
			"max_results": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of results (default: 5)",
			},
		},
		"required": []string{"query"},
	}
}

// Execute executes the smart search.
func (t *SmartSearchTool) Execute(ctx context.Context, params map[string]interface{}) (string, error) {
	query, ok := params["query"].(string)
	if !ok || query == "" {
		return "", fmt.Errorf("query parameter is required")
	}

	fetchFirst := false
	if f, ok := params["fetch_first"].(bool); ok {
		fetchFirst = f
	}

	maxResults := 5
	if mr, ok := params["max_results"].(float64); ok {
		maxResults = int(mr)
	}

	t.log.Info("Smart search",
		logger.String("query", query),
		logger.Bool("fetch_first", fetchFirst))

	// Try web_search API first
	if t.hasWebAPI {
		result, err := t.searchWithAPI(ctx, query, maxResults)
		if err == nil && t.isValidResult(result) {
			t.log.Info("Web API search successful")

			// Optionally fetch first result
			if fetchFirst && t.webFetch != nil {
				firstURL := t.extractFirstURL(result)
				if firstURL != "" {
					content, err := t.webFetch.Execute(ctx, map[string]interface{}{
						"url": firstURL,
					})
					if err == nil {
						return fmt.Sprintf("Search Results:\n%s\n\n---\n\nFirst Result Content:\n%s", result, content), nil
					}
				}
			}

			return result, nil
		}

		t.log.Warn("Web API search failed or invalid, falling back",
			logger.Error(err))
	}

	// Fallback to browser-based search
	if t.hasBrowser {
		t.log.Info("Using browser fallback")
		return t.searchWithBrowser(ctx, query, maxResults, fetchFirst)
	}

	return "", fmt.Errorf("no search method available")
}

// searchWithAPI performs search using the web search API.
func (t *SmartSearchTool) searchWithAPI(ctx context.Context, query string, maxResults int) (string, error) {
	if t.webSearch == nil {
		return "", fmt.Errorf("web search not available")
	}

	return t.webSearch.Execute(ctx, map[string]interface{}{
		"query": query,
		"count": maxResults,
	})
}

// searchWithBrowser performs search using browser automation.
func (t *SmartSearchTool) searchWithBrowser(ctx context.Context, query string, maxResults int, fetchFirst bool) (string, error) {
	if t.browser == nil {
		return "", fmt.Errorf("browser not available")
	}

	// Navigate to Google
	searchURL := fmt.Sprintf("https://www.google.com/search?q=%s", strings.ReplaceAll(query, " ", "+"))

	if _, err := t.browser.Execute(ctx, map[string]interface{}{
		"action": "navigate",
		"url":    searchURL,
	}); err != nil {
		return "", fmt.Errorf("failed to navigate: %w", err)
	}

	// Wait for results to load
	t.browser.Execute(ctx, map[string]interface{}{
		"action":   "wait",
		"duration": 2000,
	})

	// Extract page HTML
	html, err := t.browser.Execute(ctx, map[string]interface{}{
		"action": "get_html",
	})
	if err != nil {
		return "", fmt.Errorf("failed to get HTML: %w", err)
	}

	// Parse search results from HTML (simplified)
	results := t.parseGoogleResults(html)

	if fetchFirst && len(results) > 0 && t.webFetch != nil {
		firstURL := results[0]["url"]
		if firstURL != "" {
			content, err := t.webFetch.Execute(ctx, map[string]interface{}{
				"url": firstURL,
			})
			if err == nil {
				return fmt.Sprintf("Search Results:\n%s\n\n---\n\nFirst Result Content:\n%s",
					t.formatResults(results), content), nil
			}
		}
	}

	return t.formatResults(results), nil
}

// isValidResult checks if a search result is valid.
func (t *SmartSearchTool) isValidResult(result string) bool {
	if result == "" {
		return false
	}

	// Check for warning messages
	if strings.Contains(result, "[Warning:") {
		return false
	}

	// Check for mock results
	if strings.Contains(result, "Mock") {
		return false
	}

	// Check if result contains actual URLs or titles
	if strings.Contains(result, "http") || strings.Contains(result, "Title:") {
		return true
	}

	// Result too short and no URLs
	if len(result) < 50 && !strings.Contains(result, "http") {
		return false
	}

	return true
}

// extractFirstURL extracts the first URL from search results.
func (t *SmartSearchTool) extractFirstURL(result string) string {
	lines := strings.Split(result, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "URL:") {
			url := strings.TrimPrefix(line, "URL:")
			return strings.TrimSpace(url)
		}
		// Also try to find http/https URLs
		if strings.Contains(line, "http://") || strings.Contains(line, "https://") {
			start := strings.Index(line, "http")
			end := strings.IndexAny(line[start:], " \t\n")
			if end == -1 {
				return line[start:]
			}
			return line[start : start+end]
		}
	}
	return ""
}

// parseGoogleResults parses search results from Google HTML (simplified).
func (t *SmartSearchTool) parseGoogleResults(html string) []map[string]string {
	// This is a simplified parser. In production, use proper HTML parsing.
	results := []map[string]string{}

	// Look for common Google result patterns
	// This is a placeholder - real implementation would use goquery or similar
	lines := strings.Split(html, "\n")
	for _, line := range lines {
		if strings.Contains(line, "<a href=\"/url?q=") {
			// Extract URL and title (simplified)
			result := map[string]string{
				"title": "Search Result",
				"url":   "",
			}
			results = append(results, result)

			if len(results) >= 5 {
				break
			}
		}
	}

	return results
}

// formatResults formats search results as text.
func (t *SmartSearchTool) formatResults(results []map[string]string) string {
	if len(results) == 0 {
		return "No results found"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d results:\n\n", len(results)))

	for i, result := range results {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, result["title"]))
		if result["url"] != "" {
			sb.WriteString(fmt.Sprintf("   URL: %s\n", result["url"]))
		}
		if result["description"] != "" {
			sb.WriteString(fmt.Sprintf("   %s\n", result["description"]))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
