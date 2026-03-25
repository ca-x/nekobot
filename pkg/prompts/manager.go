package prompts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"go.uber.org/zap"

	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/storage/ent"
	"nekobot/pkg/storage/ent/prompt"
	"nekobot/pkg/storage/ent/promptbinding"
)

var (
	// ErrPromptNotFound indicates the requested prompt does not exist.
	ErrPromptNotFound = errors.New("prompt not found")
	// ErrBindingNotFound indicates the requested binding does not exist.
	ErrBindingNotFound = errors.New("prompt binding not found")
)

// Manager manages prompt definitions and bindings.
type Manager struct {
	cfg    *config.Config
	log    *logger.Logger
	client *ent.Client
}

// NewManager creates a prompt manager backed by the runtime database.
func NewManager(cfg *config.Config, log *logger.Logger, client *ent.Client) (*Manager, error) {
	if client == nil {
		return nil, fmt.Errorf("ent client is nil")
	}
	mgr := &Manager{
		cfg:    cfg,
		log:    log,
		client: client,
	}
	dbPath, _ := config.RuntimeDBPath(cfg)
	log.Info("Prompt storage initialized", zap.String("db_path", dbPath))
	return mgr, nil
}

// Close releases manager resources. Shared Ent client is closed by config module.
func (m *Manager) Close() error {
	return nil
}

// ListPrompts returns all prompts ordered by name and key.
func (m *Manager) ListPrompts(ctx context.Context) ([]Prompt, error) {
	recs, err := m.client.Prompt.Query().
		Order(ent.Asc(prompt.FieldName), ent.Asc(prompt.FieldPromptKey)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list prompts: %w", err)
	}
	result := make([]Prompt, 0, len(recs))
	for _, rec := range recs {
		result = append(result, toPrompt(rec))
	}
	return result, nil
}

// CreatePrompt inserts a new prompt.
func (m *Manager) CreatePrompt(ctx context.Context, item Prompt) (*Prompt, error) {
	normalized, err := normalizePrompt(item)
	if err != nil {
		return nil, err
	}
	tagsJSON, err := marshalTags(normalized.Tags)
	if err != nil {
		return nil, err
	}
	rec, err := m.client.Prompt.Create().
		SetPromptKey(normalized.Key).
		SetName(normalized.Name).
		SetDescription(normalized.Description).
		SetMode(prompt.Mode(normalized.Mode)).
		SetTemplate(normalized.Template).
		SetEnabled(normalized.Enabled).
		SetTagsJSON(tagsJSON).
		Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, fmt.Errorf("prompt key already exists")
		}
		return nil, fmt.Errorf("create prompt: %w", err)
	}
	out := toPrompt(rec)
	return &out, nil
}

// UpdatePrompt updates an existing prompt by ID.
func (m *Manager) UpdatePrompt(ctx context.Context, id string, item Prompt) (*Prompt, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("prompt id is required")
	}
	normalized, err := normalizePrompt(item)
	if err != nil {
		return nil, err
	}
	tagsJSON, err := marshalTags(normalized.Tags)
	if err != nil {
		return nil, err
	}
	rec, err := m.client.Prompt.UpdateOneID(id).
		SetPromptKey(normalized.Key).
		SetName(normalized.Name).
		SetDescription(normalized.Description).
		SetMode(prompt.Mode(normalized.Mode)).
		SetTemplate(normalized.Template).
		SetEnabled(normalized.Enabled).
		SetTagsJSON(tagsJSON).
		Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrPromptNotFound
		}
		if ent.IsConstraintError(err) {
			return nil, fmt.Errorf("prompt key already exists")
		}
		return nil, fmt.Errorf("update prompt: %w", err)
	}
	out := toPrompt(rec)
	return &out, nil
}

// DeletePrompt removes a prompt and all of its bindings.
func (m *Manager) DeletePrompt(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("prompt id is required")
	}
	if _, err := m.client.PromptBinding.Delete().Where(promptbinding.PromptIDEQ(id)).Exec(ctx); err != nil {
		return fmt.Errorf("delete prompt bindings: %w", err)
	}
	affected, err := m.client.Prompt.Delete().Where(prompt.IDEQ(id)).Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete prompt: %w", err)
	}
	if affected == 0 {
		return ErrPromptNotFound
	}
	return nil
}

