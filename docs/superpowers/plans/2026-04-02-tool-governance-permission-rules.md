# Tool Governance Permission Rules Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first persistent permission-rules layer so every tool call is evaluated through explicit `allow | deny | ask` rules before falling back to the existing approval-mode flow.

**Architecture:** Add a dedicated `permissionrules` package backed by a new Ent schema, keep rule CRUD and evaluation separate from `approval.Manager`, and integrate the evaluator at the single `Agent.executeToolCall` entry point. Expose minimal WebUI CRUD APIs after the evaluator and agent integration are proven by tests.

**Tech Stack:** Go, Ent/SQLite, Echo, existing `approval`, `agent`, and `webui` packages

---

### Task 1: Add Persistent Permission Rule Storage

**Files:**
- Create: `pkg/storage/ent/schema/permissionrule.go`
- Create: `pkg/permissionrules/manager.go`
- Create: `pkg/permissionrules/manager_test.go`
- Modify: `pkg/storage/ent/*` (generated Ent files)

- [ ] **Step 1: Write the failing storage tests**

Add tests in `pkg/permissionrules/manager_test.go` covering:
- create rule
- list rules in stable order
- update rule
- delete rule
- reject invalid `tool_name`
- reject invalid `action`

- [ ] **Step 2: Run the storage tests to verify they fail**

Run: `go test -count=1 ./pkg/permissionrules`
Expected: FAIL because package/schema/manager do not exist yet.

- [ ] **Step 3: Add the Ent schema and minimal manager**

Implement:
- `permissionrule` schema with:
  - `id`
  - `enabled`
  - `priority`
  - `tool_name`
  - `session_id`
  - `runtime_id`
  - `action`
  - `description`
  - `created_at`
  - `updated_at`
- `pkg/permissionrules.Manager` CRUD methods
- input normalization and validation

- [ ] **Step 4: Generate Ent code**

Run: `go run entgo.io/ent/cmd/ent generate ./pkg/storage/ent/schema`
Expected: Ent client/types compile with the new schema.

- [ ] **Step 5: Run the storage tests to verify they pass**

Run: `go test -count=1 ./pkg/permissionrules`
Expected: PASS

- [ ] **Step 6: Commit the storage slice**

```bash
git add pkg/storage/ent/schema/permissionrule.go pkg/storage/ent pkg/permissionrules/manager.go pkg/permissionrules/manager_test.go
git commit -m "feat(permission): add persistent permission rule storage"
```

### Task 2: Add Rule Evaluator and Explanation Output

**Files:**
- Create: `pkg/permissionrules/evaluator.go`
- Create: `pkg/permissionrules/evaluator_test.go`
- Modify: `pkg/permissionrules/manager.go`

- [ ] **Step 1: Write the failing evaluator tests**

Add tests in `pkg/permissionrules/evaluator_test.go` covering:
- global rule match
- session rule outranks global
- runtime rule outranks global
- higher priority outranks lower priority
- disabled rule ignored
- no rule match returns fallback-required result
- explanation includes `source`, `matched_rule_id`, and scope

- [ ] **Step 2: Run the evaluator tests to verify they fail**

Run: `go test -count=1 ./pkg/permissionrules -run 'TestEvaluator'`
Expected: FAIL because evaluator types and logic do not exist yet.

- [ ] **Step 3: Implement the evaluator**

Implement:
- `DecisionAction` type: `allow | deny | ask`
- evaluation input with:
  - `tool_name`
  - `session_id`
  - `runtime_id`
- first-match resolution with stable ordering:
  - priority desc
  - scope specificity desc
  - updated_at desc
  - id
- explanation payload

- [ ] **Step 4: Run the evaluator tests to verify they pass**

Run: `go test -count=1 ./pkg/permissionrules -run 'TestEvaluator'`
Expected: PASS

- [ ] **Step 5: Run the full package tests**

Run: `go test -count=1 ./pkg/permissionrules`
Expected: PASS

- [ ] **Step 6: Commit the evaluator slice**

```bash
git add pkg/permissionrules/manager.go pkg/permissionrules/evaluator.go pkg/permissionrules/manager_test.go pkg/permissionrules/evaluator_test.go
git commit -m "feat(permission): add permission rule evaluator"
```

### Task 3: Integrate Evaluator Into Agent Tool Execution

**Files:**
- Modify: `pkg/agent/agent.go`
- Modify: `pkg/agent/fx.go`
- Modify: `pkg/agent/agent_test.go`
- Modify: `pkg/webui/server_status_test.go` only if runtime/session state assertions need adjustment

