package teams

import (
	"testing"

	channelcapabilities "nekobot/pkg/channelcapabilities"
)

func TestSupportsNativeCommandsUsesCapabilityMatrix(t *testing.T) {
	channel := &Channel{channelType: "teams"}
	if channel.supportsNativeCommands() {
		t.Fatal("expected teams native commands to be disabled by capability matrix")
	}
	if channelcapabilities.IsCapabilityEnabled(
		channelcapabilities.GetDefaultCapabilitiesForChannel("teams"),
		channelcapabilities.CapabilityNativeCommands,
		channelcapabilities.CapabilityScopeGroup,
		false,
	) {
		t.Fatal("expected direct capability check to disable teams native commands")
	}
}
