# Notes: nextclaw + picoclaw → nekobot 特性分析

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
