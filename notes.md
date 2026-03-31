# Notes: nextclaw + picoclaw → nekobot 特性分析

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
