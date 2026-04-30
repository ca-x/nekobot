# VibeAround Reference Roadmap for Nekobot

Date: 2026-04-30
Source reviewed: `git@github.com:jazzenchen/VibeAround.git` at `9f521ca`

## Purpose

VibeAround is useful as a reference because it solves adjacent problems from a local-first angle: multiple coding CLIs, IM channel access, browser terminal access, profile-based launch configuration, session handover, preview sharing, and skill/MCP injection.

Nekobot should not copy the product wholesale. Nekobot is becoming a multi-user collaboration hub with daemon/gateway protocol, task/message/event replay, ownership, and runtime control. The useful parts are the boundaries and lifecycle patterns, not the exact Rust/Tauri implementation.

This review is scoped to the areas @xczyt called out: message design, agent invocation, session handover, skills/MCP injection, and IM slash commands.

## Code-Backed Findings

### Message Design

Relevant VibeAround paths:

- `src/core/src/routing.rs`
- `src/core/src/channels/types.rs`
- `src/core/src/channels/transport_stdio/mod.rs`
- `src/core/src/channels/transport_stdio/handler.rs`
- `src/core/src/channels/transport_stdio/forwarder.rs`

VibeAround splits external chat identity from internal agent session identity. Plugins see the ACP `sessionId` as the IM `chatId`, while the host maps `(channelKind, chatId)` to an internal `RouteKey`. When forwarding agent notifications back to plugins, it rewrites the real agent session id back to `chatId`, so external channels never need to understand agent runtime session ids.

The useful Nekobot lesson is to keep one canonical server/channel/thread target for authorization and replay, while adding an explicit bridge-route mapping for external IM routes. External route ids should map into Nekobot targets; they should not replace them.

### Agent Invocation

Relevant VibeAround paths:

- `src/core/src/agent/runtime.rs`
- `src/core/src/conversations/mod.rs`
- `src/core/src/conversations/conversation/mod.rs`
- `src/core/src/conversations/conversation/lifecycle.rs`
- `src/resources/agents.json`
- `src/resources/profile-catalog/*.json`

VibeAround's `ConversationManager` owns `RouteKey -> Conversation`. A conversation lazily spawns an agent process, initializes ACP, creates or loads a session, and then forwards prompt blocks. Agent launch is driven by `agents.json` plus profile rendering; launch env includes route context such as `VIBEAROUND_CHANNEL_KIND`, `VIBEAROUND_CHAT_ID`, and `VIBEAROUND_AGENT_KIND`.

For Nekobot, ACP stdio should become one runtime adapter behind the daemon/runtime boundary, not the whole protocol. Nekobot should keep request_id idempotency, event replay, capability gates, and target binding at the gateway layer, then let adapters handle ACP/CLI-specific launch and session mechanics.

### Session Handover

Relevant VibeAround paths:

- `src/core/src/conversations/handover/mod.rs`
- `src/core/src/conversations/handover/pickup_codes.rs`
- `src/core/src/conversations/handover/handler.rs`
- `src/core/src/channels/prompt/handover.rs`
- `src/core/src/channels/prompt/handler.rs`

VibeAround supports two directions:

- CLI to IM: an MCP tool prepares a 4-character pickup code with a 120-second TTL. `/pickup CODE` consumes it once and wires the target chat to a resumed agent session.
- IM to CLI: `/handover` formats a resume command from the current conversation state and `agents.json` resume templates.

It also suppresses replay notifications during `load_session` so picking up a session does not flood the IM channel with old history.

For Nekobot, the right shape is a durable `session_handover` record rather than an in-memory code only. It should bind source target, destination target, runtime session id, workspace, runtime profile, expiry, one-time consume state, and event ids.

### Skills and MCP Injection

Relevant VibeAround paths:

- `src/core/src/agent/mcp.rs`
- `src/core/src/agent/skills.rs`
- `src/resources/mcp-tools.json`
- `src/skills/codex/vibearound/SKILL.md`
- `src/skills/codex/va-session/SKILL.md`
- `src/skills/codex/va-preview/SKILL.md`
- `src/skills/codex/va-md-preview/SKILL.md`

