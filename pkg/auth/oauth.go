package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// OAuthProviderConfig contains OAuth configuration for a provider.
type OAuthProviderConfig struct {
	Provider               string
	AuthorizeURL           string
	TokenURL               string
	DeviceAuthorizationURL string
	ClientID               string
	ClientSecret           string
	Scopes                 []string
	RequiresPKCE           bool
}

// OAuthTokenResponse represents the OAuth token response.
type OAuthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresIn    int    `json:"expires_in,omitempty"`
	TokenType    string `json:"token_type,omitempty"`
	AccountID    string `json:"account_id,omitempty"`
}

// DeviceCodeResponse represents the device authorization response.
type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// LoginBrowser performs OAuth login via browser with PKCE.
func LoginBrowser(cfg OAuthProviderConfig) (*AuthCredential, error) {
	var pkce *PKCEPair
	var err error

	if cfg.RequiresPKCE {
		pkce, err = GeneratePKCE()
		if err != nil {
			return nil, fmt.Errorf("generating PKCE: %w", err)
		}
	}

	state, err := generateState()
	if err != nil {
		return nil, fmt.Errorf("generating state: %w", err)
	}

	// Start local callback server
	redirectURI := "http://localhost:8080/callback"
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	stateCh := make(chan string, 1)

	server := &http.Server{
		Addr: ":8080",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/callback" {
				http.Error(w, "Not found", http.StatusNotFound)
				return
			}

			code := r.URL.Query().Get("code")
			receivedState := r.URL.Query().Get("state")
			errorParam := r.URL.Query().Get("error")

			if errorParam != "" {
				errCh <- fmt.Errorf("OAuth error: %s", errorParam)
				fmt.Fprintf(w, "Authentication failed: %s\nYou can close this window.", errorParam)
				return
			}

			if code == "" {
				errCh <- fmt.Errorf("no authorization code received")
				fmt.Fprintf(w, "Authentication failed: no code received\nYou can close this window.")
				return
			}

			codeCh <- code
			stateCh <- receivedState
			fmt.Fprintf(w, "Authentication successful! You can close this window and return to the CLI.")
		}),
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("callback server error: %w", err)
		}
	}()

	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(ctx)
	}()

	// Build authorization URL
	authURL := buildAuthorizeURL(cfg, pkce, state, redirectURI)

	fmt.Printf("Opening browser for authentication...\n")
	fmt.Printf("If the browser doesn't open, visit this URL:\n%s\n\n", authURL)

	// Open browser
	if err := openBrowser(authURL); err != nil {
		fmt.Printf("Could not open browser automatically: %v\n", err)
	}

	// Wait for callback
	var code, receivedState string
	select {
	case code = <-codeCh:
		receivedState = <-stateCh
	case err := <-errCh:
		return nil, err
	case <-time.After(5 * time.Minute):
		return nil, fmt.Errorf("authentication timeout")
	}

	// Verify state
	if receivedState != state {
		return nil, fmt.Errorf("state mismatch: potential CSRF attack")
	}

	// Exchange code for tokens
	var codeVerifier string
	if pkce != nil {
		codeVerifier = pkce.CodeVerifier
	}

	return exchangeCodeForTokens(cfg, code, codeVerifier, redirectURI)
}

