# Claude Code Alignment Plan

## Goal
Evolve `nekobot` from a feature-accumulated single-agent application into a controlled agent runtime platform inspired by Claude Code's architecture, while preserving current Chat, Config, Harness, Gateway, and Channels behavior during the transition.

## Non-Goals
- Do not attempt a one-shot rewrite.
- Do not pause bug fixing or regression work for speculative architecture.
- Do not introduce remote/cloud execution in the first implementation phases.
- Do not break current runtime topology, channel routing, or WebUI chat as part of foundational refactors.

## Current Diagnosis
- `pkg/agent.Agent` currently acts as the de facto top-level execution object.
- `runtimeagents`, `subagent`, `skills`, `approval`, `session`, `watch`, `tool sessions`, and `memory` already exist, but they are not unified by one execution/task model.
- WebUI and Gateway expose only partial execution state.
- Skills, tools, and permissions are still more static than task-scoped.
- Subagents are queued background jobs, not full execution units with explicit state, permissions, lifecycle, and resumability.
- There is still no explicit `task lifecycle service` boundary for `enqueue -> claim -> start -> complete/fail/cancel`.
- Runtime execution concerns are not yet cleanly split into `runtime control plane`, `execenv`, and `agent backend` layers.
- Frontend control-plane state is still too page-local; it needs domain stores for runtime/task/channel/session views.

## Target Shape

### 1. AgentDefinition Layer
Create a first-class execution definition that combines:
- prompt and initial prompt.
- provider/model or provider group hints.
- tool allowlist and denylist.
- permission mode.
- memory/context policy.
- hook declarations.
- MCP/capability requirements.
- isolation mode.
- background eligibility.

This becomes the shared definition source for:
- main interactive agent.
- runtime-bound agents.
- built-in specialist agents.
- background/subagent tasks.
- future teammate-like workers.

### 2. Task Runtime Layer
Introduce a unified task model instead of treating subagents as a side queue.

### 2A. Task Lifecycle Service
Add a service boundary above the raw store so task state transitions are explicit and testable.

Core transitions:
- `enqueue`.
- `claim`.
- `start`.
- `complete`.
- `fail`.
- `cancel`.

Responsibilities:
- enforce task transition rules.
- separate runtime claiming from agent execution.
- emit state updates for WebUI, Gateway, daemon, and later inbox/coordinator layers.
- keep execution history and summary updates out of page handlers.

### 2B. Runtime Resource Control Plane
Treat runtime records as first-class control-plane resources, not only routing hints.

Each runtime should eventually expose:
- status and effective availability.
- last seen / heartbeat.
- usage and activity metrics.
- metadata and capabilities.
- bound accounts / bindings / supported channels.
- current claimed or running tasks.

Planned task categories:
- `interactive_main`.
- `local_agent`.
- `background_agent`.
- `runtime_worker`.
- `in_process_teammate` later.
- `remote_agent` later.

Each task should own:
- task ID and type.
- agent definition reference.
- session reference.
- permission context.
- execution state.
- pending action.
- task summary.
- lifecycle hooks.
- cleanup handlers.

### 3. Session/State Protocol
Promote execution state to a shared protocol across WebUI, Gateway, task runtime, and persisted sessions.

Minimum state payload:
- `state`: `idle | running | requires_action | failed | completed`.
- `pending_action`.
- `task_summary`.
- `last_error`.
- `runtime_id`.
- `actual_provider`.
- `actual_model`.
- `task_type`.
- `permission_mode`.

### 4. Dynamic Tool Assembly
Stop treating tools as one static registry for every execution path.

Instead, build task-scoped tool sets from:
- global registry.
- agent definition allowlist/denylist.
- permission context.
- runtime/channel constraints.
- feature flags and capability presence.

### 5. Permission Mode Layer
Make permission mode a runtime concept, not just an approval subsystem detail.

Initial modes:
- `default`.
- `plan`.
- `verify`.
- `restricted`.
- `manual_approval`.

Later, allow tasks and agent definitions to declare required/initial modes.

### 6. Specialist Agents
Implement `explore`, `plan`, and `verify` as controlled definitions first.

Principles:
- `explore`: read-only tools only.
- `plan`: read-only + structured output expectations.
- `verify`: adversarial validation, explicit evidence, can run in background.

These should reuse the same task runtime and state model as all other agents.

### 7. Context Economy
Upgrade context management from overflow fallback to explicit infrastructure.

Needed pieces:
- compact boundary markers.
- turn/session summaries.
- tool result summarization.
- task transcript pruning policy.
- resumable session/task metadata.

