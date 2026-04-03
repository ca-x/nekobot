# Conversation/Thread Binding Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Strengthen `pkg/conversationbindings` into a genuinely reusable conversation/thread binding layer, then prove the new contract through the existing WeChat runtime consumer without broadening into gateway or external-agent integration in the same slice.

**Architecture:** Keep the current persistence boundary on top of `toolsessions.Manager` for now, but upgrade the binding service API from a WeChat-shaped wrapper into a generic record/query layer with deterministic behavior, richer scope metadata, and clearer rebinding semantics. Use `pkg/channels/wechat/runtime.go` as the first real consumer to prove the generic contract, and explicitly defer standalone storage and gateway adoption to later slices.

**Tech Stack:** Go, Ent/SQLite, existing `toolsessions`, `conversationbindings`, and `channels/wechat` packages

---

### Phase 0: Documentation Discovery

**Sources read before planning**
- `pkg/conversationbindings/service.go`
- `pkg/conversationbindings/service_test.go`
- `pkg/toolsessions/manager.go`
- `pkg/channels/wechat/runtime.go`
- `pkg/channels/wechat/runtime_test.go`
- `pkg/channels/wechat/control.go`
- `pkg/gateway/server.go`
- `task_plan.md`
- `notes.md`

**Allowed existing APIs and seams**
- `conversationbindings.New(...)`
- `(*conversationbindings.Service).Bind(...)`
- `(*conversationbindings.Service).BindWithOptions(...)`
- `(*conversationbindings.Service).Resolve(...)`
- `(*conversationbindings.Service).Clear(...)`
- `(*conversationbindings.Service).List(...)`
- `(*conversationbindings.Service).ListBindings(...)`
- `(*conversationbindings.Service).GetBinding(...)`
- `(*conversationbindings.Service).GetBindingsBySession(...)`
- `(*conversationbindings.Service).CleanupExpired(...)`
- `(*toolsessions.Manager).FindSessionByConversation(...)`
- `(*toolsessions.Manager).BindSessionConversation(...)`
- `(*toolsessions.Manager).ClearConversationBinding(...)`
- `wechat.NewRuntimeBindingService(...)`

**Observed constraints**
- `conversationbindings.Service` already supports multiple bindings per session and expiry cleanup, so the next slice should not re-solve those same problems.
- The current generic service is still scoped at construction time to one `source/channel/prefix`, which is good enough for WeChat but thin for future consumers.
- `pkg/gateway/server.go` does not yet consume the binding layer. Planning should prepare for that path without mixing gateway implementation into this slice.
- Current persistence still lives in `tool_sessions` metadata plus `conversation_key`; avoid schema churn until the service contract is clearer.

**Anti-pattern guards**
- Do not add a new Ent schema in this slice.
- Do not mix gateway control-plane hardening into this plan.
- Do not bypass `toolsessions.Manager` with direct Ent queries from consumers.
- Do not break existing WeChat runtime behavior while generalizing the service.

---

### File Structure

**Core binding layer**
- Modify: `pkg/conversationbindings/service.go`
  - Extend the generic binding/query contract while keeping persistence on top of tool sessions.
- Modify: `pkg/conversationbindings/service_test.go`
  - Lock the new generic semantics with RED/GREEN regression coverage.

**First consumer proof**
- Modify: `pkg/channels/wechat/runtime.go`
  - Keep WeChat runtime on top of the generic service, but switch to the clearer contract added in this plan.
- Modify: `pkg/channels/wechat/runtime_test.go`
  - Prove WeChat still resolves, lists, and clears bindings correctly after the service changes.
- Modify: `pkg/channels/wechat/control_test.go`
  - Add or adjust consumer-level regressions only if the new contract changes observable control behavior.

**Planning and follow-through**
- Modify: `task_plan.md`
  - Record the approved next slice and execution order.
- Modify: `notes.md`
  - Capture the scope decision, deferred items, and verification intent.

---

### Task 1: Lock the Generic Binding Contract With RED Tests

**Files:**
- Modify: `pkg/conversationbindings/service_test.go`

- [ ] **Step 1: Add failing tests for the next generic invariants**
- [x] **Step 1: Add failing tests for the next generic invariants**

Cover at least:
- rebinding one conversation from session A to session B clears the old target
- `ListBindings` is deterministic across multiple sessions and multiple conversations
- `GetBindingsBySession` only returns records inside the service scope
- clearing one conversation on a multi-bound session preserves the other records and primary conversation key

- [ ] **Step 2: Run the targeted binding tests to verify RED**
- [x] **Step 2: Run the targeted binding tests to verify RED**

Run: `go test -count=1 ./pkg/conversationbindings -run 'TestService'`
Expected: FAIL because the stricter contract is not fully implemented yet.

- [ ] **Step 3: Commit the RED test seam**
- [x] **Step 3: Commit the RED test seam**