// ListBindings returns bindings filtered by optional scope/target.
func (m *Manager) ListBindings(ctx context.Context, scope, target string) ([]Binding, error) {
	q := m.client.PromptBinding.Query().Order(ent.Asc(promptbinding.FieldScope), ent.Asc(promptbinding.FieldTarget), ent.Asc(promptbinding.FieldPriority))
	scope = normalizeScope(scope)
	target = strings.TrimSpace(target)
	if scope != "" {
		q = q.Where(promptbinding.ScopeEQ(promptbinding.Scope(scope)))
	}
	if target != "" {
		q = q.Where(promptbinding.TargetEQ(target))
	}
	recs, err := q.All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list bindings: %w", err)
	}
	result := make([]Binding, 0, len(recs))
	for _, rec := range recs {
		result = append(result, toBinding(rec))
	}
	return result, nil
}

// CreateBinding creates a new prompt binding.
func (m *Manager) CreateBinding(ctx context.Context, item Binding) (*Binding, error) {
	normalized, err := m.normalizeBinding(ctx, item)
	if err != nil {
		return nil, err
	}
	rec, err := m.client.PromptBinding.Create().
		SetScope(promptbinding.Scope(normalized.Scope)).
		SetTarget(normalized.Target).
		SetPromptID(normalized.PromptID).
		SetEnabled(normalized.Enabled).
		SetPriority(normalized.Priority).
		Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, fmt.Errorf("prompt binding already exists")
		}
		return nil, fmt.Errorf("create prompt binding: %w", err)
	}
	out := toBinding(rec)
	return &out, nil
}

// UpdateBinding updates an existing prompt binding.
func (m *Manager) UpdateBinding(ctx context.Context, id string, item Binding) (*Binding, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("binding id is required")
	}
	normalized, err := m.normalizeBinding(ctx, item)
	if err != nil {
		return nil, err
	}
	rec, err := m.client.PromptBinding.UpdateOneID(id).
		SetScope(promptbinding.Scope(normalized.Scope)).
		SetTarget(normalized.Target).
		SetPromptID(normalized.PromptID).
		SetEnabled(normalized.Enabled).
		SetPriority(normalized.Priority).
		Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrBindingNotFound
		}
		if ent.IsConstraintError(err) {
			return nil, fmt.Errorf("prompt binding already exists")
		}
		return nil, fmt.Errorf("update prompt binding: %w", err)
	}
	out := toBinding(rec)
	return &out, nil
}

// DeleteBinding removes a prompt binding by ID.
func (m *Manager) DeleteBinding(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("binding id is required")
	}
	affected, err := m.client.PromptBinding.Delete().Where(promptbinding.IDEQ(id)).Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete prompt binding: %w", err)
	}
	if affected == 0 {
		return ErrBindingNotFound
	}
	return nil
}

// Resolve resolves global, channel, and session prompt bindings for one invocation.
func (m *Manager) Resolve(ctx context.Context, input ResolveInput) (*ResolvedPromptSet, error) {
	bindings, err := m.ListBindings(ctx, "", "")
	if err != nil {
		return nil, err
	}
	promptsList, err := m.ListPrompts(ctx)
	if err != nil {
		return nil, err
	}
	promptMap := make(map[string]Prompt, len(promptsList))
	for _, item := range promptsList {
		promptMap[item.ID] = item
	}

	selected := make([]AppliedPrompt, 0)
	indexByPromptID := make(map[string]int)
	for _, binding := range bindings {
		if !binding.Enabled {
			continue
		}
		if !bindingMatches(binding, input) {
			continue
		}
		item, ok := promptMap[binding.PromptID]
		if !ok || !item.Enabled {
			continue
		}
		rendered, err := renderPromptTemplate(item.Key, item.Template, input)
		if err != nil {
			return nil, fmt.Errorf("render prompt %s: %w", item.Key, err)
		}
		applied := AppliedPrompt{
			BindingID: binding.ID,
			PromptID:  item.ID,
			PromptKey: item.Key,
			Name:      item.Name,
			Mode:      item.Mode,
			Scope:     binding.Scope,
			Target:    binding.Target,
			Priority:  binding.Priority,
			Content:   rendered,
		}
		if existingIndex, exists := indexByPromptID[item.ID]; exists {
			if shouldReplaceAppliedPrompt(selected[existingIndex], applied) {
				selected[existingIndex] = applied
			}
			continue
		}
		indexByPromptID[item.ID] = len(selected)
		selected = append(selected, applied)
	}

	sort.SliceStable(selected, func(i, j int) bool {
		left := selected[i]
		right := selected[j]
		if scopeRank(left.Scope) != scopeRank(right.Scope) {
			return scopeRank(left.Scope) < scopeRank(right.Scope)
		}
		if left.Priority != right.Priority {
			return left.Priority < right.Priority
		}
		return left.Name < right.Name
	})

	systemParts := make([]string, 0)
	userParts := make([]string, 0)
	for _, item := range selected {
		if strings.TrimSpace(item.Content) == "" {
			continue
		}
		switch item.Mode {
		case ModeSystem:
			systemParts = append(systemParts, item.Content)
		case ModeUser:
			userParts = append(userParts, item.Content)
		}
	}

	return &ResolvedPromptSet{
		SystemText: strings.TrimSpace(strings.Join(systemParts, "\n\n---\n\n")),
		UserText:   strings.TrimSpace(strings.Join(userParts, "\n\n---\n\n")),
		Applied:    selected,
	}, nil
}

