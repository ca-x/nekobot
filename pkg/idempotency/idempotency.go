// Package idempotency provides request deduplication for mutating RPCs.
//
// Every side-effecting RPC must call [Store.Reserve] before executing. If the
// reserve returns an existing terminal record, the caller replays the cached
// response instead of re-executing the mutation.
package idempotency

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"nekobot/pkg/storage/ent"
	"nekobot/pkg/storage/ent/idempotencyrecord"
)

// Status values for an idempotency record.
const (
	StatusPending   = "pending"
	StatusSucceeded = "succeeded"
	StatusFailed    = "failed"
)

// Outcome describes what happened when reserving an idempotency key.
type Outcome string

const (
	// OutcomeReserved means a new pending record was created; the caller should execute.
	OutcomeReserved Outcome = "reserved"
	// OutcomeReplay means a prior succeeded/failed record exists with the same hash.
	OutcomeReplay Outcome = "replay"
	// OutcomeConflict means a prior record exists with a different request hash.
	OutcomeConflict Outcome = "conflict"
	// OutcomeInProgress means the same request is currently being processed.
	OutcomeInProgress Outcome = "in_progress"
)

// Result is returned by Reserve and Check.
type Result struct {
	Outcome      Outcome
	Record       *Record
	ExistingHash string // populated on conflict
}

// Record is the domain model for an idempotency record.
type Record struct {
	ID           string
	TenantID     string
	CallerKind   string
	CallerID     string
	Method       string
	RequestID    string
	RequestHash  string
	Status       string
	ResponseType string
	ResponseJSON string
	ErrorCode    string
	ErrorMessage string
	ResourceKind string
	ResourceID   string
	EventID      string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	ExpiresAt    time.Time
}

// CompleteRequest carries the terminal result of a mutation.
type CompleteRequest struct {
	ResponseType string // e.g. "proto:SendMessageResponse" or "json:result"
	ResponseJSON string // sanitized response body
	ResourceKind string // e.g. "message", "task", "run_step"
	ResourceID   string // the created/updated resource ID
	EventID      string // optional: event_log event ID
}

// FailRequest carries the terminal failure of a mutation.
type FailRequest struct {
	ErrorCode    string
	ErrorMessage string
}

// Store provides idempotency operations backed by an Ent client.
type Store struct {
	client *ent.Client
}

// NewStore creates an idempotency store.
func NewStore(client *ent.Client) *Store {
	return &Store{client: client}
}

// Key identifies an idempotency record.
type Key struct {
	TenantID   string
	CallerKind string // "user", "agent", "computer", "system"
	CallerID   string
	Method     string // e.g. "SendMessage", "AppendRunStep"
	RequestID  string
}

// Reserve attempts to create a pending idempotency record.
//
// Call this before executing the mutation. Possible outcomes:
//   - OutcomeReserved: new record created; proceed with mutation.
//   - OutcomeReplay: identical request already completed; return cached result.
//   - OutcomeConflict: same request_id but different body; reject.
//   - OutcomeInProgress: identical request is being processed; retry later.
func (s *Store) Reserve(ctx context.Context, key Key, requestHash string, ttl time.Duration) (*Result, error) {
	key = normalizeKey(key)
	if err := validateKey(key); err != nil {
		return nil, err
	}
	if requestHash == "" {
		return nil, fmt.Errorf("request_hash is required")
	}

	// Look up existing record.
	existing, err := s.client.IdempotencyRecord.Query().
		Where(
			idempotencyrecord.TenantIDEQ(key.TenantID),
			idempotencyrecord.CallerKindEQ(key.CallerKind),
			idempotencyrecord.CallerIDEQ(key.CallerID),
			idempotencyrecord.MethodEQ(key.Method),
			idempotencyrecord.RequestIDEQ(key.RequestID),
		).
		First(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, fmt.Errorf("query idempotency record: %w", err)
	}

	if existing != nil {
		// Record exists.
		if existing.RequestHash != requestHash {
			return &Result{
				Outcome:      OutcomeConflict,
				ExistingHash: existing.RequestHash,
			}, nil
		}
		switch existing.Status {
		case StatusSucceeded, StatusFailed:
			return &Result{
				Outcome: OutcomeReplay,
				Record:  entToRecord(existing),
			}, nil
		case StatusPending:
			return &Result{
				Outcome: OutcomeInProgress,
				Record:  entToRecord(existing),
			}, nil
		}
	}

	// Insert new pending record.
	if ttl <= 0 {
		ttl = 30 * 24 * time.Hour
	}
	rec, err := s.client.IdempotencyRecord.Create().
		SetTenantID(key.TenantID).
		SetCallerKind(key.CallerKind).
		SetCallerID(key.CallerID).
		SetMethod(key.Method).
		SetRequestID(key.RequestID).
		SetRequestHash(requestHash).
		SetStatus(StatusPending).
		SetExpiresAt(time.Now().Add(ttl)).
		Save(ctx)
	if err != nil {
		// Handle unique constraint violation (concurrent insert).
		if ent.IsConstraintError(err) {
			// Another goroutine inserted first — re-read.
			return s.Reserve(ctx, key, requestHash, ttl)
		}
		return nil, fmt.Errorf("insert idempotency record: %w", err)
	}
	return &Result{
		Outcome: OutcomeReserved,
		Record:  entToRecord(rec),
	}, nil
}