// LoginDeviceCode performs OAuth login via device code flow.
func LoginDeviceCode(cfg OAuthProviderConfig) (*AuthCredential, error) {
	if cfg.DeviceAuthorizationURL == "" {
		return nil, fmt.Errorf("device authorization not supported by this provider")
	}

	// Request device code
	data := url.Values{}
	data.Set("client_id", cfg.ClientID)
	if len(cfg.Scopes) > 0 {
		data.Set("scope", strings.Join(cfg.Scopes, " "))
	}

	req, err := http.NewRequest("POST", cfg.DeviceAuthorizationURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("creating device code request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("requesting device code: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("device authorization failed: %s - %s", resp.Status, string(body))
	}

	var deviceResp DeviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&deviceResp); err != nil {
		return nil, fmt.Errorf("decoding device code response: %w", err)
	}

	// Display user code
	fmt.Printf("\n┌─────────────────────────────────────────┐\n")
	fmt.Printf("│ Please visit: %s\n", deviceResp.VerificationURI)
	fmt.Printf("│ Enter code: %s\n", deviceResp.UserCode)
	fmt.Printf("└─────────────────────────────────────────┘\n\n")
	fmt.Printf("Waiting for authentication...\n\n")

	// Poll for completion
	interval := deviceResp.Interval
	if interval == 0 {
		interval = 5 // Default 5 seconds
	}

	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	timeout := time.After(time.Duration(deviceResp.ExpiresIn) * time.Second)

	for {
		select {
		case <-timeout:
			return nil, fmt.Errorf("device code expired")
		case <-ticker.C:
			tokenResp, err := pollDeviceToken(cfg, deviceResp.DeviceCode)
			if err != nil {
				if strings.Contains(err.Error(), "authorization_pending") {
					continue // Keep waiting
				}
				if strings.Contains(err.Error(), "slow_down") {
					ticker.Reset(time.Duration(interval+5) * time.Second)
					continue
				}
				return nil, err
			}

			// Success!
			cred := &AuthCredential{
				AccessToken:  tokenResp.AccessToken,
				RefreshToken: tokenResp.RefreshToken,
				AccountID:    tokenResp.AccountID,
				Provider:     cfg.Provider,
				AuthMethod:   "device_code",
			}

			if tokenResp.ExpiresIn > 0 {
				cred.ExpiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
			}

			return cred, nil
		}
	}
}

// RefreshToken refreshes an access token using a refresh token.
func RefreshToken(cfg OAuthProviderConfig, refreshToken string) (*AuthCredential, error) {
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", refreshToken)
	data.Set("client_id", cfg.ClientID)
	if cfg.ClientSecret != "" {
		data.Set("client_secret", cfg.ClientSecret)
	}

	req, err := http.NewRequest("POST", cfg.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("creating refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("refreshing token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token refresh failed: %s - %s", resp.Status, string(body))
	}

	var tokenResp OAuthTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("decoding token response: %w", err)
	}

	cred := &AuthCredential{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		AccountID:    tokenResp.AccountID,
		Provider:     cfg.Provider,
		AuthMethod:   "oauth",
	}

	if tokenResp.ExpiresIn > 0 {
		cred.ExpiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	}

	return cred, nil
}

// Helper functions

func buildAuthorizeURL(cfg OAuthProviderConfig, pkce *PKCEPair, state, redirectURI string) string {
	params := url.Values{}
	params.Set("client_id", cfg.ClientID)
	params.Set("response_type", "code")
	params.Set("redirect_uri", redirectURI)
	params.Set("state", state)

	if len(cfg.Scopes) > 0 {
		params.Set("scope", strings.Join(cfg.Scopes, " "))
	}

	if pkce != nil {
		params.Set("code_challenge", pkce.CodeChallenge)
		params.Set("code_challenge_method", "S256")
	}

	return cfg.AuthorizeURL + "?" + params.Encode()
}

func exchangeCodeForTokens(cfg OAuthProviderConfig, code, codeVerifier, redirectURI string) (*AuthCredential, error) {
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", redirectURI)
	data.Set("client_id", cfg.ClientID)

	if cfg.ClientSecret != "" {
		data.Set("client_secret", cfg.ClientSecret)
	}

	if codeVerifier != "" {
		data.Set("code_verifier", codeVerifier)
	}

	req, err := http.NewRequest("POST", cfg.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("creating token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("exchanging code for tokens: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token exchange failed: %s - %s", resp.Status, string(body))
	}

	var tokenResp OAuthTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("decoding token response: %w", err)
	}

	cred := &AuthCredential{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		AccountID:    tokenResp.AccountID,
		Provider:     cfg.Provider,
		AuthMethod:   "oauth",
	}

	if tokenResp.ExpiresIn > 0 {
		cred.ExpiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	}

	return cred, nil
}

func pollDeviceToken(cfg OAuthProviderConfig, deviceCode string) (*OAuthTokenResponse, error) {
	data := url.Values{}
	data.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
	data.Set("device_code", deviceCode)
	data.Set("client_id", cfg.ClientID)

	req, err := http.NewRequest("POST", cfg.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("creating poll request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("polling for token: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		// Check for pending or slow_down
		var errorResp struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(body, &errorResp); err == nil {
			return nil, errors.New(errorResp.Error)
		}
		return nil, fmt.Errorf("token poll failed: %s - %s", resp.Status, string(body))
	}

	var tokenResp OAuthTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("decoding token response: %w", err)
	}

	return &tokenResp, nil
}

func openBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		return fmt.Errorf("unsupported platform")
	}

	return cmd.Start()
}
