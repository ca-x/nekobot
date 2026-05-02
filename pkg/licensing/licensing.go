package licensing

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"

	"nekobot/pkg/storage/ent"
	"nekobot/pkg/storage/ent/configsection"
	"nekobot/pkg/storage/ent/user"
)

const (
	FreeUserLimit = 2

	installSection = "server_install"
	licenseSection = "server_license"
)

var (
	ErrPublicKeyUnavailable = errors.New("license public key is not configured")
	ErrLicenseRequired      = errors.New("license required")
	ErrInvalidLicense       = errors.New("invalid license")
	ErrUserLimitReached     = errors.New("user limit reached")

	// PublicKeyBase64 may be set by release builds with:
	// -ldflags "-X nekobot/pkg/licensing.PublicKeyBase64=<base64-ed25519-public-key>"
	PublicKeyBase64 string
)

type installPayload struct {
	InstallID string `json:"install_id"`
	CreatedAt string `json:"created_at"`
}

type storedLicensePayload struct {
	LicenseJSON string `json:"license_json"`
	ImportedAt  string `json:"imported_at"`
}

type LicensePayload struct {
	Version   int      `json:"version"`
	LicenseID string   `json:"license_id"`
	Subject   string   `json:"subject,omitempty"`
	InstallID string   `json:"install_id"`
	MaxUsers  int      `json:"max_users"`
	IssuedAt  string   `json:"issued_at"`
	ExpiresAt string   `json:"expires_at,omitempty"`
	Features  []string `json:"features,omitempty"`
}

type LicenseFile struct {
	Version   int            `json:"version"`
	Alg       string         `json:"alg"`
	Payload   LicensePayload `json:"payload"`
	Signature string         `json:"signature"`
}

type Status struct {
	InstallID          string `json:"install_id"`
	Licensed           bool   `json:"licensed"`
	State              string `json:"state"`
	MaxUsers           int    `json:"max_users"`
	FreeUserLimit      int    `json:"free_user_limit"`
	EnabledUserCount   int    `json:"enabled_user_count"`
	RemainingUserSlots int    `json:"remaining_user_slots"`
	LicenseID          string `json:"license_id,omitempty"`
	Subject            string `json:"subject,omitempty"`
	ExpiresAt          string `json:"expires_at,omitempty"`
	Error              string `json:"error,omitempty"`
}

type GenerateOptions struct {
	LicenseID string
	Subject   string
	InstallID string
	MaxUsers  int
	IssuedAt  time.Time
	ExpiresAt *time.Time
	Features  []string
}

func EnsureInstallID(ctx context.Context, client *ent.Client) (string, error) {
	if client == nil {
		return "", fmt.Errorf("ent client is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	rec, err := client.ConfigSection.Query().Where(configsection.SectionEQ(installSection)).Only(ctx)
	if err == nil {
		var payload installPayload
		if json.Unmarshal([]byte(rec.PayloadJSON), &payload) == nil {
			if id := strings.TrimSpace(payload.InstallID); id != "" {
				return id, nil
			}
		}
	}
	if err != nil && !ent.IsNotFound(err) {
		return "", fmt.Errorf("load install id: %w", err)
	}

	id := uuid.NewString()
	payload, err := json.Marshal(installPayload{InstallID: id, CreatedAt: time.Now().UTC().Format(time.RFC3339)})
	if err != nil {
		return "", fmt.Errorf("encode install id: %w", err)
	}
	if rec != nil {
		if _, err := client.ConfigSection.UpdateOneID(rec.ID).SetPayloadJSON(string(payload)).Save(ctx); err != nil {
			return "", fmt.Errorf("repair install id: %w", err)
		}
		return id, nil
	}
	_, err = client.ConfigSection.Create().
		SetSection(installSection).
		SetPayloadJSON(string(payload)).
		Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return EnsureInstallID(ctx, client)
		}
		return "", fmt.Errorf("save install id: %w", err)
	}
	return id, nil
}

