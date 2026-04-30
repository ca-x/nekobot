# Agent Lifecycle Supervisor and Web Controls

Status: Milestone 1 design (task #18)

Related:
- `docs/superpowers/specs/2026-04-30-daemon-collaboration-protocol-v2.md`
- task #17 lifecycle control proto/doc/stub
- task #19 lifecycle control idempotency and event replay
- task #20 direct agent message UI/API wiring

## Goal

Define the first executable boundary for Slock-style Web controls over daemon-managed agents:
terminate, restart, restart with session reset, restart with full reset, and direct message.

Milestone 1 must not pretend the runtime can be controlled before a supervisor or adapter implements it. The Web surface should show the operator what is supported, unsupported, pending, failed, or completed, using the same protocol and event log that daemon clients use.

## Control Ownership

Lifecycle control has three layers:

1. **Gateway / collaboration control plane**
   - Accepts and validates `ControlAgent` requests.
   - Applies request_id idempotency.
   - Writes `agent.control_requested` and later terminal lifecycle events when known.
   - Returns `accepted=false,state=unsupported` until an executor exists.

2. **Supervisor boundary**
   - Owns operation state and dispatches the command to the correct computer/runtime adapter.
   - Does not hard-code Codex/Claude/Kimi behavior into collaboration handlers.
   - Converts adapter outcomes into operation states and event-log records.

3. **Runtime adapter**
   - Knows how to terminate, restart, clear session state, clear runtime-local cache, and launch the agent for one runtime profile.
   - Reports unsupported actions explicitly.

## Action Semantics

### terminate

Purpose: stop the current agent process/session for the selected agent.

Minimum behavior:
- Stop the active process if a supervisor owns it.
- Mark current operation terminal once the process is stopped or confirmed absent.
- Do not delete durable collaboration history, task graph state, env, skills, or runtime profile.

Events:
- `agent.control_requested`
- `agent.terminated` on success
- `agent.control_failed` on failure

### restart

Purpose: restart the agent without clearing durable session state.

Minimum behavior:
- Stop active process if present.
- Start a new process from the same runtime profile.
- Preserve session/thread context, env, skills, workspace, and runtime-local caches.

Events:
- `agent.control_requested`
- `agent.terminated` if an existing process is stopped
- `agent.restarted` on successful relaunch
- `agent.control_failed` on failure

### restart_reset_session

Purpose: clear the current chat/session context, then restart.

Minimum behavior:
- Clear only the active agent session context selected by the operation target.
- Preserve durable channel messages, task graph, run history, env, skills, and workspace.
- Restart from the same runtime profile after session reset.

Events:
- `agent.control_requested`
- `agent.session_reset`
- `agent.restarted`
- `agent.control_failed` on failure

### restart_full_reset

Purpose: clear runtime-local state permitted by policy, then restart.

Minimum behavior:
- Stop active process.
- Clear runtime-local session/cache/state owned by the adapter for that agent/profile.
- Preserve server-owned durable collaboration state: messages, tasks, runs, event_log, env profile metadata, permissions, and audit trail.
- Must not delete workspace files unless a future explicit destructive permission is added.

Events:
- `agent.control_requested`
- `agent.full_reset`
- `agent.restarted`
- `agent.control_failed` on failure

## Operation States

Use `AgentControlOperation.state` with these string values for Milestone 1:

- `unsupported`: no supervisor/adapter can execute the requested action.
- `requested`: request accepted and recorded but not yet dispatched.
- `queued`: waiting for computer/agent availability.
- `running`: supervisor/adapter is executing the action.
- `completed`: action finished successfully.
- `failed`: action reached a terminal failure.
- `timed_out`: supervisor did not complete before deadline.

Milestone 1 gateway stubs must return `accepted=false,state=unsupported`. Later supervisor-backed implementations may return `accepted=true,state=requested|queued|running`.

## Timeout and Retry Rules

- Every mutating lifecycle request uses `request_id`.
- Same request_id and same body returns the original operation and event_id.
- Same request_id with a different body returns conflict.
- Retry after a terminal failure replays the failure, not a new operation.
- Supervisor timeout should produce `agent.control_failed` with a timeout reason or `agent.control_timed_out` if a distinct event type is added.
- Operations should have a server-generated `operation_id` that is stable under idempotency replay.

Recommended defaults:
- terminate timeout: 30 seconds.
- restart timeout: 90 seconds.
- restart_reset_session timeout: 120 seconds.
- restart_full_reset timeout: 180 seconds.

## Web Control Surface

### Entry Points

Primary entry point: each agent row/card in the daemon UI exposes an action menu.

Required actions:
- Direct message
- Terminate
- Restart
- Reset session and restart
- Full reset and restart

### Display Rules

For each action, Web should derive display state from agent profile/capabilities plus latest operation state:

- `supported`: button enabled.
- `unsupported`: button disabled with short tooltip/status copy.
- `pending`: button disabled while state is `requested`, `queued`, or `running`.
- `failed`: show failed operation state and allow retry with a new request_id.
- `completed`: show latest completed time, then return button to enabled if still supported.

Do not show lifecycle actions as successful when `ControlAgent.accepted=false` or operation state is `unsupported`.

### Confirmation Rules

No confirmation required:
- Direct message

Confirmation required:
- Terminate
- Restart
- Reset session and restart

Strong confirmation required:
- Full reset and restart

Full reset copy must state that server-owned collaboration history is preserved, while adapter-local cache/session state may be cleared.

### Button Disable Rules

Disable lifecycle buttons when:
- agent profile is missing or disabled.
- action capability is absent or explicitly unsupported.
- an operation for the same agent is `requested`, `queued`, or `running`.
- computer/daemon is offline and no queued operation support exists.
- current user lacks future lifecycle permission enforcement.

Direct message is disabled only when:
- agent_id is missing.
- DM target cannot be derived.
- message content is empty.

## API Shape for Web

Milestone 1 can call gRPC internally or expose HTTP wrappers later. If HTTP wrappers are added, keep them thin:

- `POST /api/daemon/agents/{agent_id}/control`
  - body: `{ action, reason, computer_id?, runtime_profile_id?, request_id }`
  - returns the `ControlAgentResponse` shape.

- `POST /api/daemon/agents/{agent_id}/message`
  - body: `{ content, attachment_ids?, reply_to_message_id?, request_id }`
  - calls `SendAgentDirectMessage` or existing `SendMessage` with `dm:@agent_id`.

These wrappers must not implement a second message or lifecycle protocol.

## Milestone 1 Acceptance

Task #18 is complete when:

- This boundary document exists in repo.
- Web UI action states and disable/confirmation rules are defined.
- Supervisor/adapter responsibilities are clear enough for Milestone 2 implementation.
- The design explicitly preserves collaboration state and avoids destructive workspace deletion.
- No gateway/runtime implementation is changed by this design slice.

## Recommended Follow-ups

- Milestone 1 task #19: connect `ControlAgent` idempotency and lifecycle event replay.
- Milestone 1 task #20: direct agent message UI/API wiring.
- Milestone 2: durable lifecycle operation store and supervisor dispatch loop.
- Milestone 2: adapter-specific support matrix for Codex, Claude, Kimi, ACP, stdio, and MCP runtimes.
- Milestone 2: lifecycle permission enforcement and audit UI.
