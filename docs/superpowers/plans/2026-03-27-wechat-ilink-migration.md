# WeChat iLink Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extract shared WeChat iLink binding/auth from the channel, add a reusable mock server for SDK tests, and rewire current WebUI and chat flow to the new shared auth layer.

**Architecture:** Introduce a new `pkg/ilinkauth` package as the storage and bind-lifecycle owner, plus `pkg/wechat/mockserver` for protocol-level regression tests. Keep the current WeChat chat behavior intact while redirecting auth ownership away from `pkg/channels/wechat/store.go`.

**Tech Stack:** Go, Echo, file-backed runtime storage, existing WeChat SDK packages, Go test.

---

### Task 1: Persisted planning baseline

**Files:**
- Modify: `wechat_ilink_task_plan.md`
- Create: `docs/superpowers/specs/2026-03-27-wechat-ilink-design.md`
- Create: `docs/superpowers/plans/2026-03-27-wechat-ilink-migration.md`

- [x] **Step 1: Write the design/spec file.**
- [x] **Step 2: Write the implementation plan file.**
- [x] **Step 3: Update the task plan status.**

### Task 2: WeChat mock server TDD slice

**Files:**
- Create: `pkg/wechat/mockserver/mockserver_test.go`
- Create: `pkg/wechat/mockserver/server.go`
- Create: `pkg/wechat/mockserver/engine.go`

- [ ] **Step 1: Write failing tests for QR fetch, QR poll, send message, get updates, get config, and send typing.**
- [ ] **Step 2: Run `go test ./pkg/wechat/mockserver -count=1` and confirm failure.**
- [ ] **Step 3: Implement the minimal in-memory engine and HTTP server to satisfy the tests.**
- [ ] **Step 4: Re-run `go test ./pkg/wechat/mockserver -count=1` and confirm pass.**

### Task 3: Shared auth service TDD slice

**Files:**
- Create: `pkg/ilinkauth/store_test.go`
- Create: `pkg/ilinkauth/store.go`
- Create: `pkg/ilinkauth/service_test.go`
- Create: `pkg/ilinkauth/service.go`

- [ ] **Step 1: Write failing tests for single-binding persistence, bind-session lifecycle, and sync-state path lookup.**
- [ ] **Step 2: Run `go test ./pkg/ilinkauth -count=1` and confirm failure.**
- [ ] **Step 3: Implement the minimal file-backed store and service.**
- [ ] **Step 4: Re-run `go test ./pkg/ilinkauth -count=1` and confirm pass.**

### Task 4: WebUI binding rewrite

**Files:**
- Modify: `pkg/webui/server.go`
- Modify: `pkg/webui/server_wechat_test.go`

- [ ] **Step 1: Write or update failing tests for current-user binding status and binding lifecycle through `pkg/ilinkauth`.**
- [ ] **Step 2: Run focused WebUI tests and confirm failure.**
- [ ] **Step 3: Replace direct `channelwechat.CredentialStore` usage with current-user `ilinkauth` operations.**
- [ ] **Step 4: Re-run focused WebUI tests and confirm pass.**

### Task 5: WeChat channel auth-source rewrite

**Files:**
- Modify: `pkg/channels/wechat/channel.go`
- Modify: `pkg/channels/registry.go`
- Modify: `pkg/channels/wechat/store_test.go` or replace with `pkg/ilinkauth` coverage

- [ ] **Step 1: Write failing tests for channel startup reading shared binding state.**
- [ ] **Step 2: Run focused channel tests and confirm failure.**
- [ ] **Step 3: Rewire the channel constructor and bot setup to consume shared auth state.**
- [ ] **Step 4: Re-run focused channel tests and confirm pass.**

### Task 6: Verification

**Files:**
- Modify: `wechat_ilink_task_plan.md`

- [ ] **Step 1: Run focused package tests for `pkg/wechat/mockserver`, `pkg/ilinkauth`, `pkg/webui`, and `pkg/channels/wechat`.**
- [ ] **Step 2: Run `go test -count=1 ./...`.**
- [ ] **Step 3: Update task-plan status and record any residual risks.**
