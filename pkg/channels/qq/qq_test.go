package qq

import (
	"testing"

	channelcapabilities "nekobot/pkg/channelcapabilities"
)

func TestSupportsNativeCommandsUsesCapabilityMatrix(t *testing.T) {
	channel := &Channel{channelType: "qq"}

	if !channel.supportsNativeCommands(channelcapabilities.CapabilityScopeDM) {
		t.Fatal("expected native commands enabled for qq dm scope")
	}
	if !channel.supportsNativeCommands(channelcapabilities.CapabilityScopeGroup) {
		t.Fatal("expected native commands enabled for qq group scope")
	}
}
