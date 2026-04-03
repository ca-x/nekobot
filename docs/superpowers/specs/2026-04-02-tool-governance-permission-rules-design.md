# Tool Governance Permission Rules Design

> Date: 2026-04-02
> Status: Draft approved by user, pending written-spec review

## Goal
在当前 `runtime/task lifecycle` 主线完成后，为 `nekobot` 引入第一版可持久化的 `permission rules`，把工具调用权限从仅依赖 `approval mode + allowlist/denylist` 的单点检查，升级为“统一规则入口 + 可解释决策 + approval fallback”的最小闭环。

本轮目标不是完成完整的 tool governance pipeline，而是先把第一层稳定基础设施立住，为后续：
- classifier / precheck
- pre/post/failure hooks
- AgentDefinition-scoped tool policies
- context sources explainability
提供稳定挂点。

## Global Constraints
- 当前所有 active 开发计划都不考虑历史数据迁移问题。
- 本轮不保留旧规则存储结构兼容层，也不做旧 approval 配置自动迁移。
- 现有 `approval.Manager` 的 pending queue / manual approval 行为继续保留，但不再承担未来完整规则引擎的职责。

## Scope

### In Scope
- 新增独立的 `permission rule` 持久化模型
- 新增独立的 `permission rule evaluator`
- 所有 tool call 统一先经过 permission rules 决策
- 决策结果支持：
  - `allow`
  - `deny`
  - `ask`
- 未命中规则时，回落到当前 session/global `approval mode`
- 暴露规则 CRUD API
- 暴露最小决策解释信息，供后续 WebUI 和 runtime explainability 复用

### Out of Scope
- 参数级匹配
- glob / regex 工具名匹配
- speculative classifier
- pre-hook / post-hook / failure-hook
- WebUI 上复杂 explainability 可视化
- `context sources` 展示
- `AgentDefinition` 全量落地

## Design Direction

### Why a Separate Layer
不把规则直接揉进 `approval.Manager`，原因是：
- `approval.Manager` 当前更适合承担：
  - pending request queue
  - approve / deny 操作
  - session mode override
- `permission rules` 需要逐步长成：
  - policy evaluation
  - explanation
  - later classifier/hooks glue

如果一开始就把两者混在一起，后续很快会形成一个同时负责 mode、queue、policy、hooks 的脏对象。

### Minimum Architecture
第一版统一执行入口：

1. agent 收到 tool call
2. `permissionrules.Evaluator` 评估持久化规则
3. 若命中：
   - `allow`: 直接放行
   - `deny`: 直接拒绝
   - `ask`: 强制进入现有 approval queue
4. 若未命中：
   - 回落到现有 `approval.Manager` mode 逻辑

这样可以先建立一条稳定链路：
`rule store -> evaluator -> decision -> approval fallback`

## Rule Model

### Fields
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

### Field Semantics
- `tool_name`
  - 第一版仅支持精确匹配
  - 必填
- `session_id`
  - 为空表示对所有 session 生效
- `runtime_id`
  - 为空表示对所有 runtime 生效
- `action`
  - 仅允许：
    - `allow`
    - `deny`
    - `ask`
- `priority`
  - 数值越大优先级越高
  - 用于显式覆盖全局规则

## Matching Semantics

### Candidate Selection
规则命中条件：
- `enabled = true`
- `tool_name` 精确匹配当前 tool call
- 若规则有 `session_id`，则必须等于当前 session
- 若规则有 `runtime_id`，则必须等于当前 runtime

### Resolution Order
所有候选规则按以下顺序决出唯一结果：

1. `priority` 降序
2. 作用域具体度降序：
   - `session_id + runtime_id`
   - `session_id`
   - `runtime_id`
   - global
3. `updated_at` 降序
4. `id` 作为最终稳定排序

第一条命中的规则就是结果。

### Why First-Match
第一版不做“多条规则合并”或“allow/deny 冲突归并”，因为：
- 不易解释
- 容易隐藏覆盖关系
- 会让 WebUI 管理面在第一版就变复杂

