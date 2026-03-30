# Task Plan: nekobot 功能评估、迁移与收口

> Last Updated: 2026-03-26

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
- [ ] 继续把 Telegram / WeChat / ServerChan 等仍直接调全局 agent 的 channel 逐步迁到 `pkg/inboundrouter` 主链。
- [ ] 将 runtime `PromptID` 真正接入 prompt resolution，而不是当前仅透传 runtime metadata。
- [ ] 为 runtime/account/binding 增加真正可编辑的 WebUI 页面，而不是仅有只读 topology。

### Status
**Completed** - 已建立统一入站路由主干，完成 bus 语义拆分与 gateway 接线，并通过定向测试、全量 Go 回归和前端构建验证。

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
- [ ] 将更多 channel 逐步迁入 account-aware builder，尤其是 `wechat`、`slack` 等高价值运行时。
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
- [ ] 将 `AccountBinding` 接入真实消息路由与 agent runtime 解析。

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
- [ ] Slack interactive callback 已补齐技能安装确认闭环，但 shortcut / modal submission 仍只有路由入口，尚未落具体业务闭环。
- [x] Runtime Prompts 本轮改动后已补回归测试与 smoke checklist 记录。
- [x] MaixCAM 命令执行结果已支持直写设备连接，且出站链路已补齐按 `session/device` 定向回写；后续若继续增强，可单列 richer device protocol。
- [ ] Gateway 仍偏聊天通道，缺更完整的控制面协议、连接治理和配对/授权模型。
- [x] Conversation binding 已补首批通用基础层：支持绑定记录视图、按 conversation/session 检索、绑定元数据与过期清理；更完整的跨 account/独立存储层仍待继续迁移。
- [ ] Browser session 仍是单例固定端口 CDP，缺 relay 模式与更完整的高级控制动作。
  - 进展补充：`auto/direct` 已完成，`print_pdf`、`extract_structured_data`、`get_text` 已落地；relay 与更多 CDP 高级动作仍待继续迁移。
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
- [ ] **Slack interactive callback 扩展为完整交互闭环**
  - 现状：仅有 block actions 中的 skill install confirm/cancel，而且确认流没有像 Telegram / Discord 一样的 pending state、expiry、原消息更新和统一执行路径；shortcut / modal submission 也仍未处理。
  - 进度：已补齐技能安装确认的完整闭环（pending state / expiry / 原消息更新 / 统一执行路径），并加上 shortcut / modal submission 的可扩展路由入口；后续只剩更具体的 modal/shortcut 业务。
  - 目标：继续在现有路由骨架上补具体 shortcut / modal submission 业务。
  - 来源：当前 `pkg/channels/slack/slack.go` 缺口 + `gua` Presenter/Action 思路。
  - 位置：`pkg/channels/slack/slack.go`。
- [x] **Runtime Prompts 执行链路做完整回归并固化检查清单**
  - 已完成：补齐 manager / webui 回归测试，覆盖 CRUD、session replace/cleanup、模板上下文渲染、disabled 忽略、同一 prompt 的多作用域覆盖优先级。
  - 已完成：新增 `docs/RUNTIME_PROMPTS.md`，固化 smoke checklist 与行为说明。
  - 位置：`pkg/prompts/*`、`pkg/webui/*`、`pkg/agent/*`。

### P1（高价值缺口）
- [ ] **通用 conversation/thread binding 层**
  - 现状：`pkg/conversationbindings/service.go` 只是在 tool session 之上做 source/channel/conversation 绑定，缺跨 account/conversation/session 的通用记录、清理与路由抽象。
  - 进度：已完成首批基础层增强，支持绑定记录视图、按 conversation/session 检索、绑定元数据与过期清理；当前仍复用 `tool sessions` 持久化，尚未抽出独立存储与跨 account 统一模型。
  - 目标：抽出可复用于 channels / gateway / external agent runtime 的统一绑定层。
  - 来源：`goclaw/channels/thread_bindings.go`、`thread_binding_storage.go`。
  - 位置：新建 `pkg/conversationbindings/*` 或扩展现有模块。
- [x] **Memory 检索质量增强包**
  - 现状：已有 hybrid search 与 QMD fallback；首轮质量增强项已全部补齐。
  - 进度：已完成引用格式切片、首批 MMR 重排接入、时间衰减接入与 embedding LRU cache，补齐统一 citation 生成、builtin memory 搜索后的多样性重排、时间感知排序以及重复文本的 embedding 复用。
  - 目标：提升长上下文/多来源检索质量，同时保持现有接口稳定。
  - 来源：`goclaw/memory/mmr.go`、`temporal_decay.go`、`citations.go`、`lru_cache.go`。
  - 位置：`pkg/memory/*`。
