# Weclaw Agent Scheduling Evaluation

## Summary

`weclaw` and `nekobot` both speak ACP, but they do it at different layers:

- `nekobot` today exposes itself as an ACP agent via [`cmd/nekobot/acp`](../cmd/nekobot/acp.go) and [`pkg/agent/acp_adapter.go`](../pkg/agent/acp_adapter.go).
- `weclaw` uses ACP and CLI as upstream transports for external coding agents such as Claude, Codex, Cursor, Gemini, and OpenClaw.

These are complementary, not competing, designs.

## What Is Worth Migrating

### 1. External agent auto-detection.

`weclaw/config/detect.go` is useful because it:

- prefers `claude-agent-acp` over `claude`
- prefers `codex-acp` over `codex`
- supports ACP-only agents such as Cursor, Gemini, OpenClaw, and OpenCode
- picks a default agent by priority order

This is a good fit for `nekobot`, but only after `nekobot` gains an explicit runtime model for multiple upstream agents.

### 2. ACP-first, CLI-fallback transport strategy.

`weclaw` has a pragmatic transport policy:

- ACP when available
- native CLI fallback when ACP is missing

This is operationally useful for local deployments because users may have only one of these binaries installed.

### 3. Conversation-to-session mapping for external agents.

`weclaw` keeps `conversationID -> ACP sessionID` or `conversationID -> Claude sessionID`, which preserves multi-turn behavior for external agents.

That pattern should be reused if `nekobot` later adds upstream-agent routing.

## What Should Not Be Copied As-Is

### 1. The full `weclaw` agent layer.

`weclaw` assumes a config model shaped around:

- `default_agent`
- `agents[name]`
- per-agent `type=acp|cli|http`

`nekobot` currently uses:

- a single provider/model orchestration pipeline
- ACP exposure on top of that internal agent
- no first-class runtime registry for multiple upstream agents

Dropping `weclaw`'s agent layer directly into `nekobot` would create a second routing model and duplicate orchestration concerns.

### 2. CLI invocation semantics for Codex/Claude without workspace policy alignment.

`weclaw`'s CLI wrappers are intentionally simple. `nekobot` has stricter workspace, tool-session, and orchestration expectations. Any upstream CLI agent integration should respect existing workspace and approval boundaries.

## Recommended Migration Plan

### Phase 1.

Add a new runtime section for upstream agents, for example:

- `agents.upstreams[]`
- fields: `name`, `transport`, `command`, `args`, `endpoint`, `model`, `priority`

### Phase 2.

Port `weclaw` auto-detection into a `nekobot` bootstrap/runtime helper:

- detect `claude-agent-acp`, `claude`
- detect `codex-acp`, `codex`
- detect `agent acp`, `gemini --acp`, `opencode acp`, `openclaw acp`

### Phase 3.

Implement an upstream agent adapter in `nekobot`:

- ACP transport client
- CLI transport client
- session mapping for multi-turn conversations

### Phase 4.

Expose upstream-agent selection in WebUI and optionally channel commands.

## Recommendation

Do not merge the full `weclaw` scheduling stack into this WeChat migration commit.

The right next step is:

1. keep `nekobot`'s existing internal agent and ACP server intact
2. add first-class upstream-agent configuration
3. reuse `weclaw`'s detection and transport ideas on top of that model

This keeps the architecture coherent and avoids introducing two incompatible agent-routing systems in the same codebase.
