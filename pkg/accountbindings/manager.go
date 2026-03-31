package accountbindings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"go.uber.org/zap"

	"nekobot/pkg/channelaccounts"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/runtimeagents"
	"nekobot/pkg/storage/ent"
	"nekobot/pkg/storage/ent/accountbinding"
)

var (
	// ErrBindingExists indicates the account/runtime pair is already bound.
	ErrBindingExists = errors.New("account binding already exists")
	// ErrBindingNotFound indicates the requested binding does not exist.
	ErrBindingNotFound = errors.New("account binding not found")
	// ErrBindingModeConflict indicates bindings for one account disagree on binding mode.
	ErrBindingModeConflict = errors.New("account binding mode conflict")
	// ErrSingleAgentBindingExceeded indicates a single-agent account already has another binding.
	ErrSingleAgentBindingExceeded = errors.New("single-agent account already has another binding")
)

// Manager persists account bindings and enforces per-account binding rules.
type Manager struct {
	cfg      *config.Config
	log      *logger.Logger
	client   *ent.Client
	runtimes *runtimeagents.Manager
	accounts *channelaccounts.Manager
}

// NewManager creates an account binding manager.
func NewManager(
	cfg *config.Config,
	log *logger.Logger,
	client *ent.Client,
	runtimes *runtimeagents.Manager,
	accounts *channelaccounts.Manager,
) (*Manager, error) {
	if client == nil {
		return nil, fmt.Errorf("ent client is nil")
	}
	if runtimes == nil {
		return nil, fmt.Errorf("runtime manager is nil")
	}
	if accounts == nil {
		return nil, fmt.Errorf("channel account manager is nil")
	}
	mgr := &Manager{
		cfg:      cfg,
		log:      log,
		client:   client,
		runtimes: runtimes,
		accounts: accounts,
	}
	dbPath, _ := config.RuntimeDBPath(cfg)
	log.Info("Account binding storage initialized", zap.String("db_path", dbPath))
	return mgr, nil
}

// Close releases manager resources. Shared Ent client is closed elsewhere.
func (m *Manager) Close() error {
	return nil
}

// List returns all bindings ordered by account and priority.
func (m *Manager) List(ctx context.Context) ([]AccountBinding, error) {
	recs, err := m.client.AccountBinding.Query().
		Order(
			ent.Asc(accountbinding.FieldChannelAccountID),
			ent.Asc(accountbinding.FieldPriority),
			ent.Asc(accountbinding.FieldAgentRuntimeID),
		).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list account bindings: %w", err)
	}
	result := make([]AccountBinding, 0, len(recs))
	for _, rec := range recs {
		item, err := toBinding(rec)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

// Get returns one binding by ID.
func (m *Manager) Get(ctx context.Context, id string) (*AccountBinding, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("account binding id is required")
	}
	rec, err := m.client.AccountBinding.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrBindingNotFound
		}
		return nil, fmt.Errorf("get account binding %s: %w", id, err)
	}
	item, err := toBinding(rec)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// ListByChannelAccountID returns all bindings for one account ordered by priority.
func (m *Manager) ListByChannelAccountID(ctx context.Context, channelAccountID string) ([]AccountBinding, error) {
	channelAccountID = strings.TrimSpace(channelAccountID)
	if channelAccountID == "" {
		return nil, fmt.Errorf("channel account id is required")
	}

	recs, err := m.client.AccountBinding.Query().
		Where(accountbinding.ChannelAccountIDEQ(channelAccountID)).
		Order(
			ent.Asc(accountbinding.FieldPriority),
			ent.Asc(accountbinding.FieldAgentRuntimeID),
		).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list account bindings for %s: %w", channelAccountID, err)
	}

	items := make([]AccountBinding, 0, len(recs))
	for _, rec := range recs {
		item, err := toBinding(rec)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

// ListEnabledByChannelAccountID returns enabled bindings for one account ordered by priority.
func (m *Manager) ListEnabledByChannelAccountID(ctx context.Context, channelAccountID string) ([]AccountBinding, error) {
	items, err := m.ListByChannelAccountID(ctx, channelAccountID)
	if err != nil {
		return nil, err
	}

	enabled := make([]AccountBinding, 0, len(items))
	for _, item := range items {
		if !item.Enabled {
			continue
		}
		enabled = append(enabled, item)
	}
	return enabled, nil
}

// Create inserts a new account binding.
func (m *Manager) Create(ctx context.Context, item AccountBinding) (*AccountBinding, error) {
	normalized, err := m.normalizeBinding(ctx, "", item)
	if err != nil {
		return nil, err
	}
	metadataJSON, err := marshalMap(normalized.Metadata)
	if err != nil {
		return nil, err
	}
	rec, err := m.client.AccountBinding.Create().
		SetChannelAccountID(normalized.ChannelAccountID).
		SetAgentRuntimeID(normalized.AgentRuntimeID).
		SetBindingMode(accountbinding.BindingMode(normalized.BindingMode)).
		SetEnabled(normalized.Enabled).
		SetAllowPublicReply(normalized.AllowPublicReply).
		SetReplyLabel(normalized.ReplyLabel).
		SetPriority(normalized.Priority).
		SetMetadataJSON(metadataJSON).
		Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, ErrBindingExists
		}
		return nil, fmt.Errorf("create account binding: %w", err)
	}
	out, err := toBinding(rec)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// Update updates an existing binding.
