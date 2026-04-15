package goaldriven

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	daemonv1 "nekobot/gen/go/nekobot/daemon/v1"
	"nekobot/pkg/agent"
	"nekobot/pkg/daemonhost"
	"nekobot/pkg/goaldriven/criteria"
	"nekobot/pkg/goaldriven/scope"
	"nekobot/pkg/goaldriven/shared"
	"nekobot/pkg/logger"
	"nekobot/pkg/state"
	"nekobot/pkg/tasks"
)

// Service manages GoalRun CRUD and criteria confirmation for the first vertical slice.
type Service struct {
	store         Store
	parser        *criteria.Parser
	schema        *criteria.Schema
	scopeResolver *scope.Resolver
	now           func() time.Time
	log           *logger.Logger
	agent         *agent.Agent
	kvStore       state.KV

	mu     sync.Mutex
	active map[string]context.CancelFunc

	serverRunner func(context.Context, GoalRun) (WorkerRef, error)
	daemonRunner func(context.Context, GoalRun) (WorkerRef, error)
}

// NewService creates a GoalDriven service.
func NewService(store Store, parser *criteria.Parser, schema *criteria.Schema, scopeResolver *scope.Resolver) *Service {
	return &Service{
		store:         store,
		parser:        parser,
		schema:        schema,
		scopeResolver: scopeResolver,
		now:           time.Now,
		active:        make(map[string]context.CancelFunc),
	}
}

// CreateGoalRunInput is the create request.
type CreateGoalRunInput struct {
	Name                    string
	Goal                    string
	NaturalLanguageCriteria string
	RiskLevel               RiskLevel
	AllowAutoScope          bool
	SelectedScope           *ExecutionScope
	CreatedBy               string
}

// CreateGoalRunOutput returns the created GoalRun plus draft criteria.
type CreateGoalRunOutput struct {
	GoalRun       GoalRun      `json:"goal_run"`
	DraftCriteria criteria.Set `json:"draft_criteria"`
}

// GoalRunDetail is the read model for the details page.
type GoalRunDetail struct {
	GoalRun  GoalRun      `json:"goal_run"`
	Criteria criteria.Set `json:"criteria"`
	Workers  []WorkerRef  `json:"workers"`
}

// CreateGoalRun creates a GoalRun plus draft criteria.
func (s *Service) CreateGoalRun(ctx context.Context, in CreateGoalRunInput) (CreateGoalRunOutput, error) {
	if s == nil {
		return CreateGoalRunOutput{}, fmt.Errorf("service is nil")
	}
	if err := validateCreateInput(in); err != nil {
		return CreateGoalRunOutput{}, err
	}

	recommendedScope, err := s.scopeResolver.Resolve(ctx, scope.ResolveInput{
		Goal:            in.Goal,
		NaturalCriteria: in.NaturalLanguageCriteria,
		AllowAutoScope:  in.AllowAutoScope,
		SelectedScope:   toSharedScope(in.SelectedScope),
	})
	if err != nil {
		return CreateGoalRunOutput{}, fmt.Errorf("resolve scope: %w", err)
	}
	recommendedScope = s.autofillRecommendedDaemonMachine(ctx, recommendedScope)

	draftCriteria, err := s.parser.Parse(ctx, criteria.ParseInput{
		Goal:      in.Goal,
		Natural:   in.NaturalLanguageCriteria,
		Scope:     recommendedScope,
		RiskLevel: in.RiskLevel,
	})
	if err != nil {
		return CreateGoalRunOutput{}, fmt.Errorf("parse criteria: %w", err)
	}
	if err := s.schema.Validate(draftCriteria); err != nil {
		return CreateGoalRunOutput{}, fmt.Errorf("validate criteria: %w", err)
	}

	now := s.now().UTC()
	run := GoalRun{
		ID:                      "gr_" + uuid.NewString(),
		Name:                    strings.TrimSpace(in.Name),
		Goal:                    strings.TrimSpace(in.Goal),
		NaturalLanguageCriteria: strings.TrimSpace(in.NaturalLanguageCriteria),
		Status:                  GoalStatusCriteriaPendingConfirm,
		RiskLevel:               in.RiskLevel,
		AllowAutoScope:          in.AllowAutoScope,
		AllowParallelWorkers:    false,
		RecommendedScope:        fromSharedScope(recommendedScope),
		SelectedScope:           cloneScopePtr(in.SelectedScope),
		CreatedBy:               strings.TrimSpace(in.CreatedBy),
		CreatedAt:               now,
		UpdatedAt:               now,
	}

	created, err := s.store.CreateGoalRun(ctx, run)
	if err != nil {
		return CreateGoalRunOutput{}, fmt.Errorf("create goal run: %w", err)
	}
	if err := s.store.SaveCriteria(ctx, created.ID, draftCriteria); err != nil {
		return CreateGoalRunOutput{}, fmt.Errorf("save draft criteria: %w", err)
	}
	return CreateGoalRunOutput{GoalRun: created, DraftCriteria: draftCriteria}, nil
}

