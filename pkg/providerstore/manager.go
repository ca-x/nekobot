package providerstore

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"go.uber.org/zap"

	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/providerregistry"
	"nekobot/pkg/storage/ent"
	"nekobot/pkg/storage/ent/provider"
)

var (
	// ErrProviderExists indicates a provider with the same name already exists.
	ErrProviderExists = errors.New("provider already exists")
	// ErrProviderNotFound indicates the requested provider does not exist.
	ErrProviderNotFound = errors.New("provider not found")
	// ErrInvalidProvider indicates the supplied provider profile is invalid.
	ErrInvalidProvider = errors.New("invalid provider")
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

	merged := config.ProviderProfile{
		Name:          strings.TrimSpace(profile.Name),
		ProviderKind:  strings.TrimSpace(profile.ProviderKind),
		APIKey:        strings.TrimSpace(profile.APIKey),
		APIBase:       strings.TrimSpace(profile.APIBase),
		Proxy:         strings.TrimSpace(profile.Proxy),
		DefaultWeight: profile.DefaultWeight,
		Enabled:       profile.Enabled,
		Timeout:       profile.Timeout,
	}
	if merged.Name == "" {
		merged.Name = current.Name
	}
	if merged.ProviderKind == "" {
		merged.ProviderKind = current.ProviderKind
	}
	if merged.APIKey == "" {
		merged.APIKey = current.APIKey
	}
	if merged.APIBase == "" {
		merged.APIBase = current.APIBase
	}
	if merged.Proxy == "" {
		merged.Proxy = current.Proxy
	}
	if merged.DefaultWeight == 0 {
		merged.DefaultWeight = current.DefaultWeight
	}
	if merged.Timeout == 0 {
		merged.Timeout = current.Timeout
	}

	normalized, err := normalizeProvider(merged)
	if err != nil {
		return nil, err
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

	_, err = m.client.Provider.UpdateOneID(current.ID).
		SetName(normalized.Name).
		SetProviderKind(normalized.ProviderKind).
		SetAPIKey(normalized.APIKey).
		SetAPIBase(normalized.APIBase).
		SetProxy(normalized.Proxy).
		SetDefaultWeight(normalized.DefaultWeight).
		SetEnabled(normalized.Enabled).
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
	_, err := m.client.Provider.Create().
		SetName(profile.Name).
		SetProviderKind(profile.ProviderKind).
		SetAPIKey(profile.APIKey).
		SetAPIBase(profile.APIBase).
		SetProxy(profile.Proxy).
		SetDefaultWeight(profile.DefaultWeight).
		SetEnabled(profile.Enabled).
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
	return config.ProviderProfile{
		Name:          rec.Name,
		ProviderKind:  rec.ProviderKind,
		APIKey:        rec.APIKey,
		APIBase:       rec.APIBase,
		Proxy:         rec.Proxy,
		DefaultWeight: rec.DefaultWeight,
		Enabled:       rec.Enabled,
		Timeout:       rec.Timeout,
	}, nil
}

func normalizeProvider(profile config.ProviderProfile) (config.ProviderProfile, error) {
	profile.Name = strings.TrimSpace(profile.Name)
	if profile.Name == "" {
		return config.ProviderProfile{}, fmt.Errorf("%w: provider name is required", ErrInvalidProvider)
	}

	profile.ProviderKind = strings.ToLower(strings.TrimSpace(profile.ProviderKind))
	if profile.ProviderKind == "" {
		return config.ProviderProfile{}, fmt.Errorf("%w: provider_kind is required", ErrInvalidProvider)
	}

	profile.APIKey = strings.TrimSpace(profile.APIKey)
	profile.APIBase = strings.TrimSpace(profile.APIBase)
	profile.Proxy = strings.TrimSpace(profile.Proxy)
	profile.Models = []string{}
	profile.DefaultModel = ""
	if profile.DefaultWeight <= 0 {
		profile.DefaultWeight = 1
	}
	if !profile.Enabled {
		// false is a valid explicit value; keep as-is.
	}

	if profile.Timeout <= 0 {
		profile.Timeout = 60
	}

	if meta, ok := providerregistry.Get(profile.ProviderKind); ok {
		for _, field := range meta.AuthFields {
			if !field.Required {
				continue
			}
			switch field.Key {
			case "api_key":
				if profile.APIKey == "" {
					return config.ProviderProfile{}, fmt.Errorf("%w: api key is required for %s", ErrInvalidProvider, profile.ProviderKind)
				}
			}
		}
	}
	return profile, nil
}

func cloneProviders(src []config.ProviderProfile) []config.ProviderProfile {
	if len(src) == 0 {
		return []config.ProviderProfile{}
	}
	dst := make([]config.ProviderProfile, len(src))
	for i := range src {
		dst[i] = src[i]
		dst[i].Models = []string{}
		dst[i].DefaultModel = ""
	}
	return dst
}
