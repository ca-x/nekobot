package config

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"nekobot/pkg/storage/ent"
	"nekobot/pkg/storage/ent/configsection"
	"nekobot/pkg/storage/ent/membership"
	"nekobot/pkg/storage/ent/tenant"
	"nekobot/pkg/storage/ent/user"
)

type jwtSecretPayload struct {
	JWTSecret string `json:"jwt_secret"`
}

func withTx(ctx context.Context, client *ent.Client, fn func(*ent.Tx) error) error {
	tx, err := client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("start transaction: %w", err)
	}
	if err := fn(tx); err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return fmt.Errorf("rollback transaction: %w", err)
		}
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

func ensureDefaultTenantTx(ctx context.Context, tx *ent.Tx) (string, error) {
	rec, err := tx.Tenant.Query().Where(tenant.SlugEQ(defaultTenantSlug)).Only(ctx)
	if err == nil {
		if !rec.Enabled {
			rec, err = tx.Tenant.UpdateOneID(rec.ID).SetEnabled(true).Save(ctx)
			if err != nil {
				return "", fmt.Errorf("enable default tenant: %w", err)
			}
		}
		return rec.ID, nil
	}
	if !ent.IsNotFound(err) {
		return "", fmt.Errorf("query default tenant: %w", err)
	}
	rec, err = tx.Tenant.Create().
		SetSlug(defaultTenantSlug).
		SetName("Default Tenant").
		SetEnabled(true).
		Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			rec, retryErr := tx.Tenant.Query().Where(tenant.SlugEQ(defaultTenantSlug)).Only(ctx)
			if retryErr == nil {
				return rec.ID, nil
			}
		}
		return "", fmt.Errorf("create default tenant: %w", err)
	}
	return rec.ID, nil
}

func upsertAdminUserTx(ctx context.Context, tx *ent.Tx, username, nickname, passwordHash string) (*ent.User, error) {
	rec, err := tx.User.Query().Where(user.UsernameEQ(username)).Only(ctx)
	if err == nil {
		rec, err = tx.User.UpdateOneID(rec.ID).
			SetNickname(nickname).
			SetPasswordHash(passwordHash).
			SetRole("admin").
			SetEnabled(true).
			Save(ctx)
		if err != nil {
			return nil, fmt.Errorf("update admin user: %w", err)
		}
		return rec, nil
	}
	if !ent.IsNotFound(err) {
		return nil, fmt.Errorf("query user %s: %w", username, err)
	}
	rec, err = tx.User.Create().
		SetUsername(username).
		SetNickname(nickname).
		SetPasswordHash(passwordHash).
		SetRole("admin").
		SetEnabled(true).
		Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			rec, retryErr := tx.User.Query().Where(user.UsernameEQ(username)).Only(ctx)
			if retryErr == nil {
				return rec, nil
			}
		}
		return nil, fmt.Errorf("create admin user: %w", err)
	}
	return rec, nil
}

func ensureMembershipTx(ctx context.Context, tx *ent.Tx, userID, tenantID, role string) error {
	rec, err := tx.Membership.Query().
		Where(
			membership.UserIDEQ(userID),
			membership.TenantIDEQ(tenantID),
		).
		Only(ctx)
	if err == nil {
		_, err = tx.Membership.UpdateOneID(rec.ID).
			SetRole(strings.TrimSpace(role)).
			SetEnabled(true).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("update membership: %w", err)
		}
		return nil
	}
	if !ent.IsNotFound(err) {
		return fmt.Errorf("query membership: %w", err)
	}
	_, err = tx.Membership.Create().
		SetUserID(userID).
		SetTenantID(tenantID).
		SetRole(strings.TrimSpace(role)).
		SetEnabled(true).
		Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil
		}
		return fmt.Errorf("create membership: %w", err)
	}
	return nil
}

