# Task Plan: nekobot 功能评估、迁移与收口

> Last Updated: 2026-04-06

## Plan Index

### Source of Truth
- `task_plan.md`
  - 当前主执行计划。
  - 后续活跃开发任务统一收口到这里。

### Referenced Plans
- `provider_model_migration_plan.md`
  - 角色：provider/model 管理层 clean-slate 迁移计划。
  - 状态：当前 active 前置任务。
  - 目标：先完成 provider connection 与 model config 解耦，再恢复当前 task lifecycle 主线。
- `claude_code_alignment_plan.md`
  - 角色：架构对齐参考计划。
  - 状态：仍有效，但作为架构路线输入，不再单独执行。
  - 已吸收到本计划的主题：
    - `task lifecycle service`
    - `runtime control plane`
    - `execenv`
    - `AgentDefinition`
    - `tool governance`
    - `context economy`
- `closure_task_plan.md`
  - 角色：旧的 Web bootstrap / closure 收尾计划。
  - 状态：不作为当前主线，但其中未完成的“verify / deliver”仍保留为次级收尾事项。
- `migration_task_plan.md`
  - 角色：`goclaw` / `gua` 迁移专题计划。
  - 状态：已完成，视为归档参考，不再展开为当前 active backlog。
- `wechat_ilink_task_plan.md`
  - 角色：WeChat iLink 专题迁移计划。
  - 状态：已完成，视为归档参考，不再展开为当前 active backlog。

### Consolidation Rules
- 根目录其他 `*_plan.md` 继续保留原始上下文，不删除。
- 所有仍然有效的未完成任务，后续都要在本文件中有入口，至少以：
  - 主任务
  - 引用任务
  - 已归档任务
  三种状态之一表达。
- 如果某个旁支计划已经完成，只在这里保留引用与结论，不重复维护细项。
- 当前所有开发计划一律**不考虑历史数据迁移问题**。
- 历史数据迁移统一后置：
  - 等所有当前开发任务完成后
  - 再单独立项处理 schema/data migration
  - 当前任何单项开发都不以兼容旧数据为前提约束设计

## Consolidated Active Queue

### P0 当前主线
- 完成 `provider/model` 管理层 clean-slate 迁移计划。
- 当前约束：
  - 不考虑历史数据迁移
  - 不保留旧 provider 数据结构兼容层
- 当前前置任务：
  - `provider_model_migration_plan.md`
- 当前已完成：
  - `subagent -> tasks.Service`
  - `exec.background -> process -> tasks.Service`
  - `agent tool_session spawn -> process -> tasks.Service`
  - `cron executeJob -> tasks.Service`
  - `watch executeCommand -> tasks.Service`
- 当前收口结果：
  - 本轮 `runtime/task lifecycle` 主线已完成
  - 下一步按既定计划转入：
    - `AgentDefinition`
    - `tool governance`
    - `context economy`
    - 以及 `codeany` 吸收项中的 `permission rules` / `context sources`
  - 当前已锁定的最小下一切片：
    - 已完成 `tool governance / permission rules`
    - 已完成 `AgentDefinition / prompt boundary` 最小桥接切片
    - 已完成 `context sources` explainability preview 与 chat route 只读透传
    - 已完成 `readonly preflight` 结构收口、`preflight.action` 暴露，以及 `legacy / blades` orchestrator parity
    - 已完成首个真实执行路径上的非只读 decision 点：
      - 仅当 `preflight.action == compact_before_run` 时
      - 在首次模型调用前做一次瞬时 message compression
      - 覆盖 `legacy / blades` orchestrator parity
      - 不改写 session/history
      - 不阻断请求
      - 不做自动 summary / pruning
    - 已补齐 `preflight.applied` 执行态透传：
      - 仅当运行时真的执行了 `compact_before_run`
      - 才把 `route_result.preflight.applied = true`
      - `warning/consider_compaction` 仍保持 `applied = false`
      - 覆盖 `legacy / blades` parity 与 websocket / ChatPage 展示
    - 已补齐 `route_result.preflight.action` 的 websocket 透传契约：
      - ChatPage 现在能真正收到并显示后端 preflight action
      - 不新增动作语义，仅补齐已存在字段的对外输出
  - 当前已批准的下一开发主线：
    - 先做 `conversation/thread binding` 首批收口
    - 目标不是立刻抽独立存储或接入所有消费者
    - 而是先把 `pkg/conversationbindings` 收敛成真正可复用的通用绑定层
    - 再用现有 `pkg/channels/wechat/runtime.go` 作为首个真实消费者验证该契约
    - `gateway` / `external agent runtime` / 更完整控制面接入留到后续批次

### P1 次级收尾
- [ ] 基于 `docs/LLM_WIKI_MEMORY_SPEC.md` 创建并推进 `LLM Wiki memory` 改造任务入口（参考 `docs/superpowers/plans/2026-04-13-llm-wiki-memory.md`）。
  - 当前已完成：任务入口创建 + 第一轮 brownfield audit 记录（见 `notes.md` 与任务文档）。
- `closure_task_plan.md` 中遗留的 `Phase 5: Verify, commit, and deliver` 仍未在主计划中正式关闭。
- 这条不阻塞当前 runtime/task 主线，但后续需要单独清账。
- 新增 `codeany` 调研吸收项：
  - `tool governance` 阶段补一层“可持久化权限规则”：
    - `always allow`
    - pattern-based `allow`
    - pattern-based `deny`
    - 要求进入 `nekobot` 自己的 config/store/API，而不是简单文件直写
  - `AgentDefinition / context economy` 阶段补“上下文来源可解释面”：
    - 显示当前 prompt/context 由哪些来源组成
    - 至少覆盖 skills / memory / project rules / runtime injected context / MCP related context
    - 目标是给 WebUI 与后续控制面一个 context sources 视图
- 当前切片执行边界：
  - 所有 tool call 统一走 permission rules 入口
  - 第一版规则仅支持：
    - `tool_name`
    - `session_id`
    - `runtime_id`
    - `action = allow|deny|ask`
  - 未命中规则时，继续回落到现有 `approval mode`
  - `context economy` 当前只允许：
    - preview explainability
    - chat route 透传 budget / compaction decision
    - 以 `preflight decision` 结构暴露只读决策结果
    - 以 `preflight.action` 暴露建议动作
    - 对 `compact_before_run` 执行一次瞬时 outbound compression
    - 通过 websocket `route_result.preflight.action` 把建议动作原样透传给 WebUI
  - 当前仍然**不允许**：
    - 对 `warning/consider_compaction` 自动 compaction
    - 自动 pruning
    - 因 budget 状态直接阻断请求
    - 改写持久化 session/history
    - 自动 summary 生成
  - `chat route` 的 context decision metadata 现在要求对齐所有主 orchestrator 路径：
    - legacy
    - blades

### Archived / Reference Only
- `migration_task_plan.md`
- `wechat_ilink_task_plan.md`
- 两者当前不再作为 active backlog 来源，只作为历史设计与验证记录引用。


## 2026-04-02 Provider/Model Clean-Slate 迁移前置批次

### Goal
在继续 `task lifecycle` 主线前，先参考 `axonhub` 完成当前 provider/model 管理层的 clean-slate 重构，把 provider 连接管理和 model 配置管理拆开。

### Phases
- [x] Phase 1: 复核当前 `nekobot` provider/model 耦合结构与 `axonhub` 参考形态
- [x] Phase 2: 形成独立迁移计划文件并接入主计划索引
- [x] Phase 3: 完成端到端重构设计收束并落文档
- [x] Phase 4: 执行 provider connection / model config / runtime call path 数据模型与存储层重构
- [x] Phase 5: 执行 API / WebUI 迁移
- [x] Phase 6: 恢复 `agent tool_session spawn -> process -> tasks.Service` 主线

### Decisions Made
- 当前 provider/model 迁移按 clean-slate 方式做，不考虑旧 provider 数据兼容。
- “历史数据迁移”不再是任何当前 active 计划的设计约束，而是后置专项。
- 本批次只吸收 `axonhub` 的两层拆分思想：
  - provider connections
  - model configs
- 暂不引入 `axonhub` 的完整 channel/model association DSL。

### References
- [`provider_model_migration_plan.md`](/home/czyt/code/nekobot/provider_model_migration_plan.md)
- [`docs/superpowers/specs/2026-04-02-provider-model-redesign-design.md`](/home/czyt/code/nekobot/docs/superpowers/specs/2026-04-02-provider-model-redesign-design.md)

### Status
**Phase 6 Completed** - 已确认根目录现有 `*_plan.md` 全部已纳入本主计划索引；`provider/model` 的后端、API、WebUI 和模型消费入口迁移均已打通，后续 `subagent / exec.background / agent tool_session spawn / cron / watch` 也都已接入共享 `pkg/tasks.Service`，本轮 runtime/task lifecycle 主线已收口。

### Phase 4 Progress
- [x] provider connection-only 基础切片
  - `provider` ent schema 去掉 `models/default_model`
  - `providerstore` 改为持久化连接字段 + `default_weight/enabled`
  - `/api/providers` 改为 connection-only 投影视图
  - 新增 `/api/provider-types`
  - 定向验证通过：
    - `go test -count=1 ./pkg/providerregistry ./pkg/providerstore ./pkg/webui`
- [x] model catalog / model route 存储层
  - 新增 `modelcatalog` / `modelroute` ent schema
  - 新增 `pkg/modelstore`
  - 新增 `pkg/modelroute`
  - 已覆盖：
    - catalog CRUD
    - route CRUD
    - default route
    - effective weight
    - alias / regex lookup
  - 定向验证通过：
    - `go test -count=1 ./pkg/modelstore ./pkg/modelroute`

## 2026-04-03 Conversation/Thread Binding 计划收敛批次

### Goal
把 `conversation/thread binding` 从“已有首批能力但边界仍薄”的 backlog 项，收敛为一个可直接执行的实现计划，并锁定下一批开发只覆盖通用绑定层与 WeChat 首个消费者验证。

### Phases
- [x] Phase 1: 复核 `pkg/conversationbindings`、`pkg/toolsessions`、`pkg/channels/wechat/runtime` 与 `pkg/gateway` 的当前状态
- [x] Phase 2: 比较下一批候选主线，确认 `conversation/thread binding` 优先于 `Slack interactive callback` 收尾
- [x] Phase 3: 形成独立实现计划并接入主计划引用
- [x] Phase 4: 执行通用 binding service 首批收口
- [x] Phase 5: 执行 WeChat runtime 消费者对齐与验证
- [x] Phase 6: 基于结果决定下一批切到 `gateway adoption` 还是 `Slack interactive callback`

### Decisions Made
- 当前优先级已明确调整为：先做 `conversation/thread binding`，后做 `Slack interactive callback` 具体业务闭环。
- 本批次不引入新的 Ent schema，继续复用 `tool sessions` 的 `conversation_key + metadata` 作为持久化边界。
- 本批次不把 `gateway`、`external agent runtime`、`WeChat presenter` 混入同一实现切片。
- 首个消费者验证固定为现有 `pkg/channels/wechat/runtime.go`，因为它已经真实依赖该层，验证价值高且回归面清晰。
- 本轮实现先收口“确定性查询顺序”这一条通用契约：
  - `ListBindings()`
  - `GetBindingsBySession()`
  - `sessionToBindingRecords()`
  现已统一按 `conversation_id` 稳定排序，避免消费者继续依赖写入顺序这一隐式行为。
- WeChat runtime 本轮验证后无需代码改动，说明当前消费者可直接兼容这次通用契约收口。
- 当前下一批主线已确定切到 `gateway control-plane hardening`，优先继续补控制面边界与连接治理，而不是先回到 Slack 继续扩 modal 业务。

### References
- [`docs/superpowers/plans/2026-04-03-conversation-thread-binding.md`](/home/czyt/code/go/nekobot/docs/superpowers/plans/2026-04-03-conversation-thread-binding.md)

### Status
**Completed** - 已完成 `conversationbindings.Service` 首个通用契约收口：补齐绑定查询结果的稳定排序语义，并用 WeChat runtime 做了消费者回归验证。后续已明确优先切到 `gateway control-plane hardening`，先继续收口 gateway 控制面与连接治理，再视结果回到 Slack interactive callback 的后续业务扩展。
- [x] runtime model resolution 改造
  - `agent` 已优先通过 `modelroute` 解析 provider 对应的实际模型
  - route metadata 当前支持 `provider_model_id`
  - `resolveModelForProvider()` 在新数据层存在时优先读 route，查不到时才回落旧逻辑
  - 定向验证通过：
    - `go test -count=1 ./pkg/providerregistry ./pkg/providerstore ./pkg/modelstore ./pkg/modelroute ./pkg/agent ./pkg/runtimeagents ./pkg/webui`
- [x] models/routes API
  - 新增：
    - `/api/models`
    - `/api/model-routes`
  - provider discovery 已开始 merge 到：
    - `model catalog`
    - `model route`
  - discovery 当前会自动补：
    - catalog item
    - route
    - `metadata.provider_model_id`
  - 定向验证通过：
    - `go test -count=1 ./pkg/providerregistry ./pkg/providerstore ./pkg/modelstore ./pkg/modelroute ./pkg/agent ./pkg/runtimeagents ./pkg/webui`
- [x] WebUI Providers/Models 页面迁移
  - 2026-04-02 进度补记：
    - 已确认根目录计划文件：
      - `task_plan.md`
      - `claude_code_alignment_plan.md`
      - `migration_task_plan.md`
      - `provider_model_migration_plan.md`
      - `wechat_ilink_task_plan.md`
      - `closure_task_plan.md`
    - 以上均已在本文件 `Plan Index` 中有入口，无新增遗漏 `_plan.md`
    - 当前前端迁移拆为三段：
      1. 共享 hooks 与类型层：`useProviders` / `useProviderTypes` / `useModels`
      2. Provider 连接管理 UI：`ProviderForm` / `ProvidersPage`
      3. Model 管理 UI 与调用入口：`ModelsPage` + Chat/Config/Cron 模型来源替换
    - 当前已完成：
      - `ProviderForm` 已改为 provider type registry 驱动的 connection-only 表单
      - `ProvidersPage` 已去除 provider-owned model 展示，改为连接与运行态视图
      - 已新增独立 `ModelsPage`
      - `App` / `Sidebar` 已接入 models 路由入口
      - `ChatPage` / `ConfigPage` / `CronPage` 已改从 shared model catalog + routes 读取模型选项
    - 前端验证通过：
      - `npm run build`（`pkg/webui/frontend`）


## 2026-04-02 Task Lifecycle Service 接线收口批次

### Goal
把前一轮已经落到 `pkg/tasks.Service` 的生命周期模型，真正接到现有后台执行主链，而不是继续只停留在 store/snapshot/control-plane 展示层。

### Phases
- [x] Phase 1: 复核当前 `task lifecycle service + runtime control plane` 代码状态与验证覆盖
- [x] Phase 2: 先补测试，锁定 `subagent` 尚未接入 `tasks.Service` 的缺口
- [x] Phase 3: 把 `subagent` 执行链接到 `enqueue -> claim -> start -> complete/fail/cancel`
- [x] Phase 4: 跑定向与扩大的 Go 回归
- [x] Phase 5: 基于当前收口结果决定下一条接入 `tasks.Service` 的执行路径

### Decisions Made
- `subagent` 不再继续只依赖 `ListTaskSnapshots()` 暴露任务，而是正式把生命周期写入 `pkg/tasks.Service`。
- `Agent.EnableSubagents()` 现在直接为 `subagent` 装配共享 task lifecycle service，避免控制面继续依赖“旧 source + 新 managed source”双轨并存。
- `runtime control plane` 当前已具备：
  - `status`
  - `availability_reason`
  - `current_task_count`
  - `last_seen_at`
  - `session_runtime_states`
- 下一批优先不再扩 UI 展示，而是继续把更多真实执行路径接入统一 task lifecycle。
- 后续每次切入新的主线切片前，先用 `codeagent` 拉一轮 `codex + claude` 的收束审查，只允许输出一个推荐方向和一组明确延后项，避免多条设计路线并行演化。

### Verification
- [x] `go test -count=1 ./pkg/subagent ./pkg/agent ./pkg/webui`
- [x] `go test -count=1 ./pkg/tasks ./pkg/subagent ./pkg/agent ./pkg/runtimeagents ./pkg/runtimetopology ./pkg/approval ./pkg/gateway ./pkg/inboundrouter ./pkg/webui`
- [x] `go test -count=1 ./...`

### Errors Encountered
- 暂无新的阻塞错误。

### Phase 5 Decision
- 已确认下一条最值得接入 `pkg/tasks.Service` 的执行路径为：
  - `exec.background -> process`
- 候选顺序：
  1. `exec.background -> process`
  2. `agent tool_session spawn`
  3. `cron executeJob`
  4. `watch executeCommand`
  5. `gateway/router chat` 暂不优先
- `codeagent` 审查收束后的执行规则：
  - 当前只推进 `exec.background -> process`
  - `chat/main loop`、`AgentDefinition`、`tool governance`、`frontend domain stores` 在这一切片完成前都不展开实现

### Status
**Phase 5 Completed** - `subagent` 已正式接入 `pkg/tasks.Service`，并已通过 `codeagent(codex+claude)` 收束出唯一下一主线：`exec.background -> process`。


## 2026-04-02 codeany 调研吸收项批次

### Goal
把 `codeany` 中确认对 `nekobot` 当前方向真正有价值的两点，正式接入主开发计划：一是可持久化权限规则层，二是上下文来源可解释面。

### Phases
- [x] Phase 1: 在 `tool governance` 计划阶段设计 `permission rules` 数据模型与存储入口
- [x] Phase 2: 在 `tool governance` 执行阶段实现规则匹配、持久化与解释信息输出
- [x] Phase 3: 在 `AgentDefinition / context economy` 计划阶段设计 `context sources` 结构与观测视图
- [x] Phase 4: 在后续 WebUI / control plane 阶段实现 context sources 展示

### Decisions Made
- 这两项来自 `codeany` 调研，但只吸收概念与产品边界，不迁入其终端/TUI 形态。
- 当前不新增独立“plugin system”主线，也不引入 `open-agent-sdk-go` 替换 `blades`。
- `permission rules` 必须挂到 `nekobot` 自己的治理链路里，避免形成第二套旁路权限系统。
- `context sources` 应服务于：
  - `AgentDefinition`
  - `prompt section registry`
  - `context economy`
  - WebUI / runtime control plane 的解释性展示

### Ordering
- 这两项属于已确认的后续吸收项，但不插队当前主线。
- 当前顺序调整为：
  1. 进入 `AgentDefinition / tool governance / context economy`
  2. 在对应阶段落地 `permission rules`
  3. 在对应阶段落地 `context sources`

