package agent

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"nekobot/pkg/preprocess"
	"nekobot/pkg/prompts"
)

type ContextSourceKind string

const (
	ContextSourceProjectRules ContextSourceKind = "project_rules"
	ContextSourceSkills       ContextSourceKind = "skills"
	ContextSourceMemory       ContextSourceKind = "memory"
	ContextSourceManaged      ContextSourceKind = "managed_prompts"
	ContextSourceRuntime      ContextSourceKind = "runtime_context"
	ContextSourceMCP          ContextSourceKind = "mcp"
)

type ContextSource struct {
	Kind      string         `json:"kind"`
	Title     string         `json:"title"`
	Stable    bool           `json:"stable"`
	Summary   string         `json:"summary,omitempty"`
	ItemCount int            `json:"item_count,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type ContextFootprint struct {
	SystemChars        int `json:"system_chars"`
	IdentityChars      int `json:"identity_chars"`
	BootstrapChars     int `json:"bootstrap_chars"`
	SkillsChars        int `json:"skills_chars"`
	MemoryChars        int `json:"memory_chars"`
	ManagedPromptChars int `json:"managed_prompt_chars"`
	UserPromptChars    int `json:"user_prompt_chars"`
	RawUserChars       int `json:"raw_user_chars"`
	PreprocessedChars  int `json:"preprocessed_chars"`
	FileReferenceChars int `json:"file_reference_chars"`
	FinalUserChars     int `json:"final_user_chars"`
	TotalChars         int `json:"total_chars"`
	MemoryLimitChars   int `json:"memory_limit_chars"`
	MentionCount       int `json:"mention_count"`
}

type ContextCompactionPreview struct {
	Recommended         bool     `json:"recommended"`
	Strategy            string   `json:"strategy,omitempty"`
	Reasons             []string `json:"reasons,omitempty"`
	EstimatedCharsSaved int      `json:"estimated_chars_saved,omitempty"`
}

type ContextPreflightDecision struct {
	Action        string                   `json:"action,omitempty"`
	BudgetStatus  string                   `json:"budget_status,omitempty"`
	BudgetReasons []string                 `json:"budget_reasons,omitempty"`
	Compaction    ContextCompactionPreview `json:"compaction"`
}

type ContextSourcesPreview struct {
	Sources           []ContextSource          `json:"sources"`
	SystemPromptText  string                   `json:"system_prompt_text,omitempty"`
	UserPromptText    string                   `json:"user_prompt_text,omitempty"`
	PreprocessedInput string                   `json:"preprocessed_input,omitempty"`
	Footprint         ContextFootprint         `json:"footprint"`
	Preflight         ContextPreflightDecision `json:"preflight"`
	BudgetStatus      string                   `json:"budget_status,omitempty"`
	BudgetReasons     []string                 `json:"budget_reasons,omitempty"`
	Compaction        ContextCompactionPreview `json:"compaction"`
	Warnings          []string                 `json:"warnings,omitempty"`
}

func (a *Agent) buildPromptResolveInput(
	provider, model string,
	fallback []string,
	promptCtx PromptContext,
) prompts.ResolveInput {
	return prompts.ResolveInput{
		Channel:           strings.TrimSpace(promptCtx.Channel),
		SessionID:         strings.TrimSpace(promptCtx.SessionID),
		UserID:            strings.TrimSpace(promptCtx.UserID),
		Username:          strings.TrimSpace(promptCtx.Username),
		RequestedProvider: firstNonEmpty(strings.TrimSpace(promptCtx.RequestedProvider), strings.TrimSpace(provider)),
		RequestedModel:    firstNonEmpty(strings.TrimSpace(promptCtx.RequestedModel), strings.TrimSpace(model)),
		RequestedFallback: normalizePromptFallback(promptCtx.RequestedFallback, fallback),
		Workspace:         a.config.WorkspacePath(),
		ExplicitPromptIDs: normalizePromptFallback(promptCtx.ExplicitPromptIDs, nil),
		Custom:            clonePromptCustom(promptCtx.Custom),
	}
}

// PreviewContextSources returns an explainable preview of the main prompt
// sources that will contribute to one request.
func (a *Agent) PreviewContextSources(
	ctx context.Context,
	promptCtx PromptContext,
	userMessage string,
) (*ContextSourcesPreview, error) {
	if a == nil || a.context == nil {
		return &ContextSourcesPreview{}, nil
	}

	resolveInput := a.buildPromptResolveInput(
		promptCtx.RequestedProvider,
		promptCtx.RequestedModel,
		promptCtx.RequestedFallback,
		promptCtx,
	)

	resolved := prompts.ResolvedPromptSet{}
	if a.promptManager != nil {
		item, err := a.promptManager.Resolve(ctx, resolveInput)
		if err != nil {
			return nil, fmt.Errorf("resolve prompts: %w", err)
		}
		if item != nil {
			resolved = *item
		}
	}

	preview := a.buildContextSourcesPreviewFromResolved(resolved, promptCtx, userMessage)
	return &preview, nil
}

func (a *Agent) buildContextSourcesPreviewFromResolved(
	resolved prompts.ResolvedPromptSet,
	promptCtx PromptContext,
	userMessage string,
) ContextSourcesPreview {
	if a == nil || a.context == nil {
		return ContextSourcesPreview{}
	}

	resolveInput := a.buildPromptResolveInput(
		promptCtx.RequestedProvider,
		promptCtx.RequestedModel,
		promptCtx.RequestedFallback,
		promptCtx,
	)

	sources := make([]ContextSource, 0, 8)
	footprint := ContextFootprint{}
	for _, section := range a.context.BuildPromptSections() {
		sectionChars := countRunes(section.Content)
		switch section.ID {
		case "identity":
			footprint.IdentityChars = sectionChars
		case "bootstrap":
			footprint.BootstrapChars = sectionChars
			sources = append(sources, ContextSource{
				Kind:    string(ContextSourceProjectRules),
				Title:   section.Title,
				Stable:  section.Stable,
				Summary: summarizeText(section.Content),
				Metadata: map[string]any{
					"section_id": section.ID,
					"chars":      sectionChars,
				},
			})
		case "skills":
			footprint.SkillsChars = sectionChars
			sources = append(sources, ContextSource{
				Kind:    string(ContextSourceSkills),
				Title:   section.Title,
				Stable:  section.Stable,
				Summary: summarizeText(section.Content),
				Metadata: map[string]any{
					"section_id": section.ID,
					"chars":      sectionChars,
				},
			})
		case "memory":
			footprint.MemoryChars = sectionChars
			sources = append(sources, ContextSource{
				Kind:    string(ContextSourceMemory),
				Title:   section.Title,
				Stable:  section.Stable,
				Summary: summarizeText(section.Content),
				Metadata: map[string]any{
					"section_id": section.ID,
					"chars":      sectionChars,
				},
			})
		}
	}
	footprint.ManagedPromptChars = countRunes(resolved.SystemText)
	footprint.UserPromptChars = countRunes(resolved.UserText)

	if len(resolved.Applied) > 0 || strings.TrimSpace(resolved.SystemText) != "" || strings.TrimSpace(resolved.UserText) != "" {
		promptKeys := make([]string, 0, len(resolved.Applied))
		scopes := make([]string, 0, len(resolved.Applied))
		for _, item := range resolved.Applied {
			promptKeys = append(promptKeys, item.PromptKey)
			scopes = append(scopes, item.Scope)
		}
		sources = append(sources, ContextSource{
			Kind:      string(ContextSourceManaged),
			Title:     "Managed Prompts",
			Stable:    false,
			Summary:   summarizeText(firstNonEmpty(resolved.SystemText, resolved.UserText)),
			ItemCount: len(resolved.Applied),
			Metadata: map[string]any{
				"prompt_keys": uniqueStrings(promptKeys),
				"scopes":      uniqueStrings(scopes),
				"chars":       footprint.ManagedPromptChars,
			},
		})
	}

	runtimeMetadata := make(map[string]any)
	if sessionID := strings.TrimSpace(promptCtx.SessionID); sessionID != "" {
		runtimeMetadata["session_id"] = sessionID
	}
	if channel := strings.TrimSpace(promptCtx.Channel); channel != "" {
		runtimeMetadata["channel"] = channel
	}
	if provider := firstNonEmpty(strings.TrimSpace(promptCtx.RequestedProvider), strings.TrimSpace(resolveInput.RequestedProvider)); provider != "" {
		runtimeMetadata["provider"] = provider
	}
	if model := firstNonEmpty(strings.TrimSpace(promptCtx.RequestedModel), strings.TrimSpace(resolveInput.RequestedModel)); model != "" {
		runtimeMetadata["model"] = model
	}
	if len(resolveInput.RequestedFallback) > 0 {
		runtimeMetadata["fallback"] = append([]string(nil), resolveInput.RequestedFallback...)
	}
	if len(promptCtx.Custom) > 0 {
		customKeys := make([]string, 0, len(promptCtx.Custom))
		for key := range promptCtx.Custom {
			customKeys = append(customKeys, key)
		}
		sort.Strings(customKeys)
		runtimeMetadata["custom_keys"] = customKeys
	}
	if len(runtimeMetadata) > 0 {
		sources = append(sources, ContextSource{
			Kind:     string(ContextSourceRuntime),
			Title:    "Runtime Context",
			Stable:   false,
			Summary:  "Session, route, and injected runtime metadata",
			Metadata: runtimeMetadata,
		})
	}

	if a.config != nil && len(a.config.Agents.Defaults.MCPServers) > 0 {
		names := make([]string, 0, len(a.config.Agents.Defaults.MCPServers))
		for _, server := range a.config.Agents.Defaults.MCPServers {
			name := strings.TrimSpace(server.Name)
			if name != "" {
				names = append(names, name)
			}
		}
		sources = append(sources, ContextSource{
			Kind:      string(ContextSourceMCP),
			Title:     "MCP Servers",
			Stable:    true,
			Summary:   summarizeText(strings.Join(names, ", ")),
			ItemCount: len(names),
			Metadata: map[string]any{
				"names": uniqueStrings(names),
			},
		})
	}

	processedInput := strings.TrimSpace(userMessage)
	var preprocessPreview *preprocess.Result
	if preview, err := a.PreviewPreprocessedInput(userMessage); err == nil {
		preprocessPreview = preview
		processedInput = strings.TrimSpace(preview.ProcessedInput)
	}
	finalUserMessage := prompts.ComposeUserMessage(resolved.UserText, buildPreviewUserContent(strings.TrimSpace(userMessage), preprocessPreview))
	footprint.RawUserChars = countRunes(strings.TrimSpace(userMessage))
	footprint.PreprocessedChars = countRunes(processedInput)
	footprint.FinalUserChars = countRunes(finalUserMessage)
	footprint.SystemChars = countRunes(a.context.BuildSystemPromptWithInjected(resolved))
	footprint.TotalChars = footprint.SystemChars + footprint.FinalUserChars
	if a.config != nil {
		footprint.MemoryLimitChars = a.config.Memory.Context.MaxChars
	}
	if preprocessPreview != nil {
		footprint.FileReferenceChars = countRunes(strings.TrimSpace(preprocessPreview.FileReferences))
		footprint.MentionCount = len(preprocessPreview.Mentions)
	}
	warnings := buildContextPreviewWarnings(footprint, preprocessPreview)
	budgetStatus, budgetReasons := classifyContextBudget(a, footprint, warnings)
	compaction := buildContextCompactionPreview(footprint, budgetStatus, budgetReasons)

	return ContextSourcesPreview{
		Sources:           sources,
		SystemPromptText:  resolved.SystemText,
		UserPromptText:    resolved.UserText,
		PreprocessedInput: processedInput,
		Footprint:         footprint,
		Preflight: ContextPreflightDecision{
			Action:        classifyPreflightAction(budgetStatus),
			BudgetStatus:  budgetStatus,
			BudgetReasons: budgetReasons,
			Compaction:    compaction,
		},
		BudgetStatus:      budgetStatus,
		BudgetReasons:     budgetReasons,
		Compaction:        compaction,
		Warnings:          warnings,
	}
}

func buildPreviewUserContent(rawUserMessage string, preview *preprocess.Result) string {
	if preview == nil || strings.TrimSpace(preview.FileReferences) == "" {
		return rawUserMessage
	}
	return preview.ProcessedInput + preview.BuildContextInjection()
}

func buildContextPreviewWarnings(footprint ContextFootprint, preview *preprocess.Result) []string {
	warnings := make([]string, 0, 4)
	if footprint.MemoryLimitChars > 0 && footprint.MemoryChars*100 >= footprint.MemoryLimitChars*80 {
		warnings = append(warnings, "Memory context is near its configured max chars budget.")
	}
	if footprint.RawUserChars > 0 && footprint.FinalUserChars >= footprint.RawUserChars*4 {
		warnings = append(warnings, "Referenced files expanded the final user message significantly.")
	}
	if preview != nil && len(preview.Warnings) > 0 {
		warnings = append(warnings, preview.Warnings...)
	}
	return warnings
}

func classifyContextBudget(a *Agent, footprint ContextFootprint, warnings []string) (string, []string) {
	reasons := make([]string, 0, 4)
	status := "ok"

	approxCharBudget := 0
	if a != nil && a.config != nil && a.config.Agents.Defaults.MaxTokens > 0 {
		approxCharBudget = a.config.Agents.Defaults.MaxTokens * 5 / 2
	}
	if approxCharBudget > 0 {
		if footprint.TotalChars >= approxCharBudget {
			status = "critical"
			reasons = append(reasons, "Approximate prompt chars exceed the configured max tokens budget.")
		} else if footprint.TotalChars*100 >= approxCharBudget*80 {
			status = maxBudgetStatus(status, "warning")
			reasons = append(reasons, "Approximate prompt chars are near the configured max tokens budget.")
		}
	}

	if footprint.MemoryLimitChars > 0 && footprint.MemoryChars*100 >= footprint.MemoryLimitChars*80 {
		status = maxBudgetStatus(status, "warning")
		reasons = append(reasons, "Memory context is near its configured max chars limit.")
	}
	if footprint.RawUserChars > 0 && footprint.FinalUserChars >= footprint.RawUserChars*4 {
		status = maxBudgetStatus(status, "warning")
		reasons = append(reasons, "Referenced files expanded the effective user message significantly.")
	}
	if len(warnings) > 0 {
		status = maxBudgetStatus(status, "warning")
	}

	return status, uniqueStrings(reasons)
}

func maxBudgetStatus(current, next string) string {
	order := map[string]int{
		"ok":       0,
		"warning":  1,
		"critical": 2,
	}
	if order[next] > order[current] {
		return next
	}
	return current
}

func buildContextCompactionPreview(
	footprint ContextFootprint,
	budgetStatus string,
	budgetReasons []string,
) ContextCompactionPreview {
	preview := ContextCompactionPreview{}

	switch budgetStatus {
	case "critical":
		preview.Recommended = true
		preview.Strategy = "drop_oldest_history"
		preview.EstimatedCharsSaved = footprint.TotalChars / 2
		preview.Reasons = append(preview.Reasons, "Current footprint is already beyond the approximate request budget.")
	case "warning":
		if footprint.MemoryLimitChars > 0 && footprint.MemoryChars*100 >= footprint.MemoryLimitChars*80 {
			preview.Recommended = true
			preview.Strategy = "compress_memory"
			preview.EstimatedCharsSaved = footprint.MemoryChars / 3
			preview.Reasons = append(preview.Reasons, "Memory context is the clearest near-limit source.")
		}
		if !preview.Recommended && footprint.FileReferenceChars > 0 && footprint.RawUserChars > 0 && footprint.FinalUserChars >= footprint.RawUserChars*4 {
			preview.Recommended = true
			preview.Strategy = "trim_referenced_files"
			preview.EstimatedCharsSaved = footprint.FileReferenceChars / 2
			preview.Reasons = append(preview.Reasons, "Referenced file expansion is driving the warning state.")
		}
	}

	if len(preview.Reasons) == 0 && len(budgetReasons) > 0 && preview.Recommended {
		preview.Reasons = append(preview.Reasons, budgetReasons...)
	}
	preview.Reasons = uniqueStrings(preview.Reasons)
	return preview
}

func classifyPreflightAction(budgetStatus string) string {
	switch strings.TrimSpace(budgetStatus) {
	case "critical":
		return "compact_before_run"
	case "warning":
		return "consider_compaction"
	default:
		return "proceed"
	}
}

func countRunes(value string) int {
	return utf8.RuneCountInString(strings.TrimSpace(value))
}

func summarizeText(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ""
	}
	trimmed = strings.ReplaceAll(trimmed, "\n", " ")
	fields := strings.Fields(trimmed)
	trimmed = strings.Join(fields, " ")
	const maxLen = 140
	if len(trimmed) <= maxLen {
		return trimmed
	}
	return trimmed[:maxLen-3] + "..."
}

func uniqueStrings(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	sort.Strings(result)
	return result
}
