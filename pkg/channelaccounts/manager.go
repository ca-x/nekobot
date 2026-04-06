package channelaccounts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"go.uber.org/zap"

	channelwechat "nekobot/pkg/channels/wechat"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/storage/ent"
	"nekobot/pkg/storage/ent/channelaccount"
)

var (
	// ErrAccountExists indicates a channel account with the same channel_type/account_key already exists.
	ErrAccountExists = errors.New("channel account already exists")
	// ErrAccountNotFound indicates the requested channel account does not exist.
	ErrAccountNotFound = errors.New("channel account not found")
)

// Manager persists channel accounts in the shared runtime database.
type Manager struct {
	cfg    *config.Config
	log    *logger.Logger
	client *ent.Client
}

// NewManager creates a channel account manager.
func NewManager(cfg *config.Config, log *logger.Logger, client *ent.Client) (*Manager, error) {
	if client == nil {
		return nil, fmt.Errorf("ent client is nil")
	}
	mgr := &Manager{cfg: cfg, log: log, client: client}
	dbPath, _ := config.RuntimeDBPath(cfg)
	log.Info("Channel account storage initialized", zap.String("db_path", dbPath))
	return mgr, nil
}

// Close releases manager resources. Shared Ent client is closed elsewhere.
func (m *Manager) Close() error {
	return nil
}

// List returns all channel accounts ordered by type/key.
func (m *Manager) List(ctx context.Context) ([]ChannelAccount, error) {
	recs, err := m.client.ChannelAccount.Query().
		Order(ent.Asc(channelaccount.FieldChannelType), ent.Asc(channelaccount.FieldAccountKey)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list channel accounts: %w", err)
	}
	result := make([]ChannelAccount, 0, len(recs))
	for _, rec := range recs {
		item, err := toAccount(rec)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

// Get returns one channel account by ID.
func (m *Manager) Get(ctx context.Context, id string) (*ChannelAccount, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("channel account id is required")
	}
	rec, err := m.client.ChannelAccount.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrAccountNotFound
		}
		return nil, fmt.Errorf("get channel account %s: %w", id, err)
	}
	item, err := toAccount(rec)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// FindByChannelTypeAndAccountKey returns one channel account by logical channel type and account key.
func (m *Manager) FindByChannelTypeAndAccountKey(
	ctx context.Context,
	channelType, accountKey string,
) (*ChannelAccount, error) {
	channelType = strings.TrimSpace(strings.ToLower(channelType))
	accountKey = strings.TrimSpace(accountKey)
	if channelType == "" {
		return nil, fmt.Errorf("channel type is required")
	}
	if accountKey == "" {
		return nil, fmt.Errorf("account key is required")
	}

	rec, err := m.client.ChannelAccount.Query().
		Where(
			channelaccount.ChannelTypeEQ(channelType),
			channelaccount.AccountKeyEQ(accountKey),
		).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrAccountNotFound
		}
		return nil, fmt.Errorf("find channel account %s/%s: %w", channelType, accountKey, err)
	}

	item, err := toAccount(rec)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// ResolveForChannelID resolves a channel account from a runtime channel identifier.
func (m *Manager) ResolveForChannelID(ctx context.Context, channelID string) (*ChannelAccount, error) {
	channelID = strings.TrimSpace(channelID)
	if channelID == "" {
		return nil, fmt.Errorf("channel id is required")
	}

	accounts, err := m.List(ctx)
	if err != nil {
		return nil, err
	}
	accounts = m.prioritizeAccountsForChannelID(channelID, accounts)

	for _, item := range accounts {
		candidates := channelRuntimeCandidates(item)
		if slices.Contains(candidates, channelID) {
			account := item
			return &account, nil
		}
	}

	return nil, ErrAccountNotFound
}

func (m *Manager) prioritizeAccountsForChannelID(channelID string, accounts []ChannelAccount) []ChannelAccount {
	if len(accounts) < 2 || m == nil || m.cfg == nil || channelID != "wechat" {
		return accounts
	}

	store, err := channelwechat.NewCredentialStore(m.cfg)
	if err != nil {
		return accounts
	}
	creds, err := store.LoadCredentials()
	if err != nil || creds == nil {
		return accounts
	}

	activeBotID := strings.TrimSpace(creds.ILinkBotID)
	if activeBotID == "" {
		return accounts
	}

	activeIndex := -1
	for i, account := range accounts {
		if strings.TrimSpace(account.ChannelType) != "wechat" {
			continue
		}
		if strings.TrimSpace(account.AccountKey) == activeBotID {
			activeIndex = i
			break
		}
	}
	if activeIndex <= 0 {
		return accounts
	}

	prioritized := make([]ChannelAccount, 0, len(accounts))
	prioritized = append(prioritized, accounts[activeIndex])
	prioritized = append(prioritized, accounts[:activeIndex]...)
	prioritized = append(prioritized, accounts[activeIndex+1:]...)
	return prioritized
}

