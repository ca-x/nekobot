package criteria

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"nekobot/pkg/goaldriven/shared"
)

// Parser turns natural-language criteria into a draft structured set.
type Parser struct{}

// NewParser creates a criteria parser.
func NewParser() *Parser { return &Parser{} }

// ParseInput is the parser request.
type ParseInput struct {
	Goal      string
	Natural   string
	Scope     *shared.ExecutionScope
	RiskLevel shared.RiskLevel
}

// Parse builds a conservative first-slice draft criteria set.
func (p *Parser) Parse(_ context.Context, in ParseInput) (Set, error) {
	if strings.TrimSpace(in.Goal) == "" {
		return Set{}, fmt.Errorf("goal is required")
	}
	if strings.TrimSpace(in.Natural) == "" {
		return Set{}, fmt.Errorf("natural criteria is required")
	}
	if in.Scope == nil {
		return Set{}, fmt.Errorf("scope is required")
	}

	items := make([]Item, 0, 4)
	for idx, clause := range splitNaturalCriteria(in.Natural) {
		inferred := inferCriterion(clause, *in.Scope, idx+1)
		if inferred != nil {
			items = append(items, *inferred)
		}
	}

	if len(items) == 0 {
		items = append(items, Item{
			ID:       "manual-confirmation-1",
			Title:    "Operator confirms the declared success criteria are met",
			Type:     TypeManualConfirmation,
			Scope:    *in.Scope,
			Required: true,
			Status:   StatusPending,
			Definition: map[string]any{
				"prompt": strings.TrimSpace(in.Natural),
			},
			UpdatedAt: time.Now().UTC(),
		})
	}

	return Set{Criteria: items}, nil
}

func inferCriterion(clause string, scope shared.ExecutionScope, ordinal int) *Item {
	now := time.Now().UTC()

	if targetURL, statusCode, bodyContains := inferHTTPCheck(clause); targetURL != "" {
		definition := map[string]any{
			"url": targetURL,
		}
		title := "HTTP success check"
		if statusCode != 0 {
			definition["expect_status"] = statusCode
			title = "HTTP status check"
		}
		if bodyContains != "" {
			definition["body_contains"] = bodyContains
			title = "HTTP response content check"
		}
		return &Item{
			ID:         fmt.Sprintf("http-check-%d", ordinal),
			Title:      title,
			Type:       TypeHTTPCheck,
			Scope:      scope,
			Required:   true,
			Status:     StatusPending,
			Definition: definition,
			UpdatedAt:  now,
		}
	}

	if path := inferFileExists(clause); path != "" {
		return &Item{
			ID:       fmt.Sprintf("file-exists-%d", ordinal),
			Title:    "Required file exists",
			Type:     TypeFileExists,
			Scope:    scope,
			Required: true,
			Status:   StatusPending,
			Definition: map[string]any{
				"path": path,
			},
			UpdatedAt: now,
		}
	}

	if path, needle := inferFileContains(clause); path != "" && needle != "" {
		return &Item{
			ID:       fmt.Sprintf("file-contains-%d", ordinal),
			Title:    "Required file contains expected content",
			Type:     TypeFileContains,
			Scope:    scope,
			Required: true,
			Status:   StatusPending,
			Definition: map[string]any{
				"path":     path,
				"contains": needle,
			},
			UpdatedAt: now,
		}
	}

	if cmd := inferCommand(clause); cmd != "" {
		return &Item{
			ID:       fmt.Sprintf("command-check-%d", ordinal),
			Title:    "Command-based success check",
			Type:     TypeCommand,
			Scope:    scope,
			Required: true,
			Status:   StatusPending,
			Definition: map[string]any{
				"command":          cmd,
				"expect_exit_code": 0,
			},
			UpdatedAt: now,
		}
	}

	return nil
}

