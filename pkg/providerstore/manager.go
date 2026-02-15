package providerstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"go.uber.org/zap"

	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/storage/ent"
	"nekobot/pkg/storage/ent/provider"
)

var (
	// ErrProviderExists indicates a provider with the same name already exists.
	ErrProviderExists = errors.New("provider already exists")
	// ErrProviderNotFound indicates the requested provider does not exist.
	ErrProviderNotFound = errors.New("provider not found")
)

// Manager persists provider profiles in SQLite and keeps runtime config in sync.
type Manager struct {
	cfg    *config.Config
	log    *logger.Logger
	client *ent.Client
	mu     sync.Mutex
}

// NewManager creates provider storage with an injected shared Ent client.
func NewManager(cfg *config.Config, log *logger.Logger, client *ent.Client) (*Manager, error) {
	if client == nil {
		return nil, fmt.Errorf("ent client is nil")
	}
	m := &Manager{
		cfg:    cfg,
		log:    log,
		client: client,
	}

	if err := m.syncConfig(context.Background()); err != nil {
		return nil, err
	}

	dbPath, _ := config.RuntimeDBPath(cfg)
	log.Info("Provider storage initialized", zap.String("db_path", dbPath))
	return m, nil
}

// Close releases manager resources. Shared Ent client is closed by config module.
func (m *Manager) Close() error {
	return nil
}

// List returns all providers sorted by name.
func (m *Manager) List(ctx context.Context) ([]config.ProviderProfile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	providers, err := m.listLocked(ctx)
	if err != nil {
		return nil, err
	}
	return cloneProviders(providers), nil
}

// Get returns one provider by name.
func (m *Manager) Get(ctx context.Context, name string) (*config.ProviderProfile, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("provider name is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	rec, err := m.getRecordLocked(ctx, name)
	if err != nil {
		return nil, err
	}
	profile, err := toConfigProvider(rec)
	if err != nil {
		return nil, err
	}
	return &profile, nil
}

// Create inserts a new provider profile.
func (m *Manager) Create(ctx context.Context, profile config.ProviderProfile) (*config.ProviderProfile, error) {
	normalized, err := normalizeProvider(profile)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	exists, err := m.existsLocked(ctx, normalized.Name)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrProviderExists
	}

	if err := m.insertLocked(ctx, normalized); err != nil {
		return nil, err
	}

	if err := m.syncConfigLocked(ctx); err != nil {
		return nil, err
	}

	created := normalized
	return &created, nil
}

// Update updates an existing provider. Name can be changed by setting profile.Name.
func (m *Manager) Update(ctx context.Context, name string, profile config.ProviderProfile) (*config.ProviderProfile, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("provider name is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	current, err := m.getRecordLocked(ctx, name)
	if err != nil {
		return nil, err
	}

	normalized, err := normalizeProvider(profile)
	if err != nil {
		return nil, err
	}
	if normalized.APIKey == "" {
		normalized.APIKey = current.APIKey
	}
	if normalized.Name == "" {
		normalized.Name = current.Name
	}

	if normalized.Name != name {
		exists, err := m.existsLocked(ctx, normalized.Name)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, ErrProviderExists
		}
	}

	modelsJSON, err := marshalModels(normalized.Models)
	if err != nil {
		return nil, err
	}

	_, err = m.client.Provider.UpdateOneID(current.ID).
		SetName(normalized.Name).
		SetProviderKind(normalized.ProviderKind).
		SetAPIKey(normalized.APIKey).
		SetAPIBase(normalized.APIBase).
		SetProxy(normalized.Proxy).
		SetModelsJSON(modelsJSON).
		SetDefaultModel(normalized.DefaultModel).
		SetTimeout(normalized.Timeout).
		Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, ErrProviderExists
		}
		if ent.IsNotFound(err) {
			return nil, ErrProviderNotFound
		}
		return nil, fmt.Errorf("update provider: %w", err)
	}

	if err := m.syncConfigLocked(ctx); err != nil {
		return nil, err
	}

	updated := normalized
	return &updated, nil
}

// Delete removes a provider by name.
func (m *Manager) Delete(ctx context.Context, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("provider name is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	affected, err := m.client.Provider.Delete().Where(provider.NameEQ(name)).Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete provider: %w", err)
	}
	if affected == 0 {
		return ErrProviderNotFound
	}

	return m.syncConfigLocked(ctx)
}

func (m *Manager) syncConfig(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.syncConfigLocked(ctx)
}

func (m *Manager) syncConfigLocked(ctx context.Context) error {
	providers, err := m.listLocked(ctx)
	if err != nil {
		return err
	}
	m.cfg.Providers = cloneProviders(providers)
	return nil
}

