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

// BraveSearchProvider searches via Brave Search API.
type BraveSearchProvider struct {
	apiKey string
	client *http.Client
}

func NewBraveSearchProvider(apiKey string) *BraveSearchProvider {
	return &BraveSearchProvider{
		apiKey: apiKey,
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (p *BraveSearchProvider) Name() string { return "brave" }

type braveResponse struct {
	Web struct {
		Results []struct {
			Title       string `json:"title"`
			URL         string `json:"url"`
			Description string `json:"description"`
		} `json:"results"`
	} `json:"web"`
}

func (p *BraveSearchProvider) Search(ctx context.Context, query string, count int) (string, error) {
	searchURL := fmt.Sprintf("https://api.search.brave.com/res/v1/web/search?q=%s&count=%d",
		url.QueryEscape(query), count)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Subscription-Token", p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API status %d: %s", resp.StatusCode, string(body))
	}

	var parsed braveResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	if len(parsed.Web.Results) == 0 {
		return fmt.Sprintf("No results found for: %s", query), nil
	}

	var out strings.Builder
	out.WriteString(fmt.Sprintf("Results for: %s (via Brave Search)\n\n", query))
	for i, item := range parsed.Web.Results {
		if i >= count {
			break
		}
		out.WriteString(fmt.Sprintf("%d. %s\n", i+1, strings.TrimSpace(item.Title)))
		out.WriteString(fmt.Sprintf("   URL: %s\n", strings.TrimSpace(item.URL)))
		desc := strings.TrimSpace(item.Description)
		if desc != "" {
			out.WriteString(fmt.Sprintf("   %s\n", desc))
		}
		out.WriteString("\n")
	}
	return out.String(), nil
}
