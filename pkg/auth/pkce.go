package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

// PKCEPair contains PKCE code verifier and challenge.
type PKCEPair struct {
	CodeVerifier  string
	CodeChallenge string
}

// GeneratePKCE generates a PKCE code verifier and challenge pair.
func GeneratePKCE() (*PKCEPair, error) {
	// Generate random 32-byte verifier
	verifier := make([]byte, 32)
	if _, err := rand.Read(verifier); err != nil {
		return nil, fmt.Errorf("generating code verifier: %w", err)
	}

	// Base64 URL encode verifier
	codeVerifier := base64.RawURLEncoding.EncodeToString(verifier)

	// Generate SHA256 challenge from verifier
	h := sha256.New()
	h.Write([]byte(codeVerifier))
	codeChallenge := base64.RawURLEncoding.EncodeToString(h.Sum(nil))

	return &PKCEPair{
		CodeVerifier:  codeVerifier,
		CodeChallenge: codeChallenge,
	}, nil
}

// generateState generates a random state parameter for OAuth.
func generateState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating state: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