VibeAround installs skill files and MCP server entries for supported coding agents so the agent can discover session lookup, handover, workspace registration, preview, and markdown preview tools without the user manually wiring everything.

For Nekobot, generated skills/MCP config should default to an isolated runtime/profile home. Global installation should be explicit opt-in because Nekobot is multi-user and server-mediated. The first tool pack should expose current session/task context, attachment fetch, preview creation, status update, and handover preparation.

### IM Slash Commands

Relevant VibeAround paths:

- `src/core/src/channels/slash.rs`
- `src/core/src/channels/prompt/handler.rs`
- `src/core/src/channels/prompt/mode.rs`
- `src/core/src/channels/prompt/handover.rs`
- `src/resources/commands.json`

VibeAround treats IM slash commands as a small control plane:

- `/va ...` and `/vibearound ...` are Slack-friendly aliases.
- `/agent <command>` forwards a slash command into the downstream coding agent CLI.
- `/agent` lists cached agent-native commands from ACP `available_commands_update`.
- `/new`, `/close`, `/switch <agent>`, and `/profile <profile>` mutate the current conversation route.
- `/handover` and `/pickup` drive session movement between IM and CLI.
- `/plan` and `/mode <mode>` change the current agent permission mode and report system text confirmation.
- Unknown or malformed commands are handled locally instead of being sent as accidental prompts.

For Nekobot, slash commands should be modeled as first-class channel actions with permission checks and event replay, not just frontend text shortcuts. The Slock/WebUI command layer can share the same command catalog, while external IM bridges can expose aliases that map back to the same gateway RPCs.

## What VibeAround Does Well

### 1. Stable Route Identity

VibeAround models a conversation route as:

- `channel_kind`
- `bot_id`
- `chat_id`

That gives every IM conversation a stable key and allows separate bot identities inside the same platform.

For Nekobot, the equivalent should be a canonical route key across:

- Slock server/channel/thread/DM
- daemon runtime session
- external IM bridge route
- task/message/event target

This should build on the existing `target`, `thread_id`, and server/channel model rather than invent another channel table.

### 2. Plugin Process Boundary

VibeAround keeps IM channel integrations as separate Node.js plugin processes with:

- manifest/config discovery
- runtime process supervision
- heartbeat/watchdog
- crash/backoff/restart state
- isolated message envelopes

This is a good model for future Nekobot channel bridges. It avoids putting every platform SDK into the gateway and gives each integration its own failure boundary.

### 3. Runtime State Surfaces

VibeAround uses a simple state-source shape: `list()` for authoritative snapshots and `subscribe_changes()` for invalidation. This keeps dashboard/API consumers from depending on event payload schemas for every UI state.

Nekobot already has daemon inventory/status/event replay. The same pattern is useful for WebUI live views: snapshots are authoritative; events tell clients to refresh or replay detailed history.

### 4. Launch Profiles Without Global Config Churn

VibeAround has a provider/profile catalog that renders per-profile env vars and temporary settings files for different CLIs. This lets one CLI run against multiple providers without rewriting global config.

Nekobot should adapt this into runtime launch profiles:

- provider/profile catalog lives in server storage
- secrets remain redacted in APIs
- daemon receives scoped launch material
- generated settings live under a runtime/profile state directory
- no mutation of a user's global `~/.codex`, `~/.claude`, etc. by default

### 5. Session Handover and Pickup

VibeAround supports moving a live session between IM, browser, and terminal via `/handover` and `/pickup`.

For Nekobot, the valuable concept is a first-class `handover_token` or `session_pickup` record that binds:

- source target/thread
- runtime/session id
- workspace
- agent/runtime profile
- expiry and one-time use semantics

This fits Nekobot's daemon/gateway protocol better than a plain slash-command string.

### 6. Web Terminal and Preview Sharing

VibeAround exposes browser PTY sessions and preview links with short-lived share keys.

