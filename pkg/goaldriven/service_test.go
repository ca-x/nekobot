package goaldriven

import (
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
