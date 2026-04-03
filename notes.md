# Notes: nextclaw + picoclaw → nekobot 特性分析

## 2026-04-02 全局规划约束补记

### 新的全局约束
- 当前系统的所有 active 开发计划都**不考虑历史数据迁移问题**。
- 这个约束不仅适用于本次 provider/model 迁移，也适用于后续其他主线任务。
- 历史数据迁移统一后置：
  - 等所有当前开发任务完成以后
  - 再单独作为迁移专项处理

### 对当前计划顺序的影响
- `provider/model` 迁移将按 clean-slate 方式执行。
- 当前主计划顺序调整为：
  1. 先执行 `provider_model_migration_plan.md`
  2. 再恢复 `agent tool_session spawn -> process -> tasks.Service`
  3. 然后才是 `cron executeJob` / `watch executeCommand`

### 当前明确结论
- 当前任何 active 计划都不需要为了旧数据库、旧 API 返回形状、旧 WebUI 表单状态保留兼容层。
- 如果后续某一处需要兼容历史数据，应视为违反当前计划约束，必须单独提出来，而不是在实现中顺手混入。

## 2026-04-02 Provider/Model 重构第一段后端切片补记

### 本轮完成
- `pkg/storage/ent/schema/provider.go`
  - `provider` schema 已收缩为 connection-only：
    - 去掉 `models_json`
    - 去掉 `default_model`
    - 新增 `default_weight`
    - 新增 `enabled`
- `pkg/providerstore/manager.go`
  - provider store 不再持久化 provider 自带的模型列表。
  - `normalizeProvider()` 当前明确把 provider 记录收敛为 connection-only：
    - `Models = []`
    - `DefaultModel = ""`
    - 默认 `DefaultWeight = 1`
    - `Timeout` 仍按现有规则兜底
- `pkg/providerregistry/registry.go`
  - 新增内置 provider type registry。
  - 当前先内置最小集合：
    - `openai`
    - `anthropic`
    - `gemini`
    - `openrouter`
    - `ollama`
- `pkg/webui/server.go`
  - 新增 `/api/provider-types`
  - `/api/providers` 的 provider 投影视图已改为 connection-only：
    - 返回 `default_weight`
    - 返回 `enabled`
    - 不再返回 `models/default_model`
  - `supports_discovery` 改为基于 registry 判断，而不是本地硬编码 switch

### 这一步刻意没有做的事
- 没有把 `config/import` / `config/export` 的 provider 外部格式一起重做。
- 没有删除 `config.ProviderProfile` 上旧的 `Models/DefaultModel` 字段。
- 没有改 runtime `agent/runtimeagents/providers` 对旧模型语义的消费。
- 没有开始前端 `ProvidersPage` / `ModelsPage` 的 AxonHub 对齐迁移。

### 原因
- 当前这一步只想先把 provider 基础层真正从“连接+模型配置”里拆出来，而不把 runtime / import-export / frontend 同时打碎。
- 为了维持阶段性可编译和可验证，先允许：
  - 外层 config shape 仍保留旧字段
  - 但 provider store / provider API 投影已经按新语义落地

### 当前验证
- `go test -count=1 ./pkg/providerregistry ./pkg/providerstore ./pkg/webui`

### 下一步建议
- 直接进入 `model catalog + model route` 存储层：
  - 新 ent schema
  - `modelstore`
  - `modelroute`
  - route lookup / default provider / effective weight helper
- 这一步完成后，再切 runtime model resolution。

## 2026-04-02 Provider/Model 重构第二段数据层补记

### 本轮完成
- `pkg/storage/ent/schema/modelcatalog.go`
  - 新增模型目录实体。
- `pkg/storage/ent/schema/modelroute.go`
  - 新增模型到 provider 的路由实体。
- `pkg/modelstore/manager.go`
  - 已完成最小 catalog CRUD。
- `pkg/modelroute/manager.go`
  - 已完成最小 route CRUD 与解析辅助：
    - `ListByModel`
    - `ResolveInput`
    - `DefaultRoute`
    - `EffectiveWeight`

### 当前已锁定的 route 语义
- `weight_override > 0` 时优先使用 route weight。
- 否则回落到 provider 的 `default_weight`。
- `ResolveInput()` 第一版支持：
  - 直接 `model_id`
  - alias
  - regex rule
- `DefaultRoute()` 当前按：
  - `model_id`
  - `is_default = true`
  - `enabled = true`
  查唯一默认 route。

### 当前刻意保留的简化
- 还没有把 `modelstore/modelroute` 接进 WebUI API。
- 还没有让 runtime/agent 真正消费这些 store。
- `modelroute` 当前通过复用 `providerstore.Manager` 读取 provider 默认权重，后续若需要可以再抽更小的 provider read interface。

### 本轮验证
- `go test -count=1 ./pkg/modelstore ./pkg/modelroute`

### 下一步
- 切到 runtime model resolution：
  - `pkg/agent/agent.go`
  - `pkg/runtimeagents`
  - `pkg/providers/rotation_factory.go`
  - 目标是去掉 provider 自带 `models/default_model` 的运行时依赖。

## 2026-04-02 Provider/Model 重构第三段运行时补记

### 本轮完成
- `pkg/agent/agent.go`
  - `resolveModelForProvider()` 已改成：
    1. 优先读取 `modelroute`
    2. 找到当前 provider 对应 route 后，优先读 `metadata.provider_model_id`
    3. 如果没有 provider-specific model id，则回退到 route 的 `model_id`
    4. 只有在 route 层查不到时，才临时回退旧的 provider config 逻辑
- `pkg/agent/agent_test.go`
  - 原先“fallback 到 provider default model”的测试已经替换为 route 驱动测试。
  - 现在锁定：
    - 同一逻辑模型在 fallback provider 上可映射到不同的实际 provider model id。

### 当前语义
- 逻辑输入模型：
  - 先交给 `modelroute.ResolveInput()`
- provider-specific 实际模型：
  - 当前通过 `route.Metadata["provider_model_id"]` 表达
- 这意味着：
  - route 层现在已经不仅决定“用哪个 provider”
  - 也决定“这个 provider 实际该调用哪个模型 id”

### 当前刻意保留的过渡
- `config.ProviderProfile.Models/DefaultModel` 还没有从代码里完全删除。
- 但运行时主链已经优先走新 route 数据层。
- 旧字段当前只作为过渡 fallback，避免 API / 前端尚未迁完时立即断链。

### 本轮验证
- `go test -count=1 ./pkg/providerregistry ./pkg/providerstore ./pkg/modelstore ./pkg/modelroute ./pkg/agent ./pkg/runtimeagents ./pkg/webui`

### 下一步
- 开始补 WebUI API：
  - `/api/models`
  - `/api/model-routes`
  - provider discovery -> catalog + route merge

## 2026-04-02 Provider/Model 重构第四段 API 补记

### 本轮完成
- `pkg/webui/server.go`
  - 新增：
    - `/api/models`
    - `/api/model-routes`
  - provider discovery 现在不再只是返回模型列表，而是会 merge 到新数据层：
    - 若 catalog 不存在，则创建 `model catalog`
    - 为当前 provider 创建或更新 `model route`
    - 默认写入 `metadata.provider_model_id`
- `pkg/webui/server_models_test.go`
  - 已补：
    - models list/create
    - model-routes get/update
    - provider discovery -> catalog + route merge

### 当前 discovery merge 语义
- 当前保持最小可用版本：
  - discovery 拿到 provider 真实模型 id
  - catalog 直接以这个模型 id 建立基础记录
  - route metadata 同时记录 `provider_model_id`
- 这意味着：
  - 对“同一逻辑模型多个 provider 映射不同 provider model id”的更复杂归并，后续还可以继续强化
  - 但当前后端主链已经不再停留在“provider record 自己保存 models 列表”

### 本轮验证
- `go test -count=1 ./pkg/providerregistry ./pkg/providerstore ./pkg/modelstore ./pkg/modelroute ./pkg/agent ./pkg/runtimeagents ./pkg/webui`

### 下一步
- 进入前端迁移：
  - `useProviderTypes`
  - `useModels`
  - `ProvidersPage`
  - `ModelsPage`
  - `ProviderForm`
  - 以及 Chat/Config/Cron 对 provider-derived model list 的替换

## 2026-04-02 Provider/Model 前端迁移启动补记

### 计划索引复核结果
- 根目录当前存在的 `*_plan.md`：
  - `task_plan.md`
  - `claude_code_alignment_plan.md`
  - `migration_task_plan.md`
  - `provider_model_migration_plan.md`
  - `wechat_ilink_task_plan.md`
  - `closure_task_plan.md`
- 这些计划均已在 `task_plan.md -> Plan Index` 中被归类为：
  - `Source of Truth`
  - `Referenced Plans`
  - `Archived / Reference Only`
- 结论：
  - 当前没有新的未纳入主计划的 `_plan.md`
  - 不需要额外创建新的聚合计划文件

### 当前前端缺口
- `useProviders.ts` 仍暴露旧 provider-model 耦合字段：
  - `models`
  - `model_count`
  - `default_model`
  - `has_default_model`

## 2026-04-02 Context Economy chat route 透传补记

### 本轮完成
- `pkg/agent/agent.go`
  - `ChatRouteResult` 已新增只读 context pressure 字段：
    - `ContextBudgetStatus`
    - `ContextBudgetReasons`
    - `CompactionRecommended`
    - `CompactionStrategy`
  - `chatWithLegacyOrchestrator()` 现在会在真实 chat path 中复用现有 preview 计算逻辑，把预算状态和压缩建议挂到 route result。
- `pkg/agent/context_sources.go`
  - 抽出共享 helper，使：
    - `/api/prompts/context-sources` 预览
    - chat route metadata
    使用同一套 context footprint / budget / compaction 计算逻辑。
- `pkg/webui/server.go`
  - websocket `route_result` 已透传：
    - `context_budget_status`
    - `context_budget_reasons`
    - `compaction_recommended`
    - `compaction_strategy`
- `pkg/webui/frontend/src/pages/ChatPage.tsx`
  - Chat 页“实际路由”卡片现在会展示：
    - budget status badge
    - budget reasons
    - compaction strategy recommendation

### 当前刻意没有做的事
- 没有修改 provider request message。
- 没有触发自动 compaction。
- 没有根据 budget 状态拦截请求。
- 没有开始把 preview 决策写回 session 或持久化存储。

### 当前意义
- 这一步把 `context economy` 从“只在 Prompts 页面可预览”推进到“真实聊天链路可观察”。
- 但仍保持只读，所以不会引入新的隐式行为变化。

### 当前验证
- `go test -count=1 ./pkg/agent -run TestChatWithPromptContextDetailed_IncludesContextPressurePreview`

## 2026-04-03 Context Economy 首个非只读 decision 点补记

### 本轮完成
- `pkg/agent/agent.go`
  - `chatWithLegacyOrchestrator()` 现在会在首次 provider 调用前检查 `routeResult.Preflight.Action`。
  - 当动作为 `compact_before_run` 时，对 outbound `providerMessages` 执行一次 `forceCompressMessages()`。
- `pkg/agent/blades_runtime.go`
  - `chatWithBladesOrchestrator()` 会把 `preflight.action` 传入 `bladesModelProvider`。
  - `bladesModelProvider.Generate()` 会在首次底层 provider 调用前执行同样的一次性瞬时压缩。
- `pkg/agent/agent_test.go`
  - 已补 legacy critical 路径测试：
    - 首次 provider request 已被压缩。
    - session history 未被改写。
  - 已补 blades critical 路径测试：
    - 首次 provider request 已被压缩。
    - session history 未被改写。
  - 已补 warning 路径测试：
    - `consider_compaction` 仅保留建议，不自动压缩。

### 当前明确边界
- 只对 `preflight.action == compact_before_run` 自动执行。
- 只压缩一次首次 outbound request，不写回 session/history。
- 不新增阻断逻辑。
- 不自动生成摘要。
- 不做 memory / managed prompts / file context pruning。
- 如果 provider 之后仍报 context-limit，原有 retry-based emergency compression 逻辑继续保留。

### 语义影响
- `preflight` 从“只读建议”进入了最小真实执行路径，但仍保持 transient。
- UI 看到的 `preflight/action` 元数据不变；变化仅在 runtime 首次实际发给模型的 messages。
- 由于 preflight 是估算，仍可能出现：
  - 预压缩后 provider 仍报 context-limit
  - 同一次请求上再走原有 retry compression
  这两者当前都接受，属于既有 emergency path 的延续

### 本轮验证
- `go test -count=1 ./pkg/agent -run 'TestChatWithPromptContextDetailed_(AutoCompressesCriticalPreflightBeforeLegacyCall|DoesNotAutoCompressWarningPreflightBeforeLegacyCall|AutoCompressesCriticalPreflightBeforeBladesCall)$'`
- `go test -count=1 ./pkg/webui -run TestChatRouteStateJSONIncludesContextPressureFields`

## 2026-04-03 Context Economy preflight action websocket 契约补记

### 本轮完成
- `pkg/webui/server.go`
  - `chatRoutePreflightState` 已新增 `action` 字段。
  - chat websocket 的 `route_result.preflight` 现在会透传 `routeResult.Preflight.Action`。
- `pkg/webui/server_chat_test.go`
  - 已补 JSON 契约测试，锁定 `preflight.action` 会进入 websocket route payload。

### 当前意义
- `ChatPage` 之前已经优先读取 `routeResult.preflight.action`，但后端没有真正发出这个字段。
- 这一步只补齐现有 contract gap，不改变任何 preflight 语义，也不引入新的 runtime 行为。

### 本轮验证
- `go test -count=1 ./pkg/webui -run '^TestChatRouteStateJSONIncludesContextPressureFields$'`

## 2026-04-03 Context Economy orchestrator 对齐补记

### 本轮完成
- `pkg/agent/blades_runtime.go`
  - `chatWithBladesOrchestrator()` 已补齐与 legacy 路径一致的 route metadata 注入。
  - 现在 blades 路径在 `resolvedPrompts` 后也会复用共享 context preview helper，把：
    - `ContextBudgetStatus`
    - `ContextBudgetReasons`
    - `CompactionRecommended`
    - `CompactionStrategy`
    挂到 `ChatRouteResult`。
- `pkg/agent/agent_test.go`
  - 新增 blades 路径回归：
    - `TestChatWithPromptContextDetailed_BladesIncludesContextPressurePreview`

### 当前意义
- 这一步不是新增功能面，而是补齐 orchestrator parity。
- 避免 WebUI chat route 在 legacy / blades 下表现不一致。

### 当前刻意没有做的事
- 没有改变 blades 的 message compaction 行为。
- 没有让 budget/compaction 决策变成自动执行逻辑。

## 2026-04-03 Context Economy preflight applied 执行态补记

### 本轮完成
- `pkg/agent/context_sources.go`
  - `ContextPreflightDecision` 新增 `Applied bool`。
- `pkg/agent/agent.go`
  - 新增 `markPreflightApplied()`。
  - legacy 路径只有在真实执行 `compact_before_run` 压缩后，才把 `routeResult.Preflight.Applied` 标成 `true`。
- `pkg/agent/blades_runtime.go`
  - `bladesModelProvider` 新增 `onPreflightApplied` 回调。
  - blades 路径与 legacy 保持同一语义：只有真实执行 preflight 压缩时才上报 `applied = true`。
- `pkg/agent/agent_test.go`
  - 已补 legacy / blades 的 applied 行为断言。
- `pkg/webui/server.go`
  - websocket `route_result.preflight` 现在会透传 `applied`。
- `pkg/webui/frontend/src/hooks/useChat.ts`
  - `ChatRouteResult.preflight` 类型已补 `applied`。
- `pkg/webui/frontend/src/pages/ChatPage.tsx`
  - Chat 页现在能区分“只是建议动作”与“动作已实际执行”，最小展示为 `compact_before_run · applied`。
- `pkg/webui/server_chat_test.go`
  - 已补 websocket JSON 契约断言。

### 当前语义
- `preflight.action` 和 `preflight.applied` 现在明确区分：
  - `action` 表示建议或计划动作
  - `applied` 表示这次请求里是否真的执行了该动作
- 当前只会在以下场景把 `applied = true`：
  - `preflight.action == compact_before_run`
  - 且 runtime 真实执行了首次 outbound compression
- `warning/consider_compaction` 仍然只做提示，不会被错误标记成已执行。

### 本轮测试
- `go test -count=1 ./pkg/agent -run 'TestChatWithPromptContextDetailed_(IncludesContextPressurePreview|DoesNotAutoCompressWarningPreflightBeforeModelCall|AutoCompressesCriticalPreflightBeforeBlades)$'`
- `go test -count=1 ./pkg/webui -run '^TestChatRouteStateJSONIncludesContextPressureFields$'`
- `go test -count=1 ./pkg/agent ./pkg/webui`

## 2026-04-03 Context Economy preflight decision 收口补记

### 本轮完成
- `pkg/agent/context_sources.go`
  - 新增 `ContextPreflightDecision`。
  - `ContextSourcesPreview` 现在除了保留原有平铺字段，还会返回嵌套：
    - `preflight.budget_status`
    - `preflight.budget_reasons`
    - `preflight.compaction`
- `pkg/agent/agent.go`
  - `ChatRouteResult` 新增 `Preflight`。
  - legacy / blades 两条 chat 主链都复用共享 preview 结果，把只读 preflight decision 挂到 route result。
- `pkg/webui/server.go`
  - websocket `route_result` 现在新增嵌套 `preflight` 对象。
  - 现有平铺字段继续保留，作为兼容过渡。
- `pkg/webui/frontend/src/hooks/useChat.ts`
  - `ChatRouteResult` 类型已支持嵌套 `preflight`。
- `pkg/webui/frontend/src/pages/ChatPage.tsx`
  - Chat 页优先读取 `preflight`，旧平铺字段作为 fallback。

### 当前意义
- 这一步把原本分散的 budget/compaction 元数据收敛成一个更明确的“请求前决策面”。
- 后续如果进入真正的 runtime decision point、auto compact 或 request blocking，可以直接在 `preflight` 这条边界上继续扩展，而不是再造第三套字段。

### 当前刻意没有做的事
- 没有删除现有平铺字段。
- 没有把 `preflight` 变成自动执行策略。
- 没有开始保存 decision history。

## 2026-04-03 Context Economy preflight action 补记

### 本轮完成
- `pkg/agent/context_sources.go`
  - `ContextPreflightDecision` 新增 `Action`。
  - 当前建议动作规则非常窄：
    - `ok -> proceed`
    - `warning -> consider_compaction`
    - `critical -> compact_before_run`
- `pkg/agent/agent_test.go`
  - preview / legacy / blades 三条路径都已补回归，锁定 `preflight.action`。
- `pkg/webui/server_prompts_test.go`
  - 预览 API 已锁定 `preflight.action` 回包。
- `pkg/webui/frontend/src/pages/ChatPage.tsx`
  - Chat 页“实际路由”卡片已最小展示 `preflight.action`。

### 当前意义
- 这一步把 `preflight` 从“结构化观测”推进到“结构化建议”。
- 但仍然只是建议层，不参与自动执行。

### 当前提交状态
- `AgentDefinition bridge` 已作为独立功能批次提交并推送：
  - commit: `df08d2f feat(agent): add definition bridge snapshot`
  - remote: `origin/main`
- 当前剩余未提交代码已收敛为：
  - `context sources` 预览 API 与 explainability 数据结构
  - `readonly preflight` 在 chat route / Chat UI 的只读透传
  - `legacy / blades` orchestrator parity

### 当前刻意没有做的事
- 没有把 `consider_compaction` 或 `compact_before_run` 接到真正的 runtime 行为。
- 没有因为 `critical` 自动拦截请求。

### 当前前端目标拆分
1. 共享数据层
   - `useProviders`: 改为 connection-only provider 形状
   - `useProviderTypes`: 接 `/api/provider-types`
   - `useModels`: 接 `/api/models` + `/api/model-routes`
2. Provider UI
   - 提供 AxonHub 风格的 provider type 卡片选择
   - provider form 由 registry 驱动字段
   - discovery 只负责发现和落地模型，不再把模型持久化到 provider 表单状态
3. Models UI
   - 增加独立 `ModelsPage`
   - 展示 catalog + route
   - 支持 route 的默认项、权重、alias、regex、provider-specific model id
4. 消费方切换
   - Chat/Config/Cron 全部从 `/api/models` + `/api/model-routes` 取模型候选

## 2026-04-02 Provider/Model 前端迁移完成补记

### 本轮完成
- 新增共享 hooks：
  - `pkg/webui/frontend/src/hooks/useProviderTypes.ts`
  - `pkg/webui/frontend/src/hooks/useModels.ts`
- `pkg/webui/frontend/src/hooks/useProviders.ts`
  - 已改为 connection-only provider 契约：
    - `default_weight`
    - `enabled`
  - 已去掉旧的 provider-owned model 字段。
- `pkg/webui/frontend/src/components/config/ProviderForm.tsx`
  - 已改为 provider type registry 驱动表单。
  - discovery 语义改为同步到 shared Models workspace。
- `pkg/webui/frontend/src/pages/ProvidersPage.tsx`
  - 已改为 connection / runtime 视图，不再显示 provider 自带 models/default model。
- `pkg/webui/frontend/src/pages/ModelsPage.tsx`
  - 已新增独立模型管理页。
  - 当前支持：
    - catalog 列表
    - 搜索
    - route 展开编辑
    - `enabled`
    - `is_default`
    - `weight_override`
    - `aliases`
    - `regex_rules`
    - `metadata.provider_model_id`
- `pkg/webui/frontend/src/pages/ChatPage.tsx`
- `pkg/webui/frontend/src/pages/ConfigPage.tsx`
- `pkg/webui/frontend/src/pages/CronPage.tsx`
  - 已切换为从 shared model catalog + routes 构建模型选项。

### 当前验证
- `npm run build`（`pkg/webui/frontend`）已通过。

### 当前结论
- `provider/model` clean-slate 迁移的 WebUI 关键闭环已经成立：
  - provider 负责连接
  - model 负责逻辑模型与 route
  - discovery 负责把 provider 能力灌入 model catalog / routes
  - 消费侧不再从 provider record 推导模型列表

## 2026-04-02 codeany / open-agent-sdk-go 调研补记

### 调研对象
- `/home/czyt/code/codeany`
- `github.com/codeany-ai/open-agent-sdk-go@v0.3.0`
- 当前 `nekobot` 已集成的 `github.com/go-kratos/blades@v0.4.0`

### 对 `codeany` 的直接观察
- `codeany` 本身更像一个“终端 agent 产品壳”：
  - Bubble Tea TUI
  - slash commands
  - session 持久化
  - skills/plugins/memory 的文件系统组织
  - 权限模式与交互批准
- 它在核心运行时上并没有自己重做完整 agent 内核，而是直接依赖：
  - `open-agent-sdk-go/agent`
  - `open-agent-sdk-go/types`
  - `open-agent-sdk-go/hooks`
  - `open-agent-sdk-go/mcp`
- 从 `internal/tui/model.go` 和 `internal/pipe/pipe.go` 看，`codeany` 对 SDK 的使用方式相对薄：
  - 构建 `agent.Options`
  - 设置 model / api key / base url / MCP / hooks / permission mode
  - `Init()`
  - `Query()` / `Prompt()`
  - 接流式事件并渲染 UI

### `open-agent-sdk-go` 更像什么
- 更像“面向 Claude Code / terminal coding agent 产品形态”的应用 SDK。
- 它对以下能力应该是内建优先的：
  - permission mode
  - tool approval callback
  - MCP client
  - hook 配置
  - query/prompt 事件流
- 但从 `codeany` 的集成厚度看，它不像一个比 `blades` 更底层、更通用、更强工作流表达的框架；
  - 它更偏向“快速搭一个 Claude Code 类终端 agent 产品”的成品运行时。

### 与 `blades` 的差异

#### `blades` 更强的地方
- 更像通用 agent framework，而不是终端产品 SDK。
- 当前公开结构已经覆盖：
  - `Agent`
  - `Runner`
  - `ModelProvider`
  - `Tool`
  - `Memory`
  - `Middleware`
  - `flow`
  - `graph`
  - `skills`
  - `evaluator`
- 对多 agent workflow、graph orchestration、tool middleware、memory adapter 这些“运行平台层能力”，`blades` 显然更完整。
- 当前 `nekobot` 也已经围绕 `blades` 做了较深的适配：
  - blades runtime provider fallback
  - blades skill adapter
  - blades memory adapter
  - 现有 orchestrator 开关和测试覆盖

#### `open-agent-sdk-go` / `codeany` 值得吸收的地方
- 产品化终端体验更直接：
  - Bubble Tea TUI 结构更轻
  - slash commands 组织直给
  - 本地 session / memory / skills / plugins 目录约定清晰
  - permission mode 和 interactive approval 体验成型
- 如果 `nekobot` 后续要补“类似 Claude Code 的纯终端使用体验”，可以参考它的：
  - TUI 状态机组织
  - slash command 体系
  - 本地 config / session / memory 的落盘习惯
  - plugin/skill 目录协议

### 是否比当前 `blades` 更完善
- 如果问题是“作为通用 agent framework / orchestration substrate，是否比 blades 更完善”：
  - 当前证据不支持这个结论。
  - 反而更像：
    - `blades` 更强在框架层
    - `open-agent-sdk-go` 更偏产品 SDK / coding-agent runtime
- 如果问题是“作为快速做出 Claude Code 式终端 agent 产品的现成运行时，是否更省事”：
  - 是，有这个可能。
  - 尤其在：
    - permission modes
    - MCP
    - terminal event streaming
    - agent UI integration
    这些产品侧能力上，可能比你现在自拼 `blades + nekobot runtime + webui` 更直给。

### 对 `nekobot` 的建议
1. 不建议把当前 `blades` 主线直接切换到 `open-agent-sdk-go`。
   - 当前 `nekobot` 已经围绕 `blades` 做了不少 runtime、tool、memory、skills 适配。
   - 硬切会打断现有 `task lifecycle / runtime control plane / AgentDefinition / tool governance` 主线。
