# NekoBot Provider Architecture

A unified, extensible provider architecture for LLM APIs inspired by newapi, built for the NekoBot project.

## Architecture Overview

The provider system uses a clean **adaptor pattern** to abstract away provider-specific details:

```
┌─────────────────────────────────────────────────────────┐
│                    Application Layer                     │
│            (Agent, CLI, Gateway, etc.)                   │
├─────────────────────────────────────────────────────────┤
│              Provider Client (High-level API)            │
├─────────────────────────────────────────────────────────┤
│         Unified Format Layer (UnifiedRequest/Response)   │
├─────────────────────────────────────────────────────────┤
│  Format Converters    │   OpenAI ↔ Claude ↔ Gemini     │
├─────────────────────────────────────────────────────────┤
│  Provider Adaptors    │   HTTP + Streaming Handlers     │
├─────────────────────────────────────────────────────────┤
│            Provider APIs (OpenAI, Claude, Gemini, etc.)  │
└─────────────────────────────────────────────────────────┘
```

## Key Features

✅ **Unified Interface**: All providers use the same request/response format internally
✅ **Format Agnostic**: Automatic conversion between OpenAI, Claude, and Gemini formats
✅ **Streaming First**: Native streaming support with SSE and JSON-lines
✅ **Extensible**: Add new providers by implementing the `Adaptor` interface
✅ **Thread-Safe**: Provider registry uses mutex-protected maps
✅ **Zero External Dependencies**: Pure Go implementation (only stdlib)

## Supported Providers

| Provider | Status | Format | Streaming |
|----------|--------|--------|-----------|
| OpenAI | ✅ Complete | OpenAI | SSE |
| Claude (Anthropic) | ✅ Complete | Claude | SSE |
| Gemini (Google) | ✅ Complete | Gemini | JSON-lines |
| Generic (OpenAI-compatible) | ✅ Complete | OpenAI | SSE |

**Generic Provider supports:**
- OpenRouter
- Groq
- vLLM
- Together AI
- Perplexity
- DeepSeek
- Moonshot
- Zhipu (GLM)
- NVIDIA NIM

## Usage

### Basic Chat (Non-streaming)

```go
package main

import (
    "context"
    "fmt"
    "nekobot/pkg/providers"
    _ "nekobot/pkg/providers/init" // Register all adaptors
)

func main() {
    // Create client
    client, err := providers.NewClient("openai", &providers.RelayInfo{
        ProviderName: "openai",
        APIKey:       "your-api-key",
        APIBase:      "https://api.openai.com/v1",
        Model:        "gpt-3.5-turbo",
    })
    if err != nil {
        panic(err)
    }

    // Build request
    req := &providers.UnifiedRequest{
        Model: "gpt-3.5-turbo",
        Messages: []providers.UnifiedMessage{
            {
                Role:    "user",
                Content: "Hello, how are you?",
            },
        },
        MaxTokens:   100,
        Temperature: 0.7,
    }

    // Make request
    resp, err := client.Chat(context.Background(), req)
    if err != nil {
        panic(err)
    }

    fmt.Println(resp.Content)
}
```

### Streaming Chat

```go
// Create stream handler
handler := &providers.SimpleStreamHandler{
    OnChunkFunc: func(chunk *providers.UnifiedStreamChunk) error {
        if chunk.Delta.Content != "" {
            fmt.Print(chunk.Delta.Content)
        }
        return nil
    },
    OnErrorFunc: func(err error) {
        log.Printf("Stream error: %v", err)
    },
    OnCompleteFunc: func(usage *providers.UnifiedUsage) {
        fmt.Println()
        if usage != nil {
            fmt.Printf("Tokens used: %d\n", usage.TotalTokens)
        }
    },
}

// Make streaming request
err := client.ChatStream(context.Background(), req, handler)
```

### Tool/Function Calling

