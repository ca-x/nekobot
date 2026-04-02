package providerregistry

import "sort"

type Field struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Type        string `json:"type"`
	Placeholder string `json:"placeholder,omitempty"`
	Required    bool   `json:"required"`
	Secret      bool   `json:"secret,omitempty"`
}

type Type struct {
	ID                string   `json:"id"`
	DisplayName       string   `json:"display_name"`
	Icon              string   `json:"icon"`
	Description       string   `json:"description"`
	DefaultAPIBase    string   `json:"default_api_base,omitempty"`
	SupportsDiscovery bool     `json:"supports_discovery"`
	Capabilities      []string `json:"capabilities,omitempty"`
	AuthFields        []Field  `json:"auth_fields,omitempty"`
	AdvancedFields    []Field  `json:"advanced_fields,omitempty"`
}

var builtins = []Type{
	{
		ID:                "openai",
		DisplayName:       "OpenAI Compatible",
		Icon:              "openai",
		Description:       "Standard OpenAI-compatible chat and model discovery APIs.",
		DefaultAPIBase:    "https://api.openai.com/v1",
		SupportsDiscovery: true,
		Capabilities:      []string{"chat", "discovery"},
		AuthFields:        []Field{{Key: "api_key", Label: "API Key", Type: "password", Required: true, Secret: true}},
		AdvancedFields:    []Field{{Key: "api_base", Label: "API Base", Type: "text", Placeholder: "https://api.openai.com/v1"}},
	},
	{
		ID:                "anthropic",
		DisplayName:       "Anthropic",
		Icon:              "anthropic",
		Description:       "Claude provider with direct Anthropic credentials.",
		DefaultAPIBase:    "https://api.anthropic.com",
		SupportsDiscovery: true,
		Capabilities:      []string{"chat", "discovery"},
		AuthFields:        []Field{{Key: "api_key", Label: "API Key", Type: "password", Required: true, Secret: true}},
		AdvancedFields:    []Field{{Key: "api_base", Label: "API Base", Type: "text", Placeholder: "https://api.anthropic.com"}},
	},
	{
		ID:                "gemini",
		DisplayName:       "Gemini",
		Icon:              "gemini",
		Description:       "Google Gemini and compatible endpoints.",
		DefaultAPIBase:    "https://generativelanguage.googleapis.com",
		SupportsDiscovery: true,
		Capabilities:      []string{"chat", "discovery"},
		AuthFields:        []Field{{Key: "api_key", Label: "API Key", Type: "password", Required: true, Secret: true}},
	},
	{
		ID:                "openrouter",
		DisplayName:       "OpenRouter",
		Icon:              "openrouter",
		Description:       "OpenRouter aggregator with OpenAI-compatible routing.",
		DefaultAPIBase:    "https://openrouter.ai/api/v1",
		SupportsDiscovery: true,
		Capabilities:      []string{"chat", "discovery"},
		AuthFields:        []Field{{Key: "api_key", Label: "API Key", Type: "password", Required: true, Secret: true}},
	},
	{
		ID:                "ollama",
		DisplayName:       "Ollama",
		Icon:              "ollama",
		Description:       "Local Ollama server.",
		DefaultAPIBase:    "http://127.0.0.1:11434/v1",
		SupportsDiscovery: true,
		Capabilities:      []string{"chat", "discovery", "local"},
		AdvancedFields:    []Field{{Key: "api_base", Label: "API Base", Type: "text", Placeholder: "http://127.0.0.1:11434/v1"}},
	},
}

func List() []Type {
	items := append([]Type(nil), builtins...)
	sort.Slice(items, func(i, j int) bool { return items[i].DisplayName < items[j].DisplayName })
	return items
}

func Get(id string) (Type, bool) {
	for _, item := range builtins {
		if item.ID == id {
			return item, true
		}
	}
	return Type{}, false
}
