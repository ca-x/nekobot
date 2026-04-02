# Provider/Model Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rebuild provider/model management into separate provider connections, model catalog, and model routes; update runtime model resolution and WebUI to the new structure without supporting historical data migration.

**Architecture:** Backend-first rollout. First introduce provider type registry plus new provider/model/route data layer and runtime resolution, then replace provider APIs and WebUI pages so provider management and model management become separate surfaces. Existing provider record semantics are removed rather than shimmed.

**Tech Stack:** Go, Ent, Echo, React, TypeScript, TanStack Query, existing Nekobot runtime/provider infrastructure

---

### File Structure

**Backend new/changed responsibility map**
- Modify: `pkg/config/config.go`
  - Remove provider-carried model semantics; add new config shapes if still needed in runtime config.
- Modify: `pkg/storage/ent/schema/provider.go`
  - Shrink provider schema to provider connection only.
- Create: `pkg/storage/ent/schema/modelcatalog.go`
  - Model catalog entity.
- Create: `pkg/storage/ent/schema/modelroute.go`
  - Model route entity.
- Create: `pkg/providerregistry/registry.go`
  - Built-in provider type registry modeled after axonhub provider list metadata.
- Create: `pkg/modelstore/manager.go`
  - CRUD for model catalog.
- Create: `pkg/modelroute/manager.go`
  - CRUD and resolution helpers for model routes.
- Modify: `pkg/providerstore/manager.go`
  - Remove `models/default_model` handling; add `default_weight/enabled` semantics.
- Modify: `pkg/webui/server.go`
  - Replace `/api/providers` behavior, add provider type API, add model/model-route/discovery endpoints.
- Modify: `pkg/agent/agent.go`
  - Replace provider-default-model resolution path with model-route-based resolution.
- Modify: `pkg/agent/provider_groups.go`
  - Ensure provider group behavior still works with provider connection list.
- Modify: `pkg/runtimeagents/manager.go`
  - Ensure runtime selected `provider/model` resolves through new model route logic.
- Modify: `pkg/providers/rotation_factory.go`
  - Remove dependency on provider-carried models/default_model assumptions.

**Frontend new/changed responsibility map**
- Create: `pkg/webui/frontend/src/lib/provider-types.ts`
  - Frontend-facing provider type metadata normalization.
- Create: `pkg/webui/frontend/src/hooks/useProviderTypes.ts`
  - Fetch provider registry from backend.
- Create: `pkg/webui/frontend/src/hooks/useModels.ts`
  - CRUD/query hooks for model catalog and routes.
- Modify: `pkg/webui/frontend/src/hooks/useProviders.ts`
  - Match new provider connection shape.
- Modify: `pkg/webui/frontend/src/pages/ProvidersPage.tsx`
  - Align to axonhub-like provider UI, connection-only responsibility.
- Modify or replace: `pkg/webui/frontend/src/components/config/ProviderForm.tsx`
  - Registry-driven provider form with automatic discovery flow.
- Create: `pkg/webui/frontend/src/pages/ModelsPage.tsx`
  - Axonhub-like model management surface.
- Create: `pkg/webui/frontend/src/components/models/*`
  - Model table/dialog/route editor components.
- Modify: `pkg/webui/frontend/src/App.tsx`
  - Route registration for models page.
- Modify: `pkg/webui/frontend/src/components/layout/Sidebar.tsx`
  - Add models navigation entry.

**High-risk runtime call sites**
- `pkg/agent/agent.go`
  - `callLLMWithFallback`, provider default model fallback logic.
- `pkg/config/config.go`
  - `ProviderProfile.GetDefaultModel`, `GetProviderConfig`.
- `pkg/webui/server.go`
  - provider list views and discovery flow.
- `pkg/webui/frontend/src/pages/ChatPage.tsx`
  - current default-model aggregation for selectors.
- `pkg/webui/frontend/src/pages/ConfigPage.tsx`
  - route default model UI assumptions.
- `pkg/webui/frontend/src/pages/CronPage.tsx`
  - provider-derived model option assembly.
- `pkg/commands/advanced.go`
  - debug output assuming provider default model.

### Task 1: Lock backend seam with failing tests

**Files:**
- Modify: `pkg/providerstore/manager_test.go`
- Create: `pkg/modelstore/manager_test.go`
- Create: `pkg/modelroute/manager_test.go`
- Modify: `pkg/agent/agent_test.go`
- Modify: `pkg/webui/server_config_test.go`

- [ ] **Step 1: Write failing tests for new provider connection semantics**

Write tests that require provider records to stop storing `models/default_model`, and instead verify `default_weight/enabled` behavior.

- [ ] **Step 2: Run targeted backend tests to verify RED**

Run: `go test -count=1 ./pkg/providerstore ./pkg/agent ./pkg/webui`
Expected: FAIL due to old provider shape assumptions.

- [ ] **Step 3: Write failing tests for model catalog and model route managers**

Cover:
- create/list/update model catalog
- create/list/update model routes
- effective weight resolution
- default provider resolution
- alias/regex route lookup

