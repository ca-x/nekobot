package config

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"nekobot/pkg/storage/ent"
	"nekobot/pkg/storage/ent/user"
)

const (
	adminCredSection  = "webui_auth"
	defaultTenantSlug = "default"
)

var (
	ErrAdminNotInitialized = errors.New("admin is not initialized")
	ErrUsernameAlreadyUsed = errors.New("username is already used")
)

// AdminCredential holds WebUI admin authentication data stored in the database.
type AdminCredential struct {
	Username     string `json:"username"`
	Nickname     string `json:"nickname"`
	PasswordHash string `json:"password_hash"`
	JWTSecret    string `json:"jwt_secret"`
}

// LoginUser describes a user loaded for authentication checks.
type LoginUser struct {
	ID           string
	Username     string
	Nickname     string
	Role         string
	Enabled      bool
	PasswordHash string
}

// AuthProfile is the authenticated profile embedded in API responses and JWT.
type AuthProfile struct {
	UserID     string
	Username   string
	Nickname   string
	Role       string
	TenantID   string
	TenantSlug string
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
	if client == nil {
		return nil, fmt.Errorf("ent client is nil")
	}
	ctx := context.Background()

	admin, err := client.User.Query().
		Where(user.RoleEQ("admin")).
		Order(ent.Desc(user.FieldCreatedAt)).
		First(ctx)
	if err == nil {
		profile, profileErr := BuildAuthProfileByUserID(ctx, client, admin.ID)
		if profileErr != nil {
			return nil, profileErr
		}
		secret, secretErr := getOrCreateJWTSecret(ctx, client)
		if secretErr != nil {
			return nil, secretErr
		}
		return &AdminCredential{
			Username:     profile.Username,
			Nickname:     profile.Nickname,
			PasswordHash: admin.PasswordHash,
			JWTSecret:    secret,
		}, nil
	}
	if err != nil && !ent.IsNotFound(err) {
		return nil, fmt.Errorf("query admin user: %w", err)
	}

	legacy, legacyErr := loadLegacyAdminCredential(ctx, client)
	if legacyErr != nil {
		return nil, legacyErr
	}
	return legacy, nil
}

// SaveAdminCredential persists the admin credential to the database.
func SaveAdminCredential(client *ent.Client, cred *AdminCredential) error {
	if client == nil {
		return fmt.Errorf("ent client is nil")
	}
	if cred == nil {
		return fmt.Errorf("admin credential is nil")
	}
	username := strings.TrimSpace(cred.Username)
	if username == "" {
		return fmt.Errorf("username is required")
	}
	if strings.TrimSpace(cred.PasswordHash) == "" {
		return fmt.Errorf("password hash is required")
	}

	ctx := context.Background()
	return withTx(ctx, client, func(tx *ent.Tx) error {
		tenantID, err := ensureDefaultTenantTx(ctx, tx)
		if err != nil {
			return err
		}

		adminUser, err := upsertAdminUserTx(ctx, tx, username, strings.TrimSpace(cred.Nickname), strings.TrimSpace(cred.PasswordHash))
		if err != nil {
			return err
		}

		if err := ensureMembershipTx(ctx, tx, adminUser.ID, tenantID, "owner"); err != nil {
			return err
		}

		if err := clearLegacyAdminCredentialSectionTx(ctx, tx); err != nil {
			return err
		}
		return setJWTSecretTx(ctx, tx, strings.TrimSpace(cred.JWTSecret))
	})
}