func EnabledUserCount(ctx context.Context, client *ent.Client) (int, error) {
	if client == nil {
		return 0, fmt.Errorf("ent client is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	count, err := client.User.Query().Where(user.EnabledEQ(true)).Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("count enabled users: %w", err)
	}
	return count, nil
}

func StatusForClient(ctx context.Context, client *ent.Client) (*Status, error) {
	if client == nil {
		return nil, fmt.Errorf("ent client is nil")
	}
	installID, err := EnsureInstallID(ctx, client)
	if err != nil {
		return nil, err
	}
	enabledCount, err := EnabledUserCount(ctx, client)
	if err != nil {
		return nil, err
	}

	status := &Status{
		InstallID:        installID,
		State:            "free",
		MaxUsers:         FreeUserLimit,
		FreeUserLimit:    FreeUserLimit,
		EnabledUserCount: enabledCount,
	}

	raw, ok, err := loadStoredLicense(ctx, client)
	if err != nil {
		return nil, err
	}
	if ok {
		file, verifyErr := VerifyLicense(raw, installID, time.Now().UTC(), configuredPublicKey())
		if verifyErr != nil {
			status.State = "invalid"
			status.Error = verifyErr.Error()
		} else {
			status.Licensed = true
			status.State = "licensed"
			status.MaxUsers = file.Payload.MaxUsers
			status.LicenseID = file.Payload.LicenseID
			status.Subject = file.Payload.Subject
			status.ExpiresAt = file.Payload.ExpiresAt
		}
	}

	status.RemainingUserSlots = status.MaxUsers - enabledCount
	if status.RemainingUserSlots < 0 {
		status.RemainingUserSlots = 0
	}
	return status, nil
}

func EffectiveMaxUsers(ctx context.Context, client *ent.Client) (int, error) {
	status, err := StatusForClient(ctx, client)
	if err != nil {
		return 0, err
	}
	if status.MaxUsers <= 0 {
		return FreeUserLimit, nil
	}
	return status.MaxUsers, nil
}

func CheckCanEnableAdditionalUsers(ctx context.Context, client *ent.Client, additional int) (*Status, error) {
	if additional <= 0 {
		additional = 1
	}
	status, err := StatusForClient(ctx, client)
	if err != nil {
		return nil, err
	}
	if status.EnabledUserCount+additional > status.MaxUsers {
		return status, ErrUserLimitReached
	}
	return status, nil
}

func ImportLicense(ctx context.Context, client *ent.Client, raw string) (*Status, error) {
	if client == nil {
		return nil, fmt.Errorf("ent client is nil")
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("%w: license payload is empty", ErrInvalidLicense)
	}
	installID, err := EnsureInstallID(ctx, client)
	if err != nil {
		return nil, err
	}
	if _, err := VerifyLicense(raw, installID, time.Now().UTC(), configuredPublicKey()); err != nil {
		return nil, err
	}

	payload, err := json.Marshal(storedLicensePayload{
		LicenseJSON: raw,
		ImportedAt:  time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return nil, fmt.Errorf("encode license: %w", err)
	}
	rec, err := client.ConfigSection.Query().Where(configsection.SectionEQ(licenseSection)).Only(ctx)
	if err != nil {
		if !ent.IsNotFound(err) {
			return nil, fmt.Errorf("load stored license: %w", err)
		}
		_, err = client.ConfigSection.Create().SetSection(licenseSection).SetPayloadJSON(string(payload)).Save(ctx)
		if err != nil {
			if ent.IsConstraintError(err) {
				_, updateErr := client.ConfigSection.Update().
					Where(configsection.SectionEQ(licenseSection)).
					SetPayloadJSON(string(payload)).
					Save(ctx)
				if updateErr == nil {
					return StatusForClient(ctx, client)
				}
			}
			return nil, fmt.Errorf("save license: %w", err)
		}
		return StatusForClient(ctx, client)
	}
	if _, err := client.ConfigSection.UpdateOneID(rec.ID).SetPayloadJSON(string(payload)).Save(ctx); err != nil {
		return nil, fmt.Errorf("update license: %w", err)
	}
	return StatusForClient(ctx, client)
}

func GenerateLicense(opts GenerateOptions, privateKey ed25519.PrivateKey) (*LicenseFile, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid ed25519 private key")
	}
	installID := strings.TrimSpace(opts.InstallID)
	if installID == "" {
		return nil, fmt.Errorf("install id is required")
	}
	maxUsers := opts.MaxUsers
	if maxUsers <= FreeUserLimit {
		maxUsers = FreeUserLimit + 1
	}
	issuedAt := opts.IssuedAt
	if issuedAt.IsZero() {
		issuedAt = time.Now().UTC()
	}
	licenseID := strings.TrimSpace(opts.LicenseID)
	if licenseID == "" {
		licenseID = uuid.NewString()
	}
	payload := LicensePayload{
		Version:   1,
		LicenseID: licenseID,
		Subject:   strings.TrimSpace(opts.Subject),
		InstallID: installID,
		MaxUsers:  maxUsers,
		IssuedAt:  issuedAt.UTC().Format(time.RFC3339),
		Features:  cleanFeatures(opts.Features),
	}
	if opts.ExpiresAt != nil && !opts.ExpiresAt.IsZero() {
		payload.ExpiresAt = opts.ExpiresAt.UTC().Format(time.RFC3339)
	}
	canonical, err := canonicalPayload(payload)
	if err != nil {
		return nil, err
	}
	sig := ed25519.Sign(privateKey, canonical)
	return &LicenseFile{
		Version:   1,
		Alg:       "ed25519",
		Payload:   payload,
		Signature: base64.StdEncoding.EncodeToString(sig),
	}, nil
}

func MarshalLicense(file *LicenseFile) (string, error) {
	if file == nil {
		return "", fmt.Errorf("license is nil")
	}
	raw, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal license: %w", err)
	}
	return string(raw), nil
}

