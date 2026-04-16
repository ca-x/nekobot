package goaldriven

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	daemonv1 "nekobot/gen/go/nekobot/daemon/v1"
	"nekobot/pkg/daemonhost"
	goaldrivencriteria "nekobot/pkg/goaldriven/criteria"
	goaldrivenscope "nekobot/pkg/goaldriven/scope"
	"nekobot/pkg/logger"
	"nekobot/pkg/state"
)

func TestServiceCreateGoalRunCreatesDraftWithCriteria(t *testing.T) {
	t.Parallel()

	svc := NewService(
		NewMemoryStore(),
		goaldrivencriteria.NewParser(),
		goaldrivencriteria.NewSchema(),
		goaldrivenscope.NewResolver(),
	)

	result, err := svc.CreateGoalRun(t.Context(), CreateGoalRunInput{
		Name:                    "daemon rollout",
		Goal:                    "fix daemon rollout on remote machine",
		NaturalLanguageCriteria: "confirm the daemon host rollout succeeded",
		RiskLevel:               RiskBalanced,
		AllowAutoScope:          true,
		CreatedBy:               "alice",
	})
	if err != nil {
		t.Fatalf("CreateGoalRun failed: %v", err)
	}
	if result.GoalRun.ID == "" {
		t.Fatalf("expected goal run id, got %+v", result.GoalRun)
	}
	if result.GoalRun.Status != GoalStatusCriteriaPendingConfirm {
		t.Fatalf("expected criteria pending confirm, got %q", result.GoalRun.Status)
	}
	if result.GoalRun.RecommendedScope == nil || result.GoalRun.RecommendedScope.Kind != ScopeDaemon {
		t.Fatalf("expected daemon recommended scope, got %+v", result.GoalRun.RecommendedScope)
	}
	if len(result.DraftCriteria.Criteria) != 1 {
		t.Fatalf("expected one draft criterion, got %+v", result.DraftCriteria.Criteria)
	}
	if got := result.DraftCriteria.Criteria[0].Type; got != goaldrivencriteria.TypeManualConfirmation {
		t.Fatalf("expected manual confirmation criterion, got %q", got)
	}
}

type failCriteriaStore struct {
	Store
}

func (s failCriteriaStore) SaveCriteria(ctx context.Context, goalRunID string, set goaldrivencriteria.Set) error {
	return fmt.Errorf("forced criteria failure")
}

type failRollbackStore struct {
	Store
}

func (s failRollbackStore) SaveCriteria(ctx context.Context, goalRunID string, set goaldrivencriteria.Set) error {
	return fmt.Errorf("forced criteria failure")
}

func (s failRollbackStore) DeleteGoalRun(ctx context.Context, goalRunID string) error {
	return fmt.Errorf("forced delete failure")
}

func TestServiceCreateGoalRunRollsBackWhenCriteriaSaveFails(t *testing.T) {
	t.Parallel()

	baseStore := NewMemoryStore()
	svc := NewService(
		failCriteriaStore{Store: baseStore},
		goaldrivencriteria.NewParser(),
		goaldrivencriteria.NewSchema(),
		goaldrivenscope.NewResolver(),
	)

	_, err := svc.CreateGoalRun(t.Context(), CreateGoalRunInput{
		Name:                    "rollback",
		Goal:                    "do not persist partial create",
		NaturalLanguageCriteria: "confirm manually",
		RiskLevel:               RiskBalanced,
		AllowAutoScope:          true,
		CreatedBy:               "alice",
	})
	if err == nil {
		t.Fatal("expected create to fail when criteria save fails")
	}

	items, err := baseStore.ListGoalRuns(t.Context())
	if err != nil {
		t.Fatalf("ListGoalRuns failed: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected rollback to remove partial goal runs, got %+v", items)
	}
}

func TestServiceCreateGoalRunReportsRollbackFailure(t *testing.T) {
	t.Parallel()

	baseStore := NewMemoryStore()
	svc := NewService(
		failRollbackStore{Store: baseStore},
		goaldrivencriteria.NewParser(),
		goaldrivencriteria.NewSchema(),
		goaldrivenscope.NewResolver(),
	)

	_, err := svc.CreateGoalRun(t.Context(), CreateGoalRunInput{
		Name:                    "rollback failure",
		Goal:                    "surface rollback failure",
		NaturalLanguageCriteria: "confirm manually",
		RiskLevel:               RiskBalanced,
		AllowAutoScope:          true,
		CreatedBy:               "alice",
	})
	if err == nil {
		t.Fatal("expected create to fail")
	}
	if !strings.Contains(err.Error(), "rollback failed") {
		t.Fatalf("expected rollback failure details, got %v", err)
	}
}

