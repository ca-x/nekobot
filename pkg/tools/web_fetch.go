package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	userAgent = "Mozilla/5.0 (compatible; nekobot/1.0)"
)

// WebFetchTool fetches web content and extracts readable text.
type WebFetchTool struct {
	maxChars int
}

// NewWebFetchTool creates a new web fetch tool.
func NewWebFetchTool(maxChars int) *WebFetchTool {
	if maxChars <= 0 {
		maxChars = 50000
	}
	return &WebFetchTool{
		maxChars: maxChars,
	}
}

func (t *WebFetchTool) Name() string {
	return "web_fetch"
}

func (t *WebFetchTool) Description() string {
	return "Fetch content from a URL and extract readable text. Works with HTML pages (converts to text), JSON (formats nicely), and plain text. Use this to read articles, documentation, API responses, or any web content."
}

func (t *WebFetchTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"url": map[string]interface{}{
				"type":        "string",
				"description": "URL to fetch (must be http:// or https://)",
			},
			"max_chars": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum characters to extract (default: 50000)",
				"minimum":     100,
			},
		},
		"required": []string{"url"},
	}
}

func (t *WebFetchTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	urlStr, ok := args["url"].(string)
	if !ok {
		return "", fmt.Errorf("url parameter is required and must be a string")
	}

	// Parse and validate URL
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return "", fmt.Errorf("only http:// and https:// URLs are allowed")
	}

	if parsedURL.Host == "" {
		return "", fmt.Errorf("URL must include a domain name")
	}

	// Get max chars parameter
	maxChars := t.maxChars
	if mc, ok := args["max_chars"].(float64); ok {
		if int(mc) >= 100 {
			maxChars = int(mc)
		}
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/json,text/plain,*/*")

	// Create HTTP client with timeouts and redirect limits
	client := &http.Client{
		Timeout: 60 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        10,
			IdleConnTimeout:     30 * time.Second,
			DisableCompression:  false,
			TLSHandshakeTimeout: 15 * time.Second,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("stopped after 5 redirects")
			}
			return nil
		},
	}

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	// Process content based on Content-Type
	contentType := resp.Header.Get("Content-Type")
	var text string
	var contentFormat string

	if strings.Contains(contentType, "application/json") {
		// Format JSON nicely
		var jsonData interface{}
		if err := json.Unmarshal(body, &jsonData); err == nil {
			formatted, _ := json.MarshalIndent(jsonData, "", "  ")
			text = string(formatted)
			contentFormat = "json"
		} else {
			text = string(body)
			contentFormat = "text"
		}
	} else if strings.Contains(contentType, "text/html") || isHTML(body) {
		// Extract text from HTML
		text = t.extractTextFromHTML(string(body))
		contentFormat = "html"
	} else {
		// Plain text or other
		text = string(body)
		contentFormat = "text"
	}

	// Truncate if needed
	truncated := false
	if len(text) > maxChars {
		text = text[:maxChars]
		truncated = true
	}

	// Build result
	var output strings.Builder
	output.WriteString(fmt.Sprintf("URL: %s\n", urlStr))
	output.WriteString(fmt.Sprintf("Status: %d\n", resp.StatusCode))
	output.WriteString(fmt.Sprintf("Content-Type: %s\n", contentType))
	output.WriteString(fmt.Sprintf("Format: %s\n", contentFormat))
	if truncated {
		output.WriteString(fmt.Sprintf("(Truncated to %d characters)\n", maxChars))
	}
	output.WriteString("\n--- Content ---\n\n")
	output.WriteString(text)

	return output.String(), nil
}

// isHTML checks if content looks like HTML.
func isHTML(content []byte) bool {
	s := string(content)
	lower := strings.ToLower(s)
	return strings.HasPrefix(lower, "<!doctype") ||
		strings.HasPrefix(lower, "<html") ||
		strings.Contains(lower[:min(200, len(lower))], "<html")
}

// extractTextFromHTML extracts readable text from HTML.
func (t *WebFetchTool) extractTextFromHTML(html string) string {
	// Remove script and style tags
	scriptRe := regexp.MustCompile(`(?i)<script[^>]*>.*?</script>`)
	styleRe := regexp.MustCompile(`(?i)<style[^>]*>.*?</style>`)
	html = scriptRe.ReplaceAllString(html, "")
	html = styleRe.ReplaceAllString(html, "")

	// Remove HTML comments
	commentRe := regexp.MustCompile(`<!--.*?-->`)
	html = commentRe.ReplaceAllString(html, "")

	// Replace common block elements with newlines
	blockRe := regexp.MustCompile(`(?i)</(div|p|h[1-6]|li|tr|br|blockquote|pre|section|article|header|footer|nav|aside)>`)
	html = blockRe.ReplaceAllString(html, "\n")

	// Remove all remaining HTML tags
	tagRe := regexp.MustCompile(`<[^>]+>`)
	text := tagRe.ReplaceAllString(html, " ")

	// Decode common HTML entities
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&quot;", "\"")
	text = strings.ReplaceAll(text, "&#39;", "'")
	text = strings.ReplaceAll(text, "&apos;", "'")

	// Clean up whitespace
	// Replace multiple spaces with single space
	spaceRe := regexp.MustCompile(`[ \t]+`)
	text = spaceRe.ReplaceAllString(text, " ")

	// Replace multiple newlines with max 2 newlines
	newlineRe := regexp.MustCompile(`\n\n+`)
	text = newlineRe.ReplaceAllString(text, "\n\n")

	// Trim whitespace from each line
	lines := strings.Split(text, "\n")
	var cleanLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			cleanLines = append(cleanLines, trimmed)
		}
	}

	return strings.Join(cleanLines, "\n")
}

