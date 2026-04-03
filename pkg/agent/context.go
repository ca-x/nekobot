package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	promptmemory "nekobot/pkg/memory/prompt"
	"nekobot/pkg/message"
	"nekobot/pkg/preprocess"
	"nekobot/pkg/prompts"
	"nekobot/pkg/skills"
)

// Message is an alias for message.Message for backward compatibility.
type Message = message.Message

// ToolCall is an alias for message.ToolCall for backward compatibility.
type ToolCall = message.ToolCall

const currentTimePlaceholder = "__NEKOBOT_CURRENT_TIME__"

// ContextBuilder builds system prompts and message contexts for the agent.
// It aggregates information from multiple sources:
// - Identity and runtime info
// - Bootstrap files (AGENTS.md, SOUL.md, USER.md, etc.)
// - Available tools
// - Memory (long-term and recent)
// - Skills
type ContextBuilder struct {
	workspace string
	memory    *promptmemory.Store
	composer  *promptmemory.ContextComposer

	// Tool registry reference (set after creation)
	getToolDescriptions func() []string

	// Skills manager reference (set after creation)
	skillsManager *skills.Manager

	// Orchestrator mode affects how skills are rendered.
	orchestratorMode string

	// Preprocessor for @file and @dir mentions
	preprocessor *preprocess.Preprocessor

	cacheMu             sync.RWMutex
	cachedStaticReady   bool
	cachedStaticFiles   []trackedFileState
	cachedToolSignature string
	cachedStaticBlock   string
}

type trackedFileState struct {
	path   string
	exists bool
	mtime  int64
}

// PromptSection describes one rendered system-prompt block together with its
// stability boundary so future context-sources and AgentDefinition views can
// inspect prompt composition without reparsing raw text.
type PromptSection struct {
	ID      string
	Title   string
	Content string
	Stable  bool
}

// NewContextBuilder creates a new context builder for the given workspace.
func NewContextBuilder(workspace string) *ContextBuilder {
	return NewContextBuilderWithMemory(workspace, promptmemory.NewStore(workspace))
}

