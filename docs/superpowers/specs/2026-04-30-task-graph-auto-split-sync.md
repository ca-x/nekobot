# Task Graph: Auto-create/Split/Assign and Agent-aware Sync

Status: design (task #13)

Related:
- `docs/superpowers/specs/2026-04-30-daemon-idempotency-event-replay-design.md` (Slice D)
- `docs/superpowers/specs/2026-04-30-daemon-collaboration-protocol-v2.md`
- task #11 (idempotency store), task #12 (event_log replay)

## Problem

The current task model is flat: each `Task` is an independent unit with no parent/child or dependency relationships. The in-memory `tasks.Service` manages lifecycle but has no persistence or graph semantics.

Slock-style server automation needs:
1. Automatic task creation from rules (e.g., "when a message arrives in #support, create a triage task").
2. Task splitting: a large task can be decomposed into subtasks, either by an agent or by server rules.
3. Agent-aware sync: agents must discover relevant task graph changes (assigned, mentioned, capability-matched) through the event log without polling all tasks.
4. Restart safety: task graph mutations must be idempotent and replayable.

## Current State

### Proto Task (flat)

```protobuf
message Task {
  string task_id = 1;
  string summary = 2;
  string state = 3;
  string runtime_id = 4;
  string thread_id = 5;
  string workspace_id = 6;
  string computer_id = 7;
  string created_by_user_id = 8;
  string blocked_reason = 9;
  string target = 10;
  string assignee_id = 11;
  string current_run_id = 12;
}
```

### Go Task struct (in-memory)

```go
type Task struct {
    ID, Type, State, Summary, SessionID, RuntimeID string
    ActualProvider, ActualModel, PendingAction      string
    LastError, PermissionMode                       string
    CreatedAt, StartedAt, CompletedAt               time.Time
    Metadata                                        map[string]any
}
```

### Existing RPCs

- `CreateCollaborationTask` — creates a flat task, stores in-memory via `tasks.Service.Enqueue`
- `ListCollaborationTasks` — filters in-memory tasks by target/agent_id
- `ClaimCollaborationTask` — claims a pending task

No graph, split, dependency, or assignment rule support.

## Proposed Proto Changes

### Extended Task message

```protobuf
message Task {
  // --- existing fields (1-12) unchanged ---

  // --- graph relationships ---
  string root_task_id = 13;               // self or root of the graph
  string parent_task_id = 14;             // empty if top-level
  repeated string subtask_ids = 15;       // direct children
  repeated string depends_on_task_ids = 16; // must complete before this can start
  repeated string blocked_by_task_ids = 17; // currently blocking this task

  // --- provenance ---
  string source = 18;                     // human | agent | server_rule | scheduler | import
  string created_by_agent_id = 19;        // agent that created this task
  string server_rule_id = 20;             // server automation rule that created this task
  string split_proposal_id = 21;          // if created by a split, the proposal ID

  // --- graph versioning ---
  int64 graph_version = 22;               // increments on any graph structure change

  // --- capability requirements ---
  // Agent must have ALL listed capabilities to claim (subset match: required_capabilities ⊆ agent.capabilities).
  // e.g., ["shell", "repo.write"] means agent must have both; having only "shell" is insufficient.
  // Future: if any-of/expressions are needed, introduce a separate `capability_expression` field.
  repeated string required_capabilities = 23;
}
```

### TaskGraphSnapshot (for ListTaskGraph)

```protobuf
message TaskGraphSnapshot {
  string root_task_id = 1;
  int64 graph_version = 2;
  repeated Task nodes = 3;               // all tasks in the graph
  repeated TaskEdge edges = 4;           // dependency/blocking edges
}

message TaskEdge {
  string from_task_id = 1;
  string to_task_id = 2;
  string kind = 3;                       // "depends_on" | "blocks" | "parent_child"
}
```

### New RPCs

```protobuf
// Split a task into subtasks (server or agent initiated)
rpc ProposeTaskSplit(ProposeTaskSplitRequest) returns (ProposeTaskSplitResponse);
rpc ApplyTaskSplit(ApplyTaskSplitRequest) returns (ApplyTaskSplitResponse);

// Create an entire task graph in one atomic operation
rpc CreateTaskGraph(CreateTaskGraphRequest) returns (CreateTaskGraphResponse);

// Query the task graph structure
rpc ListTaskGraph(ListTaskGraphRequest) returns (ListTaskGraphResponse);

// Update task graph relationships (add/remove dependencies, reassign)
rpc UpdateTaskGraph(UpdateTaskGraphRequest) returns (UpdateTaskGraphResponse);
```

#### ProposeTaskSplit

```protobuf
message ProposeTaskSplitRequest {
  string parent_task_id = 1;
  repeated ProposedSubtask proposed_tasks = 2;
  string request_id = 3;
}

message ProposedSubtask {
  string client_proposed_id = 1;           // caller-assigned stable ID (e.g., "subtask-1"), used for dependency references
  string summary = 2;
  string assignee_id = 3;
  repeated string depends_on_proposed_ids = 4; // references other ProposedSubtask.client_proposed_id values
  repeated string required_capabilities = 5;
}

message ProposeTaskSplitResponse {
  string proposal_id = 1;
  Task parent_task = 2;
  repeated Task proposed_tasks = 3;        // server assigns temporary IDs
  bool accepted = 4;
  string rejection_reason = 5;
}
```

#### ApplyTaskSplit

```protobuf
message ApplyTaskSplitRequest {
  string parent_task_id = 1;
  string proposal_id = 2;
  repeated string selected_task_ids = 3;   // which proposed tasks to materialize (empty = all)
  string request_id = 4;
}

message ApplyTaskSplitResponse {
  Task parent_task = 1;
  repeated Task created_subtasks = 2;
  int64 new_graph_version = 3;
}
```

#### CreateTaskGraph

```protobuf
message CreateTaskGraphRequest {
  Task root_task = 1;
  repeated Task subtasks = 2;
  repeated TaskEdge dependencies = 3;
  string request_id = 4;
}

message CreateTaskGraphResponse {
  TaskGraphSnapshot graph = 1;
}
```

#### ListTaskGraph

```protobuf
message ListTaskGraphRequest {
  string root_task_id = 1;
  int64 since_graph_version = 2;          // if set and matches current, server can return early (stale check);
                                          // if stale, always returns the full authoritative snapshot.
}

message ListTaskGraphResponse {
  TaskGraphSnapshot graph = 1;            // v1 always returns the full snapshot, not a delta.
                                          // Agents compare graph_version locally to detect what changed.
}
```

#### UpdateTaskGraph

```protobuf
message UpdateTaskGraphRequest {
  string task_id = 1;
  // optional patches
  string new_assignee_id = 2;
  repeated string add_dependencies = 3;
  repeated string remove_dependencies = 4;
  string new_parent_task_id = 5;
  string request_id = 6;
}

message UpdateTaskGraphResponse {
  Task task = 1;
  int64 new_graph_version = 2;
}
```

## graph_version Semantics

`graph_version` is a monotonically increasing counter per task graph (rooted at `root_task_id`).

Rules:
1. A new task graph starts at `graph_version = 1`.
2. `graph_version` increments by 1 on **any structural change**:
   - Adding a subtask (split, create child)
   - Removing a subtask
   - Adding/removing a dependency edge
   - Changing parent/child relationships
   - Task assignment changes (assignee_id change on a graph node)
3. `graph_version` does **not** increment on lifecycle state changes (pending→claimed→running→completed). State changes emit their own events but don't alter graph structure.
4. All graph mutation RPCs return the new `graph_version` so callers can track it.
5. Agents can compare their cached `graph_version` against `ListTaskGraph.since_graph_version` to detect stale views and fetch only incremental changes.

Implementation note: `graph_version` should be stored on the root task row. A simple approach is a `SELECT ... FOR UPDATE` on the root task, increment, then apply changes in the same transaction.

## Server Auto Create/Split/Assign Rules

### Rule Types

1. **Target rules**: When a message arrives matching a pattern (e.g., `#support:*`), automatically create a triage task.
2. **Split rules**: When a task exceeds a complexity threshold or matches a decomposition pattern, propose a split.
3. **Assignment rules**: When a task is created or becomes unblocked, auto-assign based on:
   - Agent capability match (`required_capabilities` ∩ agent's registered capabilities)
   - Target channel membership
   - Round-robin within eligible agents
   - Explicit server rule configuration

### Rule Entry Points

```go
// pkg/tasks/rules package (new)

type RuleEngine struct {
    store      tasks.StoreInterface
    eventLog   events.Writer       // from #12
    idempotency idempotency.Store  // from #11
}

// Called after a message is delivered to a channel
func (r *RuleEngine) EvaluateTargetRules(ctx context.Context, msg Message) error

// Called after a task is created or updated
func (r *RuleEngine) EvaluateAssignmentRules(ctx context.Context, task Task) error

// Called when a task's metadata indicates it should be split
func (r *RuleEngine) EvaluateSplitRules(ctx context.Context, task Task) error
```

Rule definitions are loaded from config (not hardcoded). First implementation can be a simple YAML/JSON config:

```yaml
task_rules:
  - name: support-triage
    trigger: message.created
    target: "#support"
    action: create_task
    template:
      summary: "Triage: {{.MessageSummary}}"
      required_capabilities: ["support"]

  - name: large-task-split
    trigger: task.created
    condition: "len(summary) > 500 || metadata.complexity == 'high'"
    action: propose_split
    strategy: subagent-decompose
```

### Atomicity

All rule-triggered mutations must be idempotent (via #11's request_id) and emit events (to #12's event_log) in the same transaction boundary. The rule engine generates deterministic request_ids:

```
server_rule:{rule_id}:{trigger_subject_id}:{trigger_event_seq}
```

This ensures replaying the same trigger doesn't duplicate task creation.

## Events for #12 Event Log

Task graph mutations must emit these event types (for #12's `collaboration_event` table):

### Lifecycle events (existing, enhanced)

| Event Type | When | Key Payload Fields |
|---|---|---|
| `task.created` | Any task creation (human, agent, server rule) | `source`, `created_by_agent_id`, `server_rule_id`, `graph_version` |
| `task.claimed` | Agent claims a task | `agent_id` |
| `task.status_changed` | State transition | `old_state`, `new_state` |
| `task.done` | Task completed | `result_summary` |
| `task.cancelled` | Task cancelled | `reason` |

### Graph mutation events (new)

| Event Type | When | Key Payload Fields |
|---|---|---|
| `task.split_proposed` | ProposeTaskSplit called | `parent_task_id`, `proposed_subtask_ids[]`, `proposal_id` |
| `task.split_applied` | ApplyTaskSplit called | `parent_task_id`, `child_task_ids[]`, `graph_version` |
| `task.child_created` | A subtask is created under a parent | `parent_task_id`, `child_task_id`, `graph_version` |
| `task.assigned` | Task assigned to an agent (manual or auto) | `assignee_id`, `assignment_reason` (manual/capability/round-robin/rule) |
| `task.assignee_changed` | Reassignment | `old_assignee_id`, `new_assignee_id` |
| `task.dependency_added` | New dependency edge | `from_task_id`, `to_task_id`, `kind` |
| `task.dependency_removed` | Dependency removed | `from_task_id`, `to_task_id` |
| `task.blocked` | Task becomes blocked by an incomplete dependency | `blocked_by_task_id` |
| `task.unblocked` | Blocking dependency completed | `unblocked_by_task_id` |
| `task.sync_required` | Server detects graph inconsistency and agent needs to re-sync | `root_task_id`, `known_graph_version`, `actual_graph_version` |

### Event metadata (precomputed for #12 filtering)

Each task graph event must precompute:
- `target`: the task's target channel/DM
- `assignee_id`: the task's current assignee (if any)
- `mentioned_agent_ids`: agents referenced in the event (e.g., new assignee, agent that proposed split)
- `capability_keys`: from `required_capabilities` of the affected task
- `subject_kind`: `"task"`
- `subject_id`: the task_id
- `parent_subject_kind`: `"task"` (for child tasks, the parent's task_id)
- `parent_subject_id`: parent_task_id
- `graph_version`: the resulting graph version after the mutation

## Agent Awareness Model

Agents discover task graph changes through `ListEventsSince` (#12), not by polling tasks.

An agent receives events where ANY of these match:
1. `target` is a channel/thread the agent follows
2. `assignee_id` matches the agent's ID
3. `mentioned_agent_ids` contains the agent's ID
4. `capability_keys` is a subset of the agent's registered capabilities (required_capabilities ⊆ agent.capabilities, and the agent has permission on the target)
5. The agent owns a computer/run lease for the affected task's computer

The gateway's `ListEventsSince` implementation applies these filters server-side using the precomputed indexed fields (from #12's `collaboration_event` table).

### Reconnect flow

1. Agent reconnects with last known event cursor and `graph_version`.
2. Calls `ListEventsSince(cursor)` to get all missed events.
3. For task graph events, updates local cache:
   - `task.created` / `task.child_created`: add to local graph
   - `task.split_applied`: add children, update parent
   - `task.assigned` / `task.assignee_changed`: update assignment
   - `task.dependency_added` / `task.dependency_removed`: update edges
   - `task.status_changed` / `task.done` / `task.cancelled`: update lifecycle
4. If agent detects `task.sync_required` or graph_version gap > 1, calls `ListTaskGraph(root_task_id, since_graph_version)` for authoritative state.

## Migration Path

### Phase 1: Design (this document, task #13)
- Finalize proto additions and RPC definitions
- Define event types and precomputed fields
- Document rule engine interface

### Phase 2: Proto + shallow implementation (after #12 event store)
- Add graph fields to Task proto message
- Implement `CreateTaskGraph` and `ListTaskGraph` RPCs (initially backed by in-memory tasks.Service)
- Emit graph events to #12's event_log on task creation

### Phase 3: Split/dependency implementation
- Implement `ProposeTaskSplit` / `ApplyTaskSplit` with idempotency (#11)
- Implement `UpdateTaskGraph` for dependency management
- Add `graph_version` tracking

### Phase 4: Server rules + assignment
- Implement rule engine (config-driven)
- Auto-assignment based on capabilities
- Agent-aware filtering in `ListEventsSince`

### Phase 5: Persistence migration
- Move task graph storage from in-memory `tasks.Service` to Ent-backed persistent store
- Ensure restart safety for graph state

## Risks and Open Questions

1. **In-memory vs persistent**: Current `tasks.Service` is in-memory. Task graph persistence is needed for restart safety. The graph fields can be added to the in-memory model first, but Ent persistence should follow quickly.

2. **graph_version race conditions**: Concurrent graph mutations could race on version increment. Mitigation: `SELECT ... FOR UPDATE` on root task row, or optimistic locking with retry.

3. **Split proposal lifecycle**: Proposals may be stale if the parent task changes between propose and apply. The `ApplyTaskSplit` must validate the parent's current state and reject stale proposals.

4. **Capability matching complexity**: V1 uses strict subset matching (required_capabilities ⊆ agent.capabilities). Complex capability expressions (any-of, version ranges, optional capabilities) can be added later via a `capability_expression` field without changing the event model or breaking existing semantics.

5. **Rule engine scope**: First implementation should be minimal (config-driven, no dynamic rules). Dynamic rule authoring (via UI or API) is a future concern.

6. **Backward compatibility**: New proto fields are additive (field numbers > 12). Old clients ignore them. The existing `CreateCollaborationTask` RPC continues to work for flat tasks.
