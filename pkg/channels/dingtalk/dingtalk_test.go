package dingtalk

import (
	"testing"

	channelcapabilities "nekobot/pkg/channelcapabilities"
)

func TestSupportsNativeCommandsUsesCapabilityMatrix(t *testing.T) {
	channel := &Channel{channelType: "dingtalk"}

	if !channel.supportsNativeCommands(channelcapabilities.CapabilityScopeDM) {
		t.Fatal("expected native commands enabled for dingtalk dm scope")
	}
	if !channel.supportsNativeCommands(channelcapabilities.CapabilityScopeGroup) {
		t.Fatal("expected native commands enabled for dingtalk group scope")
	}
}