### References
- [`notes.md`](/home/czyt/code/nekobot/notes.md)

### Status
**Initial Scope Completed** - 当前只读边界内的 `codeany` 吸收项已完成首轮落地：`permission rules` MVP、`AgentDefinition` bridge、`context sources` preview，以及 `context economy` 的 readonly `preflight` 元数据都已接入预览 API 与 chat route。后续剩余工作不再是“初始落地”，而是进入首个真实 runtime decision 切片。


## 2026-04-02 tool governance / permission rules 最小闭环批次

### Goal
为 `nekobot` 引入第一版可持久化 `permission rules`，让所有 tool call 统一先经过规则评估，再回落到现有 `approval.Manager` 的 mode/pending 流程，形成最小可执行的 tool governance 闭环。

### Phases
- [x] Phase 1: 收敛当前 approval/tool execution 现状，并锁定最小设计边界
- [x] Phase 2: 落 spec、计划与数据模型设计
- [x] Phase 3: 实现 `permission rule store + evaluator`
- [x] Phase 4: 实现 `agent tool execution -> permission rules -> approval fallback`
- [x] Phase 5: 实现最小 API / WebUI 管理面并完成验证

### Decisions Made
- 先做独立的 `permission rules` 层，不把规则直接揉进 `approval.Manager`。
- 所有 tool call 都走统一规则入口，但第一版匹配维度只做：
  - `tool_name`
  - `session_id`
  - `runtime_id`
  - `action`
- 第一版 action 只有：
  - `allow`
  - `deny`
  - `ask`
- 第一版不做：
  - 参数级匹配
  - classifier
  - pre/post/failure hooks
  - 复杂 explainability UI
- 未命中规则时，继续回落到现有 `approval mode` 行为，避免打散现有系统。

### References
- [`docs/superpowers/specs/2026-04-02-tool-governance-permission-rules-design.md`](/home/czyt/code/nekobot/docs/superpowers/specs/2026-04-02-tool-governance-permission-rules-design.md)
- [`claude_code_alignment_plan.md`](/home/czyt/code/nekobot/claude_code_alignment_plan.md)

### Status
**Phase 5 Completed** - `permission rules` 最小闭环已落地并已切成独立功能批次：已补 ent schema、持久化 manager、evaluator、agent 执行入口接线、强制 pending approval 的 `ask` 语义、WebUI CRUD API，以及最小前端管理页。当前已通过定向 Go 回归、`cmd/nekobot` 回归与前端构建验证；该批次提交后，主线继续进入首个真实 runtime decision 切片，而不再新增只读 preview 面。


## 2026-04-02 AgentDefinition / prompt boundary 最小桥接批次

### Goal
把当前运行中的主 agent 配置收敛成一个可读取的 `AgentDefinition` 兼容快照，并把 system prompt 明确拆成 stable/dynamic sections，为后续 `context sources` 和 definition-driven runtime 铺路。

### Phases
- [x] Phase 1: 复核当前 `AgentDefinition` 计划边界与现有 `ContextBuilder`/`Agent` 实现
- [x] Phase 2: 先补 RED 测试，锁定 `AgentDefinition` compatibility bridge 与 prompt section 边界
- [x] Phase 3: 实现 `AgentDefinitionFromRuntimeConfig` 与 `ContextBuilder.BuildPromptSections()`
- [x] Phase 4: 把 definition 快照挂到 `Agent` 和 `/api/status`
- [x] Phase 5: 更新计划并切到最小 `context sources` 预览

### Decisions Made
- 当前不直接切完整多-definition runtime，只先做 compatibility bridge。
- 当前 `AgentDefinition` 第一版只覆盖：
  - route default
  - permission mode
  - tool policy allow/deny
  - max tool iterations
  - prompt section boundary metadata
- `ContextBuilder` 继续输出原有 system prompt 文本，但内部先统一经过 section 列表装配。
- `System` 页先只读展示 definition snapshot，不提供编辑能力。
- 下一切片不做完整 `context economy`，只先交付最小 explainability preview：
  - 复用 `BuildPromptSections()`
  - 复用 `prompts.Resolve(...)`
  - 不额外引入第二套 prompt 装配系统

### Status
**Phase 5 Completed** - `AgentDefinition` 已作为独立功能批次完成并推送；当前剩余活跃切片已收敛为 `context sources` explainability 与 readonly `preflight` chat-route parity 的独立收口，不再混入 definition snapshot 相关改动。


## 2026-04-02 context sources 最小 explainability preview 批次

### Goal
为当前 prompt/runtime 组合增加一个最小可解释的 `context sources` 预览能力，让 WebUI 能看到一次请求会由哪些上下文来源组成，而不提前引入完整的 context economy/token budget 系统。

### Phases
- [x] Phase 1: 基于现有 `ContextBuilder` / `prompts.Resolve` 收敛最小数据边界
- [x] Phase 2: 先补 RED 测试，锁定 agent 侧和 WebUI 侧返回形状
- [x] Phase 3: 实现 `Agent.PreviewContextSources()` 与 `/api/prompts/context-sources`
- [x] Phase 4: 实现 Prompts 页预览面板与多语言文案
- [x] Phase 5: 跑定向与扩大的回归验证

### Decisions Made
- 当前只做 explainability preview，不做 context 截断、权重预算、来源排序策略。
- 数据来源复用现有运行时边界：
  - prompt sections
  - managed prompts resolve
  - runtime route metadata
  - MCP 配置
  - preprocessor preview
- 当前至少覆盖这些来源类型：
  - `project_rules`
  - `skills`
  - `memory`
  - `managed_prompts`
  - `runtime_context`
  - `mcp`
- 当前额外补了一层轻量 `footprint` 观测：
  - system chars
  - memory chars
  - managed prompt chars
  - final user chars
  - referenced file chars
  - mention count
  - warning signals
- 当前进一步补了显式 `budget status`：
  - `ok`
  - `warning`
  - `critical`
  - 以及 `budget reasons`
- 当前再补了一层 `compaction recommendation`：
  - 是否建议压缩
  - 建议策略
  - 预计可节省字符数
  - recommendation reasons
- 这层 `footprint` 只做 explainability，不参与真实 runtime 的自动裁剪或 provider request budget 决策。
- `budget status` 当前也只做 explainability 和前端提示，不触发真实 runtime 的自动 compaction / pruning / request blocking。
- `compaction recommendation` 当前也只做 explainability，不会自动改写会话历史、memory 或引用文件载荷。
- 展示入口先放在 `Prompts` 页面，作为 prompt/runtime 解释工具；暂不扩到独立控制面页面。

### Verification
- [x] `go test -count=1 ./pkg/agent ./pkg/webui -run 'Test(PreviewContextSources_IncludesKeySourceTypes|PromptHandlers_ContextSourcesPreview)'`
- [x] `go test -count=1 ./pkg/agent ./pkg/webui ./pkg/prompts`
- [x] `cd pkg/webui/frontend && npm run build`

### Status
**Phase 5 Completed** - `context sources` 最小 explainability preview 已落地，后端可按单次请求返回来源列表、managed prompt 文本、预处理输入、轻量 `footprint` 指标、显式 `budget status`、`compaction recommendation` 以及嵌套 `preflight` 决策结构，前端已在 `Prompts` 页面提供预览面板。下一步可继续沿 `context economy` 主线扩展真正的预算/排序/来源治理能力。


## 2026-04-03 Context Economy readonly preflight / orchestrator parity 批次

### Goal
把 `context economy` 从“预览 explainability + chat route 平铺元数据”推进到统一的只读 `preflight` 决策面，补齐 `legacy / blades` 两条主 orchestrator 路径的一致性，并在前端展示最小建议动作。

### Phases
- [x] Phase 1: 收敛 preview / chat route 现有 budget 与 compaction 元数据边界
- [x] Phase 2: 实现嵌套 `preflight` 结构，并保留平铺字段作为兼容过渡
- [x] Phase 3: 补齐 `legacy / blades` 两条 chat 主链的 readonly preflight parity
- [x] Phase 4: 在 Chat / Prompts 前端接上 `preflight` 契约并补 `warning` 路径验证
- [x] Phase 5: 跑定向 Go 回归与前端构建，确认该切片可独立收口

### Decisions Made
- `preflight` 当前只表达只读建议，不直接驱动真实 runtime 行为。
- 当前建议动作规则保持最小集合：
  - `ok -> proceed`
  - `warning -> consider_compaction`
  - `critical -> compact_before_run`
- websocket chat route 和 preview API 优先暴露嵌套 `preflight`，但保留原有平铺 budget/compaction 字段，避免一次性打断现有前端消费方。
- `Prompts` 页面与 `Chat` 页面都优先读取 `preflight`，旧字段只作为兼容 fallback。
- 当前仍然不做：
  - 自动 compaction
  - 自动 pruning
  - 因 `critical` 直接阻断请求
  - 决策历史持久化

### Verification
- [x] `go test -count=1 ./pkg/agent -run 'Test(PreviewContextSources_IncludesKeySourceTypes|PreviewContextSources_ReportsWarningBudgetStatusForMemoryPressure|PreviewContextSources_ReportsCriticalBudgetStatus|ChatWithPromptContextDetailed_IncludesContextPressurePreview|ChatWithPromptContextDetailed_BladesIncludesContextPressurePreview)'`
- [x] `go test -count=1 ./pkg/webui -run 'Test(PromptHandlers_ContextSourcesPreview|ChatRouteStateJSONIncludesContextPressureFields)'`
- [x] `cd pkg/webui/frontend && npm run build`

### Status
**Phase 5 Completed** - readonly `preflight` 已作为 canonical 决策结构接入 preview API 与 chat route，`preflight.action` 已在 Chat 和 Prompts 侧最小展示，`legacy / blades` orchestrator 已完成一致性对齐，旧平铺字段继续保留作兼容过渡。当前该批次正按独立功能切片提交收口；提交推送后，下一步才进入首个非只读 runtime decision 切片。


## 2026-04-02 watch executeCommand -> tasks.Service 接线批次

### Goal
把 `watch executeCommand` 触发的本地 shell 执行接入共享 `pkg/tasks.Service`，让文件变更触发命令在控制面里具备统一的生命周期可见性。

### Phases
- [x] Phase 1: 复核 `pkg/watch` 当前执行入口、`execenv` 预处理链和共享 task lifecycle 边界
- [x] Phase 2: 先补 RED 测试，锁定 watch 命令成功/失败时的 managed task 行为
- [x] Phase 3: 实现 `watch executeCommand -> tasks.Service` 接线
- [x] Phase 4: 跑 watch/webui/全仓验证，确认 CI 通过
- [x] Phase 5: 更新主计划并切换到后续治理主线

### Decisions Made
- `watch` 为每次触发生成独立的 managed task，不复用底层 shell 进程 id。
- 当前复用 `tasks.TypeLocalAgent` 表达 watch 驱动的本地命令执行，不额外新增 task type。
- `RuntimeID` 固定为 `watch`，`SessionID` 使用 `watch:<patternIdx>`，便于把同一 watch pattern 的执行归到稳定会话范围。
- task metadata 至少记录：
  - `source=watch`
  - `file`
  - `op`
  - `pattern`
  - `command`
  - `fail_command`
- WebUI `/api/status` 不扩 schema，继续直接消费现有 `tasks.Store` 聚合结果。

### Verification
- [x] `go test -count=1 ./pkg/watch`
- [x] `go test -count=1 ./pkg/watch ./pkg/webui`
- [x] `go test ./...`
- [x] `cd pkg/webui/frontend && npm ci && npm run build`

### Status
**Phase 5 Completed** - `watch executeCommand -> tasks.Service` 已落地并通过本地 CI；当前 `runtime/task lifecycle` 主线已全部完成，后续转入 `AgentDefinition / tool governance / context economy` 与已登记的 `codeany` 吸收项。


## 2026-04-02 cron executeJob -> tasks.Service 接线批次

### Goal
把 `cron executeJob` 触发的 agent 执行纳入共享 `pkg/tasks.Service`，让定时任务在控制面里具备统一的生命周期可见性。

### Phases
- [x] Phase 1: 复核 `pkg/cron` 当前执行入口与共享 task lifecycle 边界
- [x] Phase 2: 先补测试，锁定 cron job 执行成功/失败时的 managed task 行为
- [x] Phase 3: 实现 `cron executeJob -> tasks.Service` 接线
- [x] Phase 4: 跑 cron/webui/全仓验证，确认 CI 通过
- [x] Phase 5: 切换到下一条唯一主线

### Decisions Made
- `cron` 采用独立的 managed task，而不是复用 chat 内部 task id。
- 每次 cron run 生成独立 task id，`SessionID` 复用 `job.ID`，`RuntimeID` 固定为 `cron`。
- 当前复用 `tasks.TypeLocalAgent` 表达 cron 驱动的本地 agent 执行，不额外新增 task type。
- 只向执行上下文注入 `runtime_id=cron`，不把 cron run task id 向内继续透传为工具链的父 task id，避免与现有 process-backed task 复用语义冲突。
- WebUI `/api/status` 与 runtime topology 不扩 schema，直接消费现有聚合任务视图。

### Verification
- [x] `go test -count=1 ./pkg/cron`
- [x] `go test -count=1 ./pkg/webui -run 'TestHandle(CreateCronJob_AcceptsRouteOverrides|CreateCronJob_AtScheduleValidatesRFC3339|RunCronJob_NotFound|RunCronJob_DisabledJobDoesNotExecute|CronJobLifecycle)'`
- [x] `go test ./...`
- [x] `cd pkg/webui/frontend && npm ci && npm run build`

### Status
**Phase 5 Completed** - `cron executeJob -> tasks.Service` 已落地并通过本地 CI；当前唯一剩余主线为 `watch executeCommand -> tasks.Service`。


## 2026-04-02 exec.background -> process 接线收口批次

### Goal
把 `exec.background` 创建出来的后台 PTY 进程正式接入共享 `pkg/tasks.Service`，让 process-backed 任务进入统一的 `enqueue -> claim -> start -> complete/fail/cancel` 生命周期，而不是只返回一个 session ID。

### Phases
- [x] Phase 1: 复核 `exec.background`、`process.Manager` 与共享 `taskStore/taskService` 的现状
- [x] Phase 2: 先补 RED 测试，锁定 process 生命周期与缺省 taskID 行为
- [x] Phase 3: 实现 `exec.background -> process -> tasks.Service` 接线
- [x] Phase 4: 跑切片相关与外围回归，确认 `/api/status` 聚合链未回退
- [ ] Phase 5: 切换到下一条主线前，重新做一次方向收束审查

### Decisions Made
- `process.Manager` 而不是 `ExecTool` 负责 process-backed task 的生命周期迁移。
- `Agent` 统一持有共享 `taskStore` 与 `taskService`，并同时装配给 `processMgr` 与 `subagent`，避免多个 `tasks.Service` 实例对同一 store 造成 source 冲突。
- 只有携带 task 语义的 process session 才进入 managed lifecycle；当前切片先覆盖 `exec.background`。
- `exec.background` 在没有显式 `TaskID` 时，使用生成的 `sessionID` 作为 `TaskID`。
- `Kill`/`Reset` 只标记 cancel 请求，最终 `Cancel` 状态由 `waitForExit` 统一发出，避免重复终态写入。
- `/api/status` 现有的 task/runtime 聚合测试已经覆盖 runtime worker 对 `CurrentTaskCount` 的影响，因此本切片不额外扩 schema。

### Verification
- [x] `go test -count=1 ./pkg/process ./pkg/tools ./pkg/agent`
- [x] `go test -count=1 ./pkg/process ./pkg/tools ./pkg/agent ./pkg/webui`
- [x] `go test -count=1 ./pkg/runtimeagents ./pkg/runtimetopology ./pkg/tasks`
- [x] `go test -count=1 ./pkg/subagent ./pkg/approval ./pkg/inboundrouter ./pkg/gateway`
- [x] `go test -count=1 ./...`

### Errors Encountered
- 收口审查发现 `process.Manager.Kill()` 存在 cancel/fail 分类竞态：
  - 旧实现先 `kill` 再标记 cancel，理论上可能把用户主动终止记成 `failed`。
  - 已在本切片内修复为先标记 cancel，再发送 kill signal，并补回归锁定顺序。
- `go test -count=1 ./...` 仍会间歇失败于 `pkg/gateway`：
  - `TestProcessMessageDoesNotEmitInboundWhenRouterHandlesWebsocketChat`
  - 本轮切片相关定向回归全部通过，但仓库级全量验证仍受这条已知基线问题影响。

### Status
**Phase 5 Completed** - `exec.background -> process -> tasks.Service` 已落地，补修了 `Kill()` cancel 竞态并通过切片相关回归。仓库级全量验证仍被既有 `pkg/gateway` 基线问题阻塞；下一条唯一主线已收束为 `agent tool_session spawn -> process -> tasks.Service`。


## 2026-04-01 Claude Code Deep Dive V2 计划补强批次

### Goal
把 `ai-agent-deep-dive-v2.pdf` 中补充出来的 Claude Code 运行时细节，具体映射到 `nekobot` 的开发目标里，尤其补齐 `query loop`、`streaming tool execution`、工具治理流水线、上下文预算、cleanup contract 这些此前还偏抽象的基础设施目标。

### Phases
- [x] Phase 1: 提取 PDF 中比前两轮文档更细的 runtime / tool / context / lifecycle 设计点
- [x] Phase 2: 对照现有主计划，找出仍然缺少明确实现目标的部分
- [x] Phase 3: 把新结论写入 `notes.md`、`claude_code_alignment_plan.md` 与本计划文件
- [ ] Phase 4: 按更新后的目标继续推进当前 `Phase 2` 开发

### Decisions Made
- 后续不仅要有 `task lifecycle service`，还要预留 turn 级状态表达，避免 chat/main loop 继续被 handler 粗粒度包住。
- 工具治理阶段必须预留 `streaming tool execution` 插槽，不把工具调用固化成“整轮输出后再批处理”。
- prompt 系统的目标是 `section-based assembler + stable/dynamic boundary + capability-aware sections`。
- `execenv` 阶段必须包含 cleanup contract 和 resume metadata，而不是只做环境准备。
- 上下文经济学后续要明确落成：task/session summary、tool result summary、reactive compact fallback、token budget aware continuation。

### New Development Tasks Added
- 在 `tasks/service` 设计里预留 runtime claim/report 语义。
- 在 runtime control-plane 里加入 active task / effective status / last seen 视图。
- 在后续 `AgentDefinition` 阶段补 `prompt section registry` 与稳定/动态 section 边界。
- 在 tool governance 阶段补 `classifier -> hooks -> permission -> execution -> post/failure hooks` 流水线。
- 在 execenv/daemon 阶段补 `cleanup handlers`、resume metadata 与后台任务资源清理语义。

