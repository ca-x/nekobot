package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"nekobot/pkg/state"
)

type acpAuthenticatedUserKey struct{}

// ACPSessionRuntimeMapping stores the durable session-to-runtime mapping.
type ACPSessionRuntimeMapping struct {
	UserID     string `json:"user_id"`
	SessionID  string `json:"session_id"`
	RuntimeID  string `json:"runtime_id"`
	SourceKind string `json:"source_kind"`
}

type acpSessionRuntimeMappingStore interface {
	PutSessionRuntimeMapping(ctx context.Context, mapping ACPSessionRuntimeMapping) error
	GetSessionRuntimeMapping(ctx context.Context, userID, sessionID string) (*ACPSessionRuntimeMapping, error)
}

type kvACPSessionRuntimeMappingStore struct {
	store state.KV
}

func newKVACPSessionRuntimeMappingStore(store state.KV) *kvACPSessionRuntimeMappingStore {
	if store == nil {
		return nil
	}
	return &kvACPSessionRuntimeMappingStore{store: store}
}

// WithACPAuthenticatedUserID binds the authenticated owner id to the ACP request context.
func WithACPAuthenticatedUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, acpAuthenticatedUserKey{}, strings.TrimSpace(userID))
}

func acpAuthenticatedUserID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	userID, _ := ctx.Value(acpAuthenticatedUserKey{}).(string)
	return strings.TrimSpace(userID)
}

func requireACPAuthenticatedUserID(ctx context.Context) (string, error) {
	userID := acpAuthenticatedUserID(ctx)
	if userID == "" {
		return "", fmt.Errorf("authenticated user_id is required")
	}
	return userID, nil
}

func newACPRuntimeID() string {
	return "runtime:" + uuid.NewString()
}

func (s *kvACPSessionRuntimeMappingStore) PutSessionRuntimeMapping(
	ctx context.Context,
	mapping ACPSessionRuntimeMapping,
) error {
	if s == nil || s.store == nil {
		return fmt.Errorf("acp session runtime mapping store is not configured")
	}

	mapping.UserID = strings.TrimSpace(mapping.UserID)
	mapping.SessionID = strings.TrimSpace(mapping.SessionID)
	mapping.RuntimeID = strings.TrimSpace(mapping.RuntimeID)
	mapping.SourceKind = strings.TrimSpace(mapping.SourceKind)

	if mapping.UserID == "" {
		return fmt.Errorf("user_id is required")
	}
	if mapping.SessionID == "" {
		return fmt.Errorf("session_id is required")
	}
	if mapping.RuntimeID == "" {
		return fmt.Errorf("runtime_id is required")
	}
	if mapping.SourceKind == "" {
		return fmt.Errorf("source_kind is required")
	}

	if err := s.store.Set(ctx, s.key(mapping.UserID, mapping.SessionID), map[string]any{
		"user_id":     mapping.UserID,
		"session_id":  mapping.SessionID,
		"runtime_id":  mapping.RuntimeID,
		"source_kind": mapping.SourceKind,
	}); err != nil {
		return fmt.Errorf(
			"persist acp session runtime mapping for user %s session %s: %w",
			mapping.UserID,
			mapping.SessionID,
			err,
		)
	}
	return nil
}

func (s *kvACPSessionRuntimeMappingStore) GetSessionRuntimeMapping(
	ctx context.Context,
	userID, sessionID string,
) (*ACPSessionRuntimeMapping, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("acp session runtime mapping store is not configured")
	}

	userID = strings.TrimSpace(userID)
	sessionID = strings.TrimSpace(sessionID)
	if userID == "" {
		return nil, fmt.Errorf("user_id is required")
	}
	if sessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}

	value, ok, err := s.store.GetMap(ctx, s.key(userID, sessionID))
	if err != nil {
		return nil, fmt.Errorf(
			"load acp session runtime mapping for user %s session %s: %w",
			userID,
			sessionID,
			err,
		)
	}
	if !ok {
		return nil, fmt.Errorf("acp session runtime mapping not found for user %s session %s", userID, sessionID)
	}

	mapping := &ACPSessionRuntimeMapping{
		UserID:     strings.TrimSpace(stringValue(value["user_id"])),
		SessionID:  strings.TrimSpace(stringValue(value["session_id"])),
		RuntimeID:  strings.TrimSpace(stringValue(value["runtime_id"])),
		SourceKind: strings.TrimSpace(stringValue(value["source_kind"])),
	}
	if mapping.UserID == "" || mapping.SessionID == "" || mapping.RuntimeID == "" {
		return nil, fmt.Errorf("acp session runtime mapping for user %s session %s is incomplete", userID, sessionID)
	}
	return mapping, nil
}

func (s *kvACPSessionRuntimeMappingStore) key(userID, sessionID string) string {
	return "acp_session_runtime:" + strings.TrimSpace(userID) + ":" + strings.TrimSpace(sessionID)
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return fmt.Sprint(typed)
	}
}
