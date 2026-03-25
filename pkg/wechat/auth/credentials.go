package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"nekobot/pkg/wechat/types"
)

// SaveCredentials marshals credentials to JSON and writes them to path.
func SaveCredentials(path string, creds *types.Credentials) error {
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal credentials: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create credentials dir: %w", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write credentials: %w", err)
	}

	return nil
}

// LoadCredentials reads and unmarshals credentials from a JSON file.
func LoadCredentials(path string) (*types.Credentials, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read credentials: %w", err)
	}

	var creds types.Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("unmarshal credentials: %w", err)
	}

	return &creds, nil
}
