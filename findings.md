# Findings

## 2026-02-15 Provider DB + Dashboard UX

- WebUI provider CRUD currently reads/writes `s.config.Providers` directly in `pkg/webui/server.go` and persists to config file via `persistConfig()`.
- Frontend provider flow is two-step (dialog `Apply` only updates JSON draft; separate `Save` persists), which is easy to误操作 and likely causes "added but not actually saved" confusion.
- Backend create/update/delete provider handlers do not validate duplicates/empty names robustly and log persistence failures without surfacing errors to frontend.
- Agent/provider resolution still relies on `cfg.Providers`, so database-backed provider storage must keep `cfg.Providers` synchronized at runtime.
- User confirmed requirement: provider storage must reuse a single existing database file; implementation now reuses `tool_sessions.db` instead of creating `providers.db`.
- `ent` code was centralized under `pkg/storage/ent` (moved from `pkg/toolsessions/ent`) and all imports were updated to this unified location.
- Added `pkg/config/db_store.go`: runtime sections (agents/channels/gateway/tools/heartbeat/approval/logger/webui) now load/save via `tool_sessions.db` table `config_sections`.
- `ProvideConfig` now applies DB overrides during startup, then runs validation.
- WebUI config/channel/auth/provider-routing persistence switched from `config.json` write-back to DB section writes.
- Updated both config example files to a minimal bootstrap-oriented shape and updated `docs/CONFIG.md` with DB runtime persistence notes.
- Dashboard Config tab interaction was reworked from full-JSON blob editing to section-based editing (agents/gateway/tools/heartbeat/approval/logger/webui), reducing accidental overwrite risk.
- Outdated provider-in-config documentation/examples were removed; bootstrap config now emphasizes logger/gateway/webui only.
- Runtime config section writes now include `webui`, and `/api/config` now returns `webui` for consistent section editing.
