package wechat

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"nekobot/pkg/config"
	wxtypes "nekobot/pkg/wechat/types"
)

const (
	accountsDirName   = "wechat"
	bindStateFile     = "bind_state.json"
	activeAccountFile = "active_account"
)

// BindStatus represents the current binding flow state.
type BindStatus string

const (
	BindStatusPending   BindStatus = "pending"
	BindStatusScanned   BindStatus = "scanned"
	BindStatusConfirmed BindStatus = "confirmed"
	BindStatusExpired   BindStatus = "expired"
	BindStatusFailed    BindStatus = "failed"
)

// BindState stores the active WebUI binding session.
type BindState struct {
	QRCode        string     `json:"qrcode"`
	QRCodeContent string     `json:"qrcode_content"`
	Status        BindStatus `json:"status"`
	BotID         string     `json:"bot_id,omitempty"`
	UserID        string     `json:"user_id,omitempty"`
	Error         string     `json:"error,omitempty"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// Credentials stores WeChat login session data.
type Credentials = wxtypes.Credentials

// StoredAccount describes a persisted WeChat account and whether it is active.
type StoredAccount struct {
	AccountID string       `json:"account_id"`
	Active    bool         `json:"active"`
	Creds     *Credentials `json:"creds,omitempty"`
}

// CredentialStore manages single-account WeChat credentials and bind state.
type CredentialStore struct {
	fs          fs.StatFS
	baseDir     string
	accountsDir string
}

// NewCredentialStore creates a credential store under the runtime DB directory.
func NewCredentialStore(cfg *config.Config) (*CredentialStore, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}

	baseDir := filepath.Join(cfg.DatabaseDir(), accountsDirName)
	accountsDir := filepath.Join(baseDir, "accounts")
	if err := os.MkdirAll(accountsDir, 0o700); err != nil {
		return nil, fmt.Errorf("create accounts dir: %w", err)
	}

	return &CredentialStore{
		fs:          os.DirFS(string(filepath.Separator)).(fs.StatFS),
		baseDir:     baseDir,
		accountsDir: accountsDir,
	}, nil
}

// NormalizeAccountID converts raw bot ID to a filesystem-safe format.
func NormalizeAccountID(raw string) string {
	replacer := strings.NewReplacer("@", "-", ".", "-", ":", "-", "/", "-")
	return replacer.Replace(strings.TrimSpace(raw))
}

// LoadCredentials loads the currently bound account credentials.
func (s *CredentialStore) LoadCredentials() (*Credentials, error) {
	accounts, err := s.ListCredentials()
	if err != nil {
		return nil, err
	}
	for _, account := range accounts {
		if account == nil || !account.Active || account.Creds == nil {
			continue
		}
		return account.Creds, nil
	}
	return nil, nil
}

// ListCredentials lists all stored WeChat accounts and marks the active one.
func (s *CredentialStore) ListCredentials() ([]*StoredAccount, error) {
	entries, err := os.ReadDir(s.accountsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read accounts dir: %w", err)
	}

	activeID, err := s.readActiveAccountID()
	if err != nil {
		return nil, err
	}

	accounts := make([]*StoredAccount, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(s.accountsDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read credentials: %w", err)
		}
		var creds Credentials
		if err := json.Unmarshal(data, &creds); err != nil {
			return nil, fmt.Errorf("unmarshal credentials: %w", err)
		}
		if strings.TrimSpace(creds.BotToken) == "" {
			continue
		}
		accountID := strings.TrimSpace(creds.ILinkBotID)
		accounts = append(accounts, &StoredAccount{
			AccountID: accountID,
			Active:    accountID != "" && accountID == activeID,
			Creds:     &creds,
		})
	}

	if len(accounts) == 0 {
		return nil, nil
	}
	if activeID == "" {
		accounts[0].Active = true
		if err := s.writeActiveAccountID(accounts[0].AccountID); err != nil {
			return nil, err
		}
		return accounts, nil
	}

	hasActive := false
	for _, account := range accounts {
		if account != nil && account.Active {
			hasActive = true
			break
		}
	}
	if !hasActive {
		accounts[0].Active = true
		if err := s.writeActiveAccountID(accounts[0].AccountID); err != nil {
			return nil, err
		}
	}
	return accounts, nil
}

// ReplaceCredentials stores the new account and makes it active.
func (s *CredentialStore) ReplaceCredentials(creds *Credentials) error {
	return s.SaveCredentials(creds, true)
}

// SaveCredentials stores a WeChat account and optionally makes it active.
func (s *CredentialStore) SaveCredentials(creds *Credentials, active bool) error {
	if creds == nil {
		return fmt.Errorf("credentials is nil")
	}
	if strings.TrimSpace(creds.BotToken) == "" || strings.TrimSpace(creds.ILinkBotID) == "" {
		return fmt.Errorf("bot_token and ilink_bot_id are required")
	}

	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal credentials: %w", err)
	}

	path := filepath.Join(s.accountsDir, NormalizeAccountID(creds.ILinkBotID)+".json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write credentials: %w", err)
	}
	if active {
		if err := s.writeActiveAccountID(creds.ILinkBotID); err != nil {
			return err
		}
	}

	return nil
}

// ClearCredentials removes all bound credentials and sync state.
func (s *CredentialStore) ClearCredentials() error {
	if err := s.clearAccountFiles(); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(s.baseDir, activeAccountFile)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove active account file: %w", err)
	}
	return nil
}

// SetActiveAccount makes an existing stored account active.
func (s *CredentialStore) SetActiveAccount(accountID string) error {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return fmt.Errorf("account id is required")
	}
	path := filepath.Join(s.accountsDir, NormalizeAccountID(accountID)+".json")
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("account not found: %s", accountID)
		}
		return fmt.Errorf("stat account: %w", err)
	}
	return s.writeActiveAccountID(accountID)
}

// DeleteCredentials removes a single stored account and its sync state.
func (s *CredentialStore) DeleteCredentials(accountID string) error {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return fmt.Errorf("account id is required")
	}

	path := filepath.Join(s.accountsDir, NormalizeAccountID(accountID)+".json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove credentials: %w", err)
	}
	if err := os.Remove(s.SyncStatePath(accountID)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove sync state: %w", err)
	}

	activeID, err := s.readActiveAccountID()
	if err != nil {
		return err
	}
	if activeID != accountID {
		return nil
	}

	accounts, err := s.ListCredentials()
	if err != nil {
		return err
	}
	if len(accounts) == 0 {
		if err := os.Remove(filepath.Join(s.baseDir, activeAccountFile)); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove active account file: %w", err)
		}
		return nil
	}
	return s.writeActiveAccountID(accounts[0].AccountID)
}

func (s *CredentialStore) clearAccountFiles() error {
	entries, err := os.ReadDir(s.accountsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read accounts dir: %w", err)
	}

	for _, entry := range entries {
		path := filepath.Join(s.accountsDir, entry.Name())
		if entry.IsDir() {
			if err := os.RemoveAll(path); err != nil {
				return fmt.Errorf("remove account dir %s: %w", path, err)
			}
			continue
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove account file %s: %w", path, err)
		}
	}

	return nil
}

func (s *CredentialStore) readActiveAccountID() (string, error) {
	data, err := os.ReadFile(filepath.Join(s.baseDir, activeAccountFile))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("read active account: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

func (s *CredentialStore) writeActiveAccountID(accountID string) error {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return fmt.Errorf("account id is required")
	}
	if err := os.WriteFile(filepath.Join(s.baseDir, activeAccountFile), []byte(accountID), 0o600); err != nil {
		return fmt.Errorf("write active account: %w", err)
	}
	return nil
}

// SyncStatePath returns the long-poll cursor file path for a bot account.
func (s *CredentialStore) SyncStatePath(botID string) string {
	return filepath.Join(s.accountsDir, NormalizeAccountID(botID)+".sync.json")
}

// ReadSyncState loads the long-poll cursor for a bot account.
func (s *CredentialStore) ReadSyncState(botID string) (string, error) {
	path := s.SyncStatePath(botID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("read sync state: %w", err)
	}

	var payload struct {
		GetUpdatesBuf string `json:"get_updates_buf"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return "", fmt.Errorf("unmarshal sync state: %w", err)
	}
	return payload.GetUpdatesBuf, nil
}