func TestServiceConfirmCriteriaMarksGoalRunReady(t *testing.T) {
	t.Parallel()

	svc := NewService(
		NewMemoryStore(),
		goaldrivencriteria.NewParser(),
		goaldrivencriteria.NewSchema(),
		goaldrivenscope.NewResolver(),
	)

	created, err := svc.CreateGoalRun(t.Context(), CreateGoalRunInput{
		Name:                    "server build",
		Goal:                    "verify local build",
		NaturalLanguageCriteria: "go build ./cmd/nekobot",
		RiskLevel:               RiskBalanced,
		AllowAutoScope:          true,
		CreatedBy:               "alice",
	})
	if err != nil {
		t.Fatalf("CreateGoalRun failed: %v", err)
	}

	set := goaldrivencriteria.Set{
		Criteria: []goaldrivencriteria.Item{
			{
				ID:       "build-pass",
				Title:    "Build passes",
				Type:     goaldrivencriteria.TypeCommand,
				Scope:    ExecutionScope{Kind: ScopeServer, Source: "manual"},
				Required: true,
				Status:   goaldrivencriteria.StatusPending,
				Definition: map[string]any{
					"command":          "go build ./cmd/nekobot",
					"expect_exit_code": 0,
				},
			},
		},
	}

	updated, err := svc.ConfirmCriteria(
		t.Context(),
		created.GoalRun.ID,
		set,
		&ExecutionScope{Kind: ScopeServer, Source: "manual"},
	)
	if err != nil {
		t.Fatalf("ConfirmCriteria failed: %v", err)
	}
	if updated.Status != GoalStatusReady {
		t.Fatalf("expected ready status, got %q", updated.Status)
	}
	if updated.SelectedScope == nil || updated.SelectedScope.Kind != ScopeServer {
		t.Fatalf("expected selected scope server, got %+v", updated.SelectedScope)
	}

	detail, ok, err := svc.GetGoalRunDetail(t.Context(), created.GoalRun.ID)
	if err != nil || !ok {
		t.Fatalf("GetGoalRunDetail failed: ok=%v err=%v", ok, err)
	}
	if len(detail.Criteria.Criteria) != 1 {
		t.Fatalf("expected confirmed criteria, got %+v", detail.Criteria.Criteria)
	}
}

func TestServiceConfirmCriteriaRejectsDaemonScopeWithoutMachineID(t *testing.T) {
	t.Parallel()

	svc := NewService(
		NewMemoryStore(),
		goaldrivencriteria.NewParser(),
		goaldrivencriteria.NewSchema(),
		goaldrivenscope.NewResolver(),
	)

	created, err := svc.CreateGoalRun(t.Context(), CreateGoalRunInput{
		Name:                    "daemon goal",
		Goal:                    "fix daemon machine rollout",
		NaturalLanguageCriteria: "confirm daemon rollout manually",
		RiskLevel:               RiskBalanced,
		AllowAutoScope:          false,
		CreatedBy:               "alice",
	})
	if err != nil {
		t.Fatalf("CreateGoalRun failed: %v", err)
	}

	_, err = svc.ConfirmCriteria(t.Context(), created.GoalRun.ID, goaldrivencriteria.Set{
		Criteria: []goaldrivencriteria.Item{
			{
				ID:       "manual-1",
				Title:    "Manual confirmation",
				Type:     goaldrivencriteria.TypeManualConfirmation,
				Scope:    ExecutionScope{Kind: ScopeDaemon, Source: "manual"},
				Required: true,
				Status:   goaldrivencriteria.StatusPending,
				Definition: map[string]any{
					"prompt": "Confirm success",
				},
				UpdatedAt: time.Now().UTC(),
			},
		},
	}, &ExecutionScope{Kind: ScopeDaemon, Source: "manual"})
	if err == nil {
		t.Fatal("expected daemon scope without machine_id to fail")
	}
}

