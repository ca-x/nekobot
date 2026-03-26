# Migration Tasks: goclaw and gua to nekobot

## Scope
Compare current `nekobot` against local `goclaw` and `gua`, remove already-migrated items from the backlog, and execute the highest-value remaining migration slice first.

## Task List
- [ ] Refresh migration inventory by treating `docs/GOCLAW_FEATURES.md` as historical, not authoritative.
- [ ] Document already-migrated `goclaw` capabilities to avoid duplicate work.
- [x] Add WeChat ACP permission routing so `session/request_permission` is exposed to users instead of auto-approved.
- [x] Extend WeChat interaction handling to support generic runtime approvals and selections, not just skill-install confirmation.
- [ ] Add ACP interaction/event buffering so runtime status and logs are inspectable.
- [ ] Evaluate follow-up `gua` parity tasks: `/share`, multi-account WeChat account management, session management UX.

## Execution Log
- Research complete enough to start Phase 1 execution on the highest-priority `gua` gap.
- Selected first migration slice: WeChat-side ACP permission routing and generic pending interaction support.
- Implemented ACP pending prompt flow for WeChat runtimes:
  - ACP chat requests can now return a pending permission prompt instead of auto-allowing.
  - WeChat `/yes` and `/no` now resolve pending runtime approvals before messages are forwarded to the bound runtime.
  - Added tests covering pending prompt surfacing, approval continuation, and Channel-level delegation.
- Verified with `go test -count=1 ./pkg/channels/wechat`.
