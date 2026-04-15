# Runtime Transport Abstraction Implementation Plan

## Overview

Extract the current tmux-specific runtime session lifecycle into a transport abstraction while preserving all existing tmux behavior. This refactor is the prerequisite for adding zellij later without destabilizing Tool Sessions, external agent launches, or WebUI runtime restore flows.

## Architecture Decisions
- Introduce a small runtime transport interface around the existing behaviors: wrap start, restore/reattach, existence checks, and terminate.
- Keep tmux as the only concrete transport in phase 1; behavior and metadata stay backward-compatible.
- Add generic metadata (`runtime_session`) while preserving current tmux-specific metadata (`tmux_session`) for compatibility.
- Prefer concentrated transport code in one package/file rather than duplicating command construction across `pkg/tools`, `pkg/webui`, and `pkg/externalagent`.

## Task List

### Phase 1: Lock Current Behavior
- [ ] Task 1: Document the tmux seam and desired abstraction boundary
- [ ] Task 2: Add/adjust regression tests so current tmux launch + restore metadata expectations are explicit, including the new generic `runtime_session` compatibility field

### Checkpoint: Behavior Locked
- [ ] Targeted Go tests for toolsession and webui pass before refactor

### Phase 2: Introduce Transport Abstraction
- [ ] Task 3: Add runtime transport interface + tmux implementation in shared code
- [ ] Task 4: Refactor `pkg/tools/toolsession.go` to depend on the transport abstraction instead of local tmux helpers
- [ ] Task 5: Refactor `pkg/externalagent/starter.go` and call sites to depend on the shared transport abstraction
- [ ] Task 6: Refactor `pkg/webui/server.go` runtime launch/restore/kill logic to use the shared transport abstraction

### Checkpoint: Refactor Complete
- [ ] Targeted Go tests pass
- [ ] No behavior regression in launch/restore metadata fields

### Phase 3: Hardening / Follow-up Prep
- [ ] Task 7: Update docs/notes to reflect transport abstraction and tmux-as-current-backend
- [ ] Task 8: Leave clear extension seam for future zellij backend without enabling it yet

### Checkpoint: Ready For Future zellij Work
- [ ] Focused verification passes
- [ ] Diff remains small and behavior-preserving

## Risks and Mitigations
| Risk | Impact | Mitigation |
|------|--------|------------|
| WebUI restore flow regresses | High | Preserve exact tmux attach semantics and cover with existing restore test |
| Metadata consumers break | High | Keep `tmux_session`, add `runtime_session`, do not remove old fields yet |
| Refactor spreads across too many files | Medium | Centralize transport logic and avoid changing unrelated flows |
| Existing Docker/QMD changes get mixed into this work | Medium | Stage/commit only transport-related files separately |

## Verification Plan
- `go test ./pkg/tools ./pkg/webui ./pkg/externalagent ./buildtest`
- If package-level tests are too broad/noisy, run focused tests for transport-related cases first:
  - `go test ./pkg/tools -run ToolSession`
  - `go test ./pkg/webui -run ToolSession`
- Re-read changed metadata assertions to confirm backward compatibility:
  - `runtime_transport`
  - `runtime_session`
  - `tmux_session`
  - `launch_cmd`
