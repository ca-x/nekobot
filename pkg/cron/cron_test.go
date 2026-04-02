package cron

import (
	"context"
	"fmt"
	"testing"
	"time"

	"nekobot/pkg/agent"
	"nekobot/pkg/config"
	"nekobot/pkg/execenv"
	"nekobot/pkg/logger"
	"nekobot/pkg/tasks"
)

func TestManagerPersistsJobsInDatabase(t *testing.T) {
	t.Parallel()

	manager, cleanup := newTestManager(t)
	defer cleanup()

	ctx := t.Context()
	job, err := manager.AddCronJob("db-cron", "0 0 * * *", "ping")
	if err != nil {
		t.Fatalf("add cron job: %v", err)
	}

	stored, err := manager.client.CronJob.Get(ctx, job.ID)
	if err != nil {
		t.Fatalf("get stored cron job: %v", err)
	}
	if stored.Name != "db-cron" {
		t.Fatalf("expected persisted name db-cron, got %q", stored.Name)
	}
	if stored.ScheduleKind != string(ScheduleCron) {
		t.Fatalf("expected schedule kind %q, got %q", ScheduleCron, stored.ScheduleKind)
	}
	if stored.Schedule != "0 0 * * *" {
		t.Fatalf("expected schedule to persist, got %q", stored.Schedule)
	}

	if err := manager.DisableJob(job.ID); err != nil {
		t.Fatalf("disable job: %v", err)
	}
	updated, err := manager.client.CronJob.Get(ctx, job.ID)
	if err != nil {
		t.Fatalf("get disabled job: %v", err)
	}
	if updated.Enabled {
		t.Fatalf("expected job to be disabled in database")
	}

	if err := manager.EnableJob(job.ID); err != nil {
		t.Fatalf("enable job: %v", err)
	}
	updated, err = manager.client.CronJob.Get(ctx, job.ID)
	if err != nil {
		t.Fatalf("get enabled job: %v", err)
	}
	if !updated.Enabled {
		t.Fatalf("expected job to be enabled in database")
	}

	if err := manager.RemoveJob(job.ID); err != nil {
		t.Fatalf("remove job: %v", err)
	}
	if _, err := manager.client.CronJob.Get(ctx, job.ID); err == nil {
		t.Fatalf("expected removed job to be deleted from database")
	}
}

func TestManagerPersistsRouteOverridesInDatabase(t *testing.T) {
	t.Parallel()

	manager, cleanup := newTestManager(t)
	defer cleanup()

	ctx := t.Context()
	job, err := manager.AddCronJobWithRoute("db-cron", "0 0 * * *", "ping", RouteOptions{
		Provider: "pool-a",
		Model:    "claude-sonnet",
		Fallback: []string{"backup", "pool-b"},
	})
	if err != nil {
		t.Fatalf("add cron job with route: %v", err)
	}

	stored, err := manager.client.CronJob.Get(ctx, job.ID)
	if err != nil {
		t.Fatalf("get stored cron job: %v", err)
	}
	if stored.Provider != "pool-a" {
		t.Fatalf("expected provider pool-a, got %q", stored.Provider)
	}
	if stored.Model != "claude-sonnet" {
		t.Fatalf("expected model claude-sonnet, got %q", stored.Model)
	}
	if stored.FallbackJSON != "[\"backup\",\"pool-b\"]" {
		t.Fatalf("unexpected fallback json: %q", stored.FallbackJSON)
	}
}

