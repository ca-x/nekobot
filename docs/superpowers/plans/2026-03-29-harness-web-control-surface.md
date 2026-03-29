# Harness Web Control Surface Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make harness-related runtime config truthful in WebUI and add Web-first watch, safe multi-step undo, and `@file` feedback workflows.

**Architecture:** First fix runtime config persistence for harness sections so the UI reflects real state. Then add small, explicit backend APIs for watch status/control and chat undo, plus lightweight chat metadata events for file-mention feedback. Finally wire the frontend to these APIs without changing the overall WebSocket chat model.

**Tech Stack:** Go, Echo, React, TypeScript, TanStack Query, existing WebSocket chat API, JSON config persistence

---

### Task 1: Fix Harness Runtime Config Persistence

**Files:**
- Modify: `pkg/config/db_store.go`
- Modify: `pkg/webui/server.go`
- Test: `pkg/webui/server_config_test.go`

- [ ] **Step 1: Write failing tests for harness config sections in `/api/config`**
- [ ] **Step 2: Run targeted config tests and confirm failure**
- [ ] **Step 3: Extend runtime config section registry and WebUI config handlers**
- [ ] **Step 4: Re-run targeted config tests and confirm pass**

### Task 2: Add Watch Status and Control API

**Files:**
- Modify: `pkg/watch/watcher.go`
- Modify: `pkg/webui/server.go`
- Test: `pkg/watch/watcher_test.go`
- Test: `pkg/webui/server_status_test.go` or new watch-focused server test file

- [ ] **Step 1: Write failing tests for watcher status snapshot and watch API response**
- [ ] **Step 2: Run targeted watch/server tests and confirm failure**
- [ ] **Step 3: Implement watcher runtime status fields and Web API**
- [ ] **Step 4: Re-run targeted tests and confirm pass**

### Task 3: Add Safe Multi-Step Undo API For Web Chat

**Files:**
- Modify: `pkg/session/manager.go`
- Modify: `pkg/session/snapshot.go` if helper needed
- Modify: `pkg/webui/server.go`
- Test: `pkg/webui/server_chat_test.go`

- [ ] **Step 1: Write failing tests for multi-step chat undo and session state rewrite**
- [ ] **Step 2: Run targeted chat tests and confirm failure**
- [ ] **Step 3: Implement session message replacement helper and undo API**
- [ ] **Step 4: Re-run targeted tests and confirm pass**

### Task 4: Add `@file` Feedback To Web Chat

**Files:**
- Modify: `pkg/agent/context.go` or adjacent helper file
- Modify: `pkg/webui/server.go`
- Modify: `pkg/webui/frontend/src/hooks/useChat.ts`
- Modify: `pkg/webui/frontend/src/pages/ChatPage.tsx`
- Modify: `pkg/webui/frontend/public/i18n/en.json`
- Modify: `pkg/webui/frontend/public/i18n/zh-CN.json`
- Modify: `pkg/webui/frontend/public/i18n/ja.json`
- Test: `pkg/webui/server_chat_test.go`

- [ ] **Step 1: Write failing tests for chat metadata feedback when file mentions are expanded**
- [ ] **Step 2: Run targeted chat tests and confirm failure**
- [ ] **Step 3: Implement preprocessing preview plumbing and WebSocket metadata event**
- [ ] **Step 4: Update frontend hook/page to render feedback**
- [ ] **Step 5: Re-run targeted tests and frontend build**

### Task 5: Wire Web UX For Watch and Undo

**Files:**
- Modify: `pkg/webui/frontend/src/hooks/useConfig.ts`
- Modify: `pkg/webui/frontend/src/pages/ChatPage.tsx`
- Modify: `pkg/webui/frontend/public/i18n/en.json`
- Modify: `pkg/webui/frontend/public/i18n/zh-CN.json`
- Modify: `pkg/webui/frontend/public/i18n/ja.json`

- [ ] **Step 1: Add frontend hooks for watch status/control and chat undo**
- [ ] **Step 2: Render status pill, undo button, and metadata panel in Chat page**
- [ ] **Step 3: Run frontend build and confirm type safety**

### Task 6: Verify and Ship

**Files:**
- Modify: `task_plan.md`
- Modify: `notes.md`

- [ ] **Step 1: Run `go test -count=1 ./pkg/watch ./pkg/webui ./pkg/session`**
- [ ] **Step 2: Run `go test -count=1 ./cmd/nekobot/...`**
- [ ] **Step 3: Run `npm --prefix pkg/webui/frontend run build`**
- [ ] **Step 4: Update plan and notes with findings and final state**
- [ ] **Step 5: Commit with conventional message and push**
