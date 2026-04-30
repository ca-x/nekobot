# Hermes-style Memory and Curator Redesign

Status: design proposal for task #44
Date: 2026-04-30
Owner: @柯蒂斯

Related:
- `pkg/agent/context.go`
- `pkg/memory/prompt/context.go`
- `pkg/memory/prompt/store.go`
- `pkg/tools/memory.go`
- `pkg/session/manager.go`
- `pkg/session/jsonl.go`
- `pkg/skills/manager.go`
- `pkg/skills/loader.go`
- `docs/superpowers/specs/2026-04-30-slock-message-board-attachment-api-design.md`

## Goal

Adopt the useful parts of Hermes Agent's memory design without copying its implementation blindly:

- Keep the system prompt stable and cache-friendly.
- Keep always-injected memory tiny, curated, and high-signal.
- Move historical recall to explicit tools/search.
- Treat skills as procedural memory, but prevent self-generated skills from becoming an unmanaged junk drawer.
- Remove prompt-facing memory surfaces that encourage logs, TODOs, and task progress to leak into every turn.

This is not part of the daemon/gateway protocol critical path. It is an agent runtime architecture slice that should follow the current protocol hardening work.

## Current Nekobot State

### Prompt Assembly

`pkg/agent/context.go` builds prompt sections in this order:

1. Identity/runtime/tools section.
2. Bootstrap files from `SOUL.md`, `USER.md`, `IDENTITY.md`.
3. Skills section.
4. Memory section.

The section model already marks identity/bootstrap as stable and skills/memory as dynamic. That is a good foundation, but the current identity text still tells the model:

- `Memory: <workspace>/memory/MEMORY.md`
- `Daily Notes: <workspace>/memory/YYYYMM/YYYYMMDD.md`
- important information should be saved to `memory/MEMORY.md` using `write_file`

That instruction pushes the model toward direct file edits and log-like memory, which is the opposite of the Hermes hot-memory model.

### Prompt-facing Memory

`pkg/memory/prompt/store.go` and `pkg/memory/prompt/context.go` currently inject a combined block from:

- workspace `MEMORY.md`
- backend long-term memory
- `memory/active_learnings.md`
- recent daily notes, defaulting to today

Default max size is `8000` chars. This is bigger and less stable than Hermes-style hot memory. Daily notes are especially risky because they turn task progress into prompt state and invalidate prompt caching frequently.

### Search Memory

`pkg/tools/memory.go` exposes a semantic memory tool with `search`, `add`, and `status`. It is useful, but it is not the same as Hermes `session_search`:

- It searches embedding/QMD memory entries, not necessarily raw session history.
- It stores arbitrary content through `add`, with no curated hot-memory policy.
- It does not support `replace` or `remove` by substring for curated prompt memories.
- It does not enforce the “no task progress, no TODOs, no session result logs” boundary.

`pkg/session/manager.go` and `pkg/session/jsonl.go` already persist sessions as JSONL, which is enough to implement a session search layer without adding a vector DB first.

### Skills as Procedural Memory

`pkg/skills/manager.go` is already close to Hermes:

- Always-on skills are injected inline.
- Regular skills are rendered as an index using `formatSkillSummaryXML`.
- Detailed instructions are loaded on demand by skill tooling.
- Built-in skills are embedded and protected from in-place modification.
- Version history and snapshots already exist.

The missing piece is lifecycle hygiene: usage stats, stale/overlapping generated skill detection, dry-run cleanup plans, and explicit protection for builtin/external/pinned skills.

## Target Architecture

### Layer 1: Hot Prompt Memory

Create a small, curated prompt memory layer with two explicit files:

- `memory/MEMORY.md`: agent/project/environment facts, recurring corrections, stable operating rules.
- `memory/USER.md`: user profile, communication preferences, stable user context.

Recommended limits:

- `MEMORY.md`: 2200 characters.
- `USER.md`: 1400 characters.
- Hard cap by Unicode rune count or byte count; character count is acceptable and model-independent.

