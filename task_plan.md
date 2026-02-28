# Task Plan: 从 nextclaw 移植 Web UI 功能到 nekobot

## Goal
参考 nextclaw 项目的 Web 体验，将前端迁移到 React + TypeScript + Vite 技术栈，移植 Provider/Channel 管理、Runtime 配置、Session 管理、Cron 管理、Marketplace 等功能，同时完整保留现有的 Tool Session 和 Chat Playground 功能。

## Phases

### Phase 1: 项目脚手架搭建
- [ ] 1.1 在 `pkg/webui/frontend/` 下初始化 React + Vite + TypeScript 项目
- [ ] 1.2 安装核心依赖 (React Router, TanStack Query, Zustand, Tailwind, shadcn/ui, Lucide, Sonner, React Hook Form, Zod, xterm.js)
- [ ] 1.3 配置 Vite (dev proxy 到 nekobot 后端, build 输出到 dist/)
- [ ] 1.4 设置 Tailwind + 设计系统 CSS 变量 (参考 nextclaw design-system.css)
- [ ] 1.5 搭建 shadcn/ui 基础组件 (button, card, dialog, input, label, select, switch, tabs, tooltip, scroll-area, skeleton)
- [ ] 1.6 更新 `embed.go` 适配新的构建产物
- [ ] 1.7 配置 Makefile/脚本: `npm run build` → Go embed 流程

### Phase 2: 基础框架层
- [ ] 2.1 API 客户端层 (`src/api/client.ts`) - JWT token 管理, fetch 封装, 错误处理
- [ ] 2.2 WebSocket 客户端 (`src/api/websocket.ts`)
- [ ] 2.3 认证 API (`src/api/auth.ts`) - login, init, profile, password
- [ ] 2.4 i18n 系统 (`src/lib/i18n.ts`) - 支持 en/zh-CN/ja 三语言, 迁移现有 177 键
- [ ] 2.5 主题系统 (`src/lib/theme.ts`) - light/dark 双主题
- [ ] 2.6 Zustand 全局状态 (`src/stores/`)
- [ ] 2.7 路由设置 (`src/App.tsx`) - React Router 懒加载各页面

### Phase 3: 布局与认证
- [ ] 3.1 AppLayout 组件 - 侧边栏 + 主内容区
- [ ] 3.2 Sidebar 导航 - Logo, 导航项, 语言/主题切换
- [ ] 3.3 Header 组件 - 页面标题 + 描述
- [ ] 3.4 初始化页面 - 首次运行创建管理员账户
- [ ] 3.5 登录页面 - JWT 认证
- [ ] 3.6 账户设置对话框 - 密码修改/个人信息
- [ ] 3.7 通用 UI 组件 - MaskedInput, KeyValueEditor, TagInput, LogoBadge, StatusBadge, ConfirmDialog

### Phase 4: Chat Playground (保留重写)
- [ ] 4.1 Chat 页面组件 - Provider/Model 选择器
- [ ] 4.2 Chat WebSocket 通信 - 实时对话
- [ ] 4.3 聊天消息渲染 - Markdown 支持
- [ ] 4.4 Fallback provider 配置
- [ ] 4.5 自定义模型输入

### Phase 5: Tool Sessions (保留重写)
- [ ] 5.1 Tool Sessions 页面 - 侧边栏会话列表 + 主面板
- [ ] 5.2 xterm.js 终端组件 - WebSocket PTY 通信
- [ ] 5.3 创建/编辑工具会话对话框 (10+ 字段)
- [ ] 5.4 访问控制对话框 (OTP/永久密码)
- [ ] 5.5 会话状态管理 (running/detached/terminated/archived)
- [ ] 5.6 终端分屏视图
- [ ] 5.7 会话重启/终止/清理操作

### Phase 6: Provider 管理 (参考 nextclaw 升级)
- [ ] 6.1 Provider API hooks (`src/hooks/useProviders.ts`)
- [ ] 6.2 ProvidersList 页面 - 卡片网格视图, "已配置/全部" 分页 (参考 nextclaw ProvidersList)
- [ ] 6.3 ProviderForm 模态框 - API Key (MaskedInput), Endpoint, Proxy, Timeout, ExtraHeaders (KeyValueEditor)
- [ ] 6.4 模型发现/选择 UI - 自动发现 + 手动添加
- [ ] 6.5 Provider Logo 映射 + 模板预设

