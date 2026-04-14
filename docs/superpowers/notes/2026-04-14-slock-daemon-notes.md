# Notes: Nekobot daemon protocol wave

## Sources
- `@slock-ai/daemon@latest` packed artifact
- local Nekobot codebase

## Synthesized Findings
- Pending analysis.


## Modeling note
- User clarified that Nekobot channels should remain ingress surfaces.
- daemon-backed conversations should be modeled as threads under channels, not as new channels.

## 2026-04-14 progress update
- Verified current gap: daemon-backed WebUI chat only enqueues `remote_agent` tasks and returns a queue acknowledgement.
- Next implementation target: daemon task execution loop + task status/result payloads + server-side session writeback so thread/session views reflect remote execution outcomes.
- Keep `channel` as ingress only; session/thread remains the writeback surface for daemon task lifecycle.
- Added offline derivation from heartbeat age so stale machines degrade to `offline` in the control plane.
- Added WebUI bootstrap visibility so operators can copy the generated daemon command/token from System page.
- Fresh verification evidence: targeted daemon/webui/cmd Go tests passed after the second polish wave.
- Fresh verification evidence: frontend production build passed after bootstrap UX changes.
- Main branch push confirmed at commit `79dc8f6`.
- Replaced the placeholder daemon executor with a runtime-aware CLI executor. Current first-class paths are `codex` and `claude`.
- Daemon task fetch now filters to installed + healthy runtimes so the server does not assign work to unavailable local runtimes.
- Fresh verification evidence: HTTP-level daemon E2E test passed for register -> heartbeat -> fetch -> execute -> update -> session writeback.
- Added `opencode` as a daemon executor path, and confirmed it can execute successfully when the daemon isolates HOME/XDG_CONFIG_HOME and runs with `--pure`.
- Added `claimed` session feedback so daemon-backed chats now show claim -> running -> completion/failure progression more explicitly.
- Fresh verification evidence: live daemon process E2E passed using a real `nekobot daemon run` process plus fake installed `claude` binary and real HTTP control-plane endpoints.
- Fresh verification evidence: isolated live `opencode --pure run --format json` returned `daemon-ok` on this host when HOME/XDG_CONFIG_HOME were sandboxed.
- Fresh verification evidence: live daemon process E2E also passed for `opencode`, using an isolated runtime environment and real HTTP control-plane endpoints.
- Fresh verification evidence: live daemon process E2E also passed for `codex`, using a fake codex binary with real daemon startup and HTTP control-plane flow.
- Fresh verification evidence: session-level runtime binding now persists through the sessions API and is reused by WebUI runtime selection when no explicit runtime_id is provided.
- Fresh verification evidence: WebUI session runtime bindings now persist through backend API tests and the Sessions page frontend build after exposing the binding UI.
- Fresh verification evidence: Chat page frontend build passed after wiring session runtime bindings directly into the runtime selector UX.
- Fresh verification evidence: Chat page now reuses and updates persisted session runtime bindings, with targeted webui tests and a passing frontend production build.
- Fresh verification evidence: Chat page frontend build passed after adding near-real-time session-detail polling for daemon-backed runtime sessions.
- Fresh verification evidence: thread-style topic metadata now persists through the sessions API and both Sessions/Chat frontend surfaces build successfully with the new topic fields.
- Fresh verification evidence: lightweight Threads API and Threads page build both passed, proving topic+runtime metadata can be surfaced as a first-class thread view.
- Fresh verification evidence: lightweight Threads page and Chat/Session surfaces all build successfully after introducing thread topic/runtime first-class UI.
- Fresh verification evidence: Threads-to-Chat handoff UI shipped cleanly, with Chat rehydrating the selected runtime from thread handoff state after frontend production build.
- Fresh verification evidence: independent `threads.Manager` persistence passed dedicated tests and now backs the session/thread read-write path in webui handlers.
- Fresh verification evidence: daemon chat event SSE plumbing passed backend tests and the Chat frontend build after wiring EventSource-based updates for daemon-backed sessions.
- Fresh verification evidence: SSE event-stream groundwork remains green after re-running threads/webui backend tests and a fresh frontend production build on the current main-based branch.
