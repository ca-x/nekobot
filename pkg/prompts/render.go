package prompts

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
	"time"
)

func renderPromptTemplate(promptKey, raw string, input ResolveInput) (string, error) {
	now := time.Now()
	channelData := map[string]any{
		"id": strings.TrimSpace(input.Channel),
	}
	sessionData := map[string]any{
		"id": strings.TrimSpace(input.SessionID),
	}
	userData := map[string]any{
		"id":   strings.TrimSpace(input.UserID),
		"name": strings.TrimSpace(input.Username),
	}
	routeData := map[string]any{
		"provider": strings.TrimSpace(input.RequestedProvider),
		"model":    strings.TrimSpace(input.RequestedModel),
		"fallback": append([]string(nil), input.RequestedFallback...),
	}
	workspaceData := map[string]any{
		"path": strings.TrimSpace(input.Workspace),
	}
	customData := cloneMap(input.Custom)
	nowData := map[string]any{
		"rfc3339": now.Format(time.RFC3339),
		"date":    now.Format("2006-01-02"),
		"time":    now.Format("15:04:05"),
	}

	tmpl, err := template.New(promptKey).
		Funcs(template.FuncMap{
			"now":       func() map[string]any { return nowData },
			"channel":   func() map[string]any { return channelData },
			"session":   func() map[string]any { return sessionData },
			"user":      func() map[string]any { return userData },
			"route":     func() map[string]any { return routeData },
			"workspace": func() map[string]any { return workspaceData },
			"custom":    func() map[string]any { return customData },
		}).
		Option("missingkey=error").
		Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	data := map[string]any{
		"now":       nowData,
		"channel":   channelData,
		"session":   sessionData,
		"user":      userData,
		"route":     routeData,
		"workspace": workspaceData,
		"custom":    customData,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}
	return strings.TrimSpace(buf.String()), nil
}

func ComposeUserMessage(userPrompt, userMessage string) string {
	trimmedPrompt := strings.TrimSpace(userPrompt)
	trimmedMessage := strings.TrimSpace(userMessage)
	if trimmedPrompt == "" {
		return trimmedMessage
	}
	if trimmedMessage == "" {
		return trimmedPrompt
	}
	return "[Prompt Template]\n" + trimmedPrompt + "\n\n[User Message]\n" + trimmedMessage
}

func cloneMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}
