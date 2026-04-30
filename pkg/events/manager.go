// Package events manages the durable append-only daemon collaboration event log.
package events

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"nekobot/pkg/storage/ent"
	"nekobot/pkg/storage/ent/collaborationevent"
)

const (
	DefaultTenantID = "default"
	DefaultStream   = "tenant:default"
	DefaultLimit    = 100
	MaxLimit        = 500
)

var ErrCursorFilterMismatch = errors.New("event cursor filter mismatch")

// EventRecord is the domain model for one collaboration event.
type EventRecord struct {
	ID                string
	TenantID          string
	ServerID          string
	Stream            string
	Sequence          int64
	EventID           string
	EventType         string
	Target            string
	ThreadID          string
	ActorKind         string
	ActorID           string
	SubjectKind       string
	SubjectID         string
	ParentSubjectKind string
	ParentSubjectID   string
	AssigneeID        string
	MentionedAgentIDs []string
	CapabilityKeys    []string
	GraphVersion      int64
	IdempotencyKey    string
	PayloadJSON       string
	CreatedAt         time.Time
}

// ListFilter constrains replay results and is included in opaque cursors.
type ListFilter struct {
	TenantID    string
	ServerID    string
	Stream      string
	Target      string
	ActorID     string
	AssigneeID  string
	SubjectKind string
	SubjectID   string
	EventTypes  []string
}

// Manager appends and replays collaboration events.
type Manager struct {
	client *ent.Client
}

// NewManager creates a collaboration event manager backed by Ent.
func NewManager(client *ent.Client) (*Manager, error) {
	if client == nil {
		return nil, fmt.Errorf("ent client is nil")
	}
	return &Manager{client: client}, nil
}

// Append inserts a new event and assigns the next sequence for its tenant/stream.
func (m *Manager) Append(ctx context.Context, item EventRecord) (*EventRecord, error) {
	item = normalizeRecord(item)
	if item.EventType == "" {
		return nil, fmt.Errorf("event_type is required")
	}
	if item.EventID == "" {
		item.EventID = uuid.NewString()
	}
	if item.ID == "" {
		item.ID = uuid.NewString()
	}

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		seq, err := m.nextSequence(ctx, item.TenantID, item.Stream)
		if err != nil {
			return nil, err
		}
		item.Sequence = seq

		rec, err := m.client.CollaborationEvent.Create().
			SetID(item.ID).
			SetTenantID(item.TenantID).
			SetServerID(item.ServerID).
			SetStream(item.Stream).
			SetSequence(item.Sequence).
			SetEventID(item.EventID).
			SetEventType(item.EventType).
			SetTarget(item.Target).
			SetThreadID(item.ThreadID).
			SetActorKind(item.ActorKind).
			SetActorID(item.ActorID).
			SetSubjectKind(item.SubjectKind).
			SetSubjectID(item.SubjectID).
			SetParentSubjectKind(item.ParentSubjectKind).
			SetParentSubjectID(item.ParentSubjectID).
			SetAssigneeID(item.AssigneeID).
			SetMentionedAgentIdsJSON(marshalStrings(item.MentionedAgentIDs)).
			SetCapabilityKeysJSON(marshalStrings(item.CapabilityKeys)).
			SetGraphVersion(item.GraphVersion).
			SetIdempotencyKey(item.IdempotencyKey).
			SetPayloadJSON(item.PayloadJSON).
			Save(ctx)
		if err == nil {
			return entEventToRecord(rec), nil
		}
		lastErr = err
		if !ent.IsConstraintError(err) {
			break
		}
	}
	return nil, fmt.Errorf("append collaboration event: %w", lastErr)
}

// ListSince returns events after an opaque cursor. The returned cursor should be
// passed unchanged to the next call.
func (m *Manager) ListSince(ctx context.Context, cursor string, filter ListFilter, limit int) ([]EventRecord, string, error) {
	filter = normalizeFilter(filter)
	afterSeq := int64(0)
	if strings.TrimSpace(cursor) != "" {
		decoded, err := decodeCursor(cursor)
		if err != nil {
			return nil, "", err
		}
		if decoded.FiltersHash != filterHash(filter) {
			return nil, "", ErrCursorFilterMismatch
		}
		if decoded.TenantID != filter.TenantID || decoded.Stream != filter.Stream || decoded.ServerID != filter.ServerID {
			return nil, "", ErrCursorFilterMismatch
		}
		afterSeq = decoded.AfterSeq
	}
	limit = normalizeLimit(limit)

	q := m.client.CollaborationEvent.Query().
		Where(
			collaborationevent.TenantIDEQ(filter.TenantID),
			collaborationevent.StreamEQ(filter.Stream),
			collaborationevent.SequenceGT(afterSeq),
		).
		Order(ent.Asc(collaborationevent.FieldSequence)).
		Limit(limit)

	if filter.Target != "" {
		q = q.Where(collaborationevent.TargetEQ(filter.Target))
	}
	if filter.ActorID != "" {
		q = q.Where(collaborationevent.ActorIDEQ(filter.ActorID))
	}
	if filter.AssigneeID != "" {
		q = q.Where(collaborationevent.AssigneeIDEQ(filter.AssigneeID))
	}
	if filter.SubjectKind != "" {
		q = q.Where(collaborationevent.SubjectKindEQ(filter.SubjectKind))
	}
	if filter.SubjectID != "" {
		q = q.Where(collaborationevent.SubjectIDEQ(filter.SubjectID))
	}
	if len(filter.EventTypes) > 0 {
		q = q.Where(collaborationevent.EventTypeIn(filter.EventTypes...))
	}

	recs, err := q.All(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("list collaboration events: %w", err)
	}
	out := make([]EventRecord, 0, len(recs))
	for _, rec := range recs {
		out = append(out, *entEventToRecord(rec))
		if rec.Sequence > afterSeq {
			afterSeq = rec.Sequence
		}
	}
	next, err := encodeCursor(cursorPayload{
		Version:     1,
		ServerID:    filter.ServerID,
		Stream:      filter.Stream,
		TenantID:    filter.TenantID,
		AfterSeq:    afterSeq,
		FiltersHash: filterHash(filter),
		IssuedAt:    time.Now().UTC(),
	})
	if err != nil {
		return nil, "", err
	}
	return out, next, nil
}

