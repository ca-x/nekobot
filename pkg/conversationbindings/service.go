package conversationbindings

import (
	"context"
	"fmt"
	"strings"
	"time"

	"nekobot/pkg/toolsessions"
)

const bindingMetadataKey = "conversation_binding"

// BindOptions describes optional metadata for a conversation binding.
type BindOptions struct {
	TargetKind string
	Placement  string
	ThreadName string
	Label      string
	BoundBy    string
	IntroText  string
	SessionCwd string
	Details    map[string]interface{}
	ExpiresAt  *time.Time
}

// BindingConversation identifies the bound source conversation.
type BindingConversation struct {
	Source         string
	Channel        string
	ConversationID string
}

// BindingMetadata contains extra binding presentation and lifecycle fields.
type BindingMetadata struct {
	ThreadName string
	Label      string
	BoundBy    string
	IntroText  string
	SessionCwd string
	Details    map[string]interface{}
}

// BindingRecord is the reusable thread/conversation binding view built on tool sessions.
type BindingRecord struct {
	TargetSessionID string
	TargetKind      string
	Placement       string
	Conversation    BindingConversation
	Metadata        BindingMetadata
	CreatedAt       time.Time
	UpdatedAt       time.Time
	ExpiresAt       *time.Time
}

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
	return s.BindWithOptions(ctx, conversationID, sessionID, BindOptions{})
}

// BindWithOptions binds a conversation identifier to a session with binding metadata.
func (s *Service) BindWithOptions(ctx context.Context, conversationID, sessionID string, opts BindOptions) error {
	if s == nil || s.mgr == nil {
		return fmt.Errorf("tool session manager is required")
	}
	if err := s.mgr.BindSessionConversation(ctx, sessionID, s.source, s.channel, s.ConversationKey(conversationID)); err != nil {
		return err
	}

	session, err := s.mgr.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}

	metadata := cloneMetadata(session.Metadata)
	metadata[bindingMetadataKey] = map[string]interface{}{
		"target_kind": normalizeString(opts.TargetKind, "session"),
		"placement":   normalizeString(opts.Placement, "child"),
		"thread_name": strings.TrimSpace(opts.ThreadName),
		"label":       strings.TrimSpace(opts.Label),
		"bound_by":    strings.TrimSpace(opts.BoundBy),
		"intro_text":  strings.TrimSpace(opts.IntroText),
		"session_cwd": strings.TrimSpace(opts.SessionCwd),
		"details":     cloneMetadata(opts.Details),
		"expires_at":  formatExpiry(opts.ExpiresAt),
	}
	return s.mgr.UpdateSessionMetadata(ctx, sessionID, metadata)
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
	key := s.ConversationKey(conversationID)
	session, err := s.mgr.FindSessionByConversation(ctx, s.source, s.channel, key)
	if err != nil {
		return err
	}
	if err := s.mgr.ClearConversationBinding(ctx, s.source, s.channel, key); err != nil {
		return err
	}
	if session != nil {
		return s.clearBindingMetadata(ctx, session)
	}
	return nil
}

