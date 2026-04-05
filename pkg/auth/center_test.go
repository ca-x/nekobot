package auth

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestCredentialCenterStoresLegacyAdapterRecords(t *testing.T) {
	store := &AuthStore{
		credentials: make(map[string]*AuthCredential),
		filePath:    filepath.Join(t.TempDir(), "auth.json"),
	}
	center := NewCredentialCenter(store)

	record := CredentialRecord{
		UserScope: "global",
		Provider:  "openai",
		AccountID: "acct-1",
		Credential: AuthCredential{
			AccessToken:  "access-1",
			RefreshToken: "refresh-1",
			AccountID:    "acct-1",
			Provider:     "openai",
			AuthMethod:   "oauth",
		},
	}

	if err := center.Put(context.Background(), record); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	stored, ok := store.Get("openai")
	if !ok || stored == nil {
		t.Fatal("expected legacy store write-through")
	}
	if stored.AccessToken != "access-1" || stored.RefreshToken != "refresh-1" || stored.AccountID != "acct-1" {
		t.Fatalf("unexpected stored credential: %+v", stored)
	}
}

func TestCredentialCenterRefreshesAndPersistsCredential(t *testing.T) {
	store := &AuthStore{
		credentials: map[string]*AuthCredential{
			"openai": {
				AccessToken:  "access-old",
				RefreshToken: "refresh-old",
				AccountID:    "acct-1",
				Provider:     "openai",
				AuthMethod:   "oauth",
				ExpiresAt:    time.Now().Add(-time.Minute),
			},
		},
		filePath: filepath.Join(t.TempDir(), "auth.json"),
	}
	center := NewCredentialCenter(store)
	center.refreshFn = func(cfg OAuthProviderConfig, refreshToken string) (*AuthCredential, error) {
		if cfg.Provider != "openai" {
			t.Fatalf("expected provider openai, got %q", cfg.Provider)
		}
		if refreshToken != "refresh-old" {
			t.Fatalf("expected refresh token refresh-old, got %q", refreshToken)
		}
		return &AuthCredential{
			AccessToken:  "access-new",
			RefreshToken: "refresh-new",
			AccountID:    "acct-1",
			Provider:     "openai",
			AuthMethod:   "oauth",
			ExpiresAt:    time.Now().Add(time.Hour),
		}, nil
	}

	record, err := center.Refresh(context.Background(), CredentialQuery{
		UserScope: "global",
		Provider:  "openai",
		AccountID: "acct-1",
	})
	if err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}
	if record.Credential.AccessToken != "access-new" || record.Credential.RefreshToken != "refresh-new" {
		t.Fatalf("unexpected refreshed record: %+v", record)
	}

	stored, ok := store.Get("openai")
	if !ok || stored == nil {
		t.Fatal("expected refreshed credential persisted")
	}
	if stored.AccessToken != "access-new" || stored.RefreshToken != "refresh-new" {
		t.Fatalf("unexpected persisted refreshed credential: %+v", stored)
	}
}

func TestCredentialCenterDerivesLifecycleState(t *testing.T) {
	now := time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		cred *AuthCredential
		want CredentialStatus
	}{
		{
			name: "active without expiry",
			cred: &AuthCredential{
				AccessToken: "access-1",
				Provider:    "openai",
				AuthMethod:  "oauth",
			},
			want: CredentialStatusActive,
		},
		{
			name: "expiring soon",
			cred: &AuthCredential{
				AccessToken:  "access-1",
				RefreshToken: "refresh-1",
				Provider:     "openai",
				AuthMethod:   "oauth",
				ExpiresAt:    now.Add(4 * time.Minute),
			},
			want: CredentialStatusExpiring,
		},
		{
			name: "expired",
			cred: &AuthCredential{
				AccessToken:  "access-1",
				RefreshToken: "refresh-1",
				Provider:     "openai",
				AuthMethod:   "oauth",
				ExpiresAt:    now.Add(-time.Minute),
			},
			want: CredentialStatusExpired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &AuthStore{
				credentials: map[string]*AuthCredential{
					"openai": tt.cred,
				},
				filePath: filepath.Join(t.TempDir(), "auth.json"),
			}
			center := NewCredentialCenter(store)
			center.nowFn = func() time.Time { return now }

			result, err := center.Validate(context.Background(), CredentialQuery{
				UserScope: "global",
				Provider:  "openai",
			})
			if err != nil {
				t.Fatalf("Validate failed: %v", err)
			}
			if result.Status != tt.want {
				t.Fatalf("expected status %q, got %q", tt.want, result.Status)
			}
		})
	}
}

func TestCredentialCenterRevokeRemovesLegacyRecord(t *testing.T) {
	store := &AuthStore{
		credentials: map[string]*AuthCredential{
			"openai": {
				AccessToken: "access-1",
				Provider:    "openai",
				AuthMethod:  "oauth",
			},
		},
		filePath: filepath.Join(t.TempDir(), "auth.json"),
	}
	center := NewCredentialCenter(store)

	if err := center.Revoke(context.Background(), CredentialQuery{
		UserScope: "global",
		Provider:  "openai",
	}); err != nil {
		t.Fatalf("Revoke failed: %v", err)
	}

	if _, ok := store.Get("openai"); ok {
		t.Fatalf("expected credential removal after revoke")
	}
}

func TestCredentialCenterRefreshRejectsCredentialWithoutRefreshToken(t *testing.T) {
	store := &AuthStore{
		credentials: map[string]*AuthCredential{
			"openai": {
				AccessToken: "access-1",
				Provider:    "openai",
				AuthMethod:  "oauth",
			},
		},
		filePath: filepath.Join(t.TempDir(), "auth.json"),
	}
	center := NewCredentialCenter(store)

	_, err := center.Refresh(context.Background(), CredentialQuery{
		UserScope: "global",
		Provider:  "openai",
	})
	if err == nil {
		t.Fatal("expected refresh failure without refresh token")
	}
	if !errors.Is(err, ErrCredentialNotRefreshable) {
		t.Fatalf("expected ErrCredentialNotRefreshable, got %v", err)
	}
}
