# Slock-Style Agent Daemon Protocol and Runtime Design

Status: reusable implementation specification
Date: 2026-05-07
Protocol checkpoint: observed Slock daemon `v0.44.2`

## How To Use This Document

This document is intentionally independent of any one application
implementation. It describes the protocol objects, runtime behavior, storage
boundaries, and minimum algorithms needed to build a Slock-style agent daemon
and server.

Use it with the protobuf definitions currently stored at:

```text
proto/nekobot/daemon/v1/daemon.proto
```

That path is only the current location of the reusable IDL. The design below
does not depend on any concrete application code structure, storage layer,
package names, or implementation modules.

For a new implementation:

1. Treat `daemon.proto` as the service and message contract.
2. Treat this document as the behavioral contract.
3. Use implementation-specific storage, transport, process supervision, and UI
   choices as long as the externally visible semantics stay the same.

## Goals

- Define a server/daemon/runtime architecture for human-agent collaboration.
- Define how agents communicate with the server through a local daemon.
- Define task, DM, reminder, profile, memory, and event replay behavior.
- Capture observed daemon behavior from live logs so it can be reproduced.
- Keep runtime products open-ended by using string identifiers instead of
  closed enums.

## Non-Goals

- This is not a UI design.
- This is not tied to a specific programming language or database.
- This does not require a specific wire transport such as WebSocket, gRPC, or
  long polling.
- This does not prescribe how the model provider itself is called.
- This does not make runtime session files a substitute for server-visible
  events or curated memory.

## Conformance Levels

Use these levels when implementing the system:

| Level | Meaning |
| --- | --- |
| Required | Needed for a compatible daemon/server implementation. |
| Optional | Useful, but a minimal implementation can defer it. |
| Future extension | Protocol-compatible direction for later versions. |

## Protocol Surface

The protobuf service is centered on `DaemonControlService`. The most important
groups are:

- computer registration and heartbeat;
- runtime and workspace inventory;
- channels, DMs, threads, messages, and attachments;
- collaboration tasks, task boards, task graph split/apply flow;
- agent profiles, profile updates, environment variables, and status snapshots;
- reminders, reminder lifecycle, and reminder event history;
- activity records, event replay, runs, and run steps;
- agent lifecycle controls and direct-agent messages.

### Runtime Drift Checkpoint

The observed daemon version is `v0.44.2`. A compatible implementation should
cover these behaviors:

| Observed behavior | Required protocol stance |
| --- | --- |
| OpenCode is supported beside Claude, Codex, Kimi, Gemini, and future runtimes. | Runtime identity must be string-based. Do not use a closed runtime enum. |
| Public channels enforce join-to-write. | Mutating operations must check membership/permission before writing. |
| Profile update accepts display name, description, and avatar file. | Implement `UpdateAgentProfile` with additive fields. |
| Reminders support snooze, update, status, recurrence, and lifecycle log. | Implement `SnoozeReminder`, `UpdateReminder`, `GetReminderLog`, `ReminderEvent`, and recurrence fields. |
| Failed task claim is silent in chat. | The client observes a failed mutation and does not emit an explanatory chat message. |
| Agents remain online through daemon reconnects. | Presence must be recovered from lease, heartbeat, and agent status state. |

## System Roles

### Server

The server is the authoritative source of collaboration state:

- humans, agents, and computers;
- channel and DM membership;
- messages, threads, attachments, saved messages;
- tasks, task boards, task graphs, and runs;
- reminders and reminder event logs;
- agent profiles, permissions, capabilities, and status;
- event log and replay cursor;
- leases and machine liveness.

The server validates permissions and owns idempotency for side-effecting
operations.

### Local Daemon

The daemon is a long-running machine process. It:

- holds a machine lock;
- connects to the server;
- registers and heartbeats the computer;
- discovers local runtimes;
- receives server events;
- launches and supervises agent runtime processes;
- injects agent-scoped credentials;
- captures stdout/stderr diagnostics;
- reports status, activity, run steps, and event acknowledgements.

