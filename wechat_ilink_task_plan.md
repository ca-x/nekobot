# Task Plan: WeChat iLink Capability Migration

## Goal
Compare `nekobot`'s current WeChat/iLink SDK and channel implementation with `/home/czyt/code/openilink-hub`, identify high-value capabilities to absorb, then design a safe migration sequence for integrating them into the existing SDK and channel stack.

## Phases
- [x] Phase 1: Establish comparison scope and current code locations
- [x] Phase 2: Audit capability gaps between the two projects
- [x] Phase 3: Propose migration approaches and recommended sequence
- [x] Phase 4: Confirm product split and shared-auth target design
- [x] Phase 5: Write spec and implementation plan files
- [x] Phase 6: Implement approved first batch with TDD
- [x] Phase 7: Run verification and summarize remaining gaps

## Key Questions
1. Which `openilink-hub` capabilities are genuinely reusable in `nekobot`, versus being hub-specific product logic?
2. Should the first migration target SDK-level primitives, channel behaviors, or both?
3. What compatibility boundaries must be preserved for the current WeChat channel?

## Decisions Made
- Treat this as a cross-project capability comparison first, not immediate code copy.
- Prioritize reusable iLink SDK primitives over hub-specific application workflows.
- Do not design for historical-data compatibility; the system is still in the design stage.
- Split WeChat iLink into two product paths:
  - AI chat entry remains in the WeChat channel.
  - User push delivery is a separate path.
- Extract shared scanned iLink auth/binding into a reusable layer used by both chat and push.

## Errors Encountered

## Status
**Completed** - Shared iLink auth extraction first batch is implemented and verified with `go test ./...`.