2. 值得吸收的重点应放在产品层，而不是立即替换框架层：
   - terminal/TUI 交互模式
   - permission mode UX
   - slash command 体系
   - plugin/skills 文件约定
   - session/memory 落盘方式
3. 如果真要验证 `open-agent-sdk-go`，建议单开旁路 spike：
   - 只做一个最小 terminal client
   - 不碰现有 web/runtime 主线
   - 验证：
     - tool approval 能力
     - MCP 集成深度
      - 可否承载你需要的 provider/model route 语义
      - 是否支持你后续要做的 `AgentDefinition / task lifecycle / runtime topology`

## 2026-04-02 codeany 自身设计观察补记

### 总体印象
- `codeany` 的价值不只是“调用了 SDK”。
- 它自己的设计重点很明显偏向：
  - 终端产品体验
  - 本地优先的状态与配置组织
  - 低复杂度、够用即可的工程分层
- 它不是一个大而全的平台，而是一个“把 coding agent 产品形态迅速拼完整”的实现。

### 值得关注的设计点

#### 1. CLI / Pipe / Interactive 三入口分得很干净
- `internal/cli/root.go` 把入口分成：
  - 交互 TUI
  - pipe mode
  - print/json mode
- 这种设计很适合 agent 产品：
  - 同一个 runtime
  - 多种使用姿势
  - 不强制每次都走重 UI
- 对 `nekobot` 的启发：
  - 你后续如果补 terminal client，最好不要把 web/runtime 专用入口硬绑在一起。
  - 应该保留：
    - interactive terminal mode
    - non-interactive batch mode
    - machine-readable output mode

#### 2. TUI 状态机设计是“产品导向”的，不追求抽象完美
- `internal/tui/model.go` 的状态很少：
  - `init`
  - `input`

## 2026-04-02 AgentDefinition / context sources 收口补记

### 本轮完成
- `pkg/agent/definition.go`
  - 新增 `AgentDefinition` compatibility bridge。
  - 运行时默认 route / permission mode / tool policy / prompt section 边界现在可被显式读取。
- `pkg/agent/context.go`
  - 新增 `BuildPromptSections()`，把当前 system prompt 拆成稳定 section 和动态 section。
- `pkg/agent/context_sources.go`
  - 新增 `PreviewContextSources()`。
  - 当前会聚合：
    - `project_rules`
    - `skills`
    - `memory`
    - `managed_prompts`
    - `runtime_context`
    - `mcp`
  - 同时返回：
    - resolved system/user prompt 文本
    - preprocessed input
    - 轻量 `footprint` 指标与 warning
    - `budget_status` / `budget_reasons`
    - `compaction` recommendation preview
- `pkg/webui/server.go`
  - 新增 `POST /api/prompts/context-sources`。
- `pkg/webui/frontend/src/pages/PromptsPage.tsx`
  - 新增最小 context sources 预览面板。
  - 支持输入：
    - channel
    - session
    - provider
    - model
    - user message
  - 支持展示：
    - processed input
    - source summary
    - stable/dynamic 标记
    - metadata key/value
    - footprint metrics
    - 轻量 warning
    - budget badge 与 reason 列表
    - compaction 建议卡片

### 当前刻意没做的事
- 没有引入完整的 context budget / token accounting。
- 当前的 `footprint` 只是字符量观测，不驱动真实 runtime 的自动 compaction / pruning。
- 当前 `budget status` 只是基于现有配置做分级提示，不驱动真实 runtime 行为。
- 当前 `compaction recommendation` 只是预览建议，不自动改写历史、memory 或文件注入策略。
- 没有做来源优先级可视化或拖拽排序。
- 没有在 `System` 页或 runtime topology 里重复放第二个入口。
- 没有改写现有 prompt 组装主链，只是复用现有边界做 explainability preview。

### 当前验证
- `go test -count=1 ./pkg/agent ./pkg/webui -run 'Test(PreviewContextSources_IncludesKeySourceTypes|PromptHandlers_ContextSourcesPreview)'`
- `go test -count=1 ./pkg/agent ./pkg/webui ./pkg/prompts`
- `npm run build`（`pkg/webui/frontend`）

### 当前结论
- `AgentDefinition` 和 `context sources` 现在已经形成了一条连续边界：
  - `AgentDefinition` 负责暴露“当前主 agent 是怎么定义的”
  - `context sources` 负责暴露“一次请求实际会吃到哪些上下文来源”
- 后续如果继续做 `context economy`，应直接在这条边界上扩展：
  - 预算
  - 排序
  - explainability 细化
  - 来源治理
  - `querying`
  - `permission`
- 这说明它在终端交互上是刻意压复杂度的。
- 它没有试图把所有 agent 内部生命周期都建模成很细的 domain state。
- 对 `nekobot` 的启发：
  - terminal/TUI 层不要直接照抄 runtime control plane 的复杂状态图。
  - 前端或终端体验层应该是更粗粒度、更面向用户心智的状态机。

#### 3. 输入组件专门为 IME / 中文输入做了取舍
- `internal/tui/input.go` 直接用 `bubbles/textarea` 包装，并把：
  - 中文/IME
  - history
  - 多行输入
  - 自动扩高
  一起处理掉。
- 这比很多 agent 终端只顾英文命令行体验要成熟。
- 对 `nekobot` 的启发：
  - 如果要做 terminal client，中文输入法兼容要一开始就考虑，不然后面很难补。

#### 4. 渲染层很务实
- `internal/tui/render.go` 做了几件很产品化的事：
  - markdown 渲染
  - 超长内容裁剪
  - tool input 的“人类可读摘要”
  - 路径缩短
- 这类逻辑不是框架能力，但非常影响实际可用性。
- 对 `nekobot` 的启发：
  - 你现在 webui 已经有 runtime/task 维度的结构化信息，后续 terminal client 也应该有一层“人类可读摘要器”，而不是直接吐原始 JSON/tool args。

#### 5. slash command 体系非常“产品壳友好”
- `internal/slash/slash.go` 是一个很典型的命令中枢：
  - 命令注册表
  - fuzzy match/autocomplete
  - handler 分发
- 虽然实现不复杂，但对扩展性足够友好。
- 它的问题也很明显：
  - 命令很多
  - 目前比较偏平铺式 switch
  - 长期扩展后会变重
- 对 `nekobot` 的启发：
  - 如果补 terminal command surface，可以先学它的“统一命令索引 + autocomplete + result contract”。
  - 但不要照搬它的大 switch，最好直接做成 registry/command object。

#### 6. skills / plugins / memory 都是文件系统优先
- `internal/skills/skills.go`
- `internal/plugins/plugins.go`
- `internal/memory/memory.go`
- 它们都走：
  - 明确目录约定
  - frontmatter
  - 轻解析
  - prompt 拼装
- 这套模式不复杂，但非常适合终端产品和个人本地使用。
- 对 `nekobot` 的启发：
  - 你不一定要把所有扩展能力都做进数据库。
  - 对 skills / local plugins / prompt fragments / workspace memory，这种“文件系统协议优先”的模式有很高性价比。

#### 7. session 设计简单但够用
- `internal/session/session.go` 没追求事件溯源或复杂 runtime projection。
- 它就是：
  - metadata
  - conversation log
  - 最近会话列表
  - resume
- 这是很典型的“用户体验优先”设计。
- 对 `nekobot` 的启发：
  - 你当前 `tasks/runtime topology/webui` 已经比较系统化。
  - 但如果要补 terminal session UX，完全可以单独做一层轻量会话视图，不必让用户直接面对底层 runtime/task 全量模型。

#### 8. worktree 设计很朴素，但产品闭环完整
- `internal/worktree/worktree.go` 没做复杂调度。
- 但它把：
  - create
  - enter
  - exit
  - remove
  - metadata
  这一整套闭环走通了。
- 对 `nekobot` 的启发：
  - 很多产品能力不需要一开始就做成平台级抽象。
  - 先把“能闭环完成一个真实动作”的最小工作流做出来，价值比抽象漂亮更高。

### 不建议照搬的地方
- 很多模块是“单机终端产品”的合理设计，但不一定适合 `nekobot`：
  - slash command 直接大 switch
  - memory/skills/plugins 完全本地文件导向
  - 权限与交互直接绑在 TUI 主循环里
- `nekobot` 当前已经明显朝：
  - runtime control plane
  - tasks.Service
  - webui + backend API
  - provider/model route
  这个方向发展。
- 所以更合适的做法是吸收其产品层经验，而不是把它的内部组织整体迁过来。

### 最值得吸收的不是“框架”，而是产品工程取舍
1. 用少量状态把终端体验做顺。
2. 把 CLI / pipe / interactive 明确分开。
3. 对输入法、多行输入、工具摘要、会话恢复做足产品细节。
4. 对 skills/plugins/session/memory 采用简单稳定的本地协议。
5. 先完成闭环，再考虑更大的抽象。

## 2026-04-02 codeany 吸收项已纳入计划

### 已确认纳入主计划的两点
1. `tool governance` 阶段补 `permission rules`
   - 目标能力：
     - always allow
     - pattern allow
     - pattern deny
   - 进入 `nekobot` 自己的 config/store/API 与解释链路
2. `AgentDefinition / context economy` 阶段补 `context sources`
   - 目标能力：
     - 展示当前 prompt/context 的来源组成
     - 至少覆盖：
       - skills
       - memory
       - project rules
       - runtime injected context
       - MCP related context

### 当前明确不纳入的内容
- 不新增独立 plugin system 主线。
- 不引入 `open-agent-sdk-go` 替换 `blades`。
- 不把 `codeany` 的 terminal/TUI 设计迁入 `nekobot`。

## 2026-04-02 Task Lifecycle Service 接线收口补记

### 本轮确认的状态
1. `runtime control plane` 这一层已经不只是只读 topology：
   - `/api/status` 和 System 页已能看到：
     - `runtime_states`
     - `session_runtime_states`
     - `current_task_count`
     - `last_seen_at`
     - `availability_reason`
2. 但在这次收口前，`task lifecycle service` 仍主要停留在“定义好了 + 测试好了”，真实执行链并没有正式接入：
   - `subagent` 还是靠 `ListTaskSnapshots()` 暴露状态。
   - 这意味着控制面虽然能看见任务，但生命周期迁移仍不是真正由统一 service 驱动。

### 本轮已完成的代码收口
- `pkg/subagent/manager.go`
  - 新增对共享 task lifecycle service 的挂接。
  - `Spawn()` 时写入 `enqueue`。
  - worker 开始执行时写入 `claim -> start`。
  - 成功完成时写入 `complete`。
  - 执行失败或队列满时写入 `fail`。
  - `CancelTask()` 时写入 `cancel`。
- `pkg/agent/agent.go`
  - `EnableSubagents()` 现在直接为 `subagent` 装配 `tasks.NewService(a.taskStore)`。
  - 不再继续把 `subagent` 作为一个独立 snapshot source 注册到 `taskStore`，避免和 `managed` source 双轨重复。
- 新增/补强回归：
  - `pkg/subagent/manager_test.go`
    - 锁定 subagent 成功/失败路径会驱动 `tasks.Service` 生命周期迁移。
  - `pkg/agent/agent_test.go`
    - 锁定 `EnableSubagents()` 会装配共享 task lifecycle service。

### 当前结论
- 现在 `nekobot` 的后台任务链路终于从：
  - “subagent 自己维护状态 -> store 聚合 snapshot -> WebUI 看见”
- 前进到：
  - “subagent 执行路径 -> `pkg/tasks.Service` 生命周期迁移 -> store / status / WebUI 统一消费”
- 这意味着 `tasks.Service` 已经从纯设计/测试对象，变成真实运行时主链的一部分。

### 收束后的下一步建议
在补完计划索引后，又用 `codeagent` 分别拉了 `codex` 与 `claude` 对当前所有根目录计划文件做了一轮“只允许一个方向”的收束审查。

共识结论：
1. 下一条唯一主线切片是 `exec.background -> process` 接入 `pkg/tasks.Service`。
2. 当前不再把 `chat/main loop` 作为下一步实现目标。
   - 它仍是重要后续方向，但现在推进会和 `exec.background/process`、`AgentDefinition`、`tool governance` 形成多条主线并行，容易把运行态模型再次做散。
3. 当前明确延后：
   - `chat/main loop` task-backed 化
   - `agent tool_session spawn`
   - `cron executeJob`
   - `watch executeCommand`
   - `AgentDefinition`
   - `tool governance`
   - `frontend domain stores`

### 统一后的执行顺序
1. `exec.background -> process`
2. `agent tool_session spawn`
3. `cron executeJob`
4. `watch executeCommand`
5. 之后才回到更大的 `chat/main loop` / `AgentDefinition` / `tool governance`

### 计划治理规则
- 后续每次准备切入新的主线切片前，先用 `codeagent` 做一轮 `codex + claude` 收束审查。
- 该审查只允许产出：
  - 一个推荐的下一切片
  - 一组明确延后项
- 不再接受“当前可以同时做 A/B/C 三条方向”的计划输出。

### 已完成验证
- `go test -count=1 ./pkg/subagent ./pkg/agent ./pkg/webui`
- `go test -count=1 ./pkg/tasks ./pkg/subagent ./pkg/agent ./pkg/runtimeagents ./pkg/runtimetopology ./pkg/approval ./pkg/gateway ./pkg/inboundrouter ./pkg/webui`
- `go test -count=1 ./...`

## 2026-04-02 exec.background -> process 接线补记

### 本轮新增完成的代码收口
- `pkg/process/manager.go`
  - 新增可选 `taskLifecycle` 挂点与 `SetTaskService()`。
  - `StartWithSpec()` 会在 task-aware session 上执行：
    - `enqueue`
    - `claim -> start`
    - prepare/startup 失败时 `fail`
  - `waitForExit()` 会根据最终退出原因统一写入：
    - `complete`
    - `fail`
    - `cancel`
  - `Kill()` / `Reset()` 不直接终结 task，而是只标记 cancel 请求，避免重复终态。
- `pkg/tools/exec.go`
  - `exec.background` 在没有显式 `TaskID` 时，自动用生成出来的 `sessionID` 作为 `TaskID`，确保进入统一 task lifecycle。
- `pkg/agent/agent.go`
  - `Agent` 现在显式持有共享 `taskStore` 与 `taskService`。
  - `processMgr` 与 `subagent` 都复用同一个 `tasks.Service`，避免多个 service 实例对同一个 store 造成状态源冲突。

### 本轮新增回归
- `pkg/process/manager_test.go`
  - `TestManagerStartWithSpecTracksManagedTaskLifecycle`
    - 锁定 process-backed task 会记录 runtime/session/lifecycle timestamps，并最终进入 `completed`。
  - `TestManagerKillCancelsManagedTask`
    - 锁定 `Kill()` 后最终状态为 `canceled`，而不是遗失或双写。
- `pkg/tools/exec_test.go`
  - `TestExecToolBackgroundCreatesManagedTaskWhenTaskIDMissing`
    - 锁定 `exec.background` 在无 `TaskID` 时也会创建一个 managed task，并复用返回给用户的 session 语义。

### 当前结论
- `tasks.Service` 现在已经同时挂到两条真实后台执行链：
  - `subagent`
  - `exec.background -> process`
- 这意味着 `/api/status` / System 页里看到的 active task，不再只是“逻辑上应该能接”，而是已经覆盖到真实 PTY 后台执行路径。
- 现有 `pkg/webui/server_status_test.go` 已经验证 `TypeRuntimeWorker` 任务会影响 runtime `CurrentTaskCount`，因此本轮没有再额外扩一条 process-source 专属状态测试。

### 本轮验证结果
- `go test -count=1 ./pkg/process ./pkg/tools ./pkg/agent`
- `go test -count=1 ./pkg/process ./pkg/tools ./pkg/agent ./pkg/webui`
- `go test -count=1 ./pkg/runtimeagents ./pkg/runtimetopology ./pkg/tasks`
- `go test -count=1 ./pkg/subagent ./pkg/approval ./pkg/inboundrouter ./pkg/gateway`
- `go test -count=1 ./...`
  - 本轮仓库级回归通过，之前记录的 `pkg/gateway` 失败未复现。

### 下一步边界
- 暂时不直接展开：
  - `cron executeJob`
  - `watch executeCommand`
  - `chat/main loop`
  - `AgentDefinition`
  - `tool governance`
- 切下一条主线前，仍先按既定规则做一轮 `codeagent` 收束审查，只选一个方向推进。

## 2026-04-02 exec.background -> process 收口补修

### 审查发现
- 并行 reviewer 发现 `pkg/process/manager.go` 的 `Kill()` 存在一个真实竞态：
  - 旧实现先调用 `Process.Kill()`
  - 再写入 `cancelRequested`
- 而 `waitForExit()` 会根据 `cancelRequested` 来区分最终应写入 `cancel` 还是 `fail`。
- 这意味着在极端时序下，用户主动 kill 的 session 可能被错误记录为 `failed`。

### 本轮补修
- `pkg/process/manager.go`
  - 抽出可替换的 `killProcess` seam，便于测试 kill 顺序。
  - `Kill()` 改为先 `markCancelRequested()`，再发送 kill signal。
- `pkg/process/manager_test.go`
  - 新增 `TestManagerKillMarksCancelBeforeSendingSignal`
  - 直接锁定 kill signal 发出前 `cancelRequested` 已经为真，避免竞态回归。

### 补修后验证
- `go test -count=1 ./pkg/process`
- `go test -count=1 ./pkg/process ./pkg/tools ./pkg/agent ./pkg/webui`
- `go test -count=1 ./pkg/subagent ./pkg/approval ./pkg/gateway ./pkg/inboundrouter`
- `go test -count=1 ./...`
  - 再次复现既有 `pkg/gateway` 间歇性失败：
    - `TestProcessMessageDoesNotEmitInboundWhenRouterHandlesWebsocketChat`
  - 因此当前只能确认本切片相关回归通过，不能声称仓库级基线已经稳定全绿。

### 下一条唯一主线
- 新一轮收束审查的结论是：
  - 下一条唯一主线应为 `agent tool_session spawn -> process -> tasks.Service`
- 原因：
  - 它继续复用已有的 `process.Manager` 生命周期接线
  - 能补齐 agent 创建 tool session 的任务可观测性缺口
  - 不会像 `cron` / `watch` 那样过早把 task 语义拉回 chat/main-loop 或另一套 shell 执行路径

### 明确延后项
- `cron executeJob`
- `watch executeCommand`
- `chat/main loop`
- `gateway/router`
- `AgentDefinition`
- `tool governance`
- `toolsessions.Manager` 内部再造一套 task lifecycle

## 2026-04-01 Claude Code AI Agent Deep Dive V2 补记

### 新增参考范围
- `/home/czyt/code/claude-code/docs/ai-agent-deep-dive-v2.pdf`

### 相比前两轮 Claude Code 文档补充出来的关键点
1. `query` 主循环本身是一个状态机，而不是“调用模型 -> 执行工具 -> 结束”这么简单。
   - 每轮迭代里会经历：上下文预处理、token budget 检查、system prompt 组装、流式响应处理、错误恢复、stop hooks、工具执行、附件注入、下一轮状态转移。
   - 对 `nekobot` 的直接启发是：后续 `task lifecycle service` 之外，还要预留“turn-level runtime state”而不是只记录 task final snapshot。
2. `streaming tool execution` 值得单独进入规划。
   - Claude Code 会在模型还在输出时提前执行已经完整的 tool block。
   - 这对当前 `nekobot` 的 harness / chat / future agent runtime 有直接价值，应在工具治理阶段预留“流式工具调度”插槽，而不是把所有工具执行都假设成 response 完成后的批处理。
3. prompt 不只是静态/动态两段，还包括 `section registry` 和缓存边界约束。
   - 后续 `nekobot` 的 prompt assembler 不能只有“切几个 section 函数”；还要区分：
     - 稳定 section，可缓存。
     - 会话动态 section，可重算。
     - 明确不能缓存的 capability / MCP / runtime-dependent section。
4. 工具系统的目标不是注册表，而是“治理流水线”。
   - PDF 更明确了执行链：schema 校验、validateInput、speculative classifier、pre-hook、hook 权限结果解析、permission 决策、输入修正、tool.call、遥测、post-hook、failure-hook。
   - 当前 `nekobot` 后续设计不能只停在 allow/deny；要按 pipeline 分层预留节点。
5. 多 Agent 体系的重点是“角色切分 + 运行时复用”，不是先堆 agent 数量。
   - `explore/plan/verify` 是同一运行时里的不同定义和权限裁剪。
   - `nekobot` 后续 specialist agent 也应走相同路径：
     - 共用 task runtime。
     - 共用 tool governance。
     - 通过 AgentDefinition 和 permission mode 做差异化。
6. 生命周期管理要覆盖“第二天”的问题。
   - transcript、agent metadata、perf tracing、shell task cleanup、hook cleanup、file state cleanup 都属于正式运行时的一部分。
   - 对 `nekobot` 的启发是：`execenv + daemon substrate` 不应只负责启动，还要定义 cleanup contract。
7. 安全层必须三层互不绕过。
   - classifier 只是辅助，hook 不能绕过 settings deny，permission 决策是最终关口。
   - 这意味着 `nekobot` 的 hooks / approval / permission mode 设计必须从一开始就避免出现“插件直接放行高风险操作”的旁路。
8. 上下文经济学需要专门基础设施，不只是超长后压缩。
   - 轻量压缩、重量压缩、reactive compact、tool result budget、skill/MCP 按需注入都值得进入后续路线。
   - 当前最适合先落的是：session/task summary、tool result summary、prompt stable/dynamic boundary、resume metadata。

### 对现有路线的修正
- `Phase 2` 不能只做 task lifecycle service 的 CRUD 语义，还要明确：
  - runtime claim/report contract。
  - runtime active task projection。
  - 为后续 query/turn 级 state 留出挂点。
- `Phase 4` 的 prompt assembly 需要升级为：
  - section-based assembler。
  - stable/dynamic boundary。
  - capability-aware dynamic sections。
- `Phase 5` 的 tool governance 必须显式包含：
  - classifier/precheck。
  - pre/post/failure hooks。
  - permission decision glue。
  - future streaming tool execution slot。
- `Phase 7` 的 context lifecycle 需要改成更细项：
  - session summary。
  - task summary。
  - transcript compaction policy。
  - tool result budget。
  - reactive compact fallback。

### 当前建议的最小下一步顺序
1. 完成 `task lifecycle service`。
2. 完成 `runtime control plane` 的 status/active tasks/last_seen 视图。
3. 在此基础上抽出 `execenv` 启动与 cleanup contract。
4. 再进入 `AgentDefinition + prompt assembler`。
5. 之后再上 `tool governance pipeline`。
6. `context compaction` 放在工具治理和 specialist agent 之后，但要提前预埋 metadata。

## 2026-04-01 Multica 参考架构补记

### 参考范围
- `/home/czyt/code/multica/server/internal/service/task.go`
- `/home/czyt/code/multica/server/internal/daemon/daemon.go`
- `/home/czyt/code/multica/server/internal/handler/runtime.go`
- `/home/czyt/code/multica/apps/web/features/workspace/store.ts`

### 对当前 `nekobot` 最有价值的设计点
1. `task lifecycle service` 是独立层，而不是散在 handler/manager 里的状态跳转。
   - `enqueue -> claim -> start -> complete/fail/cancel` 语义清楚。
   - runtime claim 和 agent 执行被拆开，适合多 runtime / 多 agent / 后台 worker。
2. `daemon -> execenv -> agent backend` 三层分离。
   - 后台 supervisor 负责领取任务和生命周期。
   - `execenv` 负责工作目录、上下文注入、环境变量、resume/workdir reuse。
   - backend 只负责真正运行 agent CLI/模型。
3. `runtime` 是一等控制面资源。
   - 除了 provider/model，还要有 `status`、`last_seen`、`metadata`、`usage/activity`。
   - 这和 `nekobot` 的 runtime/account/binding 模型天然兼容，适合继续补 runtime health 和 task view。
4. 前端 store 按 domain 拆分。
   - `workspace` store 统一协调 `agents`、`members`、`skills`，并在切换 workspace 时主动清空其他 domain 的 stale state。
   - 对 `nekobot` 的启发是：Chat、Runtime Topology、Tasks、Channels/Accounts、System/Status 不应继续主要靠页面内局部状态拼装。

### 对 `nekobot` 的直接映射
- 应新增中层：`task lifecycle service`。
  - 当前 `tasks.Store` 还是聚合快照容器，不够承担状态转换约束。
- 应新增执行环境层：`execenv`。
  - 后续 watch、background task、daemon worker、resume task 都会需要这层。
- 应增强 runtime 控制面。
  - 继续保留现有 `runtimeagents` / topology 模型，但补 `health/last_seen/usage/activity/current_tasks`。
- WebUI 应开始按 domain 重构状态。
  - 优先顺序：`chat`、`tasks`、`runtimes`、`channels/accounts`、`system status`。

### 不应照搬的部分
- `multica` 的 issue/workspace/task board 产品域不属于当前 `nekobot` 核心目标。
- 当前只吸收其执行模型和控制面组织方式，不迁移其产品对象模型。

### 更新后的优先级
1. session runtime state 收口。
2. task lifecycle service。
3. runtime control plane enhancement。
4. execenv + daemon substrate。
5. AgentDefinition / tool governance / permission mode。
6. frontend domain stores。

## 2026-03-31 Chat Session 桶路由与错误态补记

### 问题确认
- `pkg/webui/frontend/src/hooks/useChat.ts` 之前已经切到 `messagesBySession` 这类 session bucket 结构，但 websocket 回包仍然没有稳定的 `session_id`。
- 前端只能依赖 `pendingSessionKeyRef.current ?? activeSessionKey` 猜测本次响应属于哪个 session。
- 这在以下场景不够稳：
  - 用户切换 runtime 后，上一轮回复晚到。
  - 同一个 websocket 连接快速连续发送不同 runtime 的请求。
  - 错误消息或系统消息在前端切 runtime 之后才返回。