// AuthenticateUser validates username and password against users table.
func AuthenticateUser(ctx context.Context, client *ent.Client, username, password string) (*LoginUser, error) {
	if client == nil {
		return nil, fmt.Errorf("ent client is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	name := strings.TrimSpace(username)
	if name == "" {
		return nil, ErrAdminNotInitialized
	}
	usr, err := client.User.Query().Where(user.UsernameEQ(name)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrAdminNotInitialized
		}
		return nil, fmt.Errorf("query user %s: %w", name, err)
	}
	if !usr.Enabled || strings.TrimSpace(usr.PasswordHash) == "" {
		return nil, ErrAdminNotInitialized
	}
	if !CheckPassword(usr.PasswordHash, password) {
		return nil, ErrAdminNotInitialized
	}
	return &LoginUser{
		ID:           usr.ID,
		Username:     usr.Username,
		Nickname:     usr.Nickname,
		Role:         usr.Role,
		Enabled:      usr.Enabled,
		PasswordHash: usr.PasswordHash,
	}, nil
}

// UpdateUserPassword rotates password hash for an existing user.
func UpdateUserPassword(ctx context.Context, client *ent.Client, userID, passwordHash string) error {
	if client == nil {
		return fmt.Errorf("ent client is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	id := strings.TrimSpace(userID)
	if id == "" {
		return fmt.Errorf("user id is required")
	}
	hash := strings.TrimSpace(passwordHash)
	if hash == "" {
		return fmt.Errorf("password hash is required")
	}
	if _, err := client.User.UpdateOneID(id).SetPasswordHash(hash).Save(ctx); err != nil {
		if ent.IsNotFound(err) {
			return ErrAdminNotInitialized
		}
		return fmt.Errorf("update user password %s: %w", id, err)
	}
	return nil
}

// UpdateUserProfile updates username and nickname for an existing user.
func UpdateUserProfile(ctx context.Context, client *ent.Client, userID, username, nickname string) (*LoginUser, error) {
	if client == nil {
		return nil, fmt.Errorf("ent client is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	id := strings.TrimSpace(userID)
	if id == "" {
		return nil, fmt.Errorf("user id is required")
	}
	name := strings.TrimSpace(username)
	if name == "" {
		return nil, fmt.Errorf("username is required")
	}
	updated, err := client.User.UpdateOneID(id).
		SetUsername(name).
		SetNickname(strings.TrimSpace(nickname)).
		Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrAdminNotInitialized
		}
		if ent.IsConstraintError(err) {
			return nil, ErrUsernameAlreadyUsed
		}
		return nil, fmt.Errorf("update user profile %s: %w", id, err)
	}
	return &LoginUser{
		ID:           updated.ID,
		Username:     updated.Username,
		Nickname:     updated.Nickname,
		Role:         updated.Role,
		Enabled:      updated.Enabled,
		PasswordHash: updated.PasswordHash,
	}, nil
}

// RecordUserLogin stores the latest login timestamp.
func RecordUserLogin(ctx context.Context, client *ent.Client, userID string) error {
	if client == nil {
		return fmt.Errorf("ent client is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	id := strings.TrimSpace(userID)
	if id == "" {
		return fmt.Errorf("user id is required")
	}
	if _, err := client.User.UpdateOneID(id).SetLastLogin(time.Now()).Save(ctx); err != nil {
		if ent.IsNotFound(err) {
			return ErrAdminNotInitialized
		}
		return fmt.Errorf("record user login %s: %w", id, err)
	}
	return nil
}

// BuildAuthProfileByUsername loads auth profile by username.
func BuildAuthProfileByUsername(ctx context.Context, client *ent.Client, username string) (*AuthProfile, error) {
	if client == nil {
		return nil, fmt.Errorf("ent client is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	name := strings.TrimSpace(username)
	if name == "" {
		return nil, ErrAdminNotInitialized
	}
	usr, err := client.User.Query().Where(user.UsernameEQ(name)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrAdminNotInitialized
		}
		return nil, fmt.Errorf("query user %s: %w", name, err)
	}
	return BuildAuthProfileByUserID(ctx, client, usr.ID)
}

// BuildAuthProfileByUserID loads auth profile by user ID.
func BuildAuthProfileByUserID(ctx context.Context, client *ent.Client, userID string) (*AuthProfile, error) {
	if client == nil {
		return nil, fmt.Errorf("ent client is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	id := strings.TrimSpace(userID)
	if id == "" {
		return nil, ErrAdminNotInitialized
	}
	usr, err := client.User.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrAdminNotInitialized
		}
		return nil, fmt.Errorf("query user by id %s: %w", id, err)
	}
	if !usr.Enabled {
		return nil, ErrAdminNotInitialized
	}
	membership, tenantRec, err := getPrimaryMembership(ctx, client, usr.ID)
	if err != nil {
		return nil, err
	}
	role := strings.TrimSpace(membership.Role)
	if role == "" {
		role = strings.TrimSpace(usr.Role)
	}
	if role == "" {
		role = "member"
	}
	return &AuthProfile{
		UserID:     usr.ID,
		Username:   usr.Username,
		Nickname:   usr.Nickname,
		Role:       role,
		TenantID:   tenantRec.ID,
		TenantSlug: tenantRec.Slug,
	}, nil
}

// GetJWTSecret returns JWT secret from DB. Falls back to legacy section if needed.
func GetJWTSecret(client *ent.Client) (string, error) {
	if client == nil {
		return "", fmt.Errorf("ent client is nil")
	}
	secret, err := getJWTSecret(context.Background(), client)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(secret) == "" {
		return "", ErrAdminNotInitialized
	}
	return secret, nil
}

// RotateJWTSecret stores new secret in DB.
func RotateJWTSecret(client *ent.Client, secret string) error {
	if client == nil {
		return fmt.Errorf("ent client is nil")
	}
	ctx := context.Background()
	return withTx(ctx, client, func(tx *ent.Tx) error {
		return setJWTSecretTx(ctx, tx, strings.TrimSpace(secret))
	})
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
