package permissionrules

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/storage/ent"
	"nekobot/pkg/storage/ent/permissionrule"
)

var (
	ErrRuleNotFound = errors.New("permission rule not found")
	ErrInvalidRule  = errors.New("invalid permission rule")
)

type Action string

const (
	ActionAllow Action = "allow"
	ActionDeny  Action = "deny"
	ActionAsk   Action = "ask"
)

type Rule struct {
	ID          string    `json:"id,omitempty"`
	Enabled     bool      `json:"enabled"`
	Priority    int       `json:"priority"`
	ToolName    string    `json:"tool_name"`
	SessionID   string    `json:"session_id,omitempty"`
	RuntimeID   string    `json:"runtime_id,omitempty"`
	Action      Action    `json:"action"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
}

type Manager struct {
	cfg    *config.Config
	log    *logger.Logger
	client *ent.Client
}

func NewManager(cfg *config.Config, log *logger.Logger, client *ent.Client) (*Manager, error) {
	if client == nil {
		return nil, fmt.Errorf("ent client is nil")
	}
	if log != nil {
		dbPath, _ := config.RuntimeDBPath(cfg)
		log.Info("Permission rule storage initialized", zap.String("db_path", dbPath))
	}
	return &Manager{cfg: cfg, log: log, client: client}, nil
}

func (m *Manager) Create(ctx context.Context, item Rule) (*Rule, error) {
	normalized, err := normalize(item)
	if err != nil {
		return nil, err
	}
	rec, err := m.client.PermissionRule.Create().
		SetEnabled(normalized.Enabled).
		SetPriority(normalized.Priority).
		SetToolName(normalized.ToolName).
		SetSessionID(normalized.SessionID).
		SetRuntimeID(normalized.RuntimeID).
		SetAction(string(normalized.Action)).
		SetDescription(normalized.Description).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create permission rule: %w", err)
	}
	out := toRule(rec)
	return &out, nil
}

func (m *Manager) List(ctx context.Context) ([]Rule, error) {
	recs, err := m.client.PermissionRule.Query().
		Order(
			ent.Desc(permissionrule.FieldPriority),
			ent.Desc(permissionrule.FieldUpdatedAt),
			ent.Asc(permissionrule.FieldID),
		).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list permission rules: %w", err)
	}
	items := make([]Rule, 0, len(recs))
	for _, rec := range recs {
		items = append(items, toRule(rec))
	}
	return items, nil
}

func (m *Manager) Get(ctx context.Context, id string) (*Rule, error) {
	trimmedID := strings.TrimSpace(id)
	if trimmedID == "" {
		return nil, fmt.Errorf("%w: id is required", ErrInvalidRule)
	}
	rec, err := m.client.PermissionRule.Get(ctx, trimmedID)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrRuleNotFound
		}
		return nil, fmt.Errorf("get permission rule %s: %w", trimmedID, err)
	}
	out := toRule(rec)
	return &out, nil
}

func (m *Manager) Update(ctx context.Context, id string, item Rule) (*Rule, error) {
	trimmedID := strings.TrimSpace(id)
	if trimmedID == "" {
		return nil, fmt.Errorf("%w: id is required", ErrInvalidRule)
	}
	normalized, err := normalize(item)
	if err != nil {
		return nil, err
	}
	rec, err := m.client.PermissionRule.UpdateOneID(trimmedID).
		SetEnabled(normalized.Enabled).
		SetPriority(normalized.Priority).
		SetToolName(normalized.ToolName).
		SetSessionID(normalized.SessionID).
		SetRuntimeID(normalized.RuntimeID).
		SetAction(string(normalized.Action)).
		SetDescription(normalized.Description).
		Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrRuleNotFound
		}
		return nil, fmt.Errorf("update permission rule %s: %w", trimmedID, err)
	}
	out := toRule(rec)
	return &out, nil
}

func (m *Manager) Delete(ctx context.Context, id string) error {
	trimmedID := strings.TrimSpace(id)
	if trimmedID == "" {
		return fmt.Errorf("%w: id is required", ErrInvalidRule)
	}
	err := m.client.PermissionRule.DeleteOneID(trimmedID).Exec(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return ErrRuleNotFound
		}
		return fmt.Errorf("delete permission rule %s: %w", trimmedID, err)
	}
	return nil
}

func normalize(item Rule) (Rule, error) {
	item.ToolName = strings.TrimSpace(item.ToolName)
	item.SessionID = strings.TrimSpace(item.SessionID)
	item.RuntimeID = strings.TrimSpace(item.RuntimeID)
	item.Description = strings.TrimSpace(item.Description)
	item.Action = Action(strings.TrimSpace(string(item.Action)))

	if item.ToolName == "" {
		return Rule{}, fmt.Errorf("%w: tool_name is required", ErrInvalidRule)
	}
	switch item.Action {
	case ActionAllow, ActionDeny, ActionAsk:
	default:
		return Rule{}, fmt.Errorf("%w: unsupported action %q", ErrInvalidRule, item.Action)
	}
	return item, nil
}

func toRule(rec *ent.PermissionRule) Rule {
	return Rule{
		ID:          rec.ID,
		Enabled:     rec.Enabled,
		Priority:    rec.Priority,
		ToolName:    rec.ToolName,
		SessionID:   rec.SessionID,
		RuntimeID:   rec.RuntimeID,
		Action:      Action(rec.Action),
		Description: rec.Description,
		CreatedAt:   rec.CreatedAt,
		UpdatedAt:   rec.UpdatedAt,
	}
}