- 同时，`ChatPage.tsx` 对 provider / prompts / runtime topology 查询失败仍然使用默认空数组，视觉上会退化成“像是没有配置”，而不是“请求失败”。

### 已修复
- `pkg/webui/server.go`
  - `chatWSResponse` 新增 `session_id`。
  - `welcome` / `routing` / `pong` / `message` / `route_result` / `file_mentions system` / `clear system` / `error` 全部可携带 `session_id`。
  - 错误路径也按 `baseSessionID` 或本次 `sessionID` 回传明确归属，避免前端把错误打到错误的会话桶里。
- `pkg/webui/frontend/src/hooks/useChat.ts`
  - websocket 入站事件优先使用服务端 `session_id` 决定 `targetSessionKey`。
  - 仅当旧事件没有 `session_id` 时，才回退到 `pendingSessionKeyRef` / `activeSessionKey`。
  - `route_result` / `message` / `error` / `file_mentions` / `system` 都改成 session-scoped 写入。
- `pkg/webui/frontend/src/pages/ChatPage.tsx`
  - provider / prompts / runtimes / channel accounts / account bindings 改成显式 query object 使用。
  - 当 runtime/provider/prompt 查询失败且没有缓存数据时，展示阻断型 error card，而不是空态。
  - prompt session binding 继续跟随 `activeSessionBindingID`，避免 runtime 切换后 prompt chip 漂移到旧 session。
- `pkg/webui/frontend/public/i18n/{en,zh-CN,ja}.json`
  - 新增 chat runtime/provider/prompt load failure 文案。

### 当前语义
- Chat 响应、系统事件和错误事件都可以被服务端显式声明到某个 session。
- 前端 session bucket 不再只靠“当前 UI 正在看哪个 runtime”来推断响应归属。
- 查询失败时，Chat 控制面不会再伪装成“没有 provider / 没有 prompt / 没有 runtime”，而是明确告诉用户当前是加载失败。

### 已完成验证
- `go test -count=1 ./pkg/inboundrouter -run 'TestChatWebsocketFallsBackWithoutTopologyBinding|TestChatWebsocketRejectsDisabledWebsocketAccount|TestChatWebsocketRejectsConfiguredAccountWithoutActiveBindings'`
- `go test -count=1 ./pkg/gateway -run 'TestProcessMessageDoesNotFallbackWhenRouterReturnsEmptyReply|TestProcessMessageDoesNotFallbackWhenExplicitRuntimeFails|TestProcessMessageDoesNotEmitInboundWhenRouterHandlesWebsocketChat|TestProcessMessagePassesExplicitRuntimeIDToRouter'`
- `go test -count=1 ./pkg/webui -run 'TestBuildWebUIChatPromptContextIncludesExplicitRuntimeID|TestHandleUndoChatSession|TestClearChatSessionRemovesUndoSnapshots|TestResolveWebUIRuntimeSelectionUsesRuntimeDefaults|TestResolveWebUIRuntimeSelectionRejectsUnboundRuntime|TestResolveWebUIRuntimeSelectionFallsBackToRequestedRoute'`
- `go test -count=1 ./pkg/webui ./pkg/gateway ./pkg/inboundrouter`
- `npm --prefix pkg/webui/frontend run build`

### 结论
- 这次收口后，WebUI Chat 至少在协议层和前端状态层实现了“服务端声明 session，前端按 session 精确入桶”的一致模型。
- 这也是后续引入 task-backed chat state、background task summary 和多 agent 协作状态栏的必要前置。

## 2026-03-31 Claude Code 扩展能力分层结论

### 适合优先学习并引入的能力
- `Coordinator`
  - 当前 `nekobot` 已经有 subagent / runtime / tool session 雏形，但没有一个真正统一的本地 task coordinator。
  - 这部分应落到 task runtime + inbox/notification，而不是先做远程 agent。
- `Kairos`
  - 当前已有 sessions / notes / learnings / watch / audit，可以自然扩展到“每日追加日志 + 后台压缩/蒸馏”。
  - 这比直接上 remote memory 更符合当前系统阶段。
- `Daemon`
  - 适合承接 background tasks、watch jobs、summarizer、future background verify agents。
- `UDS Inbox`
  - 适合做本地跨进程 IPC，把 gateway/webui/background worker/daemon 串起来，成为后续 teammate/swarm 的本地桥。
- `Auto-Dream`
  - 可以先做成本地 summarizer 调度，不需要 GrowthBook 或远端依赖。

### 适合后续跟进的能力
- `Buddy`
  - 更像前端产品层增强，适合在任务/会话基础设施稳定后作为 UI personality 层引入。
  - 可以挂接到 chat composer / task state / agent persona，但不应抢在 task runtime 前。
- `Bridge`
  - 值得保留抽象接口，但真正跨设备/跨重启控制先放后面。

### 暂不优先的能力
- `Ultraplan`
  - 它依赖远程执行、计划审批与远程会话归档，本地基础设施没稳定之前上这个只会把复杂度提前。

### 推荐落地顺序
1. `Task Runtime + Session State Protocol`
2. `AgentDefinition + Dynamic Tool Assembly + Permission Modes`
3. `Daemon + UDS Inbox + Background Task Supervision`
4. `Kairos-style daily log + Auto-Dream summarizer`
5. `Coordinator + local teammate/swarm prep`
6. `Buddy` 和更强的前端任务体验
7. 最后再看 `Bridge` / `Ultraplan`

## 2026-03-31 Binding 启用态一致性补记

### 问题确认
- `pkg/accountbindings/manager.go` 之前只校验 account/runtime 是否存在、binding mode 是否冲突，但没有约束 `Enabled=true` 的 binding 必须指向 enabled target。
- 这会允许控制面持久化一种“结构存在但执行路径永远不会使用”的半有效状态：
  - Chat/Router 只会使用 enabled runtime/account。
  - Runtime Topology 页面却还能显示这条 binding 处于 enabled。
- 结果是控制面与数据面语义分裂，前端用户很难判断为什么一个“启用的绑定”从不生效。

### 已修复
- `pkg/accountbindings/manager.go`
  - `normalizeBinding(...)` 现在会在 `item.Enabled == true` 时额外加载并检查：
    - `channel_account.enabled`
    - `agent_runtime.enabled`
  - 任何一方为 disabled 都会直接返回明确错误，不再允许写入 enabled binding。
- `pkg/accountbindings/manager_test.go`
  - 既有 CRUD 正向用例显式把 runtime/account/binding 标成 enabled，避免依赖默认值。
  - 新增 `TestManagerRejectsEnabledBindingForDisabledRuntimeOrAccount`，覆盖：
    - enabled binding + disabled runtime => 拒绝
    - enabled binding + disabled account => 拒绝
    - disabled binding + disabled runtime/account => 允许
- `pkg/webui/server_topology_test.go`
  - 新增 `TestHandleCreateAccountBindingRejectsDisabledTargetsWhenEnabled`，锁定 API 请求边界返回 `400`。
  - 同时修正 `TestRuntimeTopologyHandlers_CRUDAndSnapshot` 的基线输入：
    - runtime 显式 `enabled:true`
    - WeChat channel account 显式 `enabled:true`
    - 补齐 `bot_token` / `ilink_bot_id`，使其符合当前 account 校验规则

### 当前语义
- 允许：
  - disabled binding -> disabled runtime/account
  - enabled binding -> enabled runtime + enabled account
- 不允许：
  - enabled binding -> disabled runtime
  - enabled binding -> disabled account

### 影响
- 这次收紧的是控制面写入规则，不改变既有执行路径。
- 但它让 Runtime Topology / WebUI 不再能制造“看起来已启用、实际上永远不工作”的绑定。
- 也顺带暴露出一批旧测试依赖默认 disabled 值的隐式假设，后续相关测试都应显式声明 enabled 状态。

### 已完成验证
- `go test -count=1 ./pkg/accountbindings -run 'TestManagerCRUDAndModeRules|TestManagerRejectsEnabledBindingForDisabledRuntimeOrAccount'`
- `go test -count=1 ./pkg/webui -run 'TestRuntimeTopologyHandlers_CRUDAndSnapshot|TestHandleCreateChannelAccountRejectsEnabledWechatAccountWithoutCredentials|TestHandleCreateAccountBindingRejectsDisabledTargetsWhenEnabled'`
- `go test -count=1 ./pkg/accountbindings ./pkg/webui`
- `npm --prefix pkg/webui/frontend run build`
- `go test -count=1 ./...`

### 前端一并收口
- `pkg/webui/frontend/src/pages/RuntimeTopologyPage.tsx`
  - 新建 binding 时优先默认选 enabled account/runtime；若系统中没有 enabled target，则默认把新 binding 设为 disabled，避免一打开对话框就进入非法状态。
  - 当 binding 处于 enabled 状态时，候选列表只展示 enabled target。
  - 若用户切回 enabled 但当前选中的 target 已 disabled，前端会主动修正到可用 target；若无可用 target，则直接提示并禁用保存。
- `pkg/webui/frontend/public/i18n/{en,zh-CN,ja}.json`
  - 补齐“没有可用于 active binding 的 enabled target”与“active binding 必须指向 enabled target”的三语提示。

### 结论
- 这批修复后，Runtime Topology 的控制面、API 边界与实际执行语义已经对齐：
  - 后端不会再接受半有效 binding。
  - 前端也不会再默认诱导用户创建这类无效组合。

## 2026-03-31 Binding 实际有效态可视化补记

### 问题确认
- 上一批修复已经阻止创建新的“enabled binding -> disabled target”组合。
- 但仍然存在第二类错觉：
  - binding 本身仍存储为 `enabled=true`
  - 后续用户把 runtime 或 channel account 禁用
  - 路由层会直接跳过该 binding
  - Runtime Topology 页面却仍可能把这条 binding 显示为启用
- 这会继续造成“记录仍亮着，但实际不会路由”的控制面误导。

### 已修复
- `pkg/runtimetopology/service.go`
  - `BindingEdge` 新增：
    - `effective_enabled`
    - `disabled_reason`
  - Snapshot 聚合时会基于三层状态计算 binding 的实际有效态：
    - `binding.enabled`
    - `channel_account.enabled`
    - `agent_runtime.enabled`
- `pkg/webui/server_topology_test.go`
  - 新增 `TestRuntimeTopologySnapshotMarksBindingsInactiveForDisabledTargets`
  - 覆盖：
    - 禁用 runtime 后，binding edge 必须变为 `effective_enabled=false`
    - 禁用 account 后，binding edge 必须变为 `effective_enabled=false`
    - 两种情况都必须带 `disabled_reason`
- `pkg/webui/frontend/src/hooks/useTopology.ts`
  - 为 topology edge 补齐 `effective_enabled` / `disabled_reason` 类型。
- `pkg/webui/frontend/src/pages/RuntimeTopologyPage.tsx`
  - binding 卡片状态 pill 现在优先使用 edge 的 `effective_enabled`
  - 卡片 chip 新增失效原因提示
- `pkg/webui/frontend/public/i18n/{en,zh-CN,ja}.json`
  - 补齐三类失效原因文案：
    - `binding_disabled`
    - `account_disabled`
    - `runtime_disabled`

### 当前语义
- binding 记录是否存在、是否被启用：仍由 `binding.enabled` 表达。
- binding 当前是否真的可参与路由：由 `effective_enabled` 表达。
- 如果 `effective_enabled=false`，前端必须显示明确的失效原因，而不是继续复用“开启/关闭”的存储态错觉。

### 已完成验证
- `go test -count=1 ./pkg/webui -run 'TestRuntimeTopologyHandlers_CRUDAndSnapshot|TestHandleCreateAccountBindingRejectsDisabledTargetsWhenEnabled|TestRuntimeTopologySnapshotMarksBindingsInactiveForDisabledTargets'`
- `npm --prefix pkg/webui/frontend run build`

### 待继续验证
- `go test -count=1 ./...`

## 2026-03-31 Chat Runtime Selector 空态补记

### 问题确认
- Chat 页面 runtime selector 当前只展示：
  - `websocket/default` account 下
  - `binding.enabled == true`
  - `runtime.enabled == true`
  - account 自身 `enabled == true`
  的 runtime。
- 这个筛选本身没错，但 UX 上有一个空白区：
  - 如果一个 runtime 都没有，用户看不到是“从未配置 websocket binding”
  - 还是“配置过 binding，但 account/runtime/binding 现在都失效了”
- 两种情况的排查路径完全不同，之前 selector 为空时没有任何说明。

### 已修复
- `pkg/webui/frontend/src/pages/ChatPage.tsx`
  - 新增 `hasRuntimeBindings` 判断。
  - 在 runtime selector 下补两类空态提示：
    - `chatRuntimeEmptyHint`
    - `chatRuntimeUnavailableHint`
- `pkg/webui/frontend/public/i18n/{en,zh-CN,ja}.json`
  - 补齐上述两条三语文案。

### 当前语义
- `chatRuntimeIDs.size == 0`
  - 说明 websocket chat 还没有配置任何 runtime binding。
- `chatRuntimeIDs.size > 0 && chatRuntimes.length == 0`
  - 说明 binding 记录存在，但当前没有任何 active runtime 可用于 Chat。
  - 典型原因是 account/runtime/binding 中至少一层被禁用。

### 已完成验证
- `npm --prefix pkg/webui/frontend run build`

## 2026-03-31 Runtime 禁用影响提示补记

### 问题确认
- Runtime Topology 现在已经能正确显示 binding 的 `effective_enabled` 与失效原因。
- 但在编辑 runtime/account 时，`enabled` 开关旁边原来只有一条泛化提示：
  - disabled entries stay in topology but are excluded from active routing
- 这不足以帮助用户理解两个关键事实：
  - 禁用 runtime/account 会让相关 binding 立即变成 inactive
  - 但 binding record 本身不会被删除

### 已修复
- `pkg/webui/frontend/src/pages/RuntimeTopologyPage.tsx`
  - runtime 编辑弹窗中，当 `enabled=false` 时，切换为 `runtimeTopologyDisableRuntimeHint`
  - account 编辑弹窗中，当 `enabled=false` 时，切换为 `runtimeTopologyDisableAccountHint`
- `pkg/webui/frontend/public/i18n/{en,zh-CN,ja}.json`
  - 补齐两条三语文案，明确描述：
    - related bindings immediately become inactive
    - binding records are retained

### 已完成验证
- `npm --prefix pkg/webui/frontend run build`

## 2026-03-31 Chat Runtime 后端绑定约束补记

### 问题确认
- 之前已经做过两层收口：
  - 前端 Chat runtime picker 只展示 `websocket/default` 下已绑定且 enabled 的 runtime
  - Router 执行路径会在真正路由时按 binding 关系工作
- 但 WebUI chat 服务端的 `resolveWebUIRuntimeSelection()` 仍有一处空档：
  - 只校验 runtime 是否存在且 enabled
  - 不校验它是否真的绑定到 `websocket/default`
- 这意味着只要绕过前端 picker，直接发送 `runtime_id`，后端仍可能接受一个不属于 WebUI chat 面的 runtime。

### 已修复
- `pkg/webui/server.go`
  - `resolveWebUIRuntimeSelection()` 现在会进一步校验：
    - `accountMgr` / `bindingMgr` 可用
    - 存在 `websocket/default` channel account
    - 该 account 当前启用
    - 该 account 的 enabled bindings 中包含目标 runtime
  - 否则统一返回：
    - `runtime <id> is not available for websocket chat`
- `pkg/webui/server_chat_test.go`
  - `TestResolveWebUIRuntimeSelectionUsesRuntimeDefaults`
    - 现在补齐真实的 `websocket/default` account + binding，和新的后端语义对齐
  - 新增 `TestResolveWebUIRuntimeSelectionRejectsUnboundRuntime`
    - 锁定未绑定 runtime 会被服务端拒绝

### 结果
- 现在 Chat 显式 runtime 的约束已经形成闭环：
  - 前端 selector 只显示合法候选
  - 服务端也只接受合法候选
  - Router 最终仍按 topology/binding 关系执行

### 已完成验证
- `go test -count=1 ./pkg/webui -run 'TestResolveWebUIRuntimeSelectionUsesRuntimeDefaults|TestResolveWebUIRuntimeSelectionRejectsUnboundRuntime'`
- `go test -count=1 ./pkg/webui`
- `npm --prefix pkg/webui/frontend run build`

## 2026-03-31 Gateway Websocket 单路径护栏补记

### 调查目的
- 在当前架构里，Gateway websocket 消息会先进入 `processMessage()`。
- 代码里同时存在两条看起来可能重叠的链路：
  - `s.bus.SendInbound(busMsg)`
  - `s.router.ChatWebsocket(...)`
- 需要确认是否存在“同一条 websocket 消息被 runtime 处理两次”的风险。

### 调查结果
- `pkg/inboundrouter/fx.go` 确实会注册 `router.RegisterChannel("websocket")`。
- 但对当前 `gateway.processMessage()` 路径补定向测试后，没有观测到额外的 inbound handler 命中。
- 暂时没有证据表明现状已发生重复执行，因此本批次不改实现，只补护栏测试。

### 已补测试
- `pkg/gateway/server_test.go`
  - 新增 `TestProcessMessageDoesNotEmitInboundWhenRouterHandlesWebsocketChat`
  - 断言当 Gateway 已通过 `router.ChatWebsocket()` 处理 websocket chat 时，不会额外命中 `websocket` inbound bus handler。

### 已完成验证
- `go test -count=1 ./pkg/gateway -run 'TestProcessMessagePassesExplicitRuntimeIDToRouter|TestProcessMessageDoesNotFallbackWhenExplicitRuntimeFails|TestProcessMessageDoesNotEmitInboundWhenRouterHandlesWebsocketChat'`

### 结论
- 这条风险目前没有被测试证实为现网问题。
- 但新增的单路径测试会在后续 Gateway/Channels 运行时重构时提供回归保护。

## 2026-03-31 Chat 显式 Runtime 选路补记

### 问题确认
- 虽然后端已经有 `channel account -> account bindings -> runtimes` 的模型，但 WebUI / Gateway 的聊天链路此前没有显式 runtime 选择入口。
- `pkg/inboundrouter/router.go` 的 websocket chat 选路只会按默认 binding mode 取首个或广播全部 binding，无法表达“这次只发给某个 runtime”。
- WebUI chat 的 session、prompt binding、undo/clear 之前都固定绑定 `webui-chat:<username>`，即使将来前端支持 runtime 选择，也会把不同 runtime 的上下文混在同一个 session 里。

### 已修复
- `pkg/inboundrouter/router.go`
  - `ChatWebsocket(...)` 新增 `runtimeID` 参数。
  - `selectBindings(...)` 支持显式 runtime 过滤；命中后仅选择对应 binding/runtime 对，不再广播到其他 runtime。
- `pkg/gateway/server.go`
  - websocket 消息协议新增 `runtime_id` 字段。
  - `processMessage(...)` 将显式 runtime 原样透传给 router。
- `pkg/webui/server.go`
  - chat websocket 消息新增 `runtime_id`。
  - 新增 `webUIRuntimeChatSessionID(...)`，让 runtime 选择落到独立 session 命名空间。
  - prompt context 的 `Custom["runtime_id"]` 同步写入，供后续 runtime 级上下文链继续复用。
  - `route_result` 回包新增 `runtime_id`，便于前端展示真实选路结果。
- `pkg/webui/frontend/src/pages/ChatPage.tsx`
  - Route Studio 新增 runtime selector。
  - 当前生效路由区域新增 runtime badge。
  - `send / clear / undo / session prompt bindings` 全部改为使用 runtime 作用域 session key。
- `pkg/webui/frontend/src/hooks/useChat.ts`
  - 发送消息与清空会话时支持透传 `runtime_id`。

### 当前语义
- 未显式选择 runtime 时：
  - 保持现有 binding mode 语义，`single_agent` 走首个 binding，`multi_agent` 走全部 enabled bindings。
- 显式选择 runtime 时：

## 2026-03-31 Claude Code 基础能力对标补记

### 已对照的 `claude-code` 设计点
- `src/utils/sessionState.ts`
  - 把会话粗状态 `idle | running | requires_action` 与 richer metadata 解耦：
    - `pending_action`
    - `task_summary`
    - `permission_mode`
- `src/state/store.ts` + `src/state/AppStateStore.ts`
  - 使用很小的 store 把状态变更集中到单一更新出口，便于 UI / bridge / notification 共享同一事实来源。
- `src/services/compact/sessionMemoryCompact.ts`
  - 上下文压缩不是简单“截断消息”，而是带边界、保工具调用配对、保计划片段、保 session memory。
- `src/skills/loadSkillsDir.ts`
  - skills 不只是“全量加载”，还支持：
    - 多来源优先级
    - 去重
    - 条件激活
    - agent/mcp 相关元数据
- `src/services/AgentSummary/agentSummary.ts`
  - 为长时间运行的 sub-agent/后台任务周期性产出 1-2 句 progress summary。

### 对 `nekobot` 当前现状的判断
- 已有基础不错，但核心短板是“缺少统一状态层”：
  - WebUI chat/gateway 主要还是靠消息流和局部布尔值表达状态。
  - session 持久化只有 `summary/source`，没有 `state/pending_action/last_error/runtime/provider` 之类的结构化元数据。
- skills 系统已经有：
  - 多路径发现
  - eligibility
  - registry/install
  - watcher
  - snapshot/version
  但还缺：
  - 条件自动激活
  - 基于当前任务/触达文件/运行时的自动推荐与前置筛选
  - 与 agent runtime / orchestrator 更强的耦合入口。
- memory/context 已有：
  - semantic memory
  - learnings
  - workspace memory
  - 紧急压缩重试
  但还缺：
  - 可观测的压缩边界
  - turn summary / compact summary
  - session 级压缩策略与 UI 呈现。
- subagent 已有最基础队列执行，但离 Claude Code 的协作能力还差很远：
  - 没有显式的 progress summary
  - 没有任务依赖/合流
  - 没有统一的前台/后台状态投影
  - 没有待批准动作/待用户输入状态机。

### 最值得直接引入的能力
1. 显式会话状态协议。
   - 建议为 Chat/Gateway/ToolSession/Subagent 统一增加：
     - `state`
     - `pending_action`
     - `task_summary`
     - `last_error`
     - `runtime_id`
     - `actual_provider`
     - `actual_model`
   - 这是后续所有前端体验和调度能力的底座。

2. WebSocket 生命周期状态机。
   - 现状只有 `connecting/connected/disconnected`，不够表达“临时断线重连中”。
   - 应补：
     - `reconnecting`
     - close reason / retry budget
     - 服务端与前端统一状态说明。

3. 条件 skills 激活与自动推荐。
   - 参考 Claude Code 的 conditional skill 思路，把 skill 触发从“用户显式点名”扩到：
     - 路径匹配
     - channel/runtime/agent 能力匹配
     - 当前任务类型匹配
   - 这对 Claude agent 支持尤其重要，因为 Claude 的工作模式很依赖技能编排。

4. session / subagent 的周期性 summary。
   - harness、多 agent 协作、长时任务都需要这个能力，否则 WebUI 和 channel 侧只能看到“正在跑”，不知道在干什么。

5. 上下文压缩从“紧急 fallback”升级为“可解释策略”。
   - 压缩边界应可记录、可恢复、可被前端解释，而不是只在 provider context overflow 时兜底。

### 对 Claude agent 特别有价值的增强
- 权限模式与 pending action 显式化。
  - 例如 `manual / acceptEdits / fullAuto` 之类的模式，配合 approval manager 做 UI 投影。
- agent profile / capability card。
  - 每个 agent runtime 直接展示：
    - provider/model
    - skills
    - tools
    - prompt
    - approval mode
    - memory/context policy
- 任务型子代理编排。
  - 当前 subagent 更像简单异步任务池。
  - 若要更好支持 Claude agent，应补：
    - task role
    - progress summary
    - result contract
    - merge/integration hook。
- MCP / external tool capability registry。
  - Claude Code 在工具与能力来源组织上更系统。
  - `nekobot` 后续可把 tool session、skills、channel actions、browser/tooling 收口到统一 capability registry。

### 未来可继续集成的好特性
- 远程/计划任务 agent。
  - 类似 Claude Code 的 scheduled remote agents，但先用本地 harness/runtime 版本落地。
- 会话事件历史分页与审计。
  - 不只看消息正文，也看事件流：状态切换、tool call、approval、handoff、compression。
- 文件触达驱动的 skill 激活。
  - 当前最适合与 `watch`、`@file`、workspace memory 串起来。
- agent 协作面板。
  - 主 agent 只是调度入口，其他 agent 作为可选资源显示状态、摘要、结果。
- 配置缓存与变更探测。
  - 适合后续做 Config 热更新、runtime topology 大规模化之后再补。

### 建议优先级
- P0:
  - 会话状态协议。
  - Chat/Gateway WebSocket 重连状态机。
- P1:
  - session/subagent progress summary。
  - 技能条件激活与自动推荐索引。
- P2:
  - 可解释上下文压缩。
  - Claude agent capability registry / profile。
- P3:
  - 计划任务 agent、远程执行、事件流审计面板。

## 2026-03-31 Claude Code 第二轮设计拆解补记

### 用户补充要点的直接结论
- Claude Code 不是“一个顶层常驻 agent + prompt + tools”。
- 它更接近一个运行平台：
  - 入口层。
  - AgentDefinition 层。
  - 动态工具注册层。
  - 状态层。
  - 任务层。
  - swarm / teammate / bridge 扩展层。
- 这意味着 `nekobot` 后续不能只围绕 `pkg/agent.Agent` 继续增肥。

