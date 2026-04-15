package goaldriven

import (
	"time"

	"nekobot/pkg/goaldriven/shared"
)

// GoalStatus describes the lifecycle state of a GoalRun.
type GoalStatus string

// RiskLevel defines how aggressively a GoalRun may act.
type RiskLevel = shared.RiskLevel

// ExecutionScopeKind identifies where a GoalRun should execute.
type ExecutionScopeKind = shared.ExecutionScopeKind

const (
	GoalStatusDraft                  GoalStatus = "draft"
	GoalStatusCriteriaPendingConfirm GoalStatus = "criteria_pending_confirmation"
	GoalStatusReady                  GoalStatus = "ready"
	GoalStatusRunning                GoalStatus = "running"
	GoalStatusVerifying              GoalStatus = "verifying"
	GoalStatusNeedsApproval          GoalStatus = "needs_approval"
	GoalStatusNeedsHumanConfirmation GoalStatus = "needs_human_confirmation"
	GoalStatusCompleted              GoalStatus = "completed"
	GoalStatusFailed                 GoalStatus = "failed"
	GoalStatusCanceled               GoalStatus = "canceled"
)

const (
	RiskConservative RiskLevel = shared.RiskConservative
	RiskBalanced     RiskLevel = shared.RiskBalanced
	RiskAggressive   RiskLevel = shared.RiskAggressive
)

const (
	ScopeServer ExecutionScopeKind = shared.ScopeServer
	ScopeDaemon ExecutionScopeKind = shared.ScopeDaemon
)

// ExecutionScope describes the selected execution surface for one GoalRun.
type ExecutionScope = shared.ExecutionScope

// WorkerRef tracks one worker owned by a GoalRun.
type WorkerRef struct {
	ID              string         `json:"id"`
	Name            string         `json:"name"`
	Status          string         `json:"status"`
	Scope           ExecutionScope `json:"scope"`
	TaskID          string         `json:"task_id,omitempty"`
	LastHeartbeatAt time.Time      `json:"last_heartbeat_at,omitempty"`
	LastProgressAt  time.Time      `json:"last_progress_at,omitempty"`
	LeaseExpiresAt  time.Time      `json:"lease_expires_at,omitempty"`
	RestartCount    int            `json:"restart_count"`
	LastError       string         `json:"last_error,omitempty"`
}

// GoalRun is the top-level persisted goal orchestration record.
type GoalRun struct {
	ID                      string          `json:"id"`
	Name                    string          `json:"name"`
	Goal                    string          `json:"goal"`
	NaturalLanguageCriteria string          `json:"natural_language_criteria"`
	Status                  GoalStatus      `json:"status"`
	RiskLevel               RiskLevel       `json:"risk_level"`
	AllowAutoScope          bool            `json:"allow_auto_scope"`
	AllowParallelWorkers    bool            `json:"allow_parallel_workers"`
	RecommendedScope        *ExecutionScope `json:"recommended_scope,omitempty"`
	SelectedScope           *ExecutionScope `json:"selected_scope,omitempty"`
	CurrentWorkerIDs        []string        `json:"current_worker_ids,omitempty"`
	LastEvaluationID        string          `json:"last_evaluation_id,omitempty"`
	LastActivityAt          time.Time       `json:"last_activity_at,omitempty"`
	CreatedBy               string          `json:"created_by"`
	CreatedAt               time.Time       `json:"created_at"`
	UpdatedAt               time.Time       `json:"updated_at"`
	StartedAt               time.Time       `json:"started_at,omitempty"`
	CompletedAt             time.Time       `json:"completed_at,omitempty"`
}
