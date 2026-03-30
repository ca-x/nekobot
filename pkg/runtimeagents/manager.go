package runtimeagents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"go.uber.org/zap"

	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/storage/ent"
	"nekobot/pkg/storage/ent/agentruntime"
)

var (
	// ErrRuntimeExists indicates a runtime with the same name already exists.
	ErrRuntimeExists = errors.New("agent runtime already exists")
	// ErrRuntimeNotFound indicates the requested runtime does not exist.
	ErrRuntimeNotFound = errors.New("agent runtime not found")
)

// Manager persists agent runtime definitions in the shared runtime database.
type Manager struct {
	cfg    *config.Config
	log    *logger.Logger
	client *ent.Client
}

// NewManager creates an agent runtime manager backed by the runtime database.
func NewManager(cfg *config.Config, log *logger.Logger, client *ent.Client) (*Manager, error) {
	if client == nil {
		return nil, fmt.Errorf("ent client is nil")
	}
	mgr := &Manager{cfg: cfg, log: log, client: client}
	dbPath, _ := config.RuntimeDBPath(cfg)
	log.Info("Agent runtime storage initialized", zap.String("db_path", dbPath))
	return mgr, nil
}

// Close releases manager resources. Shared Ent client is closed elsewhere.
func (m *Manager) Close() error {
	return nil
}

// List returns all runtimes ordered by display fields.
func (m *Manager) List(ctx context.Context) ([]AgentRuntime, error) {
	recs, err := m.client.AgentRuntime.Query().
		Order(ent.Asc(agentruntime.FieldName), ent.Asc(agentruntime.FieldUpdatedAt)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list agent runtimes: %w", err)
	}
	result := make([]AgentRuntime, 0, len(recs))
	for _, rec := range recs {
		item, err := toRuntime(rec)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

// Get returns one runtime by ID.
func (m *Manager) Get(ctx context.Context, id string) (*AgentRuntime, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("runtime id is required")
	}
	rec, err := m.client.AgentRuntime.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrRuntimeNotFound
		}
		return nil, fmt.Errorf("get agent runtime %s: %w", id, err)
	}
	item, err := toRuntime(rec)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// Create inserts a new runtime.
func (m *Manager) Create(ctx context.Context, item AgentRuntime) (*AgentRuntime, error) {
	normalized, err := normalizeRuntime(item)
	if err != nil {
		return nil, err
	}
	skillsJSON, err := marshalStringSlice(normalized.Skills)
	if err != nil {
		return nil, err
	}
	toolsJSON, err := marshalStringSlice(normalized.Tools)
	if err != nil {
		return nil, err
	}
	policyJSON, err := marshalMap(normalized.Policy)
	if err != nil {
		return nil, err
	}
	rec, err := m.client.AgentRuntime.Create().
		SetName(normalized.Name).
		SetDisplayName(normalized.DisplayName).
		SetDescription(normalized.Description).
		SetEnabled(normalized.Enabled).
		SetProvider(normalized.Provider).
		SetModel(normalized.Model).
		SetPromptID(normalized.PromptID).
		SetSkillsJSON(skillsJSON).
		SetToolsJSON(toolsJSON).
		SetPolicyJSON(policyJSON).
		Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, ErrRuntimeExists
		}
		return nil, fmt.Errorf("create agent runtime: %w", err)
	}
	out, err := toRuntime(rec)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// Update updates an existing runtime by ID.