### 对 `nekobot` 架构的直接启发
1. `AgentDefinition` 必须成为一等公民。
   - 至少应声明：
     - tools
     - disallowed tools
     - provider/model
     - permission mode
     - prompt / initial prompt
     - memory policy
     - runtime isolation
     - hooks / MCP / capability dependencies
   - 当前 `runtimeagents.AgentRuntime` 还不够，它更像路由配置，不是完整执行定义。

2. 任务系统必须成为 agent 的承载层。
   - 当前 `subagent` 更像异步队列，不够表达：
     - local agent
     - background agent
     - runtime worker
     - teammate-like worker
     - future remote agent
   - 后续需要统一 task model，而不是每种 agent 生命周期各写一半。

3. permission mode 不应只是 approval 的附属配置。
   - plan mode / verify mode / default mode / restricted mode 这些后续都应成为显式权限上下文。

4. tools 必须按任务动态组装。
   - Claude Code 的工具池是会话状态的一部分，不是静态常量。
   - `nekobot` 当前虽然有 registry，但仍偏静态初始化，后续应能按 agent/task/runtime 筛选。

5. 生命周期治理应视为基础能力。
   - transcript
   - pending action
   - task summary
   - cleanup
   - resume
   - background/foreground switch
   - bridge / approval sync
   这些都应成为统一 runtime 生命周期的一部分。

### 对后续开发顺序的影响
- bug 修复完成后，第一阶段不应该先做“更多 channel 功能”。
- 第一阶段应先做：
  - session/task state 协议
  - AgentDefinition 抽象
  - task-backed execution skeleton
- 第二阶段再做：
  - dynamic tool assembly
  - permission mode
  - built-in specialist agents
- 第三阶段再做：
  - context compaction
  - progress summary
  - teammate/swarm-like runtime

### 明确不再采用的旧思路
- 不继续把所有新能力都挂到一个长期存活的 `*agent.Agent` 实例上。
- 不继续把 `plan/explore/verify` 当作散落 prompt 模板或 WebUI 层按钮逻辑。
- 不继续把 subagent 只当“后台消息队列任务”。
  - 只路由到指定 runtime。
  - WebUI 会把这次对话写入 `route:<runtimeID>:webui-chat:<username>`，避免不同 runtime 共用同一段历史。
- 当前 reply label 逻辑保持不变：
  - 如果命中的 binding/runtime 在多 agent 语义下需要来源标注，回复仍可能带 `[Runtime Name]` 前缀。

### 验证
- `go test -count=1 ./pkg/inboundrouter -run 'TestChatWebsocketFallsBackWithoutTopologyBinding|TestChatWebsocketUsesExplicitRuntimeSelection'`
- `go test -count=1 ./pkg/gateway -run TestProcessMessagePassesExplicitRuntimeIDToRouter`
- `go test -count=1 ./pkg/webui -run 'TestBuildWebUIChatPromptContextIncludesExplicitRuntimeID|TestHandleUndoChatSession|TestClearChatSessionRemovesUndoSnapshots'`
- `go test -count=1 ./pkg/webui ./pkg/gateway ./pkg/inboundrouter`
- `npm --prefix pkg/webui/frontend run build`
- `go test -count=1 ./...`

### 前端验证补充
- 前端仓库当前没有 `npm test` 脚本，无法做独立单元测试；本轮前端验证仍以 `tsc --noEmit + vite build` 为主。

## 2026-03-31 WeChat 多账号运行时凭据隔离补记

### 问题确认
- `pkg/channels/registry.go` 在通过 `BuildChannelFromAccount()` 构造 WeChat account runtime 时，虽然先解了 `channel_account.config`，但真正创建 bot 的 `wechat.NewAccountChannel()` 仍会去 `CredentialStore.LoadCredentials()` 读取“当前激活账号”。
- 这会导致多 WeChat 账号场景下，`wechat:<account_key>` 运行时可能带着别的账号 token / bot id 启动，形成 silent misroute。

### 已修复
- `pkg/channels/wechat/channel.go`
  - `NewChannel()` 继续走 legacy 单账号路径，读取 active store credentials。
  - `NewAccountChannel()` 改为必须显式传入账号凭据，缺少 `bot_token` / `ilink_bot_id` 时直接报错。
  - 抽出共享 `newChannel(...)`，避免重复初始化逻辑。
- `pkg/channels/registry.go`
  - 新增 `decodeWechatAccountCredentials()`，从 `channel_account.config` 独立解码：
    - `bot_token`
    - `ilink_bot_id`
    - `base_url`
    - `ilink_user_id`
  - 显式映射到 `wechat.Credentials`，避免被底层 `baseurl` tag 误伤。
- `pkg/channels/registry_test.go`
  - 新增回归场景：store 中先放一个 active 账号，再构造另一个 account runtime，断言最终 bot id 必须来自 account config，而不是 active store。

### 额外暴露出的流程缺口
- `pkg/webui/server.go` 之前允许创建/更新“已启用但没有 `bot_token` / `ilink_bot_id`”的 WeChat channel account。
- 这类脏数据平时能存进去，但在 `reloadChannelsByType("wechat")` 时会把整条 reload 流程打断。

### 一并补齐
- `pkg/webui/server.go`
  - 为 `handleCreateChannelAccount` / `handleUpdateChannelAccount` 增加 `validateChannelAccountInput()`。
  - 当前先对 enabled WeChat account 执行最小必要校验：必须具备 `config.bot_token` 和 `config.ilink_bot_id`。
- `pkg/webui/server_topology_test.go`
  - 新增 API 级回归，确认 WebUI 会拒绝保存无凭据但 enabled 的 WeChat account。
- `pkg/webui/server_wechat_test.go`
  - 将 reload 相关测试数据补成真实合法的 WeChat account，和新的运行时约束对齐。

### 验证
- `go test -count=1 ./pkg/channels -run TestBuildChannelFromAccount_Wechat`
- `go test -count=1 ./pkg/channels/wechat ./pkg/channels`
- `go test -count=1 ./pkg/webui -run 'TestReloadChannelsByTypePrefersEnabledWechatAccounts|TestHandleCreateChannelAccountRejectsEnabledWechatAccountWithoutCredentials|TestRuntimeTopologyHandlers_CRUDAndSnapshot'`
- `go test -count=1 ./pkg/webui ./pkg/channels/wechat ./cmd/nekobot/...`
- `npm --prefix pkg/webui/frontend run build`
- `go test -count=1 ./...`

## 2026-03-29 Harness 审阅批次

### 审阅范围
- `1b7c3d0` `docs: update harness progress tracker`
- `580741d` `feat: add Turn Undo, @file Mentions, and Watch Mode`
- `46026ac` `feat: add Learnings JSONL system`
- `583245d` `feat(webui): add harness config sections`
- `c409cf1` `feat: add audit log and streaming bash`

### 结合 `yoyo-evolve` 的关键发现

#### 1. `watch` 只有配置和包实现，没有接入运行时
- 现状：
  - `pkg/watch/fx.go` 已经提供了完整 FX 模块和生命周期接线。
  - 但 `cmd/nekobot/main.go`、`tui.go`、`acp.go`、`cron.go`、`service.go` 都没有引入 `watch.Module`。
- 影响：
  - 用户在 ConfigPage 或配置文件里开启 `watch.enabled` 后，功能不会实际运行。
  - 这是典型的“配置面已上线、执行面未接通”问题。

#### 2. `undo` 快照会保存，但工具从未被真实注册到会话
- 现状：
  - `pkg/agent/agent.go` 会在每轮对话前保存 snapshot。
  - 但 `RegisterUndoTool` 只有定义，没有被聊天入口调用。
- 影响：
  - `Undo` 配置和后端存储都存在，但 agent 实际工具集里没有对应工具。
  - 这导致功能对最终用户不可用。

#### 3. audit log 缺少真实 `session_id`
- 现状：
  - `pkg/audit/fx.go` 的 hook 试图从 context 读取 `"session_id"`。
  - `pkg/agent/executeToolCall` 之前没有把 prompt context 的 session id 注入到 tool context。
- 影响：
  - 审计日志无法稳定关联到真实会话，削弱了审计定位价值。

#### 4. ConfigPage 对 harness 配置的前端体验不完整
- 现状：
  - `583245d` 只把 `audit / undo / preprocess / learnings / watch` 新分区挂进左侧导航。
  - 其中 `watch.patterns` 是对象数组，表单模式下只能显示 JSON 预览，必须切 JSON 才能编辑。
- 影响：
  - 对终端用户来说，最关键的 watch 配置仍然不可视化操作。
  - 这和 `yoyo-evolve` 的“直接可用”体验不一致。

### 已落地修复
- 后端：
  - 在 agent 聊天入口按真实 session 动态注册/替换 `undo` 工具。
  - 在 tool 执行上下文中注入 `"session_id"`，让 audit log 能记录真实会话。
  - 给 `tools.Registry` 增加 `Replace`，避免 `undo` 按 session 重注册时 panic。
  - 将 `watch.Module` 接入 CLI / TUI / ACP / Cron / Gateway/service 启动链路。
- 前端：
  - 为 ConfigPage 新增 `WatchSectionForm`。
  - 直接支持编辑 `enabled`、`debounce_ms`、`patterns[]`、`file_glob`、`command`、`fail_command`。
  - 补齐中英日文案。

### 验证
- `go test -count=1 ./pkg/agent ./pkg/watch`
- `go test -count=1 ./cmd/nekobot/...`
- `npm --prefix pkg/webui/frontend run build`

## 2026-03-29 补完未提交的 WeChat / conversationbindings 改动

### 发现的不完整点
- `pkg/conversationbindings/service.go` 已经把底层模型升级成“一个 session 可绑定多个 conversation”，但 WeChat 运行时包装层仍停留在旧接口。
- `pkg/channels/wechat/control.go` 的 `/bindings` 仍按 `Session.ConversationKey` 渲染，所以只能显示主绑定，遗漏同一 runtime 的其他 chat。
- `StopRuntime` / `DeleteRuntime` 也只清理一个 `ConversationKey`，在多绑定场景下会留下悬挂 chat 绑定。

### 已补齐修复
- `pkg/channels/wechat/runtime.go`
  - 增加 `ListBindingRecords(ctx)` 和 `GetBindingsBySession(ctx, sessionID)`，把 `conversationbindings.Service` 的多绑定能力暴露给 WeChat 控制面。
- `pkg/channels/wechat/control.go`
  - `DescribeBindings()` 改为按 binding record 枚举每个 chat -> runtime 的映射。
  - 新增 `clearRuntimeBindings()`，在 `StopRuntime()` / `DeleteRuntime()` 时清理该 runtime 的全部 WeChat chat 绑定，而不是只清一个主绑定。
- 测试补强
  - `pkg/channels/wechat/runtime_test.go` 验证 `ListBindingRecords()` 会返回同一 session 的全部 chat 绑定。
  - `pkg/channels/wechat/control_test.go` 验证 `/bindings` 会列出每个 chat，并验证 stop/delete 后所有 chat 绑定都会被清空。
  - `pkg/conversationbindings/service_test.go` 继续覆盖多绑定、promote remaining binding 等底层行为。

### 补充验证
- `go test -count=1 ./pkg/conversationbindings ./pkg/channels/wechat`

## 2026-03-29 扩展 Harness 对照草案

### 新增对照范围
- `yoyo-evolve` `/watch` 命令语义：
  - `status` / `off` / 自动探测测试命令 / 自定义命令切换。
- `yoyo-evolve` `/undo` 语义：
  - 按 turn 回滚、`/undo N`、`/undo --all`、无 turn history 时的降级提示。
- `yoyo-evolve` `@file` 体验：
  - 行内 `@path` / `@path:start-end` 扩展、邮箱样式跳过、真实文件才注入。
- `yoyo-evolve` audit 可观测性：
  - 记录工具调用、读取最近 N 条、清空日志、参数截断。
- `yoyo-evolve` learnings 闭环：
  - 原始 JSONL 持久化、压缩成 active learnings，并进入提示词上下文。

### 当前初步评估

#### 已整合且基本完整
- `learnings`：
  - `pkg/memory/learnings.go` 已支持 append-only JSONL、active learnings 压缩刷新。
  - `pkg/agent/agent.go` / `pkg/memory/prompt/store.go` 已把 active learnings 注入 prompt 上下文。
  - 这条链路较完整，主要剩可观测性和管理入口增强，不是主缺口。
- audit 基础落地：
  - `pkg/audit/*` 已有 JSONL 写入、最近 N 条读取、清空、统计能力。
  - `pkg/agent/fx.go` + 上轮修复后已可拿到真实 `session_id`。
  - 基础能力存在，但暴露面偏弱。

#### 已整合但不完整
- `undo`：
  - 当前只有 tool 语义，没有 `yoyo-evolve` 那种面向用户的 `/undo` 工作流。
  - `pkg/tools/undo.go` 只支持撤销 1 次，不支持 count / `--all` / 预览 / 无 turn history 时的降级引导。
  - 更关键的是，返回的是说明文本，没有把“回滚后的消息状态”自动接回会话管理层的用户交互闭环。
- `watch`：
  - `pkg/watch/watcher.go` 已有 watcher + debounce + command + fail command + audit。
  - 但当前主要是配置驱动后台运行，缺少类似 `yoyo-evolve` `/watch status|off|<cmd>` 的显式控制/反馈入口。
  - 这意味着功能存在，但用户难以在会话内理解“当前 watch 是否开启、在跑什么命令”。
- `@file` / mentions：
  - `pkg/preprocess/preprocessor.go` 已支持 `@file` / `@dir` 与行范围，基础能力比 `yoyo-evolve` 更强。
  - 但当前主要发生在 agent context build 阶段，缺少 `yoyo-evolve` 那种“明确提示已内联了几个文件”的用户反馈。
  - 因此是功能已整合，但体验闭环偏弱。

#### 适合本轮继续嵌入的候选
- 候选 1：补一个 `watch` 控制/状态入口
  - 风险低，因为底层 watcher 已存在，只需补可见控制面与状态反馈。
- 候选 2：增强 `undo` 为多级参数化工作流
  - 价值高，但需要先确认与现有 session/message 持久化的边界，避免出现“文件回滚了、会话展示没回滚”的双状态不一致。
- 候选 3：给 `@file` 注入补显式反馈
  - 风险低，适合作为体验增强项，尤其适合 WebUI / channel 回包提示。

### 本轮已完成继续嵌入

#### 1. Harness 配置分区改成“真实可持久化”
- `pkg/config/db_store.go` 已将 `audit` / `undo` / `preprocess` / `learnings` / `watch` 纳入 runtime DB sections。
- `pkg/webui/server.go` 已让 `/api/config`、`PUT /api/config`、`POST /api/config/import`、导出配置都完整携带这些分区。
- `pkg/webui/server_config_test.go` 已覆盖 GET/PUT/import 与 `ApplyDatabaseOverrides` 回读闭环。

#### 2. Watch 补成 Web-first 可见控制面
- `pkg/watch/watcher.go`
  - 新增 `Status()` 运行态快照，记录最近一次执行的命令、文件、成功状态、错误和结果预览。
  - 新增 `UpdateConfig()` 用于 Web 层轻量更新运行时配置镜像。
- `pkg/webui/server.go`
  - 新增 `GET /api/harness/watch`
  - 新增 `POST /api/harness/watch`
  - 当前更新策略会持久化配置并刷新 server 持有的 watcher 配置视图，同时明确 `restart_required=true`，不假装已完成完整热重载。
- `pkg/webui/server_status_test.go` / `pkg/watch/watcher_test.go` 已覆盖状态与持久化。

#### 3. Undo 从 tool 语义补成 Web 会话工作流
- `pkg/session/manager.go` 新增 `Session.ReplaceMessages()`，用于安全回写会话消息。
- `pkg/session/snapshot.go` 新增 `MessageSnapshotsToMessages()`，把 undo 快照恢复成运行时消息结构。
- `pkg/agent/agent.go` 暴露 `SnapshotManager()`，供更高层工作流使用。
- `pkg/webui/server.go` 新增 `POST /api/chat/session/:id/undo`
  - 支持 `steps`
  - 按 snapshot store 连续回滚
  - 自动把回滚后的消息写回 session 并返回最新消息列表
- `pkg/webui/server_chat_test.go` 已覆盖多步回滚。

#### 4. `@file` 注入反馈补到 Web Chat
- `pkg/agent/context.go` / `pkg/agent/agent.go`
  - 新增 `PreviewPreprocessedInput()`，把预处理 preview 收束到 agent 边界，避免 Web 层直接依赖 preprocessor 实现细节。
- `pkg/webui/server.go`
  - WebSocket 在真正发起 agent 调用前会发送一个 `system` 事件，并通过 `meta.kind=file_mentions` 携带结构化 feedback。
- `pkg/webui/frontend/src/hooks/useChat.ts`
  - 解析 file mention feedback，存成单独状态。
- `pkg/webui/frontend/src/pages/ChatPage.tsx`
  - 在会话区上方增加 `@file` 反馈卡片，可查看解析路径与 warnings。

#### 5. Chat 页面同步补 Web 体验
- 新增 watch 状态 pill。
- 新增 Undo 按钮，直接走 Web API 回滚当前 chat session。
- composer 区会展示最近一次 watch 命令摘要。
- 补齐中英日文案。

### 本轮验证
- `go test -count=1 ./pkg/watch ./pkg/session ./pkg/agent ./pkg/webui`
- `go test -count=1 ./cmd/nekobot/...`
- `npm --prefix pkg/webui/frontend run build`

## 2026-03-30 Harness WebUI 全面回归补记

### 本轮新增发现
- `pkg/webui/frontend/src/pages/HarnessAuditPage.tsx`
  - 在全新环境、尚无任何 audit 记录时，`/api/harness/audit` 返回 `entries: null`，页面首屏会对 `data?.entries.filter(...)` 直接调用 `.filter()`，导致整个审计页白屏。
  - 这类问题仅靠 `tsc` / `vite build` 不会暴露，必须通过真实浏览器回归才能发现。

### 已修复
- 将审计页的数据归一化为 `const entries = data?.entries ?? []`，所有统计与列表渲染统一基于 `entries` 计算，消除了空数据首屏崩溃。

### 全面回归结果
- 自动化回归
  - `go test -count=1 ./...`
  - `npm --prefix pkg/webui/frontend run build`
- 浏览器级 smoke test
  - 使用临时配置启动完整 `gateway + webui`
  - 覆盖登录后 `Chat -> Harness Audit -> Config(Watch) -> Chat` 主链路
  - 修复审计页空数据崩溃后重复执行，未再出现新的前端运行时错误或页面级问题

### 回归过程中的环境注意点
- 现已修复：CLI `rootCmd.PersistentPreRunE` 在未显式传 `--config` 时不再清空 `NEKOBOT_CONFIG_FILE`。
- 新增命令层测试覆盖：
  - 未传 `--config` 时保留 `NEKOBOT_CONFIG_FILE`
  - 显式传 `--config` 时仍覆盖环境变量
- 实证回归：
  - `env NEKOBOT_CONFIG_FILE=/tmp/nekobot-regression/config.json go run ./cmd/nekobot gateway run`
  - 已确认可直接按环境变量配置把服务起到目标端口并返回正确 `/api/auth/init-status`

## 2026-03-30 Multi-Agent Runtime / Channel Account Round 1 收敛结论

### 现有代码形态观察
- 后端最适合复用的模式是：
  - Ent schema + shared runtime DB。
  - 深模块 manager，例如 `pkg/prompts/manager.go`、`pkg/providerstore/manager.go`。
  - `pkg/webui/server.go` 只保留薄 handler，调用 manager 返回 JSON。
- 当前运行态仍以单一 `*agent.Agent` 和 channel type 配置为中心：
  - `pkg/agent/fx.go` 只注入一个全局 agent。
  - `pkg/channels/registry.go` 的 build 入口仍把单个 agent 直接下发给各 channel。
- 当前 WebUI 结构仍以 `Channels / Config / System` 为中心：
  - 最适合作为 Round 1 承接面的不是重做导航，而是补一个低侵入只读观察页。

### codeagent 辅助分析结论

#### 后端 Round 1 最小可交付切片
- 新增三个深模块：
  - `pkg/runtimeagents`
  - `pkg/channelaccounts`
  - `pkg/accountbindings`
- 每个模块先做：
  - JSON 类型定义。
  - normalize/validate。
  - Ent-backed `Manager`。
  - `List/Get/Create/Update/Delete`。
  - 对应单元测试。
- 新增一个 topology 聚合层：
  - 聚合 runtime/account/binding，输出给 WebUI 和后续 runtime manager 骨架使用。
  - Round 1 只读，不接入真实消息路由。
- Round 1 API 形态：
  - `GET/POST/PUT/DELETE /api/runtime-agents`
  - `GET/POST/PUT/DELETE /api/channel-accounts`
  - `GET/POST/PUT/DELETE /api/account-bindings`
  - `GET /api/runtime-topology`
- 明确延后：
  - WeChat/iLink account-aware 路由。
  - harness 下沉到 agent policy。
  - 多 agent fan-out 行为。
  - 真实 runtime manager 替换 `pkg/agent` 的执行路径。

## 2026-03-30 Gateway / Channels Round 2: 统一入站路由主干

### 关键发现
- 当前系统在这轮之前只有“配置面上的 runtime/account/binding”，没有真正消费这些对象的统一入站执行层。
- `bus` 原模型把 inbound/outbound 都堆在 `ChannelID -> handlers` 单表里，导致：
  - `channels.Manager` 只是在注册 outbound send handler。
  - 许多 channel 的 `SendInbound()` 实际没有统一消费者。
  - `gateway` 更是直接 `agent.Chat(...)`，bus 只做旁路记录。
- 如果不先补统一入站 router，后续继续做多账号 / 多 agent 绑定只会让配置和执行越走越分裂。

### 本轮已落地

#### 1. bus 正式拆分入站/出站 handler 语义
- 文件：
  - `pkg/bus/interface.go`

## 2026-03-31 Runtime Topology WebUI 可编辑化补记

### 本轮新增发现
- `pkg/webui/frontend/src/pages/RuntimeTopologyPage.tsx`
  - 在上一轮只提供了只读 topology 观察视图，但后端其实已经具备完整 CRUD：
    - `/api/runtime-agents`
    - `/api/channel-accounts`
    - `/api/account-bindings`
  - 这造成“运行时拓扑模型已经存在，但 Web 控制面不可操作”的明显断层。
- `pkg/webui/frontend/src/hooks/useTopology.ts`
  - 只有 `GET /api/runtime-topology` 的 snapshot hook，没有独立 list/mutation hook，页面层无法形成稳定的增删改工作流。

### 本轮已完成
- 前端数据层
  - 扩展 `useTopology.ts`：
    - 新增 `useRuntimeAgents`
    - 新增 `useChannelAccounts`
    - 新增 `useAccountBindings`
    - 新增三类资源的 create/update/delete mutation hooks
    - 统一失效 `runtime-topology` 与三类列表 query
- Runtime Topology 页面
  - 保留 summary metric cards。
  - 新增控制面 toolbar：
    - 新建 runtime
    - 新建 account
    - 新建 binding
  - 三个实体区块都补齐：
    - 编辑入口
    - 删除入口
    - 空状态 CTA
  - 新增三个编辑 dialog：
    - runtime 表单：name/display/provider/model/prompt/skills/tools/policy
    - account 表单：channel_type/account_key/display/config/metadata
    - binding 表单：account/runtime/mode/priority/reply_label/public_reply/metadata
  - 新增删除确认 dialog。
- 交互与体验
  - 列表字段采用“每行一项”与 JSON object 文本框混合方案，先保证真实可维护和低耦合。
  - binding 创建按钮在 runtime 或 account 缺失时自动禁用，并给出显式引导提示。
  - API 错误统一从后端 JSON 中提取 `error` 字段后 toast 展示，避免直接把原始 JSON 字符串暴露给用户。
- 文案
  - 补齐 `en / zh-CN / ja` 三语 runtime topology 管理页文案与表单提示。

### 本轮验证
- `npm --prefix pkg/webui/frontend run build`
- `go test -count=1 ./pkg/webui ./pkg/gateway ./pkg/channels ./cmd/nekobot/...`

## 2026-03-31 Runtime Prompt 执行链接通补记

### 本轮新增发现
- `pkg/runtimeagents.AgentRuntime` 已有 `prompt_id` 字段，WebUI 和 CRUD 也都能编辑它。
- 但 routed runtime chat 之前只把 `runtime_id/runtime_name` 等信息放进 `PromptContext.Custom`：
  - `pkg/inboundrouter/router.go` 并没有把 `runtime.prompt_id` 传给 agent。
  - `pkg/agent/resolvePromptSet()` 也没有显式 prompt override 输入。
- 结果是 runtime topology 虽然能配置 prompt，但执行时实际上仍只吃 global/channel/session prompt binding，runtime prompt 配置是“可保存但不生效”。

### 本轮已完成
- `pkg/agent/agent.go`
  - `PromptContext` 新增 `ExplicitPromptIDs`。
  - `resolvePromptSet()` 把显式 prompt ID 传给 prompt manager。
- `pkg/inboundrouter/router.go`
  - routed runtime chat 现在会把 `runtimeItem.PromptID` 转成 `ExplicitPromptIDs` 注入 prompt context。
- `pkg/prompts/types.go`
  - `ResolveInput` 新增 `ExplicitPromptIDs`。
- `pkg/prompts/manager.go`
  - `Resolve()` 现在支持显式 prompt 叠加。
  - 显式 prompt 会在现有 global/channel/session resolve 之后追加进 applied set。
  - 若与已有 prompt ID 重合，则用显式 prompt 覆盖已有应用结果。
  - 排序上仍沿用现有 scope/priority 逻辑，显式 prompt 以 session 级高优先级进入最终 resolved prompt。

### 测试补强
- `pkg/prompts/manager_test.go`
  - 新增 `TestResolveIncludesExplicitPromptIDs`，验证显式 runtime prompt 会进入最终 system prompt。
- `pkg/inboundrouter/router_test.go`
  - 现有 routed runtime test 现在同时验证 `runtime.prompt_id` 会进入 `agent.PromptContext.ExplicitPromptIDs`。