// NewContextBuilderWithMemory creates a new context builder with an explicit memory store.
func NewContextBuilderWithMemory(workspace string, memoryStore *promptmemory.Store) *ContextBuilder {
	if memoryStore == nil {
		memoryStore = promptmemory.NewStore(workspace)
	}
	return &ContextBuilder{
		workspace:    workspace,
		memory:       memoryStore,
		composer:     promptmemory.NewContextComposer(memoryStore, promptmemory.DefaultContextOptions()),
		preprocessor: preprocess.NewPreprocessorWithWorkspace(workspace),
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

// SetOrchestratorMode sets the orchestrator mode for context building.
// This affects how skills are rendered in the system prompt.
func (cb *ContextBuilder) SetOrchestratorMode(mode string) {
	cb.orchestratorMode = mode
}

// SetPreprocessorConfig configures the preprocessor for @file and @dir mentions.
func (cb *ContextBuilder) SetPreprocessorConfig(cfg preprocess.PreprocessorConfig) {
	cb.preprocessor = preprocess.NewPreprocessor(cfg)
}

// GetPreprocessor returns the preprocessor instance.
func (cb *ContextBuilder) GetPreprocessor() *preprocess.Preprocessor {
	return cb.preprocessor
}

// PreviewPreprocessedInput returns file-mention preprocessing metadata without
// coupling callers to the underlying preprocessor implementation.
func (cb *ContextBuilder) PreviewPreprocessedInput(input string) (*preprocess.Result, error) {
	if cb == nil || cb.preprocessor == nil {
		return &preprocess.Result{
			OriginalInput:  input,
			ProcessedInput: input,
		}, nil
	}
	return cb.preprocessor.Process(input)
}

// GetMemory returns the memory store.
func (cb *ContextBuilder) GetMemory() *promptmemory.Store {
	return cb.memory
}

// SetMemoryContextOptions updates how persistent memory is composed into the system prompt.
func (cb *ContextBuilder) SetMemoryContextOptions(opts promptmemory.ContextOptions) {
	cb.composer = promptmemory.NewContextComposer(cb.memory, opts)
}

// getIdentity returns the core identity section of the system prompt.
func (cb *ContextBuilder) getIdentity() string {
	now := currentTimePlaceholder
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

func (cb *ContextBuilder) buildToolsSection() string {
	if cb.getToolDescriptions == nil {
		return ""
	}

	descriptions := cb.getToolDescriptions()
	if len(descriptions) == 0 {
		return ""
	}
	sort.Strings(descriptions)

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

func (cb *ContextBuilder) currentToolSignature() string {
	if cb.getToolDescriptions == nil {
		return ""
	}
	descriptions := cb.getToolDescriptions()
	if len(descriptions) == 0 {
		return ""
	}
	sort.Strings(descriptions)
	return strings.Join(descriptions, "\n")
}

// buildSkillsSection creates the skills section of the system prompt.
func (cb *ContextBuilder) buildSkillsSection() string {
	if cb.skillsManager == nil {
		return ""
	}

	// For blades orchestrator: only inject always-on skills inline.
	// Regular skills are discovered through blade's list_skills meta-tool.
	if cb.orchestratorMode == orchestratorBlades {
		alwaysInstructions := cb.skillsManager.GetAlwaysInstructions()
		if alwaysInstructions == "" {
			return ""
		}
		return alwaysInstructions + "\n\nAdditional skills are available via the `list_skills` tool."
	}

	// Legacy orchestrator: full skills section.
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

// BuildPromptSections returns the current prompt as explicit sections with a
// stable/dynamic boundary, without changing the existing rendered prompt shape.
func (cb *ContextBuilder) BuildPromptSections() []PromptSection {
	sections := make([]PromptSection, 0, 5)

	identity := strings.TrimSpace(cb.getIdentity())
	if identity != "" {
		sections = append(sections, PromptSection{
			ID:      "identity",
			Title:   "Identity",
			Content: identity,
			Stable:  true,
		})
	}

	bootstrapContent := strings.TrimSpace(cb.LoadBootstrapFiles())
	if bootstrapContent != "" {
		sections = append(sections, PromptSection{
			ID:      "bootstrap",
			Title:   "Bootstrap Configuration",
			Content: "# Bootstrap Configuration\n\n" + bootstrapContent,
			Stable:  true,
		})
	}

	skillsSection := strings.TrimSpace(cb.buildSkillsSection())
	if skillsSection != "" {
		sections = append(sections, PromptSection{
			ID:      "skills",
			Title:   "Skills",
			Content: skillsSection,
			Stable:  false,
		})
	}

	memoryContext := ""
	if cb.composer != nil {
		memoryContext = strings.TrimSpace(cb.composer.Build())
	}
	if memoryContext != "" {
		sections = append(sections, PromptSection{
			ID:      "memory",
			Title:   "Memory",
			Content: "# Memory\n\n" + memoryContext,
			Stable:  false,
		})
	}

	return sections
}

// BuildSystemPrompt builds the complete system prompt.
// This includes identity, bootstrap files, tools, skills, and memory.
func (cb *ContextBuilder) BuildSystemPrompt() string {
	sections := cb.BuildPromptSections()
	if len(sections) == 0 {
		return ""
	}

	staticParts := make([]string, 0, len(sections))
	dynamicParts := make([]string, 0, len(sections))
	for _, section := range sections {
		if strings.TrimSpace(section.Content) == "" {
			continue
		}
		if section.Stable {
			staticParts = append(staticParts, section.Content)
			continue
		}
		dynamicParts = append(dynamicParts, section.Content)
	}

	staticBlock := strings.Join(staticParts, "\n\n---\n\n")
	currentTime := time.Now().Format("2006-01-02 15:04 (Monday)")
	staticBlock = strings.ReplaceAll(staticBlock, currentTimePlaceholder, currentTime)

	dynamicBlock := strings.Join(dynamicParts, "\n\n---\n\n")
	if dynamicBlock == "" {
		return staticBlock
	}
	if staticBlock == "" {
		return dynamicBlock
	}
	return staticBlock + "\n\n---\n\n" + dynamicBlock
}

// BuildSystemPromptWithInjected appends resolved system prompts to the base system prompt.
func (cb *ContextBuilder) BuildSystemPromptWithInjected(extra prompts.ResolvedPromptSet) string {
	base := cb.BuildSystemPrompt()
	injected := strings.TrimSpace(extra.SystemText)
	if injected == "" {
		return base
	}
	if strings.TrimSpace(base) == "" {
		return injected
	}
	return base + "\n\n---\n\n# Managed Prompts\n\n" + injected
}

func (cb *ContextBuilder) buildStaticPromptBlock() string {
	if cb.staticPromptCacheFresh() {
		cb.cacheMu.RLock()
		cached := cb.cachedStaticBlock
		cb.cacheMu.RUnlock()
		return cached
	}

	cb.cacheMu.Lock()
	defer cb.cacheMu.Unlock()

	if cb.staticPromptCacheFreshLocked() {
		return cb.cachedStaticBlock
	}

	var parts []string
	for _, section := range cb.BuildPromptSections() {
		if !section.Stable || strings.TrimSpace(section.Content) == "" {
			continue
		}
		parts = append(parts, section.Content)
	}

	cached := strings.Join(parts, "\n\n---\n\n")
	cb.cachedStaticBlock = cached
	cb.cachedToolSignature = cb.currentToolSignature()
	cb.cachedStaticFiles = cb.captureStaticFileStates()
	cb.cachedStaticReady = true

	return cached
}

func (cb *ContextBuilder) buildDynamicPromptBlock() string {
	parts := make([]string, 0, 2)
	for _, section := range cb.BuildPromptSections() {
		if section.Stable || strings.TrimSpace(section.Content) == "" {
			continue
		}
		parts = append(parts, section.Content)
	}

	return strings.Join(parts, "\n\n---\n\n")
}

func (cb *ContextBuilder) staticPromptCacheFresh() bool {
	cb.cacheMu.RLock()
	if !cb.cachedStaticReady {
		cb.cacheMu.RUnlock()
		return false
	}
	cachedTools := cb.cachedToolSignature
	cachedFiles := append([]trackedFileState(nil), cb.cachedStaticFiles...)
	cb.cacheMu.RUnlock()

	if cachedTools != cb.currentToolSignature() {
		return false
	}
	current := cb.captureStaticFileStates()
	if len(current) != len(cachedFiles) {
		return false
	}
	for i := range current {
		if current[i] != cachedFiles[i] {
			return false
		}
	}
	return true
}

func (cb *ContextBuilder) staticPromptCacheFreshLocked() bool {
	if !cb.cachedStaticReady {
		return false
	}
	if cb.cachedToolSignature != cb.currentToolSignature() {
		return false
	}

	current := cb.captureStaticFileStates()
	if len(current) != len(cb.cachedStaticFiles) {
		return false
	}
	for i := range current {
		if current[i] != cb.cachedStaticFiles[i] {
			return false
		}
	}
	return true
}

func (cb *ContextBuilder) captureStaticFileStates() []trackedFileState {
	files := []string{
		"AGENTS.md",
		"SOUL.md",
		"USER.md",
		"IDENTITY.md",
	}

	states := make([]trackedFileState, 0, len(files))
	for _, filename := range files {
		path := filepath.Join(cb.workspace, filename)
		state := trackedFileState{path: path}
		info, err := os.Stat(path)
		if err == nil {
			state.exists = true
			state.mtime = info.ModTime().UnixNano()
		} else if !os.IsNotExist(err) {
			state.exists = true
			state.mtime = -1
		}
		states = append(states, state)
	}

	if cb.skillsManager != nil {
		for _, skill := range cb.skillsManager.ListEnabled() {
			if skill == nil {
				continue
			}
			path := strings.TrimSpace(skill.FilePath)
			if path == "" {
				continue
			}
			state := trackedFileState{path: path}
			info, err := os.Stat(path)
			if err == nil {
				state.exists = true
				state.mtime = info.ModTime().UnixNano()
			} else if !os.IsNotExist(err) {
				state.exists = true
				state.mtime = -1
			}
			states = append(states, state)
		}
	}

	sort.Slice(states, func(i, j int) bool {
		if states[i].path == states[j].path {
			if states[i].exists == states[j].exists {
				return states[i].mtime < states[j].mtime
			}
			return !states[i].exists && states[j].exists
		}
		return states[i].path < states[j].path
	})

	return states
}

func trimTrailingCurrentUserMessage(history []Message, currentMessage string) []Message {
	if len(history) == 0 {
		return history
	}

	trimmedCurrent := strings.TrimSpace(currentMessage)
	if trimmedCurrent == "" {
		return history
	}

	last := history[len(history)-1]
	if last.Role != "user" {
		return history
	}
	if strings.TrimSpace(last.Content) != trimmedCurrent {
		return history
	}

	return history[:len(history)-1]
}

// BuildMessages builds the message array for the provider.
// It includes system prompt, sanitized conversation history, and the current message.
// If the input contains @file or @dir mentions, they are expanded and injected.
func (cb *ContextBuilder) BuildMessages(history []Message, currentMessage string) []Message {
	messages := []Message{}

	// System prompt
	systemPrompt := cb.BuildSystemPrompt()
	messages = append(messages, Message{
		Role:    "system",
		Content: systemPrompt,
	})

	// Sanitized conversation history.
	normalizedHistory := trimTrailingCurrentUserMessage(history, currentMessage)
	messages = append(messages, sanitizeHistory(normalizedHistory)...)

	// Process @file and @dir mentions
	userContent := currentMessage
	if cb.preprocessor != nil {
		result, err := cb.preprocessor.Process(currentMessage)
		if err == nil && result.FileReferences != "" {
			userContent = result.ProcessedInput + result.BuildContextInjection()
		}
	}

	// Current user message
	messages = append(messages, Message{
		Role:    "user",
		Content: userContent,
	})

	return messages
}

// BuildMessagesWithPromptSet builds messages using injected prompt overlays.
// It also processes @file and @dir mentions in the current message.
func (cb *ContextBuilder) BuildMessagesWithPromptSet(
	history []Message,
	currentMessage string,
	resolved prompts.ResolvedPromptSet,
) []Message {
	messages := []Message{{
		Role:    "system",
		Content: cb.BuildSystemPromptWithInjected(resolved),
	}}

	normalizedHistory := trimTrailingCurrentUserMessage(history, currentMessage)
	messages = append(messages, sanitizeHistory(normalizedHistory)...)

	// Process @file and @dir mentions
	userContent := currentMessage
	if cb.preprocessor != nil {
		result, err := cb.preprocessor.Process(currentMessage)
		if err == nil && result.FileReferences != "" {
			userContent = result.ProcessedInput + result.BuildContextInjection()
		}
	}

	messages = append(messages, Message{
		Role:    "user",
		Content: prompts.ComposeUserMessage(resolved.UserText, userContent),
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
