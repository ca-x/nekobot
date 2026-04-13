# 2026-04-13 LLM Wiki Memory 改造任务

## Goal
基于 `docs/LLM_WIKI_MEMORY_SPEC.md`，把 Nekobot 当前 memory 体系升级为可渐进落地的 LLM Wiki 知识库方案，并形成可执行的实现任务入口。

## Why now
- 当前 memory 体系以向量/RAG 检索为主，知识是碎片化召回，不具备结构化复利。
- 已经有明确 spec：`docs/LLM_WIKI_MEMORY_SPEC.md`。
- 该改造影响 agent memory、workspace 结构、tools 和长期知识沉淀，是后续“可上线能力增强”的系统级任务。

## Scope
### In scope
- 评估当前 `pkg/memory`、`pkg/agent/context`、workspace `memory/` 的现状与切入点
- 制定 LLM Wiki 渐进式落地计划
- 建立实现任务入口与阶段拆分

### Out of scope (for this task artifact)
- 本次不直接实现整套 wiki 子系统
- 不在这一任务里处理旧 memory 数据迁移
- 不立即替换现有 semantic memory；先规划渐进共存路径

## Source spec
- `docs/LLM_WIKI_MEMORY_SPEC.md`

## Proposed delivery phases
- [ ] Phase 1: Brownfield audit of current memory architecture and write paths
- [ ] Phase 2: Define MVP LLM Wiki slice and coexistence strategy with existing memory
- [ ] Phase 3: Design data layout / package boundaries / tool surfaces
- [ ] Phase 4: Break implementation into small executable slices
- [ ] Phase 5: Create final implementation roadmap / task list

## High-level acceptance criteria
- 明确当前 memory 相关代码触点与耦合边界
- 明确 wiki/ index / log / raw sources 的最小可行落地方案
- 明确与现有 `pkg/memory`、workspace `memory/`、QMD、agent context 的共存策略
- 给出分阶段、可执行、低风险的任务拆分
- 形成一个后续可直接执行的计划文档

## Likely touchpoints
- `pkg/memory/**`
- `pkg/agent/context.go`
- `pkg/agent/agent.go`
- `pkg/workspace/**`
- `pkg/tools/**`（新增 wiki_ingest/wiki_query/wiki_lint 时）
- `docs/LLM_WIKI_MEMORY_SPEC.md`

## Risks / open questions
- 当前 `workspace/memory/` 与拟议 `workspace/wiki/` 的职责边界如何划分
- 是否保留 vector memory 作为辅助检索层，以及保留多久
- Wiki 页面生成、更新、冲突标注的触发链路应该挂在哪一层
- tool surface 应该先从只读 query 做起，还是先做 ingest

## Recommended next step
先做 brownfield audit，并把审计结果沉淀到 `notes.md`，再形成一份面向实现的 roadmap。


## Initial brownfield findings
- 当前 memory 实际分成三条线：
  1. `pkg/memory/prompt/*`：面向 prompt/context 的长期记忆存储与组合
  2. `pkg/memory/*`：semantic/vector memory、learnings、embedding/search manager
  3. `pkg/memory/qmd/*`：QMD 集成与 session export / collection 管理
- `pkg/agent/context.go` 当前直接把 workspace `memory/` 视为长期记忆来源，并通过 `promptmemory.ContextComposer` 注入 system prompt。
- `pkg/agent/agent.go` 当前通过 `newMemoryStoreFromConfig(...)` 决定 prompt memory backend（file / kv / db），默认仍落在 `workspace/memory`。
- `pkg/memory/fx.go` 当前 semantic memory manager 的默认落盘是 `workspace/memory/embeddings.json`，说明向量层与 workspace memory 已经耦合。
- workspace 模板 (`pkg/workspace/templates/*`) 广泛假设长期记忆目录是 `workspace/memory/`，因此 LLM Wiki 不能直接硬切替换，必须先共存。

## Proposed MVP coexistence strategy
- Phase 1 不替换现有 `workspace/memory/` 约定。
- 新增 `workspace/wiki/`，作为结构化知识层；现有 `workspace/memory/` 保留为：
  - daily logs
  - prompt memory file backend
  - 兼容历史模板/习惯
- semantic/vector memory 在 MVP 阶段继续作为辅助检索层，不立刻退役。
- agent context 的最小接入策略应是“新增 wiki composer / section”，而不是直接改写现有 prompt memory composer。

## Proposed first executable slices
1. `pkg/memory/wiki/types.go`
   - 定义 Page / PageType / WikiConfig / IndexEntry / LogEntry / LintResult
2. `pkg/memory/wiki/page.go`
   - 支持 wiki page frontmatter parse / render / CRUD
3. `pkg/memory/wiki/index.go` + `log.go`
   - 管理 `wiki/index.md` 与 `wiki/log.md`
4. `pkg/workspace/manager.go`
   - 初始化 `workspace/wiki/` 基础目录与最小引导文件
5. `pkg/tools/wiki_query.go`
   - 先提供只读 query 工具，不急着做 ingest
6. `pkg/agent/context.go`
   - 在不破坏现有 memory context 的前提下，追加只读 wiki section / preview

## Recommended implementation order
- 先做“文件结构 + page/index/log 基础设施”
- 再做“只读 query 工具”
- 再做“agent context 只读接入”
- 最后才进入 ingest / lint / linkgraph / contradiction management
