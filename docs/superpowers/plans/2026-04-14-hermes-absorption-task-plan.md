# Task Plan: Hermes feature absorption into Nekobot

## Goal
Compare ~/code/hermes-agent against nekobot, identify the highest-value capabilities to absorb, implement the best fitting slices in an isolated worktree, verify them, then merge and push to main.

## Phases
- [x] Phase 1: Plan and setup
- [x] Phase 2: Research Hermes vs Nekobot capabilities
- [x] Phase 3: Select adoption slices and design implementation
- [x] Phase 4: Implement selected slices
- [ ] Phase 5: Verify, merge, and push

## Decisions Made
- Use isolated worktree branch `hermes-absorb`.
- First implementation wave: workspace `skill_manage`, fenced prompt-memory rendering, generic approval replay cleanup/resume seam.
- Keep diffs narrow; do not overwrite repo-global planning/history files.

## Verification Status
- Targeted Go tests for new/changed slices are green.
- Frontend build currently blocked by missing local `node_modules` / `tsc` in this worktree; environment fix pending.


## Scope expansion
- Additional requested comparison: `/home/czyt/code/hermes-agent/gateway/platforms`
- New objective: absorb worthwhile platform/channel capability improvements into Nekobot where the fit is clear and the slice stays reviewable.

## Updated implementation wave
- gentle-ai-inspired external-agent adapter/discovery/install-hint
- llm-wiki-inspired wiki schema/query improvements
- hermes gateway/platforms-inspired platform capability improvements


## Design constraint update
- Nekobot has no historical data burden for this wave. Prefer cleaner model/protocol design over compatibility scaffolding, as long as frontend/backend remain usable.


## Skills alignment wave

### Goal
Bring Nekobot skills closer to Hermes in the highest-value areas that remain incomplete: progressive disclosure and fine-grained skill management.

### Target slices
1. Add `patch` support to `skill_manage`
2. Add `write_file` / `remove_file` support to `skill_manage`
3. Improve `skill` tool progressive disclosure so list stays compact and get/view remains the detail path

### Verification
- targeted Go tests for `pkg/skills`, `pkg/tools`
- frontend build remains regression gate only; no intended frontend source changes in this wave
