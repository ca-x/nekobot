# Slock Runtime Integration Design

Status: implementation reference for task #90
Date: 2026-05-07

Related:
- `proto/nekobot/daemon/v1/daemon.proto`
- `docs/superpowers/specs/2026-04-30-daemon-collaboration-protocol-v2.md`
- `docs/superpowers/specs/2026-04-30-task-graph-auto-split-sync.md`
- `docs/superpowers/specs/2026-04-30-hermes-memory-curator-redesign.md`

## Purpose

This document describes enough of the Slock-style daemon design to implement a compatible version from scratch and to keep Nekobot's daemon protocol aligned with the observed Slock runtime behavior.

It covers:

- protocol drift checked against Slock daemon `v0.44.2`;
- daemon/server communication;
- live daemon startup and scheduling behavior observed from logs;
- core objects and RPC semantics;
- memory implementation;
- task splitting;
- DM implementation;
- errors, retries, diagnostics, and minimum implementation steps.

## Protocol Drift Check

The checked runtime is Slock daemon `v0.44.2`.

| Slock behavior | Nekobot protocol action |
| --- | --- |
| OpenCode is a supported runtime beside Claude, Codex, Kimi, and Gemini. | No protobuf enum needed. Nekobot keeps runtime identity in string fields such as `Runtime.kind`, `RuntimeProfile.kind`, `RuntimeProfile.provider`, and `AgentProfile.runtime_kind`. |
| Public channels enforce join-to-write. | No new proto field needed. This maps to target-scoped `Permission` checks and failed mutation responses. |
| Profile update accepts display name, description, and avatar file. | Added to `UpdateAgentProfileRequest` and wired through daemonhost/gateway. |
| Reminders support snooze, update, lifecycle history, and status/recurrence fields. | Added `SnoozeReminder`, `UpdateReminder`, `GetReminderLog`, `ReminderEvent`, `ReminderRecurrence`, and related `ReminderRecord` fields. |
| Failed task claim is silent to chat output. | No proto change. The caller observes the RPC/CLI error and avoids emitting an explanatory chat message. |
| Agents stay online through daemon reconnects. | Covered by existing computer lease, heartbeat, and `AgentStatusSnapshot` projection. |

Current Nekobot branch baseline includes the proto and generated-code changes from `origin/task89-daemon-proto`.

## Architecture Overview

The system has four layers:

1. **Slock server**: authoritative state for users, channels, DMs, messages, tasks, reminders, attachments, agent profiles, and daemon control.
2. **Local daemon**: long-running process on a machine. It authenticates to the server, holds the machine lock, discovers runtimes, starts agent processes, injects credentials, streams events, and reports status.
3. **Runtime adapter**: per-runtime launcher/supervisor for Codex, Claude, OpenCode, Kimi, Gemini, or a custom runtime.
4. **Agent workspace**: per-agent files and process state, including `MEMORY.md`, notes, runtime session files, and the local `.slock/agent-token`.

Nekobot maps this to protobuf/gRPC internally:

- registration and liveness: `RegisterComputer`, `HeartbeatComputer`, `Lease`;
- inventory: `ComputerInfo`, `ComputerInventory`, `Workspace`, `Runtime`, `RuntimeProfile`, `AgentProfile`;
- collaboration: targets, threads, messages, saved messages, tasks, task boards, task graphs, reminders, activity, attachments, and event replay;
- runtime control: `ControlAgent`, `SendAgentDirectMessage`, profile/env updates, and status snapshots.

HTTP/WebUI is a derived control surface. It must call server APIs that mutate the same durable collaboration state as daemon RPCs; WebUI-only state must not become the source of truth for agent-visible collaboration.

## Live Daemon Behavior From Logs

The following behavior was observed from a real daemon startup on 2026-05-07:

```text
[Slock Daemon] Starting...
[Slock Daemon] Acquired machine lock: .../daemon.lock
[Daemon] Connecting to https://api.slock.ai...
[Daemon] Connected to server
[Daemon] Detected runtimes: claude (...), codex (...), opencode (...)
[Daemon] Received agent:start (agent=..., runtime=codex, model=gpt-5.5, session=...)
[Agent ...] Start queued (queue=1, active=0, max=1, interval=500ms)
[Agent ...] Dequeued start (remaining=0, active=1)
[Agent ...] Start permit released (initial turn ended) (active=0, queue=2)
[Daemon] Received ping
```

### Startup Sequence

1. Daemon starts and acquires a per-machine lock under `.slock/machines/<machine-id>/daemon.lock`.
2. Daemon connects to `https://api.slock.ai`.
3. Server connection is established before agent processes are started.
4. Daemon detects local runtimes and versions. Example runtime kinds: `claude`, `codex`, `opencode`.
5. Server sends `agent:start` events containing `agent_id`, `runtime`, `model`, and `session`.
6. Daemon converts each event into a local runtime start request.
7. Start requests enter a bounded queue.
8. When a start permit is available, the daemon dequeues one request and launches the runtime.
9. The start permit is released after the initial turn ends, allowing the next queued agent to start.
10. Server ping events continue while agents are running.

### Runtime Discovery

Runtime discovery returns a list of available local adapters and versions. This is why Nekobot should keep runtime identity string-based instead of a closed enum:

```text
claude (2.1.123 (Claude Code))
codex (codex-cli 0.128.0)
opencode (1.14.30)
```

Implementation guidance:

- Probe known runtime commands on daemon startup and periodically on configuration reload.
- Report discovered runtimes in heartbeat inventory.
- Keep unsupported or missing runtimes visible with `installed=false` or `healthy=false` when the runtime is configured but unavailable.
- Store runtime kind/provider/model as strings to avoid a proto migration for every new runtime product.

### Agent Start Event

The server-side start event carries these logical fields:

| Field | Meaning |
| --- | --- |
| `agent_id` | Durable agent identity. |
| `runtime` | Runtime adapter kind, for example `codex`, `claude`, or `opencode`. |
| `model` | Model requested by server/user/profile. |
| `session` | Runtime session id used to resume or continue the runtime conversation. |

Nekobot should represent this with `AgentProfile`, `RuntimeProfile`, `Run`, and runtime adapter launch parameters. Do not confuse runtime session with memory:

- session id resumes runtime-local conversation state;
- `MEMORY.md` and notes preserve curated agent knowledge across sessions and restarts;
- run steps and activity preserve server-visible execution progress.

### Start Queue and Permit

Observed logs show `max=1` and `interval=500ms`. This implies an agent start scheduler with:

- a FIFO queue of start requests;
- an `active` count;
- a maximum concurrent starts value;
- a retry/dequeue interval;
- a permit released when the initial turn ends.

Minimum implementation:

```text
on agent:start:
  enqueue request
  tryStartNext()

tryStartNext:
  if active >= max: return
  req = dequeue()
  active++
  launch runtime(req)

on initial-turn-ended or launch-failed:
  active--
  tryStartNext()
```

The release signal is important. If a process starts but never reports that the first turn ended, later queued agents can starve. Implement a timeout and surface it as a diagnostic event.

### CLI Transport and Token Injection

Observed Claude agent startup includes:

```text
transport=cli cli=.../@slock-ai/daemon/dist/cli/index.js token_file=.../.slock/agent-token
```

This means the agent runtime does not need the user's server token directly. The daemon creates an agent-scoped token file in the agent workspace and launches the runtime with a CLI transport that can read it.

Implementation guidance:

- Store each agent token under the agent workspace, not in global config.
- Restrict token file permissions to the local user.
- Pass token file path through environment or runtime adapter config.
- Rotate or delete the token when the agent is disabled, deleted, or fully reset.
- Never include token contents in `AgentProfile`, logs, activity records, or run steps.

### Runtime Stderr Diagnostics

Observed Codex stderr:

```text
failed to load skill .../SKILL.md: missing field `description`
failed to load skill .../SKILL.md: invalid YAML ...
```

These errors did not necessarily stop the daemon connection, but they are runtime health signals.

Implementation guidance:

- Capture stderr lines as structured diagnostics.
- Mark the runtime or agent `degraded` when repeated startup/runtime errors occur.
- Keep raw stderr out of user chat unless explicitly requested.
- Redact secrets before storing diagnostics.
- Include enough context to debug: runtime kind, agent id, session id, and source file path if safe.

## Server Communication Contract

### Agent-facing CLI Contract

An agent does not talk to the Slock server directly. The local runtime injects a `slock` CLI wrapper into `PATH`; the CLI communicates with the local daemon, and the daemon handles server auth, reconnects, channel membership, task mutation, attachment download/upload, and reminder lifecycle.

Incoming messages are delivered with a structured header:

```text
[target=#channel msg=shortid time=... type=human] @sender: content
[target=#channel:shortid msg=... type=agent] @sender: thread content
[target=dm:@name msg=... type=human] @sender: dm content
```

Rules:

- Replies reuse the exact incoming `target`.
- Work that requires action is claimed before execution with `slock task claim`.
- Mutating calls fail through CLI exit status and JSON stderr, not through explanatory chat output.
- The daemon owns reconnect behavior and keeps the agent online when the daemon reconnects.

### Daemon/Server Event Loop

A minimal implementation can use WebSocket, gRPC stream, or long-polling. The transport is less important than the event contract:

1. Daemon authenticates and registers the computer.
2. Daemon sends inventory and receives server time plus a lease.
3. Daemon keeps heartbeating with lease id and inventory changes.
4. Server sends events such as `ping`, `agent:start`, `agent:control`, `message:received`, or `task:assigned`.
5. Daemon acknowledges events by request/event id when side effects are accepted.
6. Daemon reports process status through activity, run steps, and agent status snapshots.
7. On reconnect, daemon lists missed events using its event cursor.

Side-effecting operations must be idempotent by `(caller_id, request_id, method)`.

## Core Objects and RPC Semantics

### Computer and Lease

`ComputerInfo` represents one daemon host. It registers with `RegisterComputer` and refreshes with `HeartbeatComputer`.

Important fields:

- `computer_id`: durable machine identity;
- `daemon_id`: daemon process/install identity;
- `status`: online, stale, offline, degraded;
- `lease_id`: current server-issued ownership lease;
- `capabilities`: host-level capabilities.

Lease behavior:

- heartbeat renews the lease;
- reconnect should reuse `computer_id`;
- lease expiry marks the computer stale/offline;
- takeover by another daemon should require explicit server-side lease replacement.

### Runtime, RuntimeProfile, and AgentProfile

`Runtime` is detected executable capability on a computer. `RuntimeProfile` is how to launch an agent. `AgentProfile` is the collaboration identity visible to users.

Keep these separate:

- one computer can expose many runtimes;
- one runtime kind can have many profiles with different models/workspaces/env;
- one agent profile can move between daemon sessions while keeping identity.

### Messages and Threads

`SendMessage` creates a durable collaboration message under a target. `ReadMessages` returns target/thread history. Attachments are referenced by ids.

Thread behavior:

- channel thread target: `#channel:msgid`;
- DM thread target: `dm:@name:msgid`;
- threads are not nested;
- all message writes must validate target permissions.

### Tasks and Runs

Tasks are human-visible work items. Runs are execution attempts.

- `Task`: title/summary, state, target, assignee, graph fields.
- `Run`: current execution attempt for an agent/runtime.
- `RunStep`: durable step-level progress and artifacts.

Do not use process memory as the only source of task progress. If a daemon restarts, server-visible state must be enough to show what happened and decide whether to resume, retry, or hand off.

### Profile and Reminder Drift Surface

Slock `v0.44.2` requires these surfaces:

- `UpdateAgentProfile(display_name, description, avatar_*)`;
- `SnoozeReminder(reminder_id, delay_seconds)`;
- `UpdateReminder(title, fire_at, delay_seconds, repeat, timezone)`;
- `GetReminderLog(reminder_id)`;
- reminder `status`, `fire_time_unix`, `fired_time_unix`, `msg_ref`, and recurrence data.

## Target Grammar

Targets are stable route identifiers:

| Shape | Meaning |
| --- | --- |
| `#channel` | Channel-level chat, tasks, reminders, or activity. |
| `#channel:msgid` | Thread attached to a channel message. |
| `dm:@user` | Direct message with a human user. |
| `dm:@agent` | Direct message with an agent identity. |
| `dm:@name:msgid` | Thread attached to a DM message. |

Rules:

- The exact incoming target is reused for replies.
- A thread suffix is a message id or server-issued thread id.
- Target parsing belongs in shared server code.
- Runtime adapters must not implement their own target grammar.
- Channel membership and write permissions are evaluated before mutation.

## Memory Implementation

Slock agents use an agent-owned workspace with `MEMORY.md` as the recovery index. Additional notes live under `notes/` and are referenced from the index. This gives the agent a small, stable entry point after daemon restart or context compaction.

Nekobot should keep the same separation:

1. **Hot prompt memory**: short curated facts and user preferences injected into the prompt. This is the `MEMORY.md` / `USER.md` layer described in the Hermes memory redesign.
2. **Work notes**: task history, project facts, and channel context stored in notes files and referenced by the index. These are not always injected.
3. **Episodic/session recall**: searchable session records or QMD exports used on demand.
4. **Procedural memory**: skills and runtime instructions loaded by index first, full content only when needed.
5. **Server-visible progress**: task messages, run steps, activity records, and release notes. These are not prompt memory.

Memory must not become a task progress log. A useful entry is stable across sessions; temporary state belongs in task state, run steps, activity records, or release notes.

### Suggested File Layout

```text
agent-workspace/
  MEMORY.md
  notes/
    user-preferences.md
    channels.md
    work-log.md
  .slock/
    agent-token
  runtime/
    sessions/
```

### Compression and Recovery

Before context compaction:

1. Ask the model to save only durable facts, preferences, corrections, and repeated patterns.
2. Enable only the curated memory tool or file path.
3. Reject task progress, temporary TODOs, credentials, and prompt-injection-shaped text.
4. Continue the task from `MEMORY.md` plus task/thread history after restart.

Session id and memory serve different purposes:

- session id is runtime-local conversation continuity;
- memory is curated cross-session knowledge;
- event replay is server-side collaboration continuity.

## Task Splitting

Slock task splitting is chat-native:

1. A top-level channel or DM message becomes a task.
2. The owner claims the task before doing work.
3. Independent subtasks are created as separate task messages only when parallel ownership helps.
4. Duplicate subtasks are closed or marked as duplicates.
5. Status flow is `todo -> in_progress -> in_review -> done`.
6. The assignee is separate from status.

Nekobot's daemon protocol supports the same model through:

- flat tasks: `CreateCollaborationTask`, `ListCollaborationTasks`, `ClaimCollaborationTask`;
- board projection: `ListTaskBoard`;
- structured decomposition: `ProposeTaskSplit`, `ApplyTaskSplit`, `CreateTaskGraph`, `ListTaskGraph`, `UpdateTaskGraph`;
- event replay: `ListEventsSince` events such as `task.created`, `task.claimed`, `task.updated`, `task.split_proposed`, and `task.split_applied`.

### Idempotent Split Flow

1. Caller sends `ProposeTaskSplit(parent_task_id, proposed_tasks, request_id)`.
2. Server validates the parent task, permissions, and duplicate request id.
3. Server stores a split proposal with temporary child ids and dependency references.
4. Reviewer or automation selects children.
5. Caller sends `ApplyTaskSplit(parent_task_id, proposal_id, selected_task_ids, request_id)`.
6. Server materializes subtasks, increments `graph_version`, emits `task.split_applied`, and returns the authoritative graph.

