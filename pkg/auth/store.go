// Package auth provides authentication and credential management.
package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	credentialsFile = ".nekobot/auth.json"
)

// AuthCredential represents stored authentication credentials.
type AuthCredential struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	AccountID    string    `json:"account_id,omitempty"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
	Provider     string    `json:"provider"`
	AuthMethod   string    `json:"auth_method"` // "oauth", "token", "device_code"
}

// IsExpired checks if the credential has expired.
func (c *AuthCredential) IsExpired() bool {
	if c.ExpiresAt.IsZero() {
		return false // No expiry set
	}
	return time.Now().After(c.ExpiresAt)
}

// NeedsRefresh checks if the credential needs to be refreshed soon.
func (c *AuthCredential) NeedsRefresh() bool {
	if c.ExpiresAt.IsZero() || c.RefreshToken == "" {
		return false
	}
	// Refresh if expiring within 5 minutes
	return time.Until(c.ExpiresAt) < 5*time.Minute
}

// AuthStore manages credential storage.
type AuthStore struct {
	credentials map[string]*AuthCredential
	mu          sync.RWMutex
	filePath    string
}

// NewAuthStore creates a new auth store.
func NewAuthStore() (*AuthStore, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("getting home directory: %w", err)
	}

	filePath := filepath.Join(homeDir, credentialsFile)

	store := &AuthStore{
		credentials: make(map[string]*AuthCredential),
		filePath:    filePath,
	}

	// Load existing credentials
	if err := store.Load(); err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("loading credentials: %w", err)
		}
		// File doesn't exist yet, that's OK
	}

	return store, nil
}

// Load loads credentials from disk.
func (s *AuthStore) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &s.credentials)
}

// Save saves credentials to disk.
func (s *AuthStore) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Ensure directory exists
	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating auth directory: %w", err)
	}

	data, err := json.MarshalIndent(s.credentials, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling credentials: %w", err)
	}

	if err := os.WriteFile(s.filePath, data, 0600); err != nil {
		return fmt.Errorf("writing credentials file: %w", err)
	}

	return nil
}

// Get retrieves a credential for a provider.
func (s *AuthStore) Get(provider string) (*AuthCredential, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cred, exists := s.credentials[provider]
	return cred, exists
}

// Set stores a credential for a provider.
func (s *AuthStore) Set(provider string, cred *AuthCredential) error {
	s.mu.Lock()
	s.credentials[provider] = cred
	s.mu.Unlock()

	return s.Save()
}

// Delete removes a credential for a provider.
func (s *AuthStore) Delete(provider string) error {
	s.mu.Lock()
	delete(s.credentials, provider)
	s.mu.Unlock()

	return s.Save()
}

// List returns all stored credentials.
func (s *AuthStore) List() []*AuthCredential {
	s.mu.RLock()
	defer s.mu.RUnlock()

	creds := make([]*AuthCredential, 0, len(s.credentials))
	for _, cred := range s.credentials {
		creds = append(creds, cred)
	}
	return creds
}

// Clear removes all credentials.
func (s *AuthStore) Clear() error {
	s.mu.Lock()
	s.credentials = make(map[string]*AuthCredential)
	s.mu.Unlock()

	return s.Save()
}
