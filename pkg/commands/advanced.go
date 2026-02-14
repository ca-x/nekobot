package commands

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"nekobot/pkg/agent"
	"nekobot/pkg/config"
	"nekobot/pkg/skills"
	"nekobot/pkg/userprefs"
)

// ChannelManager interface to avoid circular dependency with channels package.
type ChannelManager interface {
	GetEnabledChannels() []Channel
}

// Channel interface for basic channel information.
type Channel interface {
	Name() string
	ID() string
}

// Dependencies holds dependencies needed for advanced commands.
type Dependencies struct {
	Config         *config.Config
	Agent          *agent.Agent
	SkillsManager  *skills.Manager
	ChannelManager ChannelManager
	UserPrefs      *userprefs.Manager
}

// RegisterAdvancedCommands registers advanced commands that require dependencies.
func RegisterAdvancedCommands(registry *Registry, deps Dependencies) error {
	advancedCmds := []*Command{
		{
			Name:        "model",
			Description: "List or switch AI models",
			Usage:       "/model [provider] or /model list",
			Handler:     modelHandler(deps.Config),
		},
		{
			Name:        "gateway",
			Description: "Gateway management (restart, status)",
			Usage:       "/gateway <action>",
			Handler:     gatewayHandler(deps.ChannelManager),
			AdminOnly:   true,
		},
		{
			Name:        "settings",
			Description: "Set per-channel language/name/preferences/skill install mode",
			Usage:       "/settings [show|lang <zh|en|ja>|name <text>|prefs <text>|skillmode <legacy|npx>|clear]",
			Handler:     settingsHandler(deps.UserPrefs),
		},
		{
			Name:        "agent",
			Description: "Switch agent or show agent info",
			Usage:       "/agent [name]",
			Handler:     agentHandler(deps.Config),
		},
	}

	for _, cmd := range advancedCmds {
		if err := registry.Register(cmd); err != nil {
			return fmt.Errorf("failed to register %s: %w", cmd.Name, err)
		}
	}

	// Register skill commands dynamically
	if deps.SkillsManager != nil {
		if err := registerSkillCommands(registry, deps.SkillsManager, deps.Agent, deps.UserPrefs); err != nil {
			return fmt.Errorf("failed to register skill commands: %w", err)
		}
	}

	return nil
}

// modelHandler handles the /model command.
func modelHandler(cfg *config.Config) CommandHandler {
	return func(ctx context.Context, req CommandRequest) (CommandResponse, error) {
		args := strings.TrimSpace(req.Args)

		// List models
		if args == "" || args == "list" {
			var sb strings.Builder
			sb.WriteString("🤖 **Available Providers**\n\n")

			// List all configured providers from profiles
			hasProviders := len(cfg.Providers) > 0

			for _, profile := range cfg.Providers {
				sb.WriteString(fmt.Sprintf("**%s** (%s)\n", profile.Name, profile.ProviderKind))
				if profile.APIBase != "" {
					sb.WriteString(fmt.Sprintf("  Base: %s\n", profile.APIBase))
				}
				if len(profile.Models) > 0 {
					sb.WriteString(fmt.Sprintf("  Models: %d configured\n", len(profile.Models)))
				}
				if profile.DefaultModel != "" {
					sb.WriteString(fmt.Sprintf("  Default: %s\n", profile.DefaultModel))
				}
				if profile.Timeout > 0 {
					sb.WriteString(fmt.Sprintf("  Timeout: %ds\n", profile.Timeout))
				}
				sb.WriteString("\n")
			}

			if !hasProviders {
				sb.WriteString("No providers configured.\n")
			}

			sb.WriteString("\nUse `/model <provider>` to get provider info.")

			return CommandResponse{
				Content:     sb.String(),
				ReplyInline: true,
			}, nil
		}

		// Show provider info
		providerName := strings.ToLower(args)
		var providerProfile *config.ProviderProfile
		found := false

		// Search for provider in providers
		for i := range cfg.Providers {
			if strings.ToLower(cfg.Providers[i].Name) == providerName {
				providerProfile = &cfg.Providers[i]
				found = true
				break
			}
		}

		if !found {
			return CommandResponse{
				Content:     fmt.Sprintf("❌ Provider '%s' not found or not configured. Use `/model list` to see available providers.", providerName),
				ReplyInline: true,
			}, nil
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("🤖 **Provider: %s**\n\n", providerName))
		sb.WriteString(fmt.Sprintf("Type: %s\n", providerProfile.ProviderKind))
		if providerProfile.APIBase != "" {
			sb.WriteString(fmt.Sprintf("Base URL: %s\n", providerProfile.APIBase))
		}
		if len(providerProfile.Models) > 0 {
			sb.WriteString(fmt.Sprintf("Models: %d configured\n", len(providerProfile.Models)))
			if providerProfile.DefaultModel != "" {
				sb.WriteString(fmt.Sprintf("Default Model: %s\n", providerProfile.DefaultModel))
			}
		}
		if providerProfile.Timeout > 0 {
			sb.WriteString(fmt.Sprintf("Timeout: %ds\n", providerProfile.Timeout))
		}

		return CommandResponse{
			Content:     sb.String(),
			ReplyInline: true,
		}, nil
	}
}

