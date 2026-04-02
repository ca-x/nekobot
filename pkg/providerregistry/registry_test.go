package providerregistry

import "testing"

func TestListReturnsKnownProviderTypes(t *testing.T) {
	types := List()
	if len(types) == 0 {
		t.Fatal("expected provider types")
	}

	openai, ok := Get("openai")
	if !ok {
		t.Fatalf("expected openai provider type in registry")
	}
	if openai.DisplayName == "" {
		t.Fatalf("expected openai display name, got %+v", openai)
	}
	if !openai.SupportsDiscovery {
		t.Fatalf("expected openai to support discovery")
	}
	if len(openai.AuthFields) == 0 {
		t.Fatalf("expected openai auth fields, got %+v", openai)
	}
}
