package auth

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// LoginPasteToken performs authentication by prompting user to paste an API key.
func LoginPasteToken(provider string, r io.Reader) (*AuthCredential, error) {
	fmt.Printf("\nPaste your API key for %s:\n", ProviderDisplayName(provider))
	fmt.Printf("(The input will be hidden for security)\n\n")

	scanner := bufio.NewScanner(r)
	if !scanner.Scan() {
		return nil, fmt.Errorf("failed to read input")
	}

	token := strings.TrimSpace(scanner.Text())
	if token == "" {
		return nil, fmt.Errorf("no token provided")
	}

	return &AuthCredential{
		AccessToken: token,
		Provider:    provider,
		AuthMethod:  "token",
	}, nil
}

// ProviderDisplayName returns a human-readable provider name.
func ProviderDisplayName(provider string) string {
	names := map[string]string{
		"openai":    "OpenAI",
		"anthropic": "Anthropic (Claude)",
		"google":    "Google (Gemini)",
		"cohere":    "Cohere",
		"azure":     "Azure OpenAI",
	}

	if name, ok := names[provider]; ok {
		return name
	}
	return provider
}