// ConfirmCriteria stores the user-confirmed criteria and marks the GoalRun ready.
func (s *Service) ConfirmCriteria(ctx context.Context, goalRunID string, set criteria.Set, selectedScope *ExecutionScope) (GoalRun, error) {
	if s == nil {
		return GoalRun{}, fmt.Errorf("service is nil")
	}
	if strings.TrimSpace(goalRunID) == "" {
		return GoalRun{}, fmt.Errorf("goal run id is required")
	}
	if err := s.schema.Validate(set); err != nil {
		return GoalRun{}, fmt.Errorf("validate criteria: %w", err)
	}

	run, ok, err := s.store.GetGoalRun(ctx, goalRunID)
	if err != nil {
		return GoalRun{}, fmt.Errorf("get goal run: %w", err)
	}
	if !ok {
		return GoalRun{}, fmt.Errorf("goal run %s not found", goalRunID)
	}
	if err := ValidateTransition(run.Status, GoalStatusReady); err != nil {
		return GoalRun{}, err
	}

	if selectedScope != nil {
		run.SelectedScope = cloneScopePtr(selectedScope)
	} else if run.SelectedScope == nil && run.RecommendedScope != nil {
		run.SelectedScope = cloneScopePtr(run.RecommendedScope)
	}
	if err := s.validateRunnableScope(ctx, run.SelectedScope); err != nil {
		return GoalRun{}, err
	}
	run.Status = GoalStatusReady
	run.UpdatedAt = s.now().UTC()

	if err := s.store.SaveCriteria(ctx, goalRunID, set); err != nil {
		return GoalRun{}, fmt.Errorf("save criteria: %w", err)
	}
	updated, err := s.store.UpdateGoalRun(ctx, run)
	if err != nil {
		return GoalRun{}, fmt.Errorf("update goal run: %w", err)
	}
	return updated, nil
}

// ListGoalRuns returns all GoalRuns.
func (s *Service) ListGoalRuns(ctx context.Context) ([]GoalRun, error) {
	if s == nil {
		return nil, fmt.Errorf("service is nil")
	}
	return s.store.ListGoalRuns(ctx)
}

// GetGoalRunDetail returns one GoalRun plus its stored criteria.
func (s *Service) GetGoalRunDetail(ctx context.Context, goalRunID string) (GoalRunDetail, bool, error) {
	if s == nil {
		return GoalRunDetail{}, false, fmt.Errorf("service is nil")
	}
	run, ok, err := s.store.GetGoalRun(ctx, goalRunID)
	if err != nil || !ok {
		return GoalRunDetail{}, ok, err
	}
	set, _, err := s.store.LoadCriteria(ctx, goalRunID)
	if err != nil {
		return GoalRunDetail{}, false, fmt.Errorf("load criteria: %w", err)
	}
	workers, err := s.store.LoadWorkers(ctx, goalRunID)
	if err != nil {
		return GoalRunDetail{}, false, fmt.Errorf("load workers: %w", err)
	}
	return GoalRunDetail{GoalRun: run, Criteria: set, Workers: workers}, true, nil
}

// SetLogger attaches the application logger.
func (s *Service) SetLogger(log *logger.Logger) {
	if s == nil {
		return
	}
	s.log = log
}

// SetAgent attaches the application agent.
func (s *Service) SetAgent(ag *agent.Agent) {
	if s == nil {
		return
	}
	s.agent = ag
}

// SetKVStore attaches the shared KV store.
func (s *Service) SetKVStore(kv state.KV) {
	if s == nil {
		return
	}
	s.kvStore = kv
}

// SetServerRunner overrides server-scope execution for tests or alternate executors.
func (s *Service) SetServerRunner(fn func(context.Context, GoalRun) (WorkerRef, error)) {
	if s == nil {
		return
	}
	s.serverRunner = fn
}

// SetDaemonRunner overrides daemon-scope execution for tests or alternate executors.
func (s *Service) SetDaemonRunner(fn func(context.Context, GoalRun) (WorkerRef, error)) {
	if s == nil {
		return
	}
	s.daemonRunner = fn
}

