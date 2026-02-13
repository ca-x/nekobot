// Package main provides example usage of the nanobot provider system.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"nekobot/pkg/providers"
	_ "nekobot/pkg/providers/init" // Register all adaptors
)

func main() {
	// Example 1: Non-streaming chat with OpenAI
	fmt.Println("=== Example 1: Non-streaming chat with OpenAI ===")
	if err := exampleOpenAIChat(); err != nil {
		log.Printf("OpenAI example failed: %v", err)
	}

	// Example 2: Streaming chat with Claude
	fmt.Println("\n=== Example 2: Streaming chat with Claude ===")
	if err := exampleClaudeStream(); err != nil {
		log.Printf("Claude example failed: %v", err)
	}

	// Example 3: List registered providers
	fmt.Println("\n=== Example 3: List registered providers ===")
	providersList := providers.List()
	fmt.Printf("Registered providers: %v\n", providersList)
}

func exampleOpenAIChat() error {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("OPENAI_API_KEY not set")
	}

	// Create client
	client, err := providers.NewClient("openai", &providers.RelayInfo{
		ProviderName: "openai",
		APIKey:       apiKey,
		APIBase:      "https://api.openai.com/v1",
		Model:        "gpt-3.5-turbo",
	})
	if err != nil {
		return fmt.Errorf("creating client: %w", err)
	}

	// Build request
	req := &providers.UnifiedRequest{
		Model: "gpt-3.5-turbo",
		Messages: []providers.UnifiedMessage{
			{
				Role:    "user",
				Content: "Say hello in one sentence!",
			},
		},
		MaxTokens:   100,
		Temperature: 0.7,
	}

	// Make request
	resp, err := client.Chat(context.Background(), req)
	if err != nil {
		return fmt.Errorf("chat request: %w", err)
	}

	fmt.Printf("Response: %s\n", resp.Content)
	fmt.Printf("Finish reason: %s\n", resp.FinishReason)
	if resp.Usage != nil {
		fmt.Printf("Usage: %d prompt + %d completion = %d total tokens\n",
			resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.TotalTokens)
	}

	return nil
}

func exampleClaudeStream() error {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("ANTHROPIC_API_KEY not set")
	}

	// Create client
	client, err := providers.NewClient("claude", &providers.RelayInfo{
		ProviderName: "claude",
		APIKey:       apiKey,
		APIBase:      "https://api.anthropic.com/v1",
		Model:        "claude-sonnet-4-5-20250929",
	})
	if err != nil {
		return fmt.Errorf("creating client: %w", err)
	}

	// Build request
	req := &providers.UnifiedRequest{
		Model: "claude-sonnet-4-5-20250929",
		Messages: []providers.UnifiedMessage{
			{
				Role:    "system",
				Content: "You are a helpful assistant.",
			},
			{
				Role:    "user",
				Content: "Count from 1 to 5.",
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
			fmt.Println()
			if usage != nil {
				fmt.Printf("Usage: %d prompt + %d completion = %d total tokens\n",
					usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens)
			}
		},
	}

	// Make streaming request
	if err := client.ChatStream(context.Background(), req, handler); err != nil {
		return fmt.Errorf("stream request: %w", err)
	}

	return nil
}
