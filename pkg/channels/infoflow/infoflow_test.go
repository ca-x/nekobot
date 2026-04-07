package infoflow

import (
	"testing"

	channelcapabilities "nekobot/pkg/channelcapabilities"
)

func TestSupportsNativeCommandsUsesCapabilityMatrix(t *testing.T) {
	channel := &Channel{channelType: "infoflow"}

	if !channel.supportsNativeCommands(channelcapabilities.CapabilityScopeGroup) {
		t.Fatal("expected native commands enabled for infoflow group scope")
	}
}