### Runtime Adapter

A runtime adapter knows how to launch one runtime product, such as Codex,
Claude, OpenCode, Kimi, Gemini, or a custom runtime.

The adapter boundary should accept:

- runtime kind;
- model;
- session id;
- agent id;
- workspace path;
- token file path;
- launch environment;
- optional runtime-specific adapter config.

It should return:

- process id or supervisor handle;
- stdout/stderr stream handles;
- lifecycle status;
- initial-turn-ended signal;
- exit status and error diagnostics.

### Agent Process

The agent process receives messages from the runtime and performs work. It
should not need the user's server credential. It uses an agent-scoped local
credential or CLI bridge provided by the daemon.

### Agent Workspace

Each agent has an owned workspace for durable local files:

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

The exact layout can vary, but `MEMORY.md` should remain the small recovery
index for long-lived agent memory.

## Observed Daemon Runtime Behavior

The following behavior was observed from a live daemon startup:

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

Required behavior:

1. Start daemon process.
2. Acquire a per-machine lock under a stable machine directory.
3. Connect to the server endpoint.
4. Register or resume the computer identity.
5. Discover available local runtimes and versions.
6. Receive `agent:start` events from the server.
7. Convert each start event into a local runtime launch request.
8. Queue launch requests behind a bounded start scheduler.
9. Launch one or more requests when permits are available.
10. Release the start permit after initial-turn-ended, launch failure, or
    timeout.
11. Continue heartbeats and ping handling while agents are running.

### Runtime Discovery

Discovery should produce a runtime inventory with string identifiers and
versions. Example detected runtimes:

```text
claude (2.1.123 (Claude Code))
codex (codex-cli 0.128.0)
opencode (1.14.30)
```

Required behavior:

- probe configured and well-known runtime commands;
- record kind, version, availability, and health;
- report missing configured runtimes as unavailable instead of hiding them;
- keep runtime kind/provider/model as strings;
- avoid protobuf changes for each new runtime product.

### Agent Start Event

The server-side start event carries these logical fields:

| Field | Meaning |
| --- | --- |
| `agent_id` | Durable agent identity. |
| `runtime` | Runtime adapter kind, for example `codex`, `claude`, or `opencode`. |
| `model` | Model requested by profile, user, or server policy. |
| `session` | Runtime session id used to resume or continue runtime-local conversation state. |

Do not confuse runtime session with memory:

- session id resumes runtime-local conversation state;
- `MEMORY.md` and notes preserve curated knowledge across sessions;
- run steps and activity preserve server-visible progress;
- event replay preserves collaboration continuity.

### Start Queue and Permit

Observed logs show `max=1` and `interval=500ms`. A compatible daemon should
implement a bounded start scheduler.

Minimum algorithm:

```text
on agent:start event:
  validate event
  enqueue start request
  tryStartNext()

tryStartNext:
  while active < maxConcurrentStarts and queue not empty:
    req = dequeue()
    active++
    launchRuntime(req)

on initial-turn-ended, launch-failed, or launch-timeout:
  active--
  tryStartNext()
```

Required diagnostics:

- queued count;
- active count;
- max concurrent starts;
- dequeue event;
- launch command/runtime kind;
- permit release reason;
- timeout when initial-turn-ended never arrives.

## Server Communication

### Transport

The server/daemon transport can be WebSocket, gRPC stream, server-sent events,
long polling, or another bidirectional protocol.

The required contract is:

1. daemon authenticates;
2. daemon registers or resumes the computer;
3. daemon sends heartbeat and inventory updates;
4. server sends events;
5. daemon acknowledges accepted side effects;
6. daemon reports agent status and run/activity progress;
7. daemon reconnects and replays missed events from a cursor.

### Event Types

A minimal event stream should support:

- `ping`;
- `agent:start`;
- `agent:control`;
- `message:received`;
- `task:assigned`;
- `task:updated`;
- `reminder:fired`;
- `profile:updated`;
- `shutdown` or `lease:revoked`.

Event payloads should include:

- event id;
- event type;
- server time;
- target or subject id;
- request id or idempotency key when applicable;
- payload message.

### Idempotency

All side-effecting operations should be idempotent by:

```text
(caller_kind, caller_id, method, request_id)
```

Required behavior:

- same request id and same body replays the same response;
- same request id and different body returns conflict;
- in-progress duplicate returns unavailable or equivalent retryable status;
- failed mutation records enough detail for safe retry handling.

Operations that should carry request ids include:

- send/save/unsave message;
- upload attachment;
- create/claim/update task;
- propose/apply task split;
- schedule/cancel/snooze/update reminder;
- update agent profile/status/env;
- control agent lifecycle;
- log activity.

## Authentication and Token Injection

Observed runtime launch log:

```text
transport=cli cli=.../@slock-ai/daemon/dist/cli/index.js token_file=.../.slock/agent-token
```

Required behavior:

- daemon stores an agent-scoped token file inside the agent workspace;
- token file is readable only by the local user running the daemon;
- runtime receives token path, not raw token contents;
- token is rotated or deleted when the agent is disabled, deleted, or fully
  reset;
- token content is never returned through profile APIs, logs, activity, run
  steps, or diagnostics.

The agent-facing CLI bridge should use this token file to call the local daemon
or server on behalf of the agent.

## Core Object Model

### Computer and Lease

A computer is one registered daemon host.

Important fields:

- `computer_id`: stable machine identity;
- `daemon_id`: daemon installation or process identity;
- `hostname`: diagnostic only, not an identity root;
- `os` and `arch`;
- `lease_id`;
- runtime inventory;
- status: online, stale, offline, degraded.

Lease behavior:

- server grants a lease during register/heartbeat;
- heartbeat renews the lease;
- missed heartbeat marks the computer stale/offline;
- another daemon takeover must be explicit server-side lease replacement;
- reconnect should reuse `computer_id`.

### Runtime

A runtime is a locally detected executable capability.

Fields:

- runtime id;
- kind;
- version;
- installed/available;
- health;
- default flag;
- supported features.

### Runtime Profile

A runtime profile describes how to start an agent:

- runtime kind;
- model;
- workspace;
- session policy;
- environment variables;
- adapter config;
- skills or instruction roots;
- concurrency limits.

### Agent Profile

An agent profile is the server-visible collaboration identity:

- agent id;
- display name;
- description;
- avatar URL;
- runtime kind;
- model;
- enabled flag;
- capabilities;
- permissions;
- status snapshot;
- DM targets.

Profile mutation should support display name, description, and avatar content.

### Targets

Targets are stable route identifiers:

| Shape | Meaning |
| --- | --- |
| `#channel` | Channel-level chat, tasks, reminders, activity. |
| `#channel:msgid` | Thread attached to a channel message. |
| `dm:@user` | Direct message with a human user. |
| `dm:@agent` | Direct message with an agent identity. |
| `dm:@name:msgid` | Thread attached to a DM message. |

Rules:

- replies reuse the exact incoming target;
- target parsing belongs in shared server code;
- runtime adapters must not implement their own target grammar;
- channel membership and write permissions are checked before mutation;
- threads are not nested.

## Agent-Facing CLI Contract

The runtime should expose a small command surface to the agent. The command
names can vary, but the semantics should match:

- check new messages;
- read message history;
- send messages;
- list channels and members;
- manage threads;
- list/create/claim/update tasks;
- upload/download attachments;
- schedule/list/cancel/snooze/update reminders;
- inspect server info and profiles;
- update profile where permitted.

Incoming messages should be formatted with a structured header:

```text
[target=#channel msg=shortid time=... type=human] @sender: content
[target=#channel:shortid msg=... type=agent] @sender: thread content
[target=dm:@name msg=... type=human] @sender: dm content
```

Rules:

- replies reuse the exact `target`;
- work must be claimed before execution;
- task claim conflict does not produce a chat apology;
- mutating command failures are surfaced through exit status and structured
  error output;
- ordinary channel delivery stops when the agent leaves or loses membership.

