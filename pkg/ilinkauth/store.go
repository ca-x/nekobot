package ilinkauth

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"nekobot/pkg/config"
	wxtypes "nekobot/pkg/wechat/types"
)

const (
	storeDirName        = "ilinkauth"
	usersDirName        = "users"
	bindingFileName     = "binding.json"
	bindSessionName     = "bind_session.json"
	syncStateFileSuffix = ".sync.json"
)

var (
	// ErrBindingNotFound reports that the user has no stored iLink binding.
	ErrBindingNotFound = errors.New("ilink binding not found")
)

// BindStatus describes the current QR bind lifecycle state.
type BindStatus string

const (
	// BindStatusPending means the QR code is waiting to be scanned.
	BindStatusPending BindStatus = "pending"
	// BindStatusScanned means the QR code was scanned and is awaiting confirmation.
	BindStatusScanned BindStatus = "scanned"
	// BindStatusConfirmed means the bind completed and credentials were saved.
	BindStatusConfirmed BindStatus = "confirmed"
	// BindStatusExpired means the QR code has expired.
	BindStatusExpired BindStatus = "expired"
	// BindStatusFailed means the bind flow failed.
	BindStatusFailed BindStatus = "failed"
)

// Binding stores the current user's bound iLink credentials.
type Binding struct {
	UserID      string              `json:"user_id"`
	Credentials wxtypes.Credentials `json:"credentials"`
	UpdatedAt   time.Time           `json:"updated_at"`
}

// BindSession stores the active QR binding flow for a user.
type BindSession struct {
	UserID        string     `json:"user_id"`
	QRCode        string     `json:"qrcode"`
	QRCodeContent string     `json:"qrcode_content"`
	Status        BindStatus `json:"status"`
	BotID         string     `json:"bot_id,omitempty"`
	ILinkUserID   string     `json:"ilink_user_id,omitempty"`
	Error         string     `json:"error,omitempty"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// Store manages per-user iLink binding state.
type Store struct {
	baseDir  string
	usersDir string
}

// NewStore creates a new file-backed iLink auth store.
func NewStore(cfg *config.Config) (*Store, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}

	baseDir := filepath.Join(cfg.DatabaseDir(), storeDirName)
	usersDir := filepath.Join(baseDir, usersDirName)
	if err := os.MkdirAll(usersDir, 0o700); err != nil {
		return nil, fmt.Errorf("create ilinkauth users dir: %w", err)
	}

	return &Store{
		baseDir:  baseDir,
		usersDir: usersDir,
	}, nil
}

// SaveBinding replaces the user's current binding.
func (s *Store) SaveBinding(binding *Binding) error {
	if binding == nil {
		return fmt.Errorf("binding is nil")
	}
	userID := strings.TrimSpace(binding.UserID)
	if userID == "" {
		return fmt.Errorf("user id is required")
	}
	if strings.TrimSpace(binding.Credentials.BotToken) == "" {
		return fmt.Errorf("bot token is required")
	}
	if strings.TrimSpace(binding.Credentials.ILinkBotID) == "" {
		return fmt.Errorf("ilink bot id is required")
	}

	bindingCopy := *binding
	bindingCopy.UserID = userID
	bindingCopy.UpdatedAt = time.Now()
	return s.writeJSON(s.bindingPath(userID), &bindingCopy)
}

// LoadBinding loads the user's current binding.
func (s *Store) LoadBinding(userID string) (*Binding, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("user id is required")
	}

	var binding Binding
	err := s.readJSON(s.bindingPath(userID), &binding)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrBindingNotFound
		}
		return nil, err
	}
	return &binding, nil
}

// DeleteBinding removes the user's stored binding and sync state files.
func (s *Store) DeleteBinding(userID string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return fmt.Errorf("user id is required")
	}

	binding, err := s.LoadBinding(userID)
	if err != nil && !errors.Is(err, ErrBindingNotFound) {
		return err
	}
	if binding != nil {
		if err := os.Remove(s.SyncStatePath(userID, binding.Credentials.ILinkBotID)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove sync state: %w", err)
		}
	}
	if err := os.Remove(s.bindingPath(userID)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove binding: %w", err)
	}
	return nil
}

// SaveBindSession persists the user's current QR bind session.
func (s *Store) SaveBindSession(session BindSession) error {
	userID := strings.TrimSpace(session.UserID)
	if userID == "" {
		return fmt.Errorf("user id is required")
	}
	session.UserID = userID
	session.UpdatedAt = time.Now()
	return s.writeJSON(s.bindSessionPath(userID), &session)
}

// LoadBindSession loads the user's current QR bind session.
func (s *Store) LoadBindSession(userID string) (*BindSession, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("user id is required")
	}

	var session BindSession
	err := s.readJSON(s.bindSessionPath(userID), &session)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	return &session, nil
}

// ClearBindSession removes the user's QR bind session.
func (s *Store) ClearBindSession(userID string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return fmt.Errorf("user id is required")
	}

	if err := os.Remove(s.bindSessionPath(userID)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove bind session: %w", err)
	}
	return nil
}

// SyncStatePath returns the long-poll sync-state path for a user binding.
func (s *Store) SyncStatePath(userID, botID string) string {
	return filepath.Join(s.userDir(userID), normalizeID(botID)+syncStateFileSuffix)
}

// ReadSyncState loads the saved long-poll cursor for a user binding.
func (s *Store) ReadSyncState(userID, botID string) (string, error) {
	path := s.SyncStatePath(userID, botID)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
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

// WriteSyncState saves the long-poll cursor for a user binding.
func (s *Store) WriteSyncState(userID, botID, cursor string) error {
	payload := struct {
		GetUpdatesBuf string `json:"get_updates_buf"`
	}{
		GetUpdatesBuf: cursor,
	}
	return s.writeJSON(s.SyncStatePath(userID, botID), payload)
}

func (s *Store) bindingPath(userID string) string {
	return filepath.Join(s.userDir(userID), bindingFileName)
}

func (s *Store) bindSessionPath(userID string) string {
	return filepath.Join(s.userDir(userID), bindSessionName)
}

func (s *Store) userDir(userID string) string {
	return filepath.Join(s.usersDir, normalizeID(userID))
}

// ListBindings returns all persisted bindings sorted by normalized user id.
func (s *Store) ListBindings() ([]*Binding, error) {
	entries, err := os.ReadDir(s.usersDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read users dir: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		names = append(names, entry.Name())
	}
	slices.Sort(names)

	bindings := make([]*Binding, 0, len(names))
	for _, name := range names {
		path := filepath.Join(s.usersDir, name, bindingFileName)
		var binding Binding
		if err := s.readJSON(path, &binding); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, err
		}
		bindings = append(bindings, &binding)
	}
	return bindings, nil
}

func (s *Store) writeJSON(path string, payload any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create parent dir for %s: %w", path, err)
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", path, err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func (s *Store) readJSON(path string, payload any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, payload); err != nil {
		return fmt.Errorf("unmarshal %s: %w", path, err)
	}
	return nil
}

func normalizeID(raw string) string {
	replacer := strings.NewReplacer("@", "-", ".", "-", ":", "-", "/", "-")
	return replacer.Replace(strings.TrimSpace(raw))
}
