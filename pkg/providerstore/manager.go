package providerstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	_ "github.com/lib-x/entsqlite"
	"go.uber.org/zap"

	"nekobot/pkg/config"
	"nekobot/pkg/logger"
)

// Reuse the existing runtime database file so WebUI data is consolidated.
const defaultDBName = "tool_sessions.db"

var (
	// ErrProviderExists indicates a provider with the same name already exists.
	ErrProviderExists = errors.New("provider already exists")
	// ErrProviderNotFound indicates the requested provider does not exist.
	ErrProviderNotFound = errors.New("provider not found")
)

// Manager persists provider profiles in SQLite and keeps runtime config in sync.
type Manager struct {
	cfg *config.Config
	log *logger.Logger
	db  *sql.DB
	mu  sync.Mutex
}

// NewManager creates provider storage and initializes data from config when needed.
func NewManager(cfg *config.Config, log *logger.Logger) (*Manager, error) {
	workspace := strings.TrimSpace(cfg.WorkspacePath())
	if workspace == "" {
		return nil, fmt.Errorf("workspace path is empty")
	}
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		return nil, fmt.Errorf("create workspace directory: %w", err)
	}

	dbPath := filepath.Join(workspace, defaultDBName)
	dsn := fmt.Sprintf("file:%s?cache=shared&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(10000)", dbPath)
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("open providers database: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping providers database: %w", err)
	}

	m := &Manager{
		cfg: cfg,
		log: log,
		db:  db,
	}

	ctx := context.Background()
	if err := m.initSchema(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := m.bootstrap(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	log.Info("Provider storage initialized", zap.String("db_path", dbPath))
	return m, nil
}

// Close releases DB resources.
func (m *Manager) Close() error {
	if m == nil || m.db == nil {
		return nil
	}
	return m.db.Close()
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

	row := m.db.QueryRowContext(ctx, `
		SELECT name, provider_kind, api_key, api_base, proxy, models_json, default_model, timeout
		FROM providers
		WHERE name = ?
	`, name)
	profile, err := scanProviderRow(row.Scan)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrProviderNotFound
		}
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

	current, err := m.getLocked(ctx, name)
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

	result, err := m.db.ExecContext(ctx, `
		UPDATE providers
		SET
			name = ?,
			provider_kind = ?,
			api_key = ?,
			api_base = ?,
			proxy = ?,
			models_json = ?,
			default_model = ?,
			timeout = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE name = ?
	`,
		normalized.Name,
		normalized.ProviderKind,
		normalized.APIKey,
		normalized.APIBase,
		normalized.Proxy,
		modelsJSON,
		normalized.DefaultModel,
		normalized.Timeout,
		name,
	)
	if err != nil {
		return nil, fmt.Errorf("update provider: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return nil, ErrProviderNotFound
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

	result, err := m.db.ExecContext(ctx, `DELETE FROM providers WHERE name = ?`, name)
	if err != nil {
		return fmt.Errorf("delete provider: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return ErrProviderNotFound
	}

	return m.syncConfigLocked(ctx)
}

func (m *Manager) initSchema(ctx context.Context) error {
	_, err := m.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS providers (
			name TEXT PRIMARY KEY,
			provider_kind TEXT NOT NULL,
			api_key TEXT NOT NULL DEFAULT '',
			api_base TEXT NOT NULL DEFAULT '',
			proxy TEXT NOT NULL DEFAULT '',
			models_json TEXT NOT NULL DEFAULT '[]',
			default_model TEXT NOT NULL DEFAULT '',
			timeout INTEGER NOT NULL DEFAULT 60,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("create providers schema: %w", err)
	}
	return nil
}

func (m *Manager) bootstrap(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	providers, err := m.listLocked(ctx)
	if err != nil {
		return err
	}

	if len(providers) == 0 && len(m.cfg.Providers) > 0 {
		for _, raw := range m.cfg.Providers {
			profile, err := normalizeProvider(raw)
			if err != nil {
				m.log.Warn("Skip invalid provider during database bootstrap", zap.String("provider", raw.Name), zap.Error(err))
				continue
			}
			if err := m.insertLocked(ctx, profile); err != nil {
				return err
			}
		}

		providers, err = m.listLocked(ctx)
		if err != nil {
			return err
		}
	}

	if len(providers) > 0 {
		m.cfg.Providers = cloneProviders(providers)
	}

	return nil
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
	rows, err := m.db.QueryContext(ctx, `
		SELECT name, provider_kind, api_key, api_base, proxy, models_json, default_model, timeout
		FROM providers
		ORDER BY name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("query providers: %w", err)
	}
	defer rows.Close()

	providers := make([]config.ProviderProfile, 0)
	for rows.Next() {
		profile, err := scanProviderRow(rows.Scan)
		if err != nil {
			return nil, err
		}
		providers = append(providers, profile)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate providers: %w", err)
	}

	return providers, nil
}

func (m *Manager) getLocked(ctx context.Context, name string) (*config.ProviderProfile, error) {
	row := m.db.QueryRowContext(ctx, `
		SELECT name, provider_kind, api_key, api_base, proxy, models_json, default_model, timeout
		FROM providers
		WHERE name = ?
	`, name)
	profile, err := scanProviderRow(row.Scan)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrProviderNotFound
		}
		return nil, err
	}
	return &profile, nil
}

func (m *Manager) existsLocked(ctx context.Context, name string) (bool, error) {
	var count int
	if err := m.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM providers WHERE name = ?`, name).Scan(&count); err != nil {
		return false, fmt.Errorf("check provider exists: %w", err)
	}
	return count > 0, nil
}

func (m *Manager) insertLocked(ctx context.Context, profile config.ProviderProfile) error {
	modelsJSON, err := marshalModels(profile.Models)
	if err != nil {
		return err
	}
	if _, err := m.db.ExecContext(ctx, `
		INSERT INTO providers(name, provider_kind, api_key, api_base, proxy, models_json, default_model, timeout)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`,
		profile.Name,
		profile.ProviderKind,
		profile.APIKey,
		profile.APIBase,
		profile.Proxy,
		modelsJSON,
		profile.DefaultModel,
		profile.Timeout,
	); err != nil {
		return fmt.Errorf("insert provider: %w", err)
	}
	return nil
}

func scanProviderRow(scan func(dest ...interface{}) error) (config.ProviderProfile, error) {
	var (
		name         string
		providerKind string
		apiKey       string
		apiBase      string
		proxy        string
		modelsJSON   string
		defaultModel string
		timeout      int
	)
	if err := scan(&name, &providerKind, &apiKey, &apiBase, &proxy, &modelsJSON, &defaultModel, &timeout); err != nil {
		return config.ProviderProfile{}, err
	}
	models, err := unmarshalModels(modelsJSON)
	if err != nil {
		return config.ProviderProfile{}, err
	}
	profile := config.ProviderProfile{
		Name:         name,
		ProviderKind: providerKind,
		APIKey:       apiKey,
		APIBase:      apiBase,
		Proxy:        proxy,
		Models:       models,
		DefaultModel: defaultModel,
		Timeout:      timeout,
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
