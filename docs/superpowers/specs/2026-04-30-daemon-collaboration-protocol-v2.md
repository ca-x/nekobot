# Nekobot Daemon Collaboration Protocol v2

Status: accepted design target

## Decision

Nekobot daemon should be a Slock-style collaboration control plane, not a runtime-specific agent launcher protocol.

The protocol has two layers:

1. **Collaboration protocol**: stable server-owned objects for `target`, `message`, `thread`, `task`, `run`, `run_step`, `activity`, `reminder`, `attachment`, `profile`, `computer`, `agent`, `capability`, `permission`, `lease`, and `event_cursor`.
2. **Runtime adapter protocol**: replaceable drivers that can start and supervise Codex CLI, Claude CLI, Kimi, ACP, stdio/MCP agents, or custom agents.

Core collaboration objects must not contain Codex/Claude/Kimi-specific launch flags. Runtime-specific data belongs in `runtime_profile.adapter_config`.

## Non-goals

- Do not make ACP the core Nekobot protocol. ACP is one possible runtime adapter.
- Do not make the CLI the durable server contract. CLI can remain the practical first adapter.
- Do not rely on daemon process memory for resume semantics.
- Do not hardcode a closed list of supported agent products in the core schema.

## Transport

The daemon/server protocol remains protobuf/gRPC for internal control-plane calls.

HTTP endpoints may expose derived WebUI views, and runtime adapters may use CLI/stdin/stdout/ACP/MCP internally. Those are implementation details behind the daemon.

## Target Model

Targets are stable strings that route collaboration operations:

- `#channel`
- `#channel:thread`
- `dm:@agent_or_user`

Rules:

- Messages, activity, tasks, reminders, and attachments always reference a target.
- Thread targets are not nested.
- A bare channel target can create or address the channel's default thread only when the caller explicitly asks for that behavior.
- Target parsing must be centralized; adapters must not invent their own target grammar.

## Core Objects

### Computer

Represents a daemon host.

Required fields:

- `computer_id`
- `display_name`
- `hostname`
- `os`
- `arch`
- `daemon_version`
- `status`: `online`, `stale`, `offline`, `degraded`
- `last_seen_time`
- `lease_id`
- `capabilities`

Current mapping:

- Existing `DaemonInfo.machine_id` maps to `computer_id`.
- Existing `DaemonInfo.daemon_id` is a daemon instance id, not the durable computer identity.

### Agent

Represents one agent identity visible to users and other agents.

Required fields:

- `agent_id`
- `computer_id`
- `runtime_profile_id`
- `name`
- `display_name`
- `description`
- `status`
- `last_activity_time`
- `capabilities`
- `permissions`
- `env_summary`
- `skills`
- `dm_targets`

Current mapping:

- Existing `AgentProfile.runtime_id` is close, but should become `agent_id` or explicitly reference an `agent_runtime_id`.

### Agent Lifecycle Control

Web/server control actions against an agent are explicit protocol operations, not implicit task or run status updates.

Required actions:

- `terminate`: stop the current agent process/session.
- `restart`: restart the agent without clearing durable session state.
- `restart_reset_session`: clear the current chat/session context, then restart.
- `restart_full_reset`: clear runtime-local state/env/session cache where permitted, then restart.

Required RPCs:

- `ControlAgent(ControlAgentRequest)`: accepts an `agent_id`, optional `computer_id`/`runtime_profile_id`, action, reason, requesting agent, and `request_id`.
- `SendAgentDirectMessage(SendAgentDirectMessageRequest)`: convenience wrapper for direct messages to an agent. It routes to the same DM target model as normal `SendMessage`.

Rules:

- Lifecycle controls require a runtime supervisor/adapter implementation before they can be accepted as completed work.
- Until runtime supervision exists, implementations may return a callable operation with `state = unsupported`; they must not pretend an agent was stopped or restarted.
- Lifecycle control requests must emit event-log facts such as `agent.control_requested`, then later `agent.terminated`, `agent.restarted`, `agent.session_reset`, or `agent.full_reset` when adapters perform the action.
- Direct agent messages are normal collaboration messages whose target is `dm:@agent_id`; the convenience RPC exists so WebUI/server callers do not need to construct target strings.
- Mutating lifecycle and direct-message calls must carry `request_id` and eventually participate in the same idempotency store as other mutating collaboration RPCs.