// gatewayHandler handles the /gateway command.
func gatewayHandler(channelMgr ChannelManager) CommandHandler {
	return func(ctx context.Context, req CommandRequest) (CommandResponse, error) {
		if channelMgr == nil {
			return CommandResponse{
				Content:     "ℹ️ Channel manager unavailable in current runtime.",
				ReplyInline: true,
			}, nil
		}

		args := strings.TrimSpace(req.Args)

		if args == "" || args == "status" {
			// Show gateway status
			var sb strings.Builder
			sb.WriteString("🌐 **Gateway Status**\n\n")

			channels := channelMgr.GetEnabledChannels()
			sb.WriteString(fmt.Sprintf("Active Channels: %d\n\n", len(channels)))

			for _, ch := range channels {
				sb.WriteString(fmt.Sprintf("• **%s** - %s\n", ch.Name(), ch.ID()))
			}

			return CommandResponse{
				Content:     sb.String(),
				ReplyInline: true,
			}, nil
		}

		switch args {
		case "restart":
			// Note: Actual restart would require service control
			return CommandResponse{
				Content:     "⚠️ Gateway restart is not yet implemented.\n\nThis requires integration with system service control.",
				ReplyInline: true,
			}, nil

		case "reload":
			return CommandResponse{
				Content:     "⚠️ Configuration reload is not yet implemented.\n\nThis requires hot-reload support.",
				ReplyInline: true,
			}, nil

		default:
			return CommandResponse{
				Content:     fmt.Sprintf("❌ Unknown gateway action: %s\n\nAvailable: status, restart, reload", args),
				ReplyInline: true,
			}, nil
		}
	}
}

// agentHandler handles the /agent command.
func agentHandler(cfg *config.Config) CommandHandler {
	return func(ctx context.Context, req CommandRequest) (CommandResponse, error) {
		args := strings.TrimSpace(req.Args)

		// Show current agent info
		if args == "" || args == "info" {
			var sb strings.Builder
			sb.WriteString("🤖 **Agent Information**\n\n")

			// Show default provider
			defaultProvider := cfg.Agents.Defaults.Provider
			if defaultProvider == "" {
				defaultProvider = "Not configured"
			}

			sb.WriteString(fmt.Sprintf("Default Provider: **%s**\n", defaultProvider))
			sb.WriteString(fmt.Sprintf("Model: %s\n", cfg.Agents.Defaults.Model))
			sb.WriteString(fmt.Sprintf("Max Tokens: %d\n", cfg.Agents.Defaults.MaxTokens))
			sb.WriteString(fmt.Sprintf("Temperature: %.2f\n", cfg.Agents.Defaults.Temperature))

			sb.WriteString("\nUse `/agent list` to see all available providers.")

			return CommandResponse{
				Content:     sb.String(),
				ReplyInline: true,
			}, nil
		}

		// List available agents/providers
		if args == "list" {
			var sb strings.Builder
			sb.WriteString("🤖 **Available Providers**\n\n")

			providers := []string{
				"anthropic", "openai", "openrouter", "groq",
				"zhipu", "vllm", "gemini", "nvidia",
				"moonshot", "deepseek",
			}

			currentProvider := cfg.Agents.Defaults.Provider
			for _, p := range providers {
				prefix := "  "
				if p == currentProvider {
					prefix = "→ " // Current
				}
				sb.WriteString(fmt.Sprintf("%s**%s**\n", prefix, p))
			}

			sb.WriteString("\nUse `/model <provider>` to see provider details.")

			return CommandResponse{
				Content:     sb.String(),
				ReplyInline: true,
			}, nil
		}

		// Show provider info
		return CommandResponse{
			Content:     "ℹ️ Use `/model list` to see available providers and their configuration.",
			ReplyInline: true,
		}, nil
	}
}

