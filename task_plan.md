# Task Plan: nekobot 功能评估、迁移与收口

> Last Updated: 2026-03-25

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
- [ ] Slack interactive callback 仅覆盖 skill install confirm/cancel，shortcut 与 modal submission 仍未落业务闭环。
- [ ] Runtime Prompts 本轮改动后缺一次完整端到端回归与 smoke test 记录。
- [ ] MaixCAM 命令执行后的 response 回写设备侧链路仍待补齐。
- [ ] Gateway 仍偏聊天通道，缺更完整的控制面协议、连接治理和配对/授权模型。
- [ ] Conversation binding 目前偏 tool session 绑定，缺跨 channel/account/conversation 的通用线程绑定层。
- [ ] Browser session 仍是单例固定端口 CDP，缺 relay/direct/auto 模式与高级提取动作。
- [ ] Memory 检索后处理仍偏轻量，缺 MMR、多样性、时间衰减、引用格式、缓存等质量件。
- [ ] 现有 channel 能力缺统一 capability 矩阵，平台差异还分散在各 channel 私实现里。
- [ ] 缺“按聊天用户长期驻留的外部 agent runtime”这一层，尚未形成类似 `gua` 的用户级外部代理会话编排。
- [ ] `pkg/wechat` 目前仍缺 `gua/libc/wechat` 的完整 SDK 分层，微信通道能力还主要堆在 `pkg/channels/wechat/*` 中。
- [ ] WeChat 通道发送侧仍缺基于共享 SDK 的附件上传/发送链路，无法把回复中的本地文件路径提升为平台附件消息。

## Active Backlog（含进度）

### P0（优先先做）
- [x] **`gua/libc/wechat` SDK 全量迁移到 `nekobot/pkg/wechat`**
  - 现状：`nekobot` 只有轻量 `pkg/wechat/media`，其余微信协议、client、cdn、messaging、monitor、typing、voice、parse 等能力仍散落在 `pkg/channels/wechat/*` 或尚未抽出。
  - 目标：先把 `gua/libc/wechat` 的 shared SDK 形态完整迁入 `pkg/wechat`，形成后续微信功能扩展的稳定底座。
  - 来源：`gua/libc/wechat/*`。
  - 位置：`pkg/wechat/*`。
- [ ] **WeChat 通道附件发送增强（本地文件路径 -> 图片/视频/文件消息）**
  - 现状：`pkg/channels/wechat/channel.go` 仅支持纯文本与 base64 内联图片；回复里出现本地文件路径时不会自动转成微信附件发送。
  - 目标：基于迁入后的共享 SDK，补齐 WeChat CDN 上传与图片/视频/文件发送链路，并在回复链路里自动提取本地路径、清理正文中的路径文本。
  - 来源：`gua/server/formatter.go`、`gua/channel/wechat/wechat.go`。
  - 位置：`pkg/channels/wechat/*`、`pkg/wechat/*`。
- [ ] **Slack interactive callback 扩展为完整交互闭环**
  - 现状：仅有 block actions 中的 skill install confirm/cancel，shortcut / modal submission 仍未处理。
  - 目标：至少补齐关键交互流，形成统一 action routing。
  - 来源：当前 `pkg/channels/slack/slack.go` 缺口 + `gua` Presenter/Action 思路。
  - 位置：`pkg/channels/slack/slack.go`。
- [ ] **Runtime Prompts 执行链路做完整回归并固化检查清单**
  - 现状：Prompts 存储、绑定和 WebUI 已落地，但本轮提交后尚未做一次完整端到端回归。
  - 目标：验证 CRUD、绑定、生效渲染、升级迁移、WebUI 交互，并记录到进度文件。
  - 位置：`pkg/prompts/*`、`pkg/webui/*`、`pkg/agent/*`。

### P1（高价值缺口）
- [ ] **通用 conversation/thread binding 层**
  - 现状：`pkg/conversationbindings/service.go` 只是在 tool session 之上做 source/channel/conversation 绑定，缺跨 account/conversation/session 的通用记录、清理与路由抽象。
  - 目标：抽出可复用于 channels / gateway / external agent runtime 的统一绑定层。
  - 来源：`goclaw/channels/thread_bindings.go`、`thread_binding_storage.go`。
  - 位置：新建 `pkg/conversationbindings/*` 或扩展现有模块。
