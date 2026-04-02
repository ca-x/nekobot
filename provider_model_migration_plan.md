# Provider/Model Migration Plan

> Last Updated: 2026-04-02

## Goal
参考 `/home/czyt/code/claude-code/axonhub`，把 `nekobot` 当前“provider 连接信息 + model 列表配置”耦合在同一条记录里的设计拆开，先完成：
- provider 连接管理
- model 列表与 provider 指派

迁移完成后，再继续当前主线开发任务 `agent tool_session spawn -> process -> tasks.Service`。

## Key Constraint
- **不考虑历史 provider 数据迁移。**
- 这次按 clean-slate 方式重做当前 provider/model 管理层，不要求兼容旧数据库行、旧 WebUI 表单状态、旧导入格式。
- 这个约束不是只针对 provider/model：
  - 当前系统所有 active 开发计划都不考虑历史数据迁移
  - 历史数据迁移统一后置到“全部开发完成以后”单独处理

## Current State

### Nekobot 当前实现
- [`pkg/config/config.go`](/home/czyt/code/nekobot/pkg/config/config.go)
  - `config.ProviderProfile` 同时承载：
    - provider 连接信息
    - models 列表
    - default_model
- [`pkg/storage/ent/schema/provider.go`](/home/czyt/code/nekobot/pkg/storage/ent/schema/provider.go)
  - 目前数据库 `provider` 表也沿用同样耦合结构：
    - `provider_kind`
    - `api_key`
    - `api_base`
    - `proxy`
    - `models_json`
    - `default_model`
- [`pkg/providerstore/manager.go`](/home/czyt/code/nekobot/pkg/providerstore/manager.go)
  - provider store 只管理这一种耦合后的 profile。
- [`pkg/webui/frontend/src/components/config/ProviderForm.tsx`](/home/czyt/code/nekobot/pkg/webui/frontend/src/components/config/ProviderForm.tsx)
  - Provider 表单既负责 provider 连接管理，也负责模型发现、模型挑选和默认模型设置。

### AxonHub 可借鉴点
- `/home/czyt/code/claude-code/axonhub/frontend/src/features/models/data/providers.schema.ts`
  - 把 provider/developer 元数据和 model 元数据分开管理。
- `/home/czyt/code/claude-code/axonhub/frontend/src/features/models/data/schema.ts`
  - 单独建模 model 实体，而不是把模型列表塞回 provider 记录。
- `/home/czyt/code/claude-code/axonhub/docs/zh/guides/model-management.md`
  - 明确区分“连接层”和“模型层”。

## Design Direction

### Decision 1: 先拆“连接”，再拆“模型”
- Provider 层只负责：
  - 名称
  - provider kind
  - 凭据
  - API base
  - proxy
  - timeout
- Model 层单独负责：
  - model id
  - display name
  - provider 指派
  - 默认路由/启用状态
  - 后续可扩能力元数据

### Decision 2: 不照搬 AxonHub 的完整 channel/model association DSL
- 当前只迁移最需要的两层：
  - provider connections
  - model configs
- 暂不引入 AxonHub 那套完整能力：
  - channel associations
  - regex mapping
  - tags-based selection
  - complex routing graph

### Decision 3: model 列表先支持“一个模型配置多个 provider”
- 为后续 runtime/provider failover 保留空间。
- 第一版建议 model 配置至少支持：
  - `model_id`
  - `display_name`
  - `providers []string`
  - `default_provider string`
  - `enabled bool`
- 这样能满足“在模型列表设置不同的提供商”这个目标，同时不把路由复杂度一次性拉满。

### Decision 4: 当前 provider CRUD / API / WebUI 可以直接替换
- 因为不需要历史数据迁移，本次允许：
  - 替换 ent schema
  - 替换 provider store 结构
  - 调整 `/api/providers` 返回形状
  - 新增 `/api/models`
  - 重做 WebUI Provider / Model 页面职责边界

## Recommended Migration Shape

### Backend
1. 新建 provider connection 实体，替代当前耦合的 `ProviderProfile`。
2. 新建 model config 实体，单独保存模型列表与 provider 指派。
3. provider discovery 结果不再直接写回 provider connection，而是写入/更新 model config。
4. `agent` / `runtimeagents` / provider routing 改为：
   - 先解析 model config
   - 再得到 provider connection 列表
   - 再交给现有 provider client / fallback 逻辑