func settingsHandler(prefsMgr *userprefs.Manager) CommandHandler {
	return func(ctx context.Context, req CommandRequest) (CommandResponse, error) {
		if prefsMgr == nil {
			return CommandResponse{Content: "❌ settings 暂不可用（state 未初始化）", ReplyInline: true}, nil
		}

		args := strings.TrimSpace(req.Args)
		channel := strings.TrimSpace(req.Channel)
		userID := strings.TrimSpace(req.UserID)

		profile, _, err := prefsMgr.Get(ctx, channel, userID)
		if err != nil {
			return CommandResponse{Content: "❌ 读取设置失败: " + err.Error(), ReplyInline: true}, nil
		}

		if args == "" || strings.EqualFold(args, "show") {
			return CommandResponse{Content: formatSettings(profile), ReplyInline: true}, nil
		}

		parts := strings.Fields(args)
		action := strings.ToLower(parts[0])
		value := ""
		if len(parts) > 1 {
			value = strings.TrimSpace(args[len(parts[0]):])
		}

		switch action {
		case "lang", "language":
			lang := strings.ToLower(strings.TrimSpace(value))
			if lang != "zh" && lang != "en" && lang != "ja" {
				return CommandResponse{Content: "❌ 仅支持: zh / en / ja", ReplyInline: true}, nil
			}
			profile.Language = userprefs.NormalizeLanguage(lang)
			if err := prefsMgr.Save(ctx, channel, userID, profile); err != nil {
				return CommandResponse{Content: "❌ 保存失败: " + err.Error(), ReplyInline: true}, nil
			}
			return CommandResponse{Content: "✅ 语言已更新为: " + profile.Language, ReplyInline: true}, nil

		case "name":
			name := strings.TrimSpace(value)
			if name == "" {
				return CommandResponse{Content: "❌ 用法: /settings name <称呼>", ReplyInline: true}, nil
			}
			profile.PreferredName = name
			if err := prefsMgr.Save(ctx, channel, userID, profile); err != nil {
				return CommandResponse{Content: "❌ 保存失败: " + err.Error(), ReplyInline: true}, nil
			}
			return CommandResponse{Content: "✅ 称呼已更新", ReplyInline: true}, nil

		case "prefs", "preference", "preferences":
			pref := strings.TrimSpace(value)
			if pref == "" {
				return CommandResponse{Content: "❌ 用法: /settings prefs <偏好描述>", ReplyInline: true}, nil
			}
			profile.Preferences = pref
			if err := prefsMgr.Save(ctx, channel, userID, profile); err != nil {
				return CommandResponse{Content: "❌ 保存失败: " + err.Error(), ReplyInline: true}, nil
			}
			return CommandResponse{Content: "✅ 偏好已更新", ReplyInline: true}, nil

		case "skillmode", "skill_mode", "skills":
			mode := strings.ToLower(strings.TrimSpace(value))
			switch mode {
			case "npx", "npx_preferred":
				profile.SkillInstallMode = "npx_preferred"
			case "legacy", "default", "current":
				profile.SkillInstallMode = "legacy"
			default:
				return CommandResponse{Content: "❌ 用法: /settings skillmode <legacy|npx>", ReplyInline: true}, nil
			}
			if err := prefsMgr.Save(ctx, channel, userID, profile); err != nil {
				return CommandResponse{Content: "❌ 保存失败: " + err.Error(), ReplyInline: true}, nil
			}
			if profile.SkillInstallMode == "npx_preferred" {
				return CommandResponse{Content: "✅ Skills 安装方式已更新为: npx 优先（失败时回退当前方式）", ReplyInline: true}, nil
			}
			return CommandResponse{Content: "✅ Skills 安装方式已更新为: 当前方式", ReplyInline: true}, nil

		case "clear", "reset":
			if err := prefsMgr.Clear(ctx, channel, userID); err != nil {
				return CommandResponse{Content: "❌ 清除失败: " + err.Error(), ReplyInline: true}, nil
			}
			return CommandResponse{Content: "✅ 设置已清除", ReplyInline: true}, nil

		default:
			return CommandResponse{Content: "ℹ️ 用法: /settings [show|lang <zh|en|ja>|name <text>|prefs <text>|skillmode <legacy|npx>|clear]", ReplyInline: true}, nil
		}
	}
}