### Status
**Phase 4 In Progress** - Deep Dive V2 的新增结论已吸收到计划文件，当前继续执行 `task lifecycle service + runtime control plane` 这一批开发。

## 2026-04-01 Multica 参考后计划补强批次

### Goal
把 `/home/czyt/code/multica` 里已经验证过的 `task lifecycle service`、`runtime/daemon/execenv` 分层、`runtime` 一等资源控制面，以及前端按 domain store 拆分状态的做法，映射到 `nekobot` 当前的 agent/channel/account/runtime/harness 演进路线中，并补成可执行开发任务。

### Phases
- [x] Phase 1: 提取 `multica` 中对当前 `nekobot` 最有价值的架构点
- [x] Phase 2: 对照现有 `task_plan.md` / `claude_code_alignment_plan.md` / `notes.md` 找出缺失任务
- [x] Phase 3: 重排后续开发顺序，补充新的中间阶段与验证项
- [ ] Phase 4: 完成当前 `session runtime state` 收口后，按新顺序进入下一批开发

### Decisions Made
- `nekobot` 后续不能只补 `tasks.Store`，还需要一个明确的 `task lifecycle service`，统一表达：
  - `enqueue`
  - `claim`
  - `start`
  - `complete/fail/cancel`
  - `summary/event propagation`
- `runtime` 需要从“绑定和路由配置”进一步提升为“一等资源控制面对象”，后续应补：
  - `status`
  - `last_seen`
  - `device/runtime metadata`
  - `usage/activity`
  - `task capacity / concurrency`
- `daemon` 与 `agent execution` 要分层，不应继续把所有执行语义都堆回 `pkg/agent.Agent`。
- `execenv` 需要成为独立关注点，后续用于承接：
  - workdir 生命周期
  - provider/runtime 注入
  - task 上下文注入
  - resume / reuse policy
- 前端控制面后续应从“页面各自抓取配置”逐步转向按 domain store 拆分，优先拆：
  - runtime topology
  - chat runtime/session state
  - harness/task center
  - workspace/global config references

### New Development Tasks Added
- 当前批次完成后，新增以下开发任务进入主计划：
  - 统一 `session_runtime_states` 收口并接入 `/api/status` 与 System 页
  - 引入 `task lifecycle service`，把 task 状态迁移从零散 manager 调用提升为独立服务层
  - 新增 runtime control-plane 字段与接口：`status/last_seen/metadata/activity`
  - 拆分 `daemon/runtime execution/execenv` 边界，避免继续向 `Agent` 聚合
  - 规划 WebUI domain store 重组，先从 runtime/chat/harness 三块开始

### Status
**Phase 4 In Progress** - `multica` 参考点已经吸收到计划，下一步先完成当前 `session runtime state` 收口，再按新的阶段顺序继续推进。

## 2026-04-01 Claude Code 文档吸收与计划重构批次

### Goal
把 `/home/czyt/code/claude-code/docs` 中新增的 Claude Code 架构/提示词/设计解析文档转化为 `nekobot` 的可执行开发路线，收敛哪些能力应该先学、哪些能力必须延后，以及每一阶段的真实依赖关系。

### Phases
- [x] Phase 1: 提取文档里的共性设计原则，避免继续按功能名堆计划
- [x] Phase 2: 对照现有 `claude_code_alignment_plan.md`，找出需要补强的阶段定义
- [x] Phase 3: 更新 `notes.md`、`claude_code_alignment_plan.md` 与本计划文件
- [x] Phase 4: 按新计划继续完成当前 in-flight 的任务运行态与系统页收口验证

### Decisions Made
- 当前最值得学习的不是 Claude Code 的彩蛋能力，而是以下基础设施：
  - 模块化 prompt 装配
  - 统一任务模型 + 运行态 store
  - 工具调用治理链（hook + permission + execution）
  - capability-aware prompt 注入
  - context compaction / transcript / resume
  - daemon + inbox + coordinator 的分层实现
- `AgentDefinition` 必须比当前理解更宽，至少覆盖：
  - prompt policy
  - tool allow/deny
  - permission mode
  - hooks
  - MCP requirements / instructions
  - context policy
- `Phase 1` 之后不能直接跳 specialist agent；中间必须先有：
  - runtime store
  - task-scoped tool assembly
  - hook/policy pipeline
- `Bridge` / `Ultraplan` 继续后置。

### Output
- `notes.md` 已补充 Claude Code 文档结论。
- `claude_code_alignment_plan.md` 已改成更细的分层路线和 delivery waves。
- 后续开发将按新的 phase/wave 顺序推进，而不是按功能名平铺。

### Status
**Completed** - 文档结论已经吸收到计划中，并已完成当前任务运行态观测链路收口；后续按新路线继续推进 runtime store / AgentDefinition / tool governance。



## 2026-04-01 Runtime State Store Skeleton 批次

### Goal
在现有共享 `tasks.Task` 和 task observability 的基础上，引入最小 `runtime state store` 骨架，把任务快照来源注册/聚合从单一 `subagent` 出口提升为统一 store，为后续接入 main chat、specialist agent、daemon 与 coordinator 打基础。

### Phases
- [x] Phase 1: 新增 `pkg/tasks.Store`，支持多 source 注册与聚合
- [x] Phase 2: `agent` 改为通过 `taskStore` 暴露任务，而不是直接返回 `subagent.ListTaskSnapshots()`
- [x] Phase 3: `webui` 改为依赖统一 `taskStore`
- [x] Phase 4: 跑格式化与定向 Go 回归
- [x] Phase 5: 更新计划与笔记

### Decisions Made
- 当前 store 只解决“聚合多个 task source”的问题，不在这一轮引入事件流、订阅、持久化或锁外缓存。
- `subagents` 通过命名 source 注册到 `Agent.taskStore`；这让后续接 `main_chat`、`verify_agent`、`daemon_worker` 时不需要再改 WebUI 协议。
- `webui.Server` 也改为依赖 `taskStore`，避免控制面继续绑死在某个具体 source 上。

### Verification
- [x] `gofmt -w pkg/tasks/task.go pkg/tasks/store.go pkg/tasks/store_test.go pkg/agent/agent.go pkg/webui/server.go pkg/webui/server_status_test.go`
- [x] `go test -count=1 ./pkg/tasks ./pkg/agent ./pkg/subagent ./pkg/webui`

### Status
**Completed** - 最小 runtime state store 骨架已经落地，当前 task 可观测链路不再依赖单一 source，后续可继续把更多 execution unit 接入统一 store。

## 2026-04-01 Execenv 最小基座批次

### Goal
为后续 daemon/runtime worker/background task 抽出最小 `execenv` 基座，先把进程启动前的工作目录归一化、环境注入、cleanup contract 从 `process.Manager` 内联逻辑里分离出来。

### Phases
- [x] Phase 1: 新增 `pkg/execenv`，定义 `StartSpec`、`Prepared`、`Preparer`
- [x] Phase 2: 实现默认本地 preparer，收口 workdir 归一化、目录准备和 runtime/task env 注入
- [x] Phase 3: 改造 `pkg/process.Manager` 支持 `SetPreparer` 与 `StartWithSpec`
- [x] Phase 4: 为 cleanup hook/preparer 集成补测试并跑定向回归
- [x] Phase 5: 更新计划状态，作为 Phase 3 的阶段性里程碑

### Decisions Made
- 当前 `execenv` 只做最小本地准备层，不在这一轮引入 worktree reuse、skill injection 或 daemon supervisor。
- `process.Manager.Start()` 保持兼容，通过委托 `StartWithSpec()` 接入新层，避免现有调用方大面积改造。
- cleanup contract 先以 `Prepared.Cleanup` 落位，并保证 `Reset` 与自然退出路径只执行一次。
- runtime/task/session 元数据先通过环境变量注入，后续再扩展 resume metadata 和 daemon-facing contracts。

### Verification
- [x] `gofmt -w pkg/execenv/*.go pkg/process/*.go`
- [x] `go test -count=1 ./pkg/execenv ./pkg/process`
- [x] `go test -count=1 ./pkg/process ./pkg/tools ./pkg/webui`

### Status
**Completed** - `execenv` 最小基座已经落地，`process.Manager` 不再硬编码全部启动准备逻辑；当前已补到工具/Tool Session 主链的 resume metadata，Phase 3 后续继续补 daemon-facing contract 与更多后台入口复用。

## 2026-04-01 Task Runtime 观测链路批次

### Goal
把已经引入的共享 `tasks.Task` 模型真正向上透出到 WebUI 控制面，让系统页和状态接口可以看见后台 task 的数量、状态分布和近期执行记录，作为后续 runtime store / daemon / coordinator 的最小可观测基础。

### Phases
- [x] Phase 1: 为 `agent/subagent` 收口统一 task snapshot 出口
- [x] Phase 2: 扩展 `/api/status`，返回 `task_count` / `task_state_counts` / `recent_tasks`
- [x] Phase 3: 扩展 `SystemPage` 与状态 hook，展示任务运行态摘要和近期任务
- [x] Phase 4: 跑 Go 定向回归、共享包回归与前端 build
- [x] Phase 5: 更新计划与笔记，记录该 slice 已完成

### Decisions Made
- 这一轮不新开单独任务页，先把最小任务观测能力并入既有 `System` 页。
- `recent_tasks` 先按最近 `completed_at > started_at > created_at` 排序，避免前端自己猜排序语义。
- 状态接口只返回摘要和近期记录，不在这一轮引入更复杂的 task filtering / pagination。
- 当前切片只覆盖已接入共享 task 模型的后台 subagent；前台 chat/main loop 改造成 task-backed 运行时属于下一阶段。

### Verification
- [x] `gofmt -w pkg/agent/agent.go pkg/webui/server.go pkg/webui/server_status_test.go`
- [x] `go test -count=1 ./pkg/webui -run 'TestHandleStatus_ReturnsExtendedFields|TestHandleStatus_IncludesRecentTasks|TestBuildWebUIChatPromptContextIncludesExplicitRuntimeID|TestHandleUndoChatSession|TestClearChatSessionRemovesUndoSnapshots|TestResolveWebUIRuntimeSelectionUsesRuntimeDefaults|TestResolveWebUIRuntimeSelectionRejectsUnboundRuntime|TestResolveWebUIRuntimeSelectionFallsBackToRequestedRoute'`
- [x] `go test -count=1 ./pkg/agent ./pkg/subagent ./pkg/tools ./pkg/webui ./pkg/gateway ./pkg/inboundrouter`
- [x] `npm --prefix pkg/webui/frontend run build`

### Status
**Completed** - 共享 task snapshot 已接到 WebUI status/system 控制面，前后端展示与回归验证通过，可作为后续 runtime store 与任务中心扩展的基础。

## 2026-03-31 Chat 显式 Runtime 选路批次

### Goal
把 runtime/account 多绑定模型真正接入 WebUI/Gateway 聊天主链，支持用户在聊天时显式指定 `runtime_id`，并让该选择同时影响选路结果、会话隔离、undo/clear 行为与前端控制面展示。

### Phases
- [x] Phase 1: 核对 gateway websocket、webui chat、inbound router 当前 runtime 选路现状
- [x] Phase 2: 先补后端测试，锁定显式 runtime 未透传的问题
- [x] Phase 3: 修改 router / gateway / webui / chat 前端，接通显式 runtime 选路与 runtime 作用域 session
- [x] Phase 4: 跑定向测试、前端构建与全量 Go 回归
- [x] Phase 5: 更新 notes/task_plan，并准备提交推送

### Key Questions
1. 显式 runtime 选择应当挂在哪个聊天协议字段上，才能最小改动接入 Gateway 与 WebUI？
2. 当一个 channel account 绑定多个 runtime 时，用户指定 runtime 后是否仍应广播给其他 runtime？
3. WebUI 的 undo / clear / prompt session 绑定，是否需要跟随 runtime 作用域隔离？

### Decisions Made
- 使用 `runtime_id` 作为聊天协议里的显式选路字段，统一透传到 gateway websocket 与 webui chat websocket。
- 当显式指定 runtime 时，只路由到该 runtime；不再按 multi-agent 模式广播到其他已绑定 runtime。
- WebUI chat 的 session、prompt binding、undo、clear 统一切到 runtime 作用域：
  - 默认：`webui-chat:<username>`
  - 显式 runtime：`route:<runtimeID>:webui-chat:<username>`
- 保持当前 multi-agent reply label 语义不变；显式 runtime 命中多 agent binding 时，返回消息仍可带 runtime 名称前缀，避免来源歧义。

### Verification
- [x] `go test -count=1 ./pkg/inboundrouter -run 'TestChatWebsocketFallsBackWithoutTopologyBinding|TestChatWebsocketUsesExplicitRuntimeSelection'`
- [x] `go test -count=1 ./pkg/gateway -run TestProcessMessagePassesExplicitRuntimeIDToRouter`
- [x] `go test -count=1 ./pkg/webui -run 'TestBuildWebUIChatPromptContextIncludesExplicitRuntimeID|TestHandleUndoChatSession|TestClearChatSessionRemovesUndoSnapshots'`
- [x] `go test -count=1 ./pkg/webui ./pkg/gateway ./pkg/inboundrouter`
- [x] `npm --prefix pkg/webui/frontend run build`
- [x] `go test -count=1 ./...`

### Status
**Completed** - 已完成 Chat 显式 runtime 选路、runtime 作用域会话隔离、前后端控制面接通与全量回归验证，待提交推送本轮代码。

## 2026-03-31 Chat Session 桶路由与显式错误态收口批次

### Goal
修正 WebUI Chat 在 runtime 切换、多 session 并行与查询失败场景下的前端体验和协议一致性，避免 websocket 响应、错误消息或路由结果落入错误会话桶，同时把 provider/prompt/runtime 的加载失败从“伪空态”改成显式错误态。

### Phases
- [x] Phase 1: 审核 `useChat` / `ChatPage` / `webui chat ws` 当前半重构状态，确认 session 桶归属风险
- [x] Phase 2: 调整 chat websocket 协议，补齐 `session_id` 回包并让错误路径也带会话标识
- [x] Phase 3: 修改前端 `useChat`，按服务端 `session_id` 精确入桶，不再只依赖本地 pending session 猜测
- [x] Phase 4: 为 Chat 页 runtime/provider/prompt 加载失败补显式错误卡片与三语文案
- [x] Phase 5: 跑定向 Go 回归与前端 build，确认聊天主链重新稳定

### Decisions Made
- Chat websocket 的 `message` / `error` / `system` / `pong` / `route_result` 全部允许携带 `session_id`，由服务端声明当前响应归属。
- 前端 `useChat` 优先使用服务端 `session_id` 入桶，仅在老事件缺字段时回退到本地 pending/active session。
- `ChatPage` 不再把 provider/prompt/runtime 查询失败伪装成“当前没有数据”，而是显示带重试入口的 blocking error state。
- 这轮只修会话隔离与错误态，不引入新的后端 API，也不改变现有 runtime topology 模型。

### Verification
- [x] `go test -count=1 ./pkg/inboundrouter -run 'TestChatWebsocketFallsBackWithoutTopologyBinding|TestChatWebsocketRejectsDisabledWebsocketAccount|TestChatWebsocketRejectsConfiguredAccountWithoutActiveBindings'`
- [x] `go test -count=1 ./pkg/gateway -run 'TestProcessMessageDoesNotFallbackWhenRouterReturnsEmptyReply|TestProcessMessageDoesNotFallbackWhenExplicitRuntimeFails|TestProcessMessageDoesNotEmitInboundWhenRouterHandlesWebsocketChat|TestProcessMessagePassesExplicitRuntimeIDToRouter'`
- [x] `go test -count=1 ./pkg/webui -run 'TestBuildWebUIChatPromptContextIncludesExplicitRuntimeID|TestHandleUndoChatSession|TestClearChatSessionRemovesUndoSnapshots|TestResolveWebUIRuntimeSelectionUsesRuntimeDefaults|TestResolveWebUIRuntimeSelectionRejectsUnboundRuntime|TestResolveWebUIRuntimeSelectionFallsBackToRequestedRoute'`
- [x] `go test -count=1 ./pkg/webui ./pkg/gateway ./pkg/inboundrouter`
- [x] `npm --prefix pkg/webui/frontend run build`

### Status
**Completed** - Chat websocket `session_id` 对齐、前端多 session 桶路由修复、Chat 查询失败显式错误态与三语文案已经收口，并通过定向 Go 回归和前端 build。

## 2026-03-31 Claude Code 对齐扩展路线批次

### Goal
把已确认有价值的 Claude Code 能力拆成可执行的本地优先路线，避免直接堆砌概念；先对齐本地任务 runtime、后台会话、上下文压缩与多 agent 协作基础，再决定哪些远程/跨端能力值得继续引入。

### Phases
- [x] Phase 1: 收敛 Claude Code 架构共性，明确 `AgentDefinition + Task Runtime + Permission Mode + Tool Registry + Session State`
- [x] Phase 2: 评估新增能力并分层
- [x] Phase 3: 优先落地本地基础设施能力
- [x] Phase 4: 在本地基础设施稳定后，再评估远程/跨端能力

### Decisions Made
- 第一优先级：
  - `Coordinator`
  - `Daemon`
  - `UDS Inbox`
  - `Kairos` 的本地持久日志与压缩管线
  - `Auto-Dream` 的本地总结/蒸馏调度
- 第二优先级：
  - `Buddy` 作为纯前端/会话层彩蛋与人格外观能力
  - `Bridge` 的本地控制面抽象，但先不做公网跨设备
- 暂缓：
  - `Ultraplan` 的远程 30 分钟规划
  - 完整 remote execution / teleport 模型
- 原因：
  - 当前系统最缺的是本地统一任务执行、后台会话监督、跨会话消息/状态桥和上下文经济学，不是先上远端编排。

### Candidate Workstreams
- Workstream A: `pkg/tasks` / `pkg/session` / `pkg/webui` / `pkg/gateway` 的任务状态协议。
- Workstream B: `pkg/agentdefs` / tool registry / permission mode。
- Workstream C: `pkg/daemon` / `pkg/inbox` / background summarizer / Kairos-style daily log。
- Workstream D: 前端任务中心、任务状态、Buddy UI、会话摘要可视化。

### Status
**Phase 3 In Progress** - 已完成能力分层，接下来按本地 runtime / daemon / inbox / context pipeline 顺序继续开发。

## 2026-03-31 Binding 启用态一致性补强批次

### Goal
阻断控制面写入“enabled binding 指向 disabled runtime/account”的半有效状态，保证 Runtime Topology、Chat 选路与 WebUI 配置语义一致，并完成 API 回归与更大范围验证。