单一命中规则更容易给出 explain 信息，也更适合后续加入 audit/logging。

## Decision Output

### Evaluator Result
Evaluator 至少返回：
- `matched`
- `action`
- `rule_id`
- `tool_name`
- `session_id`
- `runtime_id`
- `explanation`

### Explanation Shape
第一版 explanation 只需要最小结构：
- `source`: `rule` 或 `approval_mode_fallback`
- `reason`
- `matched_rule_id`（若有）
- `matched_scope`

后续 `context sources` 可直接复用这类 explain payload，而不是另起一套格式。

## Integration with Existing Approval

### Current Boundary
现有 `approval.Manager` 继续保留：
- pending request creation
- approve / deny
- session mode override
- allowlist / denylist（可先保留，后续再决定是否内收）

### New Call Chain
工具执行前：

1. 先查 `permission rules`
2. 若 evaluator 返回：
   - `allow`
     - 直接执行工具
   - `deny`
     - 返回拒绝结果
   - `ask`
     - 调用现有 approval pending 逻辑
3. 若无规则命中
   - 继续走现有 approval mode

### Fallback Semantics
未命中规则时，不改变当前：
- `auto`
- `prompt`
- `manual`

这样可以保证这轮引入 permission rules 不会打散现有行为。

## Storage and Package Layout

### New Backend Pieces
建议新增：
- `pkg/storage/ent/schema/permissionrule.go`
- `pkg/permissionrules`

`pkg/permissionrules` 职责：
- CRUD manager
- evaluator
- types / explanation types

不把这些逻辑塞回：
- `pkg/approval`
- `pkg/agent`
- `pkg/webui`

## API Surface

### Endpoints
第一版最小 API：
- `GET /api/permission-rules`
- `POST /api/permission-rules`
- `PUT /api/permission-rules/:id`
- `DELETE /api/permission-rules/:id`

### API Behavior
- 返回稳定排序结果，便于 WebUI 直接展示
- create/update 时强校验：
  - `tool_name` 非空
  - `action` 合法
  - `priority` 为整数
- 不允许把空 `tool_name` 规则写进去

## WebUI Direction

### First Version
第一版只做基础管理面即可：
- 列表
- 新建
- 编辑
- 启用/禁用
- 删除

字段只展示：
- tool name
- action
- priority
- session scope
- runtime scope
- description

### Explicitly Deferred
- 决策 trace 时间线
- explainability graph
- rule hit preview
- 与 context sources 的统一视图

## Testing Strategy

### Evaluator Tests
必须覆盖：
- 全局规则命中
- session 级规则覆盖全局规则
- runtime 级规则覆盖全局规则
- `priority` 更高的规则优先
- `ask` 返回正确决策
- disabled rule 不生效
- 未命中时返回 fallback-needed 结果

### Agent Integration Tests
必须覆盖：
- `allow` 规则时，工具直接执行且不进入 approval pending
- `deny` 规则时，工具被拒绝
- `ask` 规则时，工具进入 approval pending
- 无规则命中时，仍按现有 approval mode 行为工作

### API Tests
必须覆盖：
- list/create/update/delete
- 非法 action 拒绝
- 空 tool_name 拒绝

## Implementation Order

### Phase 1
- 补 spec 和主计划
- 新增 schema / store / manager 的最小数据层

### Phase 2
- 先写 evaluator 的 RED tests
- 实现 evaluator 和 explanation

### Phase 3
- 先写 agent tool-call 集成 RED tests
- 把 tool execution 接到 evaluator

### Phase 4
- 先写 API RED tests
- 实现 CRUD API

### Phase 5
- 补最小 WebUI 管理页
- 跑回归并更新计划

## Follow-Up After This Slice
本轮完成后，下一批才适合推进：
- `AgentDefinition` 的 tool policy surface
- `context sources` explainability
- classifier / precheck
- pre/post/failure hooks
- 更细粒度参数模式匹配