### Phase 7: Channel 管理 (参考 nextclaw 升级)
- [ ] 7.1 Channel API hooks (`src/hooks/useChannels.ts`)
- [ ] 7.2 ChannelsList 页面 - 卡片网格视图, "已启用/全部" 分页 (参考 nextclaw ChannelsList)
- [ ] 7.3 ChannelForm 动态表单 - 根据 channel 类型生成字段 (参考 nextclaw ChannelForm)
- [ ] 7.4 Channel 测试功能
- [ ] 7.5 Channel Logo 映射

### Phase 8: 配置管理 (增强)
- [ ] 8.1 Model 配置页 (新, 参考 nextclaw ModelConfig) - 默认模型, workspace, max tokens, context tokens, tool iterations
- [ ] 8.2 Runtime 配置页 (新, 参考 nextclaw RuntimeConfig) - Agent 列表, 绑定路由, DM scope, ping-pong
- [ ] 8.3 通用配置页 - 表单模式 + JSON 模式切换 (保留现有 section 编辑)
- [ ] 8.4 配置导入/导出

### Phase 9: Session 管理 (新功能, 参考 nextclaw)
- [ ] 9.1 后端 API - 会话列表/历史/编辑/删除端点
- [ ] 9.2 SessionsConfig 页面 - 分屏: 列表 + 详情
- [ ] 9.3 聊天历史浏览器 - 消息渲染 + 分页
- [ ] 9.4 会话元数据编辑 (标签/首选模型)
- [ ] 9.5 搜索和筛选功能

### Phase 10: Cron 任务管理 (新功能, 参考 nextclaw)
- [ ] 10.1 后端 API - 定时任务 CRUD 端点
- [ ] 10.2 CronConfig 页面 - 任务列表 + 搜索/筛选
- [ ] 10.3 启用/禁用/立即执行/删除操作
- [ ] 10.4 调度信息展示 (at/every/cron 三种类型)

### Phase 11: Marketplace 市场 (新功能, 参考 nextclaw)
- [ ] 11.1 后端 API - 插件/技能 浏览/安装/管理端点
- [ ] 11.2 MarketplacePage 页面 - 目录浏览/已安装 分页
- [ ] 11.3 搜索/排序/分页
- [ ] 11.4 安装/启用/禁用/卸载操作

### Phase 12: 系统状态 & 收尾
- [ ] 12.1 System 状态页增强
- [ ] 12.2 Approval 管理页 (保留现有)
- [ ] 12.3 i18n 全量翻译校对 (en/zh-CN/ja)
- [ ] 12.4 响应式布局优化 (移动端适配)
- [ ] 12.5 构建流程集成测试
- [ ] 12.6 清理旧 vanilla JS 文件

## Key Decisions
- **前端框架**: 迁移到 React 18 + TypeScript + Vite (与 nextclaw 一致，便于参考和维护)
- **UI 组件库**: shadcn/ui + Tailwind CSS (参考 nextclaw 设计系统)
- **状态管理**: Zustand (UI) + TanStack React Query (服务端状态)
- **后端 API**: 保持 nekobot 现有 API 结构，必要时新增端点
- **Tool Session**: 完整保留，用 React 组件重写前端 (保留 xterm.js)
- **Chat Playground**: 完整保留，用 React 组件重写
- **嵌入方式**: 保持 Go embed, Vite 构建输出到 dist/
- **i18n**: 保持三语言支持 (en/zh-CN/ja)

## Completed Work
- [x] Phase 1: React+Vite+TypeScript 脚手架搭建
- [x] Phase 2-3: 基础框架层+布局+认证
- [x] Phase 4: Chat Playground (React重写)
- [x] Phase 5: Tool Sessions (React重写, xterm.js)
- [x] Phase 6: Provider 管理 (卡片网格+表单)
- [x] Phase 7: Channel 管理 (动态表单)
- [x] Phase 8: Config + System 页面
- [x] Web Interface Guidelines 审查完成

## Pending Work
- [ ] 修复 Web Interface Guidelines 违规项
- [ ] Docker: 多阶段构建 + tmux
- [ ] 后端架构增强 (从 picoclaw/nextclaw 借鉴)