func (m *Manager) nextSequence(ctx context.Context, tenantID, stream string) (int64, error) {
	rec, err := m.client.CollaborationEvent.Query().
		Where(
			collaborationevent.TenantIDEQ(tenantID),
			collaborationevent.StreamEQ(stream),
		).
		Order(ent.Desc(collaborationevent.FieldSequence)).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return 1, nil
		}
		return 0, fmt.Errorf("query last collaboration event sequence: %w", err)
	}
	return rec.Sequence + 1, nil
}

type cursorPayload struct {
	Version     int       `json:"version"`
	ServerID    string    `json:"server_id"`
	Stream      string    `json:"stream"`
	TenantID    string    `json:"tenant_id"`
	AfterSeq    int64     `json:"after_seq"`
	FiltersHash string    `json:"filters_hash"`
	IssuedAt    time.Time `json:"issued_at"`
}

func encodeCursor(payload cursorPayload) (string, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal event cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func decodeCursor(raw string) (cursorPayload, error) {
	b, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(raw))
	if err != nil {
		return cursorPayload{}, fmt.Errorf("decode event cursor: %w", err)
	}
	var payload cursorPayload
	if err := json.Unmarshal(b, &payload); err != nil {
		return cursorPayload{}, fmt.Errorf("unmarshal event cursor: %w", err)
	}
	if payload.Version != 1 {
		return cursorPayload{}, fmt.Errorf("unsupported event cursor version %d", payload.Version)
	}
	return payload, nil
}

func filterHash(filter ListFilter) string {
	filter = normalizeFilter(filter)
	b, _ := json.Marshal(filter)
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func normalizeRecord(item EventRecord) EventRecord {
	item.TenantID = defaultString(item.TenantID, DefaultTenantID)
	item.Stream = defaultString(item.Stream, defaultStreamForTenant(item.TenantID))
	item.ActorKind = defaultString(item.ActorKind, "system")
	item.PayloadJSON = defaultString(item.PayloadJSON, "{}")
	item.EventType = strings.TrimSpace(item.EventType)
	item.MentionedAgentIDs = normalizeStrings(item.MentionedAgentIDs)
	item.CapabilityKeys = normalizeStrings(item.CapabilityKeys)
	return item
}

func normalizeFilter(filter ListFilter) ListFilter {
	filter.TenantID = defaultString(filter.TenantID, DefaultTenantID)
	filter.Stream = defaultString(filter.Stream, defaultStreamForTenant(filter.TenantID))
	filter.EventTypes = normalizeStrings(filter.EventTypes)
	return filter
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return DefaultLimit
	}
	if limit > MaxLimit {
		return MaxLimit
	}
	return limit
}

func defaultString(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func defaultStreamForTenant(tenantID string) string {
	tenantID = defaultString(tenantID, DefaultTenantID)
	return "tenant:" + tenantID
}

func normalizeStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func marshalStrings(values []string) string {
	values = normalizeStrings(values)
	if len(values) == 0 {
		return "[]"
	}
	b, _ := json.Marshal(values)
	return string(b)
}

func unmarshalStrings(raw string) []string {
	var values []string
	if strings.TrimSpace(raw) != "" {
		_ = json.Unmarshal([]byte(raw), &values)
	}
	return normalizeStrings(values)
}

func entEventToRecord(rec *ent.CollaborationEvent) *EventRecord {
	if rec == nil {
		return nil
	}
	return &EventRecord{
		ID:                rec.ID,
		TenantID:          rec.TenantID,
		ServerID:          rec.ServerID,
		Stream:            rec.Stream,
		Sequence:          rec.Sequence,
		EventID:           rec.EventID,
		EventType:         rec.EventType,
		Target:            rec.Target,
		ThreadID:          rec.ThreadID,
		ActorKind:         rec.ActorKind,
		ActorID:           rec.ActorID,
		SubjectKind:       rec.SubjectKind,
		SubjectID:         rec.SubjectID,
		ParentSubjectKind: rec.ParentSubjectKind,
		ParentSubjectID:   rec.ParentSubjectID,
		AssigneeID:        rec.AssigneeID,
		MentionedAgentIDs: unmarshalStrings(rec.MentionedAgentIdsJSON),
		CapabilityKeys:    unmarshalStrings(rec.CapabilityKeysJSON),
		GraphVersion:      rec.GraphVersion,
		IdempotencyKey:    rec.IdempotencyKey,
		PayloadJSON:       rec.PayloadJSON,
		CreatedAt:         rec.CreatedAt,
	}
}