func TestManagerLoadJobsFromDatabase(t *testing.T) {
	t.Parallel()

	manager, cleanup := newTestManager(t)
	defer cleanup()

	ctx := t.Context()
	at := time.Now().Add(2 * time.Minute).Round(time.Second)
	created, err := manager.client.CronJob.Create().
		SetID("job_preloaded").
		SetName("preloaded").
		SetScheduleKind(string(ScheduleAt)).
		SetSchedule("").
		SetAtTime(at).
		SetEveryDuration("").
		SetPrompt("from db").
		SetEnabled(true).
		SetDeleteAfterRun(false).
		SetCreatedAt(time.Now()).
		SetLastError("").
		SetLastSuccess(false).
		SetRunCount(3).
		Save(ctx)
	if err != nil {
		t.Fatalf("seed job: %v", err)
	}

	if err := manager.loadJobs(ctx); err != nil {
		t.Fatalf("load jobs: %v", err)
	}

	loaded, err := manager.GetJob(created.ID)
	if err != nil {
		t.Fatalf("get loaded job: %v", err)
	}
	if loaded.Name != "preloaded" {
		t.Fatalf("expected loaded name preloaded, got %q", loaded.Name)
	}
	if loaded.ScheduleKind != ScheduleAt {
		t.Fatalf("expected loaded kind %q, got %q", ScheduleAt, loaded.ScheduleKind)
	}
	if loaded.AtTime == nil {
		t.Fatalf("expected loaded at_time to be present")
	}
	if !loaded.AtTime.Equal(at) {
		t.Fatalf("expected at_time %s, got %s", at.Format(time.RFC3339), loaded.AtTime.Format(time.RFC3339))
	}
	if loaded.RunCount != 3 {
		t.Fatalf("expected run_count 3, got %d", loaded.RunCount)
	}
	if loaded.Provider != "" || loaded.Model != "" || len(loaded.Fallback) != 0 {
		t.Fatalf("expected empty route fields for legacy row, got provider=%q model=%q fallback=%v", loaded.Provider, loaded.Model, loaded.Fallback)
	}
}

func TestManagerDeleteAfterRunRemovesDatabaseRow(t *testing.T) {
	t.Parallel()

	manager, cleanup := newTestManager(t)
	defer cleanup()

	ctx := t.Context()
	job, err := manager.AddAtJob("one-shot", time.Now().Add(1*time.Minute), "prompt", true)
	if err != nil {
		t.Fatalf("add at job: %v", err)
	}

	manager.mu.Lock()
	stored := manager.jobs[job.ID]
	stored.NextRun = time.Now().Add(-1 * time.Second)
	snapshot := *stored
	manager.mu.Unlock()

	if err := manager.updateJobState(ctx, &snapshot); err != nil {
		t.Fatalf("persist due at job: %v", err)
	}

	manager.checkTimerJobs()

	if manager.hasJob(job.ID) {
		t.Fatalf("expected in-memory at job to be deleted after run")
	}
	if _, err := manager.client.CronJob.Get(ctx, job.ID); err == nil {
		t.Fatalf("expected one-time job row to be deleted after run")
	}
}