func splitNaturalCriteria(natural string) []string {
	trimmed := strings.TrimSpace(natural)
	if trimmed == "" {
		return nil
	}
	replaced := strings.ReplaceAll(trimmed, ";", "\n")
	replaced = splitClauseKeyword(replaced, "ensure")
	replaced = splitClauseKeyword(replaced, "check")
	replaced = splitClauseKeyword(replaced, "run command")
	parts := strings.Split(replaced, "\n")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

func splitClauseKeyword(input, keyword string) string {
	pattern := regexp.MustCompile(`(?i)\s+and\s+` + regexp.QuoteMeta(keyword) + `\s+`)
	return pattern.ReplaceAllStringFunc(input, func(match string) string {
		trimmed := strings.TrimSpace(match)
		lower := strings.ToLower(trimmed)
		idx := strings.Index(lower, keyword)
		if idx < 0 {
			return match
		}
		return "\n" + keyword + " "
	})
}

func inferCommand(natural string) string {
	trimmed := strings.TrimSpace(natural)
	lower := strings.ToLower(trimmed)
	switch {
	case lower == "go test ./..." || strings.HasPrefix(lower, "run command ") && strings.TrimSpace(trimmed[len("run command "):]) == "go test ./...":
		return "go test ./..."
	case lower == "go build ./cmd/nekobot" || strings.HasPrefix(lower, "run command ") && strings.TrimSpace(trimmed[len("run command "):]) == "go build ./cmd/nekobot":
		return "go build ./cmd/nekobot"
	default:
		return ""
	}
}

func inferFileExists(natural string) string {
	trimmed := strings.TrimSpace(natural)
	lower := strings.ToLower(trimmed)
	const prefix = "ensure file "
	const suffix = " exists"
	if strings.HasPrefix(lower, prefix) && strings.HasSuffix(lower, suffix) {
		path := strings.TrimSpace(trimmed[len(prefix) : len(trimmed)-len(suffix)])
		return strings.Trim(path, "`\"'")
	}
	return ""
}

func inferFileContains(natural string) (string, string) {
	trimmed := strings.TrimSpace(natural)
	lower := strings.ToLower(trimmed)
	const prefix = "ensure file "
	const marker = " contains "
	if !strings.HasPrefix(lower, prefix) {
		return "", ""
	}
	rest := trimmed[len(prefix):]
	idx := strings.Index(strings.ToLower(rest), marker)
	if idx < 0 {
		return "", ""
	}
	path := strings.TrimSpace(rest[:idx])
	needle := strings.TrimSpace(rest[idx+len(marker):])
	return strings.Trim(path, "`\"'"), strings.Trim(needle, "`\"'")
}

func inferHTTPCheck(natural string) (targetURL string, statusCode int, bodyContains string) {
	trimmed := strings.TrimSpace(natural)
	lower := strings.ToLower(trimmed)

	if strings.HasPrefix(lower, "ensure url ") {
		rest := strings.TrimSpace(trimmed[len("ensure url "):])
		lowerRest := strings.ToLower(rest)
		if idx := strings.Index(lowerRest, " returns "); idx >= 0 {
			targetURL = strings.Trim(rest[:idx], "`\"'")
			if _, err := fmt.Sscanf(strings.TrimSpace(rest[idx+len(" returns "):]), "%d", &statusCode); err == nil {
				return targetURL, statusCode, ""
			}
		}
		if idx := strings.Index(lowerRest, " contains "); idx >= 0 {
			targetURL = strings.Trim(rest[:idx], "`\"'")
			bodyContains = strings.TrimSpace(rest[idx+len(" contains "):])
			return strings.Trim(targetURL, "`\"'"), 0, strings.Trim(bodyContains, "`\"'")
		}
	}

	if strings.HasPrefix(lower, "check url ") {
		rest := strings.TrimSpace(trimmed[len("check url "):])
		lowerRest := strings.ToLower(rest)
		if idx := strings.Index(lowerRest, " returns "); idx >= 0 {
			targetURL = strings.Trim(rest[:idx], "`\"'")
			if _, err := fmt.Sscanf(strings.TrimSpace(rest[idx+len(" returns "):]), "%d", &statusCode); err == nil {
				return targetURL, statusCode, ""
			}
		}
		if idx := strings.Index(lowerRest, " contains "); idx >= 0 {
			targetURL = strings.Trim(rest[:idx], "`\"'")
			bodyContains = strings.TrimSpace(rest[idx+len(" contains "):])
			return strings.Trim(targetURL, "`\"'"), 0, strings.Trim(bodyContains, "`\"'")
		}
	}

	return "", 0, ""
}
