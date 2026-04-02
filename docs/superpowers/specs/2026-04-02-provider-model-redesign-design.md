# Provider/Model Redesign Design

> Date: 2026-04-02
> Status: Draft approved by user, pending written-spec review

## Goal
在继续当前 `task lifecycle` 主线之前，先把 `nekobot` 的 provider/model 管理层做一次端到端 clean-slate 重构，参考 `/home/czyt/code/claude-code/axonhub` 的 provider 和 model 管理体验，但内部保持 `nekobot` 自己的数据结构和运行时组织方式。

本次重构不是只改管理页面，而是要一起完成：
- provider/model 存储层重构
- provider/model API 重构
- WebUI provider/model 管理页面重构
- 运行时模型解析与 provider 选择链路改造

## Global Constraints
- 当前所有 active 开发计划都不考虑历史数据迁移问题。
- 本次重构不保留旧 schema、旧 API、旧 WebUI 状态兼容层。
- 历史数据迁移统一后置到所有当前开发任务完成以后，再单独立项处理。

## Product Scope

### In Scope
- 引入内置 `provider type registry`
- 拆分 `provider connection` 和 `model config`
- 支持 provider 添加时自动发现可用模型
- 支持独立模型管理界面
- 支持一个模型绑定多个 provider
- 支持默认 provider
- 支持权重
- 支持别名/映射
- 支持正则匹配规则
- 调整模型调用链，让运行时从 model 层解析 provider 候选

### Out of Scope
- 历史数据自动迁移
- 与旧 provider record 的兼容读写
- 完整照搬 `axonhub` 的 channel/model association DSL
- 本轮内继续推进 `agent tool_session spawn -> process -> tasks.Service`

## UX Direction

### Provider Management
- `ProvidersPage` 的图标、布局、交互节奏尽量向 `axonhub` 的 provider/channel 管理页靠齐。
- provider 类型不是硬编码在表单里的静态枚举，而是来自内置 `provider type registry`。
- 新增/编辑 provider 时：
  - 先选择 provider 类型
  - 再展示该类型的字段与默认值
  - 保存后立即支持自动模型探测
- provider 页面只负责连接管理，不再负责保存模型列表或默认模型。

### Model Management
- 新增独立 `ModelsPage`，结构和交互尽量贴近 `axonhub` models 页面。
- 模型页至少支持：
  - 列表浏览
  - 搜索/过滤
  - 启用/禁用
  - 查看关联 provider
  - 设置默认 provider
  - 配置 route weight
  - 配置 alias / mapping
  - 配置 regex 规则
- 第一版不实现 `axonhub` 那种完整 association DSL，只实现 `nekobot` 需要的等价能力。

## Data Model

### 1. ProviderTypeRegistry
仓库内置，作为 provider 类型元数据来源。

建议字段：
- `id`
- `display_name`
- `icon`
- `description`
- `default_api_base`
- `supports_discovery`
- `capabilities`
- `auth_fields`
- `advanced_fields`

用途：
- 渲染 provider type 列表
- 驱动新增 provider 表单
- 统一 provider logo / 文案 / 默认值

### 2. ProviderConnection
负责 provider 连接信息，不再保存模型列表。

建议字段：
- `id`
- `name`
- `provider_kind`
- `credentials`
- `api_base`
- `proxy`
- `timeout`
- `default_weight`
- `enabled`
- `created_at`
- `updated_at`

### 3. ModelCatalog
负责系统里的模型目录。

建议字段：
- `id`
- `model_id`
- `display_name`
- `developer`
- `family`
- `type`
- `capabilities`
- `catalog_source`
- `enabled`
- `created_at`
- `updated_at`

说明：
- 模型目录支持两种来源并存：
  - 内置目录
  - provider 探测补充

### 4. ModelRoute
负责连接 model 和 provider。

建议字段：
- `id`
- `model_id`
- `provider_name`
- `enabled`
- `is_default`
- `weight_override`
- `aliases`
- `regex_rules`
- `metadata`
- `created_at`
- `updated_at`

说明：
- `ModelRoute` 是后续路由、默认 provider、权重、映射规则的承载层。
- 不再把这些语义挂回 provider 本体。

## Weight Rules
- `provider.default_weight` 作为 provider 默认权重。
- `model-route.weight_override` 优先覆盖 provider 默认权重。
- 运行时最终排序时：
  1. 先过滤 disabled route / disabled provider
  2. 计算 effective weight
  3. 再交给现有 provider/fallback 逻辑

