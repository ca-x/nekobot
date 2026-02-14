# Notes: Feature Gap Analysis - goclaw/picoclaw vs nekobot

## Analysis Summary

### Already in nekobot (no action needed)
- 11 messaging channels (Telegram, Discord, Slack, WhatsApp, Feishu, DingTalk, QQ, WeWork, ServerChan, Google Chat, MaixCAM)
- 12+ AI providers with failover/circuit breaker
- File tools (read_file, write_file, list_dir)
- Shell exec with basic dangerous command blocking
- Web search (Brave) & web_fetch
- Browser automation (CDP - browser_session)
- Memory system (QMD vector search)
- 22 builtin skills (including summarize)
- Subagent/spawn system
- Cron + Heartbeat scheduling
- Auth CLI (login/logout/status) - fully implemented
- Session management (JSONL)
- Gateway service (install/start/stop)
- Slash commands system

### Missing Features Detail

#### 1. edit_file / append_file tools
- **Location**: `pkg/tools/` - need new files
- **Reference**: TOOLS.md says "coming soon" at line 15-16
- **goclaw approach**: String replacement based (old_string -> new_string)
- **picoclaw approach**: Line-based editing (edit specific line ranges)
- **Recommendation**: Support both modes - string replacement (primary) + line-range editing

#### 2. Provider proxy settings
- **Location**: `pkg/config/config.go` - ProviderConfig struct
- **Current state**: No proxy field exists anywhere in provider config
- **goclaw**: Per-provider proxy URL
- **picoclaw**: Per-provider proxy URL
- **Fix**: Add `Proxy string` to provider-level config

#### 3. Redis config dedup
- **Location**: `pkg/config/config.go:161-179`
- **StateConfig** has: RedisAddr, RedisPassword, RedisDB, RedisPrefix
- **BusConfig** has: RedisAddr, RedisPassword, RedisDB, RedisPrefix (duplicated!)
- **Fix**: Create shared `RedisConfig` struct, reference from both with prefix override

#### 4. Voice transcription (Groq Whisper)
- **picoclaw implementation**: Downloads voice message -> sends to Groq Whisper API -> returns text
- **Channels**: Telegram (.ogg), Discord (audio), Slack (audio)
- **API**: `https://api.groq.com/openai/v1/audio/transcriptions`
- **Model**: `whisper-large-v3-turbo`
- **Config needed**: groq api_key (can reuse existing groq provider key)

#### 5. Microsoft Teams channel
- **goclaw implementation**: Bot Framework SDK, webhook-based
- **Auth**: App ID + Password (Azure AD registration)
- **API**: Microsoft Bot Framework REST API
- **Key endpoints**: `/api/messages` for webhook, activity-based messaging

#### 6. Docker Sandbox
- **goclaw implementation**: Uses Docker SDK to create containers for exec
- **Features**: Network isolation, mount workspace, auto-cleanup, custom image
- **Config**: enabled, image, network_mode, mounts, timeout
- **Library**: `github.com/docker/docker/client`

#### 7. Approvals System
- **goclaw implementation**: Three modes - auto (allow all), manual (queue for approval), prompt (ask in chat)
- **Allowlist**: Specific tools can bypass approval
- **Flow**: Tool call -> check approval config -> auto-execute or queue -> wait for approval -> execute

#### 8. WebSocket Gateway
- **goclaw implementation**: gorilla/websocket server, connection pool, auth token, ping/pong
- **Default port**: 28789
- **Features**: TLS, max message size, concurrent connections

#### 9. Extended Thinking
- **goclaw implementation**: Detect thinking-capable models, add thinking parameters to API call
- **Claude specific**: `thinking` parameter with budget_tokens
- **Config**: Per-model thinking model selection, timeout

## Config Structure Reference

### Current nekobot Redis duplication (`pkg/config/config.go`)
```go
// StateConfig (line 161-168)
type StateConfig struct {
    Backend       string // "file" or "redis"
    Path          string
    RedisAddr     string
    RedisPassword string
    RedisDB       int
    RedisPrefix   string
}

// BusConfig (line 173-179)
type BusConfig struct {
    Type          string // "local" or "redis"
    RedisAddr     string  // DUPLICATED
    RedisPassword string  // DUPLICATED
    RedisDB       int     // DUPLICATED
    RedisPrefix   string  // DUPLICATED (different default)
}
```

### Proposed fix
```go
type RedisConfig struct {
    Addr     string `mapstructure:"addr" json:"addr"`
    Password string `mapstructure:"password" json:"password"`
    DB       int    `mapstructure:"db" json:"db"`
}

type StateConfig struct {
    Backend string // "file" or "redis"
    Path    string
    Prefix  string // default "nekobot:"
}

type BusConfig struct {
    Type   string // "local" or "redis"
    Prefix string // default "nekobot:bus:"
}

type Config struct {
    // ... existing fields
    Redis RedisConfig // shared, configured once
    State StateConfig
    Bus   BusConfig
}
```
