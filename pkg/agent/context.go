package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"nekobot/pkg/skills"
)

// ContextBuilder builds system prompts and message contexts for the agent.
// It aggregates information from multiple sources:
// - Identity and runtime info
// - Bootstrap files (AGENTS.md, SOUL.md, USER.md, etc.)
// - Available tools
// - Memory (long-term and recent)
// - Skills
type ContextBuilder struct {
	workspace string
	memory    *MemoryStore

	// Tool registry reference (set after creation)
	getToolDescriptions func() []string

	// Skills manager reference (set after creation)
	skillsManager *skills.Manager
}

// NewContextBuilder creates a new context builder for the given workspace.
func NewContextBuilder(workspace string) *ContextBuilder {
	return &ContextBuilder{
		workspace: workspace,
		memory:    NewMemoryStore(workspace),
	}
}

// SetToolDescriptionsFunc sets the function to retrieve tool descriptions.
// This allows the context builder to include tool info without circular dependencies.
func (cb *ContextBuilder) SetToolDescriptionsFunc(fn func() []string) {
	cb.getToolDescriptions = fn
}

// SetSkillsManager sets the skills manager for context building.
func (cb *ContextBuilder) SetSkillsManager(sm *skills.Manager) {
	cb.skillsManager = sm
}

// GetMemory returns the memory store.
func (cb *ContextBuilder) GetMemory() *MemoryStore {
	return cb.memory
}

// getIdentity returns the core identity section of the system prompt.
func (cb *ContextBuilder) getIdentity() string {
	now := time.Now().Format("2006-01-02 15:04 (Monday)")
	workspacePath, _ := filepath.Abs(cb.workspace)
	runtimeInfo := fmt.Sprintf("%s %s, Go %s", runtime.GOOS, runtime.GOARCH, runtime.Version())

	toolsSection := cb.buildToolsSection()

	identity := fmt.Sprintf(`# nekobot 🤖

You are nekobot, a helpful AI assistant powered by large language models.

## Current Time
%s

## Runtime
%s

## Workspace
Your workspace is at: %s
- Memory: %s/memory/MEMORY.md (long-term persistent memory)
- Daily Notes: %s/memory/YYYYMM/YYYYMMDD.md (daily activity logs)
- Bootstrap Files: %s/{AGENTS.md,SOUL.md,USER.md,IDENTITY.md}

%s

## Important Rules

1. **ALWAYS use tools** - When you need to perform an action (read/write files, execute commands, search web, etc.), you MUST call the appropriate tool. Do NOT just say you'll do it.

2. **Be helpful and accurate** - When using tools, briefly explain what you're doing.

3. **Memory management** - Important information should be saved to %s/memory/MEMORY.md using the write_file tool.

4. **File operations** - Always use absolute paths or paths relative to the workspace.

5. **Tool execution** - Some tools execute asynchronously. Check their documentation for details.`,
		now, runtimeInfo, workspacePath, workspacePath, workspacePath, workspacePath,
		toolsSection, workspacePath)

	return identity
}

// buildToolsSection creates the tools section of the system prompt.
func (cb *ContextBuilder) buildToolsSection() string {
	if cb.getToolDescriptions == nil {
		return ""
	}

	descriptions := cb.getToolDescriptions()
	if len(descriptions) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Available Tools\n\n")
	sb.WriteString("**CRITICAL**: You MUST use tools to perform actions. Do NOT pretend to execute commands or read files.\n\n")
	sb.WriteString("You have access to the following tools:\n\n")

	for _, desc := range descriptions {
		sb.WriteString(desc)
		sb.WriteString("\n\n")
	}

	return sb.String()
}

// buildSkillsSection creates the skills section of the system prompt.
func (cb *ContextBuilder) buildSkillsSection() string {
	if cb.skillsManager == nil {
		return ""
	}

	skillsInstructions := cb.skillsManager.GetInstructions()
	if skillsInstructions == "" {
		return ""
	}

	return skillsInstructions
}