- [ ] **Step 1: Write the failing agent integration tests**

Add tests covering:
- `allow` rule executes tool without pending approval
- `deny` rule blocks tool execution
- `ask` rule creates pending approval
- no rule match still follows existing approval mode

- [ ] **Step 2: Run the agent integration tests to verify they fail**

Run: `go test -count=1 ./pkg/agent -run 'TestAgent.*PermissionRule'`
Expected: FAIL because `Agent` does not consult permission rules yet.

- [ ] **Step 3: Implement the agent integration**

Implement:
- inject `permissionrules.Manager` or evaluator dependency into `Agent`
- evaluate rules at the start of `executeToolCall`
- preserve existing `approval.Manager` fallback behavior
- keep task-store session pending-action updates coherent

- [ ] **Step 4: Run the agent integration tests to verify they pass**

Run: `go test -count=1 ./pkg/agent -run 'TestAgent.*PermissionRule'`
Expected: PASS

- [ ] **Step 5: Run focused regression tests**

Run: `go test -count=1 ./pkg/approval ./pkg/agent ./pkg/webui`
Expected: PASS

- [ ] **Step 6: Commit the agent integration slice**

```bash
git add pkg/agent/agent.go pkg/agent/fx.go pkg/agent/agent_test.go
git commit -m "feat(agent): evaluate permission rules before approval fallback"
```

### Task 4: Add WebUI CRUD API For Permission Rules

**Files:**
- Modify: `pkg/webui/server.go`
- Create: `pkg/webui/server_permission_rules_test.go`

- [ ] **Step 1: Write the failing API tests**

Add tests covering:
- list permission rules
- create permission rule
- update permission rule
- delete permission rule
- invalid action rejected
- empty tool name rejected

- [ ] **Step 2: Run the API tests to verify they fail**

Run: `go test -count=1 ./pkg/webui -run 'TestHandle.*PermissionRule'`
Expected: FAIL because handlers/routes do not exist yet.

- [ ] **Step 3: Implement minimal handlers and route registration**

Implement:
- `GET /api/permission-rules`
- `POST /api/permission-rules`
- `PUT /api/permission-rules/:id`
- `DELETE /api/permission-rules/:id`

- [ ] **Step 4: Run the API tests to verify they pass**

Run: `go test -count=1 ./pkg/webui -run 'TestHandle.*PermissionRule'`
Expected: PASS

- [ ] **Step 5: Run focused regression**

Run: `go test -count=1 ./pkg/permissionrules ./pkg/agent ./pkg/webui`
Expected: PASS

- [ ] **Step 6: Commit the API slice**

```bash
git add pkg/webui/server.go pkg/webui/server_permission_rules_test.go
git commit -m "feat(webui): add permission rule management api"
```

### Task 5: Add Minimal WebUI Management Surface

**Files:**
- Modify: `pkg/webui/frontend/src/App.tsx`
- Modify: `pkg/webui/frontend/src/components/Sidebar.tsx`
- Create: `pkg/webui/frontend/src/hooks/usePermissionRules.ts`
- Create: `pkg/webui/frontend/src/pages/PermissionRulesPage.tsx`
- Modify: `pkg/webui/frontend/public/i18n/en.json`
- Modify: `pkg/webui/frontend/public/i18n/zh-CN.json`
- Modify: `pkg/webui/frontend/public/i18n/ja.json`

- [ ] **Step 1: Write the smallest possible UI contract test or at least type-safe hook usage**

If no existing UI test harness is practical, this step is:
- add the page and hook with compile-time safety
- rely on `npm run build` as the required verification gate

- [ ] **Step 2: Implement the minimal page**

Include:
- list
- create
- edit
- enable/disable
- delete

Do not add:
- hit preview
- decision trace
- explainability graph

- [ ] **Step 3: Run frontend build**

Run: `cd pkg/webui/frontend && npm ci && npm run build`
Expected: PASS

- [ ] **Step 4: Run full backend regression**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 5: Update planning files and commit**

```bash
git add pkg/webui/frontend task_plan.md notes.md
git commit -m "feat(webui): add permission rule management page"
```

### Verification And Shipping

- [ ] Run: `go test ./...`
- [ ] Run: `cd pkg/webui/frontend && npm ci && npm run build`
- [ ] Update:
  - `task_plan.md`
  - `notes.md`
- [ ] Commit any remaining documentation/progress changes