- [ ] **Memory 检索质量增强包**
  - 现状：已有 hybrid search 与 QMD fallback，但缺 MMR、多样性、时间衰减、引用格式与缓存。
  - 目标：提升长上下文/多来源检索质量，同时保持现有接口稳定。
  - 来源：`goclaw/memory/mmr.go`、`temporal_decay.go`、`citations.go`、`lru_cache.go`。
  - 位置：`pkg/memory/*`。
- [ ] **Gateway 控制面与连接治理增强**
  - 现状：`pkg/gateway/server.go` 仍是开放 `CheckOrigin`、简单 WS/REST 模式，缺 origin/IP/scope/rate limit/pairing 等治理。
  - 目标：补控制面协议与连接策略，避免 gateway 只停留在“聊天 socket”。
  - 来源：`goclaw/gateway/openclaw/*`。
  - 位置：`pkg/gateway/*`。
- [ ] **Browser session 双模式与高级提取动作**
  - 现状：`pkg/tools/browser_session.go` 为单例 + 固定 `9222` 端口，缺复用外部 Chrome、relay/direct/auto 模式、PDF/结构化抽取。
  - 目标：提升浏览器工具的可靠性和能力上限。
  - 来源：`goclaw/agent/tools/browser_session.go`、`browser_relay.go`、`browser_cdp.go`。
  - 位置：`pkg/tools/browser*.go`。
- [ ] **OAuth 凭证中心管理器**
  - 现状：`pkg/auth/*` 偏单次登录流程，缺按 provider/profile 统一管理、自动刷新、校验与持久化中心。
  - 目标：支持更稳的 OAuth provider 运维能力。
  - 来源：`goclaw/providers/oauth/*`。
  - 位置：`pkg/auth/*` 或新建 `pkg/oauth/*`。

### P2（次优先级）
- [ ] **MaixCAM 命令响应回设备端**
  - 现状：命令执行后仅日志输出。
  - 目标：将 command response 回写设备通道。
  - 位置：`pkg/channels/maixcam/maixcam.go`。
- [ ] **Channel capability 矩阵**
  - 现状：平台差异主要分散在各 channel 私实现中。
  - 目标：统一 reactions / buttons / threads / polls / streaming / native commands 等能力声明。
  - 来源：`goclaw/channels/capabilities.go`。
  - 位置：`pkg/channels/*`。
- [ ] **按用户隔离的外部 agent runtime**
  - 现状：`nekobot` 有 tool session 和本地 agent，但缺 `gua` 式“每个聊天用户绑定一个长期外部 agent 进程/工作目录/权限回路”的编排层。
  - 目标：为 Claude Code / Codex / 其他外部 agent 准备用户级长期会话底座。
  - 来源：`gua/agent/claude/session.go`、`gua/agent/claude/mcp.go`、`gua/server/server.go`。
  - 位置：新建 `pkg/externalagent/*` 或等价模块。
- [ ] **WeChat Presenter / 交互协议与附件输出管线**
  - 现状：微信通道已有基础能力，但缺 `gua` 风格的 `/yes` `/no` `/cancel` `/select N` 交互协议，以及“文本中引用文件路径 -> 平台附件发送”的统一 formatter。
  - 目标：增强弱交互通道上的可操作性。
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
- [ ] 跑 `go test -count=1 ./...`
- [ ] 跑 `npm --prefix pkg/webui/frontend run build`
- **验收**: 过期 backlog 移除，验证结果补充到 `progress.md`。

### Batch B（Channel 交互补齐）
- [x] WeChat SDK baseline migration
- [ ] WeChat attachment send pipeline
- [ ] Slack interactive handlers 扩展
- [x] Subagent origin notify 接线
- [ ] MaixCAM command response
- **验收**: 各 channel 回归测试通过，关键交互可手工验证。

### Batch C（Runtime Admin 收口）
- [ ] prompts CRUD / binding / render 手工回归
- [ ] tool sessions 与 QMD 管理页 smoke test
- [ ] frontend build 与后端全量测试补跑
- **验收**: `go test -count=1 ./...` 与 `npm --prefix pkg/webui/frontend run build` 通过。

### Batch D（goclaw 高价值迁移）
- [ ] conversation/thread binding 层
- [ ] memory quality pack（MMR / temporal decay / citations / cache）
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
- 跨设备续写建议起点：先优先执行 Batch A 验证，再进入 Batch B/C，随后开始 `goclaw` / `gua` 迁移批次。