// StartGoalRun transitions one ready GoalRun into running and starts its orchestration loop.
func (s *Service) StartGoalRun(ctx context.Context, goalRunID string) (GoalRun, error) {
	run, ok, err := s.store.GetGoalRun(ctx, strings.TrimSpace(goalRunID))
	if err != nil {
		return GoalRun{}, fmt.Errorf("get goal run: %w", err)
	}
	if !ok {
		return GoalRun{}, fmt.Errorf("goal run %s not found", goalRunID)
	}
	if err := ValidateTransition(run.Status, GoalStatusRunning); err != nil {
		return GoalRun{}, err
	}
	now := s.now().UTC()
	run.Status = GoalStatusRunning
	run.StartedAt = now
	run.UpdatedAt = now
	updated, err := s.store.UpdateGoalRun(ctx, run)
	if err != nil {
		return GoalRun{}, fmt.Errorf("update goal run: %w", err)
	}

	runCtx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	if prev, exists := s.active[updated.ID]; exists {
		prev()
	}
	s.active[updated.ID] = cancel
	s.mu.Unlock()

	go s.runGoalLoop(runCtx, updated.ID)
	return updated, nil
}

// StopGoalRun stops active execution and returns the GoalRun to ready state.
func (s *Service) StopGoalRun(ctx context.Context, goalRunID string) (GoalRun, error) {
	run, ok, err := s.store.GetGoalRun(ctx, strings.TrimSpace(goalRunID))
	if err != nil {
		return GoalRun{}, fmt.Errorf("get goal run: %w", err)
	}
	if !ok {
		return GoalRun{}, fmt.Errorf("goal run %s not found", goalRunID)
	}
	s.stopActiveRun(run.ID)
	if run.Status == GoalStatusRunning || run.Status == GoalStatusVerifying || run.Status == GoalStatusNeedsHumanConfirmation {
		run.Status = GoalStatusReady
		run.UpdatedAt = s.now().UTC()
	}
	updated, err := s.store.UpdateGoalRun(ctx, run)
	if err != nil {
		return GoalRun{}, fmt.Errorf("update goal run: %w", err)
	}
	return updated, nil
}

// CancelGoalRun cancels one GoalRun permanently.
func (s *Service) CancelGoalRun(ctx context.Context, goalRunID string) (GoalRun, error) {
	run, ok, err := s.store.GetGoalRun(ctx, strings.TrimSpace(goalRunID))
	if err != nil {
		return GoalRun{}, fmt.Errorf("get goal run: %w", err)
	}
	if !ok {
		return GoalRun{}, fmt.Errorf("goal run %s not found", goalRunID)
	}
	s.stopActiveRun(run.ID)
	run.Status = GoalStatusCanceled
	run.UpdatedAt = s.now().UTC()
	run.CompletedAt = s.now().UTC()
	updated, err := s.store.UpdateGoalRun(ctx, run)
	if err != nil {
		return GoalRun{}, fmt.Errorf("update goal run: %w", err)
	}
	return updated, nil
}

// ConfirmManualCriterion marks one manual criterion and may complete the GoalRun.
func (s *Service) ConfirmManualCriterion(ctx context.Context, goalRunID, criterionID, note string, approved bool) (GoalRun, error) {
	run, ok, err := s.store.GetGoalRun(ctx, strings.TrimSpace(goalRunID))
	if err != nil {
		return GoalRun{}, fmt.Errorf("get goal run: %w", err)
	}
	if !ok {
		return GoalRun{}, fmt.Errorf("goal run %s not found", goalRunID)
	}
	if run.Status != GoalStatusNeedsHumanConfirmation {
		return GoalRun{}, fmt.Errorf("goal run %s is not waiting for manual confirmation", goalRunID)
	}
	set, ok, err := s.store.LoadCriteria(ctx, goalRunID)
	if err != nil {
		return GoalRun{}, fmt.Errorf("load criteria: %w", err)
	}
	if !ok {
		return GoalRun{}, fmt.Errorf("goal run %s criteria not found", goalRunID)
	}

	found := false
	allPassed := true
	for i := range set.Criteria {
		item := &set.Criteria[i]
		if item.ID != strings.TrimSpace(criterionID) {
			if item.Required && item.Status != criteria.StatusPassed {
				allPassed = false
			}
			continue
		}
		found = true
		if item.Type != criteria.TypeManualConfirmation {
			return GoalRun{}, fmt.Errorf("criterion %s is not manual_confirmation", criterionID)
		}
		item.UpdatedAt = s.now().UTC()
		if approved {
			item.Status = criteria.StatusPassed
			if strings.TrimSpace(note) != "" {
				item.Evidence = append(item.Evidence, strings.TrimSpace(note))
			}
		} else {
			item.Status = criteria.StatusFailed
			item.LastError = strings.TrimSpace(note)
			allPassed = false
		}
	}
	if !found {
		return GoalRun{}, fmt.Errorf("criterion %s not found", criterionID)
	}
	for _, item := range set.Criteria {
		if item.Required && item.Status != criteria.StatusPassed {
			allPassed = false
		}
	}
	if err := s.store.SaveCriteria(ctx, goalRunID, set); err != nil {
		return GoalRun{}, fmt.Errorf("save criteria: %w", err)
	}
	if allPassed {
		run.Status = GoalStatusCompleted
		run.CompletedAt = s.now().UTC()
	} else if !approved {
		run.Status = GoalStatusFailed
		run.CompletedAt = s.now().UTC()
	}
	run.UpdatedAt = s.now().UTC()
	return s.store.UpdateGoalRun(ctx, run)
}

