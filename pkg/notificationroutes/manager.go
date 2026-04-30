package notificationroutes

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/ownership"
	"nekobot/pkg/storage/ent"
	"nekobot/pkg/storage/ent/notificationbinding"
	"nekobot/pkg/storage/ent/notificationroute"
)

// Manager persists notification routes and bindings in the runtime database.
type Manager struct {
	cfg    *config.Config
	log    *logger.Logger
	client *ent.Client
}

// NewManager creates a notification routes manager backed by the runtime database.
func NewManager(cfg *config.Config, log *logger.Logger, client *ent.Client) (*Manager, error) {
	if client == nil {
		return nil, fmt.Errorf("ent client is nil")
	}
	mgr := &Manager{cfg: cfg, log: log, client: client}
	dbPath, _ := config.RuntimeDBDisplayName(cfg)
	log.Info("Notification routes storage initialized", zap.String("db_path", dbPath))
	return mgr, nil
}

// Close releases manager resources. Shared Ent client is closed elsewhere.
func (m *Manager) Close() error {
	return nil
}

// ---------------------------------------------------------------------------
// NotificationRoute CRUD
// ---------------------------------------------------------------------------

// ListRoutes returns all notification routes, filtered by visibility when an
// AuthContext is present in ctx.
func (m *Manager) ListRoutes(ctx context.Context) ([]NotificationRoute, error) {
	q := m.client.NotificationRoute.Query().
		Order(ent.Asc(notificationroute.FieldName), ent.Asc(notificationroute.FieldUpdatedAt))
	if ac, ok := ownership.AuthContextFromContext(ctx); ok {
		q = q.Where(notificationroute.Or(
			notificationroute.OwnerUserIDEQ(ac.UserID),
			notificationroute.And(
				notificationroute.VisibilityEQ(notificationroute.Visibility(ownership.VisibilityShared)),
				notificationroute.TenantIDEQ(ac.TenantID),
			),
			notificationroute.VisibilityEQ(notificationroute.Visibility(ownership.VisibilitySystem)),
		))
	}
	recs, err := q.All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list notification routes: %w", err)
	}
	result := make([]NotificationRoute, 0, len(recs))
	for _, rec := range recs {
		result = append(result, routeFromRecord(rec))
	}
	return result, nil
}

// GetRoute returns one notification route by ID.
func (m *Manager) GetRoute(ctx context.Context, id string) (*NotificationRoute, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("route id is required")
	}
	rec, err := m.client.NotificationRoute.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("notification route not found")
		}
		return nil, fmt.Errorf("get notification route %s: %w", id, err)
	}
	item := routeFromRecord(rec)
	if ac, ok := ownership.AuthContextFromContext(ctx); ok {
		if !ac.CanRead(item.OwnerUserID, item.TenantID, item.Visibility) {
			return nil, ownership.ErrPermissionDenied
		}
	}
	return &item, nil
}

// CreateRoute inserts a new notification route.
// When an AuthContext is present, ownership fields are enforced.
func (m *Manager) CreateRoute(ctx context.Context, item NotificationRoute) (*NotificationRoute, error) {
	normalized, err := normalizeRoute(item)
	if err != nil {
		return nil, err
	}
	if ac, ok := ownership.AuthContextFromContext(ctx); ok {
		normalized.OwnerUserID, normalized.TenantID, normalized.Visibility = ac.ValidateCreateOwnership(
			normalized.OwnerUserID, normalized.TenantID, normalized.Visibility,
		)
	}
	rec, err := m.client.NotificationRoute.Create().
		SetName(normalized.Name).
		SetDescription(normalized.Description).
		SetEnabled(normalized.Enabled).
		SetChannelAccountID(normalized.ChannelAccountID).
		SetTargetConfigJSON(normalized.TargetConfigJSON).
		SetTenantID(normalized.TenantID).
		SetOwnerUserID(normalized.OwnerUserID).
		SetVisibility(notificationroute.Visibility(normalized.Visibility)).
		Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, fmt.Errorf("notification route name already exists")
		}
		return nil, fmt.Errorf("create notification route: %w", err)
	}
	out := routeFromRecord(rec)
	return &out, nil
}