- [ ] **Step 4: Run new manager tests to verify RED**

Run: `go test -count=1 ./pkg/modelstore ./pkg/modelroute`
Expected: FAIL because packages/types do not exist yet.

- [ ] **Step 5: Commit**

```bash
git add pkg/providerstore/manager_test.go pkg/modelstore/manager_test.go pkg/modelroute/manager_test.go pkg/agent/agent_test.go pkg/webui/server_config_test.go
git commit -m "test: lock provider model redesign backend seams"
```

### Task 2: Introduce provider type registry and new backend entities

**Files:**
- Create: `pkg/providerregistry/registry.go`
- Create: `pkg/providerregistry/registry_test.go`
- Modify: `pkg/storage/ent/schema/provider.go`
- Create: `pkg/storage/ent/schema/modelcatalog.go`
- Create: `pkg/storage/ent/schema/modelroute.go`

- [ ] **Step 1: Write the failing tests for provider registry**

Require:
- provider type list is non-empty
- contains known kinds mirrored from axonhub-inspired list
- registry entries expose display/icon/discovery metadata

- [ ] **Step 2: Run provider registry test to verify RED**

Run: `go test -count=1 ./pkg/providerregistry`
Expected: FAIL because package does not exist yet.

- [ ] **Step 3: Implement minimal provider registry**

Create built-in registry only. No remote sync.

- [ ] **Step 4: Add new Ent schemas and generate/update code**

Implement:
- provider connection-only schema
- model catalog schema
- model route schema

- [ ] **Step 5: Run focused schema/package tests**

Run: `go test -count=1 ./pkg/providerregistry ./pkg/providerstore`
Expected: provider registry green; providerstore still failing until next task.

- [ ] **Step 6: Commit**

```bash
git add pkg/providerregistry pkg/storage/ent/schema/provider.go pkg/storage/ent/schema/modelcatalog.go pkg/storage/ent/schema/modelroute.go pkg/storage/ent
git commit -m "feat: add provider registry and model entities"
```

### Task 3: Rebuild provider/model/route stores

**Files:**
- Modify: `pkg/providerstore/manager.go`
- Create: `pkg/modelstore/manager.go`
- Create: `pkg/modelroute/manager.go`
- Modify: `pkg/providerstore/manager_test.go`
- Create: `pkg/modelstore/manager_test.go`
- Create: `pkg/modelroute/manager_test.go`

- [ ] **Step 1: Implement providerstore as provider-connection-only**

Remove `models/default_model` persistence and normalization logic.

- [ ] **Step 2: Implement modelstore CRUD**

Support catalog CRUD/list/search.

- [ ] **Step 3: Implement modelroute manager**

Support:
- route CRUD
- default provider enforcement
- effective weight helper
- alias/regex lookup helper

- [ ] **Step 4: Run store tests**

Run: `go test -count=1 ./pkg/providerstore ./pkg/modelstore ./pkg/modelroute`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/providerstore/manager.go pkg/providerstore/manager_test.go pkg/modelstore pkg/modelroute
git commit -m "feat: rebuild provider and model stores"
```

### Task 4: Switch runtime model resolution to model routes

**Files:**
- Modify: `pkg/agent/agent.go`
- Modify: `pkg/agent/agent_test.go`
- Modify: `pkg/agent/fx.go`
- Modify: `pkg/runtimeagents/manager.go`
- Modify: `pkg/providers/rotation_factory.go`
- Modify: `pkg/config/config.go`

- [ ] **Step 1: Write failing tests for route-based resolution**

Cover:
- explicit model resolves via model route
- route weight overrides provider default weight
- default provider chosen when multiple providers exist
- alias/regex route lookup works

- [ ] **Step 2: Run targeted runtime tests to verify RED**

Run: `go test -count=1 ./pkg/agent ./pkg/runtimeagents`
Expected: FAIL due to provider default model assumptions.

- [ ] **Step 3: Implement minimal route-based resolution**

Keep fail-fast behavior when a model has no active route.

- [ ] **Step 4: Run targeted runtime tests to verify GREEN**

Run: `go test -count=1 ./pkg/agent ./pkg/runtimeagents`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/agent/agent.go pkg/agent/agent_test.go pkg/agent/fx.go pkg/runtimeagents/manager.go pkg/providers/rotation_factory.go pkg/config/config.go
git commit -m "feat: resolve models through model routes"
```

### Task 5: Replace provider/discovery API and add model APIs

**Files:**
- Modify: `pkg/webui/server.go`
- Modify: `pkg/webui/server_config_test.go`
- Create or modify: `pkg/webui/server_models_test.go`

- [ ] **Step 1: Write failing API tests**

Cover:
- `/api/provider-types`
- `/api/providers` new shape
- `/api/models`
- `/api/model-routes`
- discovery flow creates/updates catalog and routes

- [ ] **Step 2: Run targeted API tests to verify RED**

