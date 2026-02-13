# 添加自定义 API 端点

NekoBot 支持任何 **OpenAI 兼容** 的 API 端点。只需在配置文件中添加 provider 配置即可。

## 快速开始

### 1. 在配置文件中添加 Provider

编辑 `~/.nekobot/config.json`：

```json
{
  "providers": {
    "custom": {
      "api_key": "your-api-key",
      "api_base": "https://your-api.com/v1"
    }
  }
}
```

### 2. 使用自定义 Provider

```bash
# 方式 1: 配置文件指定默认 provider
{
  "agents": {
    "defaults": {
      "provider": "custom",
      "model": "your-model-name"
    }
  }
}

# 方式 2: 命令行覆盖
nekobot agent --provider custom --model your-model-name -m "Hello"

# 方式 3: 环境变量
export NEKOBOT_AGENTS_DEFAULTS_PROVIDER=custom
export NEKOBOT_AGENTS_DEFAULTS_MODEL=your-model-name
nekobot agent
```

---

## 支持的服务

### 1. OpenRouter (多模型聚合)

```json
{
  "providers": {
    "openrouter": {
      "api_key": "sk-or-v1-xxx",
      "api_base": "https://openrouter.ai/api/v1"
    }
  },
  "agents": {
    "defaults": {
      "provider": "openrouter",
      "model": "anthropic/claude-3.5-sonnet"
    }
  }
}
```

