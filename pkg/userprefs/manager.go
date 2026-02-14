package userprefs

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"nekobot/pkg/state"
)

const keyPrefix = "userprefs"

// Profile stores user preferences per (channel, user).
type Profile struct {
	Language         string    `json:"language,omitempty"`
	PreferredName    string    `json:"preferred_name,omitempty"`
	Preferences      string    `json:"preferences,omitempty"`
	SkillInstallMode string    `json:"skill_install_mode,omitempty"`
	UpdatedAt        time.Time `json:"updated_at,omitempty"`
}

// Manager manages profile persistence.
type Manager struct {
	store state.KV
}

// New creates a new preferences manager.
func New(store state.KV) *Manager {
	return &Manager{store: store}
}

// Get gets a profile by channel and user ID.
func (m *Manager) Get(ctx context.Context, channel, userID string) (Profile, bool, error) {
	if m == nil || m.store == nil {
		return Profile{}, false, nil
	}

	v, ok, err := m.store.Get(ctx, key(channel, userID))
	if err != nil || !ok {
		return Profile{}, false, err
	}

	p, err := decodeProfile(v)
	if err != nil {
		return Profile{}, false, err
	}

	return p, true, nil
}

// Save saves a profile.
func (m *Manager) Save(ctx context.Context, channel, userID string, p Profile) error {
	if m == nil || m.store == nil {
		return nil
	}

	p.Language = NormalizeLanguage(p.Language)
	p.PreferredName = strings.TrimSpace(p.PreferredName)
	p.Preferences = strings.TrimSpace(p.Preferences)
	p.SkillInstallMode = NormalizeSkillInstallMode(p.SkillInstallMode)
	p.UpdatedAt = time.Now()

	return m.store.Set(ctx, key(channel, userID), p)
}

// NormalizeSkillInstallMode returns normalized skill install preference.
func NormalizeSkillInstallMode(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	switch mode {
	case "legacy", "npx_preferred":
		return mode
	default:
		return "legacy"
	}
}

// Clear removes a profile.
func (m *Manager) Clear(ctx context.Context, channel, userID string) error {
	if m == nil || m.store == nil {
		return nil
	}
	return m.store.Delete(ctx, key(channel, userID))
}

// NormalizeLanguage returns normalized language code with default zh.
func NormalizeLanguage(lang string) string {
	lang = strings.ToLower(strings.TrimSpace(lang))
	switch lang {
	case "zh", "en", "ja":
		return lang
	default:
		return "zh"
	}
}

// PromptContext returns a compact profile context for LLM input.
func (p Profile) PromptContext() string {
	var parts []string
	if p.Language != "" {
		parts = append(parts, "language="+NormalizeLanguage(p.Language))
	}
	if p.PreferredName != "" {
		parts = append(parts, "preferred_name="+p.PreferredName)
	}
	if p.Preferences != "" {
		parts = append(parts, "preferences="+p.Preferences)
	}
	if p.SkillInstallMode != "" {
		parts = append(parts, "skill_install_mode="+NormalizeSkillInstallMode(p.SkillInstallMode))
	}
	if len(parts) == 0 {
		return ""
	}
	return "UserProfile(" + strings.Join(parts, ", ") + ")"
}

func key(channel, userID string) string {
	ch := strings.ToLower(strings.TrimSpace(channel))
	uid := strings.TrimSpace(userID)
	if ch == "" {
		ch = "default"
	}
	if uid == "" {
		uid = "unknown"
	}
	return fmt.Sprintf("%s:%s:%s", keyPrefix, ch, uid)
}

func decodeProfile(v interface{}) (Profile, error) {
	if v == nil {
		return Profile{}, nil
	}

	data, err := json.Marshal(v)
	if err != nil {
		return Profile{}, fmt.Errorf("marshal profile: %w", err)
	}

	var p Profile
	if err := json.Unmarshal(data, &p); err != nil {
		return Profile{}, fmt.Errorf("unmarshal profile: %w", err)
	}

	p.Language = NormalizeLanguage(p.Language)
	p.SkillInstallMode = NormalizeSkillInstallMode(p.SkillInstallMode)
	return p, nil
}
