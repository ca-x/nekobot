package cron

import (
	"testing"
	"time"

	"nekobot/pkg/agent"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
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