**可用模型**：
- `anthropic/claude-3.5-sonnet`
- `openai/gpt-4-turbo`
- `google/gemini-pro`
- `meta-llama/llama-3-70b`
- 更多模型见 [openrouter.ai/models](https://openrouter.ai/models)

---

### 2. DeepSeek (高性价比)

```json
{
  "providers": {
    "deepseek": {
      "api_key": "sk-xxx",
      "api_base": "https://api.deepseek.com/v1"
    }
  },
  "agents": {
    "defaults": {
      "provider": "deepseek",
      "model": "deepseek-chat"
    }
  }
}
```

**可用模型**：
- `deepseek-chat` - 通用对话
- `deepseek-coder` - 代码生成

---

### 3. Together AI

```json
{
  "providers": {
    "together": {
      "api_key": "xxx",
      "api_base": "https://api.together.xyz/v1"
    }
  },
  "agents": {
    "defaults": {
      "provider": "together",
      "model": "meta-llama/Llama-3-70b-chat-hf"
    }
  }
}
```

---

### 4. Groq (超快推理)

```json
{
  "providers": {
    "groq": {
      "api_key": "gsk_xxx",
      "api_base": "https://api.groq.com/openai/v1"
    }
  },
  "agents": {
    "defaults": {
      "provider": "groq",
      "model": "llama-3.1-70b-versatile"
    }
  }
}
```

**可用模型**：
- `llama-3.1-70b-versatile`
- `mixtral-8x7b-32768`
- `gemma-7b-it`

---

### 5. Ollama (本地部署)

```json
{
  "providers": {
    "ollama": {
      "api_key": "ollama",
      "api_base": "http://localhost:11434/v1"
    }
  },
  "agents": {
    "defaults": {
      "provider": "ollama",
      "model": "llama3:70b"
    }
  }
}
```

**安装和运行**：
```bash
# 安装 Ollama
curl -fsSL https://ollama.com/install.sh | sh

# 拉取模型
ollama pull llama3:70b

# 启动 API 服务 (自动)
# Ollama 默认在 localhost:11434 提供 OpenAI 兼容 API
```

**可用模型**：
- `llama3:70b`, `llama3:8b`
- `mistral`, `mixtral`
- `codellama`, `deepseek-coder`
- `qwen2`

---

### 6. LM Studio (本地 GUI)

```json
{
  "providers": {
    "lmstudio": {
      "api_key": "lm-studio",
      "api_base": "http://localhost:1234/v1"
    }
  },
  "agents": {
    "defaults": {
      "provider": "lmstudio",
      "model": "local-model"
    }
  }
}
```

**使用步骤**：
1. 下载 [LM Studio](https://lmstudio.ai/)
2. 在 LM Studio 中下载模型
3. 点击 "Start Server" 启动 API
4. 默认端口: `localhost:1234`

---

### 7. vLLM (高性能自建)

```json
{
  "providers": {
    "vllm": {
      "api_key": "vllm",
      "api_base": "http://localhost:8000/v1"
    }
  },
  "agents": {
    "defaults": {
      "provider": "vllm",
      "model": "meta-llama/Llama-3-70b-chat-hf"
    }
  }
}
```

**部署 vLLM**：
```bash
# 安装 vLLM
pip install vllm

# 启动服务
python -m vllm.entrypoints.openai.api_server \
  --model meta-llama/Llama-3-70b-chat-hf \
  --port 8000
```

---

### 8. Azure OpenAI

```json
{
  "providers": {
    "azure": {
      "api_key": "xxx",
      "api_base": "https://<resource-name>.openai.azure.com/openai/deployments/<deployment-name>"
    }
  },
  "agents": {
    "defaults": {
      "provider": "azure",
      "model": "gpt-4"
    }
  }
}
```

---

## 多 Provider 配置

可以同时配置多个 provider，并根据需要切换：

```json
{
  "providers": {
    "claude": {
      "api_key": "sk-ant-xxx",
      "api_base": "https://api.anthropic.com"
    },
    "openai": {
      "api_key": "sk-xxx",
      "api_base": "https://api.openai.com/v1"
    },
    "ollama": {
      "api_key": "ollama",
      "api_base": "http://localhost:11434/v1"
    }
  },
  "agents": {
    "defaults": {
      "provider": "claude",
      "model": "claude-3-5-sonnet-20241022"
    }
  }
}
```

**动态切换**：
```bash
# 使用 Claude
nekobot agent -m "Hello"

# 使用 OpenAI
nekobot agent --provider openai --model gpt-4 -m "Hello"

# 使用本地 Ollama
nekobot agent --provider ollama --model llama3:70b -m "Hello"
```

---

## API Failover 和负载均衡

### 1. Profile Rotation (API Key 轮换)

```json
{
  "providers": {
    "openai": {
      "api_key": "",
      "api_base": "https://api.openai.com/v1",
      "rotation": {
        "enabled": true,
        "strategy": "round_robin",
        "cooldown": "5m"
      },
      "profiles": {
        "account1": {
          "api_key": "sk-xxx-1",
          "priority": 1
        },
        "account2": {
          "api_key": "sk-xxx-2",
          "priority": 2
        },
        "backup": {
          "api_key": "sk-xxx-3",
          "priority": 3
        }
      }
    }
  }
}
```

**Rotation 策略**：
- `round_robin`: 轮流使用
- `least_used`: 优先使用调用次数最少的
- `random`: 随机选择

### 2. 多 Provider Fallback

在代码中实现：
```go
providers := []string{"openai", "claude", "ollama"}
for _, provider := range providers {
    response, err := agent.ChatWithProvider(ctx, provider, message)
    if err == nil {
        return response
    }
}
```

---

## 环境变量配置

所有配置项都可以通过环境变量覆盖：

```bash
# Provider 配置
export NEKOBOT_PROVIDERS_CUSTOM_API_KEY="your-key"
export NEKOBOT_PROVIDERS_CUSTOM_API_BASE="https://api.example.com/v1"

# Agent 默认配置
export NEKOBOT_AGENTS_DEFAULTS_PROVIDER="custom"
export NEKOBOT_AGENTS_DEFAULTS_MODEL="custom-model"

# 启动
nekobot agent
```

---

## 自定义端点要求

NekoBot 支持任何遵循 **OpenAI API 格式** 的端点。

### 必需的 API 端点

```
POST /v1/chat/completions
```

### 请求格式

```json
{
  "model": "model-name",
  "messages": [
    {"role": "user", "content": "Hello"}
  ],
  "stream": false,
  "temperature": 0.7,
  "max_tokens": 4096
}
```

### 响应格式

```json
{
  "id": "chatcmpl-xxx",
  "object": "chat.completion",
  "created": 1234567890,
  "model": "model-name",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Hello! How can I help you?"
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 10,
    "completion_tokens": 20,
    "total_tokens": 30
  }
}
```

### Streaming 支持

```
POST /v1/chat/completions
Content-Type: application/json

{"model": "...", "messages": [...], "stream": true}
```

SSE 格式响应：
```
data: {"choices":[{"delta":{"content":"Hello"}}]}

data: {"choices":[{"delta":{"content":" world"}}]}

data: [DONE]
```

---

## 测试自定义端点

```bash
# 测试连接
curl -X POST https://your-api.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-api-key" \
  -d '{
    "model": "your-model",
    "messages": [{"role": "user", "content": "Hello"}]
  }'

# 使用 NekoBot 测试
nekobot agent \
  --provider custom \
  --model your-model \
  -m "Hello, test message"
```

---

## 常见问题

### Q: 如何添加不兼容 OpenAI 格式的 API？

A: 需要创建自定义 Adaptor。参考 `pkg/providers/adaptors/claude/` 实现。

### Q: 支持 API 代理吗？

A: 支持。在 `api_base` 中配置代理地址即可：
```json
{
  "api_base": "http://proxy.example.com:8080/v1"
}
```

### Q: 如何使用多个 Ollama 实例？

A: 配置不同名称的 provider：
```json
{
  "providers": {
    "ollama-server1": {
      "api_key": "ollama",
      "api_base": "http://server1:11434/v1"
    },
    "ollama-server2": {
      "api_key": "ollama",
      "api_base": "http://server2:11434/v1"
    }
  }
}
```

### Q: 本地模型性能不够怎么办？

A: 考虑：
1. 使用更小的模型 (7B vs 70B)
2. 启用量化 (Q4, Q5)
3. 使用 vLLM 提高吞吐量
4. 使用 GPU 加速

---

## 推荐配置

### 生产环境

```json
{
  "providers": {
    "claude": {
      "api_key": "sk-ant-xxx",
      "api_base": "https://api.anthropic.com",
      "rotation": {
        "enabled": true,
        "strategy": "least_used",
        "profiles": {
          "prod1": {"api_key": "sk-ant-1", "priority": 1},
          "prod2": {"api_key": "sk-ant-2", "priority": 2}
        }
      }
    },
    "openai-backup": {
      "api_key": "sk-xxx",
      "api_base": "https://api.openai.com/v1"
    }
  },
  "agents": {
    "defaults": {
      "provider": "claude",
      "model": "claude-3-5-sonnet-20241022"
    }
  }
}
```

### 开发环境

```json
{
  "providers": {
    "ollama": {
      "api_key": "ollama",
      "api_base": "http://localhost:11434/v1"
    }
  },
  "agents": {
    "defaults": {
      "provider": "ollama",
      "model": "llama3:8b"
    }
  }
}
```

### 成本优化

```json
{
  "providers": {
    "deepseek": {
      "api_key": "sk-xxx",
      "api_base": "https://api.deepseek.com/v1"
    }
  },
  "agents": {
    "defaults": {
      "provider": "deepseek",
      "model": "deepseek-chat"
    }
  }
}
```

---

现在你可以连接到任何 OpenAI 兼容的 API 端点了！🚀