// WriteSyncState persists the long-poll cursor for a bot account.
func (s *CredentialStore) WriteSyncState(botID, cursor string) error {
	payload := struct {
		GetUpdatesBuf string `json:"get_updates_buf"`
	}{GetUpdatesBuf: cursor}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal sync state: %w", err)
	}

	if err := os.WriteFile(s.SyncStatePath(botID), data, 0o600); err != nil {
		return fmt.Errorf("write sync state: %w", err)
	}
	return nil
}

// SaveBindState persists the active bind flow state for WebUI polling.
func (s *CredentialStore) SaveBindState(state BindState) error {
	state.UpdatedAt = time.Now()
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal bind state: %w", err)
	}
	if err := os.WriteFile(filepath.Join(s.baseDir, bindStateFile), data, 0o600); err != nil {
		return fmt.Errorf("write bind state: %w", err)
	}
	return nil
}

// LoadBindState loads the active bind flow state.
func (s *CredentialStore) LoadBindState() (*BindState, error) {
	path := filepath.Join(s.baseDir, bindStateFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read bind state: %w", err)
	}
	var state BindState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("unmarshal bind state: %w", err)
	}
	return &state, nil
}

// ClearBindState removes any active bind flow state.
func (s *CredentialStore) ClearBindState() error {
	path := filepath.Join(s.baseDir, bindStateFile)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove bind state: %w", err)
	}
	return nil
}
