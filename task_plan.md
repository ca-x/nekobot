# Task Plan: Nekobot Feature Gap Implementation

## Goal
Implement all missing features from goclaw/picoclaw into nekobot, fix config issues, add WebUI dashboard, and deliver a complete feature-parity bot framework.

## Phases

### Phase 1: Config Fixes (High Priority, Low Complexity) — Claude
- [x] 1.1 Deduplicate Redis config - create shared `RedisConfig` struct
- [x] 1.2 Add per-provider proxy settings to `ProviderProfile`
- [x] 1.3 Update `DefaultConfig()` to reflect new structure
- [x] 1.4 Update all redis references in state/bus modules

### Phase 2: Missing Tools (High Priority, Low Complexity) — Claude
- [x] 2.1 Implement `edit_file` tool (string replacement + line-based editing)
- [x] 2.2 Implement `append_file` tool
- [x] 2.3 Register new tools in `pkg/agent/agent.go`
- [x] 2.4 Update TOOLS.md template (remove "coming soon")

### Phase 3: Voice Transcription (Medium Priority) — Codex
- [x] 3.1 Add Groq Whisper integration (`pkg/transcription/`)
- [x] 3.2 Integrate into Telegram channel (voice messages)
- [x] 3.3 Integrate into Discord channel
- [x] 3.4 Integrate into Slack channel
- [x] 3.5 Add transcription config section

### Phase 4: Microsoft Teams Channel (Medium Priority) — Codex
- [x] 4.1 Create `pkg/channels/teams/` implementation
- [x] 4.2 Add TeamsConfig to config.go and ChannelsConfig
- [x] 4.3 Implement Bot Framework message handling
- [x] 4.4 Register in channel manager

### Phase 5: Docker Sandbox (Medium Priority) — Codex
- [x] 5.1 Add Docker sandbox config to exec tool
- [x] 5.2 Implement container-based execution in `pkg/tools/exec.go`
- [x] 5.3 Network isolation, mounts, auto-cleanup
- [x] 5.4 Fallback to direct execution

### Phase 6: Approvals System (Medium Priority) — Claude
- [x] 6.1 Create `pkg/approval/` with approval manager
- [x] 6.2 Add approval config (auto/manual/prompt modes, tool allowlist)
- [x] 6.3 Integrate into tool execution pipeline
- [x] 6.4 Add CLI commands + WebUI REST endpoints

### Phase 7: WebSocket Gateway (Medium Priority) — Claude
- [x] 7.1 Add gorilla/websocket dependency
- [x] 7.2 Implement WS handler in gateway
- [x] 7.3 Connection pool, auth, keepalive
- [x] 7.4 REST API endpoints

### Phase 8: WebUI Dashboard (High Priority, High Complexity) — Claude + Codex
- [x] 8.1 Add Echo v4 dependency, create `pkg/webui/` module
- [x] 8.2 Security: JWT auth, first-run password init, session management
- [x] 8.3 API routes: provider CRUD, channel CRUD, config save/sync
- [x] 8.4 Chat Playground: WebSocket-based chat with agent integration
- [x] 8.5 Channel testing: check channel status and reachability
- [x] 8.6 Frontend: embedded SPA (use /ui-skills for design)
- [x] 8.7 Auto-start WebUI when gateway runs in daemon mode
- [x] 8.8 Add WebUI config (port, auth settings) to Config

### Phase 9: Extended Thinking (Low Priority) — Claude
- [x] 9.1 Add thinking fields to provider config
- [x] 9.2 Handle thinking blocks in Claude responses
- [x] 9.3 Thinking budget configuration

### Phase 10: TUI & Infoflow (Medium Priority)
- [x] 10.1 Simple TUI with bubbletea (minimal, preserve current functionality)
- [x] 10.2 Infoflow (如流) channel implementation

## Team Assignment

| Agent | Phases | Focus |
|-------|--------|-------|
| **Claude** | ~~1~~, ~~2~~, 6, 7, 8.1-8.3, 8.7-8.8, 9 | Config, tools, approval, WS gateway, WebUI backend |
| **Codex** | 3, 4, 5, 8.4-8.6, 10 | Voice, Teams, Docker sandbox, WebUI frontend, TUI, Infoflow |

## Key Decisions
- Redis config: Single shared `RedisConfig`, State/Bus only specify prefix
- Provider proxy: `Proxy string` field on `ProviderProfile`
- edit_file: old_string/new_string replacement (like goclaw) + line_start/line_end editing
- WebUI: Echo v5 backend, embedded SPA frontend, JWT auth
- WebUI auto-starts on daemon mode with configurable port (default: gateway port + 1)

## Status
**Phase 1-10 COMPLETE** — All planned features implemented; `go build` + `go vet` pass.

### What's Done (Claude)
- Config: shared RedisConfig, provider proxy, approval config, WebUI config
- Tools: edit_file + append_file implemented and registered
- Proxy: all 4 adaptors (openai, claude, gemini, generic) now use proxy from config
- Approval: full manager with auto/prompt/manual modes, CLI commands, WebUI REST endpoints
- WebSocket Gateway: gorilla/websocket, JWT auth, connection pool, ping/pong keepalive, REST endpoints
- WebUI: Echo v5 server with JWT auth, init flow, provider/channel/config CRUD APIs, embedded SPA frontend
- Extended Thinking: thinking config in AgentDefaults, thinking block handling in Claude converter, budget control

### What's Remaining
- Phase 8.4-8.6 follow-up polish (UX/visual improvements) as needed

### Notes
- Telegram approval interaction: In prompt mode, approval requests can be sent as inline keyboard messages in Telegram, and user taps approve/deny button
- Discord: Can use message components (buttons) for similar approval UX
- Frontend pages should be embedded into binary via Go embed (already set up)
- User prefers /ui-skills for frontend design
