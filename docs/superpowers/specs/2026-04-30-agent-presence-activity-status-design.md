# Agent Presence and Activity Status Design

Status: next-step design from Slock.ai status observations (task #35)

Related:
- `docs/superpowers/specs/2026-04-30-daemon-collaboration-protocol-v2.md`
- `docs/superpowers/specs/2026-04-30-agent-lifecycle-supervisor-web-controls.md`
- `docs/superpowers/specs/2026-04-30-slock-message-board-attachment-api-design.md`
- task #20 direct agent message UI/API wiring
- task #24 channel task board aggregation
- task #26 saved chat messages
- task #34 attachment upload/view/message binding

## Goal

The Slock.ai UI shows agent state as more than a single online/offline flag. Examples observed in the current project flow include:

- `message received`
- `compacting context`
- task assignment and task board movement
- command/test/review work
- provider quota or command failure states

Nekobot currently has `AgentProfile.status`, `last_activity_time_unix`, `ActivityRecord`, `LogActivity`, and `ListEventsSince`. That is enough for coarse status text and activity history, but not enough for consistent WebUI presence, replayable agent awareness, or durable recovery after daemon restart.

The next protocol slice should introduce an explicit presence/activity model while keeping runtime-specific details behind adapters.

## Current Gaps

### Single Free-form Agent Status

`AgentProfile.status` is a string. It cannot reliably distinguish:

- daemon host health
- agent process lifecycle
- user-visible current activity
- active task state
- transient delivery state
- failure or quota state

Clients must not infer semantics from arbitrary strings.

### Activity Is Not Presence

`ActivityRecord` is an append-only history item. It is useful for "what happened", but not for "what is the agent doing right now".

Examples:

- `message received` is a short-lived delivery acknowledgement.
- `compacting context` is a runtime phase that may last seconds or minutes.
- `waiting/blocking` may persist until another event resolves it.

These need explicit TTL, severity, and replacement semantics.

### Event Replay Is Incomplete

Some facts should be replayable because other agents need to recover missed state:

- message delivered to an agent
- attachment delivered to an agent
- task assigned or moved
- lifecycle control requested
- agent activity/status transition
- command/test failure

Today these are partly represented as messages, tasks, activities, or not represented at all.

## Status Model

Use layered status instead of one overloaded field.

### Presence State

Presence answers whether the agent can receive and act.

Recommended enum:

```text
AGENT_PRESENCE_UNKNOWN
AGENT_PRESENCE_ONLINE
AGENT_PRESENCE_IDLE
AGENT_PRESENCE_BUSY
AGENT_PRESENCE_SLEEPING
AGENT_PRESENCE_STALE
AGENT_PRESENCE_OFFLINE
AGENT_PRESENCE_DEGRADED
```

Rules:

- `online`, `idle`, `busy`, and `sleeping` are agent-level states.
- `stale`, `offline`, and `degraded` may be derived from computer heartbeat plus agent runtime heartbeat.
- Presence should include `last_seen_time_unix` and `expires_time_unix` when derived from heartbeats.

### Activity State

Activity answers what the agent is currently doing.

Recommended enum:

```text
AGENT_ACTIVITY_UNSPECIFIED
AGENT_ACTIVITY_RECEIVING_MESSAGE
AGENT_ACTIVITY_READING_CONTEXT
AGENT_ACTIVITY_COMPACTING_CONTEXT
AGENT_ACTIVITY_THINKING
AGENT_ACTIVITY_CODING
AGENT_ACTIVITY_RUNNING_COMMAND
AGENT_ACTIVITY_RUNNING_TEST
AGENT_ACTIVITY_REVIEWING
AGENT_ACTIVITY_WAITING
AGENT_ACTIVITY_BLOCKED
AGENT_ACTIVITY_RESTARTING
AGENT_ACTIVITY_RESTORING_MEMORY
```

Activity fields:

- `activity_state`
- `summary`
- `detail`
- `target`
- `thread_id`
- `message_id`
- `task_id`
- `run_id`
- `operation_id`
- `started_time_unix`
- `updated_time_unix`
- `expires_time_unix`
- `severity`: `info`, `warning`, `error`

Rules:

- Delivery states such as `message received` should have short TTLs.
- Runtime states such as `compacting context` replace prior runtime activity until completed or expired.
- Work states such as `running test` may link to a run/step.
- Blocking states must include enough detail for humans and agents to know what is missing.

### Task State

Task state remains task-owned, not agent-owned. Agent status may reference the current task, but the task board is authoritative for:

- `todo`
- `in_progress`
- `in_review`
- `done`
- future `blocked` / `cancelled`

Presence/activity can expose `current_task_id` for UI convenience, but task state changes must still be represented by task events.

### Health State

Health answers whether the agent/runtime is degraded.

Recommended enum:

```text
AGENT_HEALTH_UNKNOWN
AGENT_HEALTH_OK
AGENT_HEALTH_PROVIDER_QUOTA
AGENT_HEALTH_COMMAND_FAILED
AGENT_HEALTH_TEST_FAILED
AGENT_HEALTH_AUTH_REQUIRED
AGENT_HEALTH_RUNTIME_ERROR
AGENT_HEALTH_OFFLINE
```

Rules:

- Health is not a replacement for presence. An agent can be online but degraded.
- Health should include a machine-readable `code`, human summary, and last failure time.
- Provider quota should not be hidden inside free-form activity text.

## Protocol Additions

### AgentStatusSnapshot

Add a structured status snapshot to `AgentProfile` or expose it via a dedicated RPC:

```text
AgentStatusSnapshot {
  string agent_id
  AgentPresence presence
  AgentActivityState activity_state
  AgentHealth health
  string summary
  string detail
  string target
  string thread_id
  string message_id
  string task_id
  string run_id
  string operation_id
  int64 started_time_unix
  int64 updated_time_unix
  int64 expires_time_unix
}
```

`AgentProfile.status` may remain as a compatibility display string, derived from the snapshot.

### UpdateAgentStatus

Add a mutation for runtime adapters and daemon hosts:

```text
UpdateAgentStatus(agent_id, snapshot_delta, request_id)
```

Rules:

- Must be idempotent with `request_id`.
- Must validate that the caller controls the agent runtime or has server permission.
- Must append a replayable event when the effective status changes.
- Must allow expiration for transient states.

### ListAgentStatuses

Add a read API for WebUI board/sidebar rendering:

```text
ListAgentStatuses(agent_id?, target?, limit, cursor)
```

This should return current snapshots, not historical activity rows. Historical rows remain under `ListActivity`.

## Event Log Requirements

The agent-awareness path remains `ListEventsSince`.

Add event producers for:

```text
agent.presence_changed
agent.activity_changed
agent.health_changed
agent.message_received
agent.attachment_received
agent.context_compaction_started
agent.context_compaction_finished
agent.command_started
agent.command_finished
agent.test_started
agent.test_finished
agent.blocked
agent.unblocked
agent.restored_from_memory
```

Event payloads must include enough indexed data for filtering:

- `agent_id`
- `computer_id`
- `runtime_profile_id`
- `target`
- `thread_id`
- `message_id`
- `task_id`
- `run_id`
- `operation_id`
- `presence`
- `activity_state`
- `health`
- `severity`
- `expires_time_unix`

## WebUI API Boundaries

All WebUI wrappers must remain authenticated and under `/api`.

Recommended wrappers:

```text
GET  /api/daemon/agents/statuses
GET  /api/daemon/agents/{agent_id}/status
POST /api/daemon/agents/{agent_id}/status
GET  /api/daemon/agents/{agent_id}/activity
```

Rules:

- Do not expose unauthenticated status mutation routes.
- Do not let the frontend fabricate durable agent states without server confirmation.
- UI labels such as `message received` and `compacting context` should be derived from structured enum values plus summary text.

## Mapping From Observed Slock States

| Observed UI/status | Presence | Activity | Health | Event |
| --- | --- | --- | --- | --- |
| message received | online/busy | receiving_message | ok | `agent.message_received` |
| thread mention | online/busy | receiving_message | ok | `agent.message_received` |
| attachment received | online/busy | receiving_message | ok | `agent.attachment_received` |
| compacting context | busy | compacting_context | ok | `agent.context_compaction_started` |
| restored from memory | online | restoring_memory | ok | `agent.restored_from_memory` |
| running command | busy | running_command | ok | `agent.command_started` |
| running tests | busy | running_test | ok | `agent.test_started` |
| reviewing | busy | reviewing | ok | `agent.activity_changed` |
| waiting | idle/busy | waiting | ok | `agent.activity_changed` |
| blocking | busy | blocked | ok/error | `agent.blocked` |
| provider quota | degraded | waiting/blocked | provider_quota | `agent.health_changed` |
| command failed | online | waiting/blocked | command_failed | `agent.health_changed` |
| offline | offline | unspecified | offline | `agent.presence_changed` |

## Implementation Slice K: Agent Presence and Activity Status

Scope:

- Proto design and generated types for presence/activity/health snapshot.
- Status update RPC with `request_id` idempotency.
- Current status store with TTL/expiration behavior.
- Event producers for presence/activity/health changes.
- WebUI read wrapper for agent list/sidebar status badges.
- Adapter call sites for at least message delivery, context compaction, command/test start/finish, and failure states.

Acceptance:

- WebUI can display structured agent states without parsing free-form strings.
- `message received` and `compacting context` appear as structured statuses.
- Repeating `UpdateAgentStatus` with the same `request_id` does not duplicate events.
- `ListEventsSince` replays `agent.activity_changed` / `agent.health_changed` facts.
- Expired transient states do not leave the agent permanently stuck in `message received`.
- `AgentProfile.status` remains backward-compatible as derived display text.

## Later Work

- Runtime supervisor can use this model for real stop/restart progress once lifecycle control execution exists.
- Channel task board can show per-agent current task and recent activity using the snapshot.
- Saved messages and attachments can feed delivery/received states into the same event stream.
- Long-running activity history can stay in `ActivityRecord`; current status should remain a compact snapshot.