func formatSettings(p userprefs.Profile) string {
	lang := p.Language
	if lang == "" {
		lang = "zh"
	}
	name := p.PreferredName
	if name == "" {
		name = "(未设置)"
	}
	prefs := p.Preferences
	if prefs == "" {
		prefs = "(未设置)"
	}

	mode := userprefs.NormalizeSkillInstallMode(p.SkillInstallMode)
	modeLabel := "当前方式"
	if mode == "npx_preferred" {
		modeLabel = "npx 优先"
	}

	return fmt.Sprintf("⚙️ 当前设置\n\n语言: %s\n称呼: %s\n偏好: %s\nSkills安装: %s\n\n用法:\n/settings lang <zh|en|ja>\n/settings name <称呼>\n/settings prefs <偏好描述>\n/settings skillmode <legacy|npx>\n/settings clear", lang, name, prefs, modeLabel)
}

// registerSkillCommands registers commands for all loaded skills.
func registerSkillCommands(registry *Registry, skillsMgr *skills.Manager, ag *agent.Agent, prefsMgr *userprefs.Manager) error {
	allSkills := skillsMgr.List()

	for _, skill := range allSkills {
		// Create a command for this skill
		skillName := skill.Name
		skillDesc := strings.TrimSpace(skill.Description)
		if skillDesc == "" {
			skillDesc = fmt.Sprintf("Run %s skill", skillName)
		}
		cmd := &Command{
			Name:        skillName,
			Description: skillDesc,
			Usage:       fmt.Sprintf("/%s [args]", skillName),
			Handler:     skillHandler(skillsMgr, ag, prefsMgr, skillName),
		}

		// Try to register (ignore if already exists)
		if err := registry.Register(cmd); err != nil {
			// Skill name might conflict with builtin command, skip
			continue
		}

		// Telegram slash command list only supports [a-z0-9_], so register
		// an alias for skills that use dashes or other symbols.
		alias := toTelegramSafeCommandName(skillName)
		if alias != "" && alias != skillName {
			_ = registry.Register(&Command{
				Name:        alias,
				Description: skillDesc,
				Usage:       fmt.Sprintf("/%s [args]", alias),
				Handler:     skillHandler(skillsMgr, ag, prefsMgr, skillName),
			})
		}
	}

	return nil
}

func toTelegramSafeCommandName(name string) string {
	normalized := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(name), "/"))
	if normalized == "" {
		return ""
	}

	var b strings.Builder
	lastUnderscore := false
	for _, r := range normalized {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			lastUnderscore = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			lastUnderscore = false
		case r == '-' || r == '_':
			if b.Len() > 0 && !lastUnderscore {
				b.WriteRune('_')
				lastUnderscore = true
			}
		default:
			// Ignore unsupported characters.
		}

		if b.Len() >= 32 {
			break
		}
	}

	result := strings.Trim(b.String(), "_")
	if result == "" {
		return ""
	}
	return result
}