### RuntimeProfile

Represents how to run an agent.

Required fields:

- `runtime_profile_id`
- `kind`: `cli`, `acp`, `stdio`, `mcp`, `custom`
- `provider`: `codex`, `claude`, `kimi`, `custom`, etc.
- `model`
- `adapter_config`
- `workspace_id`
- `env`
- `skills`
- `capability_manifest`

Rules:

- Launch commands, ACP endpoint settings, stdio arguments, and model-specific options live here.
- Collaboration RPCs must not branch on provider names except through capabilities.

### Capability

Describes what an agent/runtime can do.

Examples:

- `messages.send`
- `messages.read`
- `tasks.claim`
- `tasks.update`
- `threads.follow`
- `attachments.upload`
- `reminders.schedule`
- `workspace.read`
- `workspace.write`
- `shell.exec`
- `browser.use`
- `subagents.spawn`
- `runtime.streaming`
- `runtime.resume`

Capability says "can do"; permission says "may do here".

### Permission

Describes what a user/agent is allowed to do for a target or run.

Examples:

- target-scoped message read/write
- task claim/update
- shell access
- file write access
- secret/env access
- channel notification access

Rules:

- A runtime that supports shell still cannot run shell unless the target/run permission allows it.
- Secret env values must never be returned through general profile/list APIs.

### Message

Stable collaboration message.

Required fields:

- `message_id`
- `target`
- `thread_id`
- `sender_id`
- `sender_kind`: `human`, `agent`, `system`
- `role`: `user`, `assistant`, `system`, `tool`
- `content`
- `attachments`
- `reply_to_message_id`
- `created_time`
- `request_id`

Current mapping:

- Existing `CollaborationMessage` is close but needs `sender_id`, `sender_kind`, `attachments`, and idempotent `request_id`.

### Thread

Conversation context under a target.

Required fields:

- `thread_id`
- `target`
- `channel_target`
- `topic`
- `summary`
- `followed_by`
- `message_count`
- `created_time`
- `updated_time`
- `last_message_id`

Current mapping:

- Existing `ThreadRecord` is usable but should stop hardcoding non-DM sessions as `#websocket:<session>`. Store target/channel ownership explicitly.

### Task

Human-visible work item.

Required fields:

- `task_id`
- `target`
- `thread_id`
- `title`
- `description`
- `status`: `todo`, `in_progress`, `in_review`, `done`, `blocked`, `cancelled`
- `assignee_id`
- `created_by`
- `created_time`
- `updated_time`
- `current_run_id`

Rules:

- Task is the collaboration contract.
- Execution details live in `Run` and `RunStep`.

Current mapping:

- Existing `Task` is useful but mixes assignment/runtime state with execution state. Add `target`, `assignee_id`, and `current_run_id`.

### Run

Durable execution attempt for a task or direct agent action.

Required fields:

- `run_id`
- `task_id`
- `target`
- `agent_id`
- `computer_id`
- `runtime_profile_id`
- `status`: `queued`, `leased`, `running`, `stale`, `succeeded`, `failed`, `cancelled`, `handoff_required`
- `lease_id`
- `request_id`
- `input_message_id`
- `last_seen_event_id`
- `started_time`
- `updated_time`
- `completed_time`
- `error`

Rules:

- A task may have multiple runs over time.
- Daemon restart resumes from server-side `Run` plus local workspace checkpoints.
- If the original process cannot resume, create a new run that continues from the last durable step.

### RunStep

Durable activity item inside a run.

Required fields:

- `step_id`
- `run_id`
- `sequence`
- `kind`: `message`, `tool_call`, `shell`, `file_change`, `test`, `commit`, `handoff`, `checkpoint`, `error`
- `status`
- `summary`
- `detail`
- `artifact_ids`
- `started_time`
- `completed_time`
- `request_id`