// UpdateRoute updates an existing notification route by ID.
// When an AuthContext is present, write permission is verified and ownership is enforced.
func (m *Manager) UpdateRoute(ctx context.Context, id string, item NotificationRoute) (*NotificationRoute, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("route id is required")
	}
	normalized, err := normalizeRoute(item)
	if err != nil {
		return nil, err
	}
	if ac, ok := ownership.AuthContextFromContext(ctx); ok {
		existing, lookupErr := m.client.NotificationRoute.Get(ctx, id)
		if lookupErr != nil {
			if ent.IsNotFound(lookupErr) {
				return nil, fmt.Errorf("notification route not found")
			}
			return nil, fmt.Errorf("get notification route for auth: %w", lookupErr)
		}
		if !ac.CanWrite(existing.OwnerUserID) {
			return nil, ownership.ErrPermissionDenied
		}
		normalized.OwnerUserID, normalized.TenantID, normalized.Visibility = ac.ValidateUpdateOwnership(
			existing.OwnerUserID, existing.TenantID, string(existing.Visibility),
			normalized.OwnerUserID, normalized.TenantID, normalized.Visibility,
		)
	}
	rec, err := m.client.NotificationRoute.UpdateOneID(id).
		SetName(normalized.Name).
		SetDescription(normalized.Description).
		SetEnabled(normalized.Enabled).
		SetChannelAccountID(normalized.ChannelAccountID).
		SetTargetConfigJSON(normalized.TargetConfigJSON).
		SetTenantID(normalized.TenantID).
		SetOwnerUserID(normalized.OwnerUserID).
		SetVisibility(notificationroute.Visibility(normalized.Visibility)).
		Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("notification route not found")
		}
		if ent.IsConstraintError(err) {
			return nil, fmt.Errorf("notification route name already exists")
		}
		return nil, fmt.Errorf("update notification route %s: %w", id, err)
	}
	out := routeFromRecord(rec)
	return &out, nil
}

// DeleteRoute removes one notification route by ID.
// When an AuthContext is present, write permission is verified first.
func (m *Manager) DeleteRoute(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("route id is required")
	}
	if ac, ok := ownership.AuthContextFromContext(ctx); ok {
		existing, lookupErr := m.client.NotificationRoute.Get(ctx, id)
		if lookupErr != nil {
			if ent.IsNotFound(lookupErr) {
				return fmt.Errorf("notification route not found")
			}
			return fmt.Errorf("get notification route for auth: %w", lookupErr)
		}
		if !ac.CanWrite(existing.OwnerUserID) {
			return ownership.ErrPermissionDenied
		}
	}
	affected, err := m.client.NotificationRoute.Delete().Where(notificationroute.IDEQ(id)).Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete notification route %s: %w", id, err)
	}
	if affected == 0 {
		return fmt.Errorf("notification route not found")
	}
	return nil
}

// ---------------------------------------------------------------------------
// NotificationBinding CRUD
// ---------------------------------------------------------------------------

// ListBindings returns all notification bindings, filtered by visibility.
func (m *Manager) ListBindings(ctx context.Context) ([]NotificationBinding, error) {
	q := m.client.NotificationBinding.Query().
		Order(ent.Asc(notificationbinding.FieldScope), ent.Asc(notificationbinding.FieldUpdatedAt))
	if ac, ok := ownership.AuthContextFromContext(ctx); ok {
		q = q.Where(notificationbinding.Or(
			notificationbinding.OwnerUserIDEQ(ac.UserID),
			notificationbinding.And(
				notificationbinding.VisibilityEQ(notificationbinding.Visibility(ownership.VisibilityShared)),
				notificationbinding.TenantIDEQ(ac.TenantID),
			),
			notificationbinding.VisibilityEQ(notificationbinding.Visibility(ownership.VisibilitySystem)),
		))
	}
	recs, err := q.All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list notification bindings: %w", err)
	}
	result := make([]NotificationBinding, 0, len(recs))
	for _, rec := range recs {
		result = append(result, bindingFromRecord(rec))
	}
	return result, nil
}

