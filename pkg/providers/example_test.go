package providers_test

import (
	"context"
	"fmt"
	"log"

	"nekobot/pkg/providers"
	_ "nekobot/pkg/providers/init" // Register all adaptors
)

// Example_basicChat demonstrates basic non-streaming chat usage.
func Example_basicChat() {
	// Create client
	client, err := providers.NewClient("openai", &providers.RelayInfo{
		ProviderName: "openai",
		APIKey:       "your-api-key-here",
		APIBase:      "https://api.openai.com/v1",
		Model:        "gpt-3.5-turbo",
	})
	if err != nil {
		log.Fatal(err)
	}

	// Build request
	req := &providers.UnifiedRequest{
		Model: "gpt-3.5-turbo",
		Messages: []providers.UnifiedMessage{
			{
				Role:    "user",
				Content: "Say hello!",
			},
		},
		MaxTokens:   50,
		Temperature: 0.7,
	}

	// Make request
	resp, err := client.Chat(context.Background(), req)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Response: %s\n", resp.Content)
	fmt.Printf("Finish reason: %s\n", resp.FinishReason)
}

// Example_streamingChat demonstrates streaming chat usage.
func Example_streamingChat() {
	// Create client
	client, err := providers.NewClient("claude", &providers.RelayInfo{
		ProviderName: "claude",
		APIKey:       "your-api-key-here",
		APIBase:      "https://api.anthropic.com/v1",
		Model:        "claude-sonnet-4-5-20250929",
	})
	if err != nil {
		log.Fatal(err)
	}

	// Build request
	req := &providers.UnifiedRequest{
		Model: "claude-sonnet-4-5-20250929",
		Messages: []providers.UnifiedMessage{
			{
				Role:    "user",
				Content: "Count from 1 to 3.",
			},
		},
		MaxTokens:   100,
		Temperature: 0.7,
	}

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
			fmt.Println() // New line after stream
		},
	}

	// Make streaming request
	if err := client.ChatStream(context.Background(), req, handler); err != nil {
		log.Fatal(err)
	}
}

// Example_toolCalling demonstrates function/tool calling.
func Example_toolCalling() {
	// Create client
	client, err := providers.NewClient("openai", &providers.RelayInfo{
		ProviderName: "openai",
		APIKey:       "your-api-key-here",
		Model:        "gpt-4-turbo-preview",
	})
	if err != nil {
		log.Fatal(err)
	}

	// Build request with tools
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

	// Make request
	resp, err := client.Chat(context.Background(), req)
	if err != nil {
		log.Fatal(err)
	}

	// Check for tool calls
	if len(resp.ToolCalls) > 0 {
		for _, call := range resp.ToolCalls {
			fmt.Printf("Tool: %s\n", call.Name)
			fmt.Printf("Arguments: %v\n", call.Arguments)
		}
	}
}

// Example_listProviders demonstrates how to list registered providers.
func Example_listProviders() {
	providerList := providers.List()
	fmt.Printf("Registered providers: %v\n", providerList)

	// Output will include:
	// openai, claude, anthropic, gemini, google, generic, openrouter, groq, vllm, etc.
}

// Example_multipleProviders demonstrates using different providers.
func Example_multipleProviders() {
	ctx := context.Background()

	// Same request for all providers
	req := &providers.UnifiedRequest{
		Messages: []providers.UnifiedMessage{
			{Role: "user", Content: "Say hello!"},
		},
		MaxTokens:   50,
		Temperature: 0.7,
	}

	// OpenAI
	openaiClient, _ := providers.NewClient("openai", &providers.RelayInfo{
		APIKey: "openai-key",
		Model:  "gpt-3.5-turbo",
	})
	req.Model = "gpt-3.5-turbo"
	respOpenAI, _ := openaiClient.Chat(ctx, req)
	fmt.Printf("OpenAI: %s\n", respOpenAI.Content)

	// Claude
	claudeClient, _ := providers.NewClient("claude", &providers.RelayInfo{
		APIKey: "claude-key",
		Model:  "claude-sonnet-4-5-20250929",
	})
	req.Model = "claude-sonnet-4-5-20250929"
	respClaude, _ := claudeClient.Chat(ctx, req)
	fmt.Printf("Claude: %s\n", respClaude.Content)

	// Gemini
	geminiClient, _ := providers.NewClient("gemini", &providers.RelayInfo{
		APIKey: "gemini-key",
		Model:  "gemini-1.5-flash-latest",
	})
	req.Model = "gemini-1.5-flash-latest"
	respGemini, _ := geminiClient.Chat(ctx, req)
	fmt.Printf("Gemini: %s\n", respGemini.Content)
}