func (m *Manager) Update(ctx context.Context, id string, item AgentRuntime) (*AgentRuntime, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("runtime id is required")
	}
	normalized, err := normalizeRuntime(item)
	if err != nil {
		return nil, err
	}
	skillsJSON, err := marshalStringSlice(normalized.Skills)
	if err != nil {
		return nil, err
	}
	toolsJSON, err := marshalStringSlice(normalized.Tools)
	if err != nil {
		return nil, err
	}
	policyJSON, err := marshalMap(normalized.Policy)
	if err != nil {
		return nil, err
	}
	rec, err := m.client.AgentRuntime.UpdateOneID(id).
		SetName(normalized.Name).
		SetDisplayName(normalized.DisplayName).
		SetDescription(normalized.Description).
		SetEnabled(normalized.Enabled).
		SetProvider(normalized.Provider).
		SetModel(normalized.Model).
		SetPromptID(normalized.PromptID).
		SetSkillsJSON(skillsJSON).
		SetToolsJSON(toolsJSON).
		SetPolicyJSON(policyJSON).
		Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrRuntimeNotFound
		}
		if ent.IsConstraintError(err) {
			return nil, ErrRuntimeExists
		}
		return nil, fmt.Errorf("update agent runtime %s: %w", id, err)
	}
	out, err := toRuntime(rec)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// Delete removes one runtime by ID.
func (m *Manager) Delete(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("runtime id is required")
	}
	affected, err := m.client.AgentRuntime.Delete().Where(agentruntime.IDEQ(id)).Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete agent runtime %s: %w", id, err)
	}
	if affected == 0 {
		return ErrRuntimeNotFound
	}
	return nil
}

func normalizeRuntime(item AgentRuntime) (AgentRuntime, error) {
	item.Name = strings.TrimSpace(item.Name)
	item.DisplayName = strings.TrimSpace(item.DisplayName)
	item.Description = strings.TrimSpace(item.Description)
	item.Provider = strings.TrimSpace(item.Provider)
	item.Model = strings.TrimSpace(item.Model)
	item.PromptID = strings.TrimSpace(item.PromptID)
	if item.Name == "" {
		return AgentRuntime{}, fmt.Errorf("runtime name is required")
	}
	if item.DisplayName == "" {
		item.DisplayName = item.Name
	}
	item.Skills = normalizeStringSlice(item.Skills)
	item.Tools = normalizeStringSlice(item.Tools)
	if item.Policy == nil {
		item.Policy = map[string]interface{}{}
	}
	return item, nil
}

func normalizeStringSlice(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if slices.Contains(result, trimmed) {
			continue
		}
		result = append(result, trimmed)
	}
	return result
}

func marshalStringSlice(values []string) (string, error) {
	payload, err := json.Marshal(values)
	if err != nil {
		return "", fmt.Errorf("marshal string slice: %w", err)
	}
	return string(payload), nil
}

func marshalMap(values map[string]interface{}) (string, error) {
	payload, err := json.Marshal(values)
	if err != nil {
		return "", fmt.Errorf("marshal map: %w", err)
	}
	return string(payload), nil
}

func toRuntime(rec *ent.AgentRuntime) (AgentRuntime, error) {
	skills, err := unmarshalStringSlice(rec.SkillsJSON)
	if err != nil {
		return AgentRuntime{}, fmt.Errorf("decode runtime skills %s: %w", rec.ID, err)
	}
	tools, err := unmarshalStringSlice(rec.ToolsJSON)
	if err != nil {
		return AgentRuntime{}, fmt.Errorf("decode runtime tools %s: %w", rec.ID, err)
	}
	policy, err := unmarshalMap(rec.PolicyJSON)
	if err != nil {
		return AgentRuntime{}, fmt.Errorf("decode runtime policy %s: %w", rec.ID, err)
	}
	return AgentRuntime{
		ID:          rec.ID,
		Name:        rec.Name,
		DisplayName: rec.DisplayName,
		Description: rec.Description,
		Enabled:     rec.Enabled,
		Provider:    rec.Provider,
		Model:       rec.Model,
		PromptID:    rec.PromptID,
		Skills:      skills,
		Tools:       tools,
		Policy:      policy,
		CreatedAt:   rec.CreatedAt,
		UpdatedAt:   rec.UpdatedAt,
	}, nil
}

func unmarshalStringSlice(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return []string{}, nil
	}
	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil, err
	}
	return normalizeStringSlice(values), nil
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