func (m *Manager) Update(ctx context.Context, id string, item AccountBinding) (*AccountBinding, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("account binding id is required")
	}
	normalized, err := m.normalizeBinding(ctx, id, item)
	if err != nil {
		return nil, err
	}
	metadataJSON, err := marshalMap(normalized.Metadata)
	if err != nil {
		return nil, err
	}
	rec, err := m.client.AccountBinding.UpdateOneID(id).
		SetChannelAccountID(normalized.ChannelAccountID).
		SetAgentRuntimeID(normalized.AgentRuntimeID).
		SetBindingMode(accountbinding.BindingMode(normalized.BindingMode)).
		SetEnabled(normalized.Enabled).
		SetAllowPublicReply(normalized.AllowPublicReply).
		SetReplyLabel(normalized.ReplyLabel).
		SetPriority(normalized.Priority).
		SetMetadataJSON(metadataJSON).
		Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrBindingNotFound
		}
		if ent.IsConstraintError(err) {
			return nil, ErrBindingExists
		}
		return nil, fmt.Errorf("update account binding %s: %w", id, err)
	}
	out, err := toBinding(rec)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// Delete removes one binding by ID.
func (m *Manager) Delete(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("account binding id is required")
	}
	affected, err := m.client.AccountBinding.Delete().Where(accountbinding.IDEQ(id)).Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete account binding %s: %w", id, err)
	}
	if affected == 0 {
		return ErrBindingNotFound
	}
	return nil
}

// DeleteByRuntimeID removes all bindings that reference one runtime.
func (m *Manager) DeleteByRuntimeID(ctx context.Context, runtimeID string) error {
	runtimeID = strings.TrimSpace(runtimeID)
	if runtimeID == "" {
		return fmt.Errorf("agent runtime id is required")
	}
	if _, err := m.client.AccountBinding.Delete().
		Where(accountbinding.AgentRuntimeIDEQ(runtimeID)).
		Exec(ctx); err != nil {
		return fmt.Errorf("delete account bindings for runtime %s: %w", runtimeID, err)
	}
	return nil
}

// DeleteByChannelAccountID removes all bindings that reference one channel account.
func (m *Manager) DeleteByChannelAccountID(ctx context.Context, channelAccountID string) error {
	channelAccountID = strings.TrimSpace(channelAccountID)
	if channelAccountID == "" {
		return fmt.Errorf("channel account id is required")
	}
	if _, err := m.client.AccountBinding.Delete().
		Where(accountbinding.ChannelAccountIDEQ(channelAccountID)).
		Exec(ctx); err != nil {
		return fmt.Errorf("delete account bindings for channel account %s: %w", channelAccountID, err)
	}
	return nil
}