### Phases
- [x] Phase 1: 确认当前脏状态入口与影响面
- [x] Phase 2: 先补 manager 级失败测试并修复规范化校验
- [x] Phase 3: 补 WebUI/API 级回归，锁定请求边界行为
- [ ] Phase 4: 跑包级/前端/全量回归并记录结果
- [ ] Phase 5: 提交推送本批修复后继续下一处缺口

### Key Questions
1. `account_binding.enabled=true` 是否应允许引用 disabled runtime/account，还是只在执行时静默过滤？
2. 这类非法组合应在 manager、API 还是前端哪一层拦截？
3. 收紧后是否会打破既有测试或前端假设，暴露更多默认值问题？

### Decisions Made
- `enabled=true` 的 binding 必须同时指向 enabled 的 `channel_account` 与 `agent_runtime`，否则直接拒绝。
- `enabled=false` 的 binding 仍允许引用 disabled target，便于预配置或保留草稿拓扑。
- 约束优先落在 `accountbindings.Manager.normalizeBinding(...)`，确保 CLI/WebUI/其他调用方共享一套规则。
- WebUI API 层用回归测试锁定 `400` 行为，不额外引入静默 fallback。

### Verification
- [x] `go test -count=1 ./pkg/accountbindings -run 'TestManagerCRUDAndModeRules|TestManagerRejectsEnabledBindingForDisabledRuntimeOrAccount'`
- [x] `go test -count=1 ./pkg/webui -run 'TestRuntimeTopologyHandlers_CRUDAndSnapshot|TestHandleCreateChannelAccountRejectsEnabledWechatAccountWithoutCredentials|TestHandleCreateAccountBindingRejectsDisabledTargetsWhenEnabled'`
- [x] `go test -count=1 ./pkg/accountbindings ./pkg/webui`
- [x] `npm --prefix pkg/webui/frontend run build`
- [x] `go test -count=1 ./...`

### UI Follow-up
- [x] Runtime Topology 绑定弹窗只在创建 active binding 时提供 enabled account/runtime 作为候选。
- [x] 当系统里没有 enabled target 时，前端直接显示提示并禁用保存，而不是把错误完全留给后端 `400`。
- [x] 三语文案补齐 disabled target 提示与空候选提示。

### Errors Encountered
- `TestRuntimeTopologyHandlers_CRUDAndSnapshot` 之前默认创建 disabled runtime/account，却尝试创建 enabled binding。
  - 处理：将测试基线改为显式创建 enabled runtime/account，并为 WeChat account 填入最小合法凭据。

### Status
**Completed** - 已完成 enabled binding 控制面一致性补强、Runtime Topology 前端候选收口，并通过定向回归、包级回归、前端构建与全量 Go 测试，待提交推送。

## 2026-03-31 Binding 实际有效态可视化批次

### Goal
修正 Runtime Topology 对 binding 状态的展示语义：当 binding 自身仍为 enabled，但其 runtime 或 channel account 已被禁用时，拓扑页必须明确显示该 binding 当前不生效，并给出失效原因，避免控制面继续出现“看起来启用、实际不会路由”的误导。

### Phases
- [x] Phase 1: 先补失败测试，锁定 disabled target 下的拓扑展示失真
- [x] Phase 2: 扩展 topology snapshot，计算 binding 的实际有效态与失效原因
- [x] Phase 3: 改造 Runtime Topology 前端 binding 卡片，按实际有效态展示
- [ ] Phase 4: 跑更大范围回归并更新记录
- [ ] Phase 5: 提交推送后继续下一处缺口

### Decisions Made
- 不在 runtime/account disable 时自动删除或改写 binding 记录；保留配置事实，只修正可观测语义。
- `runtimetopology.BindingEdge` 新增：
  - `effective_enabled`
  - `disabled_reason`
- 前端 binding 卡片的状态 pill 改为优先依据 `effective_enabled` 渲染，而不是只看 `binding.enabled`。
- 失效原因先收口为三个稳定枚举值：
  - `binding_disabled`
  - `account_disabled`
  - `runtime_disabled`

### Verification
- [x] `go test -count=1 ./pkg/webui -run 'TestRuntimeTopologyHandlers_CRUDAndSnapshot|TestHandleCreateAccountBindingRejectsDisabledTargetsWhenEnabled|TestRuntimeTopologySnapshotMarksBindingsInactiveForDisabledTargets'`
- [x] `npm --prefix pkg/webui/frontend run build`
- [ ] `go test -count=1 ./...`

### Status
**Phase 4 In Progress** - 拓扑快照和前端展示已收口，正在跑全量 Go 回归并准备提交推送。

## 2026-03-31 Chat Runtime Selector 空态提示批次

### Goal
补齐 Chat 页面 runtime selector 的空态语义，让用户能区分“根本没有 websocket runtime 绑定”和“绑定存在但当前全部失效”，避免 selector 为空时只能靠猜测排查。

### Phases
- [x] Phase 1: 确认 Chat runtime selector 当前筛选逻辑与缺失提示
- [x] Phase 2: 补前端空态提示与三语文案
- [x] Phase 3: 跑前端构建验证
- [ ] Phase 4: 提交推送并继续下一处缺口

### Decisions Made
- 不修改后端协议；这轮只补最小必要的前端解释层。
- 当 `chatRuntimeIDs.size == 0` 时，提示“尚未配置 websocket runtime 绑定”。
- 当存在 binding 记录但 `chatRuntimes.length == 0` 时，提示“绑定存在但当前都不处于可用状态”，引导用户去 Runtime Topology 重新启用 account/runtime/binding。

### Verification
- [x] `npm --prefix pkg/webui/frontend run build`

### Status
**Phase 4 In Progress** - 聊天页 runtime selector 空态提示与三语文案已补齐并通过前端构建，待提交推送。

## 2026-03-31 Runtime 禁用影响提示批次

### Goal
补齐 Runtime Topology 编辑弹窗对“禁用 runtime/account”的影响说明，让用户在切换 `enabled=false` 前就能明确知道：相关 binding 会立即失效，但记录不会被删除。

### Phases
- [x] Phase 1: 核对 Runtime Topology 编辑弹窗中的 enabled 文案是否足够表达副作用
- [x] Phase 2: 为 runtime/account 禁用态补专门说明文案
- [x] Phase 3: 跑前端构建验证
- [ ] Phase 4: 提交推送并继续下一处缺口

### Decisions Made
- 不新增二次确认弹窗；先用更准确的就地说明降低误操作。
- 仅在 `enabled=false` 时切换到更具体的提示：
  - runtime 禁用提示
  - account 禁用提示
- 文案明确区分“binding 立即 inactive”和“binding record 仍然保留”。

### Verification
- [x] `npm --prefix pkg/webui/frontend run build`

### Status
**Phase 4 In Progress** - Runtime Topology 禁用影响提示与三语文案已补齐并通过前端构建，待提交推送。

## 2026-03-31 Chat Runtime 后端绑定约束批次

### Goal
把 WebUI Chat 对显式 `runtime_id` 的约束从“仅前端 runtime picker 过滤”补成服务端硬约束，确保后端也只接受绑定到 `websocket/default` 且当前可用的 runtime，避免绕过前端筛选直接命中未绑定 runtime。

### Phases
- [x] Phase 1: 先补失败测试，证明服务端仍接受未绑定 runtime
- [x] Phase 2: 收紧 `resolveWebUIRuntimeSelection()` 服务端边界
- [x] Phase 3: 跑 WebUI 定向回归、包级回归与前端构建
- [ ] Phase 4: 提交推送并继续下一处缺口

### Decisions Made
- 复用已有 control-plane 模型，不新增专门 API：
  - 查找 `websocket/default` account
  - 校验该 account 已启用
  - 校验其 enabled bindings 中包含目标 runtime
- 若任一条件不满足，统一返回：
  - `runtime <id> is not available for websocket chat`
- 保持当前前端 picker 逻辑不变，但不再把正确性寄托在前端过滤上。

### Verification
- [x] `go test -count=1 ./pkg/webui -run 'TestResolveWebUIRuntimeSelectionUsesRuntimeDefaults|TestResolveWebUIRuntimeSelectionRejectsUnboundRuntime'`
- [x] `go test -count=1 ./pkg/webui`
- [x] `npm --prefix pkg/webui/frontend run build`

### Status
**Phase 4 In Progress** - Chat runtime 的服务端绑定约束已补齐并通过定向/包级回归与前端构建，待提交推送。

## 2026-03-31 Gateway Websocket 单路径护栏批次

### Goal
为 Gateway websocket 聊天链路补一个护栏测试，确认在 `router.ChatWebsocket()` 已接管 websocket chat 的情况下，不会再额外通过 inbound bus 路径重复处理同一条消息。

### Phases
- [x] Phase 1: 审查 gateway -> bus -> router 代码路径
- [x] Phase 2: 补 Gateway 定向测试，锁定 websocket 单路径语义
- [x] Phase 3: 跑 Gateway 定向回归
- [ ] Phase 4: 提交推送并开始整体流程复盘

### Decisions Made
- 本批次先不修改实现，只补测试护栏。
- 当前测试证明：在 `processMessage()` 使用 `router.ChatWebsocket()` 的路径下，没有观测到额外 `websocket` inbound handler 执行。
- 这条测试作为后续重构 Gateway/Channels 运行时时的回归保护。

### Verification
- [x] `go test -count=1 ./pkg/gateway -run 'TestProcessMessagePassesExplicitRuntimeIDToRouter|TestProcessMessageDoesNotFallbackWhenExplicitRuntimeFails|TestProcessMessageDoesNotEmitInboundWhenRouterHandlesWebsocketChat'`

### Status
**Phase 4 In Progress** - Gateway websocket 单路径护栏测试已补齐并通过，待提交推送后转入整体流程与细节复盘。

## 2026-03-31 Runtime Prompt 执行链接通批次

### Goal
把 `runtime_agents.prompt_id` 从“仅可配置、仅在 metadata 透传”补成真正参与聊天执行的 prompt 解析输入，打通 runtime/account/binding 模型与 prompt 体系之间的执行链，并完成测试、验证、提交与推送。

### Phases
- [x] Phase 1: 重新核对 runtime prompt、agent prompt resolve 与 inbound router 现状
- [x] Phase 2: 先补回归测试，证明当前 runtime prompt 尚未生效
- [x] Phase 3: 修改 agent/prompts/router 执行链，让 runtime prompt 真正进入解析
- [x] Phase 4: 跑定向与全量回归，更新 notes/task_plan
- [x] Phase 5: 提交并推送本轮代码

### Key Questions
1. `runtime.prompt_id` 应该以什么方式进入现有 prompt manager，才能不破坏 global/channel/session 三层语义？
2. runtime prompt 是补成新的 scope，还是作为显式 prompt override 更合适？
3. 变更后怎样保证 routed runtime chat 和 legacy chat 的 prompt 行为边界仍然清晰？

### Decisions Made
- 本轮先不引入新的 prompt binding scope，而是把 `runtime.prompt_id` 作为显式 runtime 级 prompt override 注入现有 prompt resolve 流程。
- 保持 global/channel/session prompt 语义不变；runtime prompt 作为额外显式 prompt 叠加进入最终 resolved prompt set。
- 优先修复 routed runtime chat 主链，不把 WeChat 控制面重构混入这一轮。

### Verification
- [x] `go test -count=1 ./pkg/prompts ./pkg/inboundrouter`
- [x] `go test -count=1 ./pkg/agent ./pkg/prompts ./pkg/inboundrouter ./pkg/webui ./pkg/channels/wechat`
- [x] `npm --prefix pkg/webui/frontend run build`

### Status
**Completed** - 已完成 runtime prompt 执行链接通、全量回归验证，并准备提交推送本轮代码。

## 2026-03-31 WeChat 控制面与 Channel Account ID 收口批次

### Goal
修正旧 WeChat 绑定控制面与新 channel-account/runtime-topology 控制面之间的 ID 语义分裂，确保“当前激活账户”使用同一主键模型，并让旧控制面操作能同步刷新新的 topology/account 观察面。

### Phases
- [x] Phase 1: 审查 WeChat binding payload、ChannelsPage 与 runtime topology 之间的状态边界
- [x] Phase 2: 先补测试，锁定 `active_account_id` 语义错误
- [x] Phase 3: 修复 payload 与前端 query 失效链路
- [x] Phase 4: 跑相关与全量回归，更新 notes/task_plan
- [x] Phase 5: 提交并推送本轮代码

### Decisions Made
- `active_account_id` 统一收口为真实的 `channel_accounts.id`，不再复用 `bot_id`。
- 不在本轮重做 WeChat 控制面 UI；先修正 API 语义与跨页 stale 状态，让旧面和新面至少共享同一事实来源。
- WeChat binding 相关 mutation 成功后，除旧 `channels` 查询外，也同步失效 `channel-accounts` 与 `runtime-topology` 查询。

### Verification
- [x] `go test -count=1 ./pkg/webui -run TestBuildWechatBindingPayloadIncludesCurrentBinding`
- [x] `npm --prefix pkg/webui/frontend run build`
- [x] `go test -count=1 ./pkg/webui ./pkg/channels/wechat ./cmd/nekobot/...`
- [x] `go test -count=1 ./...`

### Status
**Completed** - 已完成 WeChat 控制面与 channel account ID 语义收口、前端同步失效与全量回归验证，并准备提交推送本轮代码。

## 2026-03-31 WeChat 多账号运行时凭据隔离批次

### Goal
修正 WeChat channel account runtime 在多账号场景下错误复用“当前激活账号”凭据的问题，确保账号级 runtime 始终使用自身 `channel_account.config` 中的凭据启动；同时补上 WebUI channel-account 创建/更新时的最小凭据校验，避免再次写入“已启用但不可启动”的 WeChat 账号。

### Phases
- [x] Phase 1: 补失败测试，确认账号级 WeChat runtime 会串用 active store 凭据
- [x] Phase 2: 改造 WeChat runtime 构造路径，区分 legacy active-account 与 account-scoped explicit credentials
- [x] Phase 3: 为 WebUI channel-account create/update 补 WeChat enabled 约束校验
- [x] Phase 4: 跑定向回归、WebUI 回归、前端构建与全量 Go 验证
- [x] Phase 5: 更新计划/笔记并提交推送

### Decisions Made
- `wechat.NewChannel` 继续保留 legacy 语义，从凭据仓库读取当前 active account。
- `wechat.NewAccountChannel` 改为强制接收显式凭据，不再回退到 `store.LoadCredentials()`。
- WeChat account runtime 的显式凭据从 `channel_account.config` 独立解码，显式映射 `base_url -> Credentials.BaseURL`，不复用底层 `baseurl` JSON tag。
- WebUI 在保存已启用的 WeChat channel account 时，要求至少提供 `config.bot_token` 和 `config.ilink_bot_id`，提前阻断无效运行时配置。

### Verification
- [x] `go test -count=1 ./pkg/channels -run TestBuildChannelFromAccount_Wechat`
- [x] `go test -count=1 ./pkg/channels/wechat ./pkg/channels`
- [x] `go test -count=1 ./pkg/webui -run 'TestReloadChannelsByTypePrefersEnabledWechatAccounts|TestHandleCreateChannelAccountRejectsEnabledWechatAccountWithoutCredentials|TestRuntimeTopologyHandlers_CRUDAndSnapshot'`
- [x] `go test -count=1 ./pkg/webui ./pkg/channels/wechat ./cmd/nekobot/...`
- [x] `npm --prefix pkg/webui/frontend run build`
- [x] `go test -count=1 ./...`

### Status
**Completed** - 已完成 WeChat 多账号运行时凭据隔离修复、补齐 channel-account 启用校验，并通过定向回归、WebUI 回归、前端构建与全量 Go 测试。

## 2026-03-31 Runtime Topology WebUI 可编辑化批次

### Goal
把当前只读的 Runtime Topology 页面升级成可直接管理 runtime agent、channel account、account binding 的控制面，接通已有后端 CRUD，补齐必要交互与文案，并在本轮结束后完成构建、回归、提交与推送。

### Phases
- [x] Phase 1: 重新核对 topology 后端接口、前端现状与计划
- [x] Phase 2: 扩展前端 hooks，接入 runtime/account/binding 查询与变更
- [x] Phase 3: 改造 Runtime Topology 页面为可管理界面并补齐交互细节
- [x] Phase 4: 更新计划/笔记并完成前端与后端回归验证
- [x] Phase 5: 提交并推送本轮代码

### Key Questions
1. 现有后端 CRUD 是否已经足够支撑前端直接编辑，还是还存在 API 级缺口？
2. 页面是应该先做“可用优先”的轻量管理，还是继续停留在观测面等待更完整 runtime manager？
3. 运行时拓扑页如何在不引入过重导航重构的前提下，把操作链路补齐到可用状态？

### Decisions Made
- 本轮优先把 Runtime Topology 从“只读观察”补成“轻量可管理”，不等待后续更重的 runtime manager 重构。
- 前端直接复用已有 `/api/runtime-agents`、`/api/channel-accounts`、`/api/account-bindings` CRUD 接口，不新增后端 API 形状。
- 表单层对 `skills/tools/config/metadata/policy` 采用文本 + JSON 的务实输入模式，先保证真实可维护，再考虑更细粒度组件化。

### Verification
- [x] `npm --prefix pkg/webui/frontend run build`
- [x] `go test -count=1 ./pkg/webui ./pkg/gateway ./pkg/channels ./cmd/nekobot/...`

### Status
**Completed** - 已完成 Runtime Topology WebUI 可编辑化、前后端回归验证，并准备提交推送本轮代码。

## 2026-03-29 Harness 审阅批次

### Goal
审阅以下 5 次 harness 相关提交，参考 `/home/czyt/code/yoyo-evolve` 的成熟实现，修复明确存在的业务流程问题、架构缝隙和 WebUI 体验问题，并在收尾后更新本计划。

目标提交：
1. `1b7c3d0` `docs: update harness progress tracker`
2. `580741d` `feat: add Turn Undo, @file Mentions, and Watch Mode`
3. `46026ac` `feat: add Learnings JSONL system`
4. `583245d` `feat(webui): add harness config sections`
5. `c409cf1` `feat: add audit log and streaming bash`

### Phases
- [x] Phase 1: 建立计划与审阅范围
- [x] Phase 2: 对照 `yoyo-evolve` 审阅后端流程与架构
- [x] Phase 3: 审阅 WebUI 配置页与前端体验
- [x] Phase 4: 修复已确认问题并补测试/验证
- [x] Phase 5: 更新计划、记录结果并提交相关代码

