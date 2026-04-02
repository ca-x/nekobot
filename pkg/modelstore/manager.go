package modelstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/storage/ent"
	"nekobot/pkg/storage/ent/modelcatalog"
)

var (
	ErrModelExists   = errors.New("model already exists")
	ErrModelNotFound = errors.New("model not found")
)

type ModelCatalog struct {
	ID            string   `json:"id,omitempty"`
	ModelID       string   `json:"model_id"`
	DisplayName   string   `json:"display_name"`
	Developer     string   `json:"developer,omitempty"`
	Family        string   `json:"family,omitempty"`
	Type          string   `json:"type,omitempty"`
	Capabilities  []string `json:"capabilities,omitempty"`
	CatalogSource string   `json:"catalog_source,omitempty"`
	Enabled       bool     `json:"enabled"`
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
	dbPath, _ := config.RuntimeDBPath(cfg)
	log.Info("Model catalog storage initialized", zap.String("db_path", dbPath))
	return &Manager{cfg: cfg, log: log, client: client}, nil
}

func (m *Manager) Create(ctx context.Context, item ModelCatalog) (*ModelCatalog, error) {
	normalized, err := normalize(item)
	if err != nil {
		return nil, err
	}
	caps, err := marshalStringSlice(normalized.Capabilities)
	if err != nil {
		return nil, err
	}
	rec, err := m.client.ModelCatalog.Create().
		SetModelID(normalized.ModelID).
		SetDisplayName(normalized.DisplayName).
		SetDeveloper(normalized.Developer).
		SetFamily(normalized.Family).
		SetType(normalized.Type).
		SetCapabilitiesJSON(caps).
		SetCatalogSource(normalized.CatalogSource).
		SetEnabled(normalized.Enabled).
		Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, ErrModelExists
		}
		return nil, fmt.Errorf("create model: %w", err)
	}
	out, err := toModelCatalog(rec)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (m *Manager) Update(ctx context.Context, modelID string, item ModelCatalog) (*ModelCatalog, error) {
	normalized, err := normalize(item)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(modelID) == "" {
		return nil, fmt.Errorf("model_id is required")
	}
	caps, err := marshalStringSlice(normalized.Capabilities)
	if err != nil {
		return nil, err
	}
	affected, err := m.client.ModelCatalog.Update().
		Where(modelcatalog.ModelIDEQ(strings.TrimSpace(modelID))).
		SetDisplayName(normalized.DisplayName).
		SetDeveloper(normalized.Developer).
		SetFamily(normalized.Family).
		SetType(normalized.Type).
		SetCapabilitiesJSON(caps).
		SetCatalogSource(normalized.CatalogSource).
		SetEnabled(normalized.Enabled).
		Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrModelNotFound
		}
		return nil, fmt.Errorf("update model %s: %w", modelID, err)
	}
	if affected == 0 {
		return nil, ErrModelNotFound
	}
	rec, err := m.client.ModelCatalog.Query().Where(modelcatalog.ModelIDEQ(strings.TrimSpace(modelID))).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrModelNotFound
		}
		return nil, fmt.Errorf("query updated model %s: %w", modelID, err)
	}
	out, err := toModelCatalog(rec)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (m *Manager) List(ctx context.Context) ([]ModelCatalog, error) {
	recs, err := m.client.ModelCatalog.Query().
		Order(ent.Asc(modelcatalog.FieldModelID)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list models: %w", err)
	}
	items := make([]ModelCatalog, 0, len(recs))
	for _, rec := range recs {
		item, err := toModelCatalog(rec)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (m *Manager) Get(ctx context.Context, modelID string) (*ModelCatalog, error) {
	rec, err := m.client.ModelCatalog.Query().
		Where(modelcatalog.ModelIDEQ(strings.TrimSpace(modelID))).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrModelNotFound
		}
		return nil, fmt.Errorf("get model %s: %w", modelID, err)
	}
	item, err := toModelCatalog(rec)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func normalize(item ModelCatalog) (ModelCatalog, error) {
	item.ModelID = strings.TrimSpace(item.ModelID)
	item.DisplayName = strings.TrimSpace(item.DisplayName)
	item.Developer = strings.TrimSpace(item.Developer)
	item.Family = strings.TrimSpace(item.Family)
	item.Type = strings.TrimSpace(item.Type)
	item.CatalogSource = strings.TrimSpace(item.CatalogSource)
	if item.ModelID == "" {
		return ModelCatalog{}, fmt.Errorf("model_id is required")
	}
	if item.DisplayName == "" {
		item.DisplayName = item.ModelID
	}
	if item.CatalogSource == "" {
		item.CatalogSource = "builtin"
	}
	item.Capabilities = normalizeStringSlice(item.Capabilities)
	return item, nil
}

func toModelCatalog(rec *ent.ModelCatalog) (ModelCatalog, error) {
	caps, err := unmarshalStringSlice(rec.CapabilitiesJSON)
	if err != nil {
		return ModelCatalog{}, fmt.Errorf("decode model capabilities %s: %w", rec.ID, err)
	}
	return ModelCatalog{
		ID:            rec.ID,
		ModelID:       rec.ModelID,
		DisplayName:   rec.DisplayName,
		Developer:     rec.Developer,
		Family:        rec.Family,
		Type:          rec.Type,
		Capabilities:  caps,
		CatalogSource: rec.CatalogSource,
		Enabled:       rec.Enabled,
	}, nil
}

func normalizeStringSlice(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
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

func marshalStringSlice(values []string) (string, error) {
	payload, err := json.Marshal(normalizeStringSlice(values))
	if err != nil {
		return "", fmt.Errorf("marshal string slice: %w", err)
	}
	return string(payload), nil
}

func unmarshalStringSlice(raw string) ([]string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return []string{}, nil
	}
	var values []string
	if err := json.Unmarshal([]byte(trimmed), &values); err != nil {
		return nil, fmt.Errorf("unmarshal string slice: %w", err)
	}
	return normalizeStringSlice(values), nil
}
