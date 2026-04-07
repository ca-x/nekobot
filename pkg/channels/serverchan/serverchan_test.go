package serverchan

import (
	"testing"

	channelcapabilities "nekobot/pkg/channelcapabilities"
)

func TestSupportsNativeCommandsUsesCapabilityMatrix(t *testing.T) {
	channel := &Channel{channelType: "serverchan"}

	if !channel.supportsNativeCommands(channelcapabilities.CapabilityScopeDM) {
		t.Fatal("expected native commands enabled for serverchan dm scope")
	}
}
