# Multi-Agent Runtime And Channel Account Design

**Date:** 2026-03-30

## Goal
Refactor `nekobot` from a single default-agent runtime into a multi-agent runtime architecture where channel accounts are first-class ingress/egress endpoints and agents are independently configurable runtimes that can be bound to one or more channel accounts.

## Product Decisions
- No historical-data migration is required for this architecture shift.
- Agents are true runtime objects, not just prompt/provider presets.
- Channel credentials belong to `ChannelAccount`, not to `AgentRuntime`.
- A channel type can own multiple accounts/endpoints.
- Multi-account support is a platform-wide channel capability, not a WeChat-specific special case.
- Agents bind to channel accounts, not directly to channel types.
- Default binding mode is `single_agent`.
- A channel account may be switched to `multi_agent` mode by the user.
- In `multi_agent` mode, inbound messages fan out to all bound agents.
- If the user explicitly selects an agent, only that agent may send a public reply.
- If the user does not select an agent, all publicly enabled agents may reply and replies must show agent identity.
- Session/history is strongly isolated per `channel_account × agent_runtime`.
- Harness capabilities move down to the agent layer, with global defaults and per-agent overrides.
- Memory uses a mixed model: agent-private memory plus optional shared pools.
- "Main/orchestrator agent" is a runtime role, not a permanent product identity.

## Current Problems
- `pkg/agent` is effectively a single global runtime with contextual overrides.
- `pkg/channels` binds transport ingress directly to the default agent path.
- WeChat/iLink auth persistence and runtime account selection are not aligned.
- WebUI models "channels" but not "channel accounts", "agent runtimes", or "bindings".
- Harness settings are mostly global, which blocks clean multi-agent isolation.
- Existing prompt bindings are useful but too shallow to represent real multi-agent behavior.

## Chosen Architecture

### 1. AgentRuntime as a first-class runtime object.
Add a first-class `AgentRuntime` model responsible for:
- provider policy
- prompt policy
- skills policy
- tool policy
- tool-session policy
- harness policy
- memory policy
- approval/safety policy
- session policy
- collaboration policy
- response policy
- observability policy

Each runtime owns its own session namespace and harness behavior while sharing selected infrastructure such as providers, workspace primitives, and optional shared memory pools.

### 2. ChannelAccount as a first-class transport endpoint.
Add a `ChannelAccount` model under each channel type:
- concrete credentials / bot identity / webhook or poll configuration
- runtime health and connectivity state
- user-facing metadata
- public reply labeling settings

The model must stay channel-agnostic.
Per-channel details live in typed/JSON metadata or channel-specific adapters, not in separate account models per channel family.

Examples:
- `wechat/<ilink-bot-a>`
- `wechat/<ilink-bot-b>`
- `slack/<workspace-app-a>`

### 3. AccountBinding as the routing contract.
Add an `AccountBinding` model:
- `channel_account_id`
- `binding_mode` = `single_agent` or `multi_agent`
- one or more `agent_runtime_id`
- per-agent exposure flags for public replies

This becomes the authoritative routing source for inbound and outbound traffic.

### 4. Fan-out dispatch and isolated sessions.
Inbound routing flow:
1. message enters a concrete `ChannelAccount`
2. binding manager resolves one or more `AgentRuntime` targets
3. each target gets an isolated session key derived from:
   - `channel_account_id`
   - external conversation identifier
   - `agent_runtime_id`
4. each runtime executes with its own prompt/provider/harness/memory configuration

Outbound routing flow:
1. each runtime emits replies through the originating `ChannelAccount`
2. replies are labeled with agent identity
3. in explicit-agent mode, only the selected agent may emit public replies

### 5. Collaboration slots, not permanent hierarchy.
An agent runtime may declare other runtimes as collaboration resources.
This does not create a hard platform-wide hierarchy. It only enables runtime-level dispatch:
- orchestrator-style invocation
- specialist delegation
- internal-only sub-results

### 6. Harness at the agent layer.
Move `undo`, `watch`, `audit`, `learnings`, and `file mentions` into agent runtime policy.
Keep global harness config only as defaults inherited by newly created or partially configured runtimes.

### 7. Memory layering.
Support both:
- agent-private memory
- optional shared memory pools

Each agent runtime decides:
- whether it reads from private memory only
- whether it may read from shared pools
- where writes go
- how compression / retention / search policy behaves

## WebUI Information Architecture

### New primary surfaces.
- `Agents`
  - create/edit runtime definitions
  - configure policy categories
- `Channel Accounts`
  - create/edit concrete channel endpoints
  - inspect runtime health
- `Bindings`
  - connect channel accounts to agents
  - switch `single_agent` / `multi_agent`
  - control public reply permissions

### Existing surfaces to adapt.
- `Chat`
  - expose agent source labels in multi-agent replies
  - allow explicit agent targeting where supported
- `Harness`
  - show selected/current agent runtime
  - edit runtime-local harness policy
- `Channels`
  - narrow scope to channel-type level capabilities, then route users to `Channel Accounts`

## Scope For Initial Multi-Round Delivery

### Round 1: Data model and runtime manager foundation.
- Introduce `AgentRuntime` storage/model/API.
- Introduce `ChannelAccount` storage/model/API.
- Introduce `AccountBinding` storage/model/API.
- Add in-process runtime manager for multiple agent runtimes.

### Round 2: WeChat/iLink as the first channel-account implementation.
- Use WeChat/iLink as the first concrete adapter onto the shared channel-account model.
- Replace single-binding WeChat runtime assumptions with account-aware routing against shared `ChannelAccount`.
- Keep the implementation shape reusable for every other channel with account semantics.

### Round 3: Harness downshift and runtime scoping.
- Move harness policy resolution to `AgentRuntime`.
- Keep global defaults as fallback-only config.
- Update audit/watch/undo/learnings/file-mentions to become runtime-aware.

### Round 4: WebUI reorganization.
- Add `Agents / Channel Accounts / Bindings` management surfaces.
- Update Chat and Harness pages for runtime/source awareness.

### Round 5: Collaboration slots and multi-agent fan-out.
- Add runtime-to-runtime collaboration declarations.
- Add multi-agent dispatch behavior and explicit-agent reply suppression.

## Explicitly Out Of Scope For First Implementation Wave
- Automatic keyword/rule-based agent selection.
- Response aggregation/synthesis into a single answer.
- Cross-process or distributed agent workers.
- Historical config/session migration.

## Testing Strategy
- TDD for all new storage and routing primitives.
- Boundary tests for runtime manager and binding manager.
- Channel-account integration tests starting with WeChat.
- WebUI tests for account/binding management and multi-agent reply labeling.
- Full `go test -count=1 ./...` and frontend build on each completed round.

## Notes
- This design intentionally favors deep modules over incremental handler-level patches.
- The first production-quality implementation target is an in-process multi-runtime architecture; out-of-process workers remain a future evolution path.
