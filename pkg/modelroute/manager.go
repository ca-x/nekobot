package modelroute

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"go.uber.org/zap"

	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/providerstore"
	"nekobot/pkg/storage/ent"
	"nekobot/pkg/storage/ent/modelroute"
)

var (
	ErrRouteExists   = errors.New("model route already exists")
	ErrRouteNotFound = errors.New("model route not found")
)

type ModelRoute struct {
	ID             string                 `json:"id,omitempty"`
	ModelID        string                 `json:"model_id"`
	ProviderName   string                 `json:"provider_name"`
	Enabled        bool                   `json:"enabled"`
	IsDefault      bool                   `json:"is_default"`
	WeightOverride int                    `json:"weight_override,omitempty"`
	Aliases        []string               `json:"aliases,omitempty"`
	RegexRules     []string               `json:"regex_rules,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

type Manager struct {
	cfg       *config.Config
	log       *logger.Logger
	client    *ent.Client
	providers *providerstore.Manager
}

func NewManager(cfg *config.Config, log *logger.Logger, client *ent.Client) (*Manager, error) {
	if client == nil {
		return nil, fmt.Errorf("ent client is nil")
	}
	providers, err := providerstore.NewManager(cfg, log, client)
	if err != nil {
		return nil, err
	}
	dbPath, _ := config.RuntimeDBPath(cfg)
	log.Info("Model route storage initialized", zap.String("db_path", dbPath))
	return &Manager{cfg: cfg, log: log, client: client, providers: providers}, nil
}

func NewProviderWeightReader(cfg *config.Config, log *logger.Logger, client *ent.Client) (*providerstore.Manager, error) {
	return providerstore.NewManager(cfg, log, client)
}

func (m *Manager) Create(ctx context.Context, item ModelRoute) (*ModelRoute, error) {
	normalized, err := normalize(item)
	if err != nil {
		return nil, err
	}
	aliases, err := marshalStringSlice(normalized.Aliases)
	if err != nil {
		return nil, err
	}
	regexRules, err := marshalStringSlice(normalized.RegexRules)
	if err != nil {
		return nil, err
	}
	metadata, err := marshalMetadata(normalized.Metadata)
	if err != nil {
		return nil, err
	}
	rec, err := m.client.ModelRoute.Create().
		SetModelID(normalized.ModelID).
		SetProviderName(normalized.ProviderName).
		SetEnabled(normalized.Enabled).
		SetIsDefault(normalized.IsDefault).
		SetWeightOverride(normalized.WeightOverride).
		SetAliasesJSON(aliases).
		SetRegexRulesJSON(regexRules).
		SetMetadataJSON(metadata).
		Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, ErrRouteExists
		}
		return nil, fmt.Errorf("create model route: %w", err)
	}
	out, err := toModelRoute(rec)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (m *Manager) Update(ctx context.Context, modelID, providerName string, item ModelRoute) (*ModelRoute, error) {
	normalized, err := normalize(item)
	if err != nil {
		return nil, err
	}
	aliases, err := marshalStringSlice(normalized.Aliases)
	if err != nil {
		return nil, err
	}
	regexRules, err := marshalStringSlice(normalized.RegexRules)
	if err != nil {
		return nil, err
	}
	metadata, err := marshalMetadata(normalized.Metadata)
	if err != nil {
		return nil, err
	}
	affected, err := m.client.ModelRoute.Update().
		Where(modelroute.ModelIDEQ(strings.TrimSpace(modelID)), modelroute.ProviderNameEQ(strings.TrimSpace(providerName))).
		SetEnabled(normalized.Enabled).
		SetIsDefault(normalized.IsDefault).
		SetWeightOverride(normalized.WeightOverride).
		SetAliasesJSON(aliases).
		SetRegexRulesJSON(regexRules).
		SetMetadataJSON(metadata).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("update model route: %w", err)
	}
	if affected == 0 {
		return nil, ErrRouteNotFound
	}
	rec, err := m.client.ModelRoute.Query().
		Where(modelroute.ModelIDEQ(strings.TrimSpace(modelID)), modelroute.ProviderNameEQ(strings.TrimSpace(providerName))).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrRouteNotFound
		}
		return nil, fmt.Errorf("query updated model route: %w", err)
	}
	out, err := toModelRoute(rec)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (m *Manager) ListByModel(ctx context.Context, modelID string) ([]ModelRoute, error) {
	recs, err := m.client.ModelRoute.Query().
		Where(modelroute.ModelIDEQ(strings.TrimSpace(modelID))).
		Order(ent.Asc(modelroute.FieldProviderName)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list routes: %w", err)
	}
	items := make([]ModelRoute, 0, len(recs))
	for _, rec := range recs {
		item, err := toModelRoute(rec)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (m *Manager) ListByModels(ctx context.Context, modelIDs []string) (map[string][]ModelRoute, error) {
	trimmed := make([]string, 0, len(modelIDs))
	seen := make(map[string]struct{}, len(modelIDs))
	for _, modelID := range modelIDs {
		name := strings.TrimSpace(modelID)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		trimmed = append(trimmed, name)
	}

	routesByModel := make(map[string][]ModelRoute, len(trimmed))
	for _, modelID := range trimmed {
		routesByModel[modelID] = []ModelRoute{}
	}
	if len(trimmed) == 0 {
		return routesByModel, nil
	}

	recs, err := m.client.ModelRoute.Query().
		Where(modelroute.ModelIDIn(trimmed...)).
		Order(ent.Asc(modelroute.FieldModelID), ent.Asc(modelroute.FieldProviderName)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list routes by models: %w", err)
	}

	for _, rec := range recs {
		item, err := toModelRoute(rec)
		if err != nil {
			return nil, err
		}
		routesByModel[item.ModelID] = append(routesByModel[item.ModelID], item)
	}

	return routesByModel, nil
}

func (m *Manager) ResolveInput(ctx context.Context, input string) (*ModelRoute, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return nil, fmt.Errorf("model input is required")
	}

	routes, err := m.client.ModelRoute.Query().Where(modelroute.EnabledEQ(true)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query model routes: %w", err)
	}
	for _, rec := range routes {
		item, err := toModelRoute(rec)
		if err != nil {
			return nil, err
		}
		if item.ModelID == trimmed {
			return &item, nil
		}
		for _, alias := range item.Aliases {
			if alias == trimmed {
				return &item, nil
			}
		}
		for _, rule := range item.RegexRules {
			matched, err := regexp.MatchString(rule, trimmed)
			if err != nil {
				return nil, fmt.Errorf("match regex rule %q: %w", rule, err)
			}
			if matched {
				return &item, nil
			}
		}
	}
	return nil, ErrRouteNotFound
}

func (m *Manager) DefaultRoute(ctx context.Context, modelID string) (*ModelRoute, error) {
	rec, err := m.client.ModelRoute.Query().
		Where(modelroute.ModelIDEQ(strings.TrimSpace(modelID)), modelroute.IsDefaultEQ(true), modelroute.EnabledEQ(true)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrRouteNotFound
		}
		return nil, fmt.Errorf("query default route: %w", err)
	}
	item, err := toModelRoute(rec)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (m *Manager) EffectiveWeight(ctx context.Context, item ModelRoute) (int, error) {
	if item.WeightOverride > 0 {
		return item.WeightOverride, nil
	}
	provider, err := m.providers.Get(ctx, item.ProviderName)
	if err != nil {
		return 0, err
	}
	return provider.DefaultWeight, nil
}

func normalize(item ModelRoute) (ModelRoute, error) {
	item.ModelID = strings.TrimSpace(item.ModelID)
	item.ProviderName = strings.TrimSpace(item.ProviderName)
	if item.ModelID == "" {
		return ModelRoute{}, fmt.Errorf("model_id is required")
	}
	if item.ProviderName == "" {
		return ModelRoute{}, fmt.Errorf("provider_name is required")
	}
	item.Aliases = normalizeStringSlice(item.Aliases)
	item.RegexRules = normalizeStringSlice(item.RegexRules)
	if item.Metadata == nil {
		item.Metadata = map[string]interface{}{}
	}
	return item, nil
}

func toModelRoute(rec *ent.ModelRoute) (ModelRoute, error) {
	aliases, err := unmarshalStringSlice(rec.AliasesJSON)
	if err != nil {
		return ModelRoute{}, fmt.Errorf("decode route aliases %s: %w", rec.ID, err)
	}
	regexRules, err := unmarshalStringSlice(rec.RegexRulesJSON)
	if err != nil {
		return ModelRoute{}, fmt.Errorf("decode route regex %s: %w", rec.ID, err)
	}
	metadata, err := unmarshalMetadata(rec.MetadataJSON)
	if err != nil {
		return ModelRoute{}, fmt.Errorf("decode route metadata %s: %w", rec.ID, err)
	}
	return ModelRoute{
		ID:             rec.ID,
		ModelID:        rec.ModelID,
		ProviderName:   rec.ProviderName,
		Enabled:        rec.Enabled,
		IsDefault:      rec.IsDefault,
		WeightOverride: rec.WeightOverride,
		Aliases:        aliases,
		RegexRules:     regexRules,
		Metadata:       metadata,
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

func marshalMetadata(values map[string]interface{}) (string, error) {
	payload, err := json.Marshal(values)
	if err != nil {
		return "", fmt.Errorf("marshal metadata: %w", err)
	}
	return string(payload), nil
}

func unmarshalMetadata(raw string) (map[string]interface{}, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return map[string]interface{}{}, nil
	}
	var values map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &values); err != nil {
		return nil, fmt.Errorf("unmarshal metadata: %w", err)
	}
	if values == nil {
		values = map[string]interface{}{}
	}
	return values, nil
}
