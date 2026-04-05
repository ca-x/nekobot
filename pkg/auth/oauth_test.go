package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestRefreshTokenPreservesExistingRefreshTokenWhenResponseOmitsIt(t *testing.T) {
	t.Parallel()

	const existingRefreshToken = "existing-refresh-token"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Helper()

		if r.Method != http.MethodPost {
			t.Fatalf("method = %q, want %q", r.Method, http.MethodPost)
		}

		if got := r.Header.Get("Content-Type"); got != "application/x-www-form-urlencoded" {
			t.Fatalf("content-type = %q, want %q", got, "application/x-www-form-urlencoded")
		}

		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm() error = %v", err)
		}

		want := url.Values{
			"grant_type":    []string{"refresh_token"},
			"refresh_token": []string{existingRefreshToken},
			"client_id":     []string{"client-id"},
		}

		if got := r.PostForm; !valuesEqual(got, want) {
			t.Fatalf("post form = %v, want %v", got, want)
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(OAuthTokenResponse{
			AccessToken: "new-access-token",
			ExpiresIn:   3600,
		}); err != nil {
			t.Fatalf("Encode() error = %v", err)
		}
	}))
	defer server.Close()

	cred, err := RefreshToken(OAuthProviderConfig{
		Provider: "openai",
		TokenURL: server.URL,
		ClientID: "client-id",
	}, existingRefreshToken)
	if err != nil {
		t.Fatalf("RefreshToken() error = %v", err)
	}

	if cred.AccessToken != "new-access-token" {
		t.Fatalf("AccessToken = %q, want %q", cred.AccessToken, "new-access-token")
	}

	if cred.RefreshToken != existingRefreshToken {
		t.Fatalf("RefreshToken = %q, want %q", cred.RefreshToken, existingRefreshToken)
	}
}

func TestRefreshTokenRejectsResponseWithoutAccessToken(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Helper()

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(OAuthTokenResponse{
			RefreshToken: "rotated-refresh-token",
			ExpiresIn:    3600,
		}); err != nil {
			t.Fatalf("Encode() error = %v", err)
		}
	}))
	defer server.Close()

	_, err := RefreshToken(OAuthProviderConfig{
		Provider: "openai",
		TokenURL: server.URL,
		ClientID: "client-id",
	}, "existing-refresh-token")
	if err == nil {
		t.Fatal("expected missing access token error")
	}
	if err.Error() != "token refresh response missing access_token" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func valuesEqual(got, want url.Values) bool {
	if len(got) != len(want) {
		return false
	}

	for key, wantValues := range want {
		gotValues, ok := got[key]
		if !ok {
			return false
		}
		if len(gotValues) != len(wantValues) {
			return false
		}
		for i := range wantValues {
			if gotValues[i] != wantValues[i] {
				return false
			}
		}
	}

	return true
}