### 8. Capability Awareness
Make `skills`, `hooks`, `MCP`, and runtime capabilities visible to the model through a unified capability layer, so the model knows:
- what is available.
- when to use it.
- what is restricted.


## Design Inputs from Claude Code Docs

The additional documentation review on 2026-04-01 sharpened the alignment strategy in several important ways.

### 1. Prompt Assembly Is a Runtime System
We should not treat the system prompt as one static template.

Adopt:
- static sections for identity, behavior rules, tool grammar, and tone.
- dynamic sections for session guidance, capability deltas, permission mode, MCP instructions, and compacted context.
- an explicit boundary between cache-friendly stable prompt content and session-specific prompt content.

### 2. Task Runtime Needs a Store, Not Only a Schema
A task struct alone is insufficient.

We also need a runtime state holder that can later unify:
- active tasks.
- pending actions.
- permission requests and permission mode.
- task notifications and summaries.
- teammate/inbox events later.

### 3. Tool Execution Must Become a Governed Pipeline
The long-term target is:
- input validation.
- pre-tool hooks.
- permission decision.
- execution.
- post-tool hooks.
- failure hooks.

This should remain compatible with current tools while allowing progressively stronger runtime policy.

### 4. Capability Awareness Must Reach the Model
Skills, hooks, MCP, and task-specific tools should not only exist in backend registries.
They should also be reflected in the prompt/runtime contract so the model knows what is available and under what constraints.

### 5. Context Economy Is a First-Class Runtime Concern
The plan should explicitly cover:
- static/dynamic prompt boundaries.
- transcript summarization.
- task/session compact policies.
- resumable metadata for long-running work.

### 6. Coordinator/Daemon/Inbox Are One Layered Stack
Recommended sequence:
- task runtime and state store first.
- daemon supervision second.
- UDS inbox / notification bus third.
- coordinator semantics on top.
- bridge only after local control surfaces are stable.

### 7. Query Loop, Streaming Tools, and Lifecycle Cleanup Need Explicit Landing Zones
The V2 deep-dive makes three additional gaps impossible to ignore.

We should explicitly plan for:
- turn-level runtime state around the main query loop, not only coarse task snapshots.
- a future streaming tool execution slot, so tool dispatch is not permanently modeled as post-response batch work.
- cleanup contracts for execenv and daemon work, including shell/process cleanup, transcript metadata, and resume metadata.

This means:
- Phase 2 should leave room for runtime claim/report and active-turn visibility.
- Phase 3 should include cleanup handlers and resume metadata as first-class outputs.
- Phase 5 should leave an extension point for speculative classification and streaming tool execution.
- Phase 7 should include tool-result budgeting and reactive compaction, not only generic summarization.

## Additional Design Inputs from Multica

The review of `/home/czyt/code/multica` adds several concrete implementation biases that fit `nekobot`'s current transition stage.

### 1. Task Runtime Needs a Lifecycle Service, Not Only a Store
A shared `tasks.Store` is necessary but not sufficient.
We also need a lifecycle service that owns:
- enqueue
- claim
- start
- complete/fail/cancel
- task summary and event emission

This keeps execution transitions out of ad hoc manager calls and gives daemon/runtime workers one control surface.

### 2. Runtime Should Be a First-Class Control-Plane Resource
`runtime` should not remain only a routing target.
The control plane should grow toward:
- status
- last seen
- device/runtime metadata
- usage and task activity
- concurrency/capacity

This aligns with the future agent-channel-account model and makes runtime health visible before coordinator work starts.

### 3. Daemon, Runtime Execution, and ExecEnv Should Be Separate Layers
Execution should be split into:
- daemon/supervisor
- runtime worker execution
- execution environment preparation/injection
- agent backend invocation

This reduces pressure on `pkg/agent.Agent` and creates an obvious landing zone for workdir reuse, resume metadata, and task-scoped environment injection.

### 4. Frontend Control Plane Should Move Toward Domain Stores
As runtime/task surfaces grow, the frontend should stop relying on page-local fetch orchestration alone.
A better target is domain state for:
- runtime topology
- chat runtime/session state
- harness/task center
- shared workspace/config references

This should be phased in after state payloads stabilize, not before.

## Execution Phases

## Phase 0: Stabilization Gate
Must complete before architecture work expands.

Scope:
- finish current high-severity bugs.
- restore green regression baseline.
- ensure planning docs reflect actual status.

Exit criteria:
- targeted bug regressions pass.
- `npm --prefix pkg/webui/frontend run build` passes.
- relevant Go test packages pass.

## Phase 1: State, Task, and Runtime Store Foundation
Build the smallest shared runtime substrate.

