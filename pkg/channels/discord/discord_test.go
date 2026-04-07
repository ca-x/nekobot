package discord

import (
	"testing"

	channelcapabilities "nekobot/pkg/channelcapabilities"
)

func TestSupportsNativeCommandsUsesCapabilityMatrix(t *testing.T) {
	channel := &Channel{channelType: "discord"}

	if !channel.supportsNativeCommands(channelcapabilities.CapabilityScopeDM) {
		t.Fatal("expected native commands enabled for discord dm scope")
	}
	if !channel.supportsNativeCommands(channelcapabilities.CapabilityScopeGroup) {
		t.Fatal("expected native commands enabled for discord group scope")
	}
}
