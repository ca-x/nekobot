# Multi-Agent Runtime And Channel Account Implementation Plan

> **For agentic workers:** use disciplined TDD and keep write ownership disjoint when parallelizing.

**Goal:** Deliver the multi-round architecture shift from a single default agent to first-class `AgentRuntime`, `ChannelAccount`, and `AccountBinding` primitives, starting with in-process multi-runtime support and WeChat account-awareness.

**Architecture:** Build the new runtime domain first, then move one concrete channel (`WeChat/iLink`) onto it, then relocate harness policy to the agent layer, and finally reorganize the WebUI around `Agents / Channel Accounts / Bindings`.

**Tech Stack:** Go, Echo, Ent/runtime storage, React, TypeScript, TanStack Query

---

### Round 1: Runtime Domain Foundation

**Files:**
- Add: `pkg/runtimeagents/*` or equivalent new package
- Add: `pkg/channelaccounts/*` or equivalent new package
- Add: `pkg/accountbindings/*` or equivalent new package
- Modify: `pkg/agent/*`
- Modify: `pkg/channels/*`
- Modify: `pkg/webui/server.go`
- Test: new package tests + targeted server tests

- [ ] Define `AgentRuntime`, `ChannelAccount`, and `AccountBinding` domain models.
- [ ] Add storage/repository layer with runtime-safe CRUD.
- [ ] Add runtime manager capable of resolving and serving multiple agent runtimes in-process.
- [ ] Add initial APIs for listing/creating/updating runtimes, accounts, and bindings.
- [ ] Verify targeted package/server tests.

### Round 2: WeChat Channel Account Migration

**Files:**
- Modify: `pkg/channels/wechat/*`
- Modify: `pkg/ilinkauth/*` as needed
- Modify: `pkg/webui/server.go`
- Modify: `pkg/webui/frontend/src/pages/ChannelsPage.tsx` or replacement account pages
- Test: `pkg/channels/wechat/*_test.go`, `pkg/webui/server_wechat_test.go`

- [ ] Replace single-binding WeChat runtime assumptions with channel-account aware runtime selection.
- [ ] Reintroduce truthful multi-account management semantics.
- [ ] Route inbound/outbound traffic via `ChannelAccount` and binding lookup.
- [ ] Verify targeted WeChat/server tests and UI build.

### Round 3: Harness Downshift To Agent Runtime

**Files:**
- Modify: `pkg/agent/*`
- Modify: `pkg/watch/*`
- Modify: `pkg/session/*`
- Modify: `pkg/audit/*`
- Modify: `pkg/webui/*`
- Test: targeted harness and chat/runtime tests

- [ ] Introduce runtime-scoped harness policy resolution.
- [ ] Keep global harness config as defaults only.
- [ ] Make `undo/watch/audit/learnings/file-mentions` runtime-aware.
- [ ] Verify targeted tests and full regression.

### Round 4: WebUI Reorganization

**Files:**
- Add/Modify: `pkg/webui/frontend/src/pages/*`
- Add/Modify: `pkg/webui/frontend/src/hooks/*`
- Add/Modify: `pkg/webui/frontend/public/i18n/*`

- [ ] Add `Agents` management page.
- [ ] Add `Channel Accounts` management page.
- [ ] Add `Bindings` management page.
- [ ] Adapt Chat/Harness surfaces to show runtime and agent-source semantics.
- [ ] Verify frontend build and browser smoke tests.

### Round 5: Multi-Agent Collaboration Slots

**Files:**
- Modify: `pkg/agent/*`
- Modify: `pkg/subagent/*` or new collaboration package
- Modify: `pkg/webui/*`
- Test: new collaboration/runtime routing tests

- [ ] Add agent-to-agent collaboration declarations.
- [ ] Add public/internal reply policy for multi-agent bindings.
- [ ] Implement explicit-agent reply suppression.
- [ ] Verify targeted and full regressions.

### Verification And Shipping

**Files:**
- Modify: `task_plan.md`
- Modify: `notes.md`

- [ ] Update plan and notes after each completed round.
- [ ] Keep each round independently testable and shippable.
- [ ] Use conventional commits per round.
- [ ] Run `go test -count=1 ./...` and `npm --prefix pkg/webui/frontend run build` before claiming completion.