### Key Questions
1. 这几次 harness 迁移在哪些地方只迁了“配置面”而没接通“执行面”？
2. `yoyo-evolve` 的 harness 行为里，哪些最关键的约束/流程在 `nekobot` 里被做薄了？
3. WebUI ConfigPage 是否暴露了配置分区，但缺少配套文案、说明、状态反馈或交互保护？

### Decisions Made
- `watch` 不重写实现，直接复用已存在的 `pkg/watch.Module`，把它接回所有主要 FX 启动路径。
- `undo` 不做独立命令通道，继续作为 agent tool 存在，但改为按真实 session 动态注册/替换。
- `audit` 先补 `session_id` 上下文透传，不在本批次扩展更重的流式 UI 事件桥。
- ConfigPage 先补最关键的 `watch.patterns` 可视化编辑；其余 harness 分区继续保留通用表单/JSON 模式。

### Errors Encountered
- `pkg/agent/agent_test.go` 新增测试首次编译失败：缺少 `session` import。
  - 处理：补 import 后重新跑红绿测试。
- `RegisterUndoTool` 重复注册导致 panic。
  - 处理：在 `tools.Registry` 中增加 `Replace`，并让 undo 改用 replace 语义。
- `pkg/channels/wechat/runtime_test.go` 新增多绑定测试初始失败：`RuntimeBindingService` 缺少 `ListBindingRecords`，控制层仍只认单一 `ConversationKey`。
  - 处理：补 `ListBindingRecords` / `GetBindingsBySession` 包装，`/bindings` 改为按 binding record 展示，并在 stop/delete 时清理 runtime 的全部 chat 绑定。

### Status
**Completed** - 已完成 harness 审阅修复，并补完上一轮遗留的 WeChat/conversationbindings 多绑定收口与验证，待提交并推送相关代码。

## 2026-03-29 扩展 Harness 对照与继续嵌入批次

### Goal
将对 `/home/czyt/code/yoyo-evolve` 的对照范围从上次 5 个 harness 提交扩展到其相邻的 harness 工作流能力，重新评估 `nekobot` 当前已整合功能是否真正完整，并筛选可继续低风险嵌入、可验证闭环的功能；在确认方案后按计划实现、测试并持续更新状态。

### Phases
- [x] Phase 1: 扩展对照范围并补计划
- [x] Phase 2: 评估已整合功能的完整度与缺口
- [x] Phase 3: 产出可继续嵌入功能方案并确认实现边界
- [x] Phase 4: 按计划实现选定功能并补测试
- [x] Phase 5: 完整验证、更新计划与交付结果

### Key Questions
1. `yoyo-evolve` 的 harness 相邻能力里，哪些已经在 `nekobot` 存在“配置或底层能力”，但缺少真正可用的用户工作流闭环？
2. 哪些能力可以在不引入 REPL 重构的前提下，直接嵌入 `nekobot` 的现有 Agent/WebUI/命令体系？
3. 哪些差异本质上是产品形态不同，不应机械追平？

### Decisions Made
- 对照范围扩大到与 harness 工作流直接相关的相邻能力：`/undo` 使用语义、`/watch` 状态/控制模型、`@file` 注入体验、audit 可观测性、learnings 的上下文接入闭环。
- 优先选择“当前代码已有 60% 到 80% 基础设施，只差产品闭环”的功能进入本轮实现。
- 不为了追平 `yoyo-evolve` 而强行引入整套 Rust REPL 命令模型；实现应贴合 `nekobot` 现有 WebUI + Agent + channel/control 结构。

### Errors Encountered
- `pkg/webui/server_config_test.go` 首轮 harness 配置测试按旧字段名编写，未对齐当前 `Audit/Undo/Preprocess/Learnings` 配置模型。
  - 处理：按实际配置结构改写测试，再继续红绿循环。
- `pkg/webui/server_status_test.go` 因 `NewServer` 新增 watcher 参数而编译失败。
  - 处理：补齐测试调用参数。
- `pkg/webui/server_chat_test.go` 误用 Echo v4 风格 `SetParamNames/SetParamValues`。
  - 处理：改为 Echo v5 的 `SetPathValues`。

### Status
**Completed** - 已完成扩展 harness 对照、选定功能继续嵌入、补齐 Web 控制面与体验，并通过 Go/前端构建验证，待提交与推送。

## 2026-03-30 Harness WebUI 审计台与体验收口批次

### Goal
继续补齐 harness 相关 WebUI 缺失页面与体验，将已有的 watch / undo / @file / audit 能力组织成更清晰的操作面、配置面、可观测面闭环，并完成提交推送。

### Phases
- [x] Phase 1: 为 WebUI 接入 audit API 与测试
- [x] Phase 2: 新增 Harness Audit 页面、导航与多语言文案
- [x] Phase 3: 打磨 Chat / Config 的 harness 体验衔接
- [x] Phase 4: 运行 Go 测试与前端构建验证
- [x] Phase 5: 更新计划与说明，准备提交推送

### Decisions Made
- 不把 audit 混进 System 页，单独提供 `Harness Audit` 页面，避免可观测性信息继续埋在泛系统状态里。
- 保持 Chat 为操作面、Config 为配置面、Audit 为观测面三分结构，并用显式跳转入口连接三者。
- Audit API 先提供最近 N 条读取与清空日志，JSON 结构直接复用现有 `pkg/audit` 数据模型，降低前后端映射成本。

### Verification
- [x] `go test -count=1 ./pkg/webui`
- [x] `go test -count=1 ./cmd/nekobot/... ./pkg/watch ./pkg/agent`
- [x] `go test -count=1 ./...`
- [x] `npm --prefix pkg/webui/frontend run build`
- [x] 浏览器级 WebUI smoke test（初始化/登录、Chat、Harness Audit、Config/Watch、回到 Chat）

### Errors Encountered
- 首轮浏览器 smoke test 因本地回归脚本使用错误环境变量与 CLI `PersistentPreRunE` 清空 `NEKOBOT_CONFIG_FILE` 而误读默认配置，导致端口冲突。
  - 处理：先改为显式 `--config /tmp/nekobot-regression/config.json` 启动临时服务定位问题；随后修复 CLI 行为，使未显式传 `--config` 时不再清空已有 `NEKOBOT_CONFIG_FILE`，并补命令层测试与实证回归。
- `HarnessAuditPage` 在空审计数据时对 `null` 调用 `.filter()`，导致页面在无 audit 记录的新环境里直接崩溃。
  - 处理：前端将 `data?.entries` 统一归一化为 `entries = data?.entries ?? []`，再重复构建与浏览器回归确认无新问题。

### Status
**Completed** - 已补齐 Harness Audit 页面与相关 WebUI 体验收口，并完成全量 Go 回归、前端构建与浏览器 smoke test；修复空 audit 数据崩溃后复测通过，未再出现新问题，待提交与推送。

## 2026-03-30 全局交互 / 数据流 / 逻辑性 / 扩展性审查批次

### Goal
对当前整个软件做一轮完整的系统审查，覆盖交互链路、关键数据流向、业务逻辑一致性、模块边界与可扩展性；沉淀明确缺陷，先完成设计与修复计划，再在后续批次中按优先级落地修复代码与界面问题。

### Phases
- [ ] Phase 1: 建立审查范围、读当前架构与最近改动
- [ ] Phase 2: 识别交互、数据流、逻辑、扩展性四类核心问题
- [ ] Phase 3: 形成候选修复方案与优先级
- [ ] Phase 4: 与用户确认本轮优先落地范围
- [ ] Phase 5: 进入实现、验证、提交与推送

### Key Questions
1. 目前最影响真实可用性的系统缺陷，究竟集中在 WebUI 操作链路、后端状态一致性，还是模块边界与扩展成本？
2. 哪些问题属于“当前就会造成错误认知或错误行为”的 P0/P1 缺陷，必须优先修？
3. 哪些问题虽然是架构债，但不应在本轮和用户可见缺陷混做？

### Decisions Made
- 审查与修复拆为三轮推进：
  1. 第一轮以 `Chat + Config + Harness` 为主链。
  2. 第二轮以 `Gateway / Channels` 运行时为主链。
  3. 第三轮专门处理第一二轮融合后的集成问题。
- 第一轮先做最小必要修复打通主链，但允许围绕同一主链做小范围边界抽取，避免继续把状态同步逻辑散落在多个 handler 中。
- `watch` 统一按“配置保存即同步运行态”处理，覆盖 `/api/config`、`/api/config/import`、`/api/harness/watch` 三条入口。
- `chat clear` 统一按“完整清空会话 + undo snapshot”处理，不允许 clear 后还能通过 undo 找回旧上下文。

### Errors Encountered
- `pkg/webui` 新增回归测试首轮编译失败：`Server` 缺少独立的 chat clear helper，clear 语义散落在 WS handler 内。
  - 处理：抽出 `clearChatSession` helper，并让 WS clear 分支复用它。

### Status
**Completed** - 第 1 轮 `Chat + Config + Harness` 主链修复、前端一致性修补与全量验证已完成，已进入提交推送阶段。

## 2026-03-31 Claude Code 基础能力对标评估批次

### Goal
对照 `/home/czyt/code/claude-code` 的底层设计，重新评估 `nekobot` 当前在 agent 调度、skills 自动选择、上下文压缩、会话状态、WebSocket 体验与 Claude agent 支持上的基础能力缺口，并产出可执行的引入优先级。

### Phases
- [x] Phase 1: 盘点 `nekobot` 当前基础设施现状
- [x] Phase 2: 对照 `claude-code` 的 session / state / skills / compact / agent 相关实现
- [x] Phase 3: 形成分层建议：可直接引入、Claude 特化增强、未来可集成特性

### Decisions Made
- 第一优先级不是照搬某个单点功能，而是补 `nekobot` 自身缺少的“显式会话状态与元数据协议”。
- `claude-code` 最值得学习的三类基础设计：
  - 会话状态与 pending action / progress summary 的统一出口。
  - WebSocket 生命周期显式区分 `reconnecting` 与 `disconnected`。
  - skills 的条件激活 / 作用域发现 / 轻量索引，而不是只停留在静态 discover。
- `nekobot` 现有基础已经具备接这些能力的土壤：
  - 有 runtime-scoped session / route_result / tool session / subagent / memory / skills snapshot/version。
  - 但这些能力之间仍偏“并列存在”，缺少一个统一的状态协议和自动决策层。

### Recommended Next Work
- P0: 会话状态协议与前端状态机。
- P0: Chat/Gateway WebSocket 重连与状态细分。
- P1: runtime/agent 执行元数据持久化到 session。
- P1: skills 条件激活与自动选择前置索引。
- P2: 上下文压缩与 turn summary / subagent summary。
- P2: Claude agent 特化支持层（权限模式、待批准动作、agent 能力画像）。

### Status
**Completed** - 已完成 Claude Code 对标评估，产出可直接用于后续架构轮次的基础能力引入优先级。

## 2026-03-31 Claude Code 对齐架构重构计划批次

### Goal
把 `nekobot` 后续架构调整正式收口到一条可执行路线：不再围绕“顶层常驻 agent + 外挂功能”持续堆叠，而是逐步演进为 `AgentDefinition + ToolRegistry + PermissionContext + TaskState + RuntimeBinding + SessionState` 驱动的运行平台。

### Phases
- [x] Phase 1: 收敛 Claude Code 两轮对标结论与本地代码现状
- [x] Phase 2: 形成目标架构、边界划分、依赖顺序与阶段交付
- [ ] Phase 3: 先完成当前 bug 收口，作为架构重构前的稳定基线
- [ ] Phase 4: 进入架构开发第一阶段

### Key Questions
1. `AgentDefinition`、`runtime agent`、`subagent`、`background task` 在 `nekobot` 中的统一抽象边界应该放在哪里？
2. 哪些现有能力必须先下沉为基础设施，后面的 agent/task 重构才不会反复返工？
3. 怎样在不牺牲现有 channel / WebUI / harness 主链可用性的前提下做阶段式重构？

### Decisions Made
- 后续开发采用“先 bug 收口，再按阶段做架构升级”的路线，不混做。
- `agent` 后续不再被视为唯一顶层执行器，而是退化为某类执行任务的实现细节。
- `explore / plan / verify` 优先做成受控的 `AgentDefinition` 或 `permission mode` 组合，不另起三套 runtime。
- 任务系统、状态系统、权限系统、工具治理、上下文压缩被视为同一层基础设施，而不是独立 feature。
- 后续每阶段结束都必须有：
  - 最小可运行闭环
  - 回归测试
  - WebUI/控制面可观测性
  - 计划文件状态更新

### Deliverables
- [x] 架构执行计划文档：`claude_code_alignment_plan.md`
- [x] 研究/判断沉淀：`notes.md`
- [x] 主任务跟踪入口：`task_plan.md`

### Status
**Phase 3 Pending** - 计划已落地，待先完成当前 bug 修复与验证，再按新计划进入架构开发。

## 2026-03-30 Gateway / Channels 运行时路由主干批次

### Goal
补齐当前多账号 channel account / runtime agent / account binding 的执行主链，先建立统一入站路由层，消除 gateway 与 bus 在入站路径上的“只记录不路由”状态，并把 gateway 先迁移到新路由主干，同时保持现有默认 agent 行为不回退。

### Phases
- [x] Phase 1: 核对 bus、gateway、runtime topology 与 direct-call 调用点
- [x] Phase 2: 完成 bus 入站/出站 handler 语义拆分并收口调用方
- [x] Phase 3: 实现统一 inbound router 与 runtime/account/binding 解析
- [x] Phase 4: 迁移 gateway 到 router 主链并保留无 binding 的默认回退
- [x] Phase 5: 跑定向测试、全量 Go 回归、前端构建并记录结果

### Decisions Made
- `bus` 正式拆成 `RegisterInboundHandler` / `RegisterOutboundHandler` 两套 handler 注册语义；旧 `RegisterHandler` 保留为 outbound 兼容别名，避免一次性打断所有调用点。
- `channels.Manager` 只负责 outbound 注册，不再混用同一 handler map 处理入站。
- 新增 `pkg/inboundrouter` 作为统一入站决策层：
  - 通过 `channelaccounts.Manager.ResolveForChannelID()` 将运行时 `channel.ID()` 映射回 channel account。
  - 通过 `accountbindings.Manager.ListEnabledByChannelAccountID()` 选出有效 binding。
  - 通过 `runtimeagents.Manager.Get()` 解析 provider/model 并生成 runtime-scoped session。
- `gateway` 先接到 router，但保留兼容行为：
  - 若存在 `websocket/default` account + binding，则走 runtime routing。
  - 若不存在，则继续使用现有默认 agent 会话，不要求先手工建拓扑才能聊天。
- router 第一版先复用全局 `*agent.Agent` + `PromptContext` 注入 provider/model/runtime metadata，不提前引入 runtime 专属 agent 池。

### Verification
- [x] `go test -count=1 ./pkg/inboundrouter ./pkg/gateway ./pkg/bus ./pkg/channels ./pkg/channelaccounts ./pkg/accountbindings`
- [x] `go test -count=1 ./pkg/webui ./pkg/gateway ./cmd/nekobot/...`
- [x] `npm --prefix pkg/webui/frontend run build`
- [x] `go test -count=1 ./...`

### Remaining Follow-up
- [ ] 继续把剩余仍直接调全局 agent 的 channel 路径逐步迁到 `pkg/inboundrouter` 主链。
- [x] Telegram / WeChat 普通聊天主链已切到 `bus + inboundrouter`，并保留 legacy fallback。
- [ ] ServerChan 当前主要承担通知/命令通道，不作为本轮入站聊天 blocker，后续按产品定位决定是否纳入 runtime routing。
- [ ] 将 runtime `PromptID` 真正接入 prompt resolution，而不是当前仅透传 runtime metadata。
- [ ] 为 runtime/account/binding 增加真正可编辑的 WebUI 页面，而不是仅有只读 topology。

### Status
**Completed** - 已建立统一入站路由主干，完成 bus 语义拆分与 gateway 接线，并通过定向测试、全量 Go 回归和前端构建验证。

## 2026-03-31 Gateway / Channels 运行时主链收口批次

### Goal
沿着上一轮的统一入站 router，继续收口真实 channel 聊天主链，把 Telegram / WeChat 的普通消息执行路径从“channel 内直接调 agent”切到 `bus + inboundrouter`，同时修复在全面回归中暴露出的 `watch` 重启竞态，确保第二轮主链真正稳定。

### Phases
- [x] Phase 1: 重新核对 Telegram / WeChat / ServerChan / Gateway 的当前消息路径
- [x] Phase 2: 为 router 增加 legacy channel fallback，避免迁移时强依赖 topology 先配置完整
- [x] Phase 3: 迁移 Telegram / WeChat 普通聊天到 bus/router 主链
- [x] Phase 4: 修复全面回归中暴露的 `watch` restart nil-pointer 竞态
- [x] Phase 5: 跑定向测试、主链回归、前端构建与全量 Go 验证

### Decisions Made
- `Telegram` / `WeChat` 先迁“普通聊天”主链，不动现有 command / ACP runtime control / pending interaction 分支，降低改动面。
- `inboundrouter` 增加 legacy fallback：
  - 若某 channel 还没有 account/binding 配置，则按旧 `sessionID + PromptContext` 直接调用全局 agent。
  - 这样 channel 可以先切到 `SendInbound()`，而不会因为 topology 尚未配置就失效。
- `Telegram` 通过 `bus.Message.Data` 透传 `thinking_message_id` / `reply_to_message_id`，继续复用现有消息完成体验。
- `WeChat` 通过 `bus.Message.Data["context_token"]` 透传回复上下文，避免切到 router 后丢失上下文回复能力。
- `ServerChan` 本轮不强行纳入 router 聊天主链：
  - 现有实现更像通知/命令通道而不是持续对话 runtime。
  - 先避免把“是否要做多轮聊天”与当前运行时主链收口混在一起。
- `watch` 竞态修复采用最小方式：
  - event loop 在启动时捕获 `ctx` 与 `fsWatcher` 快照。
  - 运行期间不再读取可能被 `Stop()` 置空的共享指针。

### Verification
- [x] `go test -count=1 ./pkg/inboundrouter ./pkg/channels/telegram ./pkg/channels/wechat`
- [x] `go test -count=1 ./pkg/watch ./pkg/inboundrouter ./pkg/channels ./pkg/gateway ./pkg/webui ./cmd/nekobot/...`
- [x] `npm --prefix pkg/webui/frontend run build`
- [x] `go test -count=1 ./...`

### Status
**Completed** - 已将 Telegram / WeChat 普通聊天接入统一 router 主链，修复 `watch` restart 竞态，并通过定向测试、主链回归、前端构建与全量 Go 验证。