func (m *Manager) listLocked(ctx context.Context) ([]config.ProviderProfile, error) {
	records, err := m.client.Provider.Query().Order(ent.Asc(provider.FieldName)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query providers: %w", err)
	}

	providers := make([]config.ProviderProfile, 0, len(records))
	for _, rec := range records {
		profile, err := toConfigProvider(rec)
		if err != nil {
			return nil, err
		}
		providers = append(providers, profile)
	}
	return providers, nil
}

func (m *Manager) getRecordLocked(ctx context.Context, name string) (*ent.Provider, error) {
	rec, err := m.client.Provider.Query().Where(provider.NameEQ(name)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrProviderNotFound
		}
		return nil, fmt.Errorf("get provider: %w", err)
	}
	return rec, nil
}

func (m *Manager) existsLocked(ctx context.Context, name string) (bool, error) {
	exists, err := m.client.Provider.Query().Where(provider.NameEQ(name)).Exist(ctx)
	if err != nil {
		return false, fmt.Errorf("check provider exists: %w", err)
	}
	return exists, nil
}

func (m *Manager) insertLocked(ctx context.Context, profile config.ProviderProfile) error {
	modelsJSON, err := marshalModels(profile.Models)
	if err != nil {
		return err
	}
	_, err = m.client.Provider.Create().
		SetName(profile.Name).
		SetProviderKind(profile.ProviderKind).
		SetAPIKey(profile.APIKey).
		SetAPIBase(profile.APIBase).
		SetProxy(profile.Proxy).
		SetModelsJSON(modelsJSON).
		SetDefaultModel(profile.DefaultModel).
		SetTimeout(profile.Timeout).
		Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return ErrProviderExists
		}
		return fmt.Errorf("insert provider: %w", err)
	}
	return nil
}

func toConfigProvider(rec *ent.Provider) (config.ProviderProfile, error) {
	if rec == nil {
		return config.ProviderProfile{}, fmt.Errorf("provider record is nil")
	}
	models, err := unmarshalModels(rec.ModelsJSON)
	if err != nil {
		return config.ProviderProfile{}, err
	}
	profile := config.ProviderProfile{
		Name:         rec.Name,
		ProviderKind: rec.ProviderKind,
		APIKey:       rec.APIKey,
		APIBase:      rec.APIBase,
		Proxy:        rec.Proxy,
		Models:       models,
		DefaultModel: rec.DefaultModel,
		Timeout:      rec.Timeout,
	}
	normalized, err := normalizeProvider(profile)
	if err != nil {
		return config.ProviderProfile{}, err
	}
	return normalized, nil
}

func normalizeProvider(profile config.ProviderProfile) (config.ProviderProfile, error) {
	profile.Name = strings.TrimSpace(profile.Name)
	if profile.Name == "" {
		return config.ProviderProfile{}, fmt.Errorf("provider name is required")
	}

	profile.ProviderKind = strings.ToLower(strings.TrimSpace(profile.ProviderKind))
	if profile.ProviderKind == "" {
		return config.ProviderProfile{}, fmt.Errorf("provider_kind is required")
	}

	profile.APIKey = strings.TrimSpace(profile.APIKey)
	profile.APIBase = strings.TrimSpace(profile.APIBase)
	profile.Proxy = strings.TrimSpace(profile.Proxy)
	profile.DefaultModel = strings.TrimSpace(profile.DefaultModel)
	profile.Models = normalizeModelList(profile.Models)

	if profile.DefaultModel != "" {
		if !containsString(profile.Models, profile.DefaultModel) {
			profile.Models = append([]string{profile.DefaultModel}, profile.Models...)
		}
	}
	if profile.DefaultModel == "" && len(profile.Models) > 0 {
		profile.DefaultModel = profile.Models[0]
	}

	if profile.Timeout <= 0 {
		profile.Timeout = 60
	}
	return profile, nil
}

func normalizeModelList(models []string) []string {
	if len(models) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(models))
	result := make([]string, 0, len(models))
	for _, model := range models {
		trimmed := strings.TrimSpace(model)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func marshalModels(models []string) (string, error) {
	data, err := json.Marshal(normalizeModelList(models))
	if err != nil {
		return "", fmt.Errorf("marshal models: %w", err)
	}
	return string(data), nil
}

func unmarshalModels(raw string) ([]string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return []string{}, nil
	}
	var models []string
	if err := json.Unmarshal([]byte(trimmed), &models); err != nil {
		return nil, fmt.Errorf("unmarshal models: %w", err)
	}
	return normalizeModelList(models), nil
}

func cloneProviders(src []config.ProviderProfile) []config.ProviderProfile {
	if len(src) == 0 {
		return []config.ProviderProfile{}
	}
	dst := make([]config.ProviderProfile, len(src))
	for i := range src {
		dst[i] = src[i]
		dst[i].Models = append([]string(nil), src[i].Models...)
	}
	return dst
}