func validateCreateInput(in CreateGoalRunInput) error {
	if strings.TrimSpace(in.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if strings.TrimSpace(in.Goal) == "" {
		return fmt.Errorf("goal is required")
	}
	if strings.TrimSpace(in.NaturalLanguageCriteria) == "" {
		return fmt.Errorf("natural language criteria is required")
	}
	if strings.TrimSpace(in.CreatedBy) == "" {
		return fmt.Errorf("created_by is required")
	}
	switch in.RiskLevel {
	case RiskConservative, RiskBalanced, RiskAggressive:
	default:
		return fmt.Errorf("risk level must be one of: conservative, balanced, aggressive")
	}
	return nil
}

func cloneScopePtr(scope *ExecutionScope) *ExecutionScope {
	if scope == nil {
		return nil
	}
	cloned := *scope
	return &cloned
}

func toSharedScope(scope *ExecutionScope) *shared.ExecutionScope {
	if scope == nil {
		return nil
	}
	cloned := shared.ExecutionScope(*scope)
	return &cloned
}

func fromSharedScope(scope *shared.ExecutionScope) *ExecutionScope {
	if scope == nil {
		return nil
	}
	cloned := ExecutionScope(*scope)
	return &cloned
}

func (s *Service) autofillRecommendedDaemonMachine(ctx context.Context, scope *shared.ExecutionScope) *shared.ExecutionScope {
	if scope == nil || scope.Kind != shared.ScopeDaemon || strings.TrimSpace(scope.MachineID) != "" || s.kvStore == nil {
		return scope
	}
	registry := daemonhost.NewRegistry(s.kvStore)
	snapshot, err := registry.Snapshot(ctx)
	if err != nil {
		return scope
	}
	machineIDs := make([]string, 0, len(snapshot.Machines))
	for machineID, info := range snapshot.Machines {
		if info == nil || strings.TrimSpace(info.DaemonUrl) == "" {
			continue
		}
		if daemonhost.DeriveMachineStatus(info, s.now()) != "online" {
			continue
		}
		inventory := snapshot.Inventories[machineID]
		if _, _, err := chooseDaemonRuntime(inventory); err != nil {
			continue
		}
		machineIDs = append(machineIDs, strings.TrimSpace(machineID))
	}
	if len(machineIDs) != 1 {
		return scope
	}
	cloned := *scope
	cloned.MachineID = machineIDs[0]
	if strings.TrimSpace(cloned.Reason) == "" {
		cloned.Reason = "autofilled daemon machine from single connected host"
	}
	return &cloned
}

func (s *Service) validateRunnableScope(ctx context.Context, selectedScope *ExecutionScope) error {
	if selectedScope == nil {
		return fmt.Errorf("selected scope is required")
	}
	if selectedScope.Kind != ScopeDaemon {
		return nil
	}
	machineID := strings.TrimSpace(selectedScope.MachineID)
	if machineID == "" {
		return fmt.Errorf("daemon machine_id is required")
	}
	if s.kvStore == nil {
		return nil
	}
	registry := daemonhost.NewRegistry(s.kvStore)
	snapshot, err := registry.Snapshot(ctx)
	if err != nil {
		return fmt.Errorf("load daemon registry snapshot: %w", err)
	}
	info := snapshot.Machines[machineID]
	if info == nil {
		return fmt.Errorf("daemon machine %s not found", machineID)
	}
	if strings.TrimSpace(info.DaemonUrl) == "" {
		return fmt.Errorf("daemon machine %s has no daemon_url", machineID)
	}
	if daemonhost.DeriveMachineStatus(info, s.now()) != "online" {
		return fmt.Errorf("daemon machine %s is not online", machineID)
	}
	inventory := snapshot.Inventories[machineID]
	if inventory == nil {
		return fmt.Errorf("daemon machine %s inventory not found", machineID)
	}
	if _, _, err := chooseDaemonRuntime(inventory); err != nil {
		return fmt.Errorf("daemon machine %s is not runnable: %w", machineID, err)
	}
	return nil
}

func (s *Service) stopActiveRun(goalRunID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cancel, ok := s.active[goalRunID]
	if !ok {
		return
	}
	cancel()
	delete(s.active, goalRunID)
}

func (s *Service) runGoalLoop(ctx context.Context, goalRunID string) {
	defer s.stopActiveRun(goalRunID)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		run, ok, err := s.store.GetGoalRun(ctx, goalRunID)
		if err != nil || !ok {
			return
		}
		set, ok, err := s.store.LoadCriteria(ctx, goalRunID)
		if err != nil || !ok {
			return
		}

		if run.SelectedScope == nil {
			run.SelectedScope = run.RecommendedScope
		}
		if run.SelectedScope == nil {
			run.Status = GoalStatusFailed
			run.UpdatedAt = s.now().UTC()
			run.CompletedAt = s.now().UTC()
			_, _ = s.store.UpdateGoalRun(ctx, run)
			return
		}

		worker, err := s.executeAttempt(ctx, run)
		if err == nil {
			_ = s.store.SaveWorkers(ctx, run.ID, []WorkerRef{worker})
		}
		if err != nil {
			run.Status = GoalStatusFailed
			run.UpdatedAt = s.now().UTC()
			run.CompletedAt = s.now().UTC()
			_, _ = s.store.UpdateGoalRun(ctx, run)
			return
		}

		run.Status = GoalStatusVerifying
		run.UpdatedAt = s.now().UTC()
		run.LastActivityAt = s.now().UTC()
		run.CurrentWorkerIDs = []string{worker.ID}
		run, err = s.store.UpdateGoalRun(ctx, run)
		if err != nil {
			return
		}

		evalRun, evalCriteria, err := s.evaluateCriteria(ctx, run, set)
		if err != nil {
			run.Status = GoalStatusFailed
			run.UpdatedAt = s.now().UTC()
			run.CompletedAt = s.now().UTC()
			_, _ = s.store.UpdateGoalRun(ctx, run)
			return
		}
		run = evalRun
		if err := s.store.SaveCriteria(ctx, run.ID, evalCriteria); err != nil {
			return
		}
		if run.Status == GoalStatusCompleted || run.Status == GoalStatusNeedsHumanConfirmation {
			_, _ = s.store.UpdateGoalRun(ctx, run)
			return
		}
		_, _ = s.store.UpdateGoalRun(ctx, run)
		time.Sleep(250 * time.Millisecond)
	}
}