Nekobot already has process/session pieces and QMD/web tools. The roadmap should add:

- authenticated daemon terminal attach
- mobile-friendly terminal controls later
- preview artifacts for local dev servers and rendered markdown/html
- short-lived share keys with ownership/capability checks
- automatic cleanup when the owning runtime session closes

### 7. Skill and MCP Injection

VibeAround installs per-agent skill files and MCP config entries so agents discover its tools automatically.

Nekobot should use this as an opt-in runtime adapter feature:

- generate a small `nekobot` skill bundle per supported agent
- expose MCP tools for session id, task context, attachment fetch, preview creation, and status updates
- write only into an isolated runtime/profile home unless the user explicitly enables global install

### 8. Permission Request Bridging

VibeAround forwards ACP `request_permission` from an agent to the channel plugin and waits for a channel-native response.

Nekobot has permission rules and now has agent capability gates. The useful follow-up is to route agent approval requests into Slock threads/tasks so humans can approve/deny in context, with durable audit events.

### 9. IM Command Plane

VibeAround's slash parser is deliberately separate from ordinary prompt forwarding. This prevents control actions from being treated as user prompts and gives IM users a compact way to control sessions from mobile clients.

Nekobot should adopt the same separation for Slock and future IM bridges: command parse, authorization, RPC dispatch, system response, and event emission should be explicit steps. Agent-native passthrough commands should remain visibly distinct from Nekobot system commands.

## What Not To Copy Directly

- Do not adopt Tauri as the main product shell. Nekobot's current WebUI and daemon model is the right base.
- Do not make global CLI config mutation the default. Nekobot is multi-user and server-mediated; profile material should be isolated.
- Do not rely on request-field agent identity for strong auth. The #38 rollout kept this for compatibility, but future hardening should bind daemon auth identity to agent identity.
- Do not flatten all IM/channel routes into local-only route keys. Nekobot needs server/channel/thread ownership and permission checks.
- Do not use plugin crashes as the only source of channel state. Nekobot should persist status and events where they matter for replay and audit.

## Proposed Nekobot Roadmap Slices

These are VibeAround reference slices, namespaced as `VA-*`. They do not overwrite or renumber the Hermes memory/Curator slices Q-U from `docs/superpowers/specs/2026-04-30-hermes-memory-curator-redesign.md`.

### Slice VA-Q: Runtime Launch Profiles

Goal: let users define provider/API profiles and launch runtime agents with isolated env/settings.

Deliverables:

- Profile catalog schema for provider, API type, model, base URL, and secret fields.
- Server-side profile storage with secret redaction in list/get APIs.
- Runtime profile render step that produces env vars and generated config files under a scoped runtime state dir.
- Daemon launch uses rendered profile material without rewriting global CLI config.

Acceptance:

- Two runtimes can launch the same agent binary with different provider profiles.
- Profile secrets never appear in daemon inventory/profile APIs.
- Rendered files cannot path-traverse outside the profile state dir.

### Slice VA-R: Channel Bridge Plugin Runtime

Goal: add an isolated plugin process model for external IM/channel integrations.

Deliverables:

- Channel plugin manifest: id, runtime, entrypoint, config schema, capabilities.
- Plugin process supervisor with start/stop/restart, heartbeat timeout, crash count, and backoff.
- Canonical route mapping from plugin envelope to Nekobot server/channel/thread/DM target.
- Event producer for bridge status changes.

Acceptance:

- A plugin crash does not crash gateway.
- Missing heartbeat transitions to stale/offline/restarting state.
- WebUI can list plugin runtime status from an authoritative snapshot.

### Slice VA-S: Session Handover/Pickup

Goal: move an active daemon agent session between WebUI, Slock chat, and future channel plugins.

Deliverables:

- `CreateSessionHandover` and `PickupSession` protocol/API shape.
- One-time token with expiry, target binding, runtime id, workspace, and profile id.
- Event replay for `session.handover_created` and `session.picked_up`.
- UI/chat command entry points later.

Acceptance:

- Pickup from another target resumes the intended runtime session without losing target/thread linkage.
- Expired or reused tokens are rejected.
- Permission checks verify the user/agent can access both source and destination targets.

### Slice VA-T: Agent Tool Injection Pack

Goal: give supported agents a small Nekobot-native tool surface through skills/MCP.

Deliverables:

- Generated skill bundle for Codex/Claude/Gemini-style agents.
- MCP tools for current session lookup, task context, attachment fetch, preview creation, and status update.
- Runtime adapter manifest declares whether skill/MCP injection is supported.
- User-controlled install mode: isolated runtime home by default, global install only by explicit opt-in.

Acceptance:

- A launched runtime can discover Nekobot tools without manual config.
- Generated files are deterministic and auditable.
- Removing a runtime/profile removes isolated generated material.

### Slice VA-W: Channel Slash Command Plane

Goal: expose Nekobot's high-value collaboration controls as explicit Slock/IM commands instead of ad hoc message text conventions.

Deliverables:

- Command catalog shared by Slock, WebUI, and future channel bridge plugins.
- Gateway-backed handlers for session reset/close, agent switch, runtime profile switch, handover/pickup, mode changes, and agent-command passthrough.
- Capability checks before command execution.
- Event producers for command accepted/rejected and state-changing command outcomes.
- Agent-native command listing sourced from runtime adapter capabilities or ACP command updates.

Acceptance:

- A malformed command returns system text and never becomes an agent prompt.
- `/agent <cmd>` passthrough is auditable and visually distinct from Nekobot system commands.
- IM aliases map to the same gateway RPCs used by WebUI/Slock.
- State-changing commands are replayable through event_log.

### Slice VA-U: Preview and Artifact Sharing

Goal: expose local dev previews and rendered artifacts through authenticated, short-lived links.

Deliverables:

- Preview record model: owner session, workspace, kind, port/file, title, share key, expiry.
- Gateway routes for owner and share access with authorization.
- Daemon helper/API to register a running dev server or markdown/html artifact.
- Cleanup on session close.

Acceptance:

- Share links expire and cannot access other workspaces.
- Owner access remains stable while the session is active.
- Agent can create a preview event that appears in chat/thread history.

### Slice VA-V: Approval Request Bridge

Goal: route agent permission requests into Nekobot's human collaboration surface.

Deliverables:

- Durable approval request record linked to target/thread/task/session.
- Event producers for requested/approved/denied/cancelled.
- WebUI and Slock chat action surfaces for approval decisions.
- Timeout/cancel handling when a runtime/plugin dies.

Acceptance:

- Agent waits on a specific approval id and receives the human decision.
- Crashed runtime/plugin releases pending approvals as cancelled.
- Permission decisions are auditable through event replay.

## Suggested Priority

1. Slice VA-Q: Runtime Launch Profiles
2. Slice VA-T: Agent Tool Injection Pack
3. Slice VA-W: Channel Slash Command Plane
4. Slice VA-S: Session Handover/Pickup
5. Slice VA-U: Preview and Artifact Sharing
6. Slice VA-R: Channel Bridge Plugin Runtime
7. Slice VA-V: Approval Request Bridge

Reasoning: launch profiles and tool injection improve the daemon/runtime core immediately. Slash commands make the IM/Slock control surface usable without adding a new runtime subsystem. Handover and preview improve the daily collaboration loop. External IM plugin runtime is valuable, but it should wait until Nekobot's internal server/channel/thread permission model is fully stable.

## Integration Notes

- Reuse existing daemon/gateway protocol patterns: request_id idempotency, event_log replay, capability gates, target/thread binding, and redacted profile APIs.
- Keep VibeAround-inspired features as protocol slices, not one large UI feature.
- Prefer Go-native daemon/gateway implementation in Nekobot; VibeAround's Rust/Tauri code is reference architecture only.
- Treat `ACP stdio` as one runtime adapter option, not the only adapter boundary.
- Keep IM slash commands as a control plane: parse and authorize before prompt forwarding, emit events for state changes, and keep agent-native passthrough explicit.
