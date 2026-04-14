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