// Complete marks a pending record as succeeded with the mutation result.
func (s *Store) Complete(ctx context.Context, key Key, req CompleteRequest) (*Record, error) {
	key = normalizeKey(key)
	rec, err := s.lookup(ctx, key)
	if err != nil {
		return nil, err
	}
	if rec == nil {
		return nil, fmt.Errorf("no idempotency record for key")
	}
	if rec.Status != StatusPending {
		return entToRecordEnt(rec), nil // already terminal
	}
	updated, err := s.client.IdempotencyRecord.UpdateOneID(rec.ID).
		SetStatus(StatusSucceeded).
		SetResponseType(req.ResponseType).
		SetResponseJSON(req.ResponseJSON).
		SetResourceKind(req.ResourceKind).
		SetResourceID(req.ResourceID).
		SetEventID(req.EventID).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("complete idempotency record: %w", err)
	}
	return entToRecord(updated), nil
}

// Fail marks a pending record as failed.
func (s *Store) Fail(ctx context.Context, key Key, req FailRequest) (*Record, error) {
	key = normalizeKey(key)
	rec, err := s.lookup(ctx, key)
	if err != nil {
		return nil, err
	}
	if rec == nil {
		return nil, fmt.Errorf("no idempotency record for key")
	}
	if rec.Status != StatusPending {
		return entToRecordEnt(rec), nil
	}
	updated, err := s.client.IdempotencyRecord.UpdateOneID(rec.ID).
		SetStatus(StatusFailed).
		SetErrorCode(req.ErrorCode).
		SetErrorMessage(req.ErrorMessage).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("fail idempotency record: %w", err)
	}
	return entToRecord(updated), nil
}

// Check looks up an existing record without creating one.
func (s *Store) Check(ctx context.Context, key Key) (*Result, error) {
	key = normalizeKey(key)
	existing, err := s.lookup(ctx, key)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return &Result{Outcome: OutcomeReserved}, nil // no record exists
	}
	switch existing.Status {
	case StatusSucceeded, StatusFailed:
		return &Result{Outcome: OutcomeReplay, Record: entToRecord(existing)}, nil
	case StatusPending:
		return &Result{Outcome: OutcomeInProgress, Record: entToRecord(existing)}, nil
	}
	return &Result{Outcome: OutcomeReserved}, nil
}

// Hash computes a SHA-256 hex digest from a canonical request representation.
// The input should be a deterministic JSON or struct encoding of the request
// body after validation, excluding auth headers and timestamps.
func Hash(canonical []byte) string {
	h := sha256.Sum256(canonical)
	return hex.EncodeToString(h[:])
}

// HashJSON marshals v to canonical JSON and returns its hash.
func HashJSON(v interface{}) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("marshal request for hash: %w", err)
	}
	return Hash(b), nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func (s *Store) lookup(ctx context.Context, key Key) (*ent.IdempotencyRecord, error) {
	rec, err := s.client.IdempotencyRecord.Query().
		Where(
			idempotencyrecord.TenantIDEQ(key.TenantID),
			idempotencyrecord.CallerKindEQ(key.CallerKind),
			idempotencyrecord.CallerIDEQ(key.CallerID),
			idempotencyrecord.MethodEQ(key.Method),
			idempotencyrecord.RequestIDEQ(key.RequestID),
		).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("lookup idempotency record: %w", err)
	}
	return rec, nil
}

func normalizeKey(k Key) Key {
	return Key{
		TenantID:   strings.TrimSpace(k.TenantID),
		CallerKind: strings.TrimSpace(k.CallerKind),
		CallerID:   strings.TrimSpace(k.CallerID),
		Method:     strings.TrimSpace(k.Method),
		RequestID:  strings.TrimSpace(k.RequestID),
	}
}

func validateKey(k Key) error {
	if k.Method == "" {
		return fmt.Errorf("method is required")
	}
	if k.RequestID == "" {
		return fmt.Errorf("request_id is required")
	}
	if k.CallerKind == "" {
		return fmt.Errorf("caller_kind is required")
	}
	return nil
}

func entToRecord(rec *ent.IdempotencyRecord) *Record {
	if rec == nil {
		return nil
	}
	return &Record{
		ID:           rec.ID,
		TenantID:     rec.TenantID,
		CallerKind:   rec.CallerKind,
		CallerID:     rec.CallerID,
		Method:       rec.Method,
		RequestID:    rec.RequestID,
		RequestHash:  rec.RequestHash,
		Status:       rec.Status,
		ResponseType: rec.ResponseType,
		ResponseJSON: rec.ResponseJSON,
		ErrorCode:    rec.ErrorCode,
		ErrorMessage: rec.ErrorMessage,
		ResourceKind: rec.ResourceKind,
		ResourceID:   rec.ResourceID,
		EventID:      rec.EventID,
		CreatedAt:    rec.CreatedAt,
		UpdatedAt:    rec.UpdatedAt,
		ExpiresAt:    rec.ExpiresAt,
	}
}

func entToRecordEnt(rec *ent.IdempotencyRecord) *Record {
	return entToRecord(rec)
}
