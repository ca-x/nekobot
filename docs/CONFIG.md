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
# 覆盖 gateway.port
export NEKOBOT_GATEWAY_PORT="18790"

# 覆盖 logger.level
export NEKOBOT_LOGGER_LEVEL="debug"

# 启动应用
nekobot agent
```

### 运行时配置存储（SQLite）

从当前版本开始，WebUI 变更的主要运行时配置默认写入同一个数据库文件：

- 数据库文件名：`nekobot.db`
- 数据库目录优先级：`NEKOBOT_DB_DIR` > `storage.db_dir` > 可执行文件目录 > 当前工作目录
- 表：`config_sections`（agents/channels/gateway/tools/heartbeat/approval/logger/webui）
- providers 单独使用 `providers` 表（同一个 `nekobot.db`）

这意味着 `config.json` 可以只保留基础启动配置（如 gateway/webui/logger），
其余日常改动由数据库持久化。

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
always: false
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
  openclaw:
    always: false
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

### Always Skills

可通过 `always: true` 将技能标记为“始终注入”技能：

- 同时满足 `enabled: true` 且通过 eligibility 检查时，技能会进入 `Always Skills` 区块。
- `Always Skills` 以 XML 结构注入，适合放置长期安全规则、关键流程约束。
- 若未设置顶层 `always`，也兼容 `metadata.openclaw.always: true` 写法。

示例：

```yaml
always: true
enabled: true
```

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
  "logger": {
    "level": "info"
  },
  "gateway": {
    "host": "0.0.0.0",
    "port": 8080
  },
  "webui": {
    "enabled": true,
    "port": 0
  }
}
```

然后在 WebUI 中配置 providers/agents/tools/channels（会保存到上面的数据库目录下 `nekobot.db`）。

### 运行时配置（推荐在 WebUI 中编辑）

## WebUI Tool Session Runtime Transport

WebUI 现在支持为 Tool Session 选择终端 / 会话后端。

### 配置项

```json
{
  "webui": {
    "tool_session_runtime_transport": "tmux"
  }
}
```

可选值：

- `tmux` — 默认、当前最稳妥的 shipped backend
- `zellij` — 可选 backend，适合受控验证和更偏 web 的会话工作流

### 优先级

Tool Session transport 的选择顺序为：

1. **当前会话显式选择**（WebUI Tool Session 弹窗里的 Runtime transport）
2. **已保存 session metadata**（恢复 / kill / restart 时沿用）
3. **`webui.tool_session_runtime_transport` 配置值**
4. **内置默认值 `tmux`**

### 建议

- 生产默认继续使用 `tmux`
- 仅在需要验证 zellij 行为时，按会话或按实例显式启用 `zellij`
- 在切换实例级默认值之前，先验证：
  - 新建 Tool Session
  - 刷新 / 重连
  - process status / restore
  - kill / terminate

### 当前状态

截至当前版本：

- `tmux` 是默认 transport
- `zellij` 已可通过 WebUI/API 显式选择
- `zellij` 也可通过 `webui.tool_session_runtime_transport` 设为实例默认
- 仍建议先做受控 rollout，再考虑更广泛启用

```json
{
  "agents": {
    "defaults": {
      "provider": "anthropic",
      "model": "claude-3-5-sonnet-20241022"
    }
  },
  "tools": {
    "exec": {
      "timeout_seconds": 30
    }
  }
}
```

### 启用 Heartbeat（运行时配置）

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