### Round 1 Progress Notes
- [x] 已为 watch 运行态同步补回归测试，覆盖 `handleUpdateWatchStatus`、`handleSaveConfig`、`handleImportConfig`。
- [x] 已为 chat clear 与 undo 边界补回归测试，确保 clear 后不会残留 snapshot。
- [x] `pkg/watch.Watcher` 已补 `ApplyConfig` 与可重启生命周期，修复 stop 后无法可靠 restart 的隐患。
- [x] `pkg/webui.Server` 已抽出 `syncWatchRuntime` / `clearChatSession`，收口主链状态同步逻辑。
- [x] 前端 `useSaveConfig` / `useImportConfig` 已同步失效 `watch-status` 查询，修复 Config / Chat 观察面 stale 状态。

### Round 2 Progress Notes
- [x] 已修复 `handleUpdateChannel` 的状态分裂问题：改为基于配置副本做 channel 配置预校验、预构建，只有通过后才持久化并切换 live/runtime。
- [x] 已补 channel 更新回归测试，确保非法配置返回 `400` 且不会污染 live config、runtime DB 或拆掉现有 channel runtime。
- [x] 已为 System 页接通 gateway config reload 链路：新增 `/api/service/reload`、前端 hook 与按钮，避免只为运行时配置变更强制做 service restart。
- [x] 已修复 `/api/channels/:name/test` 的误导性成功语义：仅对实现真实健康探测的通道返回 `reachable=true,status=ok`，其余通道明确返回 `configured`，避免 WebUI 把“已配置”误判为“已连通”。
- [x] 已为 Slack / Gotify 接入可选 `HealthCheck` 探测能力，并补齐 Channels 页 toast 文案，区分探测成功、探测失败、未暴露探测与已禁用四种结果。

### Round 2 Verification
- [x] `go test -count=1 ./pkg/webui -run 'TestHandleTestChannelReturnsConfiguredWithoutProbe|TestHandleTestChannelUsesHealthCheckFailure|TestHandleTestChannelUsesHealthCheckSuccess|TestHandleUpdateChannelRejectsInvalidConfigWithoutMutatingState|TestHandleUpdateChannelKeepsExistingRuntimeWhenPrebuildFails'`
- [x] `go test -count=1 ./pkg/webui -run 'TestHandleUpdateChannelRejectsInvalidConfigWithoutMutatingState|TestHandleUpdateChannelKeepsExistingRuntimeWhenPrebuildFails|TestHandleGetChannelsIncludesWechat|TestHandleGetChannelsIncludesGotify|TestBuildWechatBindingPayloadIncludesCurrentBinding'`
- [x] `go test -count=1 ./pkg/webui ./pkg/channels ./pkg/gateway`
- [x] `npm --prefix pkg/webui/frontend run build`
- [x] `go test -count=1 ./...`

### Verification
- [x] `go test -count=1 ./pkg/watch ./pkg/webui -run 'TestWatcherCanRestartAfterStop|TestHandleUpdateWatchStatusStopsWatcherWhenDisabled|TestHandleSaveConfigSyncsWatcherRuntime|TestHandleImportConfigSyncsWatcherRuntime|TestClearChatSessionRemovesUndoSnapshots'`
- [x] `go test -count=1 ./pkg/webui ./pkg/watch ./pkg/session`
- [x] `npm --prefix pkg/webui/frontend run build`
- [x] `go test -count=1 ./...`

## Goal
在保持 `nekobot` 现有稳定性的前提下，基于 `~/code/goclaw` 与 `~/code/gua` 的成熟实践，完成：
1. 准确认定 `nekobot` 当前已完成与待完成功能。
2. 清理过期 backlog，避免继续围绕已完成项开发。
3. 分批迁移高价值实践、巧妙设计和更完善的功能包。
4. 每完成一个功能批次都可独立验证、提交并推送。

优先保证：
1. 现有 provider fallback / tool 执行语义不回退。
2. 每一批改动都可独立回归验证。
3. 优先完成“用户可见且当前明确未实现”的缺口。
4. 主 agent 审核所有子任务结论后再落代码。

## Progress Snapshot
- **总体状态**: 持续推进中（主线可用，进入收口阶段）。
- **关键已完成批次**:
  - [x] Feature Batch #1：多用户认证基础迁移（User/Tenant/Membership + JWT 统一存储）。
  - [x] Feature Batch #2：`chatWithBladesOrchestrator` 由真实 blades runtime 接管，保留 legacy 可切换。
  - [x] Prompt 构建优化：静态块缓存 + 动态时间占位替换，避免时间冻结。
  - [x] Feature Batch #3：Web-first runtime 管理扩展（Prompts 存储/绑定、Tool Session 控制、QMD 运行态检查、Skills 版本/快照可视化）。
- **本轮评估结论**:
  - [x] `/gateway restart` 已在 `pkg/gateway/controller.go` 实现。
  - [x] `/gateway reload` 已在 `pkg/gateway/controller.go` 实现。
  - [x] `memory hybrid search` 文本相似度已在 `pkg/memory/store.go` 实现。
  - [x] `skills version comparison` 已在 `pkg/skills/eligibility.go` 实现。
  - [x] `cron at/every/delete-after-run/run-now` 已在 `pkg/cron/*`、`pkg/webui/*`、`cmd/nekobot/cron.go` 实现。
  - [x] session history sanitize / safe window / force compression 已在 `pkg/agent/*`、`pkg/session/*` 实现。
- **最近验证**:
  - [x] `go test -count=1 ./pkg/agent`
  - [x] `go test -count=1 ./...`
  - [x] `GOPROXY=https://goproxy.cn,direct go test -count=1 ./...`
  - [x] `GOPROXY=https://goproxy.cn,direct go test -count=1 ./pkg/channels/wechat ./pkg/wechat/... ./pkg/webui`

## Completed Milestones

### A. WebUI 基础迁移（Phase 1-8 + Cron）
- [x] React + Vite + TypeScript 脚手架与构建链路。
- [x] 认证、布局、Chat Playground、Tool Sessions 重写。
- [x] Provider/Channel/Config/System 基础页面迁移。
- [x] Cron API + WebUI + CLI run-now（含测试）。

### B. Runtime/Agent 架构增强
- [x] ACP 基础能力（`nekobot acp`、session/new/load/update、set_mode/set_model）。
- [x] blades 会话历史注入语义修复（保留 assistant tool-calls / 多 tool results）。
- [x] provider failover/cooldown 语义接入 agent 主路径。
- [x] Skills `always` 元数据与 prompt 渲染增强。

### C. 稳定性/一致性
- [x] Ent runtime schema 并发初始化竞态修复。
- [x] BuildMessages 尾部用户消息去重。
- [x] 工具描述确定性排序（提升 cache 稳定性）。

### D. Web-first Runtime 管理增强
- [x] Prompt 定义与绑定改为共享 Ent runtime DB 持久化。
- [x] WebUI 新增 Prompts 管理页、后端 API 与测试。
- [x] Tool Sessions 管理补齐访问控制/运行态管理测试。
- [x] QMD 路径解析、session export 信息与手动清理入口增强。
- [x] Skills snapshot/version 与 provider cooldown 的运行态展示能力增强。
- [x] README 与 QMD 文档更新为当前 Web-first 操作模型。

### E. 已确认完成但此前 backlog 过期的事项
- [x] `/gateway restart` 真正执行路径。
- [x] `/gateway reload` 配置热重载。
- [x] Memory Hybrid Search 文本相似度。
- [x] Skills 版本约束校验。
- [x] Cron 多调度类型与运行状态展示。
- [x] Session history sanitize / safe history / context force compression。

### F. 本轮已完成迁移事项
- [x] Subagent 完成通知回推 origin channel，并补齐 `spawn` 工具注册与 origin route 透传。
- [x] `gua/libc/wechat` SDK 基线迁移到 `pkg/wechat`（types / client / auth / cdn / messaging / monitor / parse / typing / voice / bot）。
- [x] WeChat 通道切换到共享 `pkg/wechat` SDK，并补齐本地文件路径附件发送。
- [x] 删除 `pkg/channels/wechat/protocol.go` 本地重复协议层，微信通道仅保留 channel/store/runtime 胶水。
- [x] WeChat 弱交互协议首批落地：技能安装确认支持 `/yes` `/no` `/cancel`。
- [x] WeChat presenter 输出规则注入 agent 输入，补齐“纯文本/本地文件路径附件”引导。
- [x] WeChat 阶段性收口：共享 SDK、发送链路、登录绑定、首批弱交互与 presenter guidance 已完成并推送，后续只保留通用 presenter/interaction 泛化为次级事项。
- [x] Conversation/thread binding 首批迁移：在 `pkg/conversationbindings` 上补齐绑定记录视图、按 conversation/session 检索、绑定元数据与过期清理，并保持与 WeChat runtime binding 兼容。

## 2026-03-30 Multi-Agent Runtime / Channel Account 架构调整批次

### Goal
基于“真正独立的多 agent runtime + channel account + account-agent binding”模型，分多轮完成当前单默认 agent 架构向多 agent 架构的重组；其中 harness 下沉到 agent 层，channel 支持多账户，账户可按单 agent 或多 agent 模式绑定，并为后续 agent 协作预留插槽。

### Phases
- [x] Phase 1: 输出正式设计文档与多轮实施计划
- [x] Phase 2: 并行分析现有代码在 runtime/account/binding 三条轴线上的改造切口
- [x] Phase 3: 确认首轮开发边界并拆成可执行任务
- [x] Phase 4: 完成第一轮实现与验证
- [ ] Phase 5: 进入第二轮 channel runtime/account 化并持续更新计划、提交与推送

### Key Questions
1. 哪些现有模块最应该提升为一等模型：`AgentRuntime`、`ChannelAccount`、`AccountBinding`？
2. 在不考虑历史迁移的前提下，哪一轮最适合先落地：runtime 域模型、WeChat account 化、还是 harness 下沉？
3. WebUI 应如何从 `Channels/Config/Harness` 重组为 `Agents / Channel Accounts / Bindings`？

### Decisions Made
- 采用 in-process multi-runtime 方案，不做单 agent preset 过渡方案，也不直接上多进程 worker。
- `AgentRuntime` 强隔离，至少独立 `provider/prompt/skills/tools/harness/private memory/session namespace`。
- `ChannelAccount` 是 transport endpoint 的一等对象，agent 绑定到账户而不是 channel type。
- 多账号能力必须是全 channel 通用模型，不能把 `ChannelAccount` 设计成 WeChat/iLink 特例。
- 默认 `single_agent`，允许 `multi_agent`，多 agent 模式下默认 fan-out，并按 agent 来源标注回复。
- 用户显式指定 agent 时，只允许目标 agent 对外回复。
- `memory` 采用“共享池 + agent 私有池”双层模型。
- `harness` 下沉为 agent policy，全局只保留默认值。
- Round 1 固定为“基础骨架”：
  - `AgentRuntime / ChannelAccount / AccountBinding` 的 Ent schema、manager、测试。
  - 一个只读 topology 聚合接口，供 WebUI 观察新架构对象关系。
  - 基础 CRUD/list API。
- `ChannelAccount` 的字段和 API 必须 channel-agnostic；后续各 channel 仅实现各自 adapter/driver，不重新定义账户模型。
- Round 1 明确不做：
  - WeChat/iLink 路由迁移。
  - harness runtime 下沉。
  - 多 agent fan-out / explicit-agent reply 策略。
  - 大规模 WebUI 导航重组。
- Round 1 前端只做最小承接：
  - 新增轻量 `Runtime Topology` 页面。
  - 保持当前视觉和主导航结构，只增加一个低侵入入口。

### Status
**In Progress** - 已完成设计、实施计划和 Round 1 边界收敛，正在进入 Round 1 的 schema / manager / API / 最小前端观察面实现。

### Errors Encountered
- `codeagent-wrapper --backend gemini` 在当前环境不可用：`gemini command not found in PATH`。
  - 处理：UI 信息架构分析回退到 `codex` backend，继续完成规划，不阻塞本轮设计/计划工作。
- `codeagent-wrapper --backend claude` 在当前环境不可用：`claude command not found in PATH`。
  - 处理：本轮并行辅助统一回退到 `codex` backend；如后续需要 `claude-code/opencode` 并行执行，需先补环境。

### Round 1 Execution Boundary
- [x] 新增 `pkg/runtimeagents/*`：runtime 类型、校验、CRUD manager、测试。
- [x] 新增 `pkg/channelaccounts/*`：account 类型、校验、CRUD manager、测试。
- [x] 新增 `pkg/accountbindings/*`：binding 类型、校验、CRUD manager、测试。
- [x] 新增 topology 聚合服务与 `/api/runtime-topology` 只读接口。
- [x] 新增基础 API：
  - `GET/POST/PUT/DELETE /api/runtime-agents`
  - `GET/POST/PUT/DELETE /api/channel-accounts`
  - `GET/POST/PUT/DELETE /api/account-bindings`
- [x] 前端新增最小 `Runtime Topology` 页面与 hooks。
- [x] 验证：
  - `go test -count=1 ./pkg/runtimeagents ./pkg/channelaccounts ./pkg/accountbindings ./pkg/webui`
  - `npm --prefix pkg/webui/frontend run build`
  - `go test -count=1 ./...`

### Round 1 Progress Notes
- [x] `ChannelAccount` 已按全 channel 通用模型落地，字段与 API 不再耦合 WeChat。
- [x] 已新增 Ent schema：`agentruntime`、`channelaccount`、`accountbinding`，并生成运行时代码。
- [x] 已新增 manager 包与测试：
  - `pkg/runtimeagents`
  - `pkg/channelaccounts`
  - `pkg/accountbindings`
- [x] 已新增 `pkg/runtimetopology` 聚合服务，输出 summary/runtime/account/binding 四段只读快照。
- [x] `pkg/webui/server.go` 已接入 Round 1 CRUD 与 topology 接口。
- [x] WebUI 已新增 `Runtime Topology` 页面、hook、路由和侧边栏入口。
- [x] CLI / ACP / TUI / Cron / Service 启动图已挂入新模块，避免新基础层只存在于 WebUI 局部初始化。

### Round 1 Verification
- [x] `go test ./pkg/runtimeagents ./pkg/channelaccounts ./pkg/accountbindings ./pkg/runtimetopology`
- [x] `go test ./pkg/webui -run 'TestRuntimeTopologyHandlers_CRUDAndSnapshot|TestPromptHandlers_CRUDAndResolve'`
- [x] `go test ./cmd/nekobot/... ./pkg/webui ./pkg/runtimeagents ./pkg/channelaccounts ./pkg/accountbindings ./pkg/runtimetopology`
- [x] `npm --prefix pkg/webui/frontend run build`
- [x] `go test -count=1 ./...`

### Round 2 Goal
把 `channels` 运行图从“每种 channel type 一个实例”推进到“每种 channel type 可承载多个 `ChannelAccount` 实例”，并保持对所有 channel 通用，不写成 WeChat 私有路径。

### Round 2 Decisions Made
- 第二轮不重写所有 channel；先改 `pkg/channels` 的注册/构建/管理抽象，让其支持“一种 channel type -> 多个 account runtime 实例”。
- 第二轮要保证旧配置仍可工作：没有 `ChannelAccount` 记录的 channel，继续按旧全局 channel config 启动默认实例。
- 第二轮先打开通用主链，再挑 1 到 2 个 channel 做 adapter 样板；WeChat 可以是样板之一，但不能是唯一目标。
- 多账号模型的承载对象是 `ChannelAccount`，不是往 `config.Channels.*` 里继续叠数组字段。

### Round 2 Execution Boundary
- [x] 为 `pkg/channels` 引入 “channel type + account instance” 的注册与管理抽象。
- [x] 让 `Manager` 支持同一 channel type 下的多个 runtime 实例，而不是只按 channel type 唯一键管理。
- [x] 让 `registry/build` 路径支持从 `ChannelAccount` 记录构建实例；无 account 记录时保留旧配置兼容路径。
- [x] 先为 `gotify` 与 `telegram` 提供 account-aware 样板适配，分别覆盖轻量出站型与完整聊天型 channel。
- [x] WebUI/状态接口补最小运行态可见性，`/api/channels` 新增 `_instances` 视图，Channels 页新增 runtime instances 区块。
- [x] 验证：
  - `go test -count=1 ./pkg/channels ./pkg/webui`
  - `npm --prefix pkg/webui/frontend run build`
  - `go test -count=1 ./...`

### Round 2 Progress Notes
- [x] `pkg/channels.Manager` 现已同时维护实例 ID 索引与 `channel type -> 默认实例` 兼容别名，允许同一 channel type 挂多个实例而不打断旧调用方。
- [x] `RegisterChannels` 已改为优先从 `ChannelAccount` 注册启用中的实例；若某一 channel type 没有 account 记录，则继续按旧 `config.Channels.*` 构建默认实例。
- [x] `BuildChannelFromAccount` 已落地，并先为 `gotify` / `telegram` 接通 account-aware 构建能力。
- [x] `telegram` 已完成实例 ID、会话命名空间与用户偏好命名空间隔离，默认实例继续兼容旧 `telegram:*` 格式。
- [x] Channels WebUI 已新增 runtime instance 可见性，但本轮仍未进入 account CRUD / binding-driven 控制页重构。

### Round 2 Residual Scope
- [x] 将更多 channel 逐步迁入 account-aware builder，尤其是 `wechat`、`slack` 等高价值运行时。
- [ ] 把 `ChannelAccount + AccountBinding` 真正接到消息路由与 agent runtime 解析，而不只是启动/可见性层。
- [ ] 将 WeChat 现有 `ilinkauth` 单活模型替换为真正的 channel-account 主链。

### Next Round Progress Notes
- [x] WeChat 绑定控制面已进入下一轮首段迁移：扫码确认后会同步写入 `ChannelAccount` 与 WeChat `CredentialStore`，不再只停留在 `ilinkauth` 用户绑定。
- [x] WeChat `/binding/activate` 与 `/binding/accounts/:id` 已恢复真实行为：支持切换当前账号、删除单个账号，并在操作后热重载 WeChat channel。
- [x] WeChat channel runtime 已改为优先从 `CredentialStore` 加载当前激活账号，避免旧 `ilinkauth.ListBindings()` 在多绑定场景下直接报错。
- [x] WeChat 已接入 `BuildChannelFromAccount` 路径，并具备 account runtime 实例 ID 与 session namespace 逻辑。
- [x] Slack 已补成第二个完整聊天型 account-aware runtime 样板：
  - 接入 `BuildChannelFromAccount`。
  - `ChannelID / CommandRequest.Channel / runtime metadata / session namespace` 全部按 runtime instance ID 工作。
  - 修复 `slack:team-a:C123[:thread]` 这类多段实例 ID 的 session 解析。