### 本轮验证
- `go test -count=1 ./pkg/prompts ./pkg/inboundrouter`
- `go test -count=1 ./pkg/agent ./pkg/prompts ./pkg/inboundrouter ./pkg/webui ./pkg/channels/wechat`
- `npm --prefix pkg/webui/frontend run build`

## 2026-03-31 WeChat 账户控制面 ID 语义收口补记

### 本轮新增发现
- 旧 WeChat 绑定控制面与新 channel-account 模型在“当前激活账户”的 ID 语义上不一致：
  - `/api/channels/wechat/binding` 的 `active_account_id` 之前返回的是 `bot_id`。
  - 但同一 payload 里的 `accounts[]` 列表返回的是 runtime channel account 的真实 `account_id`。
- 这会导致：
  - 旧控制面与新 topology/account 模型无法用同一个主键对齐。
  - 用户在 Channels 页和 Runtime Topology 页之间切换时，看到的是两套不同的“当前激活账户”标识。

### 本轮已完成
- `pkg/webui/server.go`
  - `buildWechatBindingPayload()` 现在会根据当前激活的 `bot_id` 回填匹配到的真实 channel account ID。
  - `active_account_id` 语义已收口为真正的 `channel_accounts.id`。
- `pkg/webui/frontend/src/hooks/useChannels.ts`
  - WeChat 绑定相关 mutation 在成功后，除了失效旧的 `['channels', 'wechat', 'binding']` 与 `['channels']`，还会同步失效：
    - `['channel-accounts']`
    - `['runtime-topology']`
  - 这样旧 WeChat 控制面动作会立即同步到新的 topology 控制面，不再出现跨页 stale 状态。

### 测试补强
- `pkg/webui/server_channels_test.go`
  - 扩展 `TestBuildWechatBindingPayloadIncludesCurrentBinding`，验证 `active_account_id` 返回真实的 channel account ID，而不是 `bot_id`。

### 本轮验证
- `go test -count=1 ./pkg/webui -run TestBuildWechatBindingPayloadIncludesCurrentBinding`
- `npm --prefix pkg/webui/frontend run build`
  - `pkg/bus/local_bus.go`
  - `pkg/bus/redis_bus.go`
  - `pkg/bus/bus_test.go`
- 变更：
  - 新增 `RegisterInboundHandler` / `UnregisterInboundHandlers`
  - 新增 `RegisterOutboundHandler` / `UnregisterOutboundHandlers`
  - 旧 `RegisterHandler` / `UnregisterHandlers` 保留为 outbound 兼容别名
  - `LocalBus` / `RedisBus` 各自拆成 `inboundHandlers` 与 `outboundHandlers`
- 结果：
  - “谁负责入站消费、谁负责出站发送”终于在 API 层显式区分，不再靠约定猜测。

#### 2. Channel manager 收口为 outbound-only
- 文件：
  - `pkg/channels/manager.go`
- 变更：
  - channel runtime 注册改用 `RegisterOutboundHandler`
  - 停止/重载时改用 `UnregisterOutboundHandlers`
- 结果：
  - `channels.Manager` 的职责被收回到“把 agent reply 发回 channel”，不再和未来的 inbound router 冲突。

#### 3. 新增 `pkg/inboundrouter`
- 文件：
  - `pkg/inboundrouter/router.go`
  - `pkg/inboundrouter/fx.go`
  - `pkg/inboundrouter/router_test.go`
- 能力：
  - 消费 bus inbound message。
  - 按 `channelID -> channel account` 解析：
    - 新增 `channelaccounts.Manager.FindByChannelTypeAndAccountKey()`
    - 新增 `channelaccounts.Manager.ResolveForChannelID()`
  - 按 account 加载 enabled bindings：
    - 新增 `accountbindings.Manager.ListByChannelAccountID()`
    - 新增 `accountbindings.Manager.ListEnabledByChannelAccountID()`
  - 解析 runtime provider/model。
  - 生成 runtime-scoped session：`route:<runtimeID>:<upstreamSessionID>`
  - 用全局 `*agent.Agent` + `PromptContext` 发起真正调用，并将 `runtime_id`、`binding_id` 等 metadata 注入上下文。
  - 通过 `bus.SendOutbound()` 把 reply 再发回具体 channel runtime。
- 当前策略：
  - `single_agent` 只取优先级最高的一条 binding。
  - `multi_agent` 会 fan-out 到全部 enabled binding，并在 reply label/runtime name 上做来源标注。

#### 4. gateway 迁移到 router 主链
- 文件：
  - `pkg/gateway/server.go`
  - `cmd/nekobot/service.go`
- 变更：
  - gateway server 注入 `*inboundrouter.Router`
  - websocket 消息先继续 `SendInbound()` 作为统一入站记录
  - 真正聊天改成优先走 `router.ChatWebsocket(...)`
  - 若当前尚未配置 `websocket/default` 的 account + binding，则保留原有默认 agent 聊天回退
- 结果：
  - 新路由主干已经进入真实入口。
  - 旧行为没有被这轮强制打断，便于后续逐步把 control plane 从“可选配置”推进到“默认执行模型”。

### 测试与验证
- 定向验证：
  - `go test -count=1 ./pkg/inboundrouter ./pkg/gateway ./pkg/bus ./pkg/channels ./pkg/channelaccounts ./pkg/accountbindings`
- 中层回归：
  - `go test -count=1 ./pkg/webui ./pkg/gateway ./cmd/nekobot/...`
- 前端构建：
  - `npm --prefix pkg/webui/frontend run build`
- 全量回归：
  - `go test -count=1 ./...`

### 当前边界与下一步
- 已经打通：
  - bus 语义清晰化
  - runtime/account/binding 第一次真正参与执行
  - gateway 主入口不再绕过 routing spine
- 仍未完成：
  - Telegram / WeChat / ServerChan 等 direct agent call site 还要继续迁。
  - runtime `PromptID` 仍未真正成为 prompt resolution 输入。
  - WebUI 目前只有 topology 观察页，没有 account/binding/runtime 的完整交互编辑体验。

## 2026-03-31 Gateway / Channels Round 2 收口

### 新结论
- 昨天判断里把 Telegram 也列进“仍直接调全局 agent”的集合并不准确：
  - 它确实还在 channel 内直接调 agent，但其 session/回包结构比 WeChat 更容易平滑迁移。
  - 真正本轮最值得优先处理的是让 Telegram / WeChat 的普通聊天统一走 `SendInbound -> inboundrouter -> SendOutbound`。
- `ServerChan` 与其说是聊天 channel，不如说更偏通知/命令入口；本轮不把它当成 runtime routing 的 blocker 更合理。

### 本轮已落地

#### 1. Router 增加 legacy channel fallback
- 文件：
  - `pkg/inboundrouter/router.go`
  - `pkg/inboundrouter/router_test.go`
- 新行为：
  - 若 `ResolveForChannelID()` 找不到对应 account，不再直接丢弃 inbound message。
  - 改为走 legacy fallback：
    - 使用原 sessionID 获取 session。
    - 用原 channel/session/user prompt context 调全局 agent。
    - 将 reply 经 `bus.SendOutbound()` 发回 channel。
- 价值：
  - channel 现在可以先统一改成发 inbound bus，而不必要求用户先手工建 account/binding/runtime 拓扑。

#### 2. Telegram 普通聊天切到 `bus + router`
- 文件：
  - `pkg/channels/telegram/telegram.go`
- 变化：
  - 普通消息不再在 `handleMessage()` 里直接 `agent.ChatWithPromptContext(...)`。
  - 改为：
    - 先生成 thinking message。
    - 构造 `bus.Message`。
    - 将已套 profile 的输入写入 `busMsg.Content`。
    - 通过 `busMsg.Data` 透传：
      - `thinking_message_id`
      - `reply_to_message_id`
    - `SendInbound()` 交给 router 处理。
- 结果：
  - Telegram 聊天主链已进入统一 routing spine。
  - 现有 thinking/回复体验仍可保留给 outbound path 消费。

#### 3. WeChat 普通聊天切到 `bus + router`
- 文件：
  - `pkg/channels/wechat/channel.go`
  - `pkg/channels/wechat/channel_auth_test.go`
- 变化：
  - 普通聊天在 command / pending interaction / runtime control 分支之后，不再直接 `agent.ChatWithPromptContext(...)`。
  - 改为构造 `bus.Message` 并 `SendInbound()`。
  - 通过 `bus.Message.Data["context_token"]` 保留 WeChat 回复上下文。
  - `SendMessage()` 改为从 message data 中提取 `context_token` 再回发。
  - 补了 `messageContextToken()` helper 测试。
- 结果：
  - WeChat 普通聊天也纳入统一 router 主链。
  - `context_token` 没因为架构迁移而丢失。

#### 4. 全量回归顺手打出并修掉 `watch` 生命周期竞态
- 文件：
  - `pkg/watch/watcher.go`
- 发现：
  - `TestWatcherCanRestartAfterStop` 在全量回归时真实触发 panic：
    - `eventLoop()` 运行中持续解引用 `w.fsWatcher`
    - `Stop()` 会把 `w.fsWatcher = nil`
    - goroutine 尚未退出时出现 nil-pointer dereference
- 修复：
  - `Start()` 启动 event loop 时，捕获 `ctx` 与 `fsWatcher` 局部快照。
  - `eventLoop(ctx, fsWatcher)` 生命周期内只使用这两个快照，不再回读共享指针。
- 结果：
  - `watch` restart / stop / start 主链现在可稳定回归。

### 回归结果
- 定向：
  - `go test -count=1 ./pkg/inboundrouter ./pkg/channels/telegram ./pkg/channels/wechat`
- 主链：
  - `go test -count=1 ./pkg/watch ./pkg/inboundrouter ./pkg/channels ./pkg/gateway ./pkg/webui ./cmd/nekobot/...`
- 前端：
  - `npm --prefix pkg/webui/frontend run build`
- 全量：
  - `go test -count=1 ./...`

### 当前剩余点
- WeChat / Telegram 的 command / interaction / runtime-control 分支虽然还没统一进 router，但已经不再阻塞普通聊天主链。
- 还需要继续评估：
  - 哪些 channel 真正值得建成“多账号 + runtime binding”的持续对话通道。
  - 哪些像 ServerChan 这类更适合作为通知/命令型 channel，应该保持更轻的控制面模型。

#### 前端 Round 1 最小承接面
- 不重构 `Channels / Config / System` 主体。
- 新增一个轻量 `Runtime Topology` 页面最合适。
- 页面内容只做只读观察：
  - runtimes 列表。
  - channel accounts 列表。
  - bindings 关系图/关系卡片。
  - 顶部 summary metric。
- 推荐最小改动面：
  - `App.tsx` 新增 route。
  - `Sidebar.tsx` 新增单个导航项。
  - `hooks/useTopology.ts` 或在 `useConfig.ts` 补 topology query hook。
  - 新增 `pages/RuntimeTopologyPage.tsx`。
  - 补少量 i18n key。
- 明确延后：

## 2026-03-30 Multi-Agent Runtime / Channel Account Round 2 首批运行链落地

### 本轮目标
- 把 `pkg/channels` 从“一个 channel type 对应一个 runtime 实例”推进到“一个 channel type 可承载多个 account runtime 实例”。
- 保留现有 `config.Channels.*` 单实例兼容路径，不中断旧 WebUI/配置流。
- 至少让一类简单 channel 和一类完整聊天 channel 真正能从 `ChannelAccount` 启动。

### 已完成实现

#### 1. `pkg/channels.Manager` 改成双索引
- `channels map[id]Channel` 之外，新增：
  - `channelsByType map[channelType][]instanceID`
  - `defaultByType map[channelType]instanceID`
- 效果：
  - `Register` 允许 `telegram:alpha`、`telegram:beta` 并存。
  - `GetChannel("telegram")` 仍能取到默认实例，兼容旧控制面与旧测试。
  - `StopChannel("telegram:alpha")` 后，type 级默认别名会自动切到剩余实例。

#### 2. 引入 `TypedChannel` 与 account-aware builder
- `pkg/channels/channel.go` 新增 `TypedChannel`，把逻辑 channel family 和 runtime instance ID 区分开。
- `pkg/channels/registry.go` 为 descriptor 增加 `buildFromAccount(...)`。
- 新增 `BuildChannelFromAccount(...)`：
  - 先把 `ChannelAccount.Config` JSON 解到各 channel config struct。
  - 再按 `channel_type:account_key` 生成实例 ID。

#### 3. `RegisterChannels` 优先注册 `ChannelAccount`
- 若 runtime DB 中存在启用的 `ChannelAccount`：
  - 优先用 `BuildChannelFromAccount` 创建实例并注册到 manager。
- 若某 channel type 没有 account 记录：
  - 继续按旧 `config.Channels.<Type>` 路径构建默认实例。
- 当前保持的是“新主链优先、旧单实例兜底”的第二轮兼容模型。

#### 4. 样板适配已接通 `gotify` 与 `telegram`
- `gotify`
  - 适合作为低风险出站型样板。
  - 现在支持 account 实例 ID / 名称。
- `telegram`
  - 适合作为完整聊天链样板。
  - 已支持 account 实例 ID。
  - 会话 ID 从默认的 `telegram:<chatID>` 扩展为 account 模式下的 `<instanceID>:<chatID>`。
  - 用户偏好存储改为：
    - 默认实例继续用旧 key。
    - account 实例使用 `<instanceID>:<userID>`，避免多账号互相污染偏好状态。
  - `extractChatID` / `extractMessageID` 已兼容默认实例和 account 实例格式。

#### 5. WebUI 增加最小运行态可见性
- `GET /api/channels` 现保留原有 `{ [channelType]: config }` 结构。
- 同时新增 `_instances` 字段，返回当前已注册的 runtime instances：
  - `id`
  - `type`
  - `name`
  - `enabled`
- `ChannelsPage` 已新增 `Runtime instances` 区块，显示 account/runtime 实例，不改变现有 channel 配置编辑流。

### 测试与验证
- 目标测试：
  - `go test -count=1 ./pkg/channels ./pkg/webui`
  - `npm --prefix pkg/webui/frontend run build`
  - `go test -count=1 ./...`
- 已通过：
  - manager 多实例/默认别名测试。
  - `BuildChannelFromAccount` for `gotify` / `telegram`。
  - `RegisterChannels` 优先 account、缺省回退 legacy config。
  - `/api/channels` 返回 `_instances` 的 WebUI handler 测试。
  - 前端构建通过，Channels 页实例可见性已接入。

### 仍未完成的第二轮后续项
- `wechat` 仍是旧 iLink 单活账号模型，没有真正下沉到 `ChannelAccount` 主链。
- 绝大多数 channel 还未迁到 `buildFromAccount`，当前只完成 `gotify` / `telegram` 样板。
- `AccountBinding` 还没有驱动真实消息路由与 agent runtime 选择；本轮主要完成的是 channel runtime/account 化的启动层与可见性层。

## 2026-03-30 下一轮首段：WeChat 绑定控制面桥接到 ChannelAccount

### 发现的问题
- 第二轮结束后，WeChat 仍有一个关键断层：
  - WebUI 绑定接口写的是 `ilinkauth` 用户绑定。
  - channel runtime 启动时读的却是旧单活绑定模型。
  - 一旦系统里出现多个绑定用户/账号，旧 `loadChannelBinding()` 会因为“多绑定不支持”直接失败。

### 本轮已完成修复

#### 1. WeChat runtime 改成从 `CredentialStore` 读取当前激活账号
- `pkg/channels/wechat/channel.go`
  - 不再通过 `ilinkauth.ListBindings()` 假定系统里只能有一个唯一绑定。
  - 改为初始化 `CredentialStore`，并从其中读取当前激活 credentials。
  - `newWeChatBot()` 的 sync-state 也改为使用 `CredentialStore.SyncStatePath(botID)`。
- 结果：
  - WeChat channel runtime 不会因为存在多用户/多账号绑定记录而直接失效。
  - 当前激活账号的选择开始和 channel 侧本地账号存储对齐。

#### 2. WebUI 绑定确认后同步落到 `ChannelAccount`
- `pkg/webui/server.go`
  - `handlePollWechatBinding()` 在 iLink 确认成功后，会额外执行 `syncWechatBindingToAccounts(...)`。
  - 该步骤会：
    - 把 credentials 同步写入 WeChat `CredentialStore` 并设为 active。
    - 在 runtime DB 中按 `channel_type=wechat + account_key=ILinkBotID` upsert 一个 `ChannelAccount`。
    - `metadata.owner_user_id` 标记当前绑定用户，便于控制面筛选。

#### 3. 恢复 WeChat 多账号切换/删除接口
- `handleActivateWechatBinding()`：
  - 不再返回“multiple wechat accounts are no longer supported”。
  - 现在会：
    - 校验该 `ChannelAccount` 属于当前用户。
    - 把它设为 WeChat `CredentialStore` 的 active account。
    - 重建当前用户的 `ilinkauth.Binding` 指向该账号。
    - 热重载 `wechat` channel。
- `handleDeleteWechatBindingAccount()`：
  - 现在会：
    - 删除指定 `ChannelAccount`。
    - 删除对应 WeChat 本地 credentials/sync-state。
    - 若它正是当前 active 绑定，则清理当前 binding 并自动切换到剩余账号中的第一个。
    - 热重载 `wechat` channel。

#### 4. 绑定状态接口开始返回真实 account 列表
- `buildWechatBindingPayload()` 现在会基于 `ChannelAccount` 返回当前用户名下的 WeChat accounts。
- `active_account_id` 重新变成真实字段。
- 前端现有 WeChat binding 卡片无需重构，即可继续展示多账号列表、激活与删除操作。

### 本轮验证
- 定向回归：
  - `go test -count=1 ./pkg/webui ./pkg/channels/wechat -run 'TestHandleWechatBindingLifecycle_UsesSharedIlinkAuth|TestHandleWechatBindingActivateAndDeleteAccount|TestHandleGetWechatBindingStatus_NoBinding|TestNewCredentialStoreLoadsActiveCredentials|TestNewCredentialStoreReturnsNilWithoutStoredCredentials'`
- 扩大验证：
  - `go test -count=1 ./pkg/channels ./pkg/webui`
  - `npm --prefix pkg/webui/frontend run build`
  - `go test -count=1 ./...`

### 仍待继续的下一段
- WeChat 目前虽然已经把绑定控制面和 runtime 账号存储桥接起来，但 channel runtime 仍以默认 `wechat` 实例存在。
- 下一步仍需把 WeChat 接入 `BuildChannelFromAccount` / account runtime 实例化主链，真正做到一个 channel type 对应多个并存 WeChat runtime 实例。

## 2026-03-31 下一轮第二段：WeChat 接入 account-aware runtime builder

### 本轮新增完成
- `pkg/channels/wechat/channel.go`
  - 新增 `NewAccountChannel(...)`。
  - `Channel` 现具备：
    - `id`
    - `channelType`
    - `name`
  - 默认 `NewChannel(...)` 现在只是 `NewAccountChannel(..., "wechat", "WeChat")` 的兼容包装。
- WeChat session namespace 已具备 account runtime 语义：
  - 默认实例仍为 `wechat:<userID>`。
  - account 实例变为 `wechat:<accountKey>:<userID>`。
- `pkg/channels/registry.go`
  - `wechat` descriptor 已新增 `buildFromAccount(...)`。
  - 现在可从 `ChannelAccount` 直接构建 `wechat:<accountKey>` runtime instance。

### 本轮验证
- 新增测试：
  - `BuildChannelFromAccount_Wechat`
  - WeChat session id 在默认实例 / account 实例下的前缀行为
- 已通过：
  - `go test -count=1 ./pkg/channels ./pkg/channels/wechat`
  - `go test -count=1 ./pkg/webui ./pkg/channels ./pkg/channels/wechat`
  - `npm --prefix pkg/webui/frontend run build`

### 当前剩余问题
- 虽然 WeChat 已能从 `ChannelAccount` 构建 runtime instance，但控制面和热重载路径仍主要围绕默认 `wechat` 兼容别名。
- 后续需要继续把 WeChat 的控制/消息路由按具体 runtime instance 落实，而不是只做到“可以构建多个实例”。
  - Agents / Channel Accounts / Bindings 独立管理页。
  - 编辑表单。
  - Chat runtime selector。
  - Harness runtime-scoped 配置页。

## 2026-03-30 Round 2/3 融合修补：Slack account runtime + manager reload 健壮性

### 本轮新增发现
- Slack 虽然已经有 `id/channelType/name` 字段和 account-aware 构造器，但仍残留单实例假设：
  - `parseSessionID()` 只会把前缀后的第一段当成 channel ID，无法正确处理 `slack:team-a:C123[:thread]`。
  - slash command / skill install confirm 请求虽然已改用 `Channel: c.ID()`，但 metadata 里还缺 runtime 维度，后续排障不够直接。
- `pkg/channels.Manager` 的多实例索引还有两个真实缺口：
  - `ReloadChannel()` 替换 runtime 后没有回填 `channelsByType/defaultByType`，alias 在 reload 后可能漂移。
  - `bus == nil` 时 `Start/Stop/StopChannel/ReloadChannel` 直接调用 bus handler 注册/注销，会在测试与纯 registry 路径下崩溃。
- WeChat 控制面在激活账号时虽然已经完成 `ChannelAccount + CredentialStore + ilinkauth` 同步，但热重载仍走默认 `wechat` 兼容别名，不够精确。

### 已完成修复
- `pkg/channels/slack/slack.go`
  - 保持 account-aware runtime 结构继续收口。
  - 修复多段 runtime ID session 解析。
  - slash command / skill install confirm metadata 新增 `runtime_id`。
- `pkg/channels/registry.go`
  - Slack 已确认接入 `buildFromAccount` 主链，可由 `ChannelAccount` 直接构建 `slack:<account_key>` runtime。
- `pkg/channels/manager.go`
  - `ReloadChannel()` 现在会恢复 type 索引与默认 alias。
  - 所有 bus handler 注册/注销路径都对 `nil bus` 做了防护。
- `pkg/webui/server.go`
  - 新增 `reloadChannelForAccount()`。
  - WeChat 激活账号后优先精确重载对应 account runtime；找不到实例时再退回 type 级兼容重载。

### 新增测试
- `pkg/channels/manager_test.go`
  - `TestManagerReloadChannelRestoresTypeAliasIndex`
  - `TestManagerStopChannelWithoutBusDoesNotPanic`
- `pkg/channels/slack/slack_test.go`
  - `TestAccountChannelUsesRuntimeScopedIdentifiers`
- `pkg/channels/registry_test.go`
  - `TestBuildChannelFromAccount_Slack`

### 本轮验证
- `go test -count=1 ./pkg/channels/... ./pkg/webui -run 'TestManagerReloadChannelRestoresTypeAliasIndex|TestBuildChannelFromAccount_Slack|TestAccountChannelUsesRuntimeScopedIdentifiers|TestHandleGetChannelsIncludesRuntimeInstances|TestHandleWechatBindingActivateAndDeleteAccount'`
- `go test -count=1 ./pkg/channels ./pkg/channels/slack ./pkg/channels/wechat ./pkg/webui`
- `npm --prefix pkg/webui/frontend run build`
- `go test -count=1 ./...`

## 2026-03-31 下一轮第三段：Slack account-aware runtime 与 WeChat type-level reload 收口

### 本轮新增完成

#### 1. Slack 接入 account-aware runtime builder
- `pkg/channels/slack/slack.go`
  - 新增 `NewAccountChannel(...)`。
  - `Channel` 现具备：
    - `id`
    - `channelType`
    - `name`
  - 默认 `NewChannel(...)` 现在只是 `NewAccountChannel(..., "slack", "Slack")` 的兼容包装。
- `pkg/channels/registry.go`
  - `slack` descriptor 已新增 `buildFromAccount(...)`。
  - 现在可从 `ChannelAccount` 直接构建 `slack:<accountKey>` runtime instance。

#### 2. Slack 聊天与命令路径改为 runtime-aware
- 入站消息：
  - `handleMessageEvent()` / `handleAppMentionEvent()` 现在使用 `c.ID()` 作为 `bus.Message.ChannelID`。
  - session namespace 从固定 `slack:<channel>` 扩展为：
    - 默认实例：`slack:<channel>`
    - account 实例：`slack:<accountKey>:<channel>`
    - thread 场景继续附带 `:<thread_ts>`。
- 命令执行：
  - slash command 和 skill-install confirm 的 `commands.CommandRequest.Channel` 改为 `c.ID()`。
  - metadata 补 `runtime_id`，便于后续统一路由/审计。
- 出站发送：
  - `parseSessionID()` 改为同时兼容默认前缀和 account runtime 前缀。

#### 3. WeChat 绑定控制面改为按 channel type 重载 account runtimes
- `pkg/webui/server.go`
  - 新增 `reloadChannelsByType(channelType)`：
    - 若存在启用的 `ChannelAccount`，逐个 `BuildChannelFromAccount + ReloadChannel`。
    - 若存在旧 legacy 默认 runtime，则清掉 legacy runtime，避免与 account runtimes 并存误导控制面。
    - 若该 type 没有 account 记录，则回退到旧 `reloadChannel(channelType)`。
- WeChat 绑定相关 handler 已统一改用 `reloadChannelsByType("wechat")`：
  - `handlePollWechatBinding()`
  - `handleDeleteWechatBinding()`
  - `handleActivateWechatBinding()`
  - `handleDeleteWechatBindingAccount()`

### 本轮验证
- 新增测试：
  - `BuildChannelFromAccount_Slack`
  - Slack account runtime session / command namespace tests
  - `ReloadChannelsByTypePrefersEnabledWechatAccounts`
