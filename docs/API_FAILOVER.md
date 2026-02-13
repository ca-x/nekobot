# API Failover & Load Balancing

NekoBot 支持跨 Provider 的自动故障转移和负载均衡，提供高可用性和可靠性。

## 简化的配置方式

NekoBot 采用极简配置理念，只需一个 `fallback` 数组即可实现强大的故障转移功能：

```json
{
  "agents": {
    "defaults": {
      "provider": "anthropic",
      "fallback": ["openai", "ollama"],
      "model": "claude-3-5-sonnet-20241022"
    }
  }
}
```

## 工作原理

### 1. 自动故障转移

请求按顺序尝试 provider，直到成功：

```
1. 尝试 anthropic (主 provider)
   ↓ 失败
2. 尝试 openai (fallback[0])
   ↓ 失败
3. 尝试 ollama (fallback[1])
   ✓ 成功
```

### 2. 断路器保护 (Circuit Breaker)

自动保护失败的 provider，避免重复请求：

**触发条件**：
- 连续 5 次失败

**行为**：
- Provider 进入 `Open` 状态
- 5 分钟内跳过该 provider
- 5 分钟后尝试恢复 (`HalfOpen` 状态)

**恢复条件**：
- 连续 2 次成功后恢复正常 (`Closed` 状态)

### 3. 智能超时

根据 provider 类型自动设置超时：

| Provider 类型 | 超时时间 | 示例 |
|--------------|----------|------|
| 云端 API | 30 秒 | anthropic, openai, deepseek |
| 本地模型 | 60 秒 | ollama, lmstudio, vllm |

### 4. 断路器状态

| 状态 | 说明 | 行为 |
|------|------|------|
| `Closed` | 正常工作 | 接受请求 |
| `Open` | 断路保护中 | 跳过该 provider |
| `HalfOpen` | 恢复测试中 | 尝试部分请求 |

## 配置示例

### 示例 1: 云端主力 + 本地备份

```json
{
  "agents": {
    "defaults": {
      "provider": "anthropic",
      "fallback": ["openai", "ollama"]
    }
  }
}
```

**使用场景**：
- 主用 Claude API
- OpenAI 作为第一备份
- 本地 Ollama 作为最后备份

### 示例 2: 国产大模型

```json
{
  "agents": {
    "defaults": {
      "provider": "deepseek",
      "fallback": ["zhipu", "moonshot"]
    }
  }
}
```

**使用场景**：
- 主用 DeepSeek（性价比高）
- 智谱 AI 作为备份
- 月之暗面作为最后备份

### 示例 3: 本地优先节省成本

```json
{
  "agents": {
    "defaults": {
      "provider": "ollama",
      "fallback": ["openai"]
    }
  }
}
```

**使用场景**：
- 优先使用免费的本地模型
- 本地失败时再使用付费 API

### 示例 4: 高可靠性配置

```json
{
  "agents": {
    "defaults": {
      "provider": "anthropic",
      "fallback": ["openai", "groq", "ollama"]
    }
  }
}
```

**使用场景**：
- 多个云端 provider 备份
- 最后使用本地模型兜底

## 实际运行场景

### 场景 1: API 限流

```
1. 请求 anthropic
2. 收到 429 (rate limit)
3. anthropic 断路器打开，进入 5 分钟冷却
4. 自动切换到 openai
5. 请求成功
6. 5 分钟后 anthropic 自动恢复可用
```

### 场景 2: 网络故障

```
1. 请求 openai
2. 网络超时 (30秒)
3. openai 连续失败 5 次
4. 断路器打开
5. 自动切换到 groq
6. 请求成功
```

### 场景 3: 所有 Provider 失败

```
1. 尝试 anthropic → 失败
2. 尝试 openai → 失败
3. 尝试 ollama → 失败
4. 返回错误: "all providers failed, last error: ..."
```

## 监控和统计

LoadBalancer 提供统计信息（未来版本将提供 CLI 查看）：

```go
stats := loadBalancer.GetStats()
for name, stat := range stats {
    fmt.Printf("Provider: %s\n", name)
    fmt.Printf("  Requests: %d\n", stat.RequestCount)
    fmt.Printf("  Success: %d\n", stat.SuccessCount)
    fmt.Printf("  Failure: %d\n", stat.FailureCount)
    fmt.Printf("  Consecutive Errors: %d\n", stat.ConsecutiveErrors)
    fmt.Printf("  Circuit State: %s\n", stat.CircuitState)
    fmt.Printf("  Last Success: %s\n", stat.LastSuccess)
    fmt.Printf("  Last Failure: %s\n", stat.LastFailure)
}
```