func (m *Manager) normalizeBinding(
	ctx context.Context,
	currentID string,
	item AccountBinding,
) (AccountBinding, error) {
	item.ChannelAccountID = strings.TrimSpace(item.ChannelAccountID)
	item.AgentRuntimeID = strings.TrimSpace(item.AgentRuntimeID)
	item.BindingMode = strings.TrimSpace(item.BindingMode)
	item.ReplyLabel = strings.TrimSpace(item.ReplyLabel)
	if item.ChannelAccountID == "" {
		return AccountBinding{}, fmt.Errorf("channel_account_id is required")
	}
	if item.AgentRuntimeID == "" {
		return AccountBinding{}, fmt.Errorf("agent_runtime_id is required")
	}
	if item.BindingMode == "" {
		item.BindingMode = ModeSingleAgent
	}
	if item.BindingMode != ModeSingleAgent && item.BindingMode != ModeMultiAgent {
		return AccountBinding{}, fmt.Errorf("invalid binding_mode")
	}
	if item.Priority <= 0 {
		item.Priority = 100
	}
	if item.Metadata == nil {
		item.Metadata = map[string]interface{}{}
	}
	if _, err := m.accounts.Get(ctx, item.ChannelAccountID); err != nil {
		if errors.Is(err, channelaccounts.ErrAccountNotFound) {
			return AccountBinding{}, fmt.Errorf("channel account %s not found", item.ChannelAccountID)
		}
		return AccountBinding{}, err
	}
	if _, err := m.runtimes.Get(ctx, item.AgentRuntimeID); err != nil {
		if errors.Is(err, runtimeagents.ErrRuntimeNotFound) {
			return AccountBinding{}, fmt.Errorf("agent runtime %s not found", item.AgentRuntimeID)
		}
		return AccountBinding{}, err
	}
	if err := m.ensureAccountMode(ctx, currentID, item); err != nil {
		return AccountBinding{}, err
	}
	return item, nil
}

func (m *Manager) ensureAccountMode(
	ctx context.Context,
	currentID string,
	item AccountBinding,
) error {
	recs, err := m.client.AccountBinding.Query().
		Where(accountbinding.ChannelAccountIDEQ(item.ChannelAccountID)).
		All(ctx)
	if err != nil {
		return fmt.Errorf("load existing account bindings: %w", err)
	}

	activeBindings := 0
	for _, rec := range recs {
		if rec.ID == currentID {
			continue
		}
		if rec.AgentRuntimeID == item.AgentRuntimeID {
			continue
		}
		if rec.BindingMode != accountbinding.BindingMode(item.BindingMode) {
			return ErrBindingModeConflict
		}
		if rec.Enabled {
			activeBindings++
		}
	}
	if item.BindingMode == ModeSingleAgent && item.Enabled && activeBindings > 0 {
		return ErrSingleAgentBindingExceeded
	}
	return nil
}

func marshalMap(values map[string]interface{}) (string, error) {
	payload, err := json.Marshal(values)
	if err != nil {
		return "", fmt.Errorf("marshal map: %w", err)
	}
	return string(payload), nil
}

func distinctSortedBindings(items []AccountBinding) []AccountBinding {
	result := make([]AccountBinding, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		if _, ok := seen[item.ID]; ok {
			continue
		}
		seen[item.ID] = struct{}{}
		result = append(result, item)
	}
	slices.SortFunc(result, func(a, b AccountBinding) int {
		if a.Priority != b.Priority {
			return a.Priority - b.Priority
		}
		return strings.Compare(a.AgentRuntimeID, b.AgentRuntimeID)
	})
	return result
}

func unmarshalMap(raw string) (map[string]interface{}, error) {
	if strings.TrimSpace(raw) == "" {
		return map[string]interface{}{}, nil
	}
	var values map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil, err
	}
	if values == nil {
		return map[string]interface{}{}, nil
	}
	return values, nil
}

func toBinding(rec *ent.AccountBinding) (AccountBinding, error) {
	metadata, err := unmarshalMap(rec.MetadataJSON)
	if err != nil {
		return AccountBinding{}, fmt.Errorf("decode account binding metadata %s: %w", rec.ID, err)
	}
	return AccountBinding{
		ID:               rec.ID,
		ChannelAccountID: rec.ChannelAccountID,
		AgentRuntimeID:   rec.AgentRuntimeID,
		BindingMode:      string(rec.BindingMode),
		Enabled:          rec.Enabled,
		AllowPublicReply: rec.AllowPublicReply,
		ReplyLabel:       rec.ReplyLabel,
		Priority:         rec.Priority,
		Metadata:         metadata,
		CreatedAt:        rec.CreatedAt,
		UpdatedAt:        rec.UpdatedAt,
	}, nil
}