Format:

```text
entry one
§
entry two
§
entry three
```

Rules:

- Inject this layer into prompt, but keep it tiny.
- Snapshot once per session/route invocation; writes during a turn should persist but not rebuild the already-sent prompt.
- Do not store task progress, temporary TODOs, implementation status, meeting notes, or one-off results.
- Prefer stable facts, user preferences, environment constraints, recurring mistakes, and durable project conventions.

Replace the current default prompt memory composition:

- Disable recent daily notes by default.
- Stop injecting broad long-term memory by default.
- Stop telling the model to edit memory files directly with `write_file`.
- Keep `active_learnings.md` only if compressed under the hot-memory cap or moved behind an explicit recall tool.

### Layer 2: Session Search

Add a dedicated `session_search` tool for episodic recall.

First implementation can use existing JSONL sessions:

1. Scan indexed session metadata and messages.
2. Perform keyword/BM25-like search first; QMD/embedding integration can follow later.
3. Group results by session.
4. Return concise snippets and links/session IDs.
5. Optional later: summarize top sessions with a cheap model.

This layer answers “what did we discuss before?” without placing all history into every prompt.

Acceptance for first slice:

- Search across persisted sessions by query.
- Return session ID, timestamp, role, excerpt, and match reason.
- Do not mutate prompt memory.
- Do not require QMD availability.

### Layer 3: Memory Tool for Curated Prompt State

Change the prompt-memory tool surface from generic vector memory to a curated hot-memory editor:

```text
memory(action=add|replace|remove|status, scope=memory|user, text?, old_text?)
```

Behavior:

- `add`: append one curated entry if it passes policy and cap checks.
- `replace`: substring match exactly one existing entry or fail with ambiguity.
- `remove`: substring match exactly one existing entry or fail with ambiguity.
- `status`: show usage percent and remaining characters.

Guards:

- Reject exact duplicates.
- Reject likely secrets/API keys/tokens.
- Reject prompt-injection shaped entries, for example “ignore previous instructions”.
- Reject hidden Unicode controls and suspicious bidi characters.
- Warn/reject task-progress language such as “currently working on”, “TODO”, “done in this session”, unless explicitly saved as a stable convention.

The existing semantic memory tool can remain, but should be renamed or separated from hot prompt memory to avoid conflating “search archive” with “always inject”.

### Layer 4: Compression Memory Flush

Before any automatic context compression, run a memory-flush pass:

- Prompt the model to preserve only durable facts/preferences/corrections/repeated patterns.
- Enable only the curated `memory` tool.
- Disable shell/file/network tools during this pass.
- Do not preserve transient task state.

Current `pkg/agent/compress.go` does lossy message dropping/compression. The flush should run before that path when compaction is critical or explicit.

Acceptance:

- Compression can trigger a tool-only memory flush.
- Flush updates hot memory files but does not expand the current compressed prompt with broad history.
- Tests prove task-progress phrases are rejected by policy.

### Layer 5: Skills as Procedural Memory

Keep the existing direction:

- Always-on skills may be injected inline.
- Regular skills stay as an index: id, name, description, instruction length.
- Full skill content loads only on demand.

Improve the index with source/protection metadata:

- `source`: builtin, executable, global, workspace, local, external, generated.
- `pinned`: true/false.
- `last_used_at`, `use_count`.
- `updated_at`.

Do not inject all skill text into the prompt by default.

## Curator Design

Curator is the lifecycle manager for procedural memory. It should clean up only skills that are safe to mutate.

### Scope

Curator may inspect all skills but may only propose mutations for:

- agent-generated skills.
- user-authored workspace/local skills.

Curator must not mutate by default:

- builtin skills (`builtin://...`).
- externally installed/registry skills unless copied into workspace and explicitly unprotected.
- pinned skills.
- skills with unknown source/provenance.

### Inputs

Curator computes a report from:

- skill source and file path.
- `metadata.generated_by`, `metadata.pinned`, `metadata.provenance`.
- usage count and last-used timestamp.
- last modified timestamp.
- instruction length and supporting files.
- fuzzy overlap between name/description/tags/instructions.
- version history and snapshots.

### Outputs

Default output is a dry-run plan:

```text
CuratorPlan {
  repeated CuratorAction actions
}

CuratorAction {
  action: merge | archive | demote_to_template | demote_to_script | pin_suggest | no_op
  skill_id
  target_skill_id
  rationale
  risk
  protected
}
```

No file edits happen unless a user or owner explicitly applies the plan.

### Action Rules

- Merge: for overlapping skills with similar purpose and compatible requirements.
- Archive: for stale generated/user skills with low usage and no recent updates.
- Demote to template/script: for overly specific skills that are better as a reference artifact under a broader skill.
- Pin suggest: for frequently used generated skills that should be protected.
- No-op: for protected or ambiguous cases.

### Safety

- Always create a snapshot before applying curator actions.
- Archive to `.nekobot/skills-archive/` rather than delete on first implementation.
- Never mutate builtin paths.
- Never mutate external installed paths unless source metadata says workspace-owned.
- Emit an audit/log record for each applied action.

## What To Remove Or De-emphasize

- Stop using daily notes as a default prompt input.
- Stop presenting long-term memory as a large prompt block.
- Stop recommending direct `write_file` edits to prompt memory.
- Do not treat task progress as long-term memory.
- Do not auto-generate many tiny workflow skills without usage tracking and curator hygiene.

## Implementation Slices

### Slice Q: Hot Prompt Memory

Scope:

- Add `memory/MEMORY.md` and `memory/USER.md` curated stores with character caps.
- Add add/replace/remove/status operations.
- Add duplicate/secret/injection/hidden-Unicode/task-progress guards.
- Change prompt composer defaults to inject only hot memory, no daily notes.

Acceptance:

- Prompt memory stays under caps.
- Writes persist but do not inject broad logs.
- Tests cover duplicate, cap, and unsafe content rejection.

### Slice R: Session Search

Scope:

- Implement `session_search` over existing JSONL sessions.
- Return grouped, bounded snippets with session IDs.
- Optionally wire QMD/embedding later behind the same interface.

Acceptance:

- Agent can retrieve previous conversation context on demand.
- Prompt memory is not modified by search.
- Works when QMD is unavailable.

### Slice S: Compression Memory Flush

Scope:

- Add pre-compaction flush pass.
- Restrict tools to curated memory operations.
- Reuse hot-memory policy.

Acceptance:

- Critical compaction can preserve durable facts before loss.
- Task-progress entries are rejected.
- Tests verify flush does not run unrestricted tools.

### Slice T: Skill Curator

Scope:

- Track skill usage and source/protection metadata.
- Implement curator dry-run report.
- Protect builtin/external/pinned skills.
- Archive/merge/demote only behind explicit apply.

Acceptance:

- Weekly dry-run can list stale/overlapping generated skills.
- Protected skills are never modified.
- Apply path snapshots before mutation.

### Slice U: Prompt Cache Boundary Cleanup

Scope:

- Make prompt section stability explicit in runtime diagnostics.
- Keep hot memory and skill index stable within a session where possible.
- Append optional deep user-model recall to user turn rather than rewriting system prompt.

Acceptance:

- Context preview shows stable vs dynamic sections.
- Prompt cache invalidation sources are visible.
- New memory writes do not unexpectedly rewrite the current turn's system prompt.

## Recommended Priority

1. Slice Q: Hot prompt memory and tool policy.
2. Slice R: Session search.
3. Slice T: Curator dry-run, because skill sprawl can become costly quickly.
4. Slice S: Compression flush.
5. Slice U: deeper cache-boundary and optional user-model integration.

This order keeps the prompt smaller first, then restores recall via tools, then cleans procedural memory.
