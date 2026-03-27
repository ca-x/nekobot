# Closure Tasks

## Scope
Convert the remaining high-confidence product closure gaps into discrete implementation slices, then execute them in priority order.

## Task List
- [x] Add Web config support for `storage`.
- [x] Add Web config support for `redis`.
- [x] Add Web config support for `state`.
- [x] Add Web config support for `bus`.
- [x] Re-evaluate first-run/bootstrap closure gaps after config coverage expands.
- [x] Add first-run Web bootstrap summary endpoint data.
- [x] Allow first-run Web setup to save safe bootstrap sections (`logger`, `gateway`, `webui`).
- [x] Show restart-required guidance in the first-run Web flow.
- [x] Make Config/import flows persist bootstrap sections (`storage`, `logger`, `gateway`, `webui`) consistently.
- [x] Show restart-required guidance after Config/import changes that only take effect on restart.
- [x] Revisit storage-path migration so `storage.db_dir` changes no longer strand runtime/auth data in the old DB.
- [x] Add explicit restart/rebind UX for a fully closed-loop bootstrap story.

## Execution Log
- Closure audit started from the post-migration state.
- Selected first closure slice: extend Web config coverage to `storage`, `redis`, `state`, and `bus`.
- Implemented first closure slice:
  - `handleGetConfig` / `handleSaveConfig` / import/export now expose `storage`, `redis`, `state`, and `bus`.
  - Config page now shows those sections in the normal Web config workflow.
  - Runtime config persistence now includes `redis`, `state`, and `bus`.
  - Bootstrap config persistence now updates `storage`.
  - Explicit config-file loads no longer overwrite a user-managed `storage.db_dir`.
- Verified with `go test -count=1 ./pkg/config ./pkg/webui`.
- Verified with `npm --prefix pkg/webui/frontend run build`.
- Selected second closure slice: make first-run setup more Web-led without introducing storage migration bugs.
- Implemented second closure slice:
  - `GET /api/auth/init-status` now returns bootstrap summary data for first-run setup.
  - `POST /api/auth/init` now accepts and persists safe bootstrap sections (`logger`, `gateway`, `webui`) before creating the admin account.
  - Init response now reports `restart_required` and the affected bootstrap sections.
  - `InitPage` now displays bootstrap path/runtime-location summary, editable safe startup settings, and restart guidance.
  - Storage/database/workspace paths remain read-only during first-run because changing them still requires an explicit migration/restart path.
- Implemented third closure slice:
  - `PUT /api/config` and `POST /api/config/import` now save bootstrap-backed sections (`storage`, `logger`, `gateway`, `webui`) back to `config.json`.
  - Config save/import responses now report `restart_required` and `restart_sections`.
  - Frontend save/import toasts now surface restart guidance when startup-setting changes are persisted.
- Implemented fourth closure slice:
  - Changing `storage.db_dir` from Config/import now copies the unified runtime DB to the new directory before bootstrap config is rewritten.
  - This keeps admin auth and runtime-managed config sections aligned with the new storage target after restart, instead of leaving them behind in the previous DB directory.
- Implemented fifth closure slice:
  - Added reusable gateway service inspection/restart helpers in `pkg/servicecontrol`.
  - Exposed `GET /api/service` and `POST /api/service/restart` in WebUI.
  - System page now shows service install/run state, config path, arguments, and a restart action for startup-setting changes.
