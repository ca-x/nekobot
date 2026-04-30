package runs

import (
	"context"
	"testing"

	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/storage/ent"
)

func newTestManager(t *testing.T) (*Manager, *ent.Client) {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	logCfg := logger.DefaultConfig()
	logCfg.OutputPath = ""
	log, err := logger.New(logCfg)
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}
	client, err := config.OpenRuntimeEntClient(cfg)
	if err != nil {
		t.Fatalf("open runtime ent client: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	if err := config.EnsureRuntimeEntSchema(client); err != nil {
		t.Fatalf("ensure runtime schema: %v", err)
	}
	mgr, err := NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new runs manager: %v", err)
	}
	return mgr, client
}

func TestCreateAndGetRun(t *testing.T) {
	mgr, _ := newTestManager(t)
	ctx := context.Background()

	created, err := mgr.CreateRun(ctx, RunRecord{
		TaskID:           "task-1",
		Target:           "#ops",
		AgentID:          "agent-1",
		ComputerID:       "comp-1",
		RuntimeProfileID: "rp-1",
		Status:           "running",
		Summary:          "doing work",
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected non-empty run ID")
	}
	if created.TaskID != "task-1" {
		t.Errorf("task_id = %q, want task-1", created.TaskID)
	}
	if created.Status != "running" {
		t.Errorf("status = %q, want running", created.Status)
	}

	got, err := mgr.GetRun(ctx, created.ID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if got == nil {
		t.Fatal("expected run to exist")
	}
	if got.ID != created.ID {
		t.Errorf("id = %q, want %q", got.ID, created.ID)
	}
	if got.AgentID != "agent-1" {
		t.Errorf("agent_id = %q, want agent-1", got.AgentID)
	}
}

func TestGetRunNotFound(t *testing.T) {
	mgr, _ := newTestManager(t)
	ctx := context.Background()

	got, err := mgr.GetRun(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for nonexistent run, got %+v", got)
	}
}

func TestUpdateRunStatusSetsCompletedAt(t *testing.T) {
	mgr, _ := newTestManager(t)
	ctx := context.Background()

	created, err := mgr.CreateRun(ctx, RunRecord{
		TaskID:  "task-2",
		Status:  "running",
		AgentID: "agent-1",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.CompletedAt != nil {
		t.Fatal("completed_at should be nil for running run")
	}

	updated, err := mgr.UpdateRunStatus(ctx, created.ID, "succeeded", "", "done", "")
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Status != "succeeded" {
		t.Errorf("status = %q, want succeeded", updated.Status)
	}
	if updated.CompletedAt == nil {
		t.Error("completed_at should be set for terminal status")
	}
}

func TestUpdateRunStatusNoCompletedForNonTerminal(t *testing.T) {
	mgr, _ := newTestManager(t)
	ctx := context.Background()

	created, err := mgr.CreateRun(ctx, RunRecord{TaskID: "task-3", Status: "queued"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	updated, err := mgr.UpdateRunStatus(ctx, created.ID, "running", "", "started", "")
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.CompletedAt != nil {
		t.Error("completed_at should remain nil for non-terminal status")
	}
}

func TestListRunsFilters(t *testing.T) {
	mgr, _ := newTestManager(t)
	ctx := context.Background()

	for _, r := range []RunRecord{
		{TaskID: "t1", Target: "#ops", AgentID: "a1", Status: "running"},
		{TaskID: "t2", Target: "#ops", AgentID: "a2", Status: "running"},
		{TaskID: "t3", Target: "#alerts", AgentID: "a1", Status: "running"},
	} {
		if _, err := mgr.CreateRun(ctx, r); err != nil {
			t.Fatalf("create: %v", err)
		}
	}

	// Filter by target
	runs, err := mgr.ListRuns(ctx, "#ops", "", "", 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(runs) != 2 {
		t.Errorf("expected 2 runs for #ops, got %d", len(runs))
	}

	// Filter by agent_id
	runs, err = mgr.ListRuns(ctx, "", "", "a1", 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(runs) != 2 {
		t.Errorf("expected 2 runs for a1, got %d", len(runs))
	}

	// Filter by task_id
	runs, err = mgr.ListRuns(ctx, "", "t2", "", 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(runs) != 1 {
		t.Errorf("expected 1 run for t2, got %d", len(runs))
	}

	// Limit
	runs, err = mgr.ListRuns(ctx, "", "", "", 1)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(runs) != 1 {
		t.Errorf("expected 1 run with limit, got %d", len(runs))
	}
}

func TestAppendRunStepAutoSequence(t *testing.T) {
	mgr, _ := newTestManager(t)
	ctx := context.Background()

	created, err := mgr.CreateRun(ctx, RunRecord{TaskID: "task-seq", Status: "running"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	step1, err := mgr.AppendRunStep(ctx, StepRecord{
		RunID:   created.ID,
		Kind:    "message",
		Summary: "first",
	})
	if err != nil {
		t.Fatalf("append step 1: %v", err)
	}
	if step1.Sequence != 1 {
		t.Errorf("step 1 sequence = %d, want 1", step1.Sequence)
	}

	step2, err := mgr.AppendRunStep(ctx, StepRecord{
		RunID:   created.ID,
		Kind:    "shell",
		Summary: "second",
	})
	if err != nil {
		t.Fatalf("append step 2: %v", err)
	}
	if step2.Sequence != 2 {
		t.Errorf("step 2 sequence = %d, want 2", step2.Sequence)
	}
}

func TestAppendRunStepExplicitSequence(t *testing.T) {
	mgr, _ := newTestManager(t)
	ctx := context.Background()

	created, err := mgr.CreateRun(ctx, RunRecord{TaskID: "task-explicit", Status: "running"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	step, err := mgr.AppendRunStep(ctx, StepRecord{
		RunID:    created.ID,
		Sequence: 42,
		Kind:     "commit",
		Summary:  "explicit seq",
	})
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	if step.Sequence != 42 {
		t.Errorf("sequence = %d, want 42", step.Sequence)
	}
}

func TestListRunSteps(t *testing.T) {
	mgr, _ := newTestManager(t)
	ctx := context.Background()

	created, err := mgr.CreateRun(ctx, RunRecord{TaskID: "task-steps", Status: "running"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	for _, kind := range []string{"message", "tool_call", "shell", "commit"} {
		if _, err := mgr.AppendRunStep(ctx, StepRecord{
			RunID:   created.ID,
			Kind:    kind,
			Summary: "step: " + kind,
		}); err != nil {
			t.Fatalf("append %s: %v", kind, err)
		}
	}

	steps, err := mgr.ListRunSteps(ctx, created.ID)
	if err != nil {
		t.Fatalf("list steps: %v", err)
	}
	if len(steps) != 4 {
		t.Fatalf("expected 4 steps, got %d", len(steps))
	}
	for i, s := range steps {
		if s.Sequence != uint32(i+1) {
			t.Errorf("step %d sequence = %d, want %d", i, s.Sequence, i+1)
		}
	}
}

func TestAppendRunStepWithArtifacts(t *testing.T) {
	mgr, _ := newTestManager(t)
	ctx := context.Background()

	created, err := mgr.CreateRun(ctx, RunRecord{TaskID: "task-artifacts", Status: "running"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	step, err := mgr.AppendRunStep(ctx, StepRecord{
		RunID:       created.ID,
		Kind:        "file_change",
		Summary:     "modified files",
		ArtifactIDs: []string{"att-1", "att-2"},
	})
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	if len(step.ArtifactIDs) != 2 {
		t.Errorf("artifact_ids = %v, want [att-1 att-2]", step.ArtifactIDs)
	}

	steps, err := mgr.ListRunSteps(ctx, created.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(steps))
	}
	if steps[0].ArtifactIDs[0] != "att-1" || steps[0].ArtifactIDs[1] != "att-2" {
		t.Errorf("roundtrip artifact_ids = %v", steps[0].ArtifactIDs)
	}
}

func TestGetRunWithSteps(t *testing.T) {
	mgr, _ := newTestManager(t)
	ctx := context.Background()

	created, err := mgr.CreateRun(ctx, RunRecord{TaskID: "task-full", Target: "#ops", AgentID: "a1", Status: "running"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	for _, kind := range []string{"message", "shell"} {
		if _, err := mgr.AppendRunStep(ctx, StepRecord{RunID: created.ID, Kind: kind}); err != nil {
			t.Fatalf("append: %v", err)
		}
	}

	// GetRun + ListRunSteps = full view
	r, err := mgr.GetRun(ctx, created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if r.TaskID != "task-full" {
		t.Errorf("task_id = %q", r.TaskID)
	}

	steps, err := mgr.ListRunSteps(ctx, created.ID)
	if err != nil {
		t.Fatalf("list steps: %v", err)
	}
	if len(steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(steps))
	}
}

func TestAppendRunStepMissingRunID(t *testing.T) {
	mgr, _ := newTestManager(t)
	ctx := context.Background()

	_, err := mgr.AppendRunStep(ctx, StepRecord{Kind: "message"})
	if err == nil {
		t.Fatal("expected error for missing run_id")
	}
}