// List lists sessions currently bound under this service source/channel.
func (s *Service) List(ctx context.Context) ([]*toolsessions.Session, error) {
	if s == nil || s.mgr == nil {
		return nil, fmt.Errorf("tool session manager is required")
	}
	items, err := s.mgr.ListSessions(ctx, toolsessions.ListSessionsInput{
		Source: s.source,
		Limit:  200,
	})
	if err != nil {
		return nil, err
	}
	out := make([]*toolsessions.Session, 0, len(items))
	for _, item := range items {
		if !s.matchesBinding(item) {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

// ListBindings returns rich binding records under this service scope.
func (s *Service) ListBindings(ctx context.Context) ([]*BindingRecord, error) {
	items, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*BindingRecord, 0, len(items))
	for _, item := range items {
		record := s.sessionToBindingRecord(item)
		if record == nil {
			continue
		}
		out = append(out, record)
	}
	return out, nil
}

// GetBinding returns the binding record for one conversation identifier.
func (s *Service) GetBinding(ctx context.Context, conversationID string) (*BindingRecord, error) {
	session, err := s.Resolve(ctx, conversationID)
	if err != nil || session == nil {
		return nil, err
	}
	return s.sessionToBindingRecord(session), nil
}

// GetBindingsBySession returns binding records for a target session.
func (s *Service) GetBindingsBySession(ctx context.Context, sessionID string) ([]*BindingRecord, error) {
	if s == nil || s.mgr == nil {
		return nil, fmt.Errorf("tool session manager is required")
	}
	session, err := s.mgr.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if !s.matchesBinding(session) {
		return []*BindingRecord{}, nil
	}
	record := s.sessionToBindingRecord(session)
	if record == nil {
		return []*BindingRecord{}, nil
	}
	return []*BindingRecord{record}, nil
}

// CleanupExpired clears expired bindings under this service scope.
func (s *Service) CleanupExpired(ctx context.Context) (int, error) {
	records, err := s.ListBindings(ctx)
	if err != nil {
		return 0, err
	}
	now := time.Now()
	cleaned := 0
	for _, record := range records {
		if record == nil || record.ExpiresAt == nil || !record.ExpiresAt.Before(now) {
			continue
		}
		if err := s.Clear(ctx, record.Conversation.ConversationID); err != nil {
			return cleaned, err
		}
		cleaned++
	}
	return cleaned, nil
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

func (s *Service) matchesBinding(session *toolsessions.Session) bool {
	if session == nil {
		return false
	}
	if strings.TrimSpace(session.Source) != s.source {
		return false
	}
	if s.channel != "" && strings.TrimSpace(session.Channel) != s.channel {
		return false
	}
	return s.ConversationID(session.ConversationKey) != ""
}

func (s *Service) sessionToBindingRecord(session *toolsessions.Session) *BindingRecord {
	if !s.matchesBinding(session) {
		return nil
	}
	meta := readBindingMetadata(session.Metadata)
	return &BindingRecord{
		TargetSessionID: session.ID,
		TargetKind:      normalizeString(meta["target_kind"], "session"),
		Placement:       normalizeString(meta["placement"], "child"),
		Conversation: BindingConversation{
			Source:         s.source,
			Channel:        s.channel,
			ConversationID: s.ConversationID(session.ConversationKey),
		},
		Metadata: BindingMetadata{
			ThreadName: strings.TrimSpace(stringValue(meta["thread_name"])),
			Label:      strings.TrimSpace(stringValue(meta["label"])),
			BoundBy:    strings.TrimSpace(stringValue(meta["bound_by"])),
			IntroText:  strings.TrimSpace(stringValue(meta["intro_text"])),
			SessionCwd: strings.TrimSpace(stringValue(meta["session_cwd"])),
			Details:    cloneMetadata(mapValue(meta["details"])),
		},
		CreatedAt: session.CreatedAt,
		UpdatedAt: session.UpdatedAt,
		ExpiresAt: parseExpiry(meta["expires_at"]),
	}
}

func (s *Service) clearBindingMetadata(ctx context.Context, session *toolsessions.Session) error {
	if session == nil {
		return nil
	}
	metadata := cloneMetadata(session.Metadata)
	delete(metadata, bindingMetadataKey)
	return s.mgr.UpdateSessionMetadata(ctx, session.ID, metadata)
}

func cloneMetadata(src map[string]interface{}) map[string]interface{} {
	if len(src) == 0 {
		return map[string]interface{}{}
	}
	dst := make(map[string]interface{}, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func readBindingMetadata(metadata map[string]interface{}) map[string]interface{} {
	if len(metadata) == 0 {
		return map[string]interface{}{}
	}
	raw, ok := metadata[bindingMetadataKey]
	if !ok {
		return map[string]interface{}{}
	}
	return mapValue(raw)
}

func mapValue(v interface{}) map[string]interface{} {
	if v == nil {
		return map[string]interface{}{}
	}
	if typed, ok := v.(map[string]interface{}); ok {
		return cloneMetadata(typed)
	}
	return map[string]interface{}{}
}

func stringValue(v interface{}) string {
	text, _ := v.(string)
	return text
}

func normalizeString(v interface{}, fallback string) string {
	text := strings.TrimSpace(stringValue(v))
	if text == "" {
		return fallback
	}
	return text
}

func formatExpiry(expiresAt *time.Time) string {
	if expiresAt == nil {
		return ""
	}
	return expiresAt.UTC().Format(time.RFC3339)
}

func parseExpiry(v interface{}) *time.Time {
	text := strings.TrimSpace(stringValue(v))
	if text == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, text)
	if err != nil {
		return nil
	}
	return &parsed
}
