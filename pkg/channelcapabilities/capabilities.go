package channelcapabilities

import "strings"

// CapabilityType represents a channel capability.
type CapabilityType string

const (
	CapabilityReactions      CapabilityType = "reactions"
	CapabilityInlineButtons  CapabilityType = "inline_buttons"
	CapabilityThreads        CapabilityType = "threads"
	CapabilityPolls          CapabilityType = "polls"
	CapabilityStreaming      CapabilityType = "streaming"
	CapabilityMedia          CapabilityType = "media"
	CapabilityNativeCommands CapabilityType = "native_commands"
)

// CapabilityScope defines where a capability is available.
type CapabilityScope string

const (
	CapabilityScopeOff       CapabilityScope = "off"
	CapabilityScopeDM        CapabilityScope = "dm"
	CapabilityScopeGroup     CapabilityScope = "group"
	CapabilityScopeAll       CapabilityScope = "all"
	CapabilityScopeAllowlist CapabilityScope = "allowlist"
)

// ChannelCapabilities defines the supported capability matrix for a channel.
type ChannelCapabilities struct {
	Reactions      CapabilityScope `json:"reactions"`
	InlineButtons  CapabilityScope `json:"inline_buttons"`
	Threads        CapabilityScope `json:"threads"`
	Polls          CapabilityScope `json:"polls"`
	Streaming      CapabilityScope `json:"streaming"`
	Media          CapabilityScope `json:"media"`
	NativeCommands CapabilityScope `json:"native_commands"`
}

// DefaultCapabilities returns the baseline capability matrix.
func DefaultCapabilities() ChannelCapabilities {
	return ChannelCapabilities{
		Reactions:      CapabilityScopeAll,
		InlineButtons:  CapabilityScopeOff,
		Threads:        CapabilityScopeOff,
		Polls:          CapabilityScopeOff,
		Streaming:      CapabilityScopeOff,
		Media:          CapabilityScopeAll,
		NativeCommands: CapabilityScopeAll,
	}
}

// ParseCapabilityScope parses a scope string into a capability scope.
func ParseCapabilityScope(value string) CapabilityScope {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "off":
		return CapabilityScopeOff
	case "dm":
		return CapabilityScopeDM
	case "group":
		return CapabilityScopeGroup
	case "all":
		return CapabilityScopeAll
	case "allowlist":
		return CapabilityScopeAllowlist
	default:
		return CapabilityScopeOff
	}
}

// IsCapabilityEnabled checks whether a capability is enabled in the given scope.
func IsCapabilityEnabled(
	capabilities ChannelCapabilities,
	capability CapabilityType,
	scope CapabilityScope,
	isWhitelisted bool,
) bool {
	var enabled CapabilityScope

	switch capability {
	case CapabilityReactions:
		enabled = capabilities.Reactions
	case CapabilityInlineButtons:
		enabled = capabilities.InlineButtons
	case CapabilityThreads:
		enabled = capabilities.Threads
	case CapabilityPolls:
		enabled = capabilities.Polls
	case CapabilityStreaming:
		enabled = capabilities.Streaming
	case CapabilityMedia:
		enabled = capabilities.Media
	case CapabilityNativeCommands:
		enabled = capabilities.NativeCommands
	default:
		return false
	}

	if enabled == CapabilityScopeOff {
		return false
	}

	switch enabled {
	case CapabilityScopeAll:
		return true
	case CapabilityScopeDM:
		return scope == CapabilityScopeDM
	case CapabilityScopeGroup:
		return scope == CapabilityScopeGroup
	case CapabilityScopeAllowlist:
		return isWhitelisted
	default:
		return false
	}
}

// MergeCapabilities merges multiple capability overrides onto a base matrix.
func MergeCapabilities(base ChannelCapabilities, overrides []ChannelCapabilities) ChannelCapabilities {
	result := base

	for _, override := range overrides {
		if override.Reactions != "" {
			result.Reactions = ParseCapabilityScope(string(override.Reactions))
		}
		if override.InlineButtons != "" {
			result.InlineButtons = ParseCapabilityScope(string(override.InlineButtons))
		}
		if override.Threads != "" {
			result.Threads = ParseCapabilityScope(string(override.Threads))
		}
		if override.Polls != "" {
			result.Polls = ParseCapabilityScope(string(override.Polls))
		}
		if override.Streaming != "" {
			result.Streaming = ParseCapabilityScope(string(override.Streaming))
		}
		if override.Media != "" {
			result.Media = ParseCapabilityScope(string(override.Media))
		}
		if override.NativeCommands != "" {
			result.NativeCommands = ParseCapabilityScope(string(override.NativeCommands))
		}
	}

	return result
}