### API
1. 保留 provider CRUD，但语义改成“连接管理”。
2. 新增 model CRUD / list / discover-assignment API。
3. provider discovery API 改成“探测可用模型候选”，而不是直接绑定到 provider record。

### WebUI
1. Providers 页面只做 provider connection 管理。
2. 新增 Models 页面或等价的 model list 管理入口。
3. model list 支持：
   - 搜索
   - 启用/禁用
   - 选择 provider
   - 设置默认 provider
4. 当前 `ProviderForm` 内嵌的模型挑选器将下放到 model 管理页，不再继续混在 provider 表单里。

## Phases

### Phase 1: 定义新数据模型
- 新 provider connection schema / config shape
- 新 model config schema / config shape
- 明确运行时读取接口

### Phase 2: 后端存储层迁移
- 重做 ent schema
- 重做 provider store
- 新建 model store
- 去掉 provider record 上的 `models/default_model` 责任

### Phase 3: API 层迁移
- 调整 `/api/providers`
- 新增 `/api/models`
- 重写 model discovery / assignment 流程

### Phase 4: WebUI 迁移
- Provider 页面瘦身为连接管理
- 新增 Model 列表管理界面
- 把 provider/model 表单逻辑解耦
- 可直接参考 / 迁用 AxonHub 的 provider/model 管理界面代码

### Phase 5: 运行时接线与验证
- runtime/agent/provider routing 切到新 model source
- 定向测试
- WebUI API 测试
- 手动验证 provider add + model assign

### Phase 6: 恢复主线开发
- 本迁移完成后，恢复：
  - `agent tool_session spawn -> process -> tasks.Service`

## Non-Goals
- 不处理旧 provider 数据自动迁移
- 不处理任何当前系统历史数据自动迁移
- 不保留旧 provider API 返回结构兼容层
- 不引入完整 AxonHub channels/associations/routing DSL
- 不在本迁移里处理 task lifecycle 主线问题

## Affected Areas
- [`pkg/config/config.go`](/home/czyt/code/nekobot/pkg/config/config.go)
- [`pkg/storage/ent/schema/provider.go`](/home/czyt/code/nekobot/pkg/storage/ent/schema/provider.go)
- `pkg/storage/ent/schema/` 下新增 model 相关 schema
- [`pkg/providerstore/manager.go`](/home/czyt/code/nekobot/pkg/providerstore/manager.go)
- `pkg/modelstore/` 新模块
- [`pkg/webui/server.go`](/home/czyt/code/nekobot/pkg/webui/server.go)
- [`pkg/webui/frontend/src/pages/ProvidersPage.tsx`](/home/czyt/code/nekobot/pkg/webui/frontend/src/pages/ProvidersPage.tsx)
- [`pkg/webui/frontend/src/components/config/ProviderForm.tsx`](/home/czyt/code/nekobot/pkg/webui/frontend/src/components/config/ProviderForm.tsx)
- `pkg/webui/frontend/src/pages/` 下新增 model 管理页面
- `pkg/webui/frontend/src/hooks/` 下新增 model hooks

## Success Criteria
- provider 连接信息不再保存 models/default_model
- 系统存在独立 model 列表管理入口
- 单个 model 可配置一个或多个 provider
- agent/runtime 能从 model 配置解析到 provider 连接
- provider add / model assign 完成后可以正常发起模型调用
- 本迁移完成后，主线任务恢复到 `agent tool_session spawn -> process -> tasks.Service`

## Execution Order Recommendation
1. 先实现 backend schema/store/API
2. 再迁 WebUI provider/model 管理
3. 最后接运行时消费
4. 完成后再回到 task lifecycle 主线

## Status
**Planning Completed** - 设计已收口到 [`docs/superpowers/specs/2026-04-02-provider-model-redesign-design.md`](/home/czyt/code/nekobot/docs/superpowers/specs/2026-04-02-provider-model-redesign-design.md)，可作为当前主计划中的前置迁移任务执行；完成后再继续 task lifecycle 主线。
