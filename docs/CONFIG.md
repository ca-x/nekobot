# NekoBot 配置和加载说明

## 配置文件加载优先级

NekoBot 按以下顺序查找配置文件（找到第一个即使用）：

| 优先级 | 路径 | 说明 |
|--------|------|------|
| 1 (最高) | `~/.nekobot/config.json` | 用户全局配置目录 |
| 2 | `./config.json` | 当前工作目录 |
| 3 | `./config/config.json` | 当前目录的 config 子目录 |

可通过 `--config` 参数指定配置文件路径覆盖默认行为：

```bash
nekobot agent --config /path/to/custom/config.json
```

### 环境变量覆盖

配置项可以通过环境变量覆盖，前缀为 `NEKOBOT_`，使用下划线分隔：

```bash
# 覆盖 providers.anthropic.api_key
export NEKOBOT_PROVIDERS_ANTHROPIC_API_KEY="sk-ant-xxx"

# 覆盖 agents.defaults.model
export NEKOBOT_AGENTS_DEFAULTS_MODEL="claude-3-5-sonnet-20241022"

# 启动应用
nekobot agent
```

---

## Skills 加载顺序

技能按以下顺序加载，**同名技能后面的会覆盖前面的**：

| 顺序 | 路径 | 说明 | 优先级 |
|------|------|------|--------|
| 1 | 内嵌的 builtin skills | 编译进二进制的内置技能 | 0 (最低) |
| 2 | `<可执行文件路径>/skills/` | 可执行文件同级目录 | 10 |
| 3 | `~/.nekobot/skills/` | 用户全局技能目录 | 20 |
| 4 | `${WORKSPACE}/.nekobot/skills/` | 工作区隐藏目录 | 30 |
| 5 | `${WORKSPACE}/skills/` | 工作区技能目录 | 40 |
| 6 | `./skills/` | 当前目录 | 50 (最高) |

默认 workspace 为 `~/.nekobot/workspace`，可在配置文件中修改。

### 技能覆盖示例

如果同一个技能（相同 ID）在多个位置存在：

```
~/.nekobot/skills/weather/SKILL.md  (优先级 20)
./skills/weather/SKILL.md            (优先级 50)
```

最终会使用 `./skills/weather/SKILL.md`，因为它的优先级更高。

---

## 列出可用技能

```bash
# 查看所有已发现的技能
nekobot skills list

# 查看技能详情
nekobot skills show <skill-id>

# 启用/禁用技能
nekobot skills enable <skill-id>
nekobot skills disable <skill-id>
```

---

## 安装技能

### 方式 1：手动复制

将技能文件夹（包含 SKILL.md）放入以下任一位置：

- `./skills/` （当前目录，最高优先级）
- `${WORKSPACE}/skills/` （工作区目录）
- `~/.nekobot/skills/` （用户全局目录）

### 方式 2：从 GitHub 安装

```bash
# 从 GitHub 安装技能
nekobot skills install https://github.com/user/repo/tree/main/skills/weather

# 安装整个技能仓库
nekobot skills install https://github.com/user/nekobot-skills
```

### 方式 3：OpenClaw 兼容

可以直接使用 OpenClaw 生态的技能：

```bash
# 克隆 OpenClaw skills 仓库
git clone https://github.com/openclaw/skills ~/.nekobot/skills/openclaw

# 或者复制单个技能
cp -r ~/openclaw/skills/weather ~/.nekobot/skills/
```

---

## 技能格式 (SKILL.md)

技能文件采用 Markdown 格式，包含 YAML frontmatter：

```markdown
---
name: weather
description: Get weather information for a location
version: 1.0.0
author: Your Name
tags: [weather, api, web]
enabled: true
requirements:
  binaries:
    - curl
  env:
    - WEATHER_API_KEY
metadata:
  goclaw:
    emoji: "🌤️"
    requires:
      anyBins: ["curl", "wget"]
---

# Weather Skill

This skill allows the agent to fetch weather information.

## Usage

To get weather for a location, use:
- "What's the weather in Tokyo?"
- "Check weather for New York"

## Implementation

The agent will use the `exec` tool to call the weather API:

\`\`\`bash
curl "https://api.weather.com/v1/current?location=Tokyo"
\`\`\`
```