func TestManagerEveryJobPersistsNextRun(t *testing.T) {
	t.Parallel()

	manager, cleanup := newTestManager(t)
	defer cleanup()

	ctx := t.Context()
	job, err := manager.AddEveryJob("every", "1s", "ping")
	if err != nil {
		t.Fatalf("add every job: %v", err)
	}

	manager.mu.Lock()
	stored := manager.jobs[job.ID]
	stored.NextRun = time.Now().Add(-1 * time.Second)
	snapshot := *stored
	manager.mu.Unlock()

	if err := manager.updateJobState(ctx, &snapshot); err != nil {
		t.Fatalf("persist due every job: %v", err)
	}

	manager.checkTimerJobs()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		updated, err := manager.client.CronJob.Get(ctx, job.ID)
		if err != nil {
			t.Fatalf("get every job: %v", err)
		}
		if updated.RunCount > 0 && updated.NextRun != nil {
			if updated.NextRun.After(time.Now()) {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Fatalf("expected every job to update run_count and next_run")
}

func TestNormalizeScheduleKindDefaultsToCron(t *testing.T) {
	t.Parallel()

	if got := normalizeScheduleKind(""); got != ScheduleCron {
		t.Fatalf("expected empty kind to default to %q, got %q", ScheduleCron, got)
	}
	if got := normalizeScheduleKind(ScheduleEvery); got != ScheduleEvery {
		t.Fatalf("expected kind to stay %q, got %q", ScheduleEvery, got)
	}
}

func TestExecuteJobCreatesManagedTaskLifecycle(t *testing.T) {
	t.Parallel()

	manager, cleanup := newTestManager(t)
	defer cleanup()

	store := tasks.NewStore()
	taskSvc := tasks.NewService(store)
	manager.taskSvc = taskSvc
	manager.agent = &agent.Agent{}
	manager.agentChat = func(ctx context.Context, sess agent.SessionInterface, prompt, provider, model string, fallback []string) (string, error) {
		if got, _ := ctx.Value(execenv.MetadataRuntimeID).(string); got != "cron" {
			t.Fatalf("expected cron runtime id in context, got %q", got)
		}
		return "ok", nil
	}

	job, err := manager.AddCronJob("cron-task", "0 0 * * *", "ping")
	if err != nil {
		t.Fatalf("add cron job: %v", err)
	}

	manager.executeJob(job.ID)

	task := waitForCronTaskState(t, store, tasks.StateCompleted, 3*time.Second)
	if task.Type != tasks.TypeLocalAgent {
		t.Fatalf("expected local agent task type, got %q", task.Type)
	}
	if task.SessionID != job.ID {
		t.Fatalf("expected session id %q, got %q", job.ID, task.SessionID)
	}
	if task.RuntimeID != "cron" {
		t.Fatalf("expected runtime id cron, got %q", task.RuntimeID)
	}
	if got, _ := task.Metadata["source"].(string); got != "cron" {
		t.Fatalf("expected cron source metadata, got %q", got)
	}
}

func TestExecuteJobMarksManagedTaskFailedOnChatError(t *testing.T) {
	t.Parallel()

	manager, cleanup := newTestManager(t)
	defer cleanup()

	store := tasks.NewStore()
	taskSvc := tasks.NewService(store)
	manager.taskSvc = taskSvc
	manager.agent = &agent.Agent{}
	manager.agentChat = func(ctx context.Context, sess agent.SessionInterface, prompt, provider, model string, fallback []string) (string, error) {
		return "", fmt.Errorf("chat failed")
	}

	job, err := manager.AddCronJob("cron-task-fail", "0 0 * * *", "ping")
	if err != nil {
		t.Fatalf("add cron job: %v", err)
	}

	manager.executeJob(job.ID)

	task := waitForCronTaskState(t, store, tasks.StateFailed, 3*time.Second)
	if task.LastError != "chat failed" {
		t.Fatalf("expected chat failure to be recorded, got %q", task.LastError)
	}
}

func waitForCronTaskState(t *testing.T, store *tasks.Store, want tasks.State, timeout time.Duration) tasks.Task {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		all := store.List()
		if len(all) == 1 && all[0].State == want {
			return all[0]
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("cron task did not reach state %s: %+v", want, store.List())
	return tasks.Task{}
}

func newTestManager(t *testing.T) (*Manager, func()) {
	t.Helper()

	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()

	log, err := logger.New(logger.DefaultConfig())
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}

	client, err := config.OpenRuntimeEntClient(cfg)
	if err != nil {
		t.Fatalf("open runtime ent client: %v", err)
	}
	if err := config.EnsureRuntimeEntSchema(client); err != nil {
		_ = client.Close()
		t.Fatalf("ensure runtime schema: %v", err)
	}

	manager := New(log, &agent.Agent{}, client)
	cleanup := func() {
		if err := client.Close(); err != nil {
			t.Fatalf("close ent client: %v", err)
		}
	}
	return manager, cleanup
}

func (m *Manager) hasJob(jobID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.jobs[jobID]
	return ok
}
