# PTY Support in NekoBot

NekoBot 支持 PTY (Pseudo Terminal/伪终端)，为 AI Agent 提供强大的交互式工具能力和后台进程管理。

## 为什么需要 PTY？

许多 CLI 工具需要伪终端才能正常工作：

- **编码 Agent** (Codex, Claude Code, OpenCode) - 交互式 TUI 应用
- **文本编辑器** (vim, nano, emacs) - 需要终端控制
- **监控工具** (htop, top, tmux) - 实时刷新界面
- **进度条** (npm install, git clone) - 动态更新显示
- **颜色输出** - 保留 ANSI 颜色码

没有 PTY，这些工具会：
- 输出断行、乱码
- 颜色丢失
- 进度条无法显示
- Agent 可能卡死

---

## 快速开始

### 1. 标准模式 (默认)

普通命令无需 PTY：

```json
{
  "tool": "exec",
  "command": "ls -la"
}
```

### 2. PTY 模式

运行交互式工具：

```json
{
  "tool": "exec",
  "command": "python3",
  "pty": true,
  "timeout": 60
}
```

### 3. 后台模式

长时间运行的任务：

```json
{
  "tool": "exec",
  "command": "npm install",
  "background": true,
  "pty": true
}
```

返回 Session ID，用于后续监控。

---

## Exec Tool 参数

### 完整参数列表

| 参数 | 类型 | 必需 | 默认值 | 说明 |
|------|------|------|--------|------|
| `command` | string | ✅ | - | Shell 命令 |
| `pty` | boolean | ❌ | false | 使用 PTY 模式 |
| `background` | boolean | ❌ | false | 后台运行 |
| `workdir` | string | ❌ | workspace | 工作目录 |
| `timeout` | integer | ❌ | 30 | 超时（秒），后台模式忽略 |

### 执行模式对比

| 模式 | PTY | Background | 用途 |
|------|-----|------------|------|
| Standard | ❌ | ❌ | 普通命令 (ls, grep) |
| PTY | ✅ | ❌ | 交互式工具 (vim, python REPL) |
| Background | ❌/✅ | ✅ | 长时间任务，返回立即 |

---

## Process Tool - 会话管理

后台会话启动后，使用 `process` 工具管理。

### 1. 列出所有会话

```json
{
  "tool": "process",
  "action": "list"
}
```

**返回示例**：
```
Background Sessions (2):

1. Session ID: abc-123
   Command: npm install
   Status: Running
   Duration: 2m15s
   Output Size: 124 lines

2. Session ID: def-456
   Command: codex exec 'Build app'
   Status: Exited (0)
   Duration: 5m30s
   Output Size: 1523 lines
```

### 2. 检查状态 (Poll)

```json
{
  "tool": "process",
  "action": "poll",
  "sessionId": "abc-123"
}
```

**返回示例**：
```
Session: abc-123
Command: npm install
Workdir: /workspace/myproject
Started: 2024-02-14 10:30:15
Status: Running
Duration: 2m45s
Output Size: 156 lines
```

### 3. 获取输出 (Log)

```json
{
  "tool": "process",
  "action": "log",
  "sessionId": "abc-123",
  "offset": 0,
  "limit": 100
}
```

**参数说明**：
- `offset`: 起始行数 (default: 0)
- `limit`: 返回行数 (default: 100, 0 = 全部)

**返回示例**：
```
Session: abc-123
Total Lines: 156
Showing: 0-100

OUTPUT:
npm WARN deprecated package@1.0.0
...
```

### 4. 发送输入 (Write)

向运行中的进程发送输入：

```json
{
  "tool": "process",
  "action": "write",
  "sessionId": "abc-123",
  "data": "y\n"
}
```

**用途**：
- 回答交互式提示 ("y/n?")
- 发送命令到 REPL
- Ctrl+C: `"\x03"`

### 5. 终止进程 (Kill)

```json
{
  "tool": "process",
  "action": "kill",
  "sessionId": "abc-123"
}
```

---

## 使用场景

