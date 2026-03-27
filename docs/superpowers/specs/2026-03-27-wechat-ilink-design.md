# WeChat iLink Shared Auth Design

**Date:** 2026-03-27

## Goal
Refactor the current WeChat iLink integration so QR-scan binding/auth is a shared capability owned outside the channel, allowing the existing WeChat chat entry and a future push-delivery path to reuse the same binding state.

## Product Decisions
- Keep the existing WeChat chat path as the user's AI conversation entry.
- Model push delivery as a separate product path, not as a chat-channel side effect.
- A backend user can bind only one iLink recipient.
- Push and chat ultimately target the same bound iLink user.
- No historical-data compatibility is required.

## Current Problems
- `pkg/channels/wechat/store.go` mixes channel concerns with iLink auth persistence.
- `pkg/webui/server.go` binding endpoints directly manipulate channel-local storage.
- The current storage shape supports multiple accounts plus active-account switching, which is outside the approved product model.
- The SDK lacks a project-local mock iLink server, making protocol regressions hard to test.

## Chosen Architecture

### 1. Shared auth service.
Add `pkg/ilinkauth` as the owner of:
- current-user binding state
- scanned QR session state
- persisted credentials
- sync-state file paths for channel runtime

This package is keyed by backend user identity and exposes a small service/store API instead of channel-specific operations.

### 2. SDK-local mock server.
Add `pkg/wechat/mockserver` adapted from the proven `openilink-hub` test harness. It should emulate the endpoints exercised by the existing SDK:
- `GET /ilink/bot/get_bot_qrcode`
- `GET /ilink/bot/get_qrcode_status`
- `POST /ilink/bot/getupdates`
- `POST /ilink/bot/sendmessage`
- `POST /ilink/bot/getconfig`
- `POST /ilink/bot/sendtyping`
- `POST /ilink/bot/getuploadurl` when needed by media tests

The mock server must also provide control hooks for tests to simulate scan, confirm, expiry, and sent-message inspection.

### 3. WebUI rewire.
The WebUI WeChat binding endpoints should resolve the current authenticated backend user and operate through `pkg/ilinkauth`, not through `pkg/channels/wechat/store.go`.

### 4. Minimal channel integration.
The current WeChat channel should stop owning auth persistence details. It should consume shared binding data and sync-state paths from `pkg/ilinkauth` while keeping existing chat semantics.

## Scope For This Batch
- Add `pkg/wechat/mockserver`.
- Add `pkg/ilinkauth`.
- Rewire current WeChat binding endpoints to the shared auth layer.
- Rewire the existing WeChat chat path to read from the shared auth layer.

## Explicitly Out Of Scope
- Full push API/product flow.
- Hub/server/app/store logic from `openilink-hub`.
- Backward-compatible migration for legacy binding data.
- Multi-recipient or multi-account WeChat binding UX.

## Testing Strategy
- TDD for new mock server behavior.
- TDD for shared auth persistence and bind lifecycle.
- Update WebUI binding tests to assert per-user single binding behavior.
- Run focused package tests first, then `go test -count=1 ./...`.

## Notes
- The brainstorming review-loop subagent step was skipped because this session is not authorized for delegation unless the user explicitly asks for sub-agents.
