package conversationbindings

import (
	"context"
	"fmt"
	"strings"

	"nekobot/pkg/toolsessions"
)

// Service manages conversation-to-tool-session bindings on top of tool sessions.
type Service struct {
	mgr     *toolsessions.Manager
	source  string
	channel string
	prefix  string
}

// New creates a conversation binding service for a specific source/channel pair.
func New(mgr *toolsessions.Manager, source, channel, prefix string) *Service {
	return &Service{
		mgr:     mgr,
		source:  strings.TrimSpace(source),
		channel: strings.TrimSpace(channel),
		prefix:  strings.TrimSpace(prefix),
	}
}

// Bind binds a conversation identifier to a tool session.
func (s *Service) Bind(ctx context.Context, conversationID, sessionID string) error {
	if s == nil || s.mgr == nil {
		return fmt.Errorf("tool session manager is required")
	}
	return s.mgr.BindSessionConversation(ctx, sessionID, s.source, s.channel, s.ConversationKey(conversationID))
}

// Resolve resolves the tool session currently bound to a conversation identifier.
func (s *Service) Resolve(ctx context.Context, conversationID string) (*toolsessions.Session, error) {
	if s == nil || s.mgr == nil {
		return nil, fmt.Errorf("tool session manager is required")
	}
	return s.mgr.FindSessionByConversation(ctx, s.source, s.channel, s.ConversationKey(conversationID))
}

// Clear removes the binding for a conversation identifier.
func (s *Service) Clear(ctx context.Context, conversationID string) error {
	if s == nil || s.mgr == nil {
		return fmt.Errorf("tool session manager is required")
	}
	return s.mgr.ClearConversationBinding(ctx, s.source, s.channel, s.ConversationKey(conversationID))
}

// List lists sessions currently bound under this service source/channel.
func (s *Service) List(ctx context.Context) ([]*toolsessions.Session, error) {
	if s == nil || s.mgr == nil {
		return nil, fmt.Errorf("tool session manager is required")
	}
	return s.mgr.ListSessions(ctx, toolsessions.ListSessionsInput{
		Source: s.source,
		Limit:  200,
	})
}

// ConversationKey returns the persisted conversation key for a raw conversation ID.
func (s *Service) ConversationKey(conversationID string) string {
	trimmed := strings.TrimSpace(conversationID)
	if trimmed == "" {
		return ""
	}
	return s.prefix + trimmed
}

// ConversationID extracts the raw conversation ID from a persisted conversation key.
func (s *Service) ConversationID(conversationKey string) string {
	trimmed := strings.TrimSpace(conversationKey)
	if trimmed == "" {
		return ""
	}
	if s.prefix == "" {
		return trimmed
	}
	if !strings.HasPrefix(trimmed, s.prefix) {
		return ""
	}
	return strings.TrimPrefix(trimmed, s.prefix)
}