func (s *Service) executeAttempt(ctx context.Context, run GoalRun) (WorkerRef, error) {
	if run.SelectedScope == nil {
		return WorkerRef{}, fmt.Errorf("selected scope is required")
	}
	switch run.SelectedScope.Kind {
	case ScopeServer:
		if s.serverRunner != nil {
			return s.serverRunner(ctx, run)
		}
		return s.executeServerAttempt(ctx, run)
	case ScopeDaemon:
		if s.daemonRunner != nil {
			return s.daemonRunner(ctx, run)
		}
		return s.executeDaemonAttempt(ctx, run)
	default:
		return WorkerRef{}, fmt.Errorf("unsupported scope kind %q", run.SelectedScope.Kind)
	}
}

func (s *Service) executeServerAttempt(ctx context.Context, run GoalRun) (WorkerRef, error) {
	if s.agent == nil || s.agent.TaskService() == nil {
		return WorkerRef{}, fmt.Errorf("agent task service unavailable")
	}
	worker := WorkerRef{
		ID:              "gw_" + uuid.NewString(),
		Name:            "server-worker",
		Status:          "running",
		Scope:           *run.SelectedScope,
		LastHeartbeatAt: s.now().UTC(),
		LastProgressAt:  s.now().UTC(),
		LeaseExpiresAt:  s.now().UTC().Add(5 * time.Minute),
	}
	taskID := "goalrun-task-" + uuid.NewString()
	worker.TaskID = taskID
	_, err := s.agent.TaskService().Enqueue(tasks.Task{
		ID:        taskID,
		Type:      tasks.TypeBackgroundAgent,
		Summary:   run.Goal,
		SessionID: "goalrun:" + run.ID,
		RuntimeID: "goalrun:server",
		Metadata: map[string]any{
			"goal_run_id":        run.ID,
			"created_by_user_id": run.CreatedBy,
			"scope_kind":         string(run.SelectedScope.Kind),
		},
	})
	if err != nil {
		return WorkerRef{}, fmt.Errorf("enqueue server goal task: %w", err)
	}
	if _, err := s.agent.TaskService().Claim(taskID, "goalrun:server"); err != nil {
		return WorkerRef{}, fmt.Errorf("claim server goal task: %w", err)
	}
	if _, err := s.agent.TaskService().Start(taskID); err != nil {
		return WorkerRef{}, fmt.Errorf("start server goal task: %w", err)
	}

	session := &goalRunSession{}
	heartbeatDone := make(chan struct{})
	go s.heartbeatWorker(ctx, run.ID, worker.ID, heartbeatDone)
	_, chatErr := s.agent.Chat(ctx, session, run.Goal)
	close(heartbeatDone)
	if chatErr != nil {
		worker.Status = "failed"
		worker.LastError = chatErr.Error()
		_, _ = s.agent.TaskService().Fail(taskID, chatErr.Error())
		return worker, fmt.Errorf("server goal execution failed: %w", chatErr)
	}
	worker.Status = "completed"
	worker.LastHeartbeatAt = s.now().UTC()
	worker.LastProgressAt = s.now().UTC()
	_, _ = s.agent.TaskService().Complete(taskID)
	return worker, nil
}

