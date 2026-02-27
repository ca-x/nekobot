# Notes: nextclaw + picoclaw → nekobot 特性分析

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

### nextclaw 关键文件
- Agent Loop: `nextclaw-core/src/agent/loop.ts`
- Input Budget: `nextclaw-core/src/agent/input-budget-pruner.ts`
- Skills: `nextclaw-core/src/agent/skills.ts`
- Context: `nextclaw-core/src/agent/context.ts`
- Cron Service: `nextclaw-core/src/cron/service.ts`
- Provider Manager: `nextclaw-core/src/providers/provider_manager.ts`
- Memory: `nextclaw-core/src/agent/memory.ts`

### nekobot 需改进的文件
- Agent Loop: `/home/czyt/code/go/nekobot/pkg/agent/agent.go`
- Provider Failover: `/home/czyt/code/go/nekobot/pkg/providers/failover.go`
- Error Classifier: `/home/czyt/code/go/nekobot/pkg/providers/` (需增强)
- Circuit Breaker: `/home/czyt/code/go/nekobot/pkg/providers/loadbalancer.go`
- Cron: `/home/czyt/code/go/nekobot/pkg/cron/cron.go`
- Session: `/home/czyt/code/go/nekobot/pkg/session/manager.go`
- Skills: `/home/czyt/code/go/nekobot/pkg/skills/`
- Tools: `/home/czyt/code/go/nekobot/pkg/tools/registry.go`