func TestServiceAutofillsOnlyOnlineDaemonMachine(t *testing.T) {
	t.Parallel()

	log, err := logger.New(&logger.Config{Level: logger.LevelInfo, Development: true})
	if err != nil {
		t.Fatalf("logger.New failed: %v", err)
	}
	store, err := state.NewFileStore(log, &state.FileStoreConfig{FilePath: t.TempDir() + "/state.json"})
	if err != nil {
		t.Fatalf("NewFileStore failed: %v", err)
	}
	defer func() { _ = store.Close() }()

	registry := daemonhost.NewRegistry(store)
	_, err = registry.Register(t.Context(), &daemonv1.RegisterMachineRequest{
		Info: &daemonv1.DaemonInfo{
			DaemonId:     "daemon-a",
			MachineId:    "machine-a",
			MachineName:  "machine-a",
			Status:       "online",
			LastSeenUnix: time.Now().Unix(),
			DaemonUrl:    "http://127.0.0.1:7777",
		},
		Inventory: &daemonv1.RuntimeInventory{
			Workspaces: []*daemonv1.Workspace{
				{WorkspaceId: "machine-a:default", MachineId: "machine-a", Path: "/tmp/demo", DisplayName: "default", IsDefault: true},
			},
			Runtimes: []*daemonv1.Runtime{
				{RuntimeId: "machine-a:default:codex", MachineId: "machine-a", WorkspaceId: "machine-a:default", Kind: "codex", Installed: true, Healthy: true},
			},
		},
	})
	if err != nil {
		t.Fatalf("register daemon: %v", err)
	}

	svc := NewService(
		NewMemoryStore(),
		goaldrivencriteria.NewParser(),
		goaldrivencriteria.NewSchema(),
		goaldrivenscope.NewResolver(),
	)
	svc.SetKVStore(store)

	created, err := svc.CreateGoalRun(t.Context(), CreateGoalRunInput{
		Name:                    "daemon goal",
		Goal:                    "fix daemon machine rollout",
		NaturalLanguageCriteria: "confirm daemon rollout manually",
		RiskLevel:               RiskBalanced,
		AllowAutoScope:          true,
		CreatedBy:               "alice",
	})
	if err != nil {
		t.Fatalf("CreateGoalRun failed: %v", err)
	}
	if created.GoalRun.RecommendedScope == nil || created.GoalRun.RecommendedScope.MachineID != "machine-a" {
		t.Fatalf("expected recommended daemon machine to autofill, got %+v", created.GoalRun.RecommendedScope)
	}
}

func TestServiceConfirmCriteriaRejectsUnrunnableDaemonMachine(t *testing.T) {
	t.Parallel()

	log, err := logger.New(&logger.Config{Level: logger.LevelInfo, Development: true})
	if err != nil {
		t.Fatalf("logger.New failed: %v", err)
	}
	store, err := state.NewFileStore(log, &state.FileStoreConfig{FilePath: t.TempDir() + "/state.json"})
	if err != nil {
		t.Fatalf("NewFileStore failed: %v", err)
	}
	defer func() { _ = store.Close() }()

	registry := daemonhost.NewRegistry(store)
	_, err = registry.Register(t.Context(), &daemonv1.RegisterMachineRequest{
		Info: &daemonv1.DaemonInfo{
			DaemonId:     "daemon-a",
			MachineId:    "machine-a",
			MachineName:  "machine-a",
			Status:       "online",
			LastSeenUnix: time.Now().Unix(),
			DaemonUrl:    "http://127.0.0.1:7777",
		},
		Inventory: &daemonv1.RuntimeInventory{
			Workspaces: []*daemonv1.Workspace{
				{WorkspaceId: "machine-a:default", MachineId: "machine-a", Path: "/tmp/demo", DisplayName: "default", IsDefault: true},
			},
			Runtimes: []*daemonv1.Runtime{
				{RuntimeId: "machine-a:default:codex", MachineId: "machine-a", WorkspaceId: "machine-a:default", Kind: "codex", Installed: false, Healthy: false},
			},
		},
	})
	if err != nil {
		t.Fatalf("register daemon: %v", err)
	}

	svc := NewService(
		NewMemoryStore(),
		goaldrivencriteria.NewParser(),
		goaldrivencriteria.NewSchema(),
		goaldrivenscope.NewResolver(),
	)
	svc.SetKVStore(store)

	created, err := svc.CreateGoalRun(t.Context(), CreateGoalRunInput{
		Name:                    "daemon goal",
		Goal:                    "fix daemon machine rollout",
		NaturalLanguageCriteria: "confirm daemon rollout manually",
		RiskLevel:               RiskBalanced,
		AllowAutoScope:          false,
		CreatedBy:               "alice",
	})
	if err != nil {
		t.Fatalf("CreateGoalRun failed: %v", err)
	}

	_, err = svc.ConfirmCriteria(t.Context(), created.GoalRun.ID, goaldrivencriteria.Set{
		Criteria: []goaldrivencriteria.Item{
			{
				ID:       "manual-1",
				Title:    "Manual confirmation",
				Type:     goaldrivencriteria.TypeManualConfirmation,
				Scope:    ExecutionScope{Kind: ScopeDaemon, MachineID: "machine-a", Source: "manual"},
				Required: true,
				Status:   goaldrivencriteria.StatusPending,
				Definition: map[string]any{
					"prompt": "Confirm success",
				},
				UpdatedAt: time.Now().UTC(),
			},
		},
	}, &ExecutionScope{Kind: ScopeDaemon, MachineID: "machine-a", Source: "manual"})
	if err == nil {
		t.Fatal("expected unrunnable daemon machine to fail ready transition")
	}
}