// Create inserts a new channel account.
func (m *Manager) Create(ctx context.Context, item ChannelAccount) (*ChannelAccount, error) {
	normalized, err := normalizeAccount(item)
	if err != nil {
		return nil, err
	}
	configJSON, err := marshalMap(normalized.Config)
	if err != nil {
		return nil, err
	}
	metadataJSON, err := marshalMap(normalized.Metadata)
	if err != nil {
		return nil, err
	}
	rec, err := m.client.ChannelAccount.Create().
		SetChannelType(normalized.ChannelType).
		SetAccountKey(normalized.AccountKey).
		SetDisplayName(normalized.DisplayName).
		SetDescription(normalized.Description).
		SetEnabled(normalized.Enabled).
		SetConfigJSON(configJSON).
		SetMetadataJSON(metadataJSON).
		Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, ErrAccountExists
		}
		return nil, fmt.Errorf("create channel account: %w", err)
	}
	out, err := toAccount(rec)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// Update updates an existing channel account.
func (m *Manager) Update(ctx context.Context, id string, item ChannelAccount) (*ChannelAccount, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("channel account id is required")
	}
	normalized, err := normalizeAccount(item)
	if err != nil {
		return nil, err
	}
	configJSON, err := marshalMap(normalized.Config)
	if err != nil {
		return nil, err
	}
	metadataJSON, err := marshalMap(normalized.Metadata)
	if err != nil {
		return nil, err
	}
	rec, err := m.client.ChannelAccount.UpdateOneID(id).
		SetChannelType(normalized.ChannelType).
		SetAccountKey(normalized.AccountKey).
		SetDisplayName(normalized.DisplayName).
		SetDescription(normalized.Description).
		SetEnabled(normalized.Enabled).
		SetConfigJSON(configJSON).
		SetMetadataJSON(metadataJSON).
		Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrAccountNotFound
		}
		if ent.IsConstraintError(err) {
			return nil, ErrAccountExists
		}
		return nil, fmt.Errorf("update channel account %s: %w", id, err)
	}
	out, err := toAccount(rec)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// Delete removes one channel account by ID.
func (m *Manager) Delete(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("channel account id is required")
	}
	affected, err := m.client.ChannelAccount.Delete().Where(channelaccount.IDEQ(id)).Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete channel account %s: %w", id, err)
	}
	if affected == 0 {
		return ErrAccountNotFound
	}
	return nil
}

func normalizeAccount(item ChannelAccount) (ChannelAccount, error) {
	item.ChannelType = strings.TrimSpace(strings.ToLower(item.ChannelType))
	item.AccountKey = strings.TrimSpace(item.AccountKey)
	item.DisplayName = strings.TrimSpace(item.DisplayName)
	item.Description = strings.TrimSpace(item.Description)
	if item.ChannelType == "" {
		return ChannelAccount{}, fmt.Errorf("channel_type is required")
	}
	if item.AccountKey == "" {
		return ChannelAccount{}, fmt.Errorf("account_key is required")
	}
	if item.DisplayName == "" {
		item.DisplayName = item.AccountKey
	}
	if item.Config == nil {
		item.Config = map[string]interface{}{}
	}
	if item.Metadata == nil {
		item.Metadata = map[string]interface{}{}
	}
	return item, nil
}

func marshalMap(values map[string]interface{}) (string, error) {
	payload, err := json.Marshal(values)
	if err != nil {
		return "", fmt.Errorf("marshal map: %w", err)
	}
	return string(payload), nil
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

func toAccount(rec *ent.ChannelAccount) (ChannelAccount, error) {
	cfgMap, err := unmarshalMap(rec.ConfigJSON)
	if err != nil {
		return ChannelAccount{}, fmt.Errorf("decode channel account config %s: %w", rec.ID, err)
	}
	metadata, err := unmarshalMap(rec.MetadataJSON)
	if err != nil {
		return ChannelAccount{}, fmt.Errorf("decode channel account metadata %s: %w", rec.ID, err)
	}
	return ChannelAccount{
		ID:          rec.ID,
		ChannelType: rec.ChannelType,
		AccountKey:  rec.AccountKey,
		DisplayName: rec.DisplayName,
		Description: rec.Description,
		Enabled:     rec.Enabled,
		Config:      cfgMap,
		Metadata:    metadata,
		CreatedAt:   rec.CreatedAt,
		UpdatedAt:   rec.UpdatedAt,
	}, nil
}

func channelRuntimeCandidates(account ChannelAccount) []string {
	candidates := make([]string, 0, 3)
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if slices.Contains(candidates, value) {
			return
		}
		candidates = append(candidates, value)
	}

	add(account.ChannelType + ":" + account.AccountKey)
	add(account.AccountKey)
	add(account.ChannelType)

	return candidates
}