Run: `go test -count=1 ./pkg/webui -run 'TestHandle(GetProviderTypes|GetProviders|CreateProvider|DiscoverProviderModels|GetModels|CreateModel|UpdateModelRoute)'`
Expected: FAIL until handlers exist.

- [ ] **Step 3: Implement minimal backend handlers**

Boundary only. Business logic stays in stores/managers.

- [ ] **Step 4: Run full webui backend tests**

Run: `go test -count=1 ./pkg/webui`
Expected: PASS except any unrelated baseline issues already known.

- [ ] **Step 5: Commit**

```bash
git add pkg/webui/server.go pkg/webui/server_config_test.go pkg/webui/server_models_test.go
git commit -m "feat: add provider and model management APIs"
```

### Task 6: Rebuild ProvidersPage to axonhub-like provider management

**Files:**
- Modify: `pkg/webui/frontend/src/hooks/useProviders.ts`
- Create: `pkg/webui/frontend/src/hooks/useProviderTypes.ts`
- Modify or replace: `pkg/webui/frontend/src/components/config/ProviderForm.tsx`
- Modify: `pkg/webui/frontend/src/pages/ProvidersPage.tsx`
- Create: `pkg/webui/frontend/src/lib/provider-types.ts`

- [ ] **Step 1: Write or update frontend tests where feasible**

If frontend test harness is absent, at minimum add TypeScript-checked behavior through component logic and rely on build verification.

- [ ] **Step 2: Implement provider type registry integration**

Provider type list, icons, field metadata, discovery-driven create flow.

- [ ] **Step 3: Align ProvidersPage layout with axonhub-inspired structure**

Copy/adapt layout patterns, not backend assumptions.

- [ ] **Step 4: Run frontend build**

Run: `npm --prefix pkg/webui/frontend run build`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/webui/frontend/src/hooks/useProviders.ts pkg/webui/frontend/src/hooks/useProviderTypes.ts pkg/webui/frontend/src/components/config/ProviderForm.tsx pkg/webui/frontend/src/pages/ProvidersPage.tsx pkg/webui/frontend/src/lib/provider-types.ts
git commit -m "feat: redesign provider management UI"
```

### Task 7: Add ModelsPage and route management UI

**Files:**
- Create: `pkg/webui/frontend/src/hooks/useModels.ts`
- Create: `pkg/webui/frontend/src/pages/ModelsPage.tsx`
- Create: `pkg/webui/frontend/src/components/models/*`
- Modify: `pkg/webui/frontend/src/App.tsx`
- Modify: `pkg/webui/frontend/src/components/layout/Sidebar.tsx`

- [ ] **Step 1: Implement frontend model query/mutation hooks**

Support catalog and route operations.

- [ ] **Step 2: Build ModelsPage with axonhub-like interaction flow**

Support:
- list/search
- enable/disable
- route editing
- default provider
- weights
- alias/regex

- [ ] **Step 3: Register app route and navigation**

Add sidebar entry and route.

- [ ] **Step 4: Run frontend build**

Run: `npm --prefix pkg/webui/frontend run build`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/webui/frontend/src/hooks/useModels.ts pkg/webui/frontend/src/pages/ModelsPage.tsx pkg/webui/frontend/src/components/models pkg/webui/frontend/src/App.tsx pkg/webui/frontend/src/components/layout/Sidebar.tsx
git commit -m "feat: add model management UI"
```

### Task 8: Update remaining consumers and verify end-to-end

**Files:**
- Modify: `pkg/webui/frontend/src/pages/ChatPage.tsx`
- Modify: `pkg/webui/frontend/src/pages/ConfigPage.tsx`
- Modify: `pkg/webui/frontend/src/pages/CronPage.tsx`
- Modify: `pkg/commands/advanced.go`
- Modify: any runtime/provider references found during integration

- [ ] **Step 1: Write failing integration tests for remaining consumers**

Cover:
- chat model options no longer derive from provider.default_model
- cron/config pages read model catalog instead of provider records

- [ ] **Step 2: Run consumer tests to verify RED**

Run: `go test -count=1 ./pkg/webui ./pkg/commands`
Expected: FAIL on old assumptions.

- [ ] **Step 3: Implement remaining consumer updates**

Remove all provider-carried model assumptions.

- [ ] **Step 4: Run focused and broad verification**

Run:
- `go test -count=1 ./pkg/providerregistry ./pkg/providerstore ./pkg/modelstore ./pkg/modelroute ./pkg/agent ./pkg/runtimeagents ./pkg/webui`
- `npm --prefix pkg/webui/frontend run build`
- `go test -count=1 ./...`

Expected:
- focused suites pass
- note any existing unrelated repo baseline failures explicitly

- [ ] **Step 5: Commit**

```bash
git add pkg/webui/frontend/src/pages/ChatPage.tsx pkg/webui/frontend/src/pages/ConfigPage.tsx pkg/webui/frontend/src/pages/CronPage.tsx pkg/commands/advanced.go
git commit -m "feat: finish provider model redesign integration"
```
