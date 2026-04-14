# Hermes Absorption Notes

## Scope
Compare Hermes Agent with Nekobot, focused on:
- memory system evolution
- auto skill extraction/discovery
- permission approval and approve-then-retry behavior
- additional worthwhile adoption points

## Synthesized Findings

### Nekobot already strong in
- semantic memory, prompt memory, learnings, and workspace wiki
- multi-source skill loading, validation, watcher, installer, snapshots, and version history
- approval manager plus external-agent approval continuation in gateway/webui/wechat

### Hermes is stronger in
- self-evolving memory lifecycle (`MemoryProvider`, prefetch/sync/on_pre_compress hooks)
- agent-managed skill CRUD/extraction (`skill_manage`)
- broader approval resume behavior and retry ergonomics
- security-oriented skill/prompt scanning and richer skills-hub lifecycle

### Recommended first adoption wave
1. Skill management tool for Nekobot
2. Fenced prompt-memory block
3. Generic approval retry/resume primitive

### Deferred / later waves
- full memory-provider abstraction
- async memory prefetch/sync hooks per turn
- full skills hub with quarantine/index-cache/provenance
- broader prompt/context injection scanning

## Additional requested references
- `../gentle-ai` for agent management and installation patterns
- `../llm-wiki` for wiki implementation comparison against `pkg/memory/wiki`

## gentle-ai conclusions
- Best future Nekobot imports:
  - external-agent adapter abstraction
  - installed-agent discovery
  - install-command resolver / onboarding hints
  - backup-before-mutate for external agent config writes
- These are valuable, but better treated as a follow-up wave than mixed into the current narrow slice.

## llm-wiki conclusions
- Best future Nekobot imports:
  - richer wiki structure/schema conventions
  - derived index / stale-check / lint protocol ideas
  - stronger wiki query behavior patterns
- `llm-wiki` is primarily a docs/protocol architecture, so it should influence Nekobot's wiki rules and tools rather than be ported literally.
