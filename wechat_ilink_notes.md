# Notes: WeChat iLink Capability Comparison

## Sources

### Source 1: Current `nekobot` WeChat/iLink code
- Paths:
  - `pkg/wechat/*`
  - `pkg/channels/wechat/*`
- Initial findings:
  - `nekobot` already has a separated SDK layout: `client`, `auth`, `cdn`, `media`, `messaging`, `monitor`, `parse`, `typing`, `voice`.
  - Channel-specific glue is isolated under `pkg/channels/wechat/*`.

### Source 2: `openilink-hub`
- Paths:
  - `/home/czyt/code/openilink-hub/internal/provider/ilink/*`
  - `/home/czyt/code/openilink-hub/internal/api/media_handler.go`
  - `/home/czyt/code/openilink-hub/internal/api/message_handler.go`
- Initial findings:
  - The project appears to include iLink provider integration plus mock server and binding flows.
  - Some capabilities may be hub-side orchestration rather than SDK primitives.

## Synthesized Findings

### Audit Focus
- Identify reusable SDK-layer additions:
  - auth/login/binding primitives
  - media upload/download helpers
  - message send/parse/typing/runtime protocol details
  - mock/integration fixtures that improve testability
- Separate out non-reusable hub logic:
  - app installation flows
  - registry/store/API orchestration
  - hub-specific routing and UI workflows

### Reusable Capabilities Found In `openilink-hub`
- `internal/provider/ilink/bind.go`
  - Clear QR bind session lifecycle with session IDs, refresh-on-expire, and confirmed-credential handoff.
- `internal/provider/ilink/ilink.go`
  - Provider lifecycle semantics worth mirroring in SDK/channel glue: explicit status transitions, sync buffer persistence, session-expired handling, raw response capture for inbound messages, and typed send/media/typing operations.
- `internal/provider/ilink/mockserver/*`
  - High-value SDK-compatible mock HTTP server and in-memory engine for QR login, polling, media upload/download, typing, and session-expire scenarios.
- `internal/provider/ilink/silk_test.go`
  - Stronger SILK/WAV round-trip tests and explicit verification of WeChat-compatible STX-prefixed SILK handling.

### Already Present In `nekobot`
- `pkg/wechat/auth/login.go`
  - QR fetch, single status check, and poll-with-refresh behavior already exists.
- `pkg/wechat/monitor/*`
  - Sync buffer persistence, session guard, and backoff loop already exist in SDK form.
- `pkg/wechat/media/*` and `pkg/wechat/voice/*`
  - Media download/decrypt and voice decoding/transcription paths are already separated.
- `pkg/channels/wechat/control.go`
  - Channel-side runtime control and binding behavior already sit above the SDK.

### High-Confidence Migration Candidates
- Add a SDK-compatible mockserver/test engine into `pkg/wechat/...` for protocol-level tests.
- Enrich client/auth surfaces so QR bind lifecycle is stateful and easier for channel/web consumers to drive.
- Improve media/voice tests and compatibility handling using the stronger SILK round-trip patterns.
- Optionally expose raw poll/response metadata from the SDK when that improves channel debugging or recovery behavior.

### Things To Avoid Copying Directly
- `openilink-hub/internal/api/media_handler.go`
  - Useful as product reference for future media proxy UX, but it is server/product code, not SDK code.
- `openilink-hub/internal/api/message_handler.go`
  - Message listing, retry endpoints, traces, and webhook logs are hub features, not direct SDK migration targets.

### Approved Product Direction
- Keep AI chat as the current WeChat channel entry.
- Build push as a separate product path.
- Both paths share the same scanned iLink auth/binding.
- Ignore historical compatibility constraints and design the clean target shape directly.

### Implementation Outcome
- Added `pkg/wechat/mockserver` with SDK-compatible QR bind, polling, send, typing, config, and update endpoints for local tests.
- Added `pkg/ilinkauth` as the shared file-backed owner of:
  - single binding per backend user
  - QR bind session state
  - sync cursor paths for the current bound bot
- Rewired `pkg/webui/server.go` WeChat binding endpoints to use `pkg/ilinkauth` instead of `pkg/channels/wechat/store.go`.
- Rewired WeChat channel construction to read its current binding from `pkg/ilinkauth`.
- Preserved current chat behavior, but intentionally did not implement the future push product flow in this batch.

### Remaining Gap After This Batch
- The WeChat channel runtime is still effectively single-binding at process level. It now reads from shared auth, but it does not yet provide a true per-backend-user multi-runtime chat architecture.
- Push product/API remains unimplemented by design.