Scope:
- define task model.
- define session/task state payload.
- introduce a minimal runtime state store shape.
- add execution state propagation to WebUI/Gateway.
- persist richer session metadata.
- expose recent tasks, counts, pending action, and summaries in control surfaces.

Files likely affected:
- `pkg/session/*`
- `pkg/webui/server.go`
- `pkg/gateway/server.go`
- new `pkg/tasks/*` or equivalent
- future `pkg/runtime/state/*` or equivalent
- frontend chat/system state hooks

Exit criteria:
- chat and gateway expose explicit task/session state.
- task state is visible in UI and persisted.
- later daemon/inbox work has one obvious place to plug into.

## Phase 2: Task Lifecycle Service and Runtime Resource Control
Add the missing middle layer between raw state and execution backends.

Scope:
- introduce a `task lifecycle service` over the shared task store.
- define transition APIs for `enqueue/claim/start/complete/fail/cancel`.
- add runtime-scoped claiming and assignment semantics.
- expose runtime status, last seen, active-task views, and minimal claim/report telemetry in control surfaces.
- keep runtime control-plane state separate from agent execution internals.
- leave one clear landing zone for future turn-level runtime state that the main query loop can reuse.

Exit criteria:
- background and runtime work no longer depend on ad hoc state transitions.
- runtime records can report current activity and health.
- future daemon/coordinator layers have a stable claim/report contract.

## Phase 3: Execenv Layer and Daemon Preparation
Separate execution environment construction from task state and from the agent backend itself.

Scope:
- introduce `execenv` or equivalent workspace/environment preparation layer.
- isolate working directory policy, skill injection, env var injection, cleanup handlers, and resume metadata.
- define daemon-facing contracts for starting background/runtime work.
- prepare watch/summarizer/background jobs to run through the same substrate.

Exit criteria:
- runtime worker execution can be prepared without hard-coding setup logic into `agent` or `webui`.
- future daemon work will not require another core refactor.

Progress update (2026-04-01):
- landed minimal `pkg/execenv` substrate with `StartSpec`, `Prepared`, and `Preparer`.
- `pkg/process.Manager` now supports `StartWithSpec()` and preparer injection.
- cleanup hooks now run through a single execution path on reset and natural exit.
- additional Phase 3 progress: runtime/task metadata now propagates into background exec sessions, agent tool sessions, and WebUI tool-session runtime restore paths.
- remaining Phase 3 work: daemon-facing start contract and broader background-worker adoption beyond the current tool-session/process paths.

## Phase 4: AgentDefinition and Prompt Assembly Introduction
Decouple execution definitions from the monolithic agent object.

Scope:
- introduce `AgentDefinition`.
- map existing runtime agent fields into definitions.
- include prompt policy, tool allow/deny, permission mode, hooks, MCP requirements, and context policy.
- create built-in definitions for main/explore/plan/verify.
- split system prompt construction into static and dynamic sections.
- define compatibility bridge from current runtime records.

Exit criteria:
- task runtime can execute from `AgentDefinition`.
- current main chat path still works through compatibility layer.
- prompt composition has a clear stable/dynamic boundary.

## Phase 5: Dynamic Tool Assembly and Governance Pipeline
Move from static tool pool to task-scoped execution assembly.

Scope:
- task-scoped tool filtering.
- permission mode introduction.
- allowlist/denylist support.
- pre/post/failure hook pipeline.
- hook-aware permission decisions.
- speculative classification / precheck insertion point.
- future streaming tool execution insertion point.
- capability registration path for skills and MCP-derived instructions.

Exit criteria:
- read-only and verification-style agent definitions can run with distinct tool pools.
- permission mode is visible in state.
- tool execution is no longer a bare call path.

## Phase 6: Specialist Agents and Background Tasks
Turn architectural primitives into real product behavior.

Scope:
- built-in explore/plan/verify agents.
- background task execution.
- task summaries.
- pending action support.

Exit criteria:
- specialist agents are no longer prompt-only conventions.
- background tasks have visible state and summaries.

## Phase 7: Context Compaction and Resumability
Upgrade lifecycle quality.

Scope:
- turn/session summary pipeline.
- compaction markers and transcript policy.
- tool result budgeting and summarization.
- reactive compaction fallback.
- prompt static/dynamic boundary enforcement.
- resumable background tasks where feasible.
- resume metadata for long-running work.

Exit criteria:
- context management is explicit, inspectable, and no longer purely overflow-driven.
- prompt assembly and compaction cooperate instead of fighting each other.

## Phase 8: Teammate/Swarm Preparation
Prepare for multi-agent collaboration without shipping full remote complexity first.

