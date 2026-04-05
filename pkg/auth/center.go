package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

const credentialUserScopeGlobal = "global"

var (
	ErrCredentialNotFound         = errors.New("credential not found")
	ErrCredentialNotRefreshable   = errors.New("credential not refreshable")
	ErrCredentialUnsupportedScope = errors.New("credential user scope not supported")
)

type CredentialStatus string

const (
	CredentialStatusActive   CredentialStatus = "active"
	CredentialStatusExpiring CredentialStatus = "expiring"
	CredentialStatusExpired  CredentialStatus = "expired"
)

type CredentialRecord struct {
	UserScope  string
	Provider   string
	AccountID  string
	Credential AuthCredential
}

type CredentialQuery struct {
	UserScope string
	Provider  string
	AccountID string
}

type CredentialFilter struct {
	UserScope string
	Provider  string
}

type ValidationResult struct {
	Status CredentialStatus
	Record *CredentialRecord
}

type CredentialCenter struct {
	store     *AuthStore
	refreshFn func(OAuthProviderConfig, string) (*AuthCredential, error)
	nowFn     func() time.Time
}

func NewCredentialCenter(store *AuthStore) *CredentialCenter {
	return &CredentialCenter{
		store:     store,
		refreshFn: RefreshToken,
		nowFn:     time.Now,
	}
}

func (c *CredentialCenter) Put(_ context.Context, record CredentialRecord) error {
	if err := c.ensureGlobalScope(record.UserScope); err != nil {
		return err
	}
	provider := strings.TrimSpace(record.Provider)
	if provider == "" {
		provider = strings.TrimSpace(record.Credential.Provider)
	}
	if provider == "" {
		return fmt.Errorf("provider is required")
	}
	cred := record.Credential
	cred.Provider = provider
	if strings.TrimSpace(cred.AccountID) == "" {
		cred.AccountID = strings.TrimSpace(record.AccountID)
	}
	return c.store.Set(provider, &cred)
}

func (c *CredentialCenter) Get(_ context.Context, query CredentialQuery) (*CredentialRecord, error) {
	if err := c.ensureGlobalScope(query.UserScope); err != nil {
		return nil, err
	}
	provider := strings.TrimSpace(query.Provider)
	if provider == "" {
		return nil, fmt.Errorf("provider is required")
	}
	cred, ok := c.store.Get(provider)
	if !ok || cred == nil {
		return nil, ErrCredentialNotFound
	}
	if accountID := strings.TrimSpace(query.AccountID); accountID != "" && strings.TrimSpace(cred.AccountID) != accountID {
		return nil, ErrCredentialNotFound
	}
	return &CredentialRecord{
		UserScope:  credentialUserScopeGlobal,
		Provider:   provider,
		AccountID:  strings.TrimSpace(cred.AccountID),
		Credential: *cred,
	}, nil
}

func (c *CredentialCenter) List(_ context.Context, filter CredentialFilter) ([]CredentialRecord, error) {
	if err := c.ensureGlobalScope(filter.UserScope); err != nil {
		return nil, err
	}
	records := make([]CredentialRecord, 0)
	for _, cred := range c.store.List() {
		if cred == nil {
			continue
		}
		if provider := strings.TrimSpace(filter.Provider); provider != "" && strings.TrimSpace(cred.Provider) != provider {
			continue
		}
		records = append(records, CredentialRecord{
			UserScope:  credentialUserScopeGlobal,
			Provider:   strings.TrimSpace(cred.Provider),
			AccountID:  strings.TrimSpace(cred.AccountID),
			Credential: *cred,
		})
	}
	return records, nil
}

func (c *CredentialCenter) Validate(ctx context.Context, query CredentialQuery) (ValidationResult, error) {
	record, err := c.Get(ctx, query)
	if err != nil {
		return ValidationResult{}, err
	}
	return ValidationResult{
		Status: c.deriveStatus(record.Credential),
		Record: record,
	}, nil
}

func (c *CredentialCenter) Refresh(ctx context.Context, query CredentialQuery) (*CredentialRecord, error) {
	record, err := c.Get(ctx, query)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(record.Credential.RefreshToken) == "" {
		return nil, ErrCredentialNotRefreshable
	}
	cfg, err := GetOAuthConfig(record.Provider)
	if err != nil {
		return nil, fmt.Errorf("get oauth config for provider %s: %w", record.Provider, err)
	}
	cred, err := c.refreshFn(cfg, record.Credential.RefreshToken)
	if err != nil {
		return nil, err
	}
	refreshed := CredentialRecord{
		UserScope:  credentialUserScopeGlobal,
		Provider:   record.Provider,
		AccountID:  firstNonEmpty(strings.TrimSpace(cred.AccountID), strings.TrimSpace(record.AccountID)),
		Credential: *cred,
	}
	if err := c.Put(ctx, refreshed); err != nil {
		return nil, err
	}
	return c.Get(ctx, CredentialQuery{
		UserScope: credentialUserScopeGlobal,
		Provider:  refreshed.Provider,
		AccountID: refreshed.AccountID,
	})
}

func (c *CredentialCenter) Revoke(_ context.Context, query CredentialQuery) error {
	if err := c.ensureGlobalScope(query.UserScope); err != nil {
		return err
	}
	provider := strings.TrimSpace(query.Provider)
	if provider == "" {
		return fmt.Errorf("provider is required")
	}
	return c.store.Delete(provider)
}

func (c *CredentialCenter) deriveStatus(cred AuthCredential) CredentialStatus {
	if cred.ExpiresAt.IsZero() {
		return CredentialStatusActive
	}
	now := c.nowFn()
	if now.After(cred.ExpiresAt) {
		return CredentialStatusExpired
	}
	if cred.RefreshToken != "" && cred.ExpiresAt.Sub(now) < 5*time.Minute {
		return CredentialStatusExpiring
	}
	return CredentialStatusActive
}

func (c *CredentialCenter) ensureGlobalScope(scope string) error {
	scope = strings.TrimSpace(scope)
	if scope == "" || scope == credentialUserScopeGlobal {
		return nil
	}
	return ErrCredentialUnsupportedScope
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
