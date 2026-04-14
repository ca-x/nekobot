# Task Plan: Nekobot daemon protocol based on slock.ai insights

## Goal
Design and implement a Buf/protobuf-based Nekobot host daemon protocol and minimum viable daemon/server integration inspired by slock.ai's host-agent model, then verify, commit, and push it.

## Phases
- [x] Phase 1: Plan and setup
- [x] Phase 2: Analyze slock.ai daemon protocol and map it onto Nekobot
- [x] Phase 3: Design Buf/protobuf contract and integration boundaries
- [x] Phase 4: Implement minimum viable daemon + server/control-plane support
- [x] Phase 5: Verify, commit, and push

## Key Questions
1. What slock.ai concepts are architectural versus transport-specific?
2. What is the smallest protobuf surface that is genuinely useful in Nekobot now?
3. How should daemon, server, and UI boundaries be split so current frontend/backend stay usable?
4. What can be implemented now without needing a full runtime migration?

## Decisions Made
- Use scoped planning files instead of overwriting repo-global `task_plan.md` / `notes.md`.
- Prefer Buf/protobuf as the internal protocol and keep MCP/HTTP as outer adapters later.
- Build the smallest useful slice: machine/runtime inventory + daemon registration/control-plane visibility first.
- First usable daemon version must execute remote tasks and feed status/result back into the owning session; queue-only behavior is not sufficient.

## Errors Encountered
- `printf` shells with leading `---` text needed safer quoting while inspecting files.

## Status
**Completed** - Daemon protocol wave implemented, verified with targeted Go tests and frontend build, and pushed to `origin/main`.

## Modeling decision
- Reuse existing Nekobot meaning of `channel` as the user-facing entry surface.
- Model daemon/runtime-specific conversations as `thread` contexts under a channel, rather than as new channels.
- Goal: let users interact with different daemon-backed runtimes inside different message threads without colliding with Nekobot's existing channel architecture.

## First-version scope correction
The first version is only considered usable if channel-side daemon interaction exists too. This means the implementation must include a shared channel interaction layer for daemon-backed threads/tasks instead of limiting the feature to WebUI/control-plane state.

## Updated first-version target
1. daemon host startup
2. machine register + heartbeat
3. runtime/workspace inventory
4. server-side registry and status management
5. WebUI visibility for machine/runtime/task state
6. shared channel-to-daemon interaction layer
7. at least one high-value ingress surface wired through that layer
8. daemon execution/result loop that writes task outcomes back to owning sessions
9. derived online/offline machine state and WebUI bootstrap/install hints
