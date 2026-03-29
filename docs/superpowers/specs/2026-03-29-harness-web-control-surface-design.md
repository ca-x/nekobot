# Harness Web Control Surface Design

## Goal
在保持 `nekobot` 当前 Web-first 架构不回退的前提下，把已接入但不完整的 harness 相邻能力补成完整工作流：确保 harness 配置真正可通过 Web 持久化，并为 `watch`、安全版多级 `undo`、`@file` 注入反馈提供统一的后端能力和 Web 体验。

## Scope
- 修复 WebUI ConfigPage 对 `audit` / `undo` / `preprocess` / `learnings` / `watch` 的真实读写链路。
- 增加 watch 的状态读取与最小控制入口。
- 增加 Web chat 的多级 undo 能力，但不实现 `--all` 这种全工作树破坏性回滚。
- 增加 Web chat 对 `@file` / `@dir` 注入结果的显式反馈。

## Non-Goals
- 不复刻 `yoyo-evolve` 的完整 REPL 命令模型。
- 不在本轮做完整 audit log 浏览页。
- 不在本轮重做 learnings 管理台。
- 不引入 git checkout 风格的全局 `undo --all`。

## Architecture

### 1. Runtime config truthfulness
当前 `ConfigPage` 已显示 harness sections，但 `/api/config` 和 `config.SaveDatabaseSections` 没把这些 sections 纳入响应与持久化范围。先修正配置源真相：
- `handleGetConfig` 返回 `audit` / `undo` / `preprocess` / `learnings` / `watch`
- `handleSaveConfig` / `handleImportConfig` 接受并持久化这些 sections
- `config.runtimeConfigSections` 扩展以支持 DB override 和导入导出闭环

这是本轮第一优先级，因为后续 watch 控制与相关 UI 都依赖真实配置链路。

### 2. Watch control surface
`pkg/watch.Watcher` 已有运行态，但缺状态快照与显式控制。
本轮新增：
- `Watcher.Status()` 返回：
  - `enabled`
  - `running`
  - `debounce_ms`
  - `patterns`
  - `last_run_at`
  - `last_command`
  - `last_file`
  - `last_success`
  - `last_error`
  - `last_result_preview`
- Web API：
  - `GET /api/harness/watch`
  - `POST /api/harness/watch` 用于轻量更新 `enabled` / `debounce_ms` / `patterns`

这个 API 不做“局部热更新复杂 orchestration”，先直接更新 runtime config 并让现有 watcher 尽可能重载；若重载不安全，则明确返回 restart required。

### 3. Safe multi-step undo for Web chat
`UndoTool` 当前是工具语义，不适合 Web chat 的用户操作。新增 server 级 undo workflow：
- `POST /api/chat/session/:id/undo`
  - 参数：`steps`，默认 1
  - 只允许正整数，并 clamp 到可撤销量
- Server 直接调用 `snapshotMgr.GetStore(sessionID)` 完成多级回滚
- 回滚后把 snapshot messages 写回 `session.Session`
- 返回：
  - `undone_steps`
  - `remaining_turns`
  - `messages`

这会形成真正的“会话内容已回滚”的闭环，而不是只返回一句工具说明文本。

### 4. File mention feedback
`preprocess.Process()` 已能解析 `@file` 和 `@dir`，但 Web chat 看不到本轮到底内联了什么文件。
本轮新增：
- agent 暴露一个“message preprocessing preview”能力，供 Web chat 在发送前或处理时获得：
  - `processed_input`
  - `mentions`
  - `warnings`
- WebSocket 响应新增一种 metadata/system event：
  - 告知本次消息内联了多少个文件/目录引用
  - 显示引用的相对路径与可选行范围

不把完整文件内容再回显到 UI，只给出轻量可见反馈，避免噪音和泄露风险。

## Web UX

### Chat page
- 在 composer 附近新增轻量提示区：
  - 发送后若发生 `@file` 注入，显示 “Inlined 2 file references”
  - 可展开查看相对路径列表
- 在会话工具栏新增：
  - `Undo` 按钮
  - 附近显示可撤销 turns 数量
  - 若无可撤销内容则禁用
- 在 route/status 区旁新增 watch 状态 pill：
  - `Watch ON` / `Watch OFF`
  - 显示最近运行状态摘要
  - 点击可打开配置页对应 section

### Config page
- 保持现有 harness sections 布局
- 修复其真实持久化链路
- 可在 watch section 中看到最近运行摘要（若后端已提供）

## Testing strategy
- `pkg/webui/server_config_test.go`
  - harness sections 出现在 `/api/config`
  - harness sections 可保存并被 `ApplyDatabaseOverrides` 读回
- `pkg/watch/watcher_test.go`
  - `Status()` 在执行后返回最近一次运行结果
- `pkg/webui/server_chat_test.go`
  - Web chat undo API 多级回滚成功并回写 session
  - `@file` 触发时有 metadata/system feedback
- 前端：
  - 至少保持 `npm --prefix pkg/webui/frontend run build`
  - 如已有前端测试基础，补 hooks / rendering 测试；若无，则优先类型检查与 build

## Risks
- watch 运行态与配置更新之间可能需要明确 restart/reload 语义，不能默默假装热更新成功。
- undo 回写 session 时必须保持 message/tool-call 结构一致，不能只回滚文本。
- `@file` 反馈不能把大量文件内容再次塞进聊天区，否则会制造双重噪音。
