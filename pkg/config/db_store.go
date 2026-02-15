package config

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/lib-x/entsqlite"
)

const (
	runtimeConfigDBName = "tool_sessions.db"
	runtimeConfigTable  = "config_sections"
)

var runtimeConfigSections = []string{
	"agents",
	"channels",
	"gateway",
	"tools",
	"heartbeat",
	"approval",
	"logger",
	"webui",
}

// ApplyDatabaseOverrides loads runtime-config sections from SQLite.
// When a section is missing in DB, it is bootstrapped from current config.
func ApplyDatabaseOverrides(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	db, err := openRuntimeConfigDB(cfg)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := ensureRuntimeConfigSchema(db); err != nil {
		return err
	}

	for _, section := range runtimeConfigSections {
		payload, exists, err := loadSectionPayload(db, section)
		if err != nil {
			return err
		}
		if !exists {
			payload, err = marshalSection(cfg, section)
			if err != nil {
				return err
			}
			if err := upsertSectionPayload(db, section, payload); err != nil {
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

	db, err := openRuntimeConfigDB(cfg)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := ensureRuntimeConfigSchema(db); err != nil {
		return err
	}

	for _, section := range normalizeSections(sections) {
		payload, err := marshalSection(cfg, section)
		if err != nil {
			return err
		}
		if err := upsertSectionPayload(db, section, payload); err != nil {
			return err
		}
	}

	return nil
}

func openRuntimeConfigDB(cfg *Config) (*sql.DB, error) {
	workspace := strings.TrimSpace(cfg.WorkspacePath())
	if workspace == "" {
		return nil, fmt.Errorf("workspace path is empty")
	}
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		return nil, fmt.Errorf("create workspace directory: %w", err)
	}

	dbPath := filepath.Join(workspace, runtimeConfigDBName)
	dsn := fmt.Sprintf("file:%s?cache=shared&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(10000)", dbPath)
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("open runtime config database: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping runtime config database: %w", err)
	}
	return db, nil
}

func ensureRuntimeConfigSchema(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS config_sections (
			section TEXT PRIMARY KEY,
			payload_json TEXT NOT NULL,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("create config_sections schema: %w", err)
	}
	return nil
}

func loadSectionPayload(db *sql.DB, section string) ([]byte, bool, error) {
	var raw string
	err := db.QueryRow(`SELECT payload_json FROM config_sections WHERE section = ?`, section).Scan(&raw)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("load config section %s: %w", section, err)
	}
	return []byte(raw), true, nil
}

func upsertSectionPayload(db *sql.DB, section string, payload []byte) error {
	_, err := db.Exec(`
		INSERT INTO config_sections(section, payload_json, updated_at)
		VALUES(?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(section) DO UPDATE SET
			payload_json = excluded.payload_json,
			updated_at = CURRENT_TIMESTAMP
	`, section, string(payload))
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