Rules:

- All adapter output lands as `RunStep` and/or `Activity`.
- WebUI progress, recovery, and audit must read run steps instead of process memory.

### Activity

Target-visible event log.

Required fields:

- `activity_id`
- `target`
- `run_id`
- `step_id`
- `agent_id`
- `kind`
- `summary`
- `detail`
- `created_time`

Current mapping:

- Existing `ActivityRecord` is the right direction. Add `run_id`, `step_id`, and cursor support.

### Reminder

Scheduled future collaboration event.

Current mapping:

- Existing reminder RPCs can stay, but should include `request_id`, `created_by`, `target`, and permission checks.

### Attachment

File/blob associated with a target, message, activity, or run step.

Required fields:

- `attachment_id`
- `target`
- `owner_id`
- `filename`
- `mime_type`
- `size_bytes`
- `storage_ref`
- `created_time`

Current gap:

- Daemon collaboration proto has no first-class attachment RPC yet.

### Lease

Time-bounded ownership of a computer/agent/run.

Required fields:

- `lease_id`
- `holder_id`
- `resource_type`
- `resource_id`
- `expires_time`
- `heartbeat_after_seconds`

Rules:

- Daemon disconnect marks leases stale, not immediately failed.
- Same computer/agent can renew and continue.
- Different agent takeover requires explicit handoff.

### EventCursor

Replay cursor for missed collaboration events.

Required fields:

- `cursor`
- `target`
- `last_event_id`
- `last_message_id`
- `last_activity_id`

Rules:

- Daemon reconnect must be able to pull missed events since cursor.
- Real-time streams are optional optimizations, not the source of truth.

## Required RPC Surface

Keep the existing broad categories, but reshape names and payloads around the v2 objects.

### Computer and Inventory

- `RegisterComputer(RegisterComputerRequest) returns RegisterComputerResponse`
- `HeartbeatComputer(HeartbeatComputerRequest) returns HeartbeatComputerResponse`
- `GetInventory(GetInventoryRequest) returns GetInventoryResponse`

Current status:

- Implemented in v1 before external release as `RegisterComputer` and `HeartbeatComputer`.
- Inventory now evolves from `Workspace + Runtime` toward `Computer + Agent + RuntimeProfile + Workspace + Capabilities`.

### Collaboration

- `GetServerInfo`
- `ListTargets`
- `ListThreads`
- `GetThread`
- `ReadMessages`
- `SendMessage`
- `FollowThread`
- `UnfollowThread`
- `ListTasks`
- `CreateTask`
- `ClaimTask`
- `UpdateTask`
- `UploadAttachment`
- `GetAttachment`
- `ScheduleReminder`
- `ListReminders`
- `CancelReminder`
- `ListActivity`
- `LogActivity`

Current status:

- Most of this exists except attachment APIs and explicit target listing.
- Existing channel/thread/message/task/reminder/activity RPCs should be retained but renamed away from `Collaboration` prefixes if this becomes the primary daemon protocol.

### Execution

- `CreateRun`
- `ClaimRun`
- `HeartbeatRun`
- `AppendRunStep`
- `CompleteRun`
- `CancelRun`
- `ListRuns`
- `GetRun`

Current status:

- First protocol slice is implemented as `FetchAssignedRuns`, `UpdateRunStatus`, `AppendRunStep`, `ListRuns`, and `GetRun`.
- Durable run/step storage is still pending; current execution bridges remote-agent tasks into run-shaped payloads.

### Event Replay

- `SubscribeEvents` streaming RPC, optional for real-time delivery.
- `ListEventsSince(EventCursor)` mandatory for reconnect recovery.

Current status:

- First protocol slice adds `EventCursor` and `ListEventsSince`; durable event persistence is still pending.

## Idempotency

Every side-effecting request must carry `request_id`.

Applies to:

- send message
- create/update/claim task
- create/claim/update/complete run
- append run step
- upload attachment
- schedule/cancel reminder
- log activity
- follow/unfollow thread