func getPrimaryMembership(ctx context.Context, client *ent.Client, userID string) (*ent.Membership, *ent.Tenant, error) {
	rec, err := client.Membership.Query().
		Where(
			membership.UserIDEQ(userID),
			membership.EnabledEQ(true),
		).
		Order(ent.Desc(membership.FieldCreatedAt)).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil, ErrAdminNotInitialized
		}
		return nil, nil, fmt.Errorf("query membership for user %s: %w", userID, err)
	}
	tenantRec, err := client.Tenant.Get(ctx, rec.TenantID)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil, ErrAdminNotInitialized
		}
		return nil, nil, fmt.Errorf("query tenant %s: %w", rec.TenantID, err)
	}
	if !tenantRec.Enabled {
		return nil, nil, ErrAdminNotInitialized
	}
	return rec, tenantRec, nil
}

func clearLegacyAdminCredentialSectionTx(ctx context.Context, tx *ent.Tx) error {
	_, err := tx.ConfigSection.Delete().Where(configsection.SectionEQ(adminCredSection)).Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete legacy admin credential section: %w", err)
	}
	return nil
}

func getJWTSecret(ctx context.Context, client *ent.Client) (string, error) {
	payload, exists, err := loadSectionPayload(ctx, client, adminCredSection)
	if err != nil {
		return "", err
	}
	if exists {
		secret := parseJWTSecretPayload(payload)
		if secret != "" {
			return secret, nil
		}
	}
	legacy, legacyErr := loadLegacyAdminCredential(ctx, client)
	if legacyErr != nil {
		return "", legacyErr
	}
	if legacy != nil {
		return strings.TrimSpace(legacy.JWTSecret), nil
	}
	return "", nil
}

func getOrCreateJWTSecret(ctx context.Context, client *ent.Client) (string, error) {
	secret, err := getJWTSecret(ctx, client)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(secret) != "" {
		return strings.TrimSpace(secret), nil
	}
	secret = GenerateJWTSecret()
	if err := withTx(ctx, client, func(tx *ent.Tx) error {
		return setJWTSecretTx(ctx, tx, secret)
	}); err != nil {
		return "", err
	}
	return secret, nil
}

func setJWTSecretTx(ctx context.Context, tx *ent.Tx, secret string) error {
	value := strings.TrimSpace(secret)
	if value == "" {
		value = GenerateJWTSecret()
	}
	payload, err := json.Marshal(jwtSecretPayload{JWTSecret: value})
	if err != nil {
		return fmt.Errorf("encode jwt secret payload: %w", err)
	}
	rec, err := tx.ConfigSection.Query().Where(configsection.SectionEQ(adminCredSection)).Only(ctx)
	if err != nil {
		if !ent.IsNotFound(err) {
			return fmt.Errorf("load jwt secret section: %w", err)
		}
		_, err = tx.ConfigSection.Create().
			SetSection(adminCredSection).
			SetPayloadJSON(string(payload)).
			Save(ctx)
		if err != nil {
			if ent.IsConstraintError(err) {
				_, updateErr := tx.ConfigSection.Update().
					Where(configsection.SectionEQ(adminCredSection)).
					SetPayloadJSON(string(payload)).
					Save(ctx)
				if updateErr == nil {
					return nil
				}
			}
			return fmt.Errorf("save jwt secret section: %w", err)
		}
		return nil
	}
	_, err = tx.ConfigSection.UpdateOneID(rec.ID).SetPayloadJSON(string(payload)).Save(ctx)
	if err != nil {
		return fmt.Errorf("update jwt secret section: %w", err)
	}
	return nil
}

func parseJWTSecretPayload(payload []byte) string {
	if len(payload) == 0 {
		return ""
	}
	var tokenOnly jwtSecretPayload
	if err := json.Unmarshal(payload, &tokenOnly); err == nil {
		if strings.TrimSpace(tokenOnly.JWTSecret) != "" {
			return strings.TrimSpace(tokenOnly.JWTSecret)
		}
	}
	var legacy AdminCredential
	if err := json.Unmarshal(payload, &legacy); err == nil {
		return strings.TrimSpace(legacy.JWTSecret)
	}
	return ""
}

func loadLegacyAdminCredential(ctx context.Context, client *ent.Client) (*AdminCredential, error) {
	payload, exists, err := loadSectionPayload(ctx, client, adminCredSection)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}
	var legacy AdminCredential
	if err := json.Unmarshal(payload, &legacy); err != nil {
		return nil, nil
	}
	if strings.TrimSpace(legacy.Username) == "" {
		return nil, nil
	}
	return &legacy, nil
}
