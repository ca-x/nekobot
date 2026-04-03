package conversationbindings

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"nekobot/pkg/toolsessions"
)

const bindingMetadataKey = "conversation_binding"

type bindingState struct {
	ConversationID string
	TargetKind     string
	Placement      string
	ThreadName     string
	Label          string
	BoundBy        string
	IntroText      string
	SessionCwd     string
	Details        map[string]interface{}
	ExpiresAt      *time.Time
}

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
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return fmt.Errorf("conversation id is required")
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return fmt.Errorf("session id is required")
	}
	session, err := s.mgr.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}

	if err := s.removeConversationBindingFromOtherSessions(ctx, conversationID, sessionID); err != nil {
		return err
	}

	states := s.bindingStates(session)
	state := bindingState{
		ConversationID: conversationID,
		TargetKind:     normalizeString(opts.TargetKind, "session"),
		Placement:      normalizeString(opts.Placement, "child"),
		ThreadName:     strings.TrimSpace(opts.ThreadName),
		Label:          strings.TrimSpace(opts.Label),
		BoundBy:        strings.TrimSpace(opts.BoundBy),
		IntroText:      strings.TrimSpace(opts.IntroText),
		SessionCwd:     strings.TrimSpace(opts.SessionCwd),
		Details:        cloneMetadata(opts.Details),
		ExpiresAt:      opts.ExpiresAt,
	}

	replaced := false
	for i := range states {
		if states[i].ConversationID != conversationID {
			continue
		}
		states[i] = state
		replaced = true
		break
	}
	if !replaced {
		states = append(states, state)
	}

	return s.persistBindingStates(ctx, session, states, conversationID)
}

// Resolve resolves the tool session currently bound to a conversation identifier.
func (s *Service) Resolve(ctx context.Context, conversationID string) (*toolsessions.Session, error) {
	if s == nil || s.mgr == nil {
		return nil, fmt.Errorf("tool session manager is required")
	}
	session, _, err := s.findBinding(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	return session, nil
}

// Clear removes the binding for a conversation identifier.
func (s *Service) Clear(ctx context.Context, conversationID string) error {
	if s == nil || s.mgr == nil {
		return fmt.Errorf("tool session manager is required")
	}
	session, state, err := s.findBinding(ctx, conversationID)
	if err != nil {
		return err
	}
	if session == nil || state == nil {
		return nil
	}

	states := s.bindingStates(session)
	updated := make([]bindingState, 0, len(states))
	for _, item := range states {
		if item.ConversationID == state.ConversationID {
			continue
		}
		updated = append(updated, item)
	}

	primaryConversationID := s.ConversationID(session.ConversationKey)
	if primaryConversationID == state.ConversationID {
		primaryConversationID = ""
		if len(updated) > 0 {
			primaryConversationID = updated[0].ConversationID
		}
	}
	return s.persistBindingStates(ctx, session, updated, primaryConversationID)
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
		out = append(out, s.sessionToBindingRecords(item)...)
	}
	sortBindingRecords(out)
	return out, nil
}

// GetBinding returns the binding record for one conversation identifier.
func (s *Service) GetBinding(ctx context.Context, conversationID string) (*BindingRecord, error) {
	session, state, err := s.findBinding(ctx, conversationID)
	if err != nil || session == nil || state == nil {
		return nil, err
	}
	return s.bindingRecordFromState(session, *state), nil
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
	records := s.sessionToBindingRecords(session)
	if len(records) == 0 {
		return []*BindingRecord{}, nil
	}
	sortBindingRecords(records)
	return records, nil
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
	return len(s.bindingStates(session)) > 0
}

func (s *Service) sessionToBindingRecords(session *toolsessions.Session) []*BindingRecord {
	if !s.matchesBinding(session) {
		return []*BindingRecord{}
	}
	states := s.bindingStates(session)
	sortBindingStates(states)
	out := make([]*BindingRecord, 0, len(states))
	for _, state := range states {
		out = append(out, s.bindingRecordFromState(session, state))
	}
	return out
}