## Agent 架构分析 (2026-02-27)
详见 notes.md - 从 nextclaw + picoclaw 分析了4个维度:
1. 架构稳健性: 原子文件写入、Provider Failover增强、上下文压缩、历史清理
2. Skill增强: Registry Manager远程源、Always Skills、Skill Summary XML
3. Cron增强: at/every类型、原子持久化、DeleteAfterRun、Web UI
4. UI易用性: CronConfig页面、Session浏览器、确定性工具排序

## Errors Encountered
- npx tsc 安装了错误的包 → 改用 ./node_modules/.bin/tsc

## Status
**Phase 1-8 前端完成** - 待修复guidelines违规项 + Docker + 后端架构增强

---

## Current Task: 参考 nanoclaw 设计迁移到 nekobot (2026-02-27)

### Goal
系统梳理 `nanoclaw` 的优秀设计，并在 `nekobot` 中落地第一批高价值迁移改动。

### Phases
- [x] Phase 1: Plan and setup
- [x] Phase 2: Research/gather information
- [x] Phase 3: Execute/build
- [x] Phase 4: Review and deliver

### Key Questions
1. 先迁移哪一类设计（Provider 路由/容错、技能系统、会话与 Cron、配置体系）？
2. 目标是“最大化复用现有结构”还是“按 nanoclaw 方式重构相关模块”？
3. 本轮是否允许引入新的配置字段与 API 端点？

### Decisions Made
- 采用“先对比评估 + 分批迁移”的方式，避免大范围一次性改动。
- 本轮功能块聚焦 Phase 1：先完成多用户认证基础（用户/租户/成员关系 + JWT 统一秘钥管理 + WebUI 认证接口迁移）。

### Errors Encountered
- 自动记忆目录不存在：`/Users/czyt/.claude/projects/-Users-czyt-code-go-nekobot/memory/`。

### Completed Deliverables (Feature Batch #1)
- 新增 ent schema：`User` / `Tenant` / `Membership`，并完成对应 ent 代码生成。
- 新增认证存储层：`pkg/config/auth_store.go`，落地用户/租户/成员关系迁移与 JWT secret 统一存储读取。
- 重构 `pkg/config/admin_credential.go`：认证、用户资料更新、密码更新、登录记录与 auth profile 构建全部走结构化表。
- 重构 `pkg/webui/server.go`：移除内存态 admin credential，认证接口改为 DB 驱动，新增 `/auth/me`，JWT claims 补齐 `uid/tid/ts`。
- 更新 `pkg/gateway/server.go`：JWT 校验读取统一走 `config.GetJWTSecret`。
- 补充测试：`pkg/config/db_store_test.go` 新增迁移与 legacy JWT payload 兼容测试。

### Status
**Feature Batch #1 Completed** - 已完成并验证 Phase 1 多用户认证基础迁移，下一步进入 Phase 2（Agent Profile + blades orchestrator 接管设计与实现）。

### Completed Deliverables (Feature Batch #2)
- 新增 `pkg/agent/blades_runtime.go`，将 `chatWithBladesOrchestrator` 从 stub 改为真实 blades runtime 执行链路。
- 新增 `bladesModelProvider` 适配层，复用现有 `buildProviderOrder` + `callLLMWithFallback` 语义，保留 provider fallback 行为。
- 在 blades 模型调用中保留上下文超限压缩重试（`isContextLimitError` + `forceCompressMessages`），与 legacy 路径行为对齐。
- 新增 `bladesToolResolver`，将 blades tool handler 桥接到现有 `executeToolCall`，继续复用审批与工具执行逻辑。
- 会话历史接入 blades session：复用 `sanitizeHistory`，并完成 `agent.Message` 到 `blades.Message` 转换。
- `pkg/agent/agent.go` 移除旧的 blades stub；`pkg/agent/agent_test.go` 更新 blades 路径断言；`pkg/subagent/manager.go` 接口调整为 `Chat(ctx, sess, message)` 并补充 `taskSession`。
- `go.mod` / `go.sum` 引入 blades 运行时依赖。

### 验证 (Feature Batch #2)
- 已执行：`go test ./pkg/agent ./pkg/subagent ./pkg/tools ./pkg/config ./pkg/webui ./pkg/gateway`
- 结果：全部通过。

### Status
**Feature Batch #2 Completed** - blades orchestrator 已由真实 runtime 接管，legacy orchestrator 仍保留并可通过配置切换。
