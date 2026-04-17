package goaldriven

import (
	"context"
	"fmt"
	"testing"
	"time"

	"nekobot/pkg/goaldriven/criteria"
	"nekobot/pkg/logger"
	"nekobot/pkg/state"
)

type failIndexKV struct {
	state.KV
}

func (f failIndexKV) UpdateFunc(ctx context.Context, key string, updateFn func(current interface{}) interface{}) error {
	if key == goalRunIndexKey {
		return fmt.Errorf("forced index failure")
	}
	return f.KV.UpdateFunc(ctx, key, updateFn)
}

func TestKVStoreRoundTrip(t *testing.T) {
	t.Parallel()

	log, err := logger.New(&logger.Config{Level: logger.LevelInfo, Development: true})
	if err != nil {
		t.Fatalf("logger.New failed: %v", err)
	}
	kv, err := state.NewFileStore(log, &state.FileStoreConfig{FilePath: t.TempDir() + "/state.json"})
	if err != nil {
		t.Fatalf("NewFileStore failed: %v", err)
	}
	defer func() { _ = kv.Close() }()

	store := NewPersistentStore(kv)
	run := GoalRun{
		ID:                      "gr-test",
		Name:                    "goal",
		Goal:                    "do thing",
		NaturalLanguageCriteria: "confirm thing",
		Status:                  GoalStatusReady,
		RiskLevel:               RiskBalanced,
		AllowAutoScope:          true,
		CreatedBy:               "alice",
		CreatedAt:               time.Now().UTC(),
		UpdatedAt:               time.Now().UTC(),
	}
	if _, err := store.CreateGoalRun(t.Context(), run); err != nil {
		t.Fatalf("CreateGoalRun failed: %v", err)
	}
	if err := store.SaveCriteria(t.Context(), run.ID, criteria.Set{
		Criteria: []criteria.Item{{ID: "manual-1", Title: "Manual", Type: criteria.TypeManualConfirmation, Required: true}},
	}); err != nil {
		t.Fatalf("SaveCriteria failed: %v", err)
	}
	if err := store.AppendEvent(t.Context(), Event{ID: "evt-1", GoalRunID: run.ID, Type: "created", Message: "created"}); err != nil {
		t.Fatalf("AppendEvent failed: %v", err)
	}
	if err := store.SaveWorkers(t.Context(), run.ID, []WorkerRef{{ID: "gw-1", Name: "worker"}}); err != nil {
		t.Fatalf("SaveWorkers failed: %v", err)
	}

	reloaded := NewPersistentStore(kv)
	gotRun, ok, err := reloaded.GetGoalRun(t.Context(), run.ID)
	if err != nil || !ok {
		t.Fatalf("GetGoalRun failed: ok=%v err=%v", ok, err)
	}
	if gotRun.Name != run.Name || gotRun.Status != run.Status {
		t.Fatalf("unexpected reloaded run: %+v", gotRun)
	}
	gotCriteria, ok, err := reloaded.LoadCriteria(t.Context(), run.ID)
	if err != nil || !ok || len(gotCriteria.Criteria) != 1 {
		t.Fatalf("LoadCriteria failed: ok=%v err=%v criteria=%+v", ok, err, gotCriteria)
	}
	gotEvents, err := reloaded.ListEvents(t.Context(), run.ID)
	if err != nil || len(gotEvents) != 1 {
		t.Fatalf("ListEvents failed: err=%v events=%+v", err, gotEvents)
	}
	gotWorkers, err := reloaded.LoadWorkers(t.Context(), run.ID)
	if err != nil || len(gotWorkers) != 1 {
		t.Fatalf("LoadWorkers failed: err=%v workers=%+v", err, gotWorkers)
	}
}

func TestKVStoreListGoalRunsDiscoversRunWithoutIndex(t *testing.T) {
	t.Parallel()

	log, err := logger.New(&logger.Config{Level: logger.LevelInfo, Development: true})
	if err != nil {
		t.Fatalf("logger.New failed: %v", err)
	}
	kv, err := state.NewFileStore(log, &state.FileStoreConfig{FilePath: t.TempDir() + "/state.json"})
	if err != nil {
		t.Fatalf("NewFileStore failed: %v", err)
	}
	defer func() { _ = kv.Close() }()

	run := GoalRun{
		ID:                      "gr-orphan",
		Name:                    "orphan",
		Goal:                    "recover me",
		NaturalLanguageCriteria: "confirm",
		Status:                  GoalStatusRunning,
		RiskLevel:               RiskBalanced,
		CreatedBy:               "alice",
		CreatedAt:               time.Now().UTC(),
		UpdatedAt:               time.Now().UTC(),
	}
	if err := kv.Set(t.Context(), goalRunKey(run.ID), run); err != nil {
		t.Fatalf("seed raw goal run failed: %v", err)
	}

	store := NewPersistentStore(kv)
	items, err := store.ListGoalRuns(t.Context())
	if err != nil {
		t.Fatalf("ListGoalRuns failed: %v", err)
	}
	if len(items) != 1 || items[0].ID != run.ID {
		t.Fatalf("expected orphan run to be discovered, got %+v", items)
	}
}

func TestKVStoreDiscoveryPersistsIndexWhenMissing(t *testing.T) {
	t.Parallel()

	log, err := logger.New(&logger.Config{Level: logger.LevelInfo, Development: true})
	if err != nil {
		t.Fatalf("logger.New failed: %v", err)
	}
	kv, err := state.NewFileStore(log, &state.FileStoreConfig{FilePath: t.TempDir() + "/state.json"})
	if err != nil {
		t.Fatalf("NewFileStore failed: %v", err)
	}
	defer func() { _ = kv.Close() }()

	run := GoalRun{
		ID:                      "gr-discover",
		Name:                    "discover",
		Goal:                    "discover me",
		NaturalLanguageCriteria: "confirm",
		Status:                  GoalStatusReady,
		RiskLevel:               RiskBalanced,
		CreatedBy:               "alice",
		CreatedAt:               time.Now().UTC(),
		UpdatedAt:               time.Now().UTC(),
	}
	if err := kv.Set(t.Context(), goalRunKey(run.ID), run); err != nil {
		t.Fatalf("seed goal run failed: %v", err)
	}

	store := NewPersistentStore(kv)
	if _, err := store.ListGoalRuns(t.Context()); err != nil {
		t.Fatalf("ListGoalRuns failed: %v", err)
	}
	indexValue, ok, err := kv.Get(t.Context(), goalRunIndexKey)
	if err != nil || !ok {
		t.Fatalf("expected discovered index to persist, ok=%v err=%v", ok, err)
	}
	index := decodeIndex(indexValue)
	if len(index) != 1 || index[0] != run.ID {
		t.Fatalf("expected persisted index to include %s, got %+v", run.ID, index)
	}
}

func TestKVStoreCreateGoalRunRollsBackWhenIndexUpdateFails(t *testing.T) {
	t.Parallel()

	log, err := logger.New(&logger.Config{Level: logger.LevelInfo, Development: true})
	if err != nil {
		t.Fatalf("logger.New failed: %v", err)
	}
	baseKV, err := state.NewFileStore(log, &state.FileStoreConfig{FilePath: t.TempDir() + "/state.json"})
	if err != nil {
		t.Fatalf("NewFileStore failed: %v", err)
	}
	defer func() { _ = baseKV.Close() }()

	store := NewPersistentStore(failIndexKV{KV: baseKV})
	run := GoalRun{
		ID:                      "gr-index-fail",
		Name:                    "index fail",
		Goal:                    "must rollback",
		NaturalLanguageCriteria: "confirm",
		Status:                  GoalStatusReady,
		RiskLevel:               RiskBalanced,
		CreatedBy:               "alice",
		CreatedAt:               time.Now().UTC(),
		UpdatedAt:               time.Now().UTC(),
	}
	if _, err := store.CreateGoalRun(t.Context(), run); err == nil {
		t.Fatal("expected create to fail when index update fails")
	}
	if exists, err := baseKV.Exists(t.Context(), goalRunKey(run.ID)); err != nil {
		t.Fatalf("Exists failed: %v", err)
	} else if exists {
		t.Fatalf("expected goal run key %s to be rolled back", goalRunKey(run.ID))
	}
}