func sortBindingStates(states []bindingState) {
	sort.SliceStable(states, func(i, j int) bool {
		left := strings.TrimSpace(states[i].ConversationID)
		right := strings.TrimSpace(states[j].ConversationID)
		if left != right {
			return left < right
		}
		return strings.TrimSpace(states[i].Label) < strings.TrimSpace(states[j].Label)
	})
}

func sortBindingRecords(records []*BindingRecord) {
	sort.SliceStable(records, func(i, j int) bool {
		left := records[i]
		right := records[j]
		if left == nil || right == nil {
			return right != nil
		}
		if left.Conversation.ConversationID != right.Conversation.ConversationID {
			return left.Conversation.ConversationID < right.Conversation.ConversationID
		}
		if left.TargetSessionID != right.TargetSessionID {
			return left.TargetSessionID < right.TargetSessionID
		}
		return left.Metadata.Label < right.Metadata.Label
	})
}

func (s *Service) bindingRecordFromState(session *toolsessions.Session, state bindingState) *BindingRecord {
	return &BindingRecord{
		TargetSessionID: session.ID,
		TargetKind:      normalizeString(state.TargetKind, "session"),
		Placement:       normalizeString(state.Placement, "child"),
		Conversation: BindingConversation{
			Source:         s.source,
			Channel:        s.channel,
			ConversationID: state.ConversationID,
		},
		Metadata: BindingMetadata{
			ThreadName: strings.TrimSpace(state.ThreadName),
			Label:      strings.TrimSpace(state.Label),
			BoundBy:    strings.TrimSpace(state.BoundBy),
			IntroText:  strings.TrimSpace(state.IntroText),
			SessionCwd: strings.TrimSpace(state.SessionCwd),
			Details:    cloneMetadata(state.Details),
		},
		CreatedAt: session.CreatedAt,
		UpdatedAt: session.UpdatedAt,
		ExpiresAt: cloneTime(state.ExpiresAt),
	}
}

func (s *Service) findBinding(
	ctx context.Context,
	conversationID string,
) (*toolsessions.Session, *bindingState, error) {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return nil, nil, nil
	}
	items, err := s.List(ctx)
	if err != nil {
		return nil, nil, err
	}
	for _, session := range items {
		for _, state := range s.bindingStates(session) {
			if state.ConversationID != conversationID {
				continue
			}
			stateCopy := state
			return session, &stateCopy, nil
		}
	}
	return nil, nil, nil
}

func (s *Service) removeConversationBindingFromOtherSessions(
	ctx context.Context,
	conversationID, keepSessionID string,
) error {
	items, err := s.List(ctx)
	if err != nil {
		return err
	}
	for _, session := range items {
		if session == nil || strings.TrimSpace(session.ID) == keepSessionID {
			continue
		}
		states := s.bindingStates(session)
		sortBindingStates(states)
		updated := make([]bindingState, 0, len(states))
		removed := false
		for _, state := range states {
			if state.ConversationID == conversationID {
				removed = true
				continue
			}
			updated = append(updated, state)
		}
		if !removed {
			continue
		}
		primaryConversationID := s.ConversationID(session.ConversationKey)
		if primaryConversationID == conversationID {
			primaryConversationID = ""
			if len(updated) > 0 {
				primaryConversationID = updated[0].ConversationID
			}
		}
		if err := s.persistBindingStates(ctx, session, updated, primaryConversationID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) persistBindingStates(
	ctx context.Context,
	session *toolsessions.Session,
	states []bindingState,
	primaryConversationID string,
) error {
	if session == nil {
		return nil
	}
	metadata := cloneMetadata(session.Metadata)
	if len(states) == 0 {
		delete(metadata, bindingMetadataKey)
		currentPrimary := s.ConversationID(session.ConversationKey)
		if currentPrimary != "" {
			if err := s.mgr.ClearConversationBinding(ctx, s.source, s.channel, s.ConversationKey(currentPrimary)); err != nil {
				return err
			}
		}
		return s.mgr.UpdateSessionMetadata(ctx, session.ID, metadata)
	}

	primaryConversationID = strings.TrimSpace(primaryConversationID)
	sortBindingStates(states)
	if primaryConversationID == "" {
		primaryConversationID = states[0].ConversationID
	}

	metadata[bindingMetadataKey] = map[string]interface{}{
		"records": statesToMetadataRecords(states),
	}

	currentPrimary := s.ConversationID(session.ConversationKey)
	if currentPrimary != primaryConversationID {
		if err := s.mgr.BindSessionConversation(
			ctx,
			session.ID,
			s.source,
			s.channel,
			s.ConversationKey(primaryConversationID),
		); err != nil {
			return err
		}
	}

	return s.mgr.UpdateSessionMetadata(ctx, session.ID, metadata)
}

func (s *Service) bindingStates(session *toolsessions.Session) []bindingState {
	if session == nil {
		return []bindingState{}
	}
	metadata := readBindingMetadata(session.Metadata)
	if rawRecords, ok := metadata["records"].([]interface{}); ok {
		return metadataRecordsToStates(rawRecords)
	}

	conversationID := s.ConversationID(session.ConversationKey)
	if conversationID == "" {
		return []bindingState{}
	}

	return []bindingState{{
		ConversationID: conversationID,
		TargetKind:     normalizeString(metadata["target_kind"], "session"),
		Placement:      normalizeString(metadata["placement"], "child"),
		ThreadName:     strings.TrimSpace(stringValue(metadata["thread_name"])),
		Label:          strings.TrimSpace(stringValue(metadata["label"])),
		BoundBy:        strings.TrimSpace(stringValue(metadata["bound_by"])),
		IntroText:      strings.TrimSpace(stringValue(metadata["intro_text"])),
		SessionCwd:     strings.TrimSpace(stringValue(metadata["session_cwd"])),
		Details:        cloneMetadata(mapValue(metadata["details"])),
		ExpiresAt:      parseExpiry(metadata["expires_at"]),
	}}
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

func statesToMetadataRecords(states []bindingState) []interface{} {
	records := make([]interface{}, 0, len(states))
	for _, state := range states {
		records = append(records, map[string]interface{}{
			"conversation_id": strings.TrimSpace(state.ConversationID),
			"target_kind":     normalizeString(state.TargetKind, "session"),
			"placement":       normalizeString(state.Placement, "child"),
			"thread_name":     strings.TrimSpace(state.ThreadName),
			"label":           strings.TrimSpace(state.Label),
			"bound_by":        strings.TrimSpace(state.BoundBy),
			"intro_text":      strings.TrimSpace(state.IntroText),
			"session_cwd":     strings.TrimSpace(state.SessionCwd),
			"details":         cloneMetadata(state.Details),
			"expires_at":      formatExpiry(state.ExpiresAt),
		})
	}
	return records
}

func metadataRecordsToStates(records []interface{}) []bindingState {
	states := make([]bindingState, 0, len(records))
	for _, raw := range records {
		record := mapValue(raw)
		conversationID := strings.TrimSpace(stringValue(record["conversation_id"]))
		if conversationID == "" {
			continue
		}
		states = append(states, bindingState{
			ConversationID: conversationID,
			TargetKind:     normalizeString(record["target_kind"], "session"),
			Placement:      normalizeString(record["placement"], "child"),
			ThreadName:     strings.TrimSpace(stringValue(record["thread_name"])),
			Label:          strings.TrimSpace(stringValue(record["label"])),
			BoundBy:        strings.TrimSpace(stringValue(record["bound_by"])),
			IntroText:      strings.TrimSpace(stringValue(record["intro_text"])),
			SessionCwd:     strings.TrimSpace(stringValue(record["session_cwd"])),
			Details:        cloneMetadata(mapValue(record["details"])),
			ExpiresAt:      parseExpiry(record["expires_at"]),
		})
	}
	return states
}

func cloneTime(src *time.Time) *time.Time {
	if src == nil {
		return nil
	}
	dst := *src
	return &dst
}