// GetBinding returns one notification binding by ID.
func (m *Manager) GetBinding(ctx context.Context, id string) (*NotificationBinding, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("binding id is required")
	}
	rec, err := m.client.NotificationBinding.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("notification binding not found")
		}
		return nil, fmt.Errorf("get notification binding %s: %w", id, err)
	}
	item := bindingFromRecord(rec)
	return &item, nil
}

// ListBindingsByRoute returns bindings that target a specific route.
func (m *Manager) ListBindingsByRoute(ctx context.Context, routeID string) ([]NotificationBinding, error) {
	routeID = strings.TrimSpace(routeID)
	if routeID == "" {
		return nil, fmt.Errorf("route id is required")
	}
	q := m.client.NotificationBinding.Query().
		Where(notificationbinding.RouteIDEQ(routeID)).
		Order(ent.Asc(notificationbinding.FieldScope))
	if ac, ok := ownership.AuthContextFromContext(ctx); ok {
		q = q.Where(notificationbinding.Or(
			notificationbinding.OwnerUserIDEQ(ac.UserID),
			notificationbinding.And(
				notificationbinding.VisibilityEQ(notificationbinding.Visibility(ownership.VisibilityShared)),
				notificationbinding.TenantIDEQ(ac.TenantID),
			),
			notificationbinding.VisibilityEQ(notificationbinding.Visibility(ownership.VisibilitySystem)),
		))
	}
	recs, err := q.All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list bindings by route %s: %w", routeID, err)
	}
	result := make([]NotificationBinding, 0, len(recs))
	for _, rec := range recs {
		result = append(result, bindingFromRecord(rec))
	}
	return result, nil
}

// CreateBinding inserts a new notification binding.
// When an AuthContext is present, ownership fields are enforced.
func (m *Manager) CreateBinding(ctx context.Context, item NotificationBinding) (*NotificationBinding, error) {
	normalized, err := normalizeBinding(item)
	if err != nil {
		return nil, err
	}
	if ac, ok := ownership.AuthContextFromContext(ctx); ok {
		normalized.OwnerUserID, normalized.TenantID, normalized.Visibility = ac.ValidateCreateOwnership(
			normalized.OwnerUserID, normalized.TenantID, normalized.Visibility,
		)
	}
	if _, err := m.GetRoute(ctx, normalized.RouteID); err != nil {
		return nil, err
	}
	rec, err := m.client.NotificationBinding.Create().
		SetScope(normalized.Scope).
		SetTarget(normalized.Target).
		SetRouteID(normalized.RouteID).
		SetEventTypesJSON(normalized.EventTypesJSON).
		SetEnabled(normalized.Enabled).
		SetTenantID(normalized.TenantID).
		SetOwnerUserID(normalized.OwnerUserID).
		SetVisibility(notificationbinding.Visibility(normalized.Visibility)).
		Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, fmt.Errorf("notification binding constraint violation")
		}
		return nil, fmt.Errorf("create notification binding: %w", err)
	}
	out := bindingFromRecord(rec)
	return &out, nil
}