- [ ] **Gateway 控制面与连接治理增强**
  - 现状：`pkg/gateway/server.go` 仍是开放 `CheckOrigin`、简单 WS/REST 模式，缺 origin/IP/scope/rate limit/pairing 等治理。
  - 目标：补控制面协议与连接策略，避免 gateway 只停留在“聊天 socket”。
  - 来源：`goclaw/gateway/openclaw/*`。
  - 位置：`pkg/gateway/*`。
- [ ] **Browser session 双模式与高级提取动作**
  - 现状：`pkg/tools/browser_session.go` 仍缺 relay 模式与更丰富的高级浏览器动作。
  - 进度：已完成首批 session 层增强，支持 `auto/direct` 模式和优先复用已运行 Chrome 的连接策略；浏览器工具现已暴露 `mode` 参数并显式拒绝未支持的模式；高级提取动作已补 `print_pdf`、`extract_structured_data`、`get_text`；relay 模式与更多 CDP 高级动作仍待继续迁移。
  - 目标：提升浏览器工具的可靠性和能力上限。
  - 来源：`goclaw/agent/tools/browser_session.go`、`browser_relay.go`、`browser_cdp.go`。
  - 位置：`pkg/tools/browser*.go`。
- [ ] **OAuth 凭证中心管理器**
  - 现状：`pkg/auth/*` 偏单次登录流程，缺按 provider/profile 统一管理、自动刷新、校验与持久化中心。
  - 目标：支持更稳的 OAuth provider 运维能力。
  - 来源：`goclaw/providers/oauth/*`。
  - 位置：`pkg/auth/*` 或新建 `pkg/oauth/*`。

### P2（次优先级）
- [x] **MaixCAM 命令响应回设备端**
  - 现状：命令执行结果已直写回设备连接；本轮补齐了 bus 出站消息按 `session/device` 定向回写，避免设备侧回复被无差别广播到所有已连接终端。
  - 目标：保持设备命令和 agent 出站链路都能稳定回到对应设备。
  - 位置：`pkg/channels/maixcam/maixcam.go`。
- [ ] **Channel capability 矩阵**
  - 现状：基础 capability 矩阵、scope 和默认平台映射已迁入，但各 channel/runtime 尚未全面消费这层声明。
  - 目标：统一 reactions / buttons / threads / polls / streaming / native commands 等能力声明，并逐步用于运行时决策。
  - 来源：`goclaw/channels/capabilities.go`。
  - 位置：`pkg/channels/*`。
- [ ] **按用户隔离的外部 agent runtime**
  - 现状：`nekobot` 有 tool session 和本地 agent，但缺 `gua` 式“每个聊天用户绑定一个长期外部 agent 进程/工作目录/权限回路”的编排层。
  - 目标：为 Claude Code / Codex / 其他外部 agent 准备用户级长期会话底座。
  - 来源：`gua/agent/claude/session.go`、`gua/agent/claude/mcp.go`、`gua/server/server.go`。
  - 位置：新建 `pkg/externalagent/*` 或等价模块。
- [ ] **WeChat Presenter / 交互协议与附件输出管线**
  - 现状：附件输出已完成；presenter 输出规则已注入 agent 输入；交互协议方面已补齐技能安装确认的 `/yes` `/no` `/cancel` 闭环，但 `/select N` 等更通用场景还未接入。
  - 目标：继续增强弱交互通道上的可操作性。
  - 来源：`gua/channel/wechat/presenter.go`、`gua/server/formatter.go`。
  - 位置：`pkg/channels/wechat/*`、公共 formatter 层。
- [ ] **Runtime 交互检测与 tmux/TTY 控制层**
  - 现状：已有 PTY/tool session，但缺针对外部交互式 agent 的 prompt 检测、菜单识别、自动确认和持续观察层。
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
- [x] memory quality pack（MMR / temporal decay / citations / cache）
- [ ] gateway control plane hardening
- [ ] browser session dual-mode / advanced extraction
- **验收**: 每项独立测试通过，按功能独立提交与推送。

### Batch E（gua 高价值迁移）
- [ ] user-scoped external agent runtime foundation
- [ ] permission / elicitation bridge
- [ ] presenter + attachment pipeline
- [ ] runtime prompt detection / tmux control
- **验收**: 每项独立 smoke test + channel flow 验证通过，按功能独立提交与推送。

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
