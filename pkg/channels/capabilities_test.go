package channels

import "testing"

func TestGetDefaultCapabilitiesForChannel(t *testing.T) {
	slack := GetDefaultCapabilitiesForChannel("slack")
	if slack.Threads != CapabilityScopeAll {
		t.Fatalf("expected slack threads enabled, got %q", slack.Threads)
	}
	if slack.Polls != CapabilityScopeOff {
		t.Fatalf("expected slack polls off, got %q", slack.Polls)
	}

	telegram := GetDefaultCapabilitiesForChannel("telegram")
	if telegram.InlineButtons != CapabilityScopeDM {
		t.Fatalf("expected telegram inline buttons in dm, got %q", telegram.InlineButtons)
	}
	if telegram.Threads != CapabilityScopeGroup {
		t.Fatalf("expected telegram threads in groups, got %q", telegram.Threads)
	}

	unknown := GetDefaultCapabilitiesForChannel("unknown")
	if unknown.Media != CapabilityScopeAll {
		t.Fatalf("expected unknown channel to use default media scope, got %q", unknown.Media)
	}
}

func TestIsCapabilityEnabled(t *testing.T) {
	caps := ChannelCapabilities{
		Threads:       CapabilityScopeGroup,
		InlineButtons: CapabilityScopeAllowlist,
	}

	if !IsCapabilityEnabled(caps, CapabilityThreads, CapabilityScopeGroup, false) {
		t.Fatal("expected threads enabled for group scope")
	}
	if IsCapabilityEnabled(caps, CapabilityThreads, CapabilityScopeDM, false) {
		t.Fatal("expected threads disabled for dm scope")
	}
	if !IsCapabilityEnabled(caps, CapabilityInlineButtons, CapabilityScopeDM, true) {
		t.Fatal("expected inline buttons enabled for allowlist")
	}
	if IsCapabilityEnabled(caps, CapabilityInlineButtons, CapabilityScopeDM, false) {
		t.Fatal("expected inline buttons disabled when not allowlisted")
	}
}

func TestMergeCapabilities(t *testing.T) {
	base := DefaultCapabilities()
	merged := MergeCapabilities(base, []ChannelCapabilities{
		{
			Threads:        CapabilityScopeGroup,
			InlineButtons:  CapabilityScopeAll,
			NativeCommands: CapabilityScopeDM,
		},
	})

	if merged.Threads != CapabilityScopeGroup {
		t.Fatalf("expected merged threads group, got %q", merged.Threads)
	}
	if merged.InlineButtons != CapabilityScopeAll {
		t.Fatalf("expected merged inline buttons all, got %q", merged.InlineButtons)
	}
	if merged.NativeCommands != CapabilityScopeDM {
		t.Fatalf("expected merged native commands dm, got %q", merged.NativeCommands)
	}
}
