package providers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var htmlStatusPattern = regexp.MustCompile(`\b([45]\d{2})\b`)

// UnmarshalJSONResponse decodes a provider response body and upgrades common
// proxy / gateway failures into clearer errors before JSON parsing.
func UnmarshalJSONResponse(body []byte, dst any) error {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return fmt.Errorf("provider returned empty response body")
	}

	if trimmed[0] == '<' {
		snippet := responseSnippet(trimmed)
		if status := extractStatusFromSnippet(snippet); status > 0 {
			return fmt.Errorf(
				"provider returned HTML error page instead of JSON (status %d): %s",
				status,
				snippet,
			)
		}
		return fmt.Errorf("provider returned HTML error page instead of JSON: %s", snippet)
	}

	if trimmed[0] != '{' && trimmed[0] != '[' {
		return fmt.Errorf("provider returned non-JSON response: %s", responseSnippet(trimmed))
	}

	if err := json.Unmarshal(trimmed, dst); err != nil {
		return fmt.Errorf("invalid JSON response: %w", err)
	}

	return nil
}

func responseSnippet(body []byte) string {
	const maxLen = 160

	snippet := strings.Join(strings.Fields(string(body)), " ")
	if len(snippet) <= maxLen {
		return snippet
	}
	return snippet[:maxLen] + "..."
}

func extractStatusFromSnippet(snippet string) int {
	match := htmlStatusPattern.FindStringSubmatch(snippet)
	if len(match) != 2 {
		return 0
	}
	return parseDigits(match[1])
}