- 已通过：
  - `go test -count=1 ./pkg/channels/slack ./pkg/channels/wechat ./pkg/webui -run 'Test(BuildChannelFromAccount_Slack|ParseSessionIDSupportsAccountRuntimePrefix|HandleMessageEventUsesAccountRuntimeIdentifiers|ExecuteConfirmedSkillInstallUsesRuntimeChannelID|HandleWechatBindingLifecycle_UsesSharedIlinkAuth|HandleWechatBindingActivateAndDeleteAccount|ReloadChannelsByTypePrefersEnabledWechatAccounts|GetWechatBindingStatus_NoBinding)'`
  - `go test -count=1 ./...`
  - `npm --prefix pkg/webui/frontend run build`

### 本轮后仍保留的边界
- WeChat 当前更像“active account + type 级重载”的桥接模型：
  - 控制面和运行图已经能按多个 `ChannelAccount` 重建实例。
  - 但入站实际消费仍主要依赖当前 active credentials，而不是并发消费多个 WeChat 账号事件流。
- Slack 已完成 account-aware runtime 样板，但 shortcut / modal submission 业务闭环仍未继续扩展。

### Round 1 实现原则
- 先跑 TDD：
  - manager 行为测试先行。
  - server handler 测试先行。
- 先做存储和聚合，再补 API，再补最小前端。
- WebUI 首轮的目标是“truthful visibility”，不是“完整运营面”。
- 用户新约束：
  - 多账号不是 WeChat 特性，而是全 channel 通用能力。
  - 因此 `ChannelAccount` 不能绑定到 WeChat 数据结构；Round 2 只是先接 WeChat adapter，不是只支持 WeChat。

### Round 1 已落地实现
- 新增 Ent schema：
  - `pkg/storage/ent/schema/agentruntime.go`
  - `pkg/storage/ent/schema/channelaccount.go`
  - `pkg/storage/ent/schema/accountbinding.go`
- 新增数据与规则模块：
  - `pkg/runtimeagents`
  - `pkg/channelaccounts`
  - `pkg/accountbindings`
- 新增 topology 聚合层：
  - `pkg/runtimetopology/service.go`
- 新增 WebUI 后端接口：
  - `/api/runtime-agents`
  - `/api/channel-accounts`
  - `/api/account-bindings`
  - `/api/runtime-topology`
- 新增 WebUI 最小观察面：
  - `pkg/webui/frontend/src/hooks/useTopology.ts`
  - `pkg/webui/frontend/src/pages/RuntimeTopologyPage.tsx`
  - `App.tsx` route
  - `Sidebar.tsx` 导航入口

### Round 1 过程中顺手收口的历史问题
- 新基础层已接入 CLI / ACP / TUI / Cron / Service 的 FX 图，避免后续第二轮开始时又出现“只有 WebUI 看得到、运行图里没有”的历史包分裂。
- `ChannelAccount` 字段保持 channel-agnostic，避免重演过去 WeChat runtime/store/control 把通用模型和单通道模型揉在一起的问题。

### Round 1 环境问题
- `go generate ./pkg/storage/ent` 失败：
  - 原因：环境里没有 `ent` 二进制。
  - 处理：改用 `go run entgo.io/ent/cmd/ent generate ./pkg/storage/ent/schema`。
- 首次 `go run entgo.io/ent/cmd/ent generate` 失败：
  - 原因：缺少 `ent` 命令依赖的 `go.sum` 记录（`github.com/olekukonko/tablewriter` 等）。
  - 处理：执行 `go get entgo.io/ent/cmd/ent@v0.14.5` 补齐生成期依赖，再重新生成。
- 前端回归中一度把 i18n 文件误替换成只含新增键的小文件：
  - 处理：从 `HEAD` 恢复完整字典，再以最小增量追加 `Runtime Topology` 文案，随后重新构建通过。

### Round 1 验证结果
- `go test ./pkg/runtimeagents ./pkg/channelaccounts ./pkg/accountbindings ./pkg/runtimetopology`
- `go test ./pkg/webui -run 'TestRuntimeTopologyHandlers_CRUDAndSnapshot|TestPromptHandlers_CRUDAndResolve'`
- `go test ./cmd/nekobot/... ./pkg/webui ./pkg/runtimeagents ./pkg/channelaccounts ./pkg/accountbindings ./pkg/runtimetopology`
- `npm --prefix pkg/webui/frontend run build`
- `go test -count=1 ./...`

## 三项目对比总览

| 维度 | nekobot (当前) | picoclaw (Go, 灵感源) | nextclaw (TS, 参考) |
|------|---------------|----------------------|---------------------|
| 语言 | Go | Go | TypeScript (Node.js) |
| Agent Loop | 简单迭代 | 成熟迭代+压缩+KV缓存 | 成熟迭代+budget pruning |
| Provider Failover | ClassifyError+CircuitBreaker | FallbackChain+Cooldown+ErrorClassifier | ProviderManager+pool cache |
| Skill System | 6层优先级加载+eligibility | 3层+frontmatter+requirement | workspace+builtin+marketplace |
| Cron | robfig/cron, 5min超时 | 自写ticker, 原子持久化 | setTimeout, JSON持久化 |
| Memory | 2层(long-term+daily) | 2层+context格式化 | 2层+daily+workspace |
| Session | JSON文件 | JSON+atomic+sanitize | JSONL+normalize |
| Context Cache | 无 | mtime+existence tracking | 无 |
| Subagent | ToolSessionTool(spawn) | SubagentManager(async+sync) | SubagentManager(steer+cancel) |
| Approval | 3模式(auto/prompt/manual) | 无 | 无 |
| Tool Session | 完整(PTY+access+lifecycle) | 无 | 无 |

---

## 1. 架构稳健性与鲁棒性

### 1.1 picoclaw 的优秀模式 (可直接借鉴)

#### A. Provider Fallback Chain (推荐迁移)
**文件**: `/home/czyt/code/go/picoclaw/pkg/providers/fallback.go`

nekobot 已有 `ClassifyError` + `CircuitBreaker`，但 picoclaw 的实现更成熟：

- **Cooldown 指数退避**: `min(1h, 1min * 5^min(n-1, 3))`
  - 1次失败→1min, 2次→5min, 3次→25min, 4+次→1h
  - Billing 错误单独策略: `min(24h, 5h * 2^min(n-1, 10))`
- **24小时错误窗口**: 超过24h无失败则重置计数
- **~40种错误模式**: substring + regex 匹配，覆盖真实API错误
- **FallbackResult 元数据**: 记录每次尝试的耗时、原因、尝试次数

**nekobot差距**: CircuitBreaker 阈值固定(5次), 无指数退避, 错误分类模式较少

#### B. Context 缓存与压缩 (推荐迁移)
**文件**: `/home/czyt/code/go/picoclaw/pkg/agent/context.go`

- **mtime缓存**: system prompt 只在文件变更时重新构建
  - 跟踪文件存在性+修改时间
  - skills 目录递归扫描
  - 双重检查锁模式(RLock快路径 + Lock慢路径)
- **SystemParts结构**: 静态(可缓存)+动态(每请求)分离
  - 启用 provider 端 KV cache 复用
  - 工具定义排序(确保缓存命中)
- **上下文压缩**:
  - 被动: 75%窗口 或 >20消息 触发后台摘要
  - 强制: token错误时丢弃最旧50%历史
  - 多段摘要: 大历史分段→分别摘要→合并
  - 压缩说明: 追加系统提示告知LLM发生了压缩

**nekobot差距**: 无context缓存, 无自动压缩, 无token预算管理

#### C. 会话历史清理 (推荐迁移)
**文件**: `/home/czyt/code/go/picoclaw/pkg/agent/context.go:479-542`

- `sanitizeHistoryForProvider()`:
  - 移除孤立的 tool_result 消息
  - 验证 tool 消息前有对应的 assistant tool_call
  - 丢弃头尾不完整的消息
  - 防止 provider API 报错

**nekobot差距**: 无历史消息清理/验证

#### D. 原子文件写入 (推荐迁移)
**文件**: `/home/czyt/code/go/picoclaw/pkg/fileutil/file.go`

- temp文件 → Sync() → Chmod → Rename 模式
- 目录级sync确保持久性
- 所有持久化操作统一使用

**nekobot差距**: 会话/状态保存未使用原子写入

### 1.2 nextclaw 的优秀模式

#### A. Input Budget Pruner (参考)
**文件**: `nextclaw-core/src/agent/input-budget-pruner.ts`

- 上下文token: 200k默认, 保留20k, 软阈值4k
- Tool结果截断: 限制为上下文30%(max 400KB)
- 系统提示截断: 保留最少2000字符
- 用户消息截断: 保留最少1000字符

#### B. Agent Handoff 防乒乓 (参考)
- 跟踪 `agent_handoff_depth`
- 配置最大乒乓轮数
- 防止多agent间无限递归

#### C. 消息路由 (参考)
- `RouteResolver`: 7级优先级级联 (peer > parent_peer > guild > team > account > channel > default)

---

## 2. Skill 支持和便捷性

### 2.1 nekobot 现有能力 (已较好)

- **6层优先级加载**: builtin < executable < global < workspace hidden < workspace < local
- **YAML frontmatter**: name, description, version, author
- **Eligibility 检查**: 二进制依赖, 环境变量, OS, 架构
- **自动监听变更**: SkillsAutoReload
- **依赖安装器**: brew, go, uv, npm, download

### 2.2 picoclaw 可借鉴的改进

#### A. Skill Registry Manager (推荐)
**文件**: `/home/czyt/code/go/picoclaw/pkg/skills/registry.go:80-107`

- 管理多个skill注册源(如ClawHub)
- **并发搜索**: 扇出到所有注册源, 信号量控制并发
- **结果合并排序**: 按score排序
- **超时控制**: 每个注册源1分钟超时
- **部分失败容忍**: 某个注册源失败不影响其他

**nekobot差距**: 无远程skill注册源, 无skill搜索

#### B. Always Skills (推荐)
**文件**: `nextclaw-core/src/agent/skills.ts:110-120`

- `always: true` 标记的skill始终包含在上下文中
- 适用于关键准则、安全规则等

**nekobot差距**: 需检查是否已有类似机制

#### C. Skill Summary XML (参考)
- 在系统提示中列出所有可用skill的摘要
- LLM可以主动选择读取最相关的skill
- 避免一次性加载所有skill内容

### 2.3 nextclaw Marketplace 集成

- `installClawHubSkill()`: 从marketplace安装
- 支持 slug, version, registry URL
- 安装到workspace skills目录
- --force 覆盖选项

---

## 3. 定时任务的稳健性和进度反馈

### 3.1 nekobot 现有能力

**文件**: `/home/czyt/code/go/nekobot/pkg/cron/cron.go`
- 使用 `robfig/cron/v3` (成熟的Go cron库)
- JSON文件持久化
- 启用/禁用/添加/删除
- 5分钟执行超时
- 跟踪 last_run, next_run, run_count, last_error

### 3.2 picoclaw 可借鉴的改进

#### A. 三种调度类型 (推荐)
- `at`: 一次性未来执行
- `every`: 固定间隔(毫秒级)
- `cron`: CRON表达式 + 时区

**nekobot差距**: 仅支持cron表达式, 缺少 at/every 类型

#### B. 原子持久化 (推荐)
- 使用 `fileutil.WriteFileAtomic()` 保存
- 执行前解锁, 执行完更新状态
- 防止状态文件损坏

**nekobot差距**: 需确认是否使用原子写入

#### C. DeleteAfterRun (推荐)
- 一次性任务执行后自动删除
- 或执行后禁用但保留(可查看历史)

#### D. Job State 详细状态 (推荐)
```go
type CronJobState struct {
    NextRunAtMS *int64  // 下次运行时间
    LastRunAtMS *int64  // 上次运行时间
    LastStatus  string  // "ok" | "error" | "skipped"
    LastError   string  // 上次错误信息
}
```

### 3.3 nextclaw 可借鉴的改进

#### A. 进度反馈 Web UI (推荐)
**文件**: `nextclaw-ui/src/components/config/CronConfig.tsx`

- 任务名称、调度、启用状态
- 上次运行时间和状态
- 上次错误信息
- 下次计划运行
- 操作: 添加、删除、启用/禁用、立即运行

#### B. Cron Tool (参考)
- Agent可通过tool创建定时任务
- 支持 deliver 参数(结果发送到channel)
- CLI: `cron list/add/remove/enable/run`

---

## 4. 界面易用性

### 4.1 nextclaw UI 亮点 (已在移植中)

- ProviderForm: 模型发现checkbox picker, MaskedInput, KeyValueEditor
- ChannelForm: 动态字段生成(根据channel类型)
- CronConfig: 任务状态可视化
- SessionsConfig: 分屏浏览(列表+详情)
- RuntimeConfig: Agent路由绑定可视化

### 4.2 picoclaw 的 UI 创新 (参考)

#### A. 确定性工具顺序
- 工具列表排序确保KV cache稳定
- 减少不必要的provider调用开销

#### B. SystemParts 缓存标记
- 静态提示标记为 "ephemeral"
- 启用provider端前缀缓存

---

## 5. 行动建议 (按优先级排序)

### P0 - 架构稳健性 (直接影响可靠性)

1. **迁移原子文件写入** (从picoclaw/pkg/fileutil/)
   - 应用到: session保存, state保存, cron保存, memory写入
   - 工作量: 小(1个util文件 + 调用点替换)

2. **增强Provider Failover** (从picoclaw/pkg/providers/)
   - 指数退避Cooldown替代固定阈值
   - 扩展错误分类模式(~40种)
   - 24小时错误窗口重置
   - 工作量: 中(改进现有failover.go + error_classifier.go)

3. **添加上下文压缩** (从picoclaw/pkg/agent/loop.go)
   - 被动压缩(75%窗口触发)
   - 强制压缩(token错误时)
   - 多段摘要
   - 工作量: 中-大(新增context压缩模块)

4. **添加历史消息清理** (从picoclaw/pkg/agent/context.go)
   - sanitizeHistoryForProvider
   - 防止provider API错误
   - 工作量: 小(1个函数)

### P1 - Skill 增强

5. **添加Skill Registry Manager** (从picoclaw/pkg/skills/registry.go)
   - 支持远程skill注册源
   - 并发搜索+超时控制
   - 工作量: 中

6. **Always Skills机制** (从nextclaw)
   - frontmatter `always: true` 标记
   - 始终包含在系统提示中
   - 工作量: 小

### P2 - Cron 增强

7. **添加at/every调度类型** (从picoclaw/pkg/cron/)
   - 扩展现有cron到3种类型
   - 工作量: 中

8. **Cron 原子持久化 + DeleteAfterRun**
   - 使用原子写入保存jobs.json
   - 一次性任务执行后清理
   - 工作量: 小

9. **Cron Web UI** (新前端页面)
   - 任务列表+状态可视化
   - CRUD操作
   - 工作量: 中

### P3 - Context 优化

10. **System Prompt 缓存** (从picoclaw/pkg/agent/context.go)
    - mtime检测+双重检查锁
    - 静态/动态分离
    - 工作量: 中-大

11. **确定性工具排序** (从picoclaw/pkg/tools/registry.go)
    - 排序工具名确保cache稳定
    - 工作量: 小

---

## 文件参考路径

### picoclaw 关键文件
- Agent Loop: `/home/czyt/code/go/picoclaw/pkg/agent/loop.go` (1148行)
- Context Cache: `/home/czyt/code/go/picoclaw/pkg/agent/context.go` (583行)
- Fallback Chain: `/home/czyt/code/go/picoclaw/pkg/providers/fallback.go`
- Error Classifier: `/home/czyt/code/go/picoclaw/pkg/providers/error_classifier.go`
- Cooldown: `/home/czyt/code/go/picoclaw/pkg/providers/cooldown.go`
- Atomic File: `/home/czyt/code/go/picoclaw/pkg/fileutil/file.go`
- Tool Registry: `/home/czyt/code/go/picoclaw/pkg/tools/registry.go`
- Subagent: `/home/czyt/code/go/picoclaw/pkg/tools/subagent.go`
- Cron Service: `/home/czyt/code/go/picoclaw/pkg/cron/service.go`
- Skill Loader: `/home/czyt/code/go/picoclaw/pkg/skills/loader.go`
- Skill Registry: `/home/czyt/code/go/picoclaw/pkg/skills/registry.go`
- Session Manager: `/home/czyt/code/go/picoclaw/pkg/session/manager.go`
- Memory Store: `/home/czyt/code/go/picoclaw/pkg/agent/memory.go`


---

## Blades 文档与 go doc 能力边界 (2026-02-27)

### Sources consulted
- 官方文档：
  - `https://go-kratos.dev/blades/`
  - `https://go-kratos.dev/blades/get-started/quick-started/`
  - `https://go-kratos.dev/blades/get-started/run/`
  - `https://go-kratos.dev/blades/get-started/runstream/`
  - `https://go-kratos.dev/blades/tutorials/01-session/`
  - `https://go-kratos.dev/blades/tutorials/02-memory/`
  - `https://go-kratos.dev/blades/tutorials/03-prompts/`
  - `https://go-kratos.dev/blades/tutorials/04-middleware/`
  - `https://go-kratos.dev/blades/tutorials/05-obserability/`
  - `https://go-kratos.dev/blades/tutorials/06-tools/`
  - `https://go-kratos.dev/blades/agent-patterns/01-sequential-agents/`
  - `https://go-kratos.dev/blades/agent-patterns/02-loop-agents/`
  - `https://go-kratos.dev/blades/agent-patterns/03-parallel-agents/`
  - `https://go-kratos.dev/blades/agent-patterns/04-handoff-agents/`
  - `https://go-kratos.dev/blades/graph-workflows/graph-state/`
  - `https://go-kratos.dev/blades/model-providers/claude/`
  - `https://go-kratos.dev/blades/model-providers/gemini/`
  - `https://go-kratos.dev/blades/model-providers/openai/`
  - `https://go-kratos.dev/blades/evaluate/evaluation/`
- go doc / 模块证据：
  - `go list -m -json -versions github.com/go-kratos/blades`
  - `go list -m -json github.com/go-kratos/blades@v0.3.1`
  - `env GOMOD=/dev/null go mod download -json github.com/go-kratos/blades@v0.3.1`
  - `go doc "/Users/czyt/go/pkg/mod/github.com/go-kratos/blades@v0.3.1"`
  - `go doc -all "/Users/czyt/go/pkg/mod/github.com/go-kratos/blades@v0.3.1/flow"`
  - `go doc -all "/Users/czyt/go/pkg/mod/github.com/go-kratos/blades@v0.3.1/graph"`
  - `go doc -all "/Users/czyt/go/pkg/mod/github.com/go-kratos/blades@v0.3.1/memory"`

### Allowed APIs（确认可用）
- 根包 `blades`:
  - `NewAgent(name, opts...)`
  - `WithModel/WithInstruction/WithTools/WithMiddleware/WithMaxIterations/WithToolsResolver/WithDescription/WithOutputKey`
  - `NewRunner(rootAgent, opts...)`
  - `WithResumable(...)`
  - `NewSession(...)` + `WithSession(...)`
  - `UserMessage/SystemMessage/AssistantMessage`
- `flow`:
  - `NewSequentialAgent(SequentialConfig)`
  - `NewParallelAgent(ParallelConfig)`
  - `NewLoopAgent(LoopConfig)`
  - `NewRoutingAgent(RoutingConfig)`
  - `NewDeepAgent(DeepConfig)`
- `graph`:
  - `New(...)` + `AddNode/AddEdge/SetEntryPoint/SetFinishPoint/Compile`
  - `Executor.Execute/Resume`
  - `WithEdgeCondition/WithCheckpointer/WithCheckpointID`
- `memory`:
  - `NewMemoryTool(store)`
  - `MemoryStore` 接口
  - `NewInMemoryStore()`

### 能力边界（本次规划约束）
1. Blades 已有多 agent 编排抽象（顺序/并行/循环/路由），适合作为 nekobot 的 orchestrator runtime 替代层。
2. Memory 层是接口抽象 + InMemory 示例实现，不应假设已有“现成持久化后端”。
3. Graph 能力可做复杂状态机/恢复（checkpoint），但第一阶段优先 flow 即可，避免过度设计。
4. 官方 Provider 文档能力覆盖 OpenAI/Claude/Gemini；可逐步替换当前自研 adaptor。

### Anti-patterns to avoid
- 假设存在 `github.com/go-kratos/blades/agent` 公开子包（本次证据未发现）。
- 直接导入 `internal/*`。
- 认为 memory 已内置 Redis/向量库后端。
- 在未依赖 blades 的模块里直接 `go doc github.com/go-kratos/blades` 得出“包不存在”错误结论。

### Copy-ready snippets
- Run/RunStream：`get-started/run`、`get-started/runstream`
- 多 agent 编排：`agent-patterns/01..04`
- Session/Memory/Middleware：`tutorials/01/02/04`
- Graph checkpoint：`graph-workflows/graph-state`

---

## Feature Batch #1 完成记录 (2026-02-27)

### 范围
- Phase 1：多用户认证基础落地（User/Tenant/Membership + JWT 统一秘钥治理 + WebUI 认证端点迁移）。

### 代码变更摘要
- 新增 schema：`pkg/storage/ent/schema/user.go`、`pkg/storage/ent/schema/tenant.go`、`pkg/storage/ent/schema/membership.go`。
- 新增认证存储层：`pkg/config/auth_store.go`，封装事务、默认租户确保、成员关系确保、legacy 凭据清理、JWT secret 统一读写与兼容解析。
- 重构认证核心：`pkg/config/admin_credential.go`。
  - `LoadAdminCredential` 优先 users/memberships，缺失时兼容 legacy section。
  - `SaveAdminCredential` 迁移为 user+tenant+membership 模型，并写入独立 JWT secret payload。
  - 新增 `AuthenticateUser`、`BuildAuthProfileByUsername`、`BuildAuthProfileByUserID`、`UpdateUserProfile`、`UpdateUserPassword`、`RecordUserLogin`。
- 重构 WebUI 服务：`pkg/webui/server.go`。
  - 移除内存态 `adminCred`/`credMu`。
  - 新增 `/auth/me`。
  - 登录/初始化/个人资料/密码更新改为 DB 驱动。
  - JWT claims 扩展为 `sub/uid/role/tid/ts/iat/exp`。
- 网关 JWT secret 源切换：`pkg/gateway/server.go` 改为 `config.GetJWTSecret(client)`。
- 补测试：`pkg/config/db_store_test.go`。
  - `TestSaveAdminCredentialMigratesToUserTenantMembership`
  - `TestGetJWTSecretWithLegacyPayload`

### 验证
- 已执行：`go test ./pkg/config ./pkg/webui ./pkg/gateway`
- 结果：全部通过。

---

## Feature Batch #2 完成记录 (2026-02-28)

### 范围
- Phase 2：将 `chatWithBladesOrchestrator` 从 stub 切换为真实 blades runtime 接管，并保持 provider fallback 与工具执行语义一致。

### 代码变更摘要
- 新增 `pkg/agent/blades_runtime.go`。
  - 实现 `bladesModelProvider`，将 blades `ModelRequest` 转换为 `providers.UnifiedRequest`，并复用 `callLLMWithFallback`。
  - 保留上下文超限重试压缩：`isContextLimitError` 触发 `forceCompressMessages` 后重试。
  - 实现 `bladesToolResolver`，将 blades tool 调用桥接到现有 `Agent.executeToolCall`。
  - 实现 `chatWithBladesOrchestrator`：构建 blades agent/runner，注入系统提示、工具解析器、会话历史与 middleware。
- 更新 `pkg/agent/agent.go`：移除旧的 blades stub 转发实现，改由 `blades_runtime.go` 提供真实实现。
- 更新 `pkg/agent/agent_test.go`：调整 blades 路径错误断言为 `blades runner run: llm call with fallback: ...`。
- 更新 `pkg/subagent/manager.go`：`subagent.Agent` 接口改为 `Chat(ctx, sess, message)`，并增加 `taskSession` 适配。
- 更新依赖：`go.mod` / `go.sum` 增加 `github.com/go-kratos/blades` 及相关依赖。

### 验证
- 已执行：`go test ./pkg/agent ./pkg/subagent ./pkg/tools ./pkg/config ./pkg/webui ./pkg/gateway`
- 结果：全部通过。


## 2026-04-01 Claude Code 文档补充结论

### 新吸收的关键设计原则
- `Prompt assembly` 要继续从“单段系统提示”升级为“静态前缀 + 动态后缀”的模块化拼装：
  - 静态区承载身份、行为规则、工具使用语法、输出风格。
  - 动态区承载 session guidance、skills/capabilities、MCP instructions、当前权限模式、上下文摘要。
  - 这样不仅更容易维护，也更利于后续 prompt cache / context compaction。
- `Task runtime` 不应只等于 subagent 队列。
  - Claude Code 的启发是：前台 agent、后台 agent、specialist agent、teammate 都共享任务模型，只是生命周期不同。
  - `nekobot` 需要继续把 chat、background subagent、future verify/plan/explore agent 都收敛到统一任务载体，而不是各走各的状态流。
- `Runtime state store` 是下一阶段的关键。
  - Claude Code 的 `AppStateStore` 说明：权限对话、任务列表、plan 状态、team context、inbox 不应散落在多个临时对象里。
  - `nekobot` 应新增一层统一运行态存储，至少承载 task list、permission mode、pending actions、task summaries、notifications。
- `Tool execution` 应被视作治理流水线，不是直接调用。
  - 文档反复强调：输入校验 -> pre-hook -> permission -> execute -> post-hook -> failure-hook。
  - 这对 `nekobot` 后续引入 permission mode、tool hooks、runtime-specific tool filtering 非常关键。
- `Capability awareness` 要成为模型可见的协议，而不是配置存在即可。
  - Claude Code 的 skills/plugin/MCP 强在“模型知道自己现在能用什么、为什么能用、何时该用”。
  - `nekobot` 后续不应只把 skills/MCP/tools 注册到后端，还需要把 capability delta 拼进 prompt 动态区。
