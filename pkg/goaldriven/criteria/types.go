package criteria

import (
	"time"

	"nekobot/pkg/goaldriven/shared"
)

// CriterionType identifies one verifier shape.
type CriterionType string

// CriterionStatus tracks one criterion's current evaluation state.
type CriterionStatus string

const (
	TypeCommand            CriterionType = "command"
	TypeFileExists         CriterionType = "file_exists"
	TypeFileContains       CriterionType = "file_contains"
	TypeManualConfirmation CriterionType = "manual_confirmation"
)

const (
	StatusPending    CriterionStatus = "pending"
	StatusPassed     CriterionStatus = "passed"
	StatusFailed     CriterionStatus = "failed"
	StatusBlocked    CriterionStatus = "blocked"
	StatusNeedsHuman CriterionStatus = "needs_human_confirmation"
)

// Item is one criteria entry.
type Item struct {
	ID         string                `json:"id"`
	Title      string                `json:"title"`
	Type       CriterionType         `json:"type"`
	Scope      shared.ExecutionScope `json:"scope"`
	Required   bool                  `json:"required"`
	Status     CriterionStatus       `json:"status"`
	Definition map[string]any        `json:"definition"`
	Evidence   []string              `json:"evidence,omitempty"`
	LastError  string                `json:"last_error,omitempty"`
	UpdatedAt  time.Time             `json:"updated_at"`
}

// Set is the stored criteria collection for one GoalRun.
type Set struct {
	Criteria []Item `json:"criteria"`
}
