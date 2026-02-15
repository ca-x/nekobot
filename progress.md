# Progress Log

## 2026-02-15

- Initialized planning artifacts for provider DB migration task.
- Inspected current provider backend/frontend implementation paths.
- Confirmed provider CRUD currently depends on config file persistence and draft-based frontend flow.
- Next: implement DB-backed provider store and wire into WebUI handlers.
- Implemented `pkg/providerstore` with SQLite provider CRUD and runtime config sync.
- Wired provider store into WebUI APIs and gateway/CLI startup module graphs.
- Updated dashboard provider dialog: clicking dialog Apply now directly persists provider changes.
- Adjusted storage to reuse the existing single DB file `tool_sessions.db` per user request.
- Verification: `go test ./...` passed.
- Refactored Ent location to a single shared path: moved generated code from `pkg/toolsessions/ent` to `pkg/storage/ent` and updated all imports.
- Re-ran verification after Ent refactor: `go test ./...` passed.
- Implemented runtime config DB store (`config_sections`) on shared `tool_sessions.db` and startup overlay logic.
- Migrated WebUI save paths for init password, channel updates, global config save, and chat routing persistence from file writes to DB writes.
- Added config DB store tests and verified with `go test ./...`.
- Updated `config/config.example.json`, `pkg/config/config.example.json`, and `docs/CONFIG.md` to match latest minimal-file + DB-runtime behavior.
- Replanned config UX: switched dashboard Config from whole-document JSON editing to section-scoped editing with section selector/reset/save.
- Removed outdated provider-in-config snippets from README/docs and aligned examples to bootstrap-only config.
- Added WebUI config section into `/api/config` read/write path and persisted via DB section storage.
- Validation run: `go test ./...` passed.