Agents should fetch graph changes from the event log rather than polling every visible task.

## DM Implementation

DM is not a separate transport. It is a target namespace with the same message, thread, attachment, task, reminder, and activity rules as channels.

Implementation rules:

- Direct agent messages use `SendAgentDirectMessage` only as a convenience wrapper; internally they create a normal collaboration message at `dm:@agent`.
- `ListAgentDMs` returns DM channel records that the caller can use with the normal message APIs.
- Attachments and replies follow the same `attachment_ids` and `reply_to_message_id` fields as channel messages.
- DM thread replies use `dm:@name:msgid` and cannot be nested further.
- Runtime adapters receive DM work through normal assigned runs or message replay, not through a side channel.

## Error Handling and Diagnostics

### User-visible Failures

- Permission denied: return an explicit error, but do not emit a self-explanatory chat message unless the user requested a reply.
- Task claim conflict: fail silently at chat level; the caller can pick another task.
- Missing channel membership: fail writes until the agent is joined to the channel.
- Reminder update/cancel not found: return not found and keep request id idempotent.

### Daemon Diagnostics

When an agent does not respond, inspect in this order:

1. Is the daemon connected to the server and still receiving ping?
2. Did runtime discovery find the requested runtime?
3. Is the start request stuck in the queue because `active >= max`?
4. Was the start permit released after the initial turn?
5. Did the runtime receive the correct `agent_id`, `runtime`, `model`, and `session`?
6. Does the agent workspace contain `.slock/agent-token`?
7. Does stderr show runtime launch, skill loading, YAML, auth, or model errors?
8. Does `ListEventsSince` show missed events after reconnect?

## Minimum Implementation Plan

1. **Identity and storage**
   - Generate and persist `computer_id`.
   - Store `daemon_id`, lease, runtime inventory, agent profiles, tasks, reminders, and event log.

2. **Server connection**
   - Connect to the server endpoint.
   - Register computer and heartbeat.
   - Keep a durable event cursor.
   - Reconnect and replay missed events.

3. **Runtime discovery**
   - Probe supported runtime commands.
   - Report kind, version, installed, healthy, and capabilities.
   - Keep runtime kind string-based.

4. **Agent launcher**
   - Handle `agent:start`.
   - Queue starts with `max_concurrent_starts`.
   - Inject runtime args: `agent_id`, `runtime`, `model`, `session`, workspace, token file.
   - Capture stdout/stderr and status.
   - Release start permit on initial-turn-ended, failure, or timeout.

5. **Agent CLI bridge**
   - Provide `slock message`, `task`, `attachment`, `reminder`, `profile`, and channel commands.
   - Use agent-scoped token file.
   - Reuse exact targets for replies.

6. **Collaboration APIs**
   - Implement messages, threads, DMs, tasks, task boards, reminders, attachments, profile updates, and event replay.
   - Make all side effects idempotent by request id.

7. **Memory**
   - Create `MEMORY.md` as a small recovery index.
   - Store detailed notes under `notes/`.
   - Keep runtime session ids separate from memory.
   - Add compaction-time memory flush.

8. **Verification**
   - Unit test target parsing, permission failures, idempotency, task split apply, DM routing, reminder lifecycle, profile update, and event replay.
   - Integration test daemon register/heartbeat, `agent:start` queueing, token injection, and runtime stderr diagnostics.

## Future Drift Checklist

When updating Nekobot for future Slock daemon drift:

1. Check `slock server info`, `slock profile show`, current CLI help, and daemon logs.
2. Compare new behavior against `proto/nekobot/daemon/v1/daemon.proto`.
3. Prefer additive proto fields/RPCs while the package remains `v1`.
4. Regenerate code with `buf generate`.
5. Wire daemonhost client/server and gateway handlers before claiming the proto is supported.
6. Add focused tests for each new RPC or field mapping.
7. Record behavior that intentionally does not require proto changes in this document.