// LoadBootstrapFiles loads bootstrap files from the workspace.
// These files customize the agent's behavior and personality.
func (cb *ContextBuilder) LoadBootstrapFiles() string {
	bootstrapFiles := []string{
		"AGENTS.md",   // Information about agent capabilities
		"SOUL.md",     // Agent personality and values
		"USER.md",     // User preferences and information
		"IDENTITY.md", // Custom identity overrides
	}

	var parts []string
	for _, filename := range bootstrapFiles {
		filePath := filepath.Join(cb.workspace, filename)
		if data, err := os.ReadFile(filePath); err == nil {
			content := strings.TrimSpace(string(data))
			if content != "" {
				parts = append(parts, fmt.Sprintf("## %s\n\n%s", filename, content))
			}
		}
	}

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, "\n\n---\n\n")
}

// BuildSystemPrompt builds the complete system prompt.
// This includes identity, bootstrap files, tools, skills, and memory.
func (cb *ContextBuilder) BuildSystemPrompt() string {
	var parts []string

	// Core identity section
	parts = append(parts, cb.getIdentity())

	// Bootstrap files (if any)
	bootstrapContent := cb.LoadBootstrapFiles()
	if bootstrapContent != "" {
		parts = append(parts, "# Bootstrap Configuration\n\n"+bootstrapContent)
	}

	// Skills (if any)
	skillsSection := cb.buildSkillsSection()
	if skillsSection != "" {
		parts = append(parts, skillsSection)
	}

	// Memory context (if any)
	memoryContext := cb.memory.GetMemoryContext()
	if memoryContext != "" {
		parts = append(parts, "# Memory\n\n"+memoryContext)
	}

	// Join with separator
	return strings.Join(parts, "\n\n---\n\n")
}

// BuildMessages builds the message array for the provider.
// It includes system prompt, sanitized conversation history, and the current message.
func (cb *ContextBuilder) BuildMessages(history []Message, currentMessage string) []Message {
	messages := []Message{}

	// System prompt
	systemPrompt := cb.BuildSystemPrompt()
	messages = append(messages, Message{
		Role:    "system",
		Content: systemPrompt,
	})

	// Sanitized conversation history
	messages = append(messages, sanitizeHistory(history)...)

	// Current user message
	messages = append(messages, Message{
		Role:    "user",
		Content: currentMessage,
	})

	return messages
}

// sanitizeHistory removes invalid message sequences from conversation history.
// This prevents provider errors caused by orphaned tool messages, duplicate
// system messages, or invalid tool-call/tool-result pairings.
func sanitizeHistory(history []Message) []Message {
	if len(history) == 0 {
		return history
	}

	sanitized := make([]Message, 0, len(history))
	for _, msg := range history {
		switch msg.Role {
		case "system":
			// Drop system messages from history. BuildMessages always
			// constructs its own system message; extra system messages
			// break providers that only accept one (Anthropic, etc.).
			continue

		case "tool":
			if len(sanitized) == 0 {
				// Drop orphaned leading tool message.
				continue
			}
			// Walk backwards to find the nearest assistant message,
			// skipping over any preceding tool messages (multi-tool-call case).
			foundAssistant := false
			for i := len(sanitized) - 1; i >= 0; i-- {
				if sanitized[i].Role == "tool" {
					continue
				}
				if sanitized[i].Role == "assistant" && len(sanitized[i].ToolCalls) > 0 {
					foundAssistant = true
				}
				break
			}
			if !foundAssistant {
				// Drop orphaned tool message with no matching assistant tool call.
				continue
			}
			sanitized = append(sanitized, msg)

		case "assistant":
			if len(msg.ToolCalls) > 0 {
				if len(sanitized) == 0 {
					// Drop assistant tool-call turn at history start.
					continue
				}
				prev := sanitized[len(sanitized)-1]
				if prev.Role != "user" && prev.Role != "tool" {
					// Drop assistant tool-call turn with invalid predecessor.
					continue
				}
			}
			sanitized = append(sanitized, msg)

		default:
			sanitized = append(sanitized, msg)
		}
	}

	return sanitized
}

// Message represents a single message in the conversation.
// This is the agent's internal message format.
type Message struct {
	Role       string     // "system", "user", "assistant", "tool"
	Content    string     // Message content
	ToolCalls  []ToolCall // Tool calls made by assistant
	ToolCallID string     // For tool result messages
}

// ToolCall represents a tool invocation by the LLM.
type ToolCall struct {
	ID        string                 // Unique call ID
	Name      string                 // Tool name
	Arguments map[string]interface{} // Tool arguments
}