```bash
git add pkg/conversationbindings/service_test.go
git commit -m "test(bindings): lock generic binding contract"
```

### Task 2: Strengthen `conversationbindings.Service`

**Files:**
- Modify: `pkg/conversationbindings/service.go`
- Modify: `pkg/conversationbindings/service_test.go`

- [ ] **Step 1: Implement the minimal generic-service changes**
- [x] **Step 1: Implement the minimal generic-service changes**

Implement only what the RED tests require, such as:
- deterministic record ordering
- explicit rebinding semantics when a conversation moves across sessions
- stable primary-conversation promotion rules
- any small helper types needed to express scope and record behavior clearly

- [ ] **Step 2: Keep persistence on top of `toolsessions.Manager`**
- [x] **Step 2: Keep persistence on top of `toolsessions.Manager`**

Do not add new storage. Reuse:
- `BindSessionConversation`
- `ClearConversationBinding`
- session metadata under `conversation_binding`

- [ ] **Step 3: Run the binding package tests to verify GREEN**
- [x] **Step 3: Run the binding package tests to verify GREEN**

Run: `go test -count=1 ./pkg/conversationbindings`
Expected: PASS

- [ ] **Step 4: Commit the generic binding layer slice**
- [x] **Step 4: Commit the generic binding layer slice**

```bash
git add pkg/conversationbindings/service.go pkg/conversationbindings/service_test.go
git commit -m "feat(bindings): strengthen generic conversation binding service"
```

### Task 3: Re-prove the WeChat Runtime Consumer

**Files:**
- Modify: `pkg/channels/wechat/runtime.go`
- Modify: `pkg/channels/wechat/runtime_test.go`
- Modify: `pkg/channels/wechat/control_test.go`

- [ ] **Step 1: Add failing consumer tests if the generic contract changed**
- [x] **Step 1: Add failing consumer tests if the generic contract changed**

Cover:
- WeChat runtime still resolves one chat to one target session
- one runtime session can still serve multiple chats
- list/read APIs preserve chat identity after the generic-service changes
- control-layer flows that depend on binding lookup still behave the same

- [ ] **Step 2: Run the focused WeChat tests to verify RED**
- [x] **Step 2: Run the focused WeChat tests to verify RED**

Run: `go test -count=1 ./pkg/channels/wechat -run 'Test(RuntimeBindingService|HandleWechatBinding|WechatControl)'`
Expected: FAIL only if the contract changes require consumer updates.

- [ ] **Step 3: Update the WeChat runtime wrapper minimally**
- [x] **Step 3: Update the WeChat runtime wrapper minimally**

Keep the wrapper thin:
- no new WeChat-specific storage
- no presenter/protocol work in this task
- no cross-channel abstraction inside the consumer

- [ ] **Step 4: Run the focused WeChat tests to verify GREEN**
- [x] **Step 4: Run the focused WeChat tests to verify GREEN**

Run: `go test -count=1 ./pkg/channels/wechat -run 'Test(RuntimeBindingService|HandleWechatBinding|WechatControl)'`
Expected: PASS

- [ ] **Step 5: Commit the consumer-proof slice**
- [x] **Step 5: Commit the consumer-proof slice**

```bash
git add pkg/channels/wechat/runtime.go pkg/channels/wechat/runtime_test.go pkg/channels/wechat/control_test.go
git commit -m "feat(wechat): align runtime bindings with generic binding service"
```

### Task 4: Run Slice Verification and Update Planning State

**Files:**
- Modify: `task_plan.md`
- Modify: `notes.md`
- Modify: `progress.md`

- [ ] **Step 1: Run the required focused verification**
- [x] **Step 1: Run the required focused verification**

Run:
- `go test -count=1 ./pkg/conversationbindings`
- `go test -count=1 ./pkg/channels/wechat`

Expected: PASS

- [ ] **Step 2: Run expanded regression around the shared persistence seam**
- [x] **Step 2: Run expanded regression around the shared persistence seam**

Run:
- `go test -count=1 ./pkg/toolsessions ./pkg/conversationbindings ./pkg/channels/wechat`

Expected: PASS

- [ ] **Step 3: Update planning artifacts**
- [x] **Step 3: Update planning artifacts**

Record:
- what became reusable
- what remains deferred
- whether the next slice should be `gateway` adoption or `Slack interactive callback` closure

- [ ] **Step 4: Commit the planning/verification closeout**

```bash
git add task_plan.md notes.md progress.md
git commit -m "docs(plan): record conversation binding slice progress"
```

---

### Deferred Work After This Plan

- Standalone binding storage independent from `tool_sessions`.
- Gateway consumer adoption and connection-governance integration.
- External agent runtime consumer adoption.
- WeChat presenter or interaction protocol expansion.
- Cross-account binding normalization if a consumer proves the current `source/channel/prefix` scope is too narrow.
