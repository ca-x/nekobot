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

	item := Item{
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
	}

	if cmd := inferCommand(in.Natural); cmd != "" {
		item = Item{
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
		}
	}

	return Set{Criteria: []Item{item}}, nil
}

func inferCommand(natural string) string {
	trimmed := strings.TrimSpace(natural)
	lower := strings.ToLower(trimmed)
	switch {
	case strings.Contains(lower, "go test ./..."):
		return "go test ./..."
	case strings.Contains(lower, "go build ./cmd/nekobot"):
		return "go build ./cmd/nekobot"
	default:
		return ""
	}
}