// GetDefaultCapabilitiesForChannel returns the default capability matrix per channel type.
func GetDefaultCapabilitiesForChannel(channelType string) ChannelCapabilities {
	switch strings.TrimSpace(strings.ToLower(channelType)) {
	case "discord":
		return ChannelCapabilities{
			Reactions:      CapabilityScopeAll,
			InlineButtons:  CapabilityScopeAll,
			Threads:        CapabilityScopeAll,
			Polls:          CapabilityScopeAll,
			Streaming:      CapabilityScopeAll,
			Media:          CapabilityScopeAll,
			NativeCommands: CapabilityScopeAll,
		}
	case "telegram":
		return ChannelCapabilities{
			Reactions:      CapabilityScopeAll,
			InlineButtons:  CapabilityScopeDM,
			Threads:        CapabilityScopeGroup,
			Polls:          CapabilityScopeAll,
			Streaming:      CapabilityScopeDM,
			Media:          CapabilityScopeAll,
			NativeCommands: CapabilityScopeAll,
		}
	case "slack":
		return ChannelCapabilities{
			Reactions:      CapabilityScopeAll,
			InlineButtons:  CapabilityScopeAll,
			Threads:        CapabilityScopeAll,
			Polls:          CapabilityScopeOff,
			Streaming:      CapabilityScopeOff,
			Media:          CapabilityScopeAll,
			NativeCommands: CapabilityScopeAll,
		}
	case "wework":
		return ChannelCapabilities{
			Reactions:      CapabilityScopeOff,
			InlineButtons:  CapabilityScopeOff,
			Threads:        CapabilityScopeOff,
			Polls:          CapabilityScopeOff,
			Streaming:      CapabilityScopeOff,
			Media:          CapabilityScopeAll,
			NativeCommands: CapabilityScopeOff,
		}
	case "whatsapp":
		return ChannelCapabilities{
			Reactions:      CapabilityScopeAll,
			InlineButtons:  CapabilityScopeOff,
			Threads:        CapabilityScopeOff,
			Polls:          CapabilityScopeAll,
			Streaming:      CapabilityScopeOff,
			Media:          CapabilityScopeAll,
			NativeCommands: CapabilityScopeOff,
		}
	case "signal":
		return ChannelCapabilities{
			Reactions:      CapabilityScopeAll,
			InlineButtons:  CapabilityScopeOff,
			Threads:        CapabilityScopeOff,
			Polls:          CapabilityScopeOff,
			Streaming:      CapabilityScopeOff,
			Media:          CapabilityScopeAll,
			NativeCommands: CapabilityScopeOff,
		}
	case "feishu":
		return ChannelCapabilities{
			Reactions:      CapabilityScopeAll,
			InlineButtons:  CapabilityScopeOff,
			Threads:        CapabilityScopeOff,
			Polls:          CapabilityScopeAll,
			Streaming:      CapabilityScopeOff,
			Media:          CapabilityScopeAll,
			NativeCommands: CapabilityScopeAll,
		}
	case "dingtalk":
		return ChannelCapabilities{
			Reactions:      CapabilityScopeAll,
			InlineButtons:  CapabilityScopeOff,
			Threads:        CapabilityScopeOff,
			Polls:          CapabilityScopeAll,
			Streaming:      CapabilityScopeOff,
			Media:          CapabilityScopeAll,
			NativeCommands: CapabilityScopeAll,
		}
	case "qq":
		return ChannelCapabilities{
			Reactions:      CapabilityScopeAll,
			InlineButtons:  CapabilityScopeOff,
			Threads:        CapabilityScopeOff,
			Polls:          CapabilityScopeAll,
			Streaming:      CapabilityScopeOff,
			Media:          CapabilityScopeAll,
			NativeCommands: CapabilityScopeAll,
		}
	case "googlechat":
		return ChannelCapabilities{
			Reactions:      CapabilityScopeAll,
			InlineButtons:  CapabilityScopeOff,
			Threads:        CapabilityScopeGroup,
			Polls:          CapabilityScopeOff,
			Streaming:      CapabilityScopeOff,
			Media:          CapabilityScopeAll,
			NativeCommands: CapabilityScopeGroup,
		}
	case "wechat":
		return ChannelCapabilities{
			Reactions:      CapabilityScopeOff,
			InlineButtons:  CapabilityScopeOff,
			Threads:        CapabilityScopeOff,
			Polls:          CapabilityScopeOff,
			Streaming:      CapabilityScopeOff,
			Media:          CapabilityScopeAll,
			NativeCommands: CapabilityScopeOff,
		}
	case "teams":
		return ChannelCapabilities{
			Reactions:      CapabilityScopeAll,
			InlineButtons:  CapabilityScopeAll,
			Threads:        CapabilityScopeGroup,
			Polls:          CapabilityScopeAll,
			Streaming:      CapabilityScopeOff,
			Media:          CapabilityScopeAll,
			NativeCommands: CapabilityScopeOff,
		}
	default:
		return DefaultCapabilities()
	}
}
