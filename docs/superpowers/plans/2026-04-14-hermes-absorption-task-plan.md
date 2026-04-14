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
