// Package runs manages durable Run and RunStep records in the runtime database.
package runs

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/storage/ent"
	"nekobot/pkg/storage/ent/run"
	"nekobot/pkg/storage/ent/runstep"
)

// RunRecord is the domain model for a persisted execution run.
type RunRecord struct {
	ID                string
	TaskID            string
	Target            string
	AgentID           string
	ComputerID        string
	RuntimeProfileID  string
	Status            string
	LeaseID           string
	RequestID         string
	InputMessageID    string
	LastSeenEventID   string
	StartedAt         time.Time
	UpdatedAt         time.Time
	CompletedAt       *time.Time
	Error             string
	Summary           string
	State             string
	TenantID          string
	OwnerUserID       string
	Visibility        string
}

// StepRecord is the domain model for a persisted run step.
type StepRecord struct {
	ID             string
	RunID          string
	Sequence       uint32
	Kind           string
	Status         string
	Summary        string
	Detail         string
	ArtifactIDs    []string
	StartedAt      time.Time
	CompletedAt    *time.Time
	RequestID      string
}

// Manager persists runs and run steps in the runtime database.
type Manager struct {
	cfg    *config.Config
	log    *logger.Logger
	client *ent.Client
}

// NewManager creates a runs manager backed by the runtime database.
func NewManager(cfg *config.Config, log *logger.Logger, client *ent.Client) (*Manager, error) {
	if client == nil {
		return nil, fmt.Errorf("ent client is nil")
	}
	mgr := &Manager{cfg: cfg, log: log, client: client}
	dbPath, _ := config.RuntimeDBDisplayName(cfg)
	log.Info("Run/step storage initialized", zap.String("db_path", dbPath))
	return mgr, nil
}

// Close releases manager resources. Shared Ent client is closed elsewhere.
func (m *Manager) Close() error {
	return nil
}

// ---------------------------------------------------------------------------
// Run CRUD
// ---------------------------------------------------------------------------

// CreateRun persists a new run record.
func (m *Manager) CreateRun(ctx context.Context, item RunRecord) (*RunRecord, error) {
	item.ID = strings.TrimSpace(item.ID)
	if item.ID == "" {
		item.ID = newUUID()
	}
	vis := strings.TrimSpace(item.Visibility)
	if vis == "" {
		vis = "shared"
	}
	rec, err := m.client.Run.Create().
		SetID(item.ID).
		SetTaskID(item.TaskID).
		SetTarget(item.Target).
		SetAgentID(item.AgentID).
		SetComputerID(item.ComputerID).
		SetRuntimeProfileID(item.RuntimeProfileID).
		SetStatus(item.Status).
		SetLeaseID(item.LeaseID).
		SetRequestID(item.RequestID).
		SetInputMessageID(item.InputMessageID).
		SetLastSeenEventID(item.LastSeenEventID).
		SetError(item.Error).
		SetSummary(item.Summary).
		SetState(item.State).
		SetTenantID(item.TenantID).
		SetOwnerUserID(item.OwnerUserID).
		SetVisibility(run.Visibility(vis)).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create run: %w", err)
	}
	return entRunToRecord(rec), nil
}

// GetRun returns a run by ID, or nil if not found.
func (m *Manager) GetRun(ctx context.Context, id string) (*RunRecord, error) {
	rec, err := m.client.Run.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("get run %s: %w", id, err)
	}
	return entRunToRecord(rec), nil
}

// UpdateRunStatus updates the status, error, summary, and state of a run.
func (m *Manager) UpdateRunStatus(ctx context.Context, id string, status, errStr, summary, state string) (*RunRecord, error) {
	q := m.client.Run.UpdateOneID(id).
		SetStatus(status).
		SetError(errStr).
		SetSummary(summary).
		SetState(state)
	if isTerminalStatus(status) {
		q = q.SetCompletedAt(time.Now())
	}
	rec, err := q.Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("update run %s: %w", id, err)
	}
	return entRunToRecord(rec), nil
}