// GetSessionBindingSet returns the prompt IDs bound to a session.
func (m *Manager) GetSessionBindingSet(ctx context.Context, sessionID string) (*SessionBindingSet, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, fmt.Errorf("session id is required")
	}
	bindings, err := m.ListBindings(ctx, ScopeSession, sessionID)
	if err != nil {
		return nil, err
	}
	promptsList, err := m.ListPrompts(ctx)
	if err != nil {
		return nil, err
	}
	promptMode := make(map[string]string, len(promptsList))
	for _, item := range promptsList {
		promptMode[item.ID] = item.Mode
	}
	result := &SessionBindingSet{
		SystemPromptIDs: make([]string, 0),
		UserPromptIDs:   make([]string, 0),
		Bindings:        bindings,
	}
	for _, binding := range bindings {
		switch promptMode[binding.PromptID] {
		case ModeSystem:
			result.SystemPromptIDs = append(result.SystemPromptIDs, binding.PromptID)
		case ModeUser:
			result.UserPromptIDs = append(result.UserPromptIDs, binding.PromptID)
		}
	}
	return result, nil
}

// ReplaceSessionBindings replaces all session-scoped bindings with the provided prompt IDs.
func (m *Manager) ReplaceSessionBindings(ctx context.Context, sessionID string, systemPromptIDs, userPromptIDs []string) (*SessionBindingSet, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, fmt.Errorf("session id is required")
	}
	if err := m.ClearSessionBindings(ctx, sessionID); err != nil {
		return nil, err
	}

	priority := 100
	for _, promptID := range append([]string(nil), systemPromptIDs...) {
		if _, err := m.CreateBinding(ctx, Binding{
			Scope:    ScopeSession,
			Target:   sessionID,
			PromptID: promptID,
			Enabled:  true,
			Priority: priority,
		}); err != nil {
			return nil, err
		}
		priority += 10
	}
	priority = 200
	for _, promptID := range append([]string(nil), userPromptIDs...) {
		if _, err := m.CreateBinding(ctx, Binding{
			Scope:    ScopeSession,
			Target:   sessionID,
			PromptID: promptID,
			Enabled:  true,
			Priority: priority,
		}); err != nil {
			return nil, err
		}
		priority += 10
	}
	return m.GetSessionBindingSet(ctx, sessionID)
}

// ClearSessionBindings removes all session-scoped prompt bindings for one session.
func (m *Manager) ClearSessionBindings(ctx context.Context, sessionID string) error {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil
	}
	if _, err := m.client.PromptBinding.Delete().
		Where(
			promptbinding.ScopeEQ(ScopeSession),
			promptbinding.TargetEQ(sessionID),
		).
		Exec(ctx); err != nil {
		return fmt.Errorf("clear session prompt bindings: %w", err)
	}
	return nil
}