---

## 自动准入 (Gating)

NekoBot 会自动检测系统环境，只加载满足要求的技能：

### 检查项

- **操作系统**: `os` (linux, darwin, windows, unix, any)
- **二进制依赖**: `binaries` (git, curl, docker 等)
- **环境变量**: `env` (API keys, tokens 等)
- **工具版本**: `tools` (node, python 等)

### 示例

```yaml
requirements:
  binaries:
    - git      # 必须安装 git
    - curl     # 必须安装 curl
  env:
    - GITHUB_TOKEN  # 必须设置环境变量
  custom:
    os: ["linux", "darwin"]  # 只在 Linux 和 macOS 上启用
```

如果不满足要求，技能会被自动跳过，不会注入到 agent prompt 中。

---

## 内置技能

NekoBot 包含以下内置技能：

- **coding-agent**: 运行 Codex, Claude Code, OpenCode 等编码助手
- **actionbook**: 自动化任务流程
- **apple-notes**: macOS 备忘录集成
- **apple-reminders**: macOS 提醒事项集成
- **bird**: Twitter/X 集成
- 更多...

查看完整列表：

```bash
nekobot skills list --builtin
```

---

## 配置示例

完整配置示例参见 `config.example.json`。

### 最小配置

```json
{
  "agents": {
    "defaults": {
      "workspace": "~/.nekobot/workspace",
      "provider": "anthropic",
      "model": "claude-3-5-sonnet-20241022"
    }
  },
  "providers": {
    "anthropic": {
      "api_key": "sk-ant-xxx"
    }
  }
}
```

### 启用 Gateway 和 Channels

```json
{
  "gateway": {
    "host": "0.0.0.0",
    "port": 8080
  },
  "channels": {
    "telegram": {
      "enabled": true,
      "bot_token": "xxx:yyy",
      "allow_from": ["123456789"]
    },
    "discord": {
      "enabled": true,
      "bot_token": "xxx.yyy.zzz",
      "allow_from": ["user-id-1", "user-id-2"]
    }
  }
}
```

### 启用 Heartbeat 和 Cron

```json
{
  "heartbeat": {
    "enabled": true,
    "interval_minutes": 60
  }
}
```

在工作区创建 `HEARTBEAT.md` 文件定义周期性任务。

---

## WebUI 工具会话 OTP 配置

可以通过 `webui.tool_session_otp_ttl_seconds` 配置工具会话一次性访问密码（OTP）的有效期（秒）：

```json
{
  "webui": {
    "enabled": true,
    "port": 0,
    "public_base_url": "https://nekobot.example.com",
    "username": "admin",
    "password": "",
    "tool_session_otp_ttl_seconds": 180
  }
}
```

- 默认值：`180`（3 分钟）
- 最小值：`30`
- 最大值：`3600`（1 小时）

`webui.public_base_url` 用于生成工具会话分享链接的基础地址：

- 会话里手动填写了「公网访问地址」时，优先使用手动填写值
- 否则优先使用 `webui.public_base_url`
- 若未配置，则回退为当前访问 WebUI 的域名/IP

---

## 常见问题

### Q: 如何查看当前使用的配置文件？

启动时日志会显示：

```
INFO    config  Loaded configuration    {"file": "/Users/user/.nekobot/config.json"}
```

### Q: 为什么某个技能没有加载？

检查技能是否满足系统要求：

```bash
nekobot skills check <skill-id>
```

会显示缺失的依赖：

```
Skill not eligible:
  - missing binaries: git, docker
  - missing environment variables: GITHUB_TOKEN
```

### Q: 如何临时禁用所有技能？

```bash
export NEKOBOT_SKILLS_BUILTIN_ENABLED=false
nekobot agent
```

### Q: 配置文件支持 YAML 格式吗？

暂不支持，仅支持 JSON 格式。

---

## 与 goclaw/picoclaw 的兼容性

NekoBot 保持与 goclaw 的技能生态兼容：

- ✅ SKILL.md 格式完全兼容
- ✅ 自动准入 (gating) 机制
- ✅ 多路径加载顺序
- ✅ 优先级覆盖机制
- ✅ metadata.goclaw 字段支持

可以直接复制 goclaw/openclaw 的技能使用。