Scope:
- teammate-like task identity.
- shared permission bridge shape.
- message/result contract for inter-agent collaboration.
- mailbox/UDS inbox abstraction.
- idle/active worker semantics and push completion notifications.

Exit criteria:
- local collaborative task execution has clear extension points.
- coordinator semantics can be added without rewriting the runtime again.

## Extended Capabilities Roadmap

The following Claude Code-specific capabilities are worth adapting, but they should be layered on top of the core runtime rather than added as isolated features.

### A. Kairos-Style Persistent Presence
Adapt first:
- append-only daily activity log.
- background summarization pipeline.
- task/session inactivity budget with auto-backgrounding.

Recommended phase:
- after Phase 6 starts landing.
- mostly Phase 7 quality/lifecycle work.

### B. Daemon Supervisor
Adapt first:
- long-lived local supervisor process.
- worker registry for background tasks.
- restart-safe task metadata and cleanup.

Recommended phase:
- Phase 3 to Phase 7.

### C. UDS Inbox
Adapt first:
- local IPC between WebUI, Gateway, daemon, background workers, and future teammates.
- unified notification and permission/event bridge.

Recommended phase:
- Phase 7 to Phase 8.

### D. Coordinator
Adapt first:
- local master/worker orchestration over the same task runtime.
- push-style worker completion events.
- task-scoped scratch/work directories and summaries.

Recommended phase:
- Phase 8.

### E. Auto-Dream
Adapt first:
- local summarization jobs triggered by session count and age.
- distilled knowledge files instead of raw transcript growth.

Recommended phase:
- Phase 7 onward.

### F. Buddy
Adapt carefully:
- front-end persona/pet layer attached to task and session state.
- should remain optional and should not contaminate execution core.

Recommended phase:
- after task state and background runtime are already stable.

### G. Bridge
Not first:
- remote/mobile control is valuable, but only after local daemon + inbox + task state are mature.

### H. Ultraplan
Defer:
- remote planning and cloud execution should wait until local task runtime, plan approval, and session archival are robust.

## Bug-First Rule
If a bug affects:
- user-visible workflow correctness.
- runtime/account binding semantics.
- permission safety.
- state/session integrity.

then it preempts architecture work and is fixed first.

## Testing Strategy
- Add regression tests before behavior changes when practical.
- Keep targeted package tests for each phase.
- Rebuild frontend on every user-visible phase.
- Run broader Go regression after changes touching shared runtime layers.
- Do not claim completion without fresh command evidence.

## Initial Next Steps
1. Finish the current audited bug batch.
2. Freeze a green baseline.
3. Finish the session runtime state slice on top of the shared task store.
4. Start Phase 2 with a concrete task lifecycle service contract.
5. Define runtime control-plane snapshots before deeper daemon work.
6. Reserve daemon/inbox/context-compaction hooks so later Kairos/Coordinator/Buddy features do not require another execution-core rewrite.

## Success Criteria
- `nekobot` no longer depends on one monolithic always-on agent object for every execution path.
- execution units become task-backed and stateful.
- permissions, tools, and context become task-scoped.
- specialist agents reuse the same runtime model instead of bespoke flows.
- future multi-agent collaboration can be added without rewriting the execution core again.


## Recommended Delivery Waves

### Wave 1: Visibility and Runtime Store
- finish task snapshots in WebUI/system status.
- add pending action and task summary propagation.
- introduce a minimal runtime state store abstraction.

### Wave 2: Task Lifecycle and Runtime Control Plane
- add lifecycle transitions and runtime claim/report semantics.
- add runtime status, health, and active-task control-plane outputs.
- separate execution state mutation from handlers and page code.

### Wave 3: Execenv and Daemon Substrate
- introduce isolated execution environment preparation.
- unify env injection, workspace policy, resume metadata, and watch/background entrypoints.
- prepare daemon-supervised execution without shipping the full daemon yet.

### Wave 4: AgentDefinition and Prompt Boundaries
- factor `AgentDefinition`.
- split prompt construction into stable and dynamic sections.
- expose capabilities and permission mode in prompt assembly.

### Wave 5: Tool Governance
- task-scoped tool assembly.
- hook pipeline.
- permission mode enforcement.
- MCP/skill capability injection.

### Wave 6: Context Lifecycle
- transcript summaries.
- compact policies.
- resume metadata.
- Kairos-style daily activity log foundations.

### Wave 7: Supervisor and IPC
- daemon supervisor.
- UDS inbox.
- task notifications and restart-safe metadata.

### Wave 8: Coordinator and Specialists
- built-in explore/plan/verify as true runtime definitions.
- coordinator semantics.
- teammate ownership, idle semantics, and task delegation.
