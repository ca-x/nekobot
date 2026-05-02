package config

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"nekobot/pkg/storage/ent"
	"nekobot/pkg/storage/ent/membership"
	"nekobot/pkg/storage/ent/tenant"
	"nekobot/pkg/storage/ent/user"
)

var ErrCannotDisableLastPrivilegedUser = errors.New("cannot disable the last privileged user")

type UserRecord struct {
	ID         string     `json:"id"`
	Username   string     `json:"username"`
	Nickname   string     `json:"nickname"`
	Role       string     `json:"role"`
	Enabled    bool       `json:"enabled"`
	TenantID   string     `json:"tenant_id"`
	TenantSlug string     `json:"tenant_slug"`
	LastLogin  *time.Time `json:"last_login,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type UserInput struct {
	Username     string
	Nickname     string
	PasswordHash string
	Role         string
	Enabled      bool
}

func ListUsers(ctx context.Context, client *ent.Client) ([]UserRecord, error) {
	if client == nil {
		return nil, fmt.Errorf("ent client is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	recs, err := client.User.Query().Order(ent.Asc(user.FieldUsername)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	result := make([]UserRecord, 0, len(recs))
	for _, rec := range recs {
		item, err := userRecordFromEnt(ctx, client, rec)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

func CreateUser(ctx context.Context, client *ent.Client, input UserInput) (*UserRecord, error) {
	if client == nil {
		return nil, fmt.Errorf("ent client is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	normalized, err := normalizeUserInput(input, true)
	if err != nil {
		return nil, err
	}
	var createdID string
	err = withTx(ctx, client, func(tx *ent.Tx) error {
		tenantID, err := ensureDefaultTenantTx(ctx, tx)
		if err != nil {
			return err
		}
		rec, err := tx.User.Create().
			SetUsername(normalized.Username).
			SetNickname(normalized.Nickname).
			SetPasswordHash(normalized.PasswordHash).
			SetRole(normalized.Role).
			SetEnabled(normalized.Enabled).
			Save(ctx)
		if err != nil {
			if ent.IsConstraintError(err) {
				return ErrUsernameAlreadyUsed
			}
			return fmt.Errorf("create user: %w", err)
		}
		createdID = rec.ID
		return ensureMembershipTx(ctx, tx, rec.ID, tenantID, membershipRoleForUserRole(normalized.Role))
	})
	if err != nil {
		return nil, err
	}
	rec, err := client.User.Get(ctx, createdID)
	if err != nil {
		return nil, fmt.Errorf("load created user: %w", err)
	}
	out, err := userRecordFromEnt(ctx, client, rec)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func UpdateUser(ctx context.Context, client *ent.Client, id string, input UserInput) (*UserRecord, error) {
	if client == nil {
		return nil, fmt.Errorf("ent client is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("user id is required")
	}
	normalized, err := normalizeUserInput(input, false)
	if err != nil {
		return nil, err
	}
	if !normalized.Enabled {
		if err := ensureNotLastPrivilegedUser(ctx, client, id); err != nil {
			return nil, err
		}
	}
	var updatedID string
	err = withTx(ctx, client, func(tx *ent.Tx) error {
		update := tx.User.UpdateOneID(id).
			SetUsername(normalized.Username).
			SetNickname(normalized.Nickname).
			SetRole(normalized.Role).
			SetEnabled(normalized.Enabled)
		if strings.TrimSpace(normalized.PasswordHash) != "" {
			update.SetPasswordHash(normalized.PasswordHash)
		}
		rec, err := update.Save(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				return ErrAdminNotInitialized
			}
			if ent.IsConstraintError(err) {
				return ErrUsernameAlreadyUsed
			}
			return fmt.Errorf("update user: %w", err)
		}
		updatedID = rec.ID

		member, err := tx.Membership.Query().Where(membership.UserIDEQ(id), membership.EnabledEQ(true)).First(ctx)
		if err == nil {
			_, err = tx.Membership.UpdateOneID(member.ID).
				SetRole(membershipRoleForUserRole(normalized.Role)).
				SetEnabled(normalized.Enabled).
				Save(ctx)
			return err
		}
		if !ent.IsNotFound(err) {
			return fmt.Errorf("load membership: %w", err)
		}
		tenantID, err := ensureDefaultTenantTx(ctx, tx)
		if err != nil {
			return err
		}
		return ensureMembershipTx(ctx, tx, id, tenantID, membershipRoleForUserRole(normalized.Role))
	})
	if err != nil {
		return nil, err
	}
	rec, err := client.User.Get(ctx, updatedID)
	if err != nil {
		return nil, fmt.Errorf("load updated user: %w", err)
	}
	out, err := userRecordFromEnt(ctx, client, rec)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func DeleteUser(ctx context.Context, client *ent.Client, id string) error {
	if client == nil {
		return fmt.Errorf("ent client is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("user id is required")
	}
	if err := ensureNotLastPrivilegedUser(ctx, client, id); err != nil {
		return err
	}
	return withTx(ctx, client, func(tx *ent.Tx) error {
		if _, err := tx.Membership.Delete().Where(membership.UserIDEQ(id)).Exec(ctx); err != nil {
			return fmt.Errorf("delete memberships: %w", err)
		}
		affected, err := tx.User.Delete().Where(user.IDEQ(id)).Exec(ctx)
		if err != nil {
			return fmt.Errorf("delete user: %w", err)
		}
		if affected == 0 {
			return ErrAdminNotInitialized
		}
		return nil
	})
}

func normalizeUserInput(input UserInput, requirePassword bool) (UserInput, error) {
	input.Username = strings.TrimSpace(input.Username)
	input.Nickname = strings.TrimSpace(input.Nickname)
	input.PasswordHash = strings.TrimSpace(input.PasswordHash)
	input.Role = normalizeUserRole(input.Role)
	if input.Username == "" {
		return UserInput{}, fmt.Errorf("username is required")
	}
	if requirePassword && input.PasswordHash == "" {
		return UserInput{}, fmt.Errorf("password hash is required")
	}
	return input, nil
}

func normalizeUserRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "admin":
		return "admin"
	case "owner":
		return "owner"
	default:
		return "member"
	}
}

func membershipRoleForUserRole(role string) string {
	switch normalizeUserRole(role) {
	case "admin":
		return "owner"
	case "owner":
		return "owner"
	default:
		return "member"
	}
}

func userRecordFromEnt(ctx context.Context, client *ent.Client, rec *ent.User) (UserRecord, error) {
	item := UserRecord{
		ID:        rec.ID,
		Username:  rec.Username,
		Nickname:  rec.Nickname,
		Role:      rec.Role,
		Enabled:   rec.Enabled,
		LastLogin: rec.LastLogin,
		CreatedAt: rec.CreatedAt,
		UpdatedAt: rec.UpdatedAt,
	}
	member, err := client.Membership.Query().
		Where(membership.UserIDEQ(rec.ID), membership.EnabledEQ(true)).
		Order(ent.Desc(membership.FieldCreatedAt)).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return item, nil
		}
		return item, fmt.Errorf("load user membership: %w", err)
	}
	item.Role = strings.TrimSpace(member.Role)
	if item.Role == "" {
		item.Role = rec.Role
	}
	item.TenantID = member.TenantID
	tenantRec, err := client.Tenant.Query().Where(tenant.IDEQ(member.TenantID)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return item, nil
		}
		return item, fmt.Errorf("load user tenant: %w", err)
	}
	item.TenantSlug = tenantRec.Slug
	return item, nil
}

func ensureNotLastPrivilegedUser(ctx context.Context, client *ent.Client, id string) error {
	rec, err := client.User.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return ErrAdminNotInitialized
		}
		return fmt.Errorf("load user: %w", err)
	}
	if !rec.Enabled {
		return nil
	}
	profile, err := BuildAuthProfileByUserID(ctx, client, id)
	if err != nil {
		if errors.Is(err, ErrAdminNotInitialized) {
			return nil
		}
		return err
	}
	if profile.Role != "admin" && profile.Role != "owner" {
		return nil
	}
	users, err := ListUsers(ctx, client)
	if err != nil {
		return err
	}
	activePrivileged := 0
	for _, item := range users {
		if !item.Enabled {
			continue
		}
		if item.Role == "admin" || item.Role == "owner" {
			activePrivileged++
		}
	}
	if activePrivileged <= 1 {
		return ErrCannotDisableLastPrivilegedUser
	}
	return nil
}