func TestServiceAutofillSkipsOnlineDaemonWithoutHealthyRuntime(t *testing.T) {
	t.Parallel()

	log, err := logger.New(&logger.Config{Level: logger.LevelInfo, Development: true})
	if err != nil {
		t.Fatalf("logger.New failed: %v", err)
	}
	store, err := state.NewFileStore(log, &state.FileStoreConfig{FilePath: t.TempDir() + "/state.json"})
	if err != nil {
		t.Fatalf("NewFileStore failed: %v", err)
	}
	defer func() { _ = store.Close() }()

	registry := daemonhost.NewRegistry(store)
	_, err = registry.Register(t.Context(), &daemonv1.RegisterMachineRequest{
		Info: &daemonv1.DaemonInfo{
			DaemonId:     "daemon-a",
			MachineId:    "machine-a",
			MachineName:  "machine-a",
			Status:       "online",
			LastSeenUnix: time.Now().Unix(),
			DaemonUrl:    "http://127.0.0.1:7777",
		},
		Inventory: &daemonv1.RuntimeInventory{
			Workspaces: []*daemonv1.Workspace{
				{WorkspaceId: "machine-a:default", MachineId: "machine-a", Path: "/tmp/demo", DisplayName: "default", IsDefault: true},
			},
			Runtimes: []*daemonv1.Runtime{
				{RuntimeId: "machine-a:default:codex", MachineId: "machine-a", WorkspaceId: "machine-a:default", Kind: "codex", Installed: false, Healthy: false},
			},
		},
	})
	if err != nil {
		t.Fatalf("register daemon: %v", err)
	}

	svc := NewService(
		NewMemoryStore(),
		goaldrivencriteria.NewParser(),
		goaldrivencriteria.NewSchema(),
		goaldrivenscope.NewResolver(),
	)
	svc.SetKVStore(store)

	created, err := svc.CreateGoalRun(t.Context(), CreateGoalRunInput{
		Name:                    "daemon goal",
		Goal:                    "fix daemon machine rollout",
		NaturalLanguageCriteria: "confirm daemon rollout manually",
		RiskLevel:               RiskBalanced,
		AllowAutoScope:          true,
		CreatedBy:               "alice",
	})
	if err != nil {
		t.Fatalf("CreateGoalRun failed: %v", err)
	}
	if created.GoalRun.RecommendedScope == nil {
		t.Fatalf("expected recommended scope, got nil")
	}
	if created.GoalRun.RecommendedScope.MachineID != "" {
		t.Fatalf("expected no autofilled machine for unrunnable daemon, got %+v", created.GoalRun.RecommendedScope)
	}
}

