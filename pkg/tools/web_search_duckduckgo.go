package tools

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// DuckDuckGoSearchProvider searches via DuckDuckGo HTML endpoint (no API key).
type DuckDuckGoSearchProvider struct {
	client *http.Client
}

func NewDuckDuckGoSearchProvider() *DuckDuckGoSearchProvider {
	return &DuckDuckGoSearchProvider{
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (p *DuckDuckGoSearchProvider) Name() string { return "duckduckgo" }

func (p *DuckDuckGoSearchProvider) Search(ctx context.Context, query string, count int) (string, error) {
	searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(query))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}

	return p.extractResults(string(body), count, query), nil
}

func (p *DuckDuckGoSearchProvider) extractResults(html string, count int, query string) string {
	reLink := regexp.MustCompile(`<a[^>]*class="[^"]*result__a[^"]*"[^>]*href="([^"]+)"[^>]*>([\s\S]*?)</a>`)
	reSnippet := regexp.MustCompile(`<a[^>]*class="[^"]*result__snippet[^"]*"[^>]*>([\s\S]*?)</a>`)

	links := reLink.FindAllStringSubmatch(html, count+8)
	if len(links) == 0 {
		return fmt.Sprintf("No results found for: %s", query)
	}
	snippets := reSnippet.FindAllStringSubmatch(html, count+8)

	maxItems := min(len(links), count)
	var out strings.Builder
	out.WriteString(fmt.Sprintf("Results for: %s (via DuckDuckGo)\n\n", query))

	for i := 0; i < maxItems; i++ {
		rawURL := strings.TrimSpace(links[i][1])
		title := cleanHTMLText(links[i][2])
		finalURL := decodeDuckDuckGoURL(rawURL)

		out.WriteString(fmt.Sprintf("%d. %s\n", i+1, title))
		out.WriteString(fmt.Sprintf("   URL: %s\n", finalURL))
		if i < len(snippets) {
			s := cleanHTMLText(snippets[i][1])
			if s != "" {
				out.WriteString(fmt.Sprintf("   %s\n", s))
			}
		}
		out.WriteString("\n")
	}

	return out.String()
}

func decodeDuckDuckGoURL(raw string) string {
	// Some DDG links are redirect URLs containing uddg target.
	if u, err := url.QueryUnescape(raw); err == nil {
		raw = u
	}
	if idx := strings.Index(raw, "uddg="); idx >= 0 {
		return strings.TrimSpace(raw[idx+5:])
	}
	return strings.TrimSpace(raw)
}

func cleanHTMLText(s string) string {
	tagRe := regexp.MustCompile(`<[^>]+>`)
	s = tagRe.ReplaceAllString(s, "")
	repl := strings.NewReplacer(
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", "\"",
		"&#39;", "'",
		"&nbsp;", " ",
	)
	s = repl.Replace(s)
	return strings.TrimSpace(strings.Join(strings.Fields(s), " "))
}