func (s *Service) executeDaemonAttempt(ctx context.Context, run GoalRun) (WorkerRef, error) {
	if s.agent == nil || s.agent.TaskService() == nil {
		return WorkerRef{}, fmt.Errorf("agent task service unavailable")
	}
	if s.kvStore == nil {
		return WorkerRef{}, fmt.Errorf("kv store unavailable")
	}
	machineID := strings.TrimSpace(run.SelectedScope.MachineID)
	if machineID == "" {
		return WorkerRef{}, fmt.Errorf("daemon machine_id is required")
	}
	registry := daemonhost.NewRegistry(s.kvStore)
	snapshot, err := registry.Snapshot(ctx)
	if err != nil {
		return WorkerRef{}, fmt.Errorf("load daemon registry snapshot: %w", err)
	}
	inventory := snapshot.Inventories[machineID]
	if inventory == nil {
		return WorkerRef{}, fmt.Errorf("daemon machine %s inventory not found", machineID)
	}
	runtimeID, workspaceID, err := chooseDaemonRuntime(inventory)
	if err != nil {
		return WorkerRef{}, err
	}
	worker := WorkerRef{
		ID:              "gw_" + uuid.NewString(),
		Name:            "daemon-worker",
		Status:          "running",
		Scope:           *run.SelectedScope,
		LastHeartbeatAt: s.now().UTC(),
		LastProgressAt:  s.now().UTC(),
		LeaseExpiresAt:  s.now().UTC().Add(5 * time.Minute),
	}
	taskID := "goalrun-task-" + uuid.NewString()
	worker.TaskID = taskID
	_, err = s.agent.TaskService().Enqueue(tasks.Task{
		ID:        taskID,
		Type:      tasks.TypeRemoteAgent,
		Summary:   run.Goal,
		SessionID: "goalrun:" + run.ID,
		RuntimeID: runtimeID,
		Metadata: map[string]any{
			"goal_run_id":        run.ID,
			"machine_id":         machineID,
			"workspace_id":       workspaceID,
			"created_by_user_id": run.CreatedBy,
			"scope_kind":         string(run.SelectedScope.Kind),
		},
	})
	if err != nil {
		return WorkerRef{}, fmt.Errorf("enqueue daemon goal task: %w", err)
	}
	pollTicker := time.NewTicker(500 * time.Millisecond)
	defer pollTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			worker.Status = "failed"
			worker.LastError = ctx.Err().Error()
			return worker, ctx.Err()
		case <-pollTicker.C:
			items := s.agent.TaskService().List()
			for _, item := range items {
				if item.ID != taskID {
					continue
				}
				worker.LastHeartbeatAt = s.now().UTC()
				worker.LastProgressAt = s.now().UTC()
				worker.LeaseExpiresAt = s.now().UTC().Add(5 * time.Minute)
				if tasks.IsFinal(item.State) {
					if item.State == tasks.StateCompleted {
						worker.Status = "completed"
						return worker, nil
					}
					worker.Status = "failed"
					worker.LastError = item.LastError
					return worker, fmt.Errorf("daemon goal task %s finished with state %s: %s", taskID, item.State, item.LastError)
				}
				break
			}
		}
	}
}

