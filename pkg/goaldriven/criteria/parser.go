package criteria

import (
	"context"
	"fmt"
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

	items := make([]Item, 0, 3)

	if path := inferFileExists(in.Natural); path != "" {
		items = append(items, Item{
			ID:       "file-exists-1",
			Title:    "Required file exists",
			Type:     TypeFileExists,
			Scope:    *in.Scope,
			Required: true,
			Status:   StatusPending,
			Definition: map[string]any{
				"path": path,
			},
			UpdatedAt: time.Now().UTC(),
		})
	}

	if path, needle := inferFileContains(in.Natural); path != "" && needle != "" {
		items = append(items, Item{
			ID:       "file-contains-1",
			Title:    "Required file contains expected content",
			Type:     TypeFileContains,
			Scope:    *in.Scope,
			Required: true,
			Status:   StatusPending,
			Definition: map[string]any{
				"path":     path,
				"contains": needle,
			},
			UpdatedAt: time.Now().UTC(),
		})
	}

	if len(items) == 0 {
		if cmd := inferCommand(in.Natural); cmd != "" {
			items = append(items, Item{
				ID:       "command-check-1",
				Title:    "Command-based success check",
				Type:     TypeCommand,
				Scope:    *in.Scope,
				Required: true,
				Status:   StatusPending,
				Definition: map[string]any{
					"command":          cmd,
					"expect_exit_code": 0,
				},
				UpdatedAt: time.Now().UTC(),
			})
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
