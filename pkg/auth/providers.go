package auth

// GetOAuthConfig returns OAuth configuration for a provider.
func GetOAuthConfig(provider string) (OAuthProviderConfig, error) {
	configs := map[string]OAuthProviderConfig{
		"openai": {
			Provider:              "openai",
			AuthorizeURL:          "https://auth.openai.com/authorize",
			TokenURL:              "https://auth.openai.com/oauth/token",
			DeviceAuthorizationURL: "https://auth.openai.com/oauth/device/code",
			ClientID:              "nanobot-cli", // This would need to be registered
			Scopes:                []string{"openid", "profile", "email"},
			RequiresPKCE:          true,
		},
		// Add more providers as needed
	}

	cfg, ok := configs[provider]
	if !ok {
		return OAuthProviderConfig{}, nil // Return empty config if not found
	}

	return cfg, nil
}

// SupportsOAuth checks if a provider supports OAuth authentication.
func SupportsOAuth(provider string) bool {
	cfg, _ := GetOAuthConfig(provider)
	return cfg.Provider != ""
}

// SupportsDeviceCode checks if a provider supports device code flow.
func SupportsDeviceCode(provider string) bool {
	cfg, _ := GetOAuthConfig(provider)
	return cfg.DeviceAuthorizationURL != ""
}
