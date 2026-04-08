package wechat

import (
	"fmt"
	"path/filepath"
	"strings"
)

const wechatPresenterInstructions = `WeChat does not render Markdown. Use plain text only.
For rich content such as code blocks longer than 5 lines, tables, SVG, Mermaid, or formatted reports, write the content to a local file under the current workspace or /tmp and include the absolute file path in your reply.
When a local absolute file path appears in your reply, the system may send it as WeChat attachment automatically.
Short plain text can be returned directly.
When you need the user to choose among multiple options, render them as a short numbered plain-text list like "1. ..." and "2. ...", and keep the numbering stable.
When you ask the user to choose, explicitly mention that they can reply with "/select N" using that same numbering.
For confirmation prompts, output "1. Yes / 2. No" or similar binary choices clearly.
For multi-step workflows, number each step and allow "/select" to navigate between steps.`

// PresenterMode defines the output formatting mode for WeChat.
type PresenterMode string

const (
	// ModeAuto automatically detects the best format based on content.
	ModeAuto PresenterMode = "auto"
	// ModePlain forces plain text output.
	ModePlain PresenterMode = "plain"
	// ModeFile forces file attachment output.
	ModeFile PresenterMode = "file"
	// ModeInteractive formats for interactive selection.
	ModeInteractive PresenterMode = "interactive"
)

// FormatOptions provides formatting hints to the presenter.
type FormatOptions struct {
	Mode        PresenterMode
	MaxLength   int
	Workspace   string
	Attachments []string
}

func buildWeChatAgentInput(content, workspace string) string {
	trimmedContent := strings.TrimSpace(content)
	var builder strings.Builder
	builder.WriteString(wechatPresenterInstructions)

	if trimmedWorkspace := strings.TrimSpace(workspace); trimmedWorkspace != "" {
		builder.WriteString("\n\n[Workspace]\n")
		_, _ = fmt.Fprintf(&builder, "Preferred workspace root: %s", filepath.Clean(trimmedWorkspace))
	}

	if trimmedContent == "" {
		return strings.TrimSpace(builder.String())
	}

	builderWithMessage := strings.Builder{}
	builderWithMessage.WriteString("[WeChat Channel Instructions]\n")
	builderWithMessage.WriteString(builder.String())

	builderWithMessage.WriteString("\n\n[User Message]\n")
	builderWithMessage.WriteString(trimmedContent)

	return builderWithMessage.String()
}

// FormatForInteractive creates a formatted message for interactive selection.
func FormatForInteractive(prompt string, options []string) string {
	var builder strings.Builder
	builder.WriteString(prompt)
	builder.WriteString("\n\n")
	for i, opt := range options {
		_, _ = fmt.Fprintf(&builder, "%d. %s\n", i+1, opt)
	}
	builder.WriteString("\n回复 /select N 选择，例如 /select 1")
	return builder.String()
}

// FormatForConfirmation creates a yes/no confirmation prompt.
func FormatForConfirmation(prompt string) string {
	return fmt.Sprintf("%s\n\n1. 是 (Yes)\n2. 否 (No)\n\n回复 /yes、/no、/select 1、/select 2 或直接回复 1、2", prompt)
}

// ShouldUseFileAttachment determines if content should be sent as file attachment.
func ShouldUseFileAttachment(content string) bool {
	// Check for rich content indicators
	richIndicators := []string{
		"```",
		"<table",
		"<svg",
		"```mermaid",
		"| ",
	}
	lower := strings.ToLower(content)
	for _, indicator := range richIndicators {
		if strings.Contains(lower, indicator) {
			return true
		}
	}
	// Check length threshold (WeChat messages have limits)
	if len(content) > 2000 {
		return true
	}
	return false
}

// ExtractFilePaths extracts local file paths from content.
func ExtractFilePaths(content string) []string {
	var paths []string
	// Common patterns for file paths in agent responses
	patterns := []string{
		"/tmp/",
		"/home/",
		"/workspace/",
		"./",
	}
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		for _, pattern := range patterns {
			if strings.Contains(line, pattern) {
				// Extract potential file path
				trimmed := strings.TrimSpace(line)
				if isLikelyFilePath(trimmed) {
					paths = append(paths, trimmed)
				}
				break
			}
		}
	}
	return paths
}

func isLikelyFilePath(s string) bool {
	// Simple heuristic: contains extension or common file indicators
	hasExtension := strings.Contains(s, ".")
	isAbsolute := strings.HasPrefix(s, "/")
	return hasExtension && (isAbsolute || strings.HasPrefix(s, "./"))
}
