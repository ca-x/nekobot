package gateway

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	daemonv1 "nekobot/gen/go/nekobot/daemon/v1"
	"nekobot/pkg/agent"
	"nekobot/pkg/approval"
	"nekobot/pkg/bus"
	"nekobot/pkg/config"
	"nekobot/pkg/idempotency"
	"nekobot/pkg/logger"
	"nekobot/pkg/process"
	"nekobot/pkg/session"
	"nekobot/pkg/state"
	"nekobot/pkg/tasks"
)

func newTaskGraphTestServer(t *testing.T) *Server {
	t.Helper()

	cfg := config.DefaultConfig()
	cfg.Gateway.Port = 0
	cfg.Storage.DBDir = t.TempDir()
	cfg.Sessions.Sources.Gateway = true
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log, err := logger.New(&logger.Config{Level: "error"})
	if err != nil {
		t.Fatal(err)
	}
	store, err := state.NewFileStore(log, &state.FileStoreConfig{FilePath: t.TempDir() + "/daemon-state.json"})
	if err != nil {
		t.Fatalf("new state store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	entClient, err := config.OpenRuntimeEntClient(cfg)
	if err != nil {
		t.Fatalf("open ent client: %v", err)
	}
	t.Cleanup(func() { _ = entClient.Close() })
	if err := config.EnsureRuntimeEntSchema(entClient); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	ag, err := agent.New(cfg, log, nil, nil, approval.NewManager(approval.Config{Mode: approval.ModeAuto}), nil, store, entClient, nil)
	if err != nil {
		t.Fatalf("new agent: %v", err)
	}
	return NewServer(
		cfg,
		log,
		ag,
		bus.NewLocalBus(log, 10),
		nil,
		approval.NewManager(approval.Config{Mode: approval.ModeAuto}),
		nil,
		process.NewManager(log),
		session.NewManager(t.TempDir(), cfg.Sessions),
		entClient,
		store,
	)
}

func enqueueTaskGraphParent(t *testing.T, s *Server, taskID string) {
	t.Helper()
	_, err := s.agent.TaskService().Enqueue(tasks.Task{
		ID:        taskID,
		Type:      tasks.TypeRemoteAgent,
		Summary:   "parent task",
		SessionID: "LightOsClub:task-graph",
		Metadata: map[string]any{
			"target":   "#LightOsClub:task-graph",
			"delivery": "daemon-collaboration",
		},
	})
	if err != nil {
		t.Fatalf("enqueue parent task: %v", err)
	}
}

func TestTaskGraphProposeTaskSplitIsIdempotent(t *testing.T) {
	s := newTaskGraphTestServer(t)
	ctx := context.Background()
	enqueueTaskGraphParent(t, s, "parent-propose-idem")

	req := &daemonv1.ProposeTaskSplitRequest{
		ParentTaskId: "parent-propose-idem",
		RequestId:    "req-propose-idem-1",
		ProposedTasks: []*daemonv1.ProposedSubtask{
			{ClientProposedId: "first", Summary: "first child"},
			{ClientProposedId: "second", Summary: "second child"},
		},
	}
	first, err := s.ProposeTaskSplit(ctx, req)
	if err != nil {
		t.Fatalf("first propose: %v", err)
	}
	second, err := s.ProposeTaskSplit(ctx, req)
	if err != nil {
		t.Fatalf("replayed propose: %v", err)
	}
	if first.GetProposalId() != second.GetProposalId() {
		t.Fatalf("proposal_id replay mismatch: first=%q second=%q", first.GetProposalId(), second.GetProposalId())
	}
	if len(first.GetProposedTasks()) != 2 || len(second.GetProposedTasks()) != 2 {
		t.Fatalf("unexpected proposed task counts: first=%d second=%d", len(first.GetProposedTasks()), len(second.GetProposedTasks()))
	}
	for i := range first.GetProposedTasks() {
		if first.GetProposedTasks()[i].GetTaskId() != second.GetProposedTasks()[i].GetTaskId() {
			t.Fatalf("proposed task %d replay mismatch: first=%q second=%q", i, first.GetProposedTasks()[i].GetTaskId(), second.GetProposedTasks()[i].GetTaskId())
		}
	}

	_, err = s.ProposeTaskSplit(ctx, &daemonv1.ProposeTaskSplitRequest{
		ParentTaskId: "parent-propose-idem",
		RequestId:    "req-propose-idem-1",
		ProposedTasks: []*daemonv1.ProposedSubtask{
			{ClientProposedId: "changed", Summary: "changed child"},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "idempotency conflict") {
		t.Fatalf("expected idempotency conflict for changed replay body, got %v", err)
	}

	key := idempotency.Key{
		CallerKind: "agent",
		CallerID:   "parent-propose-idem",
		Method:     "ProposeTaskSplit",
		RequestID:  "req-propose-idem-1",
	}
	check, err := s.idempotencyStore.Check(ctx, key)
	if err != nil {
		t.Fatalf("check idempotency record: %v", err)
	}
	if check.Record == nil || check.Record.ResponseJSON == "" {
		t.Fatalf("expected completed idempotency record, got %+v", check.Record)
	}
}

func TestTaskGraphApplySplitPersistsDependenciesAndListability(t *testing.T) {
	s := newTaskGraphTestServer(t)
	ctx := context.Background()
	enqueueTaskGraphParent(t, s, "parent-ordinary")

	proposal, err := s.ProposeTaskSplit(ctx, &daemonv1.ProposeTaskSplitRequest{
		ParentTaskId: "parent-ordinary",
		RequestId:    "req-propose-deps-1",
		ProposedTasks: []*daemonv1.ProposedSubtask{
			{ClientProposedId: "setup", Summary: "setup environment"},
			{ClientProposedId: "verify", Summary: "verify result", DependsOnProposedIds: []string{"setup"}},
		},
	})
	if err != nil {
		t.Fatalf("propose split: %v", err)
	}
	applied, err := s.ApplyTaskSplit(ctx, &daemonv1.ApplyTaskSplitRequest{
		ParentTaskId: "parent-ordinary",
		ProposalId:   proposal.GetProposalId(),
		RequestId:    "req-apply-deps-1",
	})
	if err != nil {
		t.Fatalf("apply split: %v", err)
	}
	if len(applied.GetCreatedSubtasks()) != 2 {
		t.Fatalf("created %d subtasks, want 2", len(applied.GetCreatedSubtasks()))
	}
	setupID := applied.GetCreatedSubtasks()[0].GetTaskId()
	verifyID := applied.GetCreatedSubtasks()[1].GetTaskId()
	if setupID == "" || verifyID == "" {
		t.Fatalf("expected created task ids, got %+v", applied.GetCreatedSubtasks())
	}
	if got := applied.GetCreatedSubtasks()[1].GetDependsOnTaskIds(); len(got) != 1 || got[0] != setupID {
		t.Fatalf("verify depends_on = %v, want [%s]", got, setupID)
	}

	listed, err := s.ListTaskGraph(ctx, &daemonv1.ListTaskGraphRequest{RootTaskId: "parent-ordinary"})
	if err != nil {
		t.Fatalf("list graph: %v", err)
	}
	graph := listed.GetGraph()
	if graph.GetRootTaskId() != "parent-ordinary" {
		t.Fatalf("root_task_id = %q, want parent-ordinary", graph.GetRootTaskId())
	}
	if len(graph.GetNodes()) != 3 {
		t.Fatalf("listed %d nodes, want parent + 2 children: %+v", len(graph.GetNodes()), graph.GetNodes())
	}
	if !taskGraphHasEdge(graph, "parent-ordinary", setupID, "parent_child") {
		t.Fatalf("missing parent_child edge parent -> setup in %+v", graph.GetEdges())
	}
	if !taskGraphHasEdge(graph, "parent-ordinary", verifyID, "parent_child") {
		t.Fatalf("missing parent_child edge parent -> verify in %+v", graph.GetEdges())
	}
	if !taskGraphHasEdge(graph, setupID, verifyID, "depends_on") {
		t.Fatalf("missing depends_on edge setup -> verify in %+v", graph.GetEdges())
	}

	replayed, err := s.ApplyTaskSplit(ctx, &daemonv1.ApplyTaskSplitRequest{
		ParentTaskId: "parent-ordinary",
		ProposalId:   proposal.GetProposalId(),
		RequestId:    "req-apply-deps-1",
	})
	if err != nil {
		t.Fatalf("replay apply split: %v", err)
	}
	if got := replayed.GetCreatedSubtasks(); len(got) != 2 || got[0].GetTaskId() != setupID || got[1].GetTaskId() != verifyID {
		t.Fatalf("unexpected replay subtasks: %+v", got)
	}
	afterReplay, err := s.ListTaskGraph(ctx, &daemonv1.ListTaskGraphRequest{RootTaskId: "parent-ordinary"})
	if err != nil {
		t.Fatalf("list graph after replay: %v", err)
	}
	if len(afterReplay.GetGraph().GetNodes()) != 3 {
		t.Fatalf("apply replay duplicated nodes: %+v", afterReplay.GetGraph().GetNodes())
	}

	_, err = s.ApplyTaskSplit(ctx, &daemonv1.ApplyTaskSplitRequest{
		ParentTaskId:    "parent-ordinary",
		ProposalId:      proposal.GetProposalId(),
		SelectedTaskIds: []string{setupID},
		RequestId:       "req-apply-deps-1",
	})
	if err == nil || !strings.Contains(err.Error(), "idempotency conflict") {
		t.Fatalf("expected idempotency conflict for changed selected_task_ids, got %v", err)
	}
}

func TestTaskGraphSplitProposalsConcurrentAccess(t *testing.T) {
	s := newTaskGraphTestServer(t)
	ctx := context.Background()
	enqueueTaskGraphParent(t, s, "parent-concurrent")

	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := s.ProposeTaskSplit(ctx, &daemonv1.ProposeTaskSplitRequest{
				ParentTaskId: "parent-concurrent",
				ProposedTasks: []*daemonv1.ProposedSubtask{
					{ClientProposedId: "child", Summary: "child"},
				},
			})
			if err != nil {
				t.Errorf("propose %d: %v", i, err)
				return
			}
			if _, err := s.ApplyTaskSplit(ctx, &daemonv1.ApplyTaskSplitRequest{
				ParentTaskId: resp.GetParentTask().GetTaskId(),
				ProposalId:   resp.GetProposalId(),
			}); err != nil {
				t.Errorf("apply %d: %v", i, err)
			}
		}()
	}
	wg.Wait()
}

func TestTaskGraphSplitProposalSurvivesServerMemoryLoss(t *testing.T) {
	s := newTaskGraphTestServer(t)
	ctx := context.Background()
	enqueueTaskGraphParent(t, s, "parent-persisted-proposal")

	proposal, err := s.ProposeTaskSplit(ctx, &daemonv1.ProposeTaskSplitRequest{
		ParentTaskId: "parent-persisted-proposal",
		RequestId:    "req-propose-persisted-1",
		ProposedTasks: []*daemonv1.ProposedSubtask{
			{ClientProposedId: "restore", Summary: "restored child"},
		},
	})
	if err != nil {
		t.Fatalf("propose split: %v", err)
	}
	if proposal.GetProposalId() == "" {
		t.Fatalf("expected proposal id")
	}

	s.splitProposalsMu.Lock()
	s.splitProposals = nil
	s.splitProposalsMu.Unlock()

	persistedRaw, ok, err := s.kvStore.Get(ctx, daemonSplitProposalKey)
	if err != nil {
		t.Fatalf("get persisted split proposals: %v", err)
	}
	if !ok {
		t.Fatalf("expected persisted split proposal store")
	}
	persistedJSON, err := json.Marshal(persistedRaw)
	if err != nil {
		t.Fatalf("marshal persisted split proposals: %v", err)
	}
	if err := s.kvStore.Set(ctx, daemonSplitProposalKey, json.RawMessage(persistedJSON)); err != nil {
		t.Fatalf("simulate json-backed proposal reload: %v", err)
	}

	applied, err := s.ApplyTaskSplit(ctx, &daemonv1.ApplyTaskSplitRequest{
		ParentTaskId: "parent-persisted-proposal",
		ProposalId:   proposal.GetProposalId(),
		RequestId:    "req-apply-persisted-1",
	})
	if err != nil {
		t.Fatalf("apply persisted split proposal: %v", err)
	}
	if len(applied.GetCreatedSubtasks()) != 1 || applied.GetCreatedSubtasks()[0].GetSummary() != "restored child" {
		t.Fatalf("unexpected created subtasks: %+v", applied.GetCreatedSubtasks())
	}

	s.splitProposalsMu.Lock()
	s.splitProposals = nil
	s.splitProposalsMu.Unlock()
	_, err = s.ApplyTaskSplit(ctx, &daemonv1.ApplyTaskSplitRequest{
		ParentTaskId: "parent-persisted-proposal",
		ProposalId:   proposal.GetProposalId(),
	})
	if err == nil || !strings.Contains(err.Error(), "split proposal not found") {
		t.Fatalf("expected persisted proposal deletion after apply, got %v", err)
	}
}

func taskGraphHasEdge(graph *daemonv1.TaskGraphSnapshot, fromID, toID, kind string) bool {
	for _, edge := range graph.GetEdges() {
		if edge.GetFromTaskId() == fromID && edge.GetToTaskId() == toID && edge.GetKind() == kind {
			return true
		}
	}
	return false
}