// UpdateBinding updates an existing notification binding by ID.
// When an AuthContext is present, write permission is verified and ownership is enforced.
func (m *Manager) UpdateBinding(ctx context.Context, id string, item NotificationBinding) (*NotificationBinding, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("binding id is required")
	}
	normalized, err := normalizeBinding(item)
	if err != nil {
		return nil, err
	}
	if ac, ok := ownership.AuthContextFromContext(ctx); ok {
		existing, lookupErr := m.client.NotificationBinding.Get(ctx, id)
		if lookupErr != nil {
			if ent.IsNotFound(lookupErr) {
				return nil, fmt.Errorf("notification binding not found")
			}
			return nil, fmt.Errorf("get notification binding for auth: %w", lookupErr)
		}
		if !ac.CanWrite(existing.OwnerUserID) {
			return nil, ownership.ErrPermissionDenied
		}
		normalized.OwnerUserID, normalized.TenantID, normalized.Visibility = ac.ValidateUpdateOwnership(
			existing.OwnerUserID, existing.TenantID, string(existing.Visibility),
			normalized.OwnerUserID, normalized.TenantID, normalized.Visibility,
		)
	}
	if _, err := m.GetRoute(ctx, normalized.RouteID); err != nil {
		return nil, err
	}
	rec, err := m.client.NotificationBinding.UpdateOneID(id).
		SetScope(normalized.Scope).
		SetTarget(normalized.Target).
		SetRouteID(normalized.RouteID).
		SetEventTypesJSON(normalized.EventTypesJSON).
		SetEnabled(normalized.Enabled).
		SetTenantID(normalized.TenantID).
		SetOwnerUserID(normalized.OwnerUserID).
		SetVisibility(notificationbinding.Visibility(normalized.Visibility)).
		Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("notification binding not found")
		}
		if ent.IsConstraintError(err) {
			return nil, fmt.Errorf("notification binding constraint violation")
		}
		return nil, fmt.Errorf("update notification binding %s: %w", id, err)
	}
	out := bindingFromRecord(rec)
	return &out, nil
}

// DeleteBinding removes one notification binding by ID.
// When an AuthContext is present, write permission is verified first.
func (m *Manager) DeleteBinding(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("binding id is required")
	}
	if ac, ok := ownership.AuthContextFromContext(ctx); ok {
		existing, lookupErr := m.client.NotificationBinding.Get(ctx, id)
		if lookupErr != nil {
			if ent.IsNotFound(lookupErr) {
				return fmt.Errorf("notification binding not found")
			}
			return fmt.Errorf("get notification binding for auth: %w", lookupErr)
		}
		if !ac.CanWrite(existing.OwnerUserID) {
			return ownership.ErrPermissionDenied
		}
	}
	affected, err := m.client.NotificationBinding.Delete().Where(notificationbinding.IDEQ(id)).Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete notification binding %s: %w", id, err)
	}
	if affected == 0 {
		return fmt.Errorf("notification binding not found")
	}
	return nil
}

// DeleteBindingsForTarget removes all bindings for a scope/target pair.
// When an AuthContext is present, every matched binding must be writable.
func (m *Manager) DeleteBindingsForTarget(ctx context.Context, scope, target string) error {
	scope = strings.TrimSpace(scope)
	target = strings.TrimSpace(target)
	if scope == "" {
		return fmt.Errorf("binding scope is required")
	}
	if target == "" {
		return fmt.Errorf("binding target is required")
	}
	q := m.client.NotificationBinding.Query().
		Where(
			notificationbinding.ScopeEQ(scope),
			notificationbinding.TargetEQ(target),
		)
	recs, err := q.All(ctx)
	if err != nil {
		return fmt.Errorf("list notification bindings for target: %w", err)
	}
	if ac, ok := ownership.AuthContextFromContext(ctx); ok {
		for _, rec := range recs {
			if !ac.CanWrite(rec.OwnerUserID) {
				return ownership.ErrPermissionDenied
			}
		}
	}
	if len(recs) == 0 {
		return nil
	}
	ids := make([]string, 0, len(recs))
	for _, rec := range recs {
		ids = append(ids, rec.ID)
	}
	if _, err := m.client.NotificationBinding.Delete().
		Where(notificationbinding.IDIn(ids...)).
		Exec(ctx); err != nil {
		return fmt.Errorf("delete notification bindings for target: %w", err)
	}
	return nil
}