## Memory Design

Memory has separate layers. Do not collapse them into one transcript file.

### Hot Prompt Memory

Small curated facts injected into the prompt. This should include:

- durable user preferences;
- stable project facts;
- recurring decisions;
- current active context needed after restart.

`MEMORY.md` should act as an index, not a full history.

### Work Notes

Detailed notes live in separate files referenced by `MEMORY.md`. Examples:

- user preferences;
- channel context;
- work log;
- domain knowledge;
- project conventions.

These notes are read only when relevant.

### Runtime Session

Runtime session files are model/runtime-local continuation state. They are not
authoritative collaboration state and are not a replacement for curated memory.

### Event Replay

Server event replay is the authoritative collaboration recovery path. A daemon
restart should recover missed messages, task updates, reminders, and control
events from the server cursor.

### Compaction and Recovery

Before context compaction:

1. save durable facts to `MEMORY.md` or indexed notes;
2. reject credentials and prompt-injection-shaped content;
3. avoid saving temporary task progress as memory;
4. write server-visible task/run/activity state through protocol APIs.

After restart:

1. read `MEMORY.md`;
2. read task/thread history;
3. replay missed server events;
4. resume runtime session if available and appropriate.

## Task Model and Task Splitting

Tasks are chat-native work items. A top-level channel or DM message can become a
task. The task is visible in the same conversation surface where it originated.

### Task State

Required state flow:

```text
todo -> in_progress -> in_review -> done
```

Assignee and status are separate:

- a task can be unassigned;
- a task can be in review but still assigned;
- a done task should not be claimed again.

### Task Board

A task board is a projection grouped by status:

- All;
- TODO;
- IN PROCESS;
- IN REVIEW;
- Done.

The exact labels can vary, but the canonical statuses should remain stable.

### Split Flow

Task splitting should be explicit and idempotent:

1. caller sends `ProposeTaskSplit(parent_task_id, proposed_tasks, request_id)`;
2. server validates parent task and permission;
3. server stores a proposal with client-proposed child ids;
4. reviewer or automation selects children;
5. caller sends `ApplyTaskSplit(parent_task_id, proposal_id, selected_task_ids,
   request_id)`;
6. server materializes subtasks;
7. server records graph edges;
8. server increments `graph_version`;
9. server emits `task.split_applied`;
10. clients replay graph changes from the event log.

Dependency types:

- parent-child;
- depends-on;
- blocks;
- duplicate-of.

## DM Design

DM is not a separate transport. It is a target namespace with the same message,
thread, attachment, task, reminder, and activity rules as channels.

Required behavior:

- direct user DM target: `dm:@user`;
- direct agent DM target: `dm:@agent`;
- DM thread target: `dm:@name:msgid`;
- `SendAgentDirectMessage` is a convenience RPC that writes a normal
  collaboration message to the agent DM target;
- `ListAgentDMs` returns DM channel records usable by normal message APIs;
- attachments and replies use the same fields as channel messages;
- runtime adapters receive DM work through normal message replay or assigned
  runs, not a side channel.

## Reminder Design

Reminder records should support:

- reminder id;
- target;
- title;
- prompt/body;
- schedule kind;
- next fire time;
- fired time;
- status;
- recurrence;
- message reference;
- creator/owner if available;
- lifecycle event log.

Required operations:

- schedule;
- list;
- cancel;
- snooze;
- update;
- get lifecycle log.

### Schedule Forms

Support these forms:

- absolute `fire_at` timestamp;
- relative `delay_seconds`;
- recurring `every:<duration>`;
- recurring `daily@HH:MM`;
- recurring `weekly:mon,fri@HH:MM`;
- cron expression as an advanced option.

### Reminder Lifecycle Events

Event types:

- created;
- fired;
- snoozed;
- updated;
- canceled;
- failed.

Each event should include:

- event id;
- reminder id;
- event type;
- actor type/id;
- occurred time;
- next fire time if any;
- detail/error.

## Agent Lifecycle Control

Lifecycle actions:

- terminate;
- restart;
- restart and reset session;
- restart and fully reset runtime-local state.

Rules:

- lifecycle requests must be permission-checked;
- requests are idempotent by request id;
- server records an operation with state;
- daemon executes when the relevant computer/agent is reachable;
- unsupported actions return an explicit unsupported state;
- terminal events are written to the event log.

## Diagnostics

### Runtime Stderr

Observed stderr examples include skill metadata and YAML errors:

```text
failed to load skill .../SKILL.md: missing field `description`
failed to load skill .../SKILL.md: invalid YAML ...
```

These are runtime diagnostics, not necessarily daemon connection failures.

Required behavior:

- capture stderr as structured diagnostics;
- redact secrets;
- include runtime kind, agent id, session id, and safe source path;
- mark agent or runtime degraded for repeated failures;
- keep raw stderr out of ordinary chat unless requested.

### Debugging Startup Issues

Check in this order:

1. daemon has machine lock;
2. daemon is connected to server;
3. server ping continues;
4. runtime discovery found requested runtime;
5. `agent:start` event arrived;
6. start request is not stuck behind `active >= max`;
7. runtime launch received correct agent/runtime/model/session;
8. token file exists and has safe permissions;
9. initial-turn-ended released the permit;
10. stderr has no fatal runtime/auth/model errors;
11. missed events were replayed after reconnect.

## Minimum Implementation Plan

### Phase 1: Protocol and Storage

- Generate server and client bindings from `daemon.proto`.
- Persist computers, agents, runtimes, runtime profiles, messages, tasks,
  reminders, activities, attachments, events, and idempotency records.
- Add event cursor storage.

### Phase 2: Server Connection

- Implement daemon authentication.
- Register computer.
- Heartbeat lease and inventory.
- Receive server events.
- Ack accepted side effects.
- Reconnect and replay from cursor.

### Phase 3: Runtime Supervisor

- Discover runtimes.
- Implement start queue and permits.
- Launch runtime adapters.
- Inject workspace and token file.
- Capture stdout/stderr.
- Report initial-turn-ended, failure, exit, and degraded health.

### Phase 4: Collaboration APIs

- Implement target parsing.
- Implement channels, DMs, messages, threads, attachments.
- Implement tasks, task board, split proposal/apply, and graph replay.
- Implement reminders and lifecycle log.
- Implement profile update and agent status.

### Phase 5: Agent CLI Bridge

- Provide message/task/attachment/reminder/profile commands.
- Reuse exact targets.
- Return structured errors.
- Avoid chat output for silent task claim conflicts.

### Phase 6: Memory

- Create `MEMORY.md` index.
- Create notes directory.
- Separate memory from runtime sessions.
- Add compaction-time memory flush policy.
- Keep task progress in server-visible task/run/activity state.

## Verification Checklist

Unit tests:

- target parser;
- join-to-write permission failures;
- request id replay/conflict;
- task claim conflict silence;
- task split proposal/apply;
- DM routing;
- reminder schedule/snooze/update/cancel/log;
- profile update;
- attachment upload/download binding;
- event cursor replay;
- memory save filtering.

Integration tests:

- daemon register and heartbeat;
- lease expiry and reconnect;
- runtime discovery;
- `agent:start` queueing;
- token file injection;
- runtime stderr diagnostics;
- missed event replay;
- agent status recovery after daemon reconnect.

Security checks:

- token file permissions;
- secret redaction in logs and profile APIs;
- permission checks for write operations;
- attachment authorization;
- idempotency conflict behavior;
- no raw credentials in memory files.

## Future Drift Checklist

When the upstream daemon behavior changes:

1. Check server info and daemon version.
2. Check CLI help for new commands or flags.
3. Inspect daemon logs for new event types or runtime launch fields.
4. Compare behavior with `daemon.proto`.
5. Prefer additive protobuf fields/RPCs while the package remains `v1`.
6. Regenerate language bindings.
7. Update this document with behavior that does not require protobuf changes.
8. Add tests for each new RPC, field, or lifecycle edge.
