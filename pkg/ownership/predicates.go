package ownership

import (
	"context"
	"errors"
)

var (
	// ErrPermissionDenied is returned when an authenticated user attempts
	// to access or modify a resource they do not own.
	ErrPermissionDenied = errors.New("permission denied")
)

// AuthContext carries the authenticated user's identity for access control.
type AuthContext struct {
	UserID   string
	TenantID string
	Role     string
}

type authContextKey struct{}

// WithAuthContext embeds an AuthContext into a context for downstream managers.
// When the AuthContext has no UserID (unauthenticated), the original context
// is returned unchanged so managers fall back to unfiltered behavior.
func WithAuthContext(ctx context.Context, ac AuthContext) context.Context {
	if ac.UserID == "" {
		return ctx
	}
	return context.WithValue(ctx, authContextKey{}, ac)
}

// AuthContextFromContext extracts an AuthContext previously stored with WithAuthContext.
// The bool result is false when no AuthContext was stored (unfiltered fallback).
func AuthContextFromContext(ctx context.Context) (AuthContext, bool) {
	ac, ok := ctx.Value(authContextKey{}).(AuthContext)
	return ac, ok
}

// IsAdminOrOwner returns true for elevated roles that bypass ownership checks.
func (ac AuthContext) IsAdminOrOwner() bool {
	return ac.Role == "admin" || ac.Role == "owner"
}

// CanRead returns true if this AuthContext is allowed to read a resource
// with the given ownership fields.
func (ac AuthContext) CanRead(ownerUserID, tenantID, visibility string) bool {
	if ac.IsAdminOrOwner() {
		return true
	}
	switch NormalizeVisibility(visibility) {
	case VisibilitySystem:
		return true
	case VisibilityShared:
		return ac.TenantID != "" && ac.TenantID == tenantID
	default:
		return ac.UserID != "" && ac.UserID == ownerUserID
	}
}

// CanWrite returns true if this AuthContext is allowed to modify or delete
// a resource with the given owner. Admins/owners can write anything; normal
// users can only write their own resources.
func (ac AuthContext) CanWrite(ownerUserID string) bool {
	if ac.IsAdminOrOwner() {
		return true
	}
	if ac.UserID == "" {
		return false
	}
	return ac.UserID == ownerUserID
}

// ValidateCreateOwnership enforces ownership rules for resource creation.
// Admin/owner callers may set any owner/tenant/visibility (including system).
// Normal users are forced to their own identity and may not set system visibility.
// Returns the (ownerUserID, tenantID, visibility) that should be persisted.
func (ac AuthContext) ValidateCreateOwnership(ownerUserID, tenantID, visibility string) (string, string, string) {
	if ac.IsAdminOrOwner() {
		if ownerUserID == "" {
			ownerUserID = ac.UserID
		}
		if tenantID == "" {
			tenantID = ac.TenantID
		}
		return ownerUserID, tenantID, NormalizeVisibility(visibility)
	}
	return ac.UserID, ac.TenantID, restrictVisibility(NormalizeVisibility(visibility))
}

// ValidateUpdateOwnership enforces ownership rules for resource updates.
// Normal users cannot change owner, tenant, or escalate to system visibility.
func (ac AuthContext) ValidateUpdateOwnership(
	existingOwner, existingTenant, existingVisibility string,
	incomingOwner, incomingTenant, incomingVisibility string,
) (string, string, string) {
	if ac.IsAdminOrOwner() {
		owner := incomingOwner
		tenant := incomingTenant
		if owner == "" {
			owner = existingOwner
		}
		if tenant == "" {
			tenant = existingTenant
		}
		return owner, tenant, NormalizeVisibility(incomingVisibility)
	}
	// Normal users: retain existing owner/tenant, allow only private/shared visibility
	vis := NormalizeVisibility(incomingVisibility)
	if vis == "" || vis == VisibilitySystem {
		vis = existingVisibility
	}
	vis = restrictVisibility(vis)
	return existingOwner, existingTenant, vis
}

func restrictVisibility(vis string) string {
	switch vis {
	case VisibilitySystem:
		return VisibilityShared
	case "":
		return VisibilityShared
	default:
		return vis
	}
}
