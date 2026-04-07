package channels

import "testing"

func TestNativeCommandsCapabilityDefaultsForSelectedChannels(t *testing.T) {
	tests := []struct {
		channel string
		scope   CapabilityScope
		want    bool
	}{
		{channel: "wework", scope: CapabilityScopeDM, want: false},
		{channel: "whatsapp", scope: CapabilityScopeDM, want: false},
		{channel: "dingtalk", scope: CapabilityScopeDM, want: true},
		{channel: "qq", scope: CapabilityScopeDM, want: true},
		{channel: "qq", scope: CapabilityScopeGroup, want: true},
		{channel: "googlechat", scope: CapabilityScopeGroup, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.channel+"_"+string(tt.scope), func(t *testing.T) {
			got := IsCapabilityEnabled(
				GetDefaultCapabilitiesForChannel(tt.channel),
				CapabilityNativeCommands,
				tt.scope,
				false,
			)
			if got != tt.want {
				t.Fatalf("expected native command capability %v for %s/%s, got %v", tt.want, tt.channel, tt.scope, got)
			}
		})
	}
}
