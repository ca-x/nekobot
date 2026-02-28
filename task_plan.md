# Task Plan: nanoclaw / nextclaw 设计迁移与收口

> Last Updated: 2026-02-28

## Goal
在保持 `nekobot` 现有稳定性的前提下，分批迁移并落地 `nanoclaw` / `nextclaw` / `picoclaw` 的高价值能力，优先保证：
1. 现有 provider fallback / tool 执行语义不回退。
2. 每一批改动都可独立回归验证。
3. 优先完成“用户可见且当前明确未实现”的缺口。

## Progress Snapshot
- **总体状态**: 持续推进中（主线可用，进入收口阶段）。
- **关键已完成批次**:
  - [x] Feature Batch #1：多用户认证基础迁移（User/Tenant/Membership + JWT 统一存储）。
  - [x] Feature Batch #2：`chatWithBladesOrchestrator` 由真实 blades runtime 接管，保留 legacy 可切换。
  - [x] Prompt 构建优化：静态块缓存 + 动态时间占位替换，避免时间冻结。
- **最近验证**:
  - [x] `go test -count=1 ./pkg/agent`
  - [x] `go test -count=1 ./...`

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

## Active Backlog（含进度）

### P0（优先先做）
- [ ] **实现 `/gateway restart` 真正执行路径**
  - 现状：仅返回“not yet implemented”。
  - 目标：接入实际服务控制/进程重启策略（先本地可控场景）。
  - 位置：`pkg/commands/advanced.go`。
- [ ] **实现 `/gateway reload` 配置热重载**
  - 现状：仅返回“not yet implemented”。
  - 目标：支持配置重载且不中断核心服务。
  - 位置：`pkg/commands/advanced.go`。
- [ ] **校正文档与实现差异（slash commands）**
  - 现状：`/skill` 文档仍标注“direct execution not implemented”，但代码已执行。
  - 目标：文档与真实行为一致，补充当前限制说明。
  - 位置：`docs/SLASH_COMMANDS.md`。

### P1（高价值缺口）
- [ ] **Slack interactive callback 落地**
  - 现状：回调已接收但未处理。
  - 目标：支持关键交互（至少技能安装确认流）。
  - 位置：`pkg/channels/slack/slack.go`。
- [ ] **Subagent 完成通知回传 origin channel**
  - 现状：TODO 留空。
  - 目标：任务完成/失败时可回推通知，补齐异步体验。
  - 位置：`pkg/subagent/manager.go`。
- [ ] **Memory Hybrid Search 文本相似度**
  - 现状：`textSim` 固定 `0.0`。
  - 目标：实现轻量关键词匹配并接入混合打分。
  - 位置：`pkg/memory/store.go`。
- [ ] **Skills 版本约束校验**
  - 现状：工具/语言版本比较为 TODO。
  - 目标：支持最小可用版本比较并返回可读原因。
  - 位置：`pkg/skills/eligibility.go`。

### P2（次优先级）
- [ ] **MaixCAM 命令响应回设备端**
  - 现状：命令执行后仅日志输出。
  - 目标：将 command response 回写设备通道。
  - 位置：`pkg/channels/maixcam/maixcam.go`。
- [ ] **Docker 多阶段构建 + tmux 收口**
  - 目标：改进部署与运行环境一致性。
- [ ] **后端架构增强（借鉴 picoclaw/nextclaw）**
  - 目标：在不破坏现有模块的前提下，分批收敛到更清晰的边界。

## 当前批次规划（可并行）

### Batch 3（Command 收口）
- [ ] `/gateway restart`
- [ ] `/gateway reload`
- [ ] `docs/SLASH_COMMANDS.md` 对齐
- **验收**: command 层测试 + 最小集成验证通过。

### Batch 4（Channel 交互补齐）
- [ ] Slack interactive handlers
- [ ] Subagent origin notify
- [ ] MaixCAM command response
- **验收**: 各 channel 回归测试通过，关键交互可手工验证。

### Batch 5（Memory/Skills 质量补齐）
- [ ] memory hybrid keyword scoring
- [ ] skills version comparison
- **验收**: 单测覆盖新分支，`go test ./...` 通过。

## Phase 状态（聚合）
- [x] Phase 1-8（前端主链路）
- [x] Phase 10（Cron）
- [ ] Phase 9（Session 管理增强）
- [ ] Phase 11（Marketplace）
- [ ] Phase 12（系统状态与收尾）

## Notes
- 执行策略保持：**最小改动、完成一项继续下一项、每批次可独立回归**。
- 进度细节（逐次提交级）继续记录在 `progress.md`，本文件作为任务和里程碑主视图。