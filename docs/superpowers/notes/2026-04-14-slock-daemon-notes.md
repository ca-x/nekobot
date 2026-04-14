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
