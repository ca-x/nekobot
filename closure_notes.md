# Notes: Remaining Closure Gaps

## Sources

### Source 1: Current migration summary
- Paths:
  - `migration_task_plan.md`
  - `migration_tasks.md`
  - `migration_notes.md`
- Key points:
  - `gua`-style WeChat/runtime migration is largely complete.
  - Remaining gaps are now broader product closure issues, not obvious uncopied legacy commands.

### Source 2: Web config API and frontend
- Paths:
  - `pkg/webui/server.go`
  - `pkg/webui/frontend/src/pages/ConfigPage.tsx`
  - `pkg/webui/frontend/src/hooks/useConfig.ts`
- Key points:
  - Web config currently exposes `agents`, `gateway`, `tools`, `transcription`, `memory`, `sessions`, `heartbeat`, `approval`, `logger`, and `webui`.
  - Web config does not yet expose `storage`, `redis`, `state`, or `bus`.

### Source 3: Runtime config persistence
- Paths:
  - `pkg/config/config.go`
  - `pkg/config/db_store.go`
- Key points:
  - Runtime config already has typed sections for `Storage`, `Redis`, `State`, and `Bus`.
  - SQLite-backed runtime section persistence currently excludes those sections.

## Synthesized Findings

### Executable Closure Slice 1
- Add `storage`, `redis`, `state`, and `bus` to Web config API responses and saves.
- Persist those sections in runtime config storage.
- Expose those sections in the Config page.

### Slice 1 Outcome
- `storage`, `redis`, `state`, and `bus` are now Web-visible.
- `storage` is persisted to bootstrap config because it determines where runtime config storage lives.
- `redis`, `state`, and `bus` are persisted as runtime sections in SQLite.
- Explicit config-file loading now preserves custom `storage.db_dir` values instead of forcing them back to the config directory.

### Remaining Closure Work After Slice 1
- Revisit auth/bootstrap flow so first-run and startup concerns are more fully Web-led.
- Re-evaluate whether runtime/database location changes need explicit restart or migration UX.
- Review broader “closed loop” expectations around setup, restart semantics, and recovery workflows.

### Slice 2 Outcome
- First-run Web setup now fetches current bootstrap context from `init-status`, including config path, DB dir, workspace, logger, gateway, and WebUI settings.
- First-run initialization can now save `logger`, `gateway`, and `webui` bootstrap settings directly from Web before the admin account is created.
- The init response explicitly reports when a restart is required and which startup sections were changed.
- The Web flow now makes the current boundary visible: storage/database/workspace location changes are not yet editable at first run because current runtime handles would still point at the pre-existing DB until restart.

### Remaining Closure Work After Slice 2
- Add an explicit storage-path migration/rebind flow so `storage.db_dir` can be safely changed from Web without splitting bootstrap state and auth state across different runtime databases.
- Clarify restart/reload UX after startup-setting changes made from Config or Init pages.
- Re-evaluate whether broader operational recovery paths (fresh bootstrap, DB move, restore, restart health) now satisfy the project’s “closed loop” bar.