## 错误类型处理

LoadBalancer 对所有错误类型一视同仁：

| 错误类型 | HTTP 状态码 | 处理方式 |
|----------|-------------|----------|
| 认证错误 | 401, 403 | 计入失败次数 |
| 限流错误 | 429 | 计入失败次数 |
| 服务器错误 | 500, 502, 503 | 计入失败次数 |
| 网络错误 | Timeout, DNS | 计入失败次数 |

所有失败都会：
1. 增加 `ConsecutiveErrors` 计数
2. 达到阈值（5次）后触发断路器
3. 自动切换到下一个 provider

## 最佳实践

### 1. Provider 选择顺序

建议从可靠性高到低排列：

```
主力 provider → 备用 provider → 本地 provider
```

### 2. 至少配置一个备份

```json
// ❌ 不推荐 - 没有备份
{
  "provider": "anthropic"
}

// ✅ 推荐 - 至少一个备份
{
  "provider": "anthropic",
  "fallback": ["openai"]
}
```

### 3. 本地模型作为最后兜底

```json
{
  "provider": "anthropic",
  "fallback": ["openai", "ollama"]
}
```

这样即使所有云端 API 都失败，还能使用本地模型。

### 4. 根据成本优化顺序

```json
// 从便宜到贵
{
  "provider": "deepseek",      // ¥0.001/1K tokens
  "fallback": [
    "moonshot",                 // ¥0.012/1K tokens
    "openai"                    // ¥0.03/1K tokens
  ]
}
```

## 与旧版配置的区别

### 旧版 (复杂)

```json
{
  "providers": {
    "anthropic": {
      "rotation": {
        "enabled": true,
        "strategy": "round_robin",
        "cooldown": "5m"
      },
      "profiles": {
        "primary": {"api_key": "sk-xxx-1", "priority": 1},
        "secondary": {"api_key": "sk-xxx-2", "priority": 2}
      }
    }
  },
  "loadbalancer": {
    "enabled": true,
    "strategy": "priority",
    "providers": [
      {"name": "anthropic", "priority": 100, "max_retries": 3},
      {"name": "openai", "priority": 90, "max_retries": 3}
    ],
    "circuit_breaker": {
      "enabled": true,
      "failure_threshold": 5,
      "timeout": "5m"
    }
  }
}
```

### 新版 (简化)

```json
{
  "agents": {
    "defaults": {
      "provider": "anthropic",
      "fallback": ["openai"]
    }
  }
}
```

**简化优势**：
- ✅ 配置极简，只需一行 `fallback` 数组
- ✅ 断路器、超时等参数都有智能默认值
- ✅ 自动识别本地 provider 并调整超时
- ✅ 零配置，开箱即用

## 技术细节

### 断路器状态机

```
        +--------+
        | Closed |  <-- 初始状态，正常工作
        +--------+
             |
             | 连续 5 次失败
             v
        +--------+
        |  Open  |  <-- 断路保护，跳过该 provider
        +--------+
             |
             | 5 分钟后
             v
        +----------+
        | HalfOpen |  <-- 恢复测试中
        +----------+
          /     \
   失败  /       \  连续 2 次成功
        v         v
    +--------+  +--------+
    |  Open  |  | Closed |
    +--------+  +--------+
```

### 线程安全

LoadBalancer 使用 `sync.RWMutex` 保证线程安全：
- 多个请求可以并发访问不同 provider
- 状态更新时自动加锁
- 统计信息读取时使用读锁

### 性能特点

- **内存占用**：每个 provider state ~1KB
- **并发性能**：读操作无锁竞争
- **切换延迟**：<1ms (内存操作)

## 未来计划

- [ ] CLI 命令查看 provider 状态
- [ ] 配置热重载
- [ ] 自定义断路器参数
- [ ] Provider 健康检查
- [ ] 请求统计和图表
- [ ] 自动调整 fallback 顺序（基于成功率）

## 参考

- [CUSTOM_PROVIDERS.md](./CUSTOM_PROVIDERS.md) - 如何添加自定义 provider
- [CONFIG.md](./CONFIG.md) - 完整配置说明
- [PTY.md](./PTY.md) - PTY 支持和后台进程管理
