# Hermes Absorption Plan

Status: implementation

## Comparison summary
Hermes does not simply have “more features”; it has stronger self-evolution loops around memory, skills, and approval recovery. Nekobot already has richer core infrastructure in several areas, so the best absorption path is to add those loops on top of Nekobot's existing systems rather than porting Hermes architecture wholesale.

## Selected Phase-1 slices

### Slice A — Skill management tool
Add an agent-facing tool to create, inspect, patch, and delete workspace skills.

Why first:
- highest leverage for “automatic/extracted skills”
- Nekobot already has validators, loaders, versions, and snapshots to support it
- bounded write scope: `pkg/skills`, `pkg/tools`, agent registration/tests

### Slice B — Fenced memory context block
Wrap prompt-memory content in an explicit fenced block with instructions that it is recalled memory, not current user input.

Why first:
- tiny diff, low risk
- directly inspired by Hermes memory snapshot discipline
- improves prompt clarity and memory-safety posture

### Slice C — Generic approval retry/resume primitive
Persist one pending ordinary tool call and allow control-plane approval handlers to replay it once approved.

Why first:
- closes the current gap where only external-agent flows automatically continue after approval
- smaller and safer than a deep inline blocking approval rewrite
- aligns with the user's approval/retry interest

## Out of scope for this wave
- full memory-provider abstraction
- full async memory prefetch/sync lifecycle
- full hub/quarantine/index-cache system
- generalized security scanner for all skill/context sources

## Verification plan
1. targeted unit tests for skill CRUD and memory block formatting
2. targeted approval replay tests
3. `npm --prefix pkg/webui/frontend run build`
4. focused Go tests for touched packages
5. broader Go package regression if needed


## Additional reference conclusions

### gentle-ai
Use as the model for a future external-agent management wave:
- adapter registry
- installed-agent discovery
- install-command resolution
- optional backup/sync flows later

### llm-wiki
Use as the model for a future wiki protocol wave:
- richer wiki schema/layout rules
- derived index + lint conventions
- stronger query semantics
