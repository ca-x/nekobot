package config

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"nekobot/pkg/storage/ent"
	"nekobot/pkg/storage/ent/configsection"
)

var runtimeConfigSections = []string{
	"agents",
	"channels",
	"gateway",
	"tools",
	"heartbeat",
	"approval",
	"logger",
	"memory",
	"webui",
}

// ApplyDatabaseOverrides loads runtime-config sections from SQLite.
// When a section is missing in DB, it is bootstrapped from current config.
func ApplyDatabaseOverrides(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}

	client, err := openRuntimeConfigClient(cfg)
	if err != nil {
		return err
	}
	defer client.Close()

	ctx := context.Background()
	for _, section := range runtimeConfigSections {
		payload, exists, err := loadSectionPayload(ctx, client, section)
		if err != nil {
			return err
		}
		if !exists {
			payload, err = marshalSection(cfg, section)
			if err != nil {
				return err
			}
			if err := upsertSectionPayload(ctx, client, section, payload); err != nil {
				return err
			}
			continue
		}
		if err := applySection(cfg, section, payload); err != nil {
			return err
		}
	}

	return nil
}

// SaveDatabaseSections persists selected runtime-config sections to SQLite.
func SaveDatabaseSections(cfg *Config, sections ...string) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	if len(sections) == 0 {
		sections = runtimeConfigSections
	}

	client, err := openRuntimeConfigClient(cfg)
	if err != nil {
		return err
	}
	defer client.Close()

	ctx := context.Background()
	for _, section := range normalizeSections(sections) {
		payload, err := marshalSection(cfg, section)
		if err != nil {
			return err
		}
		if err := upsertSectionPayload(ctx, client, section, payload); err != nil {
			return err
		}
	}

	return nil
}

func openRuntimeConfigClient(cfg *Config) (*ent.Client, error) {
	client, err := OpenRuntimeEntClient(cfg)
	if err != nil {
		return nil, err
	}
	if err := EnsureRuntimeEntSchema(client); err != nil {
		_ = client.Close()
		return nil, err
	}
	return client, nil
}

func loadSectionPayload(ctx context.Context, client *ent.Client, section string) ([]byte, bool, error) {
	rec, err := client.ConfigSection.Query().Where(configsection.SectionEQ(section)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("load config section %s: %w", section, err)
	}
	return []byte(rec.PayloadJSON), true, nil
}

func upsertSectionPayload(ctx context.Context, client *ent.Client, section string, payload []byte) error {
	rec, err := client.ConfigSection.Query().Where(configsection.SectionEQ(section)).Only(ctx)
	if err != nil {
		if !ent.IsNotFound(err) {
			return fmt.Errorf("load config section %s: %w", section, err)
		}
		_, err = client.ConfigSection.Create().
			SetSection(section).
			SetPayloadJSON(string(payload)).
			Save(ctx)
		if err != nil {
			if ent.IsConstraintError(err) {
				affected, updateErr := client.ConfigSection.Update().
					Where(configsection.SectionEQ(section)).
					SetPayloadJSON(string(payload)).
					Save(ctx)
				if updateErr == nil && affected > 0 {
					return nil
				}
			}
			return fmt.Errorf("save config section %s: %w", section, err)
		}
		return nil
	}

	_, err = client.ConfigSection.UpdateOneID(rec.ID).SetPayloadJSON(string(payload)).Save(ctx)
	if err != nil {
		return fmt.Errorf("save config section %s: %w", section, err)
	}
	return nil
}

func marshalSection(cfg *Config, section string) ([]byte, error) {
	cfg.mu.RLock()
	defer cfg.mu.RUnlock()

	switch section {
	case "agents":
		return json.Marshal(cfg.Agents)
	case "channels":
		return json.Marshal(cfg.Channels)
	case "gateway":
		return json.Marshal(cfg.Gateway)
	case "tools":
		return json.Marshal(cfg.Tools)
	case "heartbeat":
		return json.Marshal(cfg.Heartbeat)
	case "approval":
		return json.Marshal(cfg.Approval)
	case "logger":
		return json.Marshal(cfg.Logger)
	case "memory":
		return json.Marshal(cfg.Memory)
	case "webui":
		return json.Marshal(cfg.WebUI)
	default:
		return nil, fmt.Errorf("unknown runtime config section: %s", section)
	}
}

func applySection(cfg *Config, section string, payload []byte) error {
	cfg.mu.Lock()
	defer cfg.mu.Unlock()

	switch section {
	case "agents":
		var v AgentsConfig
		if err := json.Unmarshal(payload, &v); err != nil {
			return fmt.Errorf("decode agents config: %w", err)
		}
		cfg.Agents = v
	case "channels":
		var v ChannelsConfig
		if err := json.Unmarshal(payload, &v); err != nil {
			return fmt.Errorf("decode channels config: %w", err)
		}
		cfg.Channels = v
	case "gateway":
		var v GatewayConfig
		if err := json.Unmarshal(payload, &v); err != nil {
			return fmt.Errorf("decode gateway config: %w", err)
		}
		cfg.Gateway = v
	case "tools":
		var v ToolsConfig
		if err := json.Unmarshal(payload, &v); err != nil {
			return fmt.Errorf("decode tools config: %w", err)
		}
		cfg.Tools = v
	case "heartbeat":
		var v HeartbeatConfig
		if err := json.Unmarshal(payload, &v); err != nil {
			return fmt.Errorf("decode heartbeat config: %w", err)
		}
		cfg.Heartbeat = v
	case "approval":
		var v ApprovalConfig
		if err := json.Unmarshal(payload, &v); err != nil {
			return fmt.Errorf("decode approval config: %w", err)
		}
		cfg.Approval = v
	case "logger":
		var v LoggerConfig
		if err := json.Unmarshal(payload, &v); err != nil {
			return fmt.Errorf("decode logger config: %w", err)
		}
		cfg.Logger = v
	case "memory":
		var v MemoryConfig
		if err := json.Unmarshal(payload, &v); err != nil {
			return fmt.Errorf("decode memory config: %w", err)
		}
		cfg.Memory = v
	case "webui":
		var v WebUIConfig
		if err := json.Unmarshal(payload, &v); err != nil {
			return fmt.Errorf("decode webui config: %w", err)
		}
		cfg.WebUI = v
	default:
		return fmt.Errorf("unknown runtime config section: %s", section)
	}
	return nil
}

func normalizeSections(sections []string) []string {
	seen := make(map[string]struct{}, len(sections))
	result := make([]string, 0, len(sections))
	for _, section := range sections {
		name := strings.TrimSpace(section)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		result = append(result, name)
	}
	return result
}