### 场景 1: 编码 Agent 集成 🤖

**最重要的应用！**

```json
// 1. 启动 Codex 后台执行
{
  "tool": "exec",
  "command": "codex exec --full-auto 'Add user authentication'",
  "pty": true,
  "background": true,
  "workdir": "myproject"
}

// 返回: Session ID: codex-001

// 2. 监控进度
{
  "tool": "process",
  "action": "log",
  "sessionId": "codex-001",
  "offset": 0,
  "limit": 50
}

// 3. 检查是否完成
{
  "tool": "process",
  "action": "poll",
  "sessionId": "codex-001"
}

// 4. 如果需要，可以发送输入
{
  "tool": "process",
  "action": "write",
  "sessionId": "codex-001",
  "data": "y\n"
}
```

### 场景 2: 交互式 Python REPL

```json
// 1. 启动 Python
{
  "tool": "exec",
  "command": "python3 -i",
  "pty": true,
  "background": true
}

// 返回: Session ID: python-001

// 2. 执行代码
{
  "tool": "process",
  "action": "write",
  "sessionId": "python-001",
  "data": "import requests\n"
}

{
  "tool": "process",
  "action": "write",
  "sessionId": "python-001",
  "data": "print(requests.get('https://api.github.com').json())\n"
}

// 3. 获取输出
{
  "tool": "process",
  "action": "log",
  "sessionId": "python-001"
}
```

### 场景 3: 长时间构建任务

```json
// 1. 启动构建
{
  "tool": "exec",
  "command": "docker build -t myapp .",
  "background": true,
  "pty": true
}

// 2. 定期检查进度
{
  "tool": "process",
  "action": "poll",
  "sessionId": "build-001"
}

// 3. 查看最新输出
{
  "tool": "process",
  "action": "log",
  "sessionId": "build-001",
  "offset": -50,
  "limit": 50
}
```

### 场景 4: 文本编辑（仅演示，实际不推荐）

```json
{
  "tool": "exec",
  "command": "vim config.yaml",
  "pty": true,
  "timeout": 300
}
```

**注意**：Agent 无法"看到"终端界面，建议使用 `read_file` + `write_file` 工具。

---

## 技术细节

### PTY vs 标准 Exec

| 特性 | 标准 Exec | PTY Exec |
|------|-----------|----------|
| STDOUT/STDERR | 分离 | 合并到 PTY |
| 颜色 | ❌ 丢失 | ✅ 保留 |
| 进度条 | ❌ 静态 | ✅ 动态 |
| 交互式 | ❌ 不支持 | ✅ 支持 |
| 性能 | 更快 | 稍慢 |
| 使用场景 | 简单命令 | TUI 应用 |

### 输出缓冲

- **内存限制**：每个会话保留最后 10,000 行输出
- **自动清理**：进程结束 1 小时后自动删除
- **实时读取**：可以随时获取新输出

### 安全机制

- **命令白名单**：`restrict_to_workspace` 启用时检查危险命令
- **超时保护**：非后台模式有超时限制
- **进程隔离**：每个会话独立管理

---

## Coding Agent 集成

NekoBot 内置对编码 Agent 的支持。参考 `builtin/coding-agent` skill。

### 支持的 Coding Agents

- ✅ **Codex** (`codex` CLI)
- ✅ **Claude Code** (`claude-code` CLI)
- ✅ **OpenCode** (`opencode` CLI)
- ✅ **Pi** (`pi` CLI)

### 使用示例

在技能中使用 PTY 模式：

```markdown
---
name: coding-helper
description: AI-powered coding assistant
requirements:
  binaries:
    - codex
---

# Coding Helper

Use codex with PTY mode for code generation:

\`\`\`json
{
  "tool": "exec",
  "command": "codex exec 'Add error handling'",
  "pty": true,
  "background": true,
  "workdir": "src/"
}
\`\`\`

Then monitor with:

\`\`\`json
{
  "tool": "process",
  "action": "log",
  "sessionId": "<returned-id>"
}
\`\`\`
```

---

## 最佳实践

