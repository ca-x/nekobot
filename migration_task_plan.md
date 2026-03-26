# Task Plan: Migrate Remaining Features from goclaw and gua

## Goal
Identify which features from the legacy `goclaw` and `gua` projects have not yet been migrated into `nekobot`, create a concrete migration task list, and execute the approved migration plan.

## Phases
- [x] Phase 1: Plan and setup
- [x] Phase 2: Research current repo and legacy feature gaps
- [x] Phase 3: Define migration tasks and implementation design
- [x] Phase 4: Execute approved migration tasks
- [ ] Phase 5: Review, verify, and deliver

## Key Questions
1. Which legacy repositories or directories define the authoritative source of remaining features?
2. Which gaps are still meaningful for the current `nekobot` architecture and should be migrated now?
3. What is the safest migration order to minimize regressions?

## Decisions Made
- Use persistent markdown files in the repo root for plan, notes, and deliverable tracking.
- Treat `nekobot` as the target of record and compare it against local `goclaw` and `gua` sources.
- Treat `docs/GOCLAW_FEATURES.md` as stale planning history rather than current backlog truth.
- Prioritize remaining `gua` interaction parity over already-implemented `goclaw` subsystems.
- Use dedicated `migration_*` files because the repo already contains unrelated `task_plan.md` and `notes.md`.
- Execute the first migration slice as WeChat ACP permission routing before larger `gua` parity items such as `/share` and multi-account support.

## Errors Encountered
- `codeagent-wrapper` was not found in `PATH`; use built-in agent orchestration instead.
- The repository already had tracked `task_plan.md` and `notes.md`; restore them and move this task to `migration_task_plan.md` and `migration_notes.md`.

## Status
**Currently in Phase 5** - Verifying the implemented WeChat ACP permission-routing slice and preparing delivery notes.