func (s *Service) heartbeatWorker(ctx context.Context, goalRunID, workerID string, done <-chan struct{}) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case <-ticker.C:
			workers, err := s.store.LoadWorkers(context.Background(), goalRunID)
			if err != nil {
				continue
			}
			for i := range workers {
				if workers[i].ID != workerID {
					continue
				}
				workers[i].LastHeartbeatAt = s.now().UTC()
				workers[i].LeaseExpiresAt = s.now().UTC().Add(5 * time.Minute)
			}
			_ = s.store.SaveWorkers(context.Background(), goalRunID, workers)
		}
	}
}

func (s *Service) evaluateCriteria(ctx context.Context, run GoalRun, set criteria.Set) (GoalRun, criteria.Set, error) {
	allPassed := true
	needsHuman := false
	for i := range set.Criteria {
		item := &set.Criteria[i]
		item.UpdatedAt = s.now().UTC()
		switch item.Type {
		case criteria.TypeManualConfirmation:
			item.Status = criteria.StatusNeedsHuman
			needsHuman = true
			allPassed = false
		case criteria.TypeCommand:
			command, _ := item.Definition["command"].(string)
			if strings.TrimSpace(command) == "" {
				item.Status = criteria.StatusFailed
				item.LastError = "command is required"
				allPassed = false
				continue
			}
			if run.SelectedScope != nil && run.SelectedScope.Kind == ScopeDaemon {
				item.Status = criteria.StatusNeedsHuman
				item.LastError = "automatic daemon command verification is not available yet"
				needsHuman = true
				allPassed = false
				continue
			}
			if err := runLocalCommand(ctx, command); err != nil {
				item.Status = criteria.StatusFailed
				item.LastError = err.Error()
				allPassed = false
				continue
			}
			item.Status = criteria.StatusPassed
		case criteria.TypeFileExists:
			path, _ := item.Definition["path"].(string)
			if strings.TrimSpace(path) == "" {
				item.Status = criteria.StatusFailed
				item.LastError = "path is required"
				allPassed = false
				continue
			}
			if run.SelectedScope != nil && run.SelectedScope.Kind == ScopeDaemon {
				ok, err := remoteFileExists(ctx, s.kvStore, run.SelectedScope.MachineID, item.Scope, path)
				if err != nil {
					item.Status = criteria.StatusFailed
					item.LastError = err.Error()
					allPassed = false
					continue
				}
				if !ok {
					item.Status = criteria.StatusFailed
					item.LastError = "file not found"
					allPassed = false
					continue
				}
				item.Status = criteria.StatusPassed
				continue
			}
			ok, err := localFileExists(path)
			if err != nil {
				item.Status = criteria.StatusFailed
				item.LastError = err.Error()
				allPassed = false
				continue
			}
			if !ok {
				item.Status = criteria.StatusFailed
				item.LastError = "file not found"
				allPassed = false
				continue
			}
			item.Status = criteria.StatusPassed
		case criteria.TypeFileContains:
			path, _ := item.Definition["path"].(string)
			needle, _ := item.Definition["contains"].(string)
			if strings.TrimSpace(path) == "" || strings.TrimSpace(needle) == "" {
				item.Status = criteria.StatusFailed
				item.LastError = "path and contains are required"
				allPassed = false
				continue
			}
			if run.SelectedScope != nil && run.SelectedScope.Kind == ScopeDaemon {
				ok, err := remoteFileContains(ctx, s.kvStore, run.SelectedScope.MachineID, item.Scope, path, needle)
				if err != nil {
					item.Status = criteria.StatusFailed
					item.LastError = err.Error()
					allPassed = false
					continue
				}
				if !ok {
					item.Status = criteria.StatusFailed
					item.LastError = "content not found"
					allPassed = false
					continue
				}
				item.Status = criteria.StatusPassed
				continue
			}
			ok, err := localFileContains(path, needle)
			if err != nil {
				item.Status = criteria.StatusFailed
				item.LastError = err.Error()
				allPassed = false
				continue
			}
			if !ok {
				item.Status = criteria.StatusFailed
				item.LastError = "content not found"
				allPassed = false
				continue
			}
			item.Status = criteria.StatusPassed
		default:
			item.Status = criteria.StatusFailed
			item.LastError = fmt.Sprintf("unsupported criterion type %q", item.Type)
			allPassed = false
		}
	}
	if allPassed {
		run.Status = GoalStatusCompleted
		run.CompletedAt = s.now().UTC()
		run.UpdatedAt = s.now().UTC()
		return run, set, nil
	}
	if needsHuman {
		run.Status = GoalStatusNeedsHumanConfirmation
		run.UpdatedAt = s.now().UTC()
		return run, set, nil
	}
	run.Status = GoalStatusRunning
	run.UpdatedAt = s.now().UTC()
	return run, set, nil
}

