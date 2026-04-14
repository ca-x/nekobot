# Nekobot Daemon Protocol Draft

Status: in-progress

## Goal
Build a first version that is genuinely usable, not just a protocol demo. A user should be able to start a daemon on a host, have the server track its liveness and inventory, and see that machine/runtime state from the WebUI/control plane.

## Guiding Principles
- `channel` keeps its existing Nekobot meaning: ingress + egress surface only.
- `thread` is the runtime interaction context under a channel.
- daemon protocol is internal system protocol and should be protobuf/buf-based.
- MCP remains an outer AI-facing adapter layer, not the core system protocol.

## First-Version Scope
The first version must support:
1. Daemon host startup
2. Machine registration with the server
3. Periodic heartbeat updates
4. Runtime + workspace inventory reporting
5. Server-side daemon state management
6. WebUI/system status visibility for online/offline, runtimes, and basic task load
7. Task fetch/update protocol that supports returning execution results to the owning Nekobot session/thread

The first version does **not** need to support:
- full @alias routing semantics
- #ALL broadcasts
- complete task dispatch to remote runtimes
- full message bridge compatibility

## Core Objects

### Machine
Represents one daemon host.

Fields:
- `machine_id`
- `machine_name`
- `hostname`
- `os`
- `arch`
- `daemon_version`
- `status` (`online`, `degraded`, `offline`)
- `last_seen_at`
- `registered_at`

### Workspace
Represents one project/workspace visible to a machine.

Fields:
- `workspace_id`
- `machine_id`
- `path`
- `display_name`
- `aliases[]`
- `is_default`

### Runtime
Represents one executable runtime within a workspace.

Fields:
- `runtime_id`
- `machine_id`
- `workspace_id`
- `kind`
- `display_name`
- `aliases[]`
- `tool`
- `command`
- `installed`
- `healthy`
- `config_dir`
- `supports_auto_install`
- `install_hint[]`
- `current_task_count`
- `pending_task_count`

### ChannelBinding (future-facing, not fully implemented in v1)
Represents how a channel chooses runtimes.

Modes:
- `exclusive`
- `shared`
- `routed`

### Thread (future-facing, not fully implemented in v1)
A runtime interaction context under a channel.

Fields:
- `thread_id`
- `channel_id`
- `workspace_id`
- `runtime_id`
- `topic`

### Task (future-facing, partial visibility only in v1)
Schedulable work unit.

Fields:
- `task_id`
- `thread_id`
- `created_by_user_id`
- `assigned_machine_id`
- `assigned_runtime_id`
- `status`
- `blocked_reason`

## First-Version Protocol Surface

### RegisterMachine
One-shot registration/upsert.

Purpose:
- establish machine identity
- publish initial workspace/runtime inventory

### HeartbeatMachine
Periodic state refresh.

Purpose:
- keep machine online
- refresh inventory snapshot
- carry lightweight task counts

### RuntimeInventory Snapshot
Heartbeat should include a full current snapshot for v1.
That is simpler and safer than premature delta protocols.

## Why heartbeat matters
The server must know when a daemon is offline so that:
- WebUI can show online/offline state
- tasks can be marked blocked/offline later
- routing avoids dead runtimes

## Channel vs Thread

### Decision
- **Channel** remains Nekobot's existing concept: the interaction entry surface (websocket, slack, telegram, wechat, etc.).
- **Thread** becomes the daemon/runtime interaction context within a channel.

### Rationale
Nekobot already uses channels to represent ingress surfaces. A slock-style daemon model should not overload `channel` to mean both ingress and runtime conversation target. Using `thread` for daemon-backed interaction contexts fits better with Nekobot's architecture and makes multi-daemon interaction inside one ingress surface meaningful.

### Implication for protocol design
The daemon protocol should favor entities like:
- `Machine`
- `Workspace`
- `Runtime`
- `ChannelBinding`
- `Thread`
- `Task`
- `AssignmentPolicy`

## Multi-user and single-ingress constraints
- Multi-user assignment is a future concern and should be modeled through thread/task/runtime assignment, not by mutating the meaning of channels.
- Single-ingress channels like WeChat should still support multiple runtimes behind one ingress by binding runtimes at the thread/task layer, not by pretending multiple channels exist.

## First implementation target
The first implementation should stop at:
- daemon run command
- register + heartbeat
- runtime/workspace inventory
- server-side registry
- WebUI status exposure

Only after that should task dispatch and thread-bound runtime selection be added.

## Task result writeback

The first usable task protocol must let a daemon return more than lifecycle state.

Additional v1 expectation:
- task completion/failure updates may include a textual result payload
- the server writes that payload back into the owning Nekobot session/thread as an assistant/system-visible message
- users can inspect remote progress from existing session views without a new dedicated daemon UI

This keeps the architecture aligned with the existing Nekobot model:
- channel = ingress/egress
- session/thread = conversation state
- daemon task updates = execution feedback feeding back into the session
