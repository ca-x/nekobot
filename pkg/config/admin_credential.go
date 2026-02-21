package config

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"golang.org/x/crypto/bcrypt"

	"nekobot/pkg/storage/ent"
)

const adminCredSection = "webui_auth"

// AdminCredential holds WebUI admin authentication data stored in the database.
type AdminCredential struct {
	Username     string `json:"username"`
	Nickname     string `json:"nickname"`
	PasswordHash string `json:"password_hash"`
	JWTSecret    string `json:"jwt_secret"`
}

// HashPassword returns a bcrypt hash of the plaintext password.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

// CheckPassword reports whether the plaintext password matches the bcrypt hash.
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// GenerateJWTSecret returns a cryptographically random 32-byte hex string.
func GenerateJWTSecret() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// LoadAdminCredential reads the admin credential from the database.
// Returns nil (no error) when no credential has been stored yet.
func LoadAdminCredential(client *ent.Client) (*AdminCredential, error) {
	ctx := context.Background()
	payload, exists, err := loadSectionPayload(ctx, client, adminCredSection)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}
	var cred AdminCredential
	if err := json.Unmarshal(payload, &cred); err != nil {
		return nil, fmt.Errorf("decode admin credential: %w", err)
	}
	return &cred, nil
}

// SaveAdminCredential persists the admin credential to the database.
func SaveAdminCredential(client *ent.Client, cred *AdminCredential) error {
	payload, err := json.Marshal(cred)
	if err != nil {
		return fmt.Errorf("encode admin credential: %w", err)
	}
	return upsertSectionPayload(context.Background(), client, adminCredSection, payload)
}

// LoadAdminCredentialFromConfig opens the runtime DB using the given config,
// loads the admin credential, and closes the client. Suitable for CLI commands
// that don't run the full fx container.
func LoadAdminCredentialFromConfig(cfg *Config) (*AdminCredential, error) {
	client, err := openRuntimeConfigClient(cfg)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	return LoadAdminCredential(client)
}

// SaveAdminCredentialFromConfig opens the runtime DB using the given config,
// saves the admin credential, and closes the client. Suitable for CLI commands
// that don't run the full fx container.
func SaveAdminCredentialFromConfig(cfg *Config, cred *AdminCredential) error {
	client, err := openRuntimeConfigClient(cfg)
	if err != nil {
		return err
	}
	defer client.Close()
	return SaveAdminCredential(client, cred)
}
