# Task Plan: Close Remaining Web Bootstrap and Product Gaps

## Goal
Break the remaining non-migrated product gaps into executable slices and implement the highest-priority closure work until the Web bootstrap surface is substantially more complete.

## Phases
- [x] Phase 1: Re-audit remaining closure gaps
- [x] Phase 2: Define executable closure slices
- [x] Phase 3: Implement Web bootstrap config coverage
- [x] Phase 4: Re-assess remaining product closure gaps
- [ ] Phase 5: Verify and deliver

## Key Questions
1. Which missing capabilities are concrete implementation gaps versus broader product/process gaps?
2. Which startup config sections can be safely moved under Web management without destabilizing runtime behavior?
3. After the first closure slice lands, what still prevents a true end-to-end Web-managed loop?

## Decisions Made
- Use a new `closure_*` planning set because the original migration plan is already complete.
- Prioritize Web bootstrap config coverage before broader UX closure work because it directly addresses the highest-confidence gap.
- Treat `storage`, `redis`, `state`, and `bus` as the first Web-managed startup sections to add.
- Persist `storage` to bootstrap `config.json`, while `redis`, `state`, and `bus` remain runtime-db sections.
- Preserve explicit `storage.db_dir` values when loading a config file; only workspace should stay colocated with the config path by default.
- Do not allow first-run Web setup to change `storage.db_dir` yet, because admin credentials are persisted through the already-open runtime DB handle and would otherwise be written to the wrong database before restart.
- Expand the first-run Web flow with safe bootstrap sections (`logger`, `gateway`, `webui`) and explicit restart guidance instead of pretending storage migration is already closed-loop.

## Errors Encountered

## Status
**Currently in Phase 5** - First-run Web setup now covers admin creation plus safe bootstrap settings; running full verification and preparing delivery with remaining gaps called out.