- `Context economy` 需要独立成基础设施。
  - 文档明确了 `microcompact / autocompact / transcript / resume / cache boundary` 是一套体系，不是一个 `/compact` 命令。
  - `nekobot` 现在已有 sessions、learnings、memory、audit；下一步应做结构化摘要边界，而不是继续依赖历史消息自然膨胀。
- `Coordinator` 的价值在于“共享任务协议 + push completion + idle worker semantics”，而不是先追求 swarm 噱头。
  - 当前更适合先实现 task ownership、task summary、worker idle/active state、completion notification。
- `Daemon + UDS inbox + bridge` 应被看作同一层。
  - daemon 解决长生命周期监督。
  - UDS inbox 解决本地跨进程控制/通知。
  - bridge 则是这套抽象对外暴露后的延伸，不适合跳过前两者直接做。

### 对现有计划的直接影响
- `Phase 1` 不应只停留在“任务字段暴露”，还应明确引入最小 `runtime state store` 轮廓。
- `Phase 2` 的 `AgentDefinition` 需要同时纳入：
  - tool allow/deny
  - permission mode
  - hooks
  - MCP requirements/instructions
  - context policy
  而不是只抽 prompt/provider/model。
- `Phase 3` 应拆成两半：
  - `task-scoped tool assembly`
  - `hook + permission pipeline`
- `Phase 5` 的上下文工程需要细化为：
  - prompt static/dynamic boundary
  - transcript summary
  - task/session compact
  - resume metadata
- `Phase 6` 的 teammate/swarm 预留应以前置的 `task store + inbox + notification contract` 为基础，不直接写多 agent orchestration 业务。

### 结论
- 继续参考 Claude Code 是对的，但 `nekobot` 当前最该学的不是“彩蛋功能”，而是：
  1. 模块化 prompt 装配。
  2. 统一任务模型与运行态存储。
  3. 工具调用治理链。
  4. capability-aware prompt 注入。
  5. 上下文压缩与 resumability。
  6. daemon/inbox/coordinator 的分层推进。


## 2026-04-01 Task Runtime 观测链路补记

### 已完成内容
- `pkg/agent/agent.go`
  - 新增 `GetTaskSnapshots()`，把 subagent manager 的 snapshot 能力以 agent 级出口暴露给上层控制面。
- `pkg/webui/server.go`
  - `Server` 新增 `taskSource` 注入点，默认接 `ag.GetTaskSnapshots`。
  - `/api/status` 新增：
    - `task_count`
    - `task_state_counts`
    - `recent_tasks`
  - 新增摘要逻辑：按 `completed_at > started_at > created_at` 排序并截取最近 5 条。
- `pkg/webui/server_status_test.go`
  - 扩展状态接口断言。
  - 新增 `TestHandleStatus_IncludesRecentTasks`。
- `pkg/webui/frontend/src/hooks/useConfig.ts`
  - 为状态接口补 `StatusData` / `StatusTask` 类型。
- `pkg/webui/frontend/src/pages/SystemPage.tsx`
  - 新增任务运行态卡片，展示总任务数、运行中、等待中、失败数。
  - 新增近期任务列表，直接展示状态 pill、task type、label、session/channel 信息和错误摘要。
  - 仍保留 raw status 区块，便于排查更深层 payload。
- `pkg/webui/frontend/public/i18n/{en,zh-CN,ja}.json`
  - 补齐 System 页任务运行态相关文案。

### 为什么这一步重要
- 这让 `pkg/tasks.Task` 不再只是内部结构，而是真正进入了控制面协议。
- 它为后续几件事打基础：
  - runtime state store
  - pending action / permission mode 可视化
  - daemon/background worker 监督
  - coordinator/teammate 的任务面板

### 当前边界
- 现在可见的主要还是 background subagent task。
- main chat loop、future verify/plan/explore agent 还没有统一接入同一 task runtime。
- 这一步解决的是“可观测性缺口”，不是完整 runtime store。

### 已完成验证
- `go test -count=1 ./pkg/webui -run 'TestHandleStatus_ReturnsExtendedFields|TestHandleStatus_IncludesRecentTasks|TestBuildWebUIChatPromptContextIncludesExplicitRuntimeID|TestHandleUndoChatSession|TestClearChatSessionRemovesUndoSnapshots|TestResolveWebUIRuntimeSelectionUsesRuntimeDefaults|TestResolveWebUIRuntimeSelectionRejectsUnboundRuntime|TestResolveWebUIRuntimeSelectionFallsBackToRequestedRoute'`
- `go test -count=1 ./pkg/agent ./pkg/subagent ./pkg/tools ./pkg/webui ./pkg/gateway ./pkg/inboundrouter`
- `npm --prefix pkg/webui/frontend run build`


## 2026-04-01 Runtime State Store Skeleton 补记

### 已完成内容
- `pkg/tasks/store.go`
  - 新增 `Store`，支持：
    - `SetSource(name, SnapshotFunc)`
    - `RemoveSource(name)`
    - `List()`
  - 当前按 source 名字稳定排序，保证聚合结果可预测。
- `pkg/tasks/store_test.go`
  - 新增 store 聚合顺序与 source 移除测试。
- `pkg/agent/agent.go`
  - `Agent` 新增 `taskStore *tasks.Store`。
  - 初始化时创建 store。
  - `EnableSubagents()` 时注册 `subagents` source。
  - `DisableSubagents()` 时移除 `subagents` source。
  - `GetTaskSnapshots()` 改为统一从 store 读取。
- `pkg/webui/server.go`
  - `Server` 从直接持有 `taskSource` 改为持有 `taskStore`。
  - 当前默认接一个 `agent` source，但协议层已经从“单 source callback”升级到“统一 store 聚合”。
- `pkg/webui/server_status_test.go`
  - 测试改为通过 `tasks.Store` 注入假数据，避免测试继续绑定旧的单函数 source 模型。

### 为什么这一步重要
- 上一轮只解决“看见 subagent task”。
- 这一轮开始解决“以后所有 execution unit 该挂到哪里”。
- 有了 store，后续可以按 source 逐步接入：
  - main chat task
  - verify / plan / explore specialist task
  - daemon/background worker
  - teammate/coordinator worker

### 当前边界
- 现在的 store 还是 pull-based snapshot aggregation，不是事件流。
- 还没有：
  - pending action registry
  - permission dialog state
  - notification inbox
  - resume metadata registry
- 所以它是 `runtime state store skeleton`，不是完整 `AppStateStore` 替代品。

### 已完成验证
- `go test -count=1 ./pkg/tasks ./pkg/agent ./pkg/subagent ./pkg/webui`

## 2026-04-02 tool_session spawn -> process 接线补记

### 本轮完成
- `pkg/tools/toolsession.go`
  - `tool_session spawn` 现在会先解析上下文里的 `runtime_id/task_id`。
  - 当上游没有显式 `task_id` 时，会为新建的 tool session 自动生成一个 task 绑定：
    - 直接复用新创建的 `session_id`
    - 回写到 tool session metadata
    - 再交给 `process.StartWithSpec()` 进入共享 `pkg/tasks.Service`
- 这意味着 agent 主动拉起的 coding tool session，已经不再只是 WebUI 可见的终端 session，控制面里也会出现对应的 managed task。

### 当前语义
- 如果上游已经有 `task_id`：
  - 保持透传，不额外重写。
- 如果上游没有 `task_id`：
  - `tool_session` 自己生成独立 task identity。
  - `process` 继续负责完整生命周期推进：
    - `enqueue`
    - `claim`
    - `start`
    - `complete/fail/cancel`
- 当前没有新增 task type；仍复用现有 `runtime_worker`，因此 WebUI `/api/status` 与 runtime topology 不需要扩 schema。

### 本轮测试
- `go test -count=1 ./pkg/tools`
- `go test -count=1 ./pkg/process ./pkg/agent ./pkg/webui`

### 结论
- `provider/model` 前置迁移后的下一条主线 `agent tool_session spawn -> process -> tasks.Service` 已完成。
- 当前唯一下一主线切换为：
  1. `cron executeJob`
  2. `watch executeCommand`

## 2026-04-02 cron executeJob -> tasks.Service 接线补记

### 本轮完成
- `pkg/cron/cron.go`
  - `executeJob()` 现在会在进入 agent 执行前创建 managed task。
  - 成功路径进入 `complete`，失败路径进入 `fail`。
  - 每次 cron run 使用独立 task id，并记录：
    - `source=cron`
    - `job_id`
    - `job_name`
- `pkg/agent/agent.go`
  - 暴露共享 `TaskService()`，供 `cron.Manager` 复用同一套生命周期服务。

### 当前语义
- cron 任务现在属于控制面可见的一类本地 agent 执行。
- 当前复用 `tasks.TypeLocalAgent`。
- `SessionID` 复用 `cron job id`，`RuntimeID` 固定为 `cron`。
- 只向执行上下文注入 `runtime_id=cron`，不向内层工具执行链透传 cron run 自己的 task id。

### 本轮测试
- `go test -count=1 ./pkg/cron`
- `go test -count=1 ./pkg/webui -run 'TestHandle(CreateCronJob_AcceptsRouteOverrides|CreateCronJob_AtScheduleValidatesRFC3339|RunCronJob_NotFound|RunCronJob_DisabledJobDoesNotExecute|CronJobLifecycle)'`
- `go test ./...`
- `cd pkg/webui/frontend && npm ci && npm run build`

### 结论
- `cron executeJob -> tasks.Service` 已完成。
- 当前唯一剩余主线切换为：
  1. `watch executeCommand -> tasks.Service`

## 2026-04-02 watch executeCommand -> tasks.Service 接线补记

### 本轮完成
- `pkg/watch/watcher.go`
  - `executeCommand()` 现在会在执行 watch 命令前创建 managed task。
  - 成功路径进入 `complete`，失败路径进入 `fail`。
  - 当前记录：
    - `source=watch`
    - `file`
    - `op`
    - `pattern`
    - `command`
    - `fail_command`
  - `RuntimeID` 固定为 `watch`，`SessionID` 使用 `watch:<patternIdx>`。
- `pkg/watch/fx.go`
  - `watch.Module` 现在会从共享 `agent.Agent` 注入 `TaskService()`，避免 watcher 自己创建第二套 lifecycle service。
- `pkg/watch/watcher_test.go`
  - 已补 watch managed task 成功/失败测试。

### 当前语义
- watch 触发的本地命令现在属于控制面可见的一类本地 agent 执行。
- 当前继续复用 `tasks.TypeLocalAgent`，不新增 task type。
- WebUI `/api/status` 不需要扩 schema，因为现有 `recent_tasks` 聚合已经直接消费 `tasks.Store`。

### 本轮测试
- `go test -count=1 ./pkg/watch`
- `go test -count=1 ./pkg/watch ./pkg/webui`

### 结论
- `watch executeCommand -> tasks.Service` 已完成。
- 到这里，当前这轮 `runtime/task lifecycle` 主线已经全部收口：
  1. `subagent`
  2. `exec.background -> process`
  3. `agent tool_session spawn -> process`
  4. `cron executeJob`
  5. `watch executeCommand`
- 后续应按主计划切换到：
  1. `AgentDefinition`
  2. `tool governance`
  3. `context economy`
  4. `permission rules` / `context sources`

## 2026-04-02 tool governance / permission rules 设计收口补记

### 当前代码现状
- 现有 `pkg/approval.Manager` 已具备：
  - `mode`
  - `allowlist`
  - `denylist`
  - pending approval queue
  - session mode override
- 现有工具权限决策主要集中在：
  - `pkg/agent/agent.go -> executeToolCall()`
- 当前没有：
  - 独立持久化规则层
  - 可解释的规则命中结果
  - 可扩展到 classifier/hooks 的治理入口

### 已锁定的第一版边界
- 所有 tool call 统一先进入 `permission rules` evaluator。
- 第一版规则匹配字段只做：
  - `tool_name`
  - `session_id`
  - `runtime_id`
- 第一版 action 只做：
  - `allow`
  - `deny`
  - `ask`
- 未命中规则时，回落到现有 approval mode。

### 为什么不先改 `approval.Manager`
- 如果直接把 rules 塞进 `approval.Manager`，它会同时承担：
  - queue
  - mode
  - policy
  - 后续 hooks glue
- 这会让 `approval` 很快变成脏对象，不利于后续：
  - `AgentDefinition`
  - `tool governance pipeline`
  - `context sources`

### 为什么不先做 `context sources`
- 现在还没有稳定的：
  - `AgentDefinition`
  - prompt section registry
  - context assembler
- 先做 `context sources` 很容易只做成一层临时展示，后面返工概率高。
- 相比之下，`permission rules` 已经有清晰的执行入口，闭环更容易成立。

## 2026-04-02 tool governance / permission rules 实现补记

### 本轮完成
- 存储层
  - 新增 `pkg/storage/ent/schema/permissionrule.go`
  - 生成 `pkg/storage/ent/permissionrule*` 相关 ent 代码
  - 新增 `pkg/permissionrules/manager.go`
  - 新增 `pkg/permissionrules/evaluator.go`
- approval / agent 接线
  - `pkg/approval/approval.go`
    - 新增强制排队的 `EnqueueRequest()`，使 `ask` 不受当前 auto/manual mode 影响
  - `pkg/agent/agent.go`
    - `executeToolCall()` 统一先走 permission rules evaluator
    - 命中 `deny` 时直接拒绝
    - 命中 `ask` 时强制进入 pending approval queue
    - 命中 `allow` 时跳过 approval fallback
    - 未命中时继续走旧 approval mode
  - `pkg/agent/fx.go`
    - 生产注入链已显式接收并挂载 `permissionrules.Manager`
  - `cmd/nekobot/{main,tui,acp,cron,service}.go`
    - 全部补挂 `permissionrules.Module`，避免不同运行模式行为不一致
- WebUI
  - `pkg/webui/server.go`
    - 新增：
      - `GET /api/permission-rules`
      - `POST /api/permission-rules`
      - `PUT /api/permission-rules/:id`
      - `DELETE /api/permission-rules/:id`
  - 前端新增：
    - `pkg/webui/frontend/src/hooks/usePermissionRules.ts`
    - `pkg/webui/frontend/src/pages/PermissionRulesPage.tsx`
    - `App.tsx` 路由
    - `Sidebar.tsx` 入口
    - `public/i18n/{en,zh-CN,ja}.json` 文案

### 当前落地语义
- 规则字段第一版保持最小集合：
  - `tool_name`
  - `session_id`
  - `runtime_id`
  - `action=allow|deny|ask`
  - `priority`
  - `enabled`
- evaluator 命中顺序：
  - `priority desc`
  - 作用域特异性 desc
  - `updated_at desc`
  - `id asc`
- `ask` 的语义已锁定为：
  - 无论当前 approval mode 是 `auto` 还是 `manual`
  - 都强制进入待审批队列
- 当前切片边界已锁定为独立提交：
  - 包含 `permission rules` 持久化、执行接线、API、WebUI 管理页
  - 不混入 `context sources` 预览
  - 不混入 `readonly preflight`
  - 不混入 `AgentDefinition` System 页展示

### 本轮测试
- `go test -count=1 ./pkg/agent -run 'TestProvideAgent_WiresPermissionRuleManager|TestExecuteToolCallPermissionRule'`
- `go test -count=1 ./pkg/permissionrules ./pkg/approval ./pkg/agent ./pkg/webui`
- `go test -count=1 ./cmd/nekobot`
- `cd pkg/webui/frontend && npm run build`

### 额外注意
- `pkg/webui/frontend/dist` 当前受版本控制，因此前端构建会同步刷新产物；这不是新增设计决策，只是仓库现状。
- `pkg/agent/agent.go`、`pkg/webui/server.go`、`public/i18n/{en,ja,zh-CN}.json` 当前存在跨切片改动，因此本轮通过部分暂存只提交 permission-rules 相关 hunks。
- 下一步不再继续扩只读 explainability，而是进入首个真实 runtime decision 点切片。
- `permission rules` 已作为独立功能批次提交并推送：
  - commit: `aad683b feat(governance): add persisted permission rules`
  - remote: `origin/main`

## 2026-04-02 AgentDefinition / prompt boundary 桥接补记

### 本轮完成
- `pkg/agent/definition.go`
  - 新增 `AgentDefinition` compatibility bridge：
    - `Route`
    - `PermissionMode`
    - `ToolPolicy`
    - `MaxToolIterations`
    - `PromptSections`
  - 新增 `AgentDefinitionFromRuntimeConfig()`
- `pkg/agent/context.go`
  - 新增 `PromptSection`
  - 新增 `BuildPromptSections()`
  - `BuildSystemPrompt()`、`buildStaticPromptBlock()`、`buildDynamicPromptBlock()` 现在统一通过 section 列表装配
- `pkg/agent/agent.go`
  - `Agent` 初始化时会保留当前 definition snapshot
  - 新增 `Definition()` accessor
- `pkg/webui/server.go`
  - `/api/status` 现已返回 `agent_definition`
- `pkg/webui/frontend/src/pages/SystemPage.tsx`
  - 新增只读 definition 区块：
    - route
    - permission mode
    - allow/deny list
    - static/dynamic sections

### 当前语义
- 这一步还不是完整的多-definition runtime。
- 但当前主 agent 已经有一个稳定的“定义快照”可被读取，后续：
  - `context sources`
  - definition-driven task runtime
  - prompt section registry
  都可以直接复用这层边界，而不用继续从 `config + context builder + approval` 三处分散推导。

### 本轮测试
- `go test -count=1 ./pkg/agent -run 'TestContextBuilderBuildPromptSections_SeparatesStaticAndDynamic|TestAgentDefinitionFromRuntimeConfig_BridgesCurrentDefaults|TestNewAgent_SeedsDefinitionSnapshot'`
- `go test -count=1 ./pkg/webui -run 'TestHandleStatus_'`
- `cd pkg/webui/frontend && npm run build`

### 当前切片边界
- 当前正重新按独立提交收口 `AgentDefinition bridge`：
  - 包含 `definition.go`、`BuildPromptSections()`、`Agent.Definition()`、`/api/status` 的 `agent_definition`、System 页只读展示
  - 不混入 `context sources` 预览 API
  - 不混入 Chat route readonly `preflight`

## 2026-04-03 conversation/thread binding 计划复核补记

### 本轮复核范围
- `pkg/conversationbindings/service.go`
- `pkg/conversationbindings/service_test.go`
- `pkg/toolsessions/manager.go`
- `pkg/channels/wechat/runtime.go`
- `pkg/channels/wechat/runtime_test.go`
- `pkg/gateway/server.go`
- `task_plan.md`
- `progress.md`

### 当前判断
- `Slack interactive callback` 仍是明确缺口，但属于局部收尾项，主要价值在 Slack 通道本身。
- `conversation/thread binding` 虽然现在已有首批能力，但它直接影响后续：
  - `gateway`
  - `external agent runtime`
  - 弱交互通道上的长期 runtime 绑定
- 因此下一开发主线更适合先落在 `conversation/thread binding`，把基础层做扎实后再扩消费者。

### 已锁定的下一切片边界
- 本轮只规划，不实现代码。
- 下一批实现只做两件事：
  1. 收口 `pkg/conversationbindings` 的通用契约：
     - rebinding 语义
     - 确定性记录顺序
     - 多 session / 多 conversation 下的稳定查询行为
  2. 用 `pkg/channels/wechat/runtime.go` 作为首个真实消费者重新验证契约
- 明确不在同批次内做：
  - 独立 Ent schema / 独立 binding store
  - gateway 接线
  - external agent runtime 接线
  - WeChat presenter / 交互协议扩展

### 为什么先不扩到 gateway
- `pkg/gateway/server.go` 当前还没有任何 binding consumer 代码。
- 如果把 gateway adoption 和 binding service 收口混在一起，失败时很难判断问题出在：
  - 基础契约
  - 消费方接线
  - gateway 自己的控制面边界
- 先用现有 WeChat runtime 做消费者验证，回归面更窄，能更快确认基础层是否成立。

### 计划产物
- 已新增独立实现计划：
  - `docs/superpowers/plans/2026-04-03-conversation-thread-binding.md`
- 主计划已更新为：
  - 下一主线先做 `conversation/thread binding`
  - `Slack interactive callback` 延后为后续收尾项

## 2026-04-03 conversation/thread binding 首批实现补记

### 本轮完成
- `pkg/conversationbindings/service_test.go`
  - 新增 `TestServiceBindingQueriesReturnDeterministicConversationOrder`。
  - 先以 RED 方式锁定：
    - `ListBindings()` 结果不能再依赖写入顺序
    - `GetBindingsBySession()` 结果不能再依赖写入顺序
- `pkg/conversationbindings/service.go`
  - 新增稳定排序逻辑：
    - `sortBindingStates()`
    - `sortBindingRecords()`
  - 当前 `sessionToBindingRecords()`、`ListBindings()`、`GetBindingsBySession()` 都统一按 `conversation_id` 稳定排序。

### 当前语义
- 这一步不是新功能扩张，而是把现有通用 binding 查询行为从“隐式依赖 metadata 写入顺序”收敛成“显式稳定顺序”。
- 这样后续消费者：
  - WeChat runtime
  - gateway
  - external agent runtime
  可以共享一致的读取语义，不需要各自再做二次排序或假设写入先后。

### WeChat 消费者验证结果
- 本轮没有修改 `pkg/channels/wechat/runtime.go`。
- 但定向与扩大的回归已经证明当前 WeChat runtime 可以直接兼容这次 contract 收口，不需要额外适配代码。

### 本轮测试
- RED:
  - `go test -count=1 ./pkg/conversationbindings -run 'TestServiceBindingQueriesReturnDeterministicConversationOrder'`
- GREEN:
  - `go test -count=1 ./pkg/conversationbindings -run 'TestServiceBindingQueriesReturnDeterministicConversationOrder'`
  - `go test -count=1 ./pkg/conversationbindings`
  - `go test -count=1 ./pkg/toolsessions ./pkg/conversationbindings ./pkg/channels/wechat`

### 仍然延后
- standalone binding store
- gateway adoption
- external agent runtime adoption
- WeChat presenter / 更广泛交互协议

## 2026-04-03 Slack shortcut/modal 首批业务闭环补记

### 本轮完成
- `pkg/channels/slack/slack.go`
  - 为 Slack API 抽象补上 `OpenView(...)` 能力。
  - `handleShortcut()` 现在会处理 `find_skills` shortcut，并打开一个最小 modal。
  - `handleViewSubmission()` 现在会处理 `find_skills_modal`：
    - 读取用户输入的 query
    - 重新走现有 `find-skills` command handler
    - 如返回 `skill_install_confirm` interaction，则继续复用现有 install confirmation message 流
    - 否则以 ephemeral message 返回结果
- `pkg/channels/slack/slack_test.go`
  - 新增：
    - `TestHandleShortcutOpensFindSkillsModal`
    - `TestHandleViewSubmissionExecutesFindSkillsCommand`

### 当前语义
- 这一步不是要把 Slack modal 框架一次性做全，而是先补一个真实的可用业务切片：
  - `find_skills` shortcut
  - `find-skills` modal submission
  - skill install confirmation follow-up
- 这样 Slack interactive callback 不再只剩“有路由入口但没业务”的状态。

### 本轮测试
- RED:
  - `go test -count=1 ./pkg/channels/slack -run 'TestHandle(ShortcutOpensFindSkillsModal|ViewSubmissionExecutesFindSkillsCommand)'`
- GREEN:
  - `go test -count=1 ./pkg/channels/slack -run 'TestHandle(ShortcutOpensFindSkillsModal|ViewSubmissionExecutesFindSkillsCommand)'`
  - `go test -count=1 ./pkg/channels/slack`
  - `go test -count=1 ./pkg/channels/slack ./pkg/commands`

## 2026-04-03 Slack settings shortcut/modal 闭环补记

### 本轮完成
- `pkg/channels/slack/slack.go`
  - `handleShortcut()` 现在额外处理 `settings` shortcut，并打开一个最小 settings modal。
  - `handleViewSubmission()` 现在按 callback id 分发到：
    - `find_skills_modal`
    - `settings_modal`
  - 新增 `handleSettingsViewSubmission()`：
    - 从 modal 里读取 `action` 和 `value`
    - 组装成现有 `/settings` command 的 args
    - 继续复用已存在的 `settings` command handler
    - 以 ephemeral message 返回结果
  - 新增 `buildSettingsModal()`，只提供最小输入：
    - action
    - value
- `pkg/channels/slack/slack_test.go`
  - 新增：
    - `TestHandleShortcutOpensSettingsModal`
    - `TestHandleViewSubmissionExecutesSettingsCommand`

### 当前语义
- 这一步继续只补“现有命令的第二个真实 modal 消费者”，不混入：
  - Slack 通用表单框架
  - 更复杂的多字段 settings UX
  - 新的后端设置协议
- 当前只把 Slack modal 作为 `/settings` 的薄 UI 壳：
  - modal 输入 -> 现有 command args
  - command handler 决定保存/校验/返回文案

### 本轮测试
- RED:
  - `go test -count=1 ./pkg/channels/slack -run 'TestHandle(ShortcutOpens(Settings|FindSkills)Modal|ViewSubmissionExecutes(Settings|FindSkills)Command)$'`
- GREEN:
  - `go test -count=1 ./pkg/channels/slack -run 'TestHandle(ShortcutOpens(Settings|FindSkills)Modal|ViewSubmissionExecutes(Settings|FindSkills)Command)$'`
  - `go test -count=1 ./pkg/channels/slack ./pkg/commands`

## 2026-04-03 Slack model shortcut/modal 闭环补记

### 本轮完成
- `pkg/channels/slack/slack.go`
  - 新增 `model` shortcut/modal 常量与 modal 构造。
  - `handleShortcut()` 现在会处理 `model` shortcut，并打开一个最小 provider/action 输入 modal。
  - `handleViewSubmission()` 现在会处理 `model_modal`，复用现有 `/model` command 执行路径。
  - modal submission 结果继续按现有 Slack 交互风格走 ephemeral 回复，不引入新的 Slack 专用协议。
