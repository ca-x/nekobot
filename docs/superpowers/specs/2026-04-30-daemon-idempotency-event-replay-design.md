# Daemon Idempotency and Event Replay Design

Status: proposed implementation plan

Related:

- `docs/superpowers/specs/2026-04-30-daemon-collaboration-protocol-v2.md`
- task #10 in Slock

## Decision

Nekobot daemon collaboration needs two durable control-plane mechanisms before the v2 protocol can be treated as restart-safe:

1. Request idempotency for every side-effecting RPC.
2. A replayable event log for messages, tasks, task graphs, runs, run steps, activity, reminders, attachments, computer/agent state, and lease changes.

These mechanisms are server-owned. Daemon process memory, runtime adapter state, and client retry behavior are not sources of truth.

## Non-goals

- Do not implement a streaming event transport in the first slice. `ListEventsSince` is the mandatory recovery path; streams can be added later.
- Do not couple idempotency to HTTP only. The same store must serve gRPC and derived HTTP endpoints.
- Do not use runtime-specific adapter details in the idempotency key or event schema.
- Do not make task graph execution part of this slice. This document only defines how server-created/split tasks become visible and retry-safe.

## Idempotency Store

### Contract

Every mutating request must carry `request_id`.

The server derives the idempotency identity as:

```text
tenant_id + caller_kind + caller_id + method + request_id
```

Rules:

- `tenant_id` comes from the authenticated request context. Use the default tenant until multi-tenant auth is fully wired.
- `caller_kind` is one of `user`, `agent`, `computer`, `system`.
- `caller_id` is derived from auth/session/daemon identity, not trusted from request body fields.
- `method` is the stable RPC/action name, for example `SendMessage`, `AppendRunStep`, or `ApplyTaskSplit`.
- `request_id` is caller-generated and unique for the caller/method retry window.
- A duplicate with the same request body returns the original terminal result.
- A duplicate with a different request body returns conflict and must not execute.

### Record Shape

Use an Ent-backed table rather than KV so SQLite/PostgreSQL/MySQL all get the same unique constraint and transaction behavior.

Suggested schema:

```text
idempotency_record
  id
  tenant_id
  caller_kind
  caller_id
  method
  request_id
  request_hash
  status                  pending | succeeded | failed
  response_type           proto:<full-name> | json:<type> | error
  response_json           sanitized terminal response
  error_code
  error_message
  resource_kind
  resource_id
  event_id
  created_at
  updated_at
  expires_at

unique(tenant_id, caller_kind, caller_id, method, request_id)
index(expires_at)
index(resource_kind, resource_id)
```

`request_hash` must be computed from a canonical request representation after validation and after removing transport-only fields. The hash must include the intended side effect. It must not include auth headers, timestamps, or generated IDs.

`response_json` must be safe to return to the caller. It must not cache raw secret values or channel credentials. If a response can contain secrets, cache a resource reference and reconstruct a sanitized response on duplicate.

### Execution Flow

1. Authenticate the caller and derive `tenant_id`, `caller_kind`, and `caller_id`.
2. Validate the request, including non-empty `request_id`.
3. Canonicalize and hash the request.
4. Try to insert a `pending` idempotency record.
5. If insert succeeds, execute the mutation and emit any resource event in the same transaction or transactional outbox boundary.
6. Mark the record `succeeded` with the sanitized response and primary resource/event references.
7. If a duplicate exists:
   - same hash + `succeeded`: return cached/reconstructed response.
   - same hash + `failed`: return cached deterministic failure.
   - same hash + `pending`: return `already_in_progress` with retry-after semantics.
   - different hash: return conflict `request_id_reused`.

For validation failures that happen before the idempotency record can be inserted, the server may return the error without storing it. For deterministic validation failures after a record exists, store a `failed` terminal response so retries remain stable.

### TTL

Default retention:

- `SendMessage`, `UploadAttachment`, `LogActivity`: 7 days.
- `CreateTask`, `ClaimTask`, `UpdateTask`, `CreateRun`, `AppendRunStep`, `ScheduleReminder`, `CancelReminder`, task graph operations: 30 days.
- `RegisterComputer`, `HeartbeatComputer`, `FollowThread`, `UnfollowThread`, `SetAgentEnv`: 7 days.

Records must not expire before the corresponding events are outside the replay retention window. If event retention is 30 days, idempotency records that reference those events should keep at least the same retention.

### First RPC Coverage

First implementation must cover these mutating daemon RPCs:

- `RegisterComputer`
- `HeartbeatComputer`
- `SendMessage`
- `FollowThread`
- `UnfollowThread`
- `CreateCollaborationTask`
- `ClaimCollaborationTask`
- `SetAgentEnv`
- `ScheduleReminder`
- `CancelReminder`
- `LogActivity`
- `UpdateRunStatus`
- `AppendRunStep`
- `UploadAttachment`

`FetchAssignedRuns`, `ListRuns`, `GetRun`, `ReadMessages`, `ListActivity`, and `ListEventsSince` are reads and do not need request IDs.

### Client Behavior

Daemon clients must generate stable request IDs at the operation boundary:

- Register and heartbeat: new request ID per attempt is acceptable because these operations are naturally upserts, but retries of the same network attempt should reuse the ID.
- Fetch assigned runs: no request ID required.
- Update run status: one request ID per run state transition.
- Append run step: one request ID per logical step. Re-sending the same step after reconnect must reuse the same ID.
- Send message/activity/reminder/task graph operations: one request ID per logical user-visible operation.

The daemon should persist outstanding request IDs in its local run checkpoint if the operation may be retried after process restart.

## Event Replay Store

### Contract

`ListEventsSince` returns all authorized events after a cursor. It is the authoritative reconnect path for agents and daemons.

Streams, websocket pushes, and polling optimizations must be derived from the same event store.

### Record Shape

Use an append-only Ent-backed event table.

Suggested schema:

```text
collaboration_event
  id
  tenant_id
  server_id
  stream
  sequence                monotonic int64 per tenant/stream
  event_id                stable ULID/UUID exposed in proto
  event_type              message.created, task.status_changed, run.step_appended, ...
  target
  thread_id
  actor_kind              user | agent | computer | system
  actor_id
  subject_kind            message | task | run | run_step | activity | reminder | attachment | computer | agent | lease
  subject_id
  parent_subject_kind
  parent_subject_id
  assignee_id
  mentioned_agent_ids_json
  capability_keys_json
  graph_version
  idempotency_key
  payload_json
  created_at

unique(tenant_id, sequence)
unique(tenant_id, event_id)
index(tenant_id, stream, sequence)
index(tenant_id, target, sequence)
index(tenant_id, assignee_id, sequence)
index(tenant_id, actor_id, sequence)
index(tenant_id, subject_kind, subject_id)
```

`sequence` is the cursor ordering source. `event_id` is stable but should not be used alone for ordering. Use a database sequence table or transactionally allocated counter per tenant/stream. ULID ordering is useful but not sufficient as the only ordering guarantee across databases.

Filtering fields must be precomputed at write time. `tenant_id`, `target`, `actor_id`, `subject_kind`, `subject_id`, `assignee_id`, `mentioned_agent_ids_json`, and `capability_keys_json` are indexed event metadata, not values that should be discovered later by scanning `payload_json`.

### Cursor Shape

The proto currently has:

```text
EventCursor {
  cursor
  target
  last_event_id
  last_message_id
  last_activity_id
}
```

Treat `cursor` as the authoritative opaque token. The other fields are compatibility hints and debugging aids.

Suggested opaque cursor payload:

```json
{
  "version": 1,
  "server_id": "7789a621-26a4-4f8e-ae25-929fa827271b",
  "stream": "tenant:default",
  "tenant_id": "default",
  "after_seq": 12345,
  "filters_hash": "sha256:...",
  "issued_at": "2026-04-30T00:00:00Z"
}
```

The server signs or MACs the cursor if it will be accepted from untrusted clients. If unsigned cursors are used initially, never trust cursor tenant/filter values over authenticated request context.

Keep the proto-level cursor opaque even if the first implementation internally stores an integer sequence. Exposing only `seq` would make later multi-tenant streams, server migration, cursor compression, or partitioned event logs protocol-breaking changes.

### ListEventsSince Semantics

Inputs:

- `cursor.cursor`: opaque position token. Empty means start from the newest safe default or from sequence 0 when an explicit replay mode is requested.
- `cursor.target`: optional target filter.
- `limit`: max events, capped server-side.

Rules:

- Return events with `sequence > cursor.after_seq` that the caller is authorized to see.
- Apply target/assignee/mention/capability filters server-side.
- Return `next_cursor` set to the last returned sequence, or the input sequence if no events are returned.
- Return at most `limit` events. If more matching events exist, clients discover that by calling again with `next_cursor`; an explicit `has_more` field can be added later but is not required for correctness.
- If the cursor is older than retained events, return a structured `cursor_expired` error with the earliest available cursor.
- If the cursor filters do not match the request filters, return `cursor_filter_mismatch`.

### Event Types

Minimum first-wave events:

```text
message.created
thread.followed
thread.unfollowed
task.created
task.claimed
task.updated
task.status_changed
task.done
task.cancelled
run.created
run.claimed
run.status_changed
run.step_appended
activity.logged
reminder.scheduled
reminder.cancelled
attachment.uploaded
computer.registered
computer.heartbeat
computer.stale
agent.profile_updated
lease.created
lease.renewed
lease.expired
```

Task graph and server automation events:

```text
task.split_proposed
task.split_applied
task.child_created
task.assigned
task.dependency_added
task.dependency_removed
task.assignee_changed
task.blocked
task.unblocked
task.sync_required
```

These events let agents observe server-side task creation, automatic decomposition, status synchronization, and work-stealing/handoff without repeatedly scanning all tasks.

Task graph events must include `graph_version` and enough snapshot metadata for clients to detect partial replay. For example, `task.split_applied` should include `parent_task_id`, `child_task_ids`, `depends_on_task_ids`, `root_task_id`, and the resulting graph version.

## Task Graph and Agent-aware Sync

Slock-style automatic task creation and splitting should be modeled as task graph mutations plus events, not as daemon-specific commands.