// skillHandler creates a handler for executing a skill.
func skillHandler(skillsMgr *skills.Manager, ag *agent.Agent, prefsMgr *userprefs.Manager, skillName string) CommandHandler {
	return func(ctx context.Context, req CommandRequest) (CommandResponse, error) {
		// Get the skill
		skill, err := skillsMgr.Get(skillName)
		if err != nil || skill == nil {
			return CommandResponse{
				Content:     fmt.Sprintf("❌ Skill '%s' not found.", skillName),
				ReplyInline: true,
			}, nil
		}

		if ag == nil {
			return CommandResponse{
				Content:     fmt.Sprintf("❌ Skill '%s' is unavailable right now.", skillName),
				ReplyInline: true,
			}, nil
		}

		userTask := strings.TrimSpace(req.Args)
		if userTask == "" {
			userTask = skillName
		}

		installMode := "legacy"
		if prefsMgr != nil {
			if profile, ok, err := prefsMgr.Get(ctx, req.Channel, req.UserID); err == nil && ok {
				installMode = userprefs.NormalizeSkillInstallMode(profile.SkillInstallMode)
			}
		}
		installModeHint := "涉及技能安装时，使用当前方式。"
		if installMode == "npx_preferred" {
			installModeHint = "涉及技能安装时，优先尝试 `npx skills add <owner/repo>`，失败再回退当前方式。"
		}

		confirmedRepo := parseConfirmedInstallRepo(userTask)
		if skillName == "find-skills" && installMode == "npx_preferred" {
			prompt := fmt.Sprintf(
				"你正在处理 Telegram slash command /%s，对应技能 %q。\n"+
					"必须调用技能 %q（skill invoke）并按技能指引执行。\n"+
					"%s\n"+
					"要求：\n"+
					"1) 只做最少必要的工具调用，避免重复尝试；\n"+
					"2) 除非用户已确认，否则不要安装；\n"+
					"3) 成功时不要返回技能说明文本。\n",
				req.Command,
				skill.Name,
				skill.Name,
				installModeHint,
			)
			if confirmedRepo != "" {
				prompt += fmt.Sprintf(
					"用户已确认安装仓库：%s。\n"+
						"请立刻执行安装：优先 `npx skills add %s`，失败再回退现有方式。\n"+
						"完成后只返回最终结果（成功/失败 + 关键原因）。",
					confirmedRepo,
					confirmedRepo,
				)
			} else {
				prompt += fmt.Sprintf(
					"用户请求：%s\n"+
						"请先使用 https://skills.sh/?q=<query> 查找最匹配仓库，并且不要执行安装。\n"+
						"你必须严格输出以下三行（不要多余内容）：\n"+
						"SKILL_INSTALL_PROPOSAL: <owner/repo>\n"+
						"REASON: <一句话原因>\n"+
						"MESSAGE: <给用户的确认提示>\n"+
						"如果没有合适结果，改为输出：\n"+
						"NO_PROPOSAL: <原因>",
					userTask,
				)
			}

			sess := newCommandSession()
			reply, err := ag.Chat(ctx, sess, prompt)
			if err != nil {
				return CommandResponse{
					Content:     fmt.Sprintf("❌ 执行技能失败: %v", err),
					ReplyInline: true,
				}, nil
			}
			return CommandResponse{Content: reply, ReplyInline: true}, nil
		}

		prompt := fmt.Sprintf(
			"你正在处理 Telegram slash command /%s。\n"+
				"必须调用技能 %q（skill invoke）并按技能指引执行。\n"+
				"用户安装偏好：%s\n"+
				"要求：\n"+
				"1) 只做最少必要的工具调用，避免重复尝试；\n"+
				"2) 成功时只返回最终执行结果，不要返回技能说明；\n"+
				"3) 失败时只返回一行错误原因。\n"+
				"用户请求：%s",
			req.Command,
			skill.Name,
			installModeHint,
			userTask,
		)

		sess := newCommandSession()
		reply, err := ag.Chat(ctx, sess, prompt)
		if err != nil {
			return CommandResponse{
				Content:     fmt.Sprintf("❌ 执行技能失败: %v", err),
				ReplyInline: true,
			}, nil
		}

		return CommandResponse{
			Content:     reply,
			ReplyInline: true,
		}, nil
	}
}

func parseConfirmedInstallRepo(task string) string {
	const prefix = "__confirm_install__"
	task = strings.TrimSpace(task)
	if !strings.HasPrefix(task, prefix) {
		return ""
	}
	repo := strings.TrimSpace(strings.TrimPrefix(task, prefix))
	if repo == "" || !strings.Contains(repo, "/") {
		return ""
	}
	return repo
}

type commandSession struct {
	messages []agent.Message
	mu       sync.RWMutex
}

func newCommandSession() *commandSession {
	return &commandSession{messages: make([]agent.Message, 0, 8)}
}

func (s *commandSession) GetMessages() []agent.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]agent.Message(nil), s.messages...)
}

func (s *commandSession) AddMessage(msg agent.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, msg)
}