Server behavior:

- `(caller_id, request_id, method)` is unique.
- Duplicate requests return the first result.
- Request IDs expire only after the operation is safely outside retry windows.

Current status:

- Added to the v2-shaped mutating requests in proto; server-side idempotency storage is still pending.

## Resume Semantics

Daemon exit and restart are normal events.

Guaranteed recoverable:

- server-side messages
- thread state
- task status and assignee
- run status
- run steps
- activity log
- attachment metadata
- reminder state
- event cursor

Not guaranteed recoverable:

- model context not written to server/local workspace
- in-flight shell processes killed by host shutdown
- tool calls whose effects were not idempotently recorded

Resume algorithm:

1. Daemon restarts and registers the same `computer_id`.
2. Server returns active/stale leases and current event cursor.
3. Daemon lists stale/running runs assigned to its agents.
4. Runtime adapter decides whether process-level resume is supported.
5. If supported, adapter resumes and appends a `checkpoint` step.
6. If unsupported, daemon marks the old run `handoff_required` or `failed` and starts a new run from last durable task/thread/activity state.

## Security Rules

- Environment variables can be listed only as names plus redacted values unless the runtime has explicit secret-read permission.
- Activity and run steps must not store raw secrets.
- Channel account configs stay server-side; daemon receives only scoped delivery handles when needed.
- Capability does not imply permission.
- Permissions are target-scoped and run-scoped.

## Compatibility Assessment of Current Code

Current `proto/nekobot/daemon/v1/daemon.proto` is directionally correct and should not be thrown away wholesale.

Kept:

- `Workspace`
- `Runtime` as a temporary runtime inventory item
- `ComputerInfo`
- `ListChannels`
- `ListThreads`
- `ReadMessages`
- `SendMessage`
- `FollowThread`
- `UnfollowThread`
- collaboration task RPCs
- agent profile/env/DM RPCs
- reminder RPCs
- activity RPCs
- workspace explorer RPCs

Adjust:

- Rename `Machine` terminology to `Computer` before the protocol is considered public. Done in the proto surface.
- Split `Runtime` into `Agent` and `RuntimeProfile`.
- Add `Capability`, `Permission`, `Run`, `RunStep`, `Lease`, `EventCursor`, and `Attachment`. Done in the proto surface.
- Add `request_id` to all mutating calls. Done for the first v2-shaped protocol slice.
- Add explicit `target` fields to tasks/runs where missing.
- Stop deriving all non-DM thread targets as `#websocket:<session>`; persist the channel/thread target relation.
- Make `ActivityRecord` run-aware.
- Replace coarse `FetchAssignedTasks` / `UpdateTaskStatus` execution flow with run/step RPCs. Done at the proto and daemon client/server boundary.

## Migration Plan

### Slice 1: Protocol Shape

- Add v2 messages for `Capability`, `Permission`, `Run`, `RunStep`, `Lease`, `EventCursor`, and `Attachment`.
- Add request IDs to mutating v1-style requests.
- Replace old machine/task-status RPC names before public release.

### Slice 2: Inventory Alignment

- Extend daemon inventory to expose `Computer -> Agent -> RuntimeProfile`.
- WebUI Daemon page reads the inventory shape instead of raw runtime/workspace lists.

### Slice 3: Run/Step Persistence

- Add storage for runs and run steps.
- Convert daemon task execution writeback to append run steps and activity.

### Slice 4: Event Replay

- Add event log table/store.
- Add cursor-based list endpoint/RPC.
- Keep real-time delivery optional.

### Slice 5: Adapter Boundary

- Define adapter interface for CLI and future ACP/stdin runtimes.
- Move command/env/provider-specific fields into runtime profiles.

## Open Implementation Questions

- Whether v2 should be a new protobuf package (`nekobot.daemon.v2`) or an additive v1 evolution before release.
- Whether run/event stores should use Ent schemas immediately or start in the existing KV store and migrate later.
- How much WebUI run-step detail should be visible to non-admin tenant users.
