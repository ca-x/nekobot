package scope

import (
	"context"
	"strings"

	"nekobot/pkg/goaldriven/shared"
)

// Resolver chooses the execution scope for one GoalRun.
type Resolver struct{}

// NewResolver creates a scope resolver.
func NewResolver() *Resolver { return &Resolver{} }

// ResolveInput is the scope resolver request.
type ResolveInput struct {
	Goal            string
	NaturalCriteria string
	AllowAutoScope  bool
	SelectedScope   *shared.ExecutionScope
}

// Resolve picks the initial execution scope.
func (r *Resolver) Resolve(_ context.Context, in ResolveInput) (*shared.ExecutionScope, error) {
	if in.SelectedScope != nil {
		cloned := *in.SelectedScope
		if strings.TrimSpace(cloned.Source) == "" {
			cloned.Source = "manual"
		}
		return &cloned, nil
	}

	text := strings.ToLower(strings.TrimSpace(in.Goal + "\n" + in.NaturalCriteria))
	if in.AllowAutoScope && (strings.Contains(text, "daemon") || strings.Contains(text, "machine")) {
		return &shared.ExecutionScope{
			Kind:   shared.ScopeDaemon,
			Source: "auto",
			Reason: "goal text references daemon or machine execution",
		}, nil
	}

	return &shared.ExecutionScope{
		Kind:   shared.ScopeServer,
		Source: "auto",
		Reason: "defaulted to server scope",
	}, nil
}