```go
req := &providers.UnifiedRequest{
    Model: "gpt-4-turbo-preview",
    Messages: []providers.UnifiedMessage{
        {
            Role:    "user",
            Content: "What's the weather in San Francisco?",
        },
    },
    Tools: []providers.UnifiedTool{
        {
            Type:        "function",
            Name:        "get_weather",
            Description: "Get the current weather in a location",
            Parameters: map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "location": map[string]interface{}{
                        "type":        "string",
                        "description": "The city and state, e.g. San Francisco, CA",
                    },
                },
                "required": []string{"location"},
            },
        },
    },
}

resp, err := client.Chat(context.Background(), req)

// Check for tool calls
if len(resp.ToolCalls) > 0 {
    for _, call := range resp.ToolCalls {
        fmt.Printf("Tool: %s\n", call.Name)
        fmt.Printf("Arguments: %v\n", call.Arguments)
    }
}
```

## Adding a New Provider

To add a new provider, implement the `providers.Adaptor` interface:

```go
package myprovider

import (
    "context"
    "nekobot/pkg/providers"
)

type MyAdaptor struct {
    // Your fields here
}

func New() *MyAdaptor {
    return &MyAdaptor{}
}

// Implement all Adaptor interface methods:
// - Init(info *RelayInfo) error
// - GetRequestURL(info *RelayInfo) (string, error)
// - SetupRequestHeader(req *http.Request, info *RelayInfo) error
// - ConvertRequest(unified *UnifiedRequest, info *RelayInfo) ([]byte, error)
// - DoRequest(ctx context.Context, req *http.Request) ([]byte, error)
// - DoResponse(body []byte, info *RelayInfo) (*UnifiedResponse, error)
// - DoStreamResponse(ctx context.Context, reader io.Reader, handler StreamHandler, info *RelayInfo) error
// - GetModelList() ([]string, error)

func init() {
    providers.Register("myprovider", func() providers.Adaptor {
        return New()
    })
}
```

## Design Decisions

### Why Adaptor Pattern?

The adaptor pattern decouples application logic from provider specifics. Each provider is completely self-contained - adding a new provider requires zero changes to existing code.

### Why Unified Format?

Different providers use different request/response formats:
- OpenAI: Messages array with tool_calls
- Claude: Content blocks with tool_use
- Gemini: Parts array with functionCall

The unified format provides a single, provider-agnostic representation that the agent core can work with, while format converters handle the translation.

### Why Separate Converters?

Separating format conversion from HTTP handling makes the system more testable and maintainable. Converters can be unit tested independently, and new formats can be added without touching the HTTP layer.

## File Structure

```
pkg/providers/
├── types.go              # Core types (UnifiedRequest, UnifiedResponse, Adaptor)
├── registry.go           # Provider registry (thread-safe)
├── client.go             # High-level client API
├── init/
│   └── init.go          # Import all adaptors (triggers registration)
├── converter/
│   ├── converter.go     # Base converter interface
│   ├── openai.go        # OpenAI format converter
│   ├── claude.go        # Claude format converter
│   └── gemini.go        # Gemini format converter
├── streaming/
│   └── processor.go     # SSE and JSON-lines stream processing
└── adaptor/
    ├── openai/
    │   └── adaptor.go   # OpenAI adaptor implementation
    ├── claude/
    │   └── adaptor.go   # Claude adaptor implementation
    ├── gemini/
    │   └── adaptor.go   # Gemini adaptor implementation
    └── generic/
        └── adaptor.go   # Generic OpenAI-compatible adaptor
```

## Testing

```bash
# Build the providers package
go build ./pkg/providers/...

# Run the example
go run examples/provider_usage.go

# Run tests (when implemented)
go test ./pkg/providers/...
```

## Next Steps

- [ ] Add unit tests for converters
- [ ] Add integration tests for adaptors
- [ ] Add benchmarks for performance testing
- [ ] Implement retry logic with exponential backoff
- [ ] Add request/response logging
- [ ] Add metrics collection (latency, token usage)
- [ ] Add support for embeddings and image generation
- [ ] Document provider-specific quirks and limitations

## License

MIT