## Discovery Strategy
- provider 添加或编辑完成后可立即执行 discovery。
- discovery 输出不直接写回 provider connection。
- discovery 结果处理规则：
  - catalog 已存在该模型：补 route 或更新 route
  - catalog 不存在该模型：新增 catalog，再补 route
- 发现模型只是“候选输入”，最终是否启用、默认 provider、权重、alias/regex 由 model management 决定。

## Backend Architecture

### Storage
- 重做当前 `provider` schema，使其只表示 `ProviderConnection`
- 新增 `model catalog` schema
- 新增 `model route` schema
- 对应新增：
  - `providerstore`
  - `modelstore`
  - `modelroutestore` 或等价模块

### API
需要拆成四组：

1. `provider type registry API`
- 提供 provider 类型列表给前端

2. `providers API`
- provider connection CRUD
- provider runtime health/status

3. `models API`
- model catalog CRUD / list / search

4. `model routes API`
- route CRUD
- 默认 provider 设置
- 权重调整
- alias / regex 配置

5. `discovery API`
- 以 provider connection 为输入执行 discovery
- 返回待绑定模型候选
- 允许落地到 catalog + routes

## Runtime Integration

### Current Problem
当前系统主要从 provider profile 直接取 `models/default_model`，provider 同时承载“连接层”和“模型层”，导致：
- provider 管理和模型管理耦合
- 多 provider per model 表达困难
- route 权重只能粗粒度依附 provider

### New Resolution Flow
运行时调用链改为：
1. 输入 `model_id`
2. 查 `ModelCatalog`
3. 查该模型的 `ModelRoute`
4. 解析可用 provider 候选
5. 计算 effective weight
6. 选出 provider connection
7. 再进入现有 provider client / fallback 执行链

### Mapping/Regex Support
第一版支持：
- alias / mapping
- regex 匹配规则

但限制为 `nekobot` 自己的 route 结构，不复制 `axonhub` 的完整 association DSL。

## Frontend Architecture

### Providers Page
重构方向：
- provider type selector 来自 registry
- 表单字段由 registry 驱动
- 图标、视觉结构、布局尽量贴近 `axonhub`
- provider 卡片/列表只显示连接相关状态
- 模型探测作为 provider 创建流程的一部分，但模型结果不存回 provider

### Models Page
新增或重做模型页，承担：
- catalog 浏览
- provider route 关系管理
- 默认 provider 管理
- 权重配置
- alias / regex 编辑

### Shared Frontend Data Sources
前端需要新增：
- `useProviderTypes`
- `useModels`
- `useModelRoutes`
- provider/model route 相关 mutation hooks

## Verification
本重构完成时至少需要：
- 后端单元测试
  - provider store
  - model store
  - route store
  - discovery merge
  - runtime resolution
- WebUI API 测试
  - providers
  - models
  - routes
  - discovery
- 前端构建验证
- 手动验证
  - 新增 provider
  - 自动发现模型
  - 在 model 页面绑定 provider
  - 设置默认 provider
  - 设置权重
  - 成功发起模型调用

## Implementation Order
1. 引入内置 `provider type registry`
2. 重做 backend schema/store
3. 重做 provider/model/routes/discovery API
4. 重构 `ProvidersPage`
5. 新增/重构 `ModelsPage`
6. 切运行时模型解析链
7. 跑验证
8. 恢复 `agent tool_session spawn -> process -> tasks.Service`

## Risks
- provider/model/runtime 三层同时改动，回归面较大
- WebUI 会从“provider 表单带模型管理”切到“双页面 + route 管理”，前后端接口变化大
- 现有运行时选择逻辑要避免把 route 层和 provider fallback 层重复实现

## Key Decisions
- 内部结构保持 `nekobot` 风格，不照搬 `axonhub` 后端 DSL
- 前端 provider/model 管理体验尽量贴近 `axonhub`
- 权重采用“两层支持，route 优先覆盖 provider 默认”
- 模型来源采用“内置目录 + provider 探测”并存
- 当前所有 active 开发不考虑历史数据迁移

## Approval State
用户已确认以下设计边界：
- provider/model 迁移是端到端改造
- provider 类型列表引入并内置在仓库中
- `ProvidersPage` 图标、布局、交互向 `axonhub` 靠齐
- `ModelsPage` 管理体验也尽量向 `axonhub` 靠齐
- 后端内部结构保持 `nekobot` 风格
- 支持现有权重，并同时支持 provider 默认权重与 model-route 覆盖权重
- 模型来源采用“目录 + discovery”双源
- 第一版支持 alias/mapping 和 regex 规则，但不做完整 association DSL