type goalRunSession struct {
	messages []agent.Message
}

func (s *goalRunSession) GetMessages() []agent.Message {
	return append([]agent.Message(nil), s.messages...)
}

func (s *goalRunSession) AddMessage(msg agent.Message) {
	s.messages = append(s.messages, msg)
}

func runLocalCommand(ctx context.Context, command string) error {
	cmd := exec.CommandContext(ctx, "bash", "-lc", strings.TrimSpace(command))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("run command %q: %w\n%s", command, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func localFileExists(path string) (bool, error) {
	_, err := os.Stat(strings.TrimSpace(path))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("stat %s: %w", path, err)
}

func localFileContains(path, needle string) (bool, error) {
	content, err := os.ReadFile(strings.TrimSpace(path))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("read %s: %w", path, err)
	}
	return strings.Contains(string(content), needle), nil
}

func remoteFileExists(ctx context.Context, kv state.KV, machineID string, itemScope shared.ExecutionScope, path string) (bool, error) {
	content, err := remoteReadFile(ctx, kv, machineID, itemScope, path)
	if err != nil {
		if strings.Contains(err.Error(), "400") {
			return false, nil
		}
		return false, err
	}
	return content != "", nil
}

func remoteFileContains(ctx context.Context, kv state.KV, machineID string, itemScope shared.ExecutionScope, path, needle string) (bool, error) {
	content, err := remoteReadFile(ctx, kv, machineID, itemScope, path)
	if err != nil {
		return false, err
	}
	return strings.Contains(content, needle), nil
}

func remoteReadFile(ctx context.Context, kv state.KV, machineID string, itemScope shared.ExecutionScope, path string) (string, error) {
	registry := daemonhost.NewRegistry(kv)
	snapshot, err := registry.Snapshot(ctx)
	if err != nil {
		return "", fmt.Errorf("load daemon registry snapshot: %w", err)
	}
	info := snapshot.Machines[strings.TrimSpace(machineID)]
	if info == nil {
		return "", fmt.Errorf("daemon machine %s not found", machineID)
	}
	if strings.TrimSpace(info.DaemonUrl) == "" {
		return "", fmt.Errorf("daemon machine %s has no daemon_url", machineID)
	}
	inventory := snapshot.Inventories[strings.TrimSpace(machineID)]
	if inventory == nil {
		return "", fmt.Errorf("daemon machine %s inventory not found", machineID)
	}
	workspaceID, err := workspaceIDForScope(inventory, itemScope)
	if err != nil {
		return "", err
	}
	client := daemonhost.NewClient(info.DaemonUrl)
	resp, err := client.ReadWorkspaceFile(&daemonv1.ReadWorkspaceFileRequest{
		WorkspaceId: workspaceID,
		Path:        strings.TrimSpace(path),
	})
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

func workspaceIDForScope(inventory *daemonv1.RuntimeInventory, itemScope shared.ExecutionScope) (string, error) {
	if inventory == nil {
		return "", fmt.Errorf("inventory is nil")
	}
	if strings.TrimSpace(itemScope.MachineID) != "" {
		for _, ws := range inventory.Workspaces {
			if ws == nil {
				continue
			}
			if strings.TrimSpace(ws.MachineId) == strings.TrimSpace(itemScope.MachineID) && ws.IsDefault {
				return strings.TrimSpace(ws.WorkspaceId), nil
			}
		}
	}
	for _, ws := range inventory.Workspaces {
		if ws == nil {
			continue
		}
		if ws.IsDefault {
			return strings.TrimSpace(ws.WorkspaceId), nil
		}
	}
	if len(inventory.Workspaces) > 0 && inventory.Workspaces[0] != nil {
		return strings.TrimSpace(inventory.Workspaces[0].WorkspaceId), nil
	}
	return "", fmt.Errorf("no workspace available in daemon inventory")
}

func chooseDaemonRuntime(inventory *daemonv1.RuntimeInventory) (string, string, error) {
	runtimeID, workspaceID, ok := daemonhost.SelectRunnableWorkspaceRuntime(inventory)
	if ok {
		return runtimeID, workspaceID, nil
	}
	if workspaceID == "" {
		return "", "", fmt.Errorf("no workspace available in daemon inventory")
	}
	return "", "", fmt.Errorf("no installed healthy daemon runtime available for workspace %s", workspaceID)
}
