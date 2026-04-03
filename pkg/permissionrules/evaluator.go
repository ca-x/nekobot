package permissionrules

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

type Input struct {
	ToolName  string `json:"tool_name"`
	SessionID string `json:"session_id,omitempty"`
	RuntimeID string `json:"runtime_id,omitempty"`
}

type Explanation struct {
	Source        string `json:"source"`
	Reason        string `json:"reason"`
	MatchedRuleID string `json:"matched_rule_id,omitempty"`
	MatchedScope  string `json:"matched_scope,omitempty"`
}

type Result struct {
	Matched     bool        `json:"matched"`
	Action      Action      `json:"action,omitempty"`
	RuleID      string      `json:"rule_id,omitempty"`
	ToolName    string      `json:"tool_name"`
	SessionID   string      `json:"session_id,omitempty"`
	RuntimeID   string      `json:"runtime_id,omitempty"`
	Explanation Explanation `json:"explanation"`
}

func (m *Manager) Evaluate(ctx context.Context, input Input) (Result, error) {
	input.ToolName = strings.TrimSpace(input.ToolName)
	input.SessionID = strings.TrimSpace(input.SessionID)
	input.RuntimeID = strings.TrimSpace(input.RuntimeID)
	if input.ToolName == "" {
		return Result{}, fmt.Errorf("%w: tool_name is required", ErrInvalidRule)
	}

	rules, err := m.List(ctx)
	if err != nil {
		return Result{}, err
	}

	candidates := make([]Rule, 0, len(rules))
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		if rule.ToolName != input.ToolName {
			continue
		}
		if rule.SessionID != "" && rule.SessionID != input.SessionID {
			continue
		}
		if rule.RuntimeID != "" && rule.RuntimeID != input.RuntimeID {
			continue
		}
		candidates = append(candidates, rule)
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Priority != candidates[j].Priority {
			return candidates[i].Priority > candidates[j].Priority
		}
		leftScope := scopeRank(candidates[i])
		rightScope := scopeRank(candidates[j])
		if leftScope != rightScope {
			return leftScope > rightScope
		}
		if !candidates[i].UpdatedAt.Equal(candidates[j].UpdatedAt) {
			return candidates[i].UpdatedAt.After(candidates[j].UpdatedAt)
		}
		return candidates[i].ID < candidates[j].ID
	})

	if len(candidates) == 0 {
		return Result{
			Matched:   false,
			ToolName:  input.ToolName,
			SessionID: input.SessionID,
			RuntimeID: input.RuntimeID,
			Explanation: Explanation{
				Source: "approval_mode_fallback",
				Reason: "no permission rule matched; fall back to approval mode",
			},
		}, nil
	}

	matched := candidates[0]
	return Result{
		Matched:   true,
		Action:    matched.Action,
		RuleID:    matched.ID,
		ToolName:  input.ToolName,
		SessionID: input.SessionID,
		RuntimeID: input.RuntimeID,
		Explanation: Explanation{
			Source:        "rule",
			Reason:        "matched permission rule",
			MatchedRuleID: matched.ID,
			MatchedScope:  scopeName(matched),
		},
	}, nil
}

func scopeRank(rule Rule) int {
	switch {
	case rule.SessionID != "" && rule.RuntimeID != "":
		return 4
	case rule.SessionID != "":
		return 3
	case rule.RuntimeID != "":
		return 2
	default:
		return 1
	}
}

func scopeName(rule Rule) string {
	switch {
	case rule.SessionID != "" && rule.RuntimeID != "":
		return "session_runtime"
	case rule.SessionID != "":
		return "session"
	case rule.RuntimeID != "":
		return "runtime"
	default:
		return "global"
	}
}
