# Migration Tasks: goclaw and gua to nekobot

## Scope
Compare current `nekobot` against local `goclaw` and `gua`, remove already-migrated items from the backlog, and execute the highest-value remaining migration slice first.

## Task List
- [x] Refresh migration inventory by treating `docs/GOCLAW_FEATURES.md` as historical, not authoritative.
- [x] Document already-migrated `goclaw` capabilities to avoid duplicate work.
- [x] Add WeChat ACP permission routing so `session/request_permission` is exposed to users instead of auto-approved.
- [x] Extend WeChat interaction handling to support generic runtime approvals and selections, not just skill-install confirmation.
- [x] Add ACP interaction/event buffering so runtime status, `/logs`, and incremental runtime reads are inspectable.
- [x] Add ACP multi-option permission selection so `/select N` resolves to concrete ACP permission options.
- [ ] Evaluate follow-up `gua` parity tasks: `/share`, multi-account WeChat account management, session management UX.

## Execution Log
- Research complete enough to start Phase 1 execution on the highest-priority `gua` gap.
- Selected first migration slice: WeChat-side ACP permission routing and generic pending interaction support.
- Implemented ACP pending prompt flow for WeChat runtimes:
  - ACP chat requests can now return a pending permission prompt instead of auto-allowing.
  - WeChat `/yes` and `/no` now resolve pending runtime approvals before messages are forwarded to the bound runtime.
  - Added tests covering pending prompt surfacing, approval continuation, and Channel-level delegation.
- Verified with `go test -count=1 ./pkg/channels/wechat`.
- Re-checked local `goclaw` and `gua` code against `nekobot`:
  - `goclaw` feature backlog in `docs/GOCLAW_FEATURES.md` is mostly stale because provider failover, skill snapshots/versioning, QMD, and workspace bootstrap are already migrated.
  - The meaningful remaining gaps are now concentrated in `gua`-style WeChat/external-runtime parity.
- Selected next migration slice: ACP event buffering for incremental runtime reads so bound ACP runtimes behave more like PTY runtimes during chat follow-up.
- Implemented ACP incremental output reading in `pkg/channels/wechat/control.go`:
  - `ReadRuntimeOutput` now reads ACP event logs as a cursor-based output stream.
  - `GetConversationRuntime` now computes ACP cursors from rendered event output instead of PTY chunks.
  - Added regression coverage for ACP incremental reads after pending permission prompts are resolved.
- Verified with `go test -count=1 ./pkg/channels/wechat`.
- Re-checked the ACP SDK and local `gua` implementation:
  - ACP itself exposes `session/request_permission`, not a separate `session/request_elicitation`.
  - The right next migration slice is therefore richer permission option handling, especially `/select N` for multi-option prompts.
- Implemented ACP multi-option permission selection in `pkg/channels/wechat/control.go`:
  - permission prompts now render numbered options with labels derived from ACP option kinds.
  - `/select N` now maps to the corresponding ACP `optionId`.
  - `/yes` and `/no` still work when ACP provides canonical allow/deny options.
  - Added regression tests for prompt rendering, option mapping, and runtime-side `/select`.
- Verified with `go test -count=1 ./pkg/channels/wechat`.