// ListRuns returns runs matching the given filters. Empty filter fields are ignored.
func (m *Manager) ListRuns(ctx context.Context, target, taskID, agentID string, limit int) ([]RunRecord, error) {
	q := m.client.Run.Query().Order(ent.Desc(run.FieldStartedAt))
	if target = strings.TrimSpace(target); target != "" {
		q = q.Where(run.TargetEQ(target))
	}
	if taskID = strings.TrimSpace(taskID); taskID != "" {
		q = q.Where(run.TaskIDEQ(taskID))
	}
	if agentID = strings.TrimSpace(agentID); agentID != "" {
		q = q.Where(run.AgentIDEQ(agentID))
	}
	if limit > 0 {
		q = q.Limit(limit)
	}
	recs, err := q.All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list runs: %w", err)
	}
	out := make([]RunRecord, 0, len(recs))
	for _, r := range recs {
		out = append(out, *entRunToRecord(r))
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// RunStep CRUD
// ---------------------------------------------------------------------------

// AppendRunStep inserts a new step for a run. If step.Sequence is 0, it is
// auto-assigned as max(existing sequence) + 1.
func (m *Manager) AppendRunStep(ctx context.Context, item StepRecord) (*StepRecord, error) {
	item.ID = strings.TrimSpace(item.ID)
	if item.ID == "" {
		item.ID = newUUID()
	}
	item.RunID = strings.TrimSpace(item.RunID)
	if item.RunID == "" {
		return nil, fmt.Errorf("run_id is required")
	}

	// Auto-sequence if not provided.
	if item.Sequence == 0 {
		count, err := m.client.RunStep.Query().
			Where(runstep.RunIDEQ(item.RunID)).
			Count(ctx)
		if err != nil {
			return nil, fmt.Errorf("count steps for run %s: %w", item.RunID, err)
		}
		item.Sequence = uint32(count) + 1
	}

	artifactJSON := "[]"
	if len(item.ArtifactIDs) > 0 {
		b, _ := json.Marshal(item.ArtifactIDs)
		artifactJSON = string(b)
	}

	rec, err := m.client.RunStep.Create().
		SetID(item.ID).
		SetRunID(item.RunID).
		SetSequence(item.Sequence).
		SetKind(item.Kind).
		SetStatus(item.Status).
		SetSummary(item.Summary).
		SetDetail(item.Detail).
		SetArtifactIdsJSON(artifactJSON).
		SetRequestID(item.RequestID).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("append run step to run %s: %w", item.RunID, err)
	}
	return entStepToRecord(rec), nil
}

// ListRunSteps returns all steps for a run, ordered by sequence.
func (m *Manager) ListRunSteps(ctx context.Context, runID string) ([]StepRecord, error) {
	recs, err := m.client.RunStep.Query().
		Where(runstep.RunIDEQ(runID)).
		Order(ent.Asc(runstep.FieldSequence)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list run steps for run %s: %w", runID, err)
	}
	out := make([]StepRecord, 0, len(recs))
	for _, r := range recs {
		out = append(out, *entStepToRecord(r))
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func entRunToRecord(rec *ent.Run) *RunRecord {
	if rec == nil {
		return nil
	}
	var completedAt *time.Time
	if rec.CompletedAt != nil {
		completedAt = rec.CompletedAt
	}
	return &RunRecord{
		ID:               rec.ID,
		TaskID:           rec.TaskID,
		Target:           rec.Target,
		AgentID:          rec.AgentID,
		ComputerID:       rec.ComputerID,
		RuntimeProfileID: rec.RuntimeProfileID,
		Status:           rec.Status,
		LeaseID:          rec.LeaseID,
		RequestID:        rec.RequestID,
		InputMessageID:   rec.InputMessageID,
		LastSeenEventID:  rec.LastSeenEventID,
		StartedAt:        rec.StartedAt,
		UpdatedAt:        rec.UpdatedAt,
		CompletedAt:      completedAt,
		Error:            rec.Error,
		Summary:          rec.Summary,
		State:            rec.State,
		TenantID:         rec.TenantID,
		OwnerUserID:      rec.OwnerUserID,
		Visibility:       string(rec.Visibility),
	}
}

func entStepToRecord(rec *ent.RunStep) *StepRecord {
	if rec == nil {
		return nil
	}
	var completedAt *time.Time
	if rec.CompletedAt != nil {
		completedAt = rec.CompletedAt
	}
	var artifactIDs []string
	if rec.ArtifactIdsJSON != "" && rec.ArtifactIdsJSON != "[]" {
		_ = json.Unmarshal([]byte(rec.ArtifactIdsJSON), &artifactIDs)
	}
	return &StepRecord{
		ID:          rec.ID,
		RunID:       rec.RunID,
		Sequence:    rec.Sequence,
		Kind:        rec.Kind,
		Status:      rec.Status,
		Summary:     rec.Summary,
		Detail:      rec.Detail,
		ArtifactIDs: artifactIDs,
		StartedAt:   rec.StartedAt,
		CompletedAt: completedAt,
		RequestID:   rec.RequestID,
	}
}

func isTerminalStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "succeeded", "failed", "cancelled", "handoff_required":
		return true
	default:
		return false
	}
}

func newUUID() string {
	return uuid.NewString()
}
