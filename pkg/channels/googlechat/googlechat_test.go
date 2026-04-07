package googlechat

import "testing"

func TestSupportsNativeCommandsUsesCapabilityMatrix(t *testing.T) {
	channel := &Channel{channelType: "googlechat"}

	if !channel.supportsNativeCommands() {
		t.Fatal("expected native commands enabled for googlechat")
	}
}