// FindBindingForTarget returns the first visible binding for a scope/target pair.
func (m *Manager) FindBindingForTarget(ctx context.Context, scope, target string) (*NotificationBinding, error) {
	scope = strings.TrimSpace(scope)
	target = strings.TrimSpace(target)
	if scope == "" {
		return nil, fmt.Errorf("binding scope is required")
	}
	if target == "" {
		return nil, fmt.Errorf("binding target is required")
	}
	q := m.client.NotificationBinding.Query().
		Where(
			notificationbinding.ScopeEQ(scope),
			notificationbinding.TargetEQ(target),
		).
		Order(ent.Desc(notificationbinding.FieldUpdatedAt)).
		Limit(1)
	if ac, ok := ownership.AuthContextFromContext(ctx); ok {
		q = q.Where(notificationbinding.Or(
			notificationbinding.OwnerUserIDEQ(ac.UserID),
			notificationbinding.And(
				notificationbinding.VisibilityEQ(notificationbinding.Visibility(ownership.VisibilityShared)),
				notificationbinding.TenantIDEQ(ac.TenantID),
			),
			notificationbinding.VisibilityEQ(notificationbinding.Visibility(ownership.VisibilitySystem)),
		))
	}
	rec, err := q.Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("find notification binding for target: %w", err)
	}
	item := bindingFromRecord(rec)
	return &item, nil
}

// ---------------------------------------------------------------------------
// Normalization helpers
// ---------------------------------------------------------------------------

func normalizeRoute(item NotificationRoute) (NotificationRoute, error) {
	item.Name = strings.TrimSpace(item.Name)
	item.Description = strings.TrimSpace(item.Description)
	item.ChannelAccountID = strings.TrimSpace(item.ChannelAccountID)
	item.TargetConfigJSON = strings.TrimSpace(item.TargetConfigJSON)
	item.TenantID = strings.TrimSpace(item.TenantID)
	item.OwnerUserID = strings.TrimSpace(item.OwnerUserID)
	item.Visibility = ownership.NormalizeVisibility(item.Visibility)
	if item.Name == "" {
		return NotificationRoute{}, fmt.Errorf("route name is required")
	}
	if item.TargetConfigJSON == "" {
		item.TargetConfigJSON = "{}"
	}
	return item, nil
}

func normalizeBinding(item NotificationBinding) (NotificationBinding, error) {
	item.Scope = strings.TrimSpace(item.Scope)
	item.Target = strings.TrimSpace(item.Target)
	item.RouteID = strings.TrimSpace(item.RouteID)
	item.EventTypesJSON = strings.TrimSpace(item.EventTypesJSON)
	item.TenantID = strings.TrimSpace(item.TenantID)
	item.OwnerUserID = strings.TrimSpace(item.OwnerUserID)
	item.Visibility = ownership.NormalizeVisibility(item.Visibility)
	if item.Scope == "" {
		return NotificationBinding{}, fmt.Errorf("binding scope is required")
	}
	if item.RouteID == "" {
		return NotificationBinding{}, fmt.Errorf("binding route_id is required")
	}
	if item.EventTypesJSON == "" {
		item.EventTypesJSON = "[]"
	}
	return item, nil
}

// ---------------------------------------------------------------------------
// Record converters
// ---------------------------------------------------------------------------

func routeFromRecord(rec *ent.NotificationRoute) NotificationRoute {
	return NotificationRoute{
		ID:               rec.ID,
		Name:             rec.Name,
		Description:      rec.Description,
		Enabled:          rec.Enabled,
		ChannelAccountID: rec.ChannelAccountID,
		TargetConfigJSON: rec.TargetConfigJSON,
		TenantID:         rec.TenantID,
		OwnerUserID:      rec.OwnerUserID,
		Visibility:       string(rec.Visibility),
		CreatedAt:        rec.CreatedAt,
		UpdatedAt:        rec.UpdatedAt,
	}
}

func bindingFromRecord(rec *ent.NotificationBinding) NotificationBinding {
	return NotificationBinding{
		ID:             rec.ID,
		Scope:          rec.Scope,
		Target:         rec.Target,
		RouteID:        rec.RouteID,
		EventTypesJSON: rec.EventTypesJSON,
		Enabled:        rec.Enabled,
		TenantID:       rec.TenantID,
		OwnerUserID:    rec.OwnerUserID,
		Visibility:     string(rec.Visibility),
		CreatedAt:      rec.CreatedAt,
		UpdatedAt:      rec.UpdatedAt,
	}
}

// Ensure JSON helpers are imported (for compile-time validation).
var _ = json.Marshal