### Task Additions

Add these fields to the task model/proto in a later protocol slice:

```text
root_task_id
parent_task_id
subtask_ids
depends_on_task_ids
blocked_by_task_ids
source                   human | agent | server_rule | scheduler | import
created_by_agent_id
created_by_user_id
server_rule_id
split_proposal_id
graph_version
current_run_id
```

`graph_version` increments when parent/child/dependency relationships change. Agents can use it to detect stale task graph views.

### Task Graph Operations

Recommended RPC/actions:

```text
ProposeTaskSplit(parent_task_id, proposed_tasks, request_id)
ApplyTaskSplit(parent_task_id, proposal_id, selected_tasks, request_id)
CreateTaskGraph(root_task, subtasks, dependencies, request_id)
UpdateTaskGraph(task_id, patch, request_id)
ListTaskGraph(root_task_id)
```

All graph mutations are idempotent. Replaying `ApplyTaskSplit` with the same request ID must return the originally created child task IDs rather than creating duplicates.

### Agent Awareness

An agent should be notified through replayable events when any of these match:

- The event target is followed by the agent.
- The task/run is assigned to the agent.
- The event mentions the agent.
- The event requires a capability the agent has and the target permission allows.
- The agent owns the relevant computer/run lease.

This is a filter over the event store, not a separate task queue. `FetchAssignedRuns` remains an execution pickup API; `ListEventsSince` is the awareness API.

## Atomicity Boundary

Resource mutation and event emission must be atomic from the caller's point of view.

Preferred pattern:

1. Start database transaction.
2. Reserve idempotency record.
3. Mutate resource.
4. Append event in the same transaction.
5. Store idempotency terminal response referencing the event/resource.
6. Commit.

If a resource still lives outside Ent, use a transactional outbox boundary:

- Commit the resource mutation.
- Immediately append an outbox event with the same idempotency key.
- A background projector writes the canonical event.
- Until projection succeeds, duplicate idempotency calls must not re-execute the side effect.

The long-term target should move collaboration resources that need replay semantics into Ent or another database-backed store with transactions.

## Implementation Slices

### Slice A: Idempotency Infrastructure

Files likely touched:

- Ent schema for `IdempotencyRecord`.
- Small helper package, for example `pkg/idempotency`.
- Gateway/daemonhost handler integration for one or two low-risk RPCs.

Acceptance:

- Duplicate same request returns the original result.
- Duplicate different body returns conflict.
- Pending duplicate returns in-progress/retryable error.
- Secret-bearing responses are sanitized before caching.

### Slice B: Event Store and Cursor

Files likely touched:

- Ent schema for `CollaborationEvent`.
- Event append/list helper package.
- `ListEventsSince` implementation.
- `LogActivity` and `SendMessage` event emission as first producers.

Acceptance:

- Events have monotonic sequence.
- `ListEventsSince` returns stable ordered events and a next cursor.
- Old/invalid cursor errors are structured.
- Auth filters prevent cross-target leakage.

### Slice C: Run/Step Integration

Depends on task #5/#9 storage boundaries.

Acceptance:

- `AppendRunStep` emits `run.step_appended`.
- `UpdateRunStatus` emits `run.status_changed`.
- Reconnect can resume from last event cursor and last run step.

### Slice D: Task Graph Sync

Acceptance:

- Server-created and server-split tasks emit graph events.
- Replaying a split request does not duplicate child tasks.
- Agents can discover assigned/mentioned/capability-relevant task graph changes through `ListEventsSince`.

## Test Plan

Idempotency:

- `TestSendMessageRequestIDReturnsOriginalMessage`
- `TestCreateTaskRequestIDDoesNotDuplicateTask`
- `TestRequestIDReuseWithDifferentBodyConflicts`
- `TestPendingDuplicateReturnsRetryableInProgress`
- `TestSecretResponseIsNotCachedRaw`
- `TestAppendRunStepRequestIDDoesNotDuplicateStep`

Event replay:

- `TestListEventsSinceReturnsOrderedEvents`
- `TestListEventsSinceFiltersByTarget`
- `TestListEventsSinceFiltersAssignedAgent`
- `TestListEventsSinceRejectsExpiredCursor`
- `TestMessageTaskRunActivityEventsShareOneSequence`
- `TestTaskSplitEventsReplayToAssignedAgents`

Resume:

- `TestDaemonReconnectFetchesMissedTaskEvents`
- `TestDaemonReconnectDoesNotRepeatCompletedRequestID`
- `TestRunResumeUsesLastStepAndEventCursor`

## Risks

- Caching raw response JSON can leak secrets. Sanitize or reconstruct responses for secret-bearing methods.
- KV-backed stores cannot provide the unique constraints and atomic event append semantics needed for this protocol. Use Ent for the durable control-plane stores.
- Method renames can break idempotency replay. Store stable action names, not transport paths, if proto names are still moving.
- Event retention shorter than idempotency retention can make duplicate responses reference missing events. Keep aligned retention policies.
- Session-file backed messages cannot be atomically appended with database events. This is acceptable only as a transition state; long-lived collaboration messages should move behind the event/resource store.