func VerifyLicense(raw, installID string, now time.Time, publicKey ed25519.PublicKey) (*LicenseFile, error) {
	if len(publicKey) != ed25519.PublicKeySize {
		return nil, ErrPublicKeyUnavailable
	}
	var file LicenseFile
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &file); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidLicense, err)
	}
	if file.Version != 1 || strings.TrimSpace(file.Alg) != "ed25519" {
		return nil, fmt.Errorf("%w: unsupported license format", ErrInvalidLicense)
	}
	if strings.TrimSpace(file.Payload.InstallID) != strings.TrimSpace(installID) {
		return nil, fmt.Errorf("%w: install_id mismatch", ErrInvalidLicense)
	}
	if file.Payload.MaxUsers <= FreeUserLimit {
		return nil, fmt.Errorf("%w: max_users must exceed free limit", ErrInvalidLicense)
	}
	if strings.TrimSpace(file.Payload.IssuedAt) == "" {
		return nil, fmt.Errorf("%w: issued_at is required", ErrInvalidLicense)
	}
	if file.Payload.ExpiresAt != "" {
		expiresAt, err := time.Parse(time.RFC3339, file.Payload.ExpiresAt)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid expires_at", ErrInvalidLicense)
		}
		if !now.IsZero() && now.After(expiresAt) {
			return nil, fmt.Errorf("%w: expired", ErrInvalidLicense)
		}
	}
	sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(file.Signature))
	if err != nil || len(sig) != ed25519.SignatureSize {
		return nil, fmt.Errorf("%w: invalid signature encoding", ErrInvalidLicense)
	}
	canonical, err := canonicalPayload(file.Payload)
	if err != nil {
		return nil, err
	}
	if !ed25519.Verify(publicKey, canonical, sig) {
		return nil, fmt.Errorf("%w: signature check failed", ErrInvalidLicense)
	}
	return &file, nil
}

func GenerateKeyPair() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	return ed25519.GenerateKey(rand.Reader)
}

func ParsePublicKey(raw string) (ed25519.PublicKey, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, ErrPublicKeyUnavailable
	}
	if block, _ := pem.Decode([]byte(raw)); block != nil {
		key, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse public key pem: %w", err)
		}
		pub, ok := key.(ed25519.PublicKey)
		if !ok || len(pub) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("public key is not ed25519")
		}
		return pub, nil
	}
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("parse public key base64: %w", err)
	}
	if len(decoded) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid ed25519 public key length")
	}
	return ed25519.PublicKey(decoded), nil
}

func ParsePrivateKey(raw string) (ed25519.PrivateKey, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("private key is required")
	}
	if block, _ := pem.Decode([]byte(raw)); block != nil {
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse private key pem: %w", err)
		}
		priv, ok := key.(ed25519.PrivateKey)
		if !ok || len(priv) != ed25519.PrivateKeySize {
			return nil, fmt.Errorf("private key is not ed25519")
		}
		return priv, nil
	}
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("parse private key base64: %w", err)
	}
	if len(decoded) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid ed25519 private key length")
	}
	return ed25519.PrivateKey(decoded), nil
}

func EncodePublicKeyPEM(publicKey ed25519.PublicKey) ([]byte, error) {
	der, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return nil, fmt.Errorf("marshal public key: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}), nil
}

func EncodePrivateKeyPEM(privateKey ed25519.PrivateKey) ([]byte, error) {
	der, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("marshal private key: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), nil
}

func configuredPublicKey() ed25519.PublicKey {
	raw := strings.TrimSpace(os.Getenv("NEKOBOT_LICENSE_PUBLIC_KEY"))
	if raw == "" {
		raw = strings.TrimSpace(PublicKeyBase64)
	}
	pub, err := ParsePublicKey(raw)
	if err != nil {
		return nil
	}
	return pub
}

func loadStoredLicense(ctx context.Context, client *ent.Client) (string, bool, error) {
	rec, err := client.ConfigSection.Query().Where(configsection.SectionEQ(licenseSection)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("load stored license: %w", err)
	}
	var payload storedLicensePayload
	if err := json.Unmarshal([]byte(rec.PayloadJSON), &payload); err != nil {
		return "", false, fmt.Errorf("decode stored license: %w", err)
	}
	if strings.TrimSpace(payload.LicenseJSON) == "" {
		return "", false, nil
	}
	return payload.LicenseJSON, true, nil
}

func canonicalPayload(payload LicensePayload) ([]byte, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("canonicalize license payload: %w", err)
	}
	return raw, nil
}

func cleanFeatures(features []string) []string {
	out := make([]string, 0, len(features))
	seen := map[string]struct{}{}
	for _, feature := range features {
		feature = strings.TrimSpace(feature)
		if feature == "" {
			continue
		}
		if _, ok := seen[feature]; ok {
			continue
		}
		seen[feature] = struct{}{}
		out = append(out, feature)
	}
	return out
}