func TestServiceResumeActiveRunsRestartsLoop(t *testing.T) {
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
	svc := NewService(
		store,
		goaldrivencriteria.NewParser(),
		goaldrivencriteria.NewSchema(),
		goaldrivenscope.NewResolver(),
	)
	svc.SetServerRunner(func(_ context.Context, run GoalRun) (WorkerRef, error) {
		return WorkerRef{
			ID:              "gw-resume",
			Name:            "server-worker",
			Status:          "completed",
			Scope:           *run.SelectedScope,
			TaskID:          "task-resume",
			LastHeartbeatAt: time.Now().UTC(),
			LastProgressAt:  time.Now().UTC(),
			LeaseExpiresAt:  time.Now().UTC().Add(5 * time.Minute),
		}, nil
	})

	run := GoalRun{
		ID:                      "gr-resume",
		Name:                    "resume",
		Goal:                    "resume goal",
		NaturalLanguageCriteria: "confirm manually",
		Status:                  GoalStatusRunning,
		RiskLevel:               RiskBalanced,
		AllowAutoScope:          true,
		SelectedScope:           &ExecutionScope{Kind: ScopeServer, Source: "manual"},
		CreatedBy:               "alice",
		CreatedAt:               time.Now().UTC(),
		UpdatedAt:               time.Now().UTC(),
	}
	if _, err := store.CreateGoalRun(t.Context(), run); err != nil {
		t.Fatalf("CreateGoalRun failed: %v", err)
	}
	if err := store.SaveCriteria(t.Context(), run.ID, goaldrivencriteria.Set{
		Criteria: []goaldrivencriteria.Item{
			{
				ID:       "manual-1",
				Title:    "Manual confirmation",
				Type:     goaldrivencriteria.TypeManualConfirmation,
				Scope:    ExecutionScope{Kind: ScopeServer, Source: "manual"},
				Required: true,
				Status:   goaldrivencriteria.StatusPending,
				Definition: map[string]any{
					"prompt": "Confirm success",
				},
				UpdatedAt: time.Now().UTC(),
			},
		},
	}); err != nil {
		t.Fatalf("SaveCriteria failed: %v", err)
	}

	if err := svc.ResumeActiveRuns(t.Context()); err != nil {
		t.Fatalf("ResumeActiveRuns failed: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		detail, ok, err := svc.GetGoalRunDetail(t.Context(), run.ID)
		if err == nil && ok && detail.GoalRun.Status == GoalStatusNeedsHumanConfirmation {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	detail, _, _ := svc.GetGoalRunDetail(t.Context(), run.ID)
	t.Fatalf("expected resumed run to reach needs_human_confirmation, got %+v", detail.GoalRun)
}

func TestServiceResumeVerifyingRunDoesNotReexecuteWorker(t *testing.T) {
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
	svc := NewService(
		store,
		goaldrivencriteria.NewParser(),
		goaldrivencriteria.NewSchema(),
		goaldrivenscope.NewResolver(),
	)
	serverRuns := 0
	svc.SetServerRunner(func(_ context.Context, run GoalRun) (WorkerRef, error) {
		serverRuns++
		return WorkerRef{
			ID:              "gw-verifying",
			Name:            "server-worker",
			Status:          "completed",
			Scope:           *run.SelectedScope,
			TaskID:          "task-verifying",
			LastHeartbeatAt: time.Now().UTC(),
			LastProgressAt:  time.Now().UTC(),
			LeaseExpiresAt:  time.Now().UTC().Add(5 * time.Minute),
		}, nil
	})

	run := GoalRun{
		ID:                      "gr-verifying",
		Name:                    "verifying",
		Goal:                    "verify goal",
		NaturalLanguageCriteria: "confirm manually",
		Status:                  GoalStatusVerifying,
		RiskLevel:               RiskBalanced,
		SelectedScope:           &ExecutionScope{Kind: ScopeServer, Source: "manual"},
		CreatedBy:               "alice",
		CreatedAt:               time.Now().UTC(),
		UpdatedAt:               time.Now().UTC(),
	}
	if _, err := store.CreateGoalRun(t.Context(), run); err != nil {
		t.Fatalf("CreateGoalRun failed: %v", err)
	}
	if err := store.SaveCriteria(t.Context(), run.ID, goaldrivencriteria.Set{
		Criteria: []goaldrivencriteria.Item{
			{
				ID:       "manual-1",
				Title:    "Manual confirmation",
				Type:     goaldrivencriteria.TypeManualConfirmation,
				Scope:    ExecutionScope{Kind: ScopeServer, Source: "manual"},
				Required: true,
				Status:   goaldrivencriteria.StatusPending,
				Definition: map[string]any{
					"prompt": "Confirm success",
				},
				UpdatedAt: time.Now().UTC(),
			},
		},
	}); err != nil {
		t.Fatalf("SaveCriteria failed: %v", err)
	}

	if err := svc.ResumeActiveRuns(t.Context()); err != nil {
		t.Fatalf("ResumeActiveRuns failed: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		detail, ok, err := svc.GetGoalRunDetail(t.Context(), run.ID)
		if err == nil && ok && detail.GoalRun.Status == GoalStatusNeedsHumanConfirmation {
			if serverRuns != 0 {
				t.Fatalf("expected no worker re-execution for verifying run, got %d", serverRuns)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	detail, _, _ := svc.GetGoalRunDetail(t.Context(), run.ID)
	t.Fatalf("expected verifying run to reach needs_human_confirmation, got %+v", detail.GoalRun)
}

func TestServiceResumeVerifyingRunFallsBackToReadyWhenVerificationStillUnmet(t *testing.T) {
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
	svc := NewService(
		store,
		goaldrivencriteria.NewParser(),
		goaldrivencriteria.NewSchema(),
		goaldrivenscope.NewResolver(),
	)

	serverRuns := 0
	svc.SetServerRunner(func(_ context.Context, run GoalRun) (WorkerRef, error) {
		serverRuns++
		return WorkerRef{
			ID:              "gw-reenter",
			Name:            "server-worker",
			Status:          "completed",
			Scope:           *run.SelectedScope,
			TaskID:          "task-reenter",
			LastHeartbeatAt: time.Now().UTC(),
			LastProgressAt:  time.Now().UTC(),
			LeaseExpiresAt:  time.Now().UTC().Add(5 * time.Minute),
		}, nil
	})

	run := GoalRun{
		ID:                      "gr-verifying-running",
		Name:                    "verifying-running",
		Goal:                    "verify command and then rerun",
		NaturalLanguageCriteria: "go build ./cmd/nekobot",
		Status:                  GoalStatusVerifying,
		RiskLevel:               RiskBalanced,
		SelectedScope:           &ExecutionScope{Kind: ScopeServer, Source: "manual"},
		CreatedBy:               "alice",
		CreatedAt:               time.Now().UTC(),
		UpdatedAt:               time.Now().UTC(),
	}
	if _, err := store.CreateGoalRun(t.Context(), run); err != nil {
		t.Fatalf("CreateGoalRun failed: %v", err)
	}
	if err := store.SaveCriteria(t.Context(), run.ID, goaldrivencriteria.Set{
		Criteria: []goaldrivencriteria.Item{
			{
				ID:       "cmd-1",
				Title:    "Impossible command",
				Type:     goaldrivencriteria.TypeCommand,
				Scope:    ExecutionScope{Kind: ScopeServer, Source: "manual"},
				Required: true,
				Status:   goaldrivencriteria.StatusPending,
				Definition: map[string]any{
					"command":          "false",
					"expect_exit_code": 0,
				},
				UpdatedAt: time.Now().UTC(),
			},
		},
	}); err != nil {
		t.Fatalf("SaveCriteria failed: %v", err)
	}

	if err := svc.ResumeActiveRuns(t.Context()); err != nil {
		t.Fatalf("ResumeActiveRuns failed: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		detail, ok, err := svc.GetGoalRunDetail(t.Context(), run.ID)
		if err == nil && ok && detail.GoalRun.Status == GoalStatusReady {
			if serverRuns != 0 {
				t.Fatalf("expected no worker re-execution for resumed verifying run, got %d", serverRuns)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	detail, _, _ := svc.GetGoalRunDetail(t.Context(), run.ID)
	t.Fatalf("expected resumed verifying-running path to fall back to ready, got %+v", detail.GoalRun)
}

func TestPersistentStoreListsActiveRunForResume(t *testing.T) {
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
		ID:                      "gr-active",
		Name:                    "active",
		Goal:                    "active goal",
		NaturalLanguageCriteria: "confirm manually",
		Status:                  GoalStatusRunning,
		RiskLevel:               RiskBalanced,
		SelectedScope:           &ExecutionScope{Kind: ScopeServer, Source: "manual"},
		CreatedBy:               "alice",
		CreatedAt:               time.Now().UTC(),
		UpdatedAt:               time.Now().UTC(),
	}
	if _, err := store.CreateGoalRun(t.Context(), run); err != nil {
		t.Fatalf("CreateGoalRun failed: %v", err)
	}
	items, err := store.ListGoalRuns(t.Context())
	if err != nil {
		t.Fatalf("ListGoalRuns failed: %v", err)
	}
	if len(items) != 1 || items[0].ID != run.ID || items[0].Status != GoalStatusRunning {
		t.Fatalf("unexpected runs after persistence round-trip: %+v", items)
	}
}
