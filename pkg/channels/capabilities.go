package channels

import channelcapabilities "nekobot/pkg/channelcapabilities"

// CapabilityType represents a channel capability.
type CapabilityType = channelcapabilities.CapabilityType

const (
	CapabilityReactions      = channelcapabilities.CapabilityReactions
	CapabilityInlineButtons  = channelcapabilities.CapabilityInlineButtons
	CapabilityThreads        = channelcapabilities.CapabilityThreads
	CapabilityPolls          = channelcapabilities.CapabilityPolls
	CapabilityStreaming      = channelcapabilities.CapabilityStreaming
	CapabilityMedia          = channelcapabilities.CapabilityMedia
	CapabilityNativeCommands = channelcapabilities.CapabilityNativeCommands
)

// CapabilityScope defines where a capability is available.
type CapabilityScope = channelcapabilities.CapabilityScope

const (
	CapabilityScopeOff       = channelcapabilities.CapabilityScopeOff
	CapabilityScopeDM        = channelcapabilities.CapabilityScopeDM
	CapabilityScopeGroup     = channelcapabilities.CapabilityScopeGroup
	CapabilityScopeAll       = channelcapabilities.CapabilityScopeAll
	CapabilityScopeAllowlist = channelcapabilities.CapabilityScopeAllowlist
)

// ChannelCapabilities defines the supported capability matrix for a channel.
type ChannelCapabilities = channelcapabilities.ChannelCapabilities

// DefaultCapabilities returns the baseline capability matrix.
func DefaultCapabilities() ChannelCapabilities {
	return channelcapabilities.DefaultCapabilities()
}

// ParseCapabilityScope parses a scope string into a capability scope.
func ParseCapabilityScope(value string) CapabilityScope {
	return channelcapabilities.ParseCapabilityScope(value)
}

// IsCapabilityEnabled checks whether a capability is enabled in the given scope.
func IsCapabilityEnabled(
	capabilities ChannelCapabilities,
	capability CapabilityType,
	scope CapabilityScope,
	isWhitelisted bool,
) bool {
	return channelcapabilities.IsCapabilityEnabled(capabilities, capability, scope, isWhitelisted)
}

// MergeCapabilities merges multiple capability overrides onto a base matrix.
func MergeCapabilities(base ChannelCapabilities, overrides []ChannelCapabilities) ChannelCapabilities {
	return channelcapabilities.MergeCapabilities(base, overrides)
}

// GetDefaultCapabilitiesForChannel returns the default capability matrix per channel type.
func GetDefaultCapabilitiesForChannel(channelType string) ChannelCapabilities {
	return channelcapabilities.GetDefaultCapabilitiesForChannel(channelType)
}