- `pkg/channels/slack/slack_test.go`
  - 新增：
    - `TestHandleShortcutOpensModelModal`
    - `TestHandleViewSubmissionExecutesModelCommand`

### 当前语义
- 这一步继续沿“在现有路由骨架上补真实业务闭环”的方向推进，不混入新的命令语义。
- 当前规则：
  - `model` shortcut 打开最小 modal。
  - modal submission 把输入原样作为 `/model` 参数执行。
  - 成功结果按 ephemeral message 返回给发起人。

### 本轮测试
- RED:
  - `go test -count=1 ./pkg/channels/slack -run 'TestHandle(ShortcutOpensModelModal|ViewSubmissionExecutesModelCommand)$'`
- GREEN:
  - `go test -count=1 ./pkg/channels/slack -run 'TestHandle(ShortcutOpensModelModal|ViewSubmissionExecutesModelCommand)$'`
  - `go test -count=1 ./pkg/channels/slack -run 'TestHandle(ShortcutOpens(FindSkills|Settings|Model)Modal|ViewSubmissionExecutes(FindSkills|Settings|Model)Command)$'`
  - `go test -count=1 ./pkg/channels/slack ./pkg/commands`

## 2026-04-03 gateway origin allowlist 首批治理补记

### 本轮完成
- `pkg/config/config.go`
  - 为 `GatewayConfig` 新增 `AllowedOrigins []string`。
- `pkg/config/validator.go`
  - 新增 `gateway.allowed_origins[*]` 的空值校验。
- `pkg/gateway/server.go`
  - 移除全局 `CheckOrigin: true`。
  - 新增 `Server.checkOrigin()`：
    - 无 `Origin` 时允许，兼容非浏览器客户端
    - `AllowedOrigins` 为空时继续兼容旧行为
    - 非空时严格按 allowlist 放行
- `pkg/gateway/server_test.go`
  - 新增：
    - `TestGatewayCheckOriginAllowsConfiguredOrigins`
    - `TestGatewayCheckOriginAllowsRequestsWithoutOrigin`
- `pkg/config/path_test.go`
  - 更新 gateway copy 断言，兼容 `AllowedOrigins` 新字段。

### 当前语义
- 这一步只做第一层 origin 治理，不混入：
  - IP 限制
  - scope / pairing
  - rate limit
  - 更完整 control plane protocol
- 目标是先把最明显的开放边界收口，同时保持现有非浏览器客户端兼容性。

### 本轮测试
- RED:
  - `go test -count=1 ./pkg/gateway -run 'TestGatewayCheckOrigin(AllowsConfiguredOrigins|AllowsRequestsWithoutOrigin)'`
- GREEN:
  - `go test -count=1 ./pkg/gateway -run 'TestGatewayCheckOrigin(AllowsConfiguredOrigins|AllowsRequestsWithoutOrigin)'`
  - `go test -count=1 ./pkg/gateway ./pkg/config`

## 2026-04-03 gateway REST 控制面鉴权补记

### 本轮完成
- `pkg/gateway/server.go`
  - 新增 `requireAuthenticatedAPI()`。
  - `handleStatus()` 和 `handleConnections()` 现在统一复用现有 `authenticateWS()` 的 JWT 校验。
  - 未携带有效 token 的 REST 控制面请求现在返回 `401 unauthorized`，不再默认裸露状态与连接信息。
- `pkg/gateway/server_test.go`
  - 新增：
    - `TestGatewayStatusEndpointRequiresAuth`
    - `TestGatewayConnectionsEndpointRequiresAuth`
  - 同时把原有 `TestStatusEndpoint`、`TestConnectionsEndpoint` 改成显式带 JWT 的成功路径，锁定控制面“鉴权失败返回 401、鉴权成功才返回 200”的语义。

### 当前语义
- 这一步继续只收口 gateway 控制面边界，不混入：
  - IP 限制
  - scope / pairing
  - rate limit
  - 更完整 control plane protocol
- 目标是先消除最直接的信息暴露面：
  - `/api/v1/status`
  - `/api/v1/connections`
  现在已经和 websocket 一样要求有效 JWT。

### 本轮测试
- RED:
  - `go test -count=1 ./pkg/gateway -run 'Test(Gateway(StatusEndpointRequiresAuth|ConnectionsEndpointRequiresAuth)|StatusEndpoint|ConnectionsEndpoint)$'`
- GREEN:
  - `go test -count=1 ./pkg/gateway -run 'Test(Gateway(StatusEndpointRequiresAuth|ConnectionsEndpointRequiresAuth)|StatusEndpoint|ConnectionsEndpoint)$'`
  - `go test -count=1 ./pkg/gateway`
  - `go test -count=1 ./pkg/gateway ./pkg/config`

## 2026-04-03 gateway 连接断开控制面补记

### 本轮完成
- `pkg/gateway/server.go`
  - 新增 `DELETE /api/v1/connections/{id}`。
  - 该接口复用现有 `requireAuthenticatedAPI()`，只允许已鉴权调用。
  - 命中已存在连接时，会主动移除 client、关闭其 send channel，并返回 `204 No Content`。
  - 目标连接不存在时返回 `404 Not Found`。
- `pkg/gateway/server_test.go`
  - 新增：
    - `TestDeleteConnectionEndpointRemovesClient`
    - `TestDeleteConnectionEndpointReturnsNotFoundForUnknownClient`
    - `TestDeleteConnectionEndpointRequiresAuth`

### 当前语义
- 这一步仍然只扩一个最小的真实控制面动作，不混入：
  - 批量踢连接
  - IP / rate limit
  - pairing / scope
  - 更复杂的 connection metadata protocol
- 目标是让 gateway 控制面不只会“看连接”，还可以安全地管理单条 websocket 连接生命周期。

### 本轮测试
- RED:
  - `go test -count=1 ./pkg/gateway -run 'TestDeleteConnectionEndpoint(RemovesClient|ReturnsNotFoundForUnknownClient|RequiresAuth)$'`
- GREEN:
  - `go test -count=1 ./pkg/gateway -run 'TestDeleteConnectionEndpoint(RemovesClient|ReturnsNotFoundForUnknownClient|RequiresAuth)$'`
  - `go test -count=1 ./pkg/gateway`
  - `go test -count=1 ./pkg/gateway ./pkg/config`

## 2026-04-03 gateway 连接列表稳定化补记

### 本轮完成
- `pkg/gateway/server.go`
  - 为 `Client` 新增：
    - `connectedAt`
    - `remoteAddr`
  - websocket 建连时现在会记录连接建立时间和远端地址。
  - `GET /api/v1/connections` 现在返回稳定按 `id` 排序的连接列表，而不是直接泄露 Go map 遍历顺序。
  - 连接列表返回体补上基本控制面元数据：
    - `connected_at`
    - `remote_addr`
    - `session_id`
- `pkg/gateway/server_test.go`
  - 强化 `TestConnectionsEndpoint`，锁定：
    - 返回顺序稳定
    - 元数据字段存在且值正确
  - 新增 `TestStatusEndpointCountsConnectionsDeterministically`，补一个最小状态端点回归。

### 当前语义
- 这一步仍然只做“连接可观测性”最小增强，不混入：
  - richer connection state machine
  - IP / rate limit
  - pairing / scope
  - 更多 runtime diagnostics
- 目标是先让 gateway 控制面返回可读、可比较、不会因 map 顺序抖动的连接视图。

### 本轮测试
- RED:
  - `go test -count=1 ./pkg/gateway -run 'Test(ConnectionsEndpoint|StatusEndpointCountsConnectionsDeterministically)$'`
- GREEN:
  - `go test -count=1 ./pkg/gateway -run 'Test(ConnectionsEndpoint|StatusEndpointCountsConnectionsDeterministically)$'`
  - `go test -count=1 ./pkg/gateway`
  - `go test -count=1 ./pkg/gateway ./pkg/config`

## 2026-04-03 gateway 单连接详情控制面补记

### 本轮完成
- `pkg/gateway/server.go`
  - 新增 `GET /api/v1/connections/{id}`。
  - 复用已有 `requireAuthenticatedAPI()` 做 JWT 鉴权。
  - 复用共享 `describeConnection()`，让单连接详情与连接列表保持同一套返回字段与编码逻辑。
  - 目标连接不存在时返回 `404 Not Found`。
- `pkg/gateway/server_test.go`
  - 新增：
    - `TestGetConnectionEndpointReturnsConnectionDetails`
    - `TestGetConnectionEndpointReturnsNotFoundForUnknownClient`
    - `TestGetConnectionEndpointRequiresAuth`

### 当前语义
- 这一步仍然只补最小控制面读路径，不混入：
  - 批量详情查询
  - richer runtime diagnostics
  - IP / rate limit
  - pairing / scope
- 目标是让 gateway 控制面在“列表”和“单连接详情”之间形成最小闭环：
  - list all
  - inspect one
  - delete one

### 本轮测试
- RED:
  - `go test -count=1 ./pkg/gateway -run 'TestGetConnectionEndpoint(ReturnsConnectionDetails|ReturnsNotFoundForUnknownClient|RequiresAuth)$'`
- GREEN:
  - `go test -count=1 ./pkg/gateway -run 'TestGetConnectionEndpoint(ReturnsConnectionDetails|ReturnsNotFoundForUnknownClient|RequiresAuth)$'`
  - `go test -count=1 ./pkg/gateway`
  - `go test -count=1 ./pkg/gateway ./pkg/config`

## 2026-04-03 gateway max_connections 首批连接治理补记

### 本轮完成
- `pkg/config/config.go`
  - 为 `GatewayConfig` 新增 `MaxConnections int`。
  - 默认值设为 `0`，表示不限制。
- `pkg/config/validator.go`
  - 新增 `gateway.max_connections >= 0` 校验。
- `pkg/config/path_test.go`
  - 更新 runtime reload copy 断言，兼容 `MaxConnections` 新字段。
  - 新增 `TestValidatorRejectsNegativeGatewayMaxConnections`。
- `pkg/gateway/server.go`
  - 新增 `checkConnectionLimit()`。
  - `handleWSChat()` 在升级 websocket 前先检查连接上限。
  - 命中上限时直接返回 `503`，不接受新连接。
- `pkg/gateway/server_test.go`
  - 新增：
    - `TestGatewayRejectsConnectionsAboveConfiguredLimit`
    - `TestGatewayAllowsConnectionsWhenLimitUnset`

### 当前语义
- 这一步只补最小连接数量治理，不混入：
  - per-user / per-IP limits
  - rate limit
  - pairing / scope
  - richer backpressure policy
- 当前规则：
  - `max_connections = 0` 表示无限制
  - `max_connections > 0` 时，达到上限就拒绝新的 websocket 连接

### 本轮测试
- RED:
  - `go test -count=1 ./pkg/gateway ./pkg/config -run 'Test(Gateway(RejectsConnectionsAboveConfiguredLimit|AllowsConnectionsWhenLimitUnset)|ValidatorRejectsNegativeGatewayMaxConnections)$'`
- GREEN:
  - `go test -count=1 ./pkg/gateway ./pkg/config -run 'Test(Gateway(RejectsConnectionsAboveConfiguredLimit|AllowsConnectionsWhenLimitUnset)|ValidatorRejectsNegativeGatewayMaxConnections)$'`
  - `go test -count=1 ./pkg/gateway`
  - `go test -count=1 ./pkg/gateway ./pkg/config`

## 2026-04-03 gateway IP allowlist 下一切片预备

### 当前判断
- `gateway control-plane hardening` 下一步继续沿“最小连接治理”推进。
- 当前优先补 IP allowlist，而不是直接跳到：
  - rate limit
  - pairing / scope
  - 更复杂控制面协议
- 原因：
  - 现有 gateway 已有 origin allowlist、JWT、连接详情/删除、最大连接数。
  - 但远端地址仍未形成任何可配置入口控制，REST 与 websocket 共享边界也还没有这层显式治理。
  - 这是比 rate limit 更基础、实现面更小且验证清晰的收口点。

### 锁定边界
- 新增 `gateway.allowed_ips` 配置。
- 第一版仅支持精确 IP 匹配，不做 CIDR、代理信任链或 `X-Forwarded-For` 解析。
- 同时覆盖：
  - websocket 握手入口
  - REST 控制面入口
- 语义保持最小化：
  - 空列表表示不限制
  - 非空列表表示仅允许命中的远端 IP
  - 解析不到合法 IP 时拒绝访问

### 计划中的验证
- RED:
  - 为 websocket 和 REST 分别补未命中 allowlist 时被拒绝的测试。
  - 补空 allowlist 放行与命中 allowlist 放行测试。
- GREEN:
  - `go test -count=1 ./pkg/gateway ./pkg/config -run 'Test(Gateway(IPAllowlist|CheckClientIP)|ValidatorRejectsBlankGatewayAllowedIPs)$'`
  - `go test -count=1 ./pkg/gateway`
  - `go test -count=1 ./pkg/gateway ./pkg/config`

## 2026-04-03 gateway IP allowlist 收口补记

### 本轮完成
- `pkg/config/config.go`
  - 为 `GatewayConfig` 新增 `AllowedIPs []string`。
- `pkg/config/validator.go`
  - 新增 `gateway.allowed_ips` 校验：
    - 不允许空字符串
    - 必须是合法字面 IP
- `pkg/config/path_test.go`
  - 更新 runtime reload copy 断言，覆盖 `AllowedIPs`。
  - 新增：
    - `TestValidatorRejectsBlankGatewayAllowedIPs`
    - `TestValidatorRejectsInvalidGatewayAllowedIPs`
- `pkg/gateway/server.go`
  - 新增 `checkClientIP()`。
  - websocket 握手入口和 REST 控制面入口现在都先做远端 IP allowlist 校验。
  - 当前语义：
    - `allowed_ips` 为空表示不限制
    - 非空时仅允许命中的远端 IP
    - `RemoteAddr` 解析失败或 IP 不合法时直接拒绝
- `pkg/gateway/server_test.go`
  - 新增：
    - `TestGatewayCheckClientIPAllowsRequestsWhenListUnset`
    - `TestGatewayCheckClientIPAllowsConfiguredIP`
    - `TestGatewayCheckClientIPRejectsUnconfiguredIP`
    - `TestGatewayStatusEndpointRejectsDisallowedIP`
    - `TestGatewayStatusEndpointAllowsConfiguredIP`
    - `TestWSChatRejectsDisallowedIP`

### 当前语义
- 这一步继续只做共享入口级的最小连接治理，不混入：
  - CIDR
  - `X-Forwarded-For`
  - rate limit
  - pairing / scope
- 目标是先让 gateway 在 JWT/origin/max_connections 之外，再补上一层显式的远端地址准入边界。

### 本轮测试
- RED:
  - `go test -count=1 ./pkg/gateway ./pkg/config -run 'Test(Gateway(CheckClientIP(AllowsRequestsWhenListUnset|AllowsConfiguredIP|RejectsUnconfiguredIP)|StatusEndpoint(RejectsDisallowedIP|AllowsConfiguredIP)|WSChatRejectsDisallowedIP)|ValidatorRejectsBlankGatewayAllowedIPs)$'`
- GREEN:
  - `go test -count=1 ./pkg/gateway ./pkg/config -run 'Test(Gateway(CheckClientIP(AllowsRequestsWhenListUnset|AllowsConfiguredIP|RejectsUnconfiguredIP)|StatusEndpoint(RejectsDisallowedIP|AllowsConfiguredIP)|WSChatRejectsDisallowedIP)|ValidatorRejects(BlankGatewayAllowedIPs|InvalidGatewayAllowedIPs))$'`
  - `go test -count=1 ./pkg/gateway ./pkg/config`

## 2026-04-03 gateway per-IP rate limit 收口补记

### 本轮完成
- `pkg/config/config.go`
  - 为 `GatewayConfig` 新增 `RateLimitPerMinute int`。
- `pkg/config/validator.go`
  - 新增 `gateway.rate_limit_per_minute >= 0` 校验。
- `pkg/config/path_test.go`
  - 更新 runtime reload copy 断言，覆盖 `RateLimitPerMinute`。
  - 新增 `TestValidatorRejectsNegativeGatewayRateLimitPerMinute`。
- `pkg/gateway/server.go`
  - 新增 `checkRateLimit()` 与 `getOrCreateRateLimiter()`。
  - 当前按远端 IP 建立 limiter bucket。
  - `requireAuthenticatedAPI()` 与 `handleWSChat()` 现在都在入口先做共享 rate limit 检查。
  - 命中限制时返回 `429 Too Many Requests`。
- `pkg/gateway/server_test.go`
  - 新增：
    - `TestGatewayRateLimitAllowsRequestsWhenUnset`
    - `TestGatewayRateLimitRejectsSecondRequestFromSameIP`
    - `TestGatewayRateLimitUsesPerIPBuckets`
    - `TestGatewayStatusEndpointRejectsRateLimitedRequest`
    - `TestWSChatRejectsRateLimitedRequest`

### 当前语义
- 这一步仍然只做最小入口级限流，不混入：
  - user/session scoped limiter
  - pairing
  - scope
  - 更复杂的 cleanup/eviction 策略
- 当前规则：
  - `rate_limit_per_minute = 0` 表示关闭限流
  - `rate_limit_per_minute > 0` 时，按远端 IP 做共享 bucket
  - REST 与 websocket 握手共用同一套 limiter 状态

### 本轮测试
- RED:
  - `go test -count=1 ./pkg/gateway ./pkg/config -run 'Test(Gateway(RateLimit(AllowsRequestsWhenUnset|RejectsSecondRequestFromSameIP|UsesPerIPBuckets)|StatusEndpointRejectsRateLimitedRequest|WSChatRejectsRateLimitedRequest)|ValidatorRejectsNegativeGatewayRateLimitPerMinute)$'`
- GREEN:
  - `go test -count=1 ./pkg/gateway ./pkg/config -run 'Test(Gateway(RateLimit(AllowsRequestsWhenUnset|RejectsSecondRequestFromSameIP|UsesPerIPBuckets)|StatusEndpointRejectsRateLimitedRequest|WSChatRejectsRateLimitedRequest)|ValidatorRejectsNegativeGatewayRateLimitPerMinute)$'`
  - `go test -count=1 ./pkg/gateway ./pkg/config`

## 2026-04-03 gateway control-plane auth scope 收口补记

### 本轮完成
- `pkg/gateway/server.go`
  - 抽出共享 `authenticateRequest()`，统一解析 gateway JWT 的 `sub` / `uid` / `role`。
  - websocket chat 入口改为复用这条共享认证路径，但仍保持“任意有效已鉴权 token 可接入聊天 websocket”的现有兼容语义。
  - `requireAuthenticatedAPI()` 现在接受 endpoint scope，并在 JWT 鉴权之后继续校验 control-plane role。
  - 对不带 `role` claim 的旧 token 保持兼容，按 legacy admin token 语义回落到 `admin`。
- `pkg/gateway/server_test.go`
  - 新增：
    - `TestGatewayStatusEndpointAllowsMemberRole`
    - `TestGatewayConnectionsEndpointAllowsMemberRole`
    - `TestGetConnectionEndpointAllowsMemberRole`
    - `TestDeleteConnectionEndpointRejectsMemberRole`
    - `TestGatewayAuthenticateRequestAllowsMemberRoleForWebsocketPath`

### 当前语义
- 这一步只补最小 control-plane auth scope，不混入：
  - device pairing / enrollment
  - tenant-aware gateway partitioning
  - websocket chat 本身的权限模型重做
- 当前规则：
  - 任意有效 gateway JWT 仍可用于 websocket chat。
  - `GET /api/v1/status`、`GET /api/v1/connections`、`GET /api/v1/connections/{id}` 现在允许 `member` / `admin` / `owner`。
  - `DELETE /api/v1/connections/{id}` 继续只允许 `admin` / `owner`。
  - 旧的只含 `sub`、不含 `role` / `uid` 的 token 继续按 admin token 兼容，避免现有控制面 token 立即失效。

### 本轮测试
- RED:
  - `go test -count=1 ./pkg/gateway -run 'Test(Gateway(StatusEndpointAllowsMemberRole|ConnectionsEndpointAllowsMemberRole)|DeleteConnectionEndpointRejectsMemberRole|GetConnectionEndpointAllowsMemberRole|AuthenticateRequestAllowsMemberRoleForWebsocketPath)$'`
- GREEN:
  - `go test -count=1 ./pkg/gateway -run 'Test(Gateway(StatusEndpointAllowsMemberRole|ConnectionsEndpointAllowsMemberRole)|DeleteConnectionEndpointRejectsMemberRole|GetConnectionEndpointAllowsMemberRole|AuthenticateRequestAllowsMemberRoleForWebsocketPath)$'`
  - `go test -count=1 ./pkg/gateway ./pkg/config`

## 2026-04-03 gateway member 只读范围收紧补记

### 本轮完成
- `pkg/gateway/server.go`
  - `requireAuthenticatedAPI()` 现在返回 `authContext`，让后续读路径可以继续做 caller-aware 过滤。
  - `GET /api/v1/status` 再次收紧为只允许 `admin` / `owner`，避免把全局连接计数暴露给 `member`。
  - `GET /api/v1/connections` 与 `GET /api/v1/connections/{id}` 现在对 `member` 只返回其本人 `uid` 对应的连接。
  - 新增 `gatewayControlPlaneCanReadConnection()`，把 member 只读范围显式收口到“仅本人连接元数据”。
- `pkg/gateway/server_test.go`
  - 调整并新增：
    - `TestGatewayStatusEndpointRejectsMemberRole`
    - `TestGatewayConnectionsEndpointAllowsMemberRoleForOwnedConnectionsOnly`
    - `TestGetConnectionEndpointAllowsMemberRoleForOwnedConnection`
    - `TestGetConnectionEndpointRejectsMemberRoleForOtherUsersConnection`

### 当前语义
- 这一步是对上一条 endpoint-scope 授权的修正收口，不新增新的控制面协议。
- 当前规则：
  - `status` 仍属于高敏感全局视图，只允许 `admin` / `owner`。
  - `member` 可以看连接，但只能看自己 `uid` 对应的连接列表/详情。
  - `DELETE /api/v1/connections/{id}` 继续只允许 `admin` / `owner`。

### 本轮测试
- GREEN:
  - `go test -count=1 ./pkg/gateway -run 'Test(Gateway(StatusEndpointRejectsMemberRole|ConnectionsEndpointAllowsMemberRoleForOwnedConnectionsOnly)|AuthenticateRequestAllowsMemberRoleForWebsocketPath|GetConnectionEndpoint(AllowsMemberRoleForOwnedConnection|RejectsMemberRoleForOtherUsersConnection))$'`
  - `go test -count=1 ./pkg/gateway`

## 2026-04-03 gateway websocket session pairing reuse 补记

### 本轮完成
- `pkg/gateway/server.go`
  - websocket 握手新增 `resolveGatewaySessionID()`，支持从 `?session_id=` 复用既有 gateway session。
  - 仅允许复用已经存在且 `source == gateway` 的 session；未知 session 或非 gateway session 直接返回 `400`。
  - `processMessage()`、router 调用和 websocket 回复现在统一使用配对后的 gateway session id，而不是临时连接 id。
  - 新增 `gatewaySessionID()`，从 client 绑定的 session 中稳定提取真实 session id。
- `pkg/gateway/server_test.go`
  - 新增：
    - `TestResolveGatewaySessionIDUsesRequestedExistingGatewaySession`
    - `TestResolveGatewaySessionIDRejectsUnknownRequestedSession`
    - `TestResolveGatewaySessionIDRejectsNonGatewaySession`
    - `TestProcessMessageUsesPairedSessionIDForRouterAndResponse`

### 当前语义
- 这一步只补首个 pairing 薄切片，不混入：
  - enrollment / claim token
  - 多设备绑定协议
  - 更复杂的 session ownership 模型
- 当前规则：
  - `/ws/chat?session_id=<gateway-session>` 可复用既有 gateway session。
  - 非 gateway 来源的 session 不允许被 websocket pairing 复用。
  - 配对成功后，消息路由与 websocket 返回的 `session_id` 都稳定对齐到被复用的 gateway session。
  - 无效 `session_id` 现在会在 websocket upgrade 之前被拒绝，真实客户端可观测到 `400`，不再出现先 hijack 再写 HTTP 错误的伪路径。

### 本轮测试
- GREEN:
  - `go test -count=1 ./pkg/gateway -run 'Test(ResolveGatewaySessionID(UsesRequestedExistingGatewaySession|RejectsUnknownRequestedSession|RejectsNonGatewaySession)|ProcessMessageUsesPairedSessionIDForRouterAndResponse)$'`
  - `go test -count=1 ./pkg/gateway`

## 2026-04-03 gateway pairing 握手时序修正补记

### 本轮完成
- `pkg/gateway/server.go`
  - 将 `resolveGatewaySessionID()` 前移到 websocket upgrade 之前。
  - 无效或不可复用的 `session_id` 现在直接走普通 HTTP 错误路径，避免先升级 websocket 再尝试写 `400`。
- `pkg/gateway/server_test.go`
  - 新增 `TestWSChatRejectsUnknownRequestedSessionBeforeUpgrade`，用真实 websocket dial 锁定这个回归点。

### 当前语义
- 这一步不扩展 pairing 能力，只修正已有 pairing 薄切片的握手边界。
- 当前规则：
  - session 复用资格必须先验证通过，才允许 websocket upgrade。
  - 失败时客户端拿到真实 `400 Bad Request`，而不是升级后的连接错误。

### 本轮测试
- RED:
  - `go test -count=1 ./pkg/gateway -run 'Test(WSChatRejectsUnknownRequestedSessionBeforeUpgrade|ResolveGatewaySessionIDRejectsUnknownRequestedSession)$'`
- GREEN:
  - `go test -count=1 ./pkg/gateway -run 'Test(WSChatRejectsUnknownRequestedSessionBeforeUpgrade|ResolveGatewaySessionIDRejectsUnknownRequestedSession)$'`
  - `go test -count=1 ./pkg/gateway`
