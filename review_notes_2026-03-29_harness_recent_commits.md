# Notes: 2026-03-29 Harness / Learnings / WebUI Recent Commit Review

## Scope

Review targets:
- `1b7c3d0` `docs: update harness progress tracker`
- `580741d` `feat: add Turn Undo, @file Mentions, and Watch Mode harness features`
- `46026ac` `feat: add Learnings JSONL system for durable context compression`
- `583245d` `feat(webui): add harness config sections to ConfigPage`
- `c409cf1` `feat: add audit log and streaming bash from yoyo-evolve harness`

## Working Tree State Before Review

- Unstaged changes already present in:
  - `pkg/agent/agent.go`
  - `pkg/watch/watcher.go`
- These files overlap with the review scope and must be treated as current state, not discarded blindly.

## Review Angles

### Business Flow
- Snapshot / undo consistency
- Watch mode lifecycle and duplicate event control
- Learning persistence and prompt inclusion boundaries
- Streaming exec / audit log event ordering and failure paths

### Frontend UX
- Config exposure vs user comprehension
- Missing validation / helper text / error surfaces
- Display completeness on existing config UI

### Architecture
- Shallow modules and duplicated orchestration logic
- Boundary ownership across `agent`, `session`, `watch`, `audit`, `tools`, `webui`

## Findings

### Commit-by-Commit Notes
- `583245d` `ConfigPage` 新增了 `audit` / `undo` / `preprocess` / `learnings` / `watch` 分组，但页面映射和 i18n 文案未补齐，属于直接可见的显示完整性回归。
- `580741d` / `pkg/session/snapshot.go` 的增量快照算法在连续多个非 checkpoint turn 下会重复拼接旧消息，导致当前态与 undo 重建错误。
- `580741d` / `pkg/session/snapshot.go` 的 `Undo()` 只裁剪内存数组，不重写磁盘 JSONL，撤销不具备持久性。
- `c409cf1` / `pkg/tools/streaming.go` 的 `StreamingUpdate.SessionID` 字段未被实际填充，前端或调用方无法稳定归属流式输出。
- `c409cf1` / `pkg/tools/exec.go` 在请求 `streaming=true` 但没有 handler 时静默降级为 buffered 输出，用户感知与请求不一致。
- `580741d` / `pkg/watch/watcher.go` 已修掉最粗粒度锁，但路径匹配仍使用独占锁，属于不必要的高频串行点。

### Findings Not Fully Fixed This Round
- `undo` 仍未达到“完整可用”状态：
  - `RegisterUndoTool()` 目前没有实际调用点，工具未确认挂载到真实会话入口。
  - `UndoTool.Execute()` 仍不会把已撤销消息和 summary 写回 live session。
  - 这属于架构/调用链问题，不宜在本轮与已有未提交会话相关改动混合硬接。

## Candidate Fixes
- 已实施：
  - 将 `ConfigPage` section label/description 改为单一 `SECTION_META` 源，并补齐 5 个新增分组的 3 语种文案。
  - 修复 `snapshot` 增量 delta 计算，改为基于上一快照的完整重建结果计算新增消息。
  - 在 `Undo()` 后重写 snapshots JSONL 文件，确保撤销可持久化。
  - 为流式更新透传 `session_id`。
  - 为 exec 的 streaming 无 handler 情况增加显式 fallback 提示。
  - watcher 的只读状态和路径匹配改用 `RLock`。

## Verification

- `npm --prefix pkg/webui/frontend run build`
- `go test -count=1 ./pkg/session`
- `go test -count=1 ./pkg/tools ./pkg/watch`