- [x] `discord`、`feishu`、`whatsapp`、`teams` 现也已接入 `BuildChannelFromAccount`：
  - 统一暴露 account runtime 的 `id/type/name`。
  - registry 层可直接构出 account-scoped channel 实例。
  - 当前仍未单独扩展这些 channel 的更深 account/session 语义时，只把这一步视为“account-aware builder 接通”，而非全部业务主链已完成。
- [x] `pkg/channels.Manager` 已修复 reload 索引一致性：
  - `ReloadChannel()` 会重新维护 `channelsByType/defaultByType`。
  - 避免 runtime reload 后默认别名与实例列表失真。
- [x] `pkg/channels.Manager` 已补 `nil bus` 健壮性：
  - `Start/Stop/StopChannel/ReloadChannel` 在纯 registry/test 场景下不再因空 bus 崩溃。
- [x] WeChat 控制面激活账号后，现优先按具体 `ChannelAccount` runtime 精确热重载，而不是始终依赖默认 `wechat` 兼容别名。

### Round 2/3 Integration Verification Addendum
- [x] `go test -count=1 ./pkg/channels/... ./pkg/webui -run 'TestManagerReloadChannelRestoresTypeAliasIndex|TestBuildChannelFromAccount_Slack|TestAccountChannelUsesRuntimeScopedIdentifiers|TestHandleGetChannelsIncludesRuntimeInstances|TestHandleWechatBindingActivateAndDeleteAccount'`
- [x] `go test -count=1 ./pkg/channels ./pkg/channels/slack ./pkg/channels/wechat ./pkg/webui`
- [x] `npm --prefix pkg/webui/frontend run build`
- [x] `go test -count=1 ./...`

### Round 2/3 Integration Issues Encountered
- `pkg/channels.Manager` 的 `ReloadChannel()` 只替换 `channels[id]`，没有回填 `channelsByType/defaultByType`，导致多实例 alias 在 reload 后可能失真。
  - 处理：补 type 索引恢复逻辑，并新增回归测试。
- `pkg/channels.Manager` 在 `bus == nil` 的测试/纯 registry 场景下，`StopChannel()` / `ReloadChannel()` 会空指针崩溃。
  - 处理：为 bus handler 注册/注销补空值防护，并新增回归测试。
- Slack account runtime 虽然已有 `id/channelType/name` 结构，但 `parseSessionID()` 仍按单实例前缀裁剪，无法正确解析 `slack:team-a:C123[:thread]`。
  - 处理：改为按 runtime instance 分段解析，并补账户实例 session round-trip 测试。
- [x] Slack 已接入 `BuildChannelFromAccount` 路径，并补齐 account runtime 的 `id/type/name`、session namespace、command request `Channel` 与交互回调执行上下文。
- [x] WebUI WeChat 绑定控制面已改为优先按 `channel type -> enabled ChannelAccount runtimes` 重载，不再只盯住默认 `wechat` 兼容别名。

### Current Remaining Scope
- [ ] 让 WeChat 入站消息路由与更细粒度控制面进一步按具体 `wechat:<account>` runtime instance 收敛，而不是主要依赖“active account + type 级重载”桥接策略。
- [ ] 继续把更多高价值 channel 迁入完整 account-aware runtime 路径。
  - 当前已完成 builder/account-instance 层的 channel：
    - `gotify`
    - `telegram`
    - `slack`
    - `wechat`
    - `discord`
    - `feishu`
    - `whatsapp`
    - `teams`
    - `wework`
    - `dingtalk`
    - `qq`
    - `googlechat`
  - 当前仍未完成的是“更深的 account-aware 业务语义”，不是 `BuildChannelFromAccount` 基础接线本身。
- [ ] 将 `AccountBinding` 接入真实消息路由与 agent runtime 解析。
  - 当前新增进度：已不止停留在 WebUI/runtime reload；`pkg/channelaccounts.Manager.ResolveForChannelID("wechat")` 现已按 active WeChat account 优先解析裸 `wechat` 别名，且 `pkg/inboundrouter` 已补端到端回归，验证裸 `wechat` 入站消息会命中 active account 绑定的 runtime，而不是按列表顺序漂移。

### Round 2.1 Verification
- [x] `go test -count=1 ./pkg/channels/slack ./pkg/channels/wechat ./pkg/webui -run 'Test(BuildChannelFromAccount_Slack|ParseSessionIDSupportsAccountRuntimePrefix|HandleMessageEventUsesAccountRuntimeIdentifiers|ExecuteConfirmedSkillInstallUsesRuntimeChannelID|HandleWechatBindingLifecycle_UsesSharedIlinkAuth|HandleWechatBindingActivateAndDeleteAccount|ReloadChannelsByTypePrefersEnabledWechatAccounts|GetWechatBindingStatus_NoBinding)'`
- [x] `go test -count=1 ./...`
- [x] `npm --prefix pkg/webui/frontend run build`

## 当前项目评估

### 已完成能力（可视为现阶段稳定基线）
- [x] Web-first runtime 管理：providers / channels / prompts / tool sessions / cron / QMD / skills 运行态可在 WebUI 管理。
- [x] 多 provider 路由：provider pools、fallback chains、cooldown、runtime route override。
- [x] 多通道接入：Telegram / Discord / WhatsApp / Feishu / QQ / DingTalk / Slack / WeChat / 更多扩展通道。
- [x] Skills 系统：多路径加载、eligibility、远程搜索/安装、快照、版本追踪、always skill。
- [x] Memory 与 QMD：builtin memory、QMD fallback、session export、运行态检查。
- [x] Cron：`cron` / `at` / `every`、delete-after-run、run-now、WebUI 管理。
- [x] Tool sessions：PTY、访问控制、生命周期管理、Web 端查看。
- [x] Agent 稳定性：session history sanitize、safe history window、tool-call 保真、context force compression。

### 当前明确未完成或做薄的能力
- [x] Slack interactive callback 已补齐首批完整业务闭环：`find_skills`、`settings`、`model`、`help`、`status`、`agent`、`start` 的 shortcut/modal submission 均已接通并补回归；后续如继续扩展，以新增业务场景单列。
- [x] Runtime Prompts 本轮改动后已补回归测试与 smoke checklist 记录。
- [x] MaixCAM 命令执行结果已支持直写设备连接，且出站链路已补齐按 `session/device` 定向回写；后续若继续增强，可单列 richer device protocol。
- [ ] Gateway 仍偏聊天通道，缺更完整的控制面协议、连接治理和配对/授权模型。
- [x] Conversation binding 已补首批通用基础层：支持绑定记录视图、按 conversation/session 检索、绑定元数据与过期清理；更完整的跨 account/独立存储层仍待继续迁移。
- [ ] Browser session 仍缺更完整的 relay/CDP 高级控制动作。
  - 进展补充：`auto/direct/relay` 已完成，`print_pdf`、`extract_structured_data`、`get_text` 已落地；同时已补 custom `debug_port` / `debug_endpoint`，把 attach 从固定本地 9222/9223/9224 端口扫描收口为“默认扫描 + 可选显式 endpoint/port 覆盖”；后续仍待继续迁移更完整的 relay/CDP 高级动作。
- [x] Memory 检索后处理首轮质量增强已完成：MMR、多样性、时间衰减、引用格式、embedding cache 已落地；后续如继续扩展，以更高阶排序/多源融合为新事项单列。
- [ ] 现有 channel 能力已补基础 capability 矩阵，但平台差异仍未全面接入各 channel 运行时消费路径。
- [ ] 缺“按聊天用户长期驻留的外部 agent runtime”这一层，尚未形成类似 `gua` 的用户级外部代理会话编排。
- [ ] `pkg/wechat` 目前仍缺 `gua/libc/wechat` 的完整 SDK 分层，微信通道能力还主要堆在 `pkg/channels/wechat/*` 中。
- [x] WeChat 通道发送侧已补齐基于共享 SDK 的附件上传/发送链路，可将回复中的本地文件路径提升为平台附件消息。
- [x] WeChat 通道本地协议/登录/client 重复实现已下线，WebUI 与 channel 已统一依赖共享 `pkg/wechat` SDK。
- [ ] WeChat Presenter / 交互协议仍未完整迁移，当前已完成输出规则注入与技能安装确认的 `/yes` `/no` `/cancel` 闭环，`/select N` 等更通用交互尚未接入更广泛场景。

## Active Backlog（含进度）

### P0（优先先做）
- [x] **`gua/libc/wechat` SDK 全量迁移到 `nekobot/pkg/wechat`**
  - 现状：`nekobot` 只有轻量 `pkg/wechat/media`，其余微信协议、client、cdn、messaging、monitor、typing、voice、parse 等能力仍散落在 `pkg/channels/wechat/*` 或尚未抽出。
  - 目标：先把 `gua/libc/wechat` 的 shared SDK 形态完整迁入 `pkg/wechat`，形成后续微信功能扩展的稳定底座。
  - 来源：`gua/libc/wechat/*`。
  - 位置：`pkg/wechat/*`。
- [x] **WeChat 通道附件发送增强（本地文件路径 -> 图片/视频/文件消息）**
  - 现状：`pkg/channels/wechat/channel.go` 仅支持纯文本与 base64 内联图片；回复里出现本地文件路径时不会自动转成微信附件发送。
  - 已完成：通道运行、发送、typing、QR 绑定已改用共享 `pkg/wechat` SDK；回复链路可自动提取本地路径，按图片/视频/文件分类上传发送，并清理正文中的路径文本。
  - 来源：`gua/server/formatter.go`、`gua/channel/wechat/wechat.go`。
  - 位置：`pkg/channels/wechat/*`、`pkg/wechat/*`。
- [x] **Slack interactive callback 扩展为完整交互闭环**
  - 已完成：技能安装确认已补齐完整闭环（pending state / expiry / 原消息更新 / 统一执行路径），并已把 `find_skills`、`settings`、`model`、`help`、`status`、`agent`、`start` 的 shortcut/modal submission 业务闭环全部接通。
  - 已验证：`pkg/channels/slack` 定向回归已覆盖 shortcut 打开 modal 与 view submission 执行 command 的主路径。
  - 来源：当前 `pkg/channels/slack/slack.go` 缺口 + `gua` Presenter/Action 思路。
  - 位置：`pkg/channels/slack/slack.go`。
- [x] **Runtime Prompts 执行链路做完整回归并固化检查清单**
  - 已完成：补齐 manager / webui 回归测试，覆盖 CRUD、session replace/cleanup、模板上下文渲染、disabled 忽略、同一 prompt 的多作用域覆盖优先级。
  - 已完成：新增 `docs/RUNTIME_PROMPTS.md`，固化 smoke checklist 与行为说明。
  - 位置：`pkg/prompts/*`、`pkg/webui/*`、`pkg/agent/*`。

### P1（高价值缺口）
- [x] **通用 conversation/thread binding 层（当前批次范围）**
  - 现状：`pkg/conversationbindings/service.go` 只是在 tool session 之上做 source/channel/conversation 绑定，缺跨 account/conversation/session 的通用记录、清理与路由抽象。
  - 进度：已完成首批基础层增强，支持绑定记录视图、按 conversation/session 检索、绑定元数据与过期清理；已补齐绑定查询结果的稳定排序语义，`ListBindings` / `GetBindingsBySession` / session 级 record 展开不再依赖写入顺序；本轮继续补齐 rebinding 语义，旧 session 在失去主 conversation 后会按稳定排序提升新的 primary conversation key。当前仍复用 `tool sessions` 持久化，尚未抽出独立存储与跨 account 统一模型。
  - 已完成范围：当前批次已完成通用查询/写入契约、确定性排序、rebinding 语义和 WeChat 首个消费者对齐。
  - 后续剩余：独立存储与更广的 `gateway/external runtime` 接入不再算本批未完成项，应在后续单列。
  - 目标：抽出可复用于 channels / gateway / external agent runtime 的统一绑定层。
  - 来源：`goclaw/channels/thread_bindings.go`、`thread_binding_storage.go`。
  - 位置：新建 `pkg/conversationbindings/*` 或扩展现有模块。
  - 计划：`docs/superpowers/plans/2026-04-03-conversation-thread-binding.md`
- [x] **Memory 检索质量增强包**
  - 现状：已有 hybrid search 与 QMD fallback；首轮质量增强项已全部补齐。
  - 进度：已完成引用格式切片、首批 MMR 重排接入、时间衰减接入与 embedding LRU cache，补齐统一 citation 生成、builtin memory 搜索后的多样性重排、时间感知排序以及重复文本的 embedding 复用。
  - 目标：提升长上下文/多来源检索质量，同时保持现有接口稳定。
  - 来源：`goclaw/memory/mmr.go`、`temporal_decay.go`、`citations.go`、`lru_cache.go`。
  - 位置：`pkg/memory/*`。
- [ ] **Gateway 控制面与连接治理增强**
  - 现状：`pkg/gateway/server.go` 原先是开放 `CheckOrigin`、简单 WS/REST 模式；本轮已补第一层 origin allowlist，且把基础连接治理推进到 IP/rate limit，但控制面授权边界此前仍停留在“任何有效 JWT 都可访问”。
  - 进度：已新增 `gateway.allowed_origins` 配置，并把 websocket origin 校验从“全部放行”收口为“配置驱动 allowlist + 空 Origin 兼容非浏览器客户端”；已把 `GET /api/v1/status` 与 `GET /api/v1/connections` 从裸露状态收口为复用现有 JWT 鉴权；已补上已鉴权的 `DELETE /api/v1/connections/{id}`，让控制面可主动断开指定 websocket 连接；已把连接列表收口为稳定排序，并补出 `connected_at` / `remote_addr` / `session_id` 基本元数据，减少控制面返回的隐式不确定性；已补上已鉴权的 `GET /api/v1/connections/{id}`，让控制面可以查询单连接详情而不必扫全量列表；已新增 `gateway.max_connections` 配置与 server 内硬限制，开始收口最基本的连接数量治理；已完成 `gateway.allowed_ips`，补齐基于远端地址的 allowlist，并同时覆盖 websocket 握手与 REST 控制面入口；已完成 `gateway.rate_limit_per_minute`，以共享入口级的 per-IP 限流同时覆盖 REST 控制面和 websocket 握手；已完成 control-plane role scope 收紧：`status` 继续只允许 `admin/owner`，`member` 只能读取属于自己的连接元数据；已完成首个 pairing 薄切片：websocket 握手支持通过 `session_id` 复用既有 gateway session，并拒绝未知 session 或非 gateway session；已补上 pairing 薄切片的握手时序修正：无效 `session_id` 现在会在 websocket upgrade 前直接返回真实 `400`；已进一步补齐 pairing hardening：兼容 legacy 空 `source` 的旧 gateway session、拒绝同一 paired session 的重复 live attach、串行化并发 attach 窗口，并要求 websocket 入站消息的 `session_id` 与当前 active paired session 保持一致；已补齐控制面细粒度删除边界：bulk delete 与 single delete 现在都允许 `member` 删除自己拥有的 live connection，对其他用户或不存在的目标统一返回 `404`，且 bulk-delete 的 `remaining` 只返回调用方可见剩余数，不再泄露全局 remaining；已进一步补 member 可见面收口：连接列表/详情里的 `remote_addr` 已对 member 响应脱敏；已补齐 pairing proof：新增 `TestWSChatConcurrentAttachConflictKeepsRequestedExistingSession`，锁住并发 attach 冲突不会破坏既有 paired gateway session；已补当前 websocket handler 的两个稳定性缺口：`session unavailable` 现在会在 upgrade 前返回真实 HTTP 失败，且 router 接管 websocket chat 时不再重复走 inbound bus；当前又补齐连接诊断字段 `session_source/requested_session_id`，让控制面能区分 `generated/requested/legacy` pairing 来源。下一最小切片再转向更细的控制面/配对协议收口。
  - 目标：补控制面协议与连接策略，避免 gateway 只停留在“聊天 socket”。
  - 来源：`goclaw/gateway/openclaw/*`。
  - 位置：`pkg/gateway/*`。
- [ ] **Browser session 双模式与高级提取动作**
  - 现状：`pkg/tools/browser_session.go` 已具备 `auto/direct/relay` attach 语义与自定义 `debug_port/debug_endpoint`，浏览器工具继续按独立 CDP 动作补能力面。
  - 进度：已完成首批 session 层增强，支持 `auto/direct` 模式和优先复用已运行 Chrome 的连接策略；浏览器工具现已暴露 `mode` 参数；第一版 `relay` 模式已落地，语义先收口为“只连接既有浏览器实例、绝不自行 launch”；高级提取动作已补 `print_pdf`、`extract_structured_data`、`get_text`；本轮继续补上 `get_metrics`、`emulate_device`、`set_viewport`，以及基于 active DevTools endpoint 的 `list_pages` / `new_page` / `activate_page` / `close_page` 页面控制动作，并补参数/辅助构造/DevTools seam 测试，作为更完整 relay/CDP 高级控制面的下一批基础能力。
  - 下一切片：继续补更细的 relay/CDP 高级动作（例如 network/console 观察或更强 session control）；page-scoped storage 控制已作为后续小切片落地，不再重复做 attach endpoint/port 接入。
  - 目标：提升浏览器工具的可靠性和能力上限。
  - 来源：`goclaw/agent/tools/browser_session.go`、`browser_relay.go`、`browser_cdp.go`。
  - 位置：`pkg/tools/browser*.go`。
- [x] **OAuth 凭证中心管理器**
  - 已完成：`pkg/auth/center.go` 已提供统一 `CredentialCenter`，支持 provider/account 级 `Put/Get/List/Validate/Refresh/Revoke`，并对现有 `AuthStore` 做兼容写透；CLI 侧也已补 `nekobot auth refresh --provider ...` 作为显式刷新入口。
  - 已验证：`pkg/auth/center_test.go` 已覆盖 write-through、refresh 持久化回写、生命周期状态推导、revoke 删除与无 refresh token 拒绝路径；`./cmd/nekobot` 已通过包级测试。
  - 来源：`goclaw/providers/oauth/*`。
  - 位置：`pkg/auth/*`。

### P2（次优先级）
- [x] **MaixCAM 命令响应回设备端**
  - 现状：命令执行结果已直写回设备连接；本轮补齐了 bus 出站消息按 `session/device` 定向回写，避免设备侧回复被无差别广播到所有已连接终端。
  - 目标：保持设备命令和 agent 出站链路都能稳定回到对应设备。
  - 位置：`pkg/channels/maixcam/maixcam.go`。
- [ ] **Channel capability 矩阵**
  - 现状：基础 capability 矩阵、scope 和默认平台映射已迁入；真实消费切片已落在：
    - `whatsapp` 的 `native_commands=off`
    - `telegram` 的 `inline_buttons=dm`
    - `wework` 的 `native_commands=off`
    这三条默认矩阵现在都会在运行时生效；本轮另外补齐了 `account-scoped runtime ID` 下的 capability 读取不再误用实例 ID，避免 `whatsapp:*` / `wework:*` 在多账号模式下错误绕过 `native_commands=off` 默认矩阵；并把 `dingtalk`、`qq`、`googlechat` 的 native command 入口统一接到 capability 判定上，减少“直接 `commands.IsCommand(...)`”旁路。当前仍未完成的是更多 channel/runtime 对其余 capability 的全面消费。
  - 目标：统一 reactions / buttons / threads / polls / streaming / native commands 等能力声明，并逐步用于运行时决策。
  - 来源：`goclaw/channels/capabilities.go`。
  - 位置：`pkg/channels/*`。
- [ ] **按用户隔离的外部 agent runtime**
  - 现状：`nekobot` 有 tool session 和本地 agent，但缺 `gua` 式“每个聊天用户绑定一个长期外部 agent 进程/工作目录/权限回路”的编排层。
  - 进度：已补最小 foundation：`pkg/externalagent.Manager` 现在能按 `owner + agent_kind + workspace` 解析或创建复用的 detached `tool_session`；空 workspace 会默认到配置根目录，相对 workspace 会解析到该根目录下；已补首个权限/工作目录策略切片：当 `agents.defaults.restrict_to_workspace=true` 时，只允许 external-agent workspace 落在配置的 workspace 根目录内；并已补首个 launcher allowlist：当前只允许 `codex/claude/opencode/aider` 四类 canonical `tool + command` 身份；WebUI `/api/external-agents/resolve-session` 已成为首个真实 consumer，并且已从只读 `launch_policy` 预览推进到首条真实 pending approval 流：deny 直接拒绝，ask/manual 进入 pending approval 并同步 session pending state；approve 后同 session 已具备最小 resume seam，且现在已能在 approve 当下直接继续启动 process；此外 WeChat `codex` create path 已成为首个真实 channel consumer，并复用 externalagent normalization，且现在也已接上 shared starter 与 shared resolve orchestrator；gateway `POST /api/v1/external-agents/resolve-session` 也已成为首个 gateway consumer，并已通过共享 orchestrator 对齐 `launch_policy` explainability，且在批准/放行后也能通过共享 starter 真实启动 process；gateway 现已补齐 externalagent approval list/approve/deny API，形成控制面闭环；同时已抽出共享 `pkg/externalagent/orchestrator.go` 与 `pkg/externalagent/starter.go`，分别统一 resolve 阶段与 process-start 阶段，并开始收敛三条 consumer 的 approval UX contract，其中 WebUI/gateway 已对齐 shared HTTP response helper。当前仍未补更广 consumer 上统一的一体化 approval UX。
  - 目标：为 Claude Code / Codex / 其他外部 agent 准备用户级长期会话底座。
  - 来源：`gua/agent/claude/session.go`、`gua/agent/claude/mcp.go`、`gua/server/server.go`。
  - 位置：新建 `pkg/externalagent/*` 或等价模块。
- [ ] **WeChat Presenter / 交互协议与附件输出管线**
  - 现状：附件输出已完成；presenter 输出规则已注入 agent 输入；交互协议方面已补齐技能安装确认的 `/yes` `/no` `/cancel` 闭环，且 presenter 现在会明确要求模型在需要用户选择时输出稳定编号列表并提示可用 `/select N`；技能安装确认提示当前也已显式渲染 `1. 允许安装 / 2. 拒绝安装`，并允许用户直接回复 `1/2`。managed externalagent launch 现在也已支持在 WeChat 里直接 `/yes` `/no` 继续或拒绝。更广泛的弱交互场景仍待继续接入。
  - 目标：继续增强弱交互通道上的可操作性。
  - 来源：`gua/channel/wechat/presenter.go`、`gua/server/formatter.go`。
  - 位置：`pkg/channels/wechat/*`、公共 formatter 层。
- [ ] **Runtime 交互检测与 tmux/TTY 控制层**
  - 现状：已有 PTY/tool session，但缺针对外部交互式 agent 的 prompt 检测、菜单识别、自动确认和持续观察层。
  - 进度：当前已补齐三条更稳定的 runtime-control 基础 seam：agent 创建的 `tool_session` 在走 `tmux` 包装启动时，会把 `runtime_transport=tmux`、`tmux_session` 与实际 `launch_cmd` 显式持久化到 session metadata，而不再只体现在返回文案里；WebUI 在按需从 tmux 恢复 detached session 时，也会把恢复后的 attach 命令和 tmux session 回写到 metadata；此外 `process/status` 已新增只读 observation 视图，能从最近 PTY 输出中识别 `idle` / `awaiting_input` / `menu_prompt` / `error_prompt` 等轻量运行态信号。当前仍未进入自动确认/自动选择，只先把观察面收口。
  - 目标：为外部 agent runtime 提供稳定的交互底座。
  - 来源：`gua/runtime/*`。
  - 位置：`pkg/toolsessions/*` 或新建 runtime 模块。
- [ ] **Docker 多阶段构建 + 外部 runtime 环境收口**
  - 目标：改进部署一致性，为 browser / tmux / external agent 运行时留出稳定基础镜像。

## 当前批次规划（可并行）

### Batch A（Backlog 收口与验证）
- [x] 清理已过期 backlog（`gateway restart/reload`、memory text similarity、skills version compare、cron 多调度类型）。
- [x] 跑 `go test -count=1 ./...`
- [x] 跑 `npm --prefix pkg/webui/frontend run build`
- **验收**: 过期 backlog 移除，验证结果补充到 `progress.md`。

### Batch B（Channel 交互补齐）
- [x] WeChat SDK baseline migration
- [x] WeChat attachment send pipeline
- [x] Slack interactive handlers 第一阶段扩展（skill install confirm 完整闭环 + shortcut/modal 路由入口）
- [x] Subagent origin notify 接线
- [x] MaixCAM command response
- **验收**: 各 channel 回归测试通过，关键交互可手工验证。

### Batch C（Runtime Admin 收口）
- [x] prompts CRUD / binding / render 回归与 smoke checklist
- [x] tool sessions 与 QMD 管理页 smoke test
- [x] frontend build 与后端全量测试补跑
- **验收**: `go test -count=1 ./...` 与 `npm --prefix pkg/webui/frontend run build` 通过。

### Batch D（goclaw 高价值迁移）
- [ ] conversation/thread binding 层
  - 当前执行顺序：先做通用 service 契约收口，再做 WeChat runtime 消费者验证，最后再决定是否扩到 gateway/external runtime。
- [x] memory quality pack（MMR / temporal decay / citations / cache）
- [ ] gateway control plane hardening
  - 当前已完成切片：`gateway.allowed_ips`、`gateway.rate_limit_per_minute`、控制面读写分离 role scope、pairing 首批 hardening、`member` 单/批量删除都仅可作用于自有 live connection、paired-session conflict retention proof、upgrade 前 session-unavailable 失败语义修正、router 接管 websocket chat 时停止重复 inbound bus 投递；当前本地继续更细的控制面/配对协议收口，不扩到 enrollment / ownership 持久化。
- [ ] browser session dual-mode / advanced extraction
- 当前已完成切片：`auto/direct/relay`、`print_pdf`、`extract_structured_data`、`get_text`、custom `debug_port/debug_endpoint`、`get_metrics`、`emulate_device`、`set_viewport`、`list_pages`、`new_page`、`activate_page`、`close_page`、`get_storage`、`set_storage`、`remove_storage`、`clear_storage`、`get_console`、`get_network`。
- 当前下一切片：继续补更完整 relay/CDP 高级动作与会话控制（network 等），而不是重复做 attach endpoint/port 接入。
- **验收**: 每项独立测试通过，按功能独立提交与推送。

### Batch E（gua 高价值迁移）
- [ ] user-scoped external agent runtime foundation
- 当前已完成切片：`pkg/externalagent.Manager` 的 session resolve/create foundation、`restrict_to_workspace` 驱动的 workspace policy gate、canonical launcher allowlist、共享 resolve orchestrator 首刀、共享 process starter 首刀、共享 resolve-flow 执行链、WebUI resolve-session consumer + 首条真实 pending approval 流、approve 后的最小 resume seam、approve 当下直接 continue 启动、WebUI consumer 上的 process spawn/continue seam、WeChat codex consumer 接线，以及 gateway resolve-session consumer 接线；其中 WebUI/gateway/WeChat(codex) 已开始共享 resolve orchestrator 与 process starter，WebUI/gateway 已开始共享 resolve-flow 执行链与 shared HTTP response helper，gateway 已补齐自身 approval UX 闭环，三条 consumer 的 approval UX contract 也已开始收敛。后续继续补更统一的跨 consumer approval UX。
- [ ] permission / elicitation bridge
- [ ] presenter + attachment pipeline
- [ ] runtime prompt detection / tmux control
  - 当前新增进度：已补 tmux transport metadata persistence seam、restore 后 attach metadata persistence，并在 `process/status` 透出只读 observation 视图；后续继续往更细的 prompt/menu detection 与自动确认控制扩展。
- **验收**: 每项独立 smoke test + channel flow 验证通过，按功能独立提交与推送。

## 2026-04-08 FastClaw 可引入能力评估（后续并行开发候选）

### 前端补全原则
- 参考 `fastclaw` 引入的新能力，默认都要在 `nekobot` 同步补齐对应前端，不只做后台。
- 至少满足三类前端交付之一：
  1. **管理面**：配置/开关/策略编辑
  2. **观测面**：状态、日志、结果、错误、待处理项
  3. **交互面**：必要的 approve/deny、测试触发、wizard/引导
- 若某一能力暂时不适合完整 UI，也至少要补：
  - API shape
  - 前端 hook
  - 最小只读状态面
- 后续所有 `fastclaw` 来源切片都按“backend + frontend paired slice”规划，不接受长期纯后端悬空。

### P1 候选：共享 Hook Pipeline
- 来源：`/home/czyt/code/fastclaw/internal/agent/hooks.go`
- 价值：
  - `nekobot` 计划里已经多次提到 `pre/post/failure hooks`
  - 当前 permission / approval / tool execution 已有基础，适合再补统一 hook surface
- 建议最小切片：
  1. 先定义 hook point 与最小上下文结构
  2. 先只接 `before_tool_call / after_tool_call / post_turn`
  3. 后续再扩 model/prompt hooks
  4. 前端至少补一个只读 hook observability 面（例如最近 hook 事件/启用状态）

### P1 候选：通用 Webhook Trigger Server
- 来源：`/home/czyt/code/fastclaw/internal/webhook/server.go`
- 价值：
  - 当前 `nekobot` 缺统一“外部系统 -> agent/message bus”触发入口
  - 这类入口适合与现有 gateway/webui/runtime control 并列，而不是塞进 channel 逻辑
- 建议最小切片：
  1. `POST /api/webhooks/agent` 或等价 endpoint
  2. bearer token 鉴权
  3. 最小 payload：agent/runtime/session/message
  4. 前端补一个最小 webhook 配置/测试面

### P1 候选：Filesystem/Network/Tool Policy Engine
- 来源：`/home/czyt/code/fastclaw/internal/policy/*`
- 价值：
  - `nekobot` 现在已有 permission rules，但主要还是 tool/action 级
  - 对外部 agent/runtime、sandbox、exec 进一步收口时，需要更底层的 fs/network/tool policy
- 建议最小切片：
  1. 先只引入只读 policy model 与 evaluator
  2. 首批只接 `tools.exec` / externalagent workspace & network
  3. 暂不做完整 YAML policy productization
  4. 前端至少补 policy inspector，再逐步补编辑面

### P2 候选：Guided Setup Wizard / First-Run Flow
- 来源：`/home/czyt/code/fastclaw/internal/setup/*`
- 价值：
  - `fastclaw` 的 setup wizard 对首次配置 provider/agent/web UI 很友好
  - `nekobot` 已有大量 config 页面，但首次启动路径仍可优化
- 建议最小切片：
  1. 只做 first-run detection
  2. 只引导 admin/provider/workspace 三项
  3. 不在同批次里接完整 cron/plugins/channels 配置
  4. 这是天然 backend+frontend paired slice，优先按全链路交付

### P2 候选：OpenAI-Compatible API Hardening
- 来源：`/home/czyt/code/fastclaw/internal/api/openai.go`
- 价值：
  - `fastclaw` 在 chat completions / SSE 上有一条更清晰的对外 API 线
  - 可作为 `nekobot` 对外 API 兼容层的参考
- 建议最小切片：
  1. 先审计现有对外 chat API 形状
  2. 只补最小 compat/sse/headers gap
  3. 不混入完整 public API 产品化
  4. 前端补最小 API smoke/debug 面即可，不强求重 UI

### P3 候选：Plugin Runtime（暂缓）
- 来源：`/home/czyt/code/fastclaw/internal/plugin/*`
- 备注：
  - 有价值，但当前 `task_plan.md` 已明确“不新增独立 plugin system 主线”
  - 先作为后续候选，不进入当前主执行线

### 推荐执行顺序
1. Hook Pipeline
2. Webhook Trigger Server
3. Policy Engine
4. Setup Wizard / First-Run
5. OpenAI-Compatible API Hardening
6. Plugin Runtime（暂缓）

### 当前执行进度
- [x] Hook Pipeline：`pkg/tools/Registry` 已形成最小 before/after hook pipeline
- [x] Webhook Trigger Server：已补 backend + frontend 最小成套（config + test UI）
- [x] Policy Engine：已补 presets + evaluator + frontend inspector
- [x] Setup Wizard / First-Run：已补 workspace bootstrap 可视化 + repair 动作
- [~] OpenAI-Compatible API Hardening：已落一个 headers hardening 小切片，后续继续补下一个 compat/smoke slice
- [~] Channel-visible tool trace：WeChat + ServerChan 已接入共享 `tool_call/tool_result` 文本 trace；其余经 bus 异步回包的 channel 还需单独补事件链路

## Phase 状态（聚合）
- [x] Phase 1-8（前端主链路）
- [x] Phase 10（Cron）
- [ ] Phase 9（Session 管理增强）
- [ ] Phase 11（Marketplace）
- [ ] Phase 12（系统状态与收尾）
- [ ] Phase 13（goclaw 高价值实践迁移）
- [ ] Phase 14（gua 外部 agent/runtime 迁移）

## Notes
- 执行策略保持：**最小改动、完成一项继续下一项、每批次可独立回归**。
- 进度细节（逐次提交级）继续记录在 `progress.md`，本文件作为任务和里程碑主视图。
- 开发要求：**功能完成一项就提交和推送代码**；非功能性的评估、计划整理不单独算功能交付。
- 本轮并行评估已吸收 `goclaw` 与 `gua` 的分析结论，后续实现时继续由主 agent 审批每个子任务结论。
- 跨设备续写建议起点：微信阶段已完成并推送，优先继续 Batch B 的 Slack 交互闭环，再补 Batch C 的 runtime prompts 回归，随后开始 `goclaw` / `gua` 迁移批次。

---

## 2026-03-29 复审批次：Harness / Learnings / WebUI Config

### Goal
复审最近 5 次与 harness、undo/watch、learnings、audit log、streaming bash、WebUI config 暴露相关提交，修复确认存在的业务流程问题、前端体验问题与必要架构缝隙。

### Phases
- [x] Phase 1: 建立复审计划并核对当前工作树状态
- [x] Phase 2: 逐提交阅读代码与影响面
- [x] Phase 3: 修复已证实问题并补验证
- [x] Phase 4: 更新计划、记录结论与剩余风险

### Review Scope
- `1b7c3d0` `docs: update harness progress tracker`
- `580741d` `feat: add Turn Undo, @file Mentions, and Watch Mode harness features`
- `46026ac` `feat: add Learnings JSONL system`
- `583245d` `feat(webui): add harness config sections to ConfigPage`
- `c409cf1` `feat: add audit log and streaming bash`

### Confirmed Fixes
- [x] WebUI ConfigPage 为新增 `audit` / `undo` / `preprocess` / `learnings` / `watch` 分组补齐 label/description 映射，并把 section 元数据收敛为单一映射源，避免再次出现新增分组但漏文案的回归。
- [x] WebUI i18n 补齐上述 5 个分组的 `en` / `zh-CN` / `ja` 文案，修复分组标题和说明显示不完整问题。
- [x] `pkg/session/snapshot.go` 修复连续增量快照错误地按空 `Messages` 长度计算 delta 的问题，避免消息重建重复。
- [x] `pkg/session/snapshot.go` 在 `Undo()` 后重写 `.snapshots.jsonl`，使撤销结果可持久化，不会下次加载又“反弹”。
- [x] `pkg/tools/streaming.go` 为流式更新补齐 `session_id` 透传。
- [x] `pkg/tools/exec.go` 对 `streaming=true` 但缺少 handler 的情况增加明确 fallback 提示，避免静默降级。
- [x] `pkg/watch/watcher.go` 将高频查找路径改为读锁，降低 watch event 串行化开销。

### Verification
- [x] `npm --prefix pkg/webui/frontend run build`
- [x] `go test -count=1 ./pkg/session`
- [x] `go test -count=1 ./pkg/tools ./pkg/watch`

### Remaining Risks / Follow-up
- [ ] `undo` 当前仍存在架构级缺口：`RegisterUndoTool()` 没有实际调用点，且工具执行不会回写 live session / summary。本轮只修了数据层正确性与持久化，未强行把共享 agent 注册模型改成会话级原子回滚，以避免把多会话串线风险带入主线。
- `46026ac` `feat: add Learnings JSONL system for durable context compression`
- `583245d` `feat(webui): add harness config sections to ConfigPage`
- `c409cf1` `feat: add audit log and streaming bash from yoyo-evolve harness`

### Review Focus
- 业务流程：
  - undo 与 session snapshot 写入顺序、session ID 解析、回滚边界
  - watch mode 的事件匹配、并发锁、日志、安全关闭、重复触发
  - learnings JSONL 的写入/读取边界和 prompt/context 注入行为
  - streaming bash / audit log 的注册、输出、失败路径、调用边界
- 前端体验：
  - ConfigPage 新增 harness config section 的可理解性、可发现性、显示完整性
- 架构：
  - 新增模块是否职责过浅、跨模块编排是否割裂、是否存在更合适的边界收敛点

### Decisions Made
- 当前工作树存在未提交修改：`pkg/agent/agent.go`、`pkg/watch/watcher.go`；视为现状的一部分一起审，不先回滚。
- 只修复能通过代码/验证证明的问题，不做泛化“优化”。

### Errors Encountered
- 待补充

### Status
**Phase 4 Complete** - 修复已证实问题并补验证，更新计划、记录结论与剩余风险。所有已确认修复已实施并验证通过。