func (m *Manager) normalizeBinding(ctx context.Context, item Binding) (Binding, error) {
	scope := normalizeScope(item.Scope)
	if scope == "" {
		return Binding{}, fmt.Errorf("binding scope is required")
	}
	target := strings.TrimSpace(item.Target)
	if scope == ScopeGlobal {
		target = ""
	}
	if scope != ScopeGlobal && target == "" {
		return Binding{}, fmt.Errorf("binding target is required for %s scope", scope)
	}
	promptID := strings.TrimSpace(item.PromptID)
	if promptID == "" {
		return Binding{}, fmt.Errorf("binding prompt_id is required")
	}
	exists, err := m.client.Prompt.Query().Where(prompt.IDEQ(promptID)).Exist(ctx)
	if err != nil {
		return Binding{}, fmt.Errorf("check prompt existence: %w", err)
	}
	if !exists {
		return Binding{}, ErrPromptNotFound
	}
	priority := item.Priority
	if priority == 0 {
		priority = 100
	}
	return Binding{
		Scope:    scope,
		Target:   target,
		PromptID: promptID,
		Enabled:  item.Enabled,
		Priority: priority,
	}, nil
}

func normalizePrompt(item Prompt) (Prompt, error) {
	key := strings.TrimSpace(item.Key)
	if key == "" {
		return Prompt{}, fmt.Errorf("prompt key is required")
	}
	name := strings.TrimSpace(item.Name)
	if name == "" {
		return Prompt{}, fmt.Errorf("prompt name is required")
	}
	mode := strings.TrimSpace(strings.ToLower(item.Mode))
	switch mode {
	case ModeSystem, ModeUser:
	default:
		return Prompt{}, fmt.Errorf("prompt mode must be system or user")
	}
	templateText := strings.TrimSpace(item.Template)
	if templateText == "" {
		return Prompt{}, fmt.Errorf("prompt template is required")
	}
	tags := make([]string, 0, len(item.Tags))
	seen := make(map[string]struct{}, len(item.Tags))
	for _, tag := range item.Tags {
		normalized := strings.TrimSpace(tag)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		tags = append(tags, normalized)
	}
	return Prompt{
		Key:         key,
		Name:        name,
		Description: strings.TrimSpace(item.Description),
		Mode:        mode,
		Template:    templateText,
		Enabled:     item.Enabled,
		Tags:        tags,
	}, nil
}

func bindingMatches(item Binding, input ResolveInput) bool {
	switch item.Scope {
	case ScopeGlobal:
		return true
	case ScopeChannel:
		return strings.TrimSpace(item.Target) == strings.TrimSpace(input.Channel)
	case ScopeSession:
		return strings.TrimSpace(item.Target) == strings.TrimSpace(input.SessionID)
	default:
		return false
	}
}

func normalizeScope(raw string) string {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case ScopeGlobal, ScopeChannel, ScopeSession:
		return strings.TrimSpace(strings.ToLower(raw))
	default:
		return ""
	}
}

func scopeRank(scope string) int {
	switch scope {
	case ScopeGlobal:
		return 1
	case ScopeChannel:
		return 2
	case ScopeSession:
		return 3
	default:
		return 99
	}
}

func shouldReplaceAppliedPrompt(current, candidate AppliedPrompt) bool {
	if scopeRank(candidate.Scope) != scopeRank(current.Scope) {
		return scopeRank(candidate.Scope) > scopeRank(current.Scope)
	}
	if candidate.Priority != current.Priority {
		return candidate.Priority < current.Priority
	}
	if candidate.Target != current.Target {
		return candidate.Target > current.Target
	}
	return candidate.BindingID > current.BindingID
}

func toPrompt(rec *ent.Prompt) Prompt {
	return Prompt{
		ID:          rec.ID,
		Key:         rec.PromptKey,
		Name:        rec.Name,
		Description: rec.Description,
		Mode:        string(rec.Mode),
		Template:    rec.Template,
		Enabled:     rec.Enabled,
		Tags:        unmarshalTags(rec.TagsJSON),
		CreatedAt:   rec.CreatedAt,
		UpdatedAt:   rec.UpdatedAt,
	}
}

func toBinding(rec *ent.PromptBinding) Binding {
	return Binding{
		ID:        rec.ID,
		Scope:     string(rec.Scope),
		Target:    rec.Target,
		PromptID:  rec.PromptID,
		Enabled:   rec.Enabled,
		Priority:  rec.Priority,
		CreatedAt: rec.CreatedAt,
		UpdatedAt: rec.UpdatedAt,
	}
}

func marshalTags(tags []string) (string, error) {
	data, err := json.Marshal(tags)
	if err != nil {
		return "", fmt.Errorf("marshal prompt tags: %w", err)
	}
	return string(data), nil
}

func unmarshalTags(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{}
	}
	var tags []string
	if err := json.Unmarshal([]byte(raw), &tags); err != nil {
		return []string{}
	}
	return tags
}
