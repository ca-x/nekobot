# Notes: Legacy Feature Migration Research

## Sources

### Source 1: nekobot README and docs
- Path: `README.md`
- Key points:
  - Current repo already claims multi-path skills, snapshots, QMD integration, workspace bootstrap, provider failover, Web-first runtime, and multi-channel support.
  - `docs/GOCLAW_FEATURES.md` exists but is stale relative to current implementation.

### Source 2: nekobot skill/provider/workspace/memory code
- Paths:
  - `pkg/skills/loader.go`
  - `pkg/skills/snapshot.go`
  - `pkg/skills/version.go`
  - `pkg/providers/rotation.go`
  - `pkg/providers/failover.go`
  - `pkg/providers/loadbalancer.go`
  - `pkg/workspace/manager.go`
  - `cmd/nekobot/onboard.go`
  - `pkg/memory/qmd/*.go`
- Key points:
  - `goclaw`-derived items listed in `docs/GOCLAW_FEATURES.md` are mostly already implemented.
  - Workspace templates, daily logs, onboard command, QMD lifecycle, skill multi-path loading, snapshots, versioning, and provider rotation/failover all exist in `nekobot`.

### Source 3: gua architecture and channel/agent code
- Paths:
  - `/home/czyt/code/gua/README.md`
  - `/home/czyt/code/gua/agent/claude/*.go`
  - `/home/czyt/code/gua/channel/wechat/*.go`
- Key points:
  - `gua` provides per-user runtime sessions, `/yes` `/no` `/cancel` `/select N` interaction model, WeChat presenter formatting, `/share`, account/session management, TUI menu handling, and permission / elicitation routing for Claude Code.
  - `gua` routes permission and elicitation hooks back to the user instead of auto-allowing them.

### Source 4: nekobot WeChat runtime control
- Paths:
  - `pkg/channels/wechat/channel.go`
  - `pkg/channels/wechat/runtime.go`
  - `pkg/channels/wechat/control.go`
  - `pkg/channels/wechat/interaction.go`
  - `pkg/channels/wechat/store.go`
- Key points:
  - `nekobot` already has WeChat runtime creation, binding, ACP session mapping, `/list`, `/bindings`, `/use`, `/new`, `/status`, `/logs`, `/restart`, `/stop`, `/delete`, and conversation-to-session persistence.
  - ACP runtime command normalization already covers `claude-agent-acp`, `codex-acp`, `agent acp`, `gemini --acp`, `opencode acp`.
  - Existing WeChat interaction handling is currently limited to skill-install confirmation.
  - ACP permission requests are auto-approved in `pkg/channels/wechat/control.go` instead of being exposed to users.
  - ACP logs are not buffered yet; `GetRuntimeLogs` returns a placeholder for ACP runtimes.
  - WeChat credential store is currently single-account oriented: `LoadCredentials` returns the first stored account.

## Synthesized Findings

### Already Migrated from goclaw
- Advanced skill loading and priority resolution are implemented.
- Skill snapshots and skill version history are implemented.
- Provider failover, rotation strategies, and cooldown tracking are implemented.
- Workspace bootstrap templates, daily logs, and onboarding are implemented.
- QMD collection management, updater, session export, and WebUI support are implemented.

### Likely Remaining goclaw Gaps
- `docs/GOCLAW_FEATURES.md` should be refreshed because it overstates missing work and is no longer a reliable backlog.
- No strong evidence yet of a major end-user `goclaw` feature still absent beyond documentation drift and possible multi-account channel parity.

### Likely Remaining gua Gaps
- WeChat runtime approvals are not user-driven yet; ACP `session/request_permission` is auto-allowed.
- Generic pending interactions do not exist for ACP permission or elicitation requests; only skill install confirmation uses `/yes` `/no`.
- ACP runtime output/status loop is partial: logs are not buffered for ACP sessions.
- `/share` and explicit account/session management UX from `gua` are not exposed in `nekobot` WeChat runtime flow.
- Multi-account WeChat account storage from `goclaw`/`gua` parity is not present; current credential loading is single-account.

### Recommended Migration Order
1. Fix the highest-risk behavior gap first: route ACP permission prompts to WeChat users instead of auto-allowing.
2. Reuse the same interaction framework to support ACP elicitation and menu-style selection.
3. Add ACP runtime event/log buffering so `/logs` and delayed reads work consistently.
4. Revisit lower-priority product gaps: `/share`, account management, and multi-account support.