### ✅ 推荐做法

1. **PTY 用于交互工具**
   ```json
   {"command": "codex exec '...'", "pty": true}
   ```

2. **后台用于长任务**
   ```json
   {"command": "npm install", "background": true}
   ```

3. **定期轮询状态**
   - 使用 `poll` 检查是否完成
   - 使用 `log` 获取增量输出

4. **设置合理超时**
   ```json
   {"command": "...", "timeout": 300}
   ```

5. **清理完成的会话**
   - 会话完成后使用 `kill` 或等待自动清理

### ❌ 避免做法

1. **不要在标准模式运行 TUI 应用**
   ```json
   // ❌ 错误
   {"command": "vim file.txt"}

   // ✅ 正确
   {"command": "vim file.txt", "pty": true}
   ```

2. **不要忘记后台任务**
   - 启动后台任务后要监控
   - 完成后及时清理

3. **不要用 PTY 运行简单命令**
   ```json
   // ❌ 不必要的开销
   {"command": "ls", "pty": true}

   // ✅ 标准模式足够
   {"command": "ls"}
   ```

4. **不要无限等待**
   - 设置合理的 `timeout`
   - 后台任务定期检查状态

---

## 故障排除

### 问题 1: 命令挂起

**症状**：PTY 命令长时间无响应

**解决**：
- 检查命令是否需要交互输入
- 使用 `write` 发送输入
- 设置合理的 `timeout`
- 考虑使用 `background` 模式

### 问题 2: 输出乱码

**症状**：PTY 输出包含控制字符

**解决**：
- 这是正常的 ANSI 转义码
- Agent 应忽略或解析这些码
- 使用工具去除：`| sed 's/\x1b\[[0-9;]*m//g'`

### 问题 3: 进程无法终止

**症状**：`kill` 后进程仍在运行

**解决**：
```bash
# 检查进程
ps aux | grep <command>

# 强制终止
kill -9 <pid>
```

### 问题 4: 会话丢失

**症状**：找不到 session ID

**解决**：
```json
{
  "tool": "process",
  "action": "list"
}
```

检查所有活动会话。

---

## 配置

### 全局配置

```json
{
  "tools": {
    "exec": {
      "enabled": true,
      "timeout": 30,
      "allow_background": true,
      "max_sessions": 10
    }
  }
}
```

### Per-Session 配置

每个 `exec` 调用可以覆盖默认值：

```json
{
  "tool": "exec",
  "command": "...",
  "timeout": 120,
  "pty": true,
  "background": true
}
```

---

## 性能指标

- **PTY 开销**: ~5-10ms 额外延迟
- **内存使用**: 每个会话 ~1-5MB
- **并发限制**: 建议 <20 个并发 PTY 会话
- **输出缓冲**: 最后 10k 行，约 1-2MB

---

## 示例工作流

### 完整的 Coding Agent 工作流

```
1. 用户: "用 Codex 给这个项目添加测试"

2. Agent 启动 Codex:
   exec command:"codex exec 'Add unit tests'" pty:true background:true
   → 返回 Session ID: codex-abc

3. Agent 监控进度 (每 10 秒):
   process action:poll sessionId:codex-abc

4. Agent 查看输出:
   process action:log sessionId:codex-abc limit:50

5. Codex 询问确认:
   Agent 看到输出: "Create new test file? (y/n)"

6. Agent 发送确认:
   process action:write sessionId:codex-abc data:"y\n"

7. Codex 完成:
   process action:poll sessionId:codex-abc
   → Status: Exited (0)

8. Agent 报告结果:
   "测试已添加完成！Codex 创建了 3 个测试文件..."
```

---

## 相关资源

- PTY 库: [github.com/creack/pty](https://github.com/creack/pty)
- Codex CLI: [github.com/your-codex-repo](https://github.com/...)
- NekoBot Skills: `pkg/skills/builtin/coding-agent/`

---

这个 PTY 支持使 NekoBot 成为真正强大的编码助手，能够运行和管理复杂的交互式工具！🚀
