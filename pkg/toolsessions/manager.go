package toolsessions

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/storage/ent"
	"nekobot/pkg/storage/ent/attachtoken"
	"nekobot/pkg/storage/ent/toolevent"
	"nekobot/pkg/storage/ent/toolsession"
)

const (
	defaultOTPTTL        = 3 * time.Minute
	minOTPTTLSeconds     = 30
	maxOTPTTLSeconds     = 3600
	defaultOTPTTLSeconds = 180
)

// Manager persists and manages tool sessions with lifecycle control.
type Manager struct {
	log       *logger.Logger
	client    *ent.Client
	lifecycle LifecycleConfig
	otpTTL    time.Duration
	otpMu     sync.Mutex
	otpCodes  map[string]sessionOTP
}

type sessionOTP struct {
	hash      string
	expiresAt time.Time
}

// NewManager creates a new tool-session manager with an injected shared Ent client.
func NewManager(cfg *config.Config, log *logger.Logger, client *ent.Client) (*Manager, error) {
	if client == nil {
		return nil, fmt.Errorf("ent client is nil")
	}
	mgr := &Manager{
		log:       log,
		client:    client,
		lifecycle: defaultLifecycleConfig(),
		otpTTL:    normalizeOTPTTLSeconds(cfg.WebUI.ToolSessionOTPTTLSeconds),
		otpCodes:  map[string]sessionOTP{},
	}
	dbPath, _ := config.RuntimeDBPath(cfg)

	log.Info("Tool session storage initialized",
		zap.String("db_path", dbPath),
	)

	return mgr, nil
}

// Close releases manager resources. Shared Ent client is closed by config module.
func (m *Manager) Close() error {
	m.otpMu.Lock()
	m.otpCodes = map[string]sessionOTP{}
	m.otpMu.Unlock()
	return nil
}

// Lifecycle returns the current lifecycle config.
func (m *Manager) Lifecycle() LifecycleConfig {
	return m.lifecycle
}

// SetLifecycle overrides lifecycle config values (zero values keep defaults).
func (m *Manager) SetLifecycle(cfg LifecycleConfig) {
	base := defaultLifecycleConfig()
	if cfg.SweepInterval > 0 {
		base.SweepInterval = cfg.SweepInterval
	}
	if cfg.RunningIdleTimeout > 0 {
		base.RunningIdleTimeout = cfg.RunningIdleTimeout
	}
	if cfg.DetachedTTL > 0 {
		base.DetachedTTL = cfg.DetachedTTL
	}
	if cfg.MaxLifetime > 0 {
		base.MaxLifetime = cfg.MaxLifetime
	}
	if cfg.TerminatedRetention > 0 {
		base.TerminatedRetention = cfg.TerminatedRetention
	}
	m.lifecycle = base
}

// CreateSession persists a new tool session.
func (m *Manager) CreateSession(ctx context.Context, input CreateSessionInput) (*Session, error) {
	now := time.Now()
	state := normalizeState(input.State)
	if state == "" {
		state = StateRunning
	}
	metadataJSON, err := marshalJSON(input.Metadata)
	if err != nil {
		return nil, fmt.Errorf("encode metadata: %w", err)
	}
	accessMode := normalizeAccessMode(input.AccessMode)
	accessSecret := strings.TrimSpace(input.AccessPassword)
	if accessMode != AccessModeNone && accessSecret == "" {
		accessSecret, err = randomToken(12)
		if err != nil {
			return nil, fmt.Errorf("generate access password: %w", err)
		}
	}
	accessHash := ""
	if accessMode != AccessModeNone {
		accessHash = hashAccessSecret(accessSecret)
	}

	builder := m.client.ToolSession.Create().
		SetOwner(strings.TrimSpace(input.Owner)).
		SetSource(normalizeSource(input.Source)).
		SetChannel(strings.TrimSpace(input.Channel)).
		SetConversationKey(strings.TrimSpace(input.ConversationKey)).
		SetTool(strings.TrimSpace(input.Tool)).
		SetTitle(strings.TrimSpace(input.Title)).
		SetCommand(strings.TrimSpace(input.Command)).
		SetWorkdir(strings.TrimSpace(input.Workdir)).
		SetState(state).
		SetAccessMode(accessMode).
		SetPinned(input.Pinned).
		SetLastActiveAt(now)

	if metadataJSON != "" {
		builder.SetMetadataJSON(metadataJSON)
	}
	if accessHash != "" {
		builder.SetAccessSecretHash(accessHash)
	}
	if input.ExpiresAt != nil {
		builder.SetExpiresAt(*input.ExpiresAt)
	}
	if state == StateDetached {
		builder.SetDetachedAt(now)
	}
	if state == StateTerminated || state == StateArchived {
		builder.SetDetachedAt(now)
		builder.SetTerminatedAt(now)
	}

	rec, err := builder.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	if err := m.appendEvent(ctx, rec.ID, "created", map[string]interface{}{"state": rec.State}); err != nil {
		m.log.Warn("Failed to record tool session event", zap.String("session_id", rec.ID), zap.Error(err))
	}
	return toSession(rec), nil
}

// GetSession returns a session by ID.
func (m *Manager) GetSession(ctx context.Context, id string) (*Session, error) {
	rec, err := m.client.ToolSession.Get(ctx, strings.TrimSpace(id))
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, os.ErrNotExist
		}
		return nil, fmt.Errorf("get session: %w", err)
	}
	return toSession(rec), nil
}

// ListSessions returns sessions in reverse creation order.
func (m *Manager) ListSessions(ctx context.Context, input ListSessionsInput) ([]*Session, error) {
	q := m.client.ToolSession.Query()
	if owner := strings.TrimSpace(input.Owner); owner != "" {
		q = q.Where(toolsession.OwnerEQ(owner))
	}
	if source := normalizeSource(input.Source); source != "" {
		q = q.Where(toolsession.SourceEQ(source))
	}
	if state := normalizeState(input.State); state != "" {
		q = q.Where(toolsession.StateEQ(state))
	}
	q = q.Order(ent.Desc(toolsession.FieldCreatedAt))
	if input.Limit > 0 {
		q = q.Limit(input.Limit)
	} else {
		q = q.Limit(200)
	}
	recs, err := q.All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	out := make([]*Session, 0, len(recs))
	for _, rec := range recs {
		out = append(out, toSession(rec))
	}
	return out, nil
}

// TouchSession updates activity timestamp and optionally sets a state.
func (m *Manager) TouchSession(ctx context.Context, id, state string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("session id is required")
	}
	now := time.Now()
	up := m.client.ToolSession.UpdateOneID(id).SetLastActiveAt(now)
	if normalized := normalizeState(state); normalized != "" {
		up = up.SetState(normalized)
		if normalized == StateRunning {
			up = up.ClearDetachedAt()
		}
	}
	if err := up.Exec(ctx); err != nil {
		if ent.IsNotFound(err) {
			return os.ErrNotExist
		}
		return fmt.Errorf("touch session: %w", err)
	}
	return nil
}

// DetachSession transitions a running session to detached.
func (m *Manager) DetachSession(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("session id is required")
	}
	now := time.Now()
	err := m.client.ToolSession.UpdateOneID(id).
		SetState(StateDetached).
		SetDetachedAt(now).
		SetLastActiveAt(now).
		Exec(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return os.ErrNotExist
		}
		return fmt.Errorf("detach session: %w", err)
	}
	if err := m.appendEvent(ctx, id, "detached", nil); err != nil {
		m.log.Warn("Failed to record tool session event", zap.String("session_id", id), zap.Error(err))
	}
	return nil
}

// TerminateSession marks a session as terminated.
func (m *Manager) TerminateSession(ctx context.Context, id, reason string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("session id is required")
	}
	now := time.Now()
	err := m.client.ToolSession.UpdateOneID(id).
		SetState(StateTerminated).
		SetTerminatedAt(now).
		SetDetachedAt(now).
		SetLastActiveAt(now).
		Exec(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return os.ErrNotExist
		}
		return fmt.Errorf("terminate session: %w", err)
	}
	payload := map[string]interface{}{}
	if strings.TrimSpace(reason) != "" {
		payload["reason"] = strings.TrimSpace(reason)
	}
	if err := m.appendEvent(ctx, id, "terminated", payload); err != nil {
		m.log.Warn("Failed to record tool session event", zap.String("session_id", id), zap.Error(err))
	}
	m.clearSessionOTP(id)
	return nil
}

// UpdateSessionLaunch updates session metadata and sets it back to running state.
func (m *Manager) UpdateSessionLaunch(ctx context.Context, id, tool, title, command, workdir string) (*Session, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errors.New("session id is required")
	}
	now := time.Now()
	err := m.client.ToolSession.UpdateOneID(id).
		SetTool(strings.TrimSpace(tool)).
		SetTitle(strings.TrimSpace(title)).
		SetCommand(strings.TrimSpace(command)).
		SetWorkdir(strings.TrimSpace(workdir)).
		SetState(StateRunning).
		SetLastActiveAt(now).
		ClearDetachedAt().
		ClearTerminatedAt().
		Exec(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, os.ErrNotExist
		}
		return nil, fmt.Errorf("update session launch: %w", err)
	}
	return m.GetSession(ctx, id)
}

// UpdateSessionConfig updates editable fields without changing runtime state.
func (m *Manager) UpdateSessionConfig(ctx context.Context, id, tool, title, command, workdir string) (*Session, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errors.New("session id is required")
	}
	up := m.client.ToolSession.UpdateOneID(id)
	if strings.TrimSpace(tool) != "" {
		up = up.SetTool(strings.TrimSpace(tool))
	}
	up = up.
		SetTitle(strings.TrimSpace(title)).
		SetCommand(strings.TrimSpace(command)).
		SetWorkdir(strings.TrimSpace(workdir))
	if err := up.Exec(ctx); err != nil {
		if ent.IsNotFound(err) {
			return nil, os.ErrNotExist
		}
		return nil, fmt.Errorf("update session config: %w", err)
	}
	return m.GetSession(ctx, id)
}

// UpdateSessionMetadata replaces session metadata payload.
func (m *Manager) UpdateSessionMetadata(ctx context.Context, id string, metadata map[string]interface{}) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("session id is required")
	}
	metadataJSON, err := marshalJSON(metadata)
	if err != nil {
		return fmt.Errorf("encode metadata: %w", err)
	}
	up := m.client.ToolSession.UpdateOneID(id)
	if metadataJSON == "" {
		up = up.ClearMetadataJSON()
	} else {
		up = up.SetMetadataJSON(metadataJSON)
	}
	if err := up.Exec(ctx); err != nil {
		if ent.IsNotFound(err) {
			return os.ErrNotExist
		}
		return fmt.Errorf("update session metadata: %w", err)
	}
	return nil
}

// ArchiveTerminatedSessions archives terminated sessions, optionally scoped by owner.
func (m *Manager) ArchiveTerminatedSessions(ctx context.Context, owner string) (int, error) {
	q := m.client.ToolSession.Update().
		Where(toolsession.StateEQ(StateTerminated)).
		SetState(StateArchived)
	if owner = strings.TrimSpace(owner); owner != "" {
		q = q.Where(toolsession.OwnerEQ(owner))
	}
	affected, err := q.Save(ctx)
	if err != nil {
		return 0, fmt.Errorf("archive terminated sessions: %w", err)
	}
	return affected, nil
}

// ConfigureSessionAccess sets one-time/permanent access password policy for a session.
// It returns the effective plain password (generated when empty for non-none modes).
func (m *Manager) ConfigureSessionAccess(ctx context.Context, id, mode, password string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", errors.New("session id is required")
	}
	accessMode := normalizeAccessMode(mode)
	secret := strings.TrimSpace(password)
	if accessMode != AccessModeNone && secret == "" {
		var err error
		secret, err = randomToken(12)
		if err != nil {
			return "", fmt.Errorf("generate access password: %w", err)
		}
	}

	up := m.client.ToolSession.UpdateOneID(id).
		SetAccessMode(accessMode).
		ClearAccessOnceUsedAt()
	if accessMode == AccessModeNone {
		up = up.SetAccessSecretHash("")
	} else {
		up = up.SetAccessSecretHash(hashAccessSecret(secret))
	}
	if err := up.Exec(ctx); err != nil {
		if ent.IsNotFound(err) {
			return "", os.ErrNotExist
		}
		return "", fmt.Errorf("configure session access: %w", err)
	}
	if err := m.appendEvent(ctx, id, "access_updated", map[string]interface{}{"mode": accessMode}); err != nil {
		m.log.Warn("Failed to record tool session event", zap.String("session_id", id), zap.Error(err))
	}
	m.clearSessionOTP(id)
	return secret, nil
}

// GenerateSessionOTP creates a short-lived one-time 6-digit code for access-login.
func (m *Manager) GenerateSessionOTP(ctx context.Context, id string, ttl time.Duration) (string, time.Time, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", time.Time{}, errors.New("session id is required")
	}
	if ttl <= 0 {
		ttl = m.otpTTL
	}
	if ttl <= 0 {
		ttl = defaultOTPTTL
	}
	ttl = normalizeOTPTTLDuration(ttl)

	rec, err := m.client.ToolSession.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return "", time.Time{}, os.ErrNotExist
		}
		return "", time.Time{}, fmt.Errorf("load session for otp: %w", err)
	}
	if normalizeAccessMode(rec.AccessMode) == AccessModeNone {
		return "", time.Time{}, os.ErrPermission
	}

	code, err := randomDigits(6)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("generate otp: %w", err)
	}
	expiresAt := time.Now().Add(ttl)
	m.storeSessionOTP(id, hashAccessSecret(code), expiresAt)
	_ = m.appendEvent(ctx, id, "otp_issued", map[string]interface{}{
		"expires_at": expiresAt.Unix(),
	})
	return code, expiresAt, nil
}

// VerifySessionAccess checks an access password and consumes one-time secrets.
func (m *Manager) VerifySessionAccess(ctx context.Context, id, password string) (*Session, error) {
	id = strings.TrimSpace(id)
	secret := strings.TrimSpace(password)
	if id == "" || secret == "" {
		return nil, os.ErrPermission
	}

	rec, err := m.client.ToolSession.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, os.ErrNotExist
		}
		return nil, fmt.Errorf("get session for access verification: %w", err)
	}

	mode := normalizeAccessMode(rec.AccessMode)
	permanentOK := false
	if mode != AccessModeNone && strings.TrimSpace(rec.AccessSecretHash) != "" {
		if !(mode == AccessModeOneTime && rec.AccessOnceUsedAt != nil) && compareAccessSecret(rec.AccessSecretHash, secret) {
			permanentOK = true
		}
	}
	otpOK := m.consumeSessionOTPIfMatch(rec.ID, secret)
	if !permanentOK && !otpOK {
		return nil, os.ErrPermission
	}

	now := time.Now()
	up := m.client.ToolSession.UpdateOneID(rec.ID).SetLastActiveAt(now)
	if permanentOK && mode == AccessModeOneTime {
		up = up.SetAccessOnceUsedAt(now).SetAccessSecretHash("").SetAccessMode(AccessModeNone)
	}
	if err := up.Exec(ctx); err != nil {
		if permanentOK && mode == AccessModeOneTime {
			return nil, fmt.Errorf("consume one-time access password: %w", err)
		}
		return nil, fmt.Errorf("update session access timestamp: %w", err)
	}

	return m.GetSession(ctx, rec.ID)
}

// CreateAttachToken creates a short-lived one-time attach token.
func (m *Manager) CreateAttachToken(ctx context.Context, sessionID, owner string, ttl time.Duration) (string, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return "", errors.New("session id is required")
	}
	if ttl <= 0 {
		ttl = time.Minute
	}
	if _, err := m.client.ToolSession.Get(ctx, sessionID); err != nil {
		if ent.IsNotFound(err) {
			return "", os.ErrNotExist
		}
		return "", fmt.Errorf("load session: %w", err)
	}

	token, err := randomToken(24)
	if err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	_, err = m.client.AttachToken.Create().
		SetToken(token).
		SetSessionID(sessionID).
		SetOwner(strings.TrimSpace(owner)).
		SetExpiresAt(time.Now().Add(ttl)).
		Save(ctx)
	if err != nil {
		return "", fmt.Errorf("create attach token: %w", err)
	}
	return token, nil
}

// ConsumeAttachToken validates and consumes one attach token.
func (m *Manager) ConsumeAttachToken(ctx context.Context, token, owner string) (*Session, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, errors.New("token is required")
	}
	now := time.Now()
	rec, err := m.client.AttachToken.Query().
		Where(attachtoken.TokenEQ(token)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, os.ErrNotExist
		}
		return nil, fmt.Errorf("load attach token: %w", err)
	}
	if rec.UsedAt != nil {
		return nil, os.ErrPermission
	}
	if now.After(rec.ExpiresAt) {
		return nil, os.ErrDeadlineExceeded
	}
	if strings.TrimSpace(rec.Owner) != "" && strings.TrimSpace(rec.Owner) != strings.TrimSpace(owner) {
		return nil, os.ErrPermission
	}

	if err := m.client.AttachToken.UpdateOneID(rec.ID).SetUsedAt(now).Exec(ctx); err != nil {
		return nil, fmt.Errorf("consume attach token: %w", err)
	}

	sess, err := m.client.ToolSession.Get(ctx, rec.SessionID)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, os.ErrNotExist
		}
		return nil, fmt.Errorf("load session from token: %w", err)
	}
	if err := m.TouchSession(ctx, sess.ID, StateRunning); err != nil {
		m.log.Warn("Failed to touch session after attach token consumption", zap.String("session_id", sess.ID), zap.Error(err))
	}
	return toSession(sess), nil
}

// AppendEvent writes a session timeline event.
func (m *Manager) AppendEvent(ctx context.Context, sessionID, eventType string, payload map[string]interface{}) error {
	return m.appendEvent(ctx, sessionID, eventType, payload)
}

func (m *Manager) appendEvent(ctx context.Context, sessionID, eventType string, payload map[string]interface{}) error {
	sessionID = strings.TrimSpace(sessionID)
	eventType = strings.TrimSpace(eventType)
	if sessionID == "" || eventType == "" {
		return nil
	}
	payloadJSON, err := marshalJSON(payload)
	if err != nil {
		return err
	}
	_, err = m.client.ToolEvent.Create().
		SetSessionID(sessionID).
		SetEventType(eventType).
		SetPayloadJSON(payloadJSON).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("append event: %w", err)
	}
	return nil
}

// ListEvents returns most recent events for a session.
func (m *Manager) ListEvents(ctx context.Context, sessionID string, limit int) ([]*Event, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, errors.New("session id is required")
	}
	q := m.client.ToolEvent.Query().
		Where(toolevent.SessionIDEQ(sessionID)).
		Order(ent.Desc(toolevent.FieldCreatedAt))
	if limit > 0 {
		q = q.Limit(limit)
	} else {
		q = q.Limit(100)
	}
	recs, err := q.All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	out := make([]*Event, 0, len(recs))
	for _, rec := range recs {
		out = append(out, &Event{
			ID:        rec.ID,
			SessionID: rec.SessionID,
			Type:      rec.EventType,
			Payload:   parseJSON(rec.PayloadJSON),
			CreatedAt: rec.CreatedAt,
		})
	}
	return out, nil
}

// Cleanup transitions session lifecycle states according to policy.
func (m *Manager) Cleanup(ctx context.Context) (GCResult, error) {
	cfg := m.lifecycle
	now := time.Now()
	result := GCResult{}

	// 1) running -> detached when idle for too long.
	if cfg.RunningIdleTimeout > 0 {
		idleCutoff := now.Add(-cfg.RunningIdleTimeout)
		affected, err := m.client.ToolSession.Update().
			Where(
				toolsession.StateEQ(StateRunning),
				toolsession.LastActiveAtLT(idleCutoff),
			).
			SetState(StateDetached).
			SetDetachedAt(now).
			Save(ctx)
		if err != nil {
			return result, fmt.Errorf("cleanup running->detached: %w", err)
		}
		result.DetachedByIdle = affected
	}

	// 2) detached -> terminated by detached TTL (except pinned sessions).
	if cfg.DetachedTTL > 0 {
		detachedCutoff := now.Add(-cfg.DetachedTTL)
		affected, err := m.client.ToolSession.Update().
			Where(
				toolsession.StateEQ(StateDetached),
				toolsession.PinnedEQ(false),
				toolsession.Or(
					toolsession.And(
						toolsession.DetachedAtIsNil(),
						toolsession.LastActiveAtLT(detachedCutoff),
					),
					toolsession.DetachedAtLT(detachedCutoff),
				),
			).
			SetState(StateTerminated).
			SetTerminatedAt(now).
			Save(ctx)
		if err != nil {
			return result, fmt.Errorf("cleanup detached->terminated: %w", err)
		}
		result.TerminatedByTTL = affected
	}

	// 3) hard lifetime cap: running/detached sessions become terminated.
	if cfg.MaxLifetime > 0 {
		lifeCutoff := now.Add(-cfg.MaxLifetime)
		affected, err := m.client.ToolSession.Update().
			Where(
				toolsession.StateIn(StateRunning, StateDetached),
				toolsession.CreatedAtLT(lifeCutoff),
			).
			SetState(StateTerminated).
			SetTerminatedAt(now).
			SetDetachedAt(now).
			Save(ctx)
		if err != nil {
			return result, fmt.Errorf("cleanup lifetime termination: %w", err)
		}
		result.TerminatedByLife = affected
	}

	// 4) terminated -> archived after retention period.
	if cfg.TerminatedRetention > 0 {
		archiveCutoff := now.Add(-cfg.TerminatedRetention)
		affected, err := m.client.ToolSession.Update().
			Where(
				toolsession.StateEQ(StateTerminated),
				toolsession.Or(
					toolsession.And(
						toolsession.TerminatedAtIsNil(),
						toolsession.UpdatedAtLT(archiveCutoff),
					),
					toolsession.TerminatedAtLT(archiveCutoff),
				),
			).
			SetState(StateArchived).
			Save(ctx)
		if err != nil {
			return result, fmt.Errorf("cleanup terminated->archived: %w", err)
		}
		result.ArchivedOld = affected
	}

	return result, nil
}

func toSession(rec *ent.ToolSession) *Session {
	if rec == nil {
		return nil
	}
	return &Session{
		ID:               rec.ID,
		Owner:            rec.Owner,
		Source:           rec.Source,
		Channel:          rec.Channel,
		ConversationKey:  rec.ConversationKey,
		Tool:             rec.Tool,
		Title:            rec.Title,
		Command:          rec.Command,
		Workdir:          rec.Workdir,
		State:            rec.State,
		AccessMode:       rec.AccessMode,
		AccessOnceUsedAt: rec.AccessOnceUsedAt,
		Pinned:           rec.Pinned,
		LastActiveAt:     rec.LastActiveAt,
		DetachedAt:       rec.DetachedAt,
		TerminatedAt:     rec.TerminatedAt,
		ExpiresAt:        rec.ExpiresAt,
		Metadata:         parseJSON(rec.MetadataJSON),
		CreatedAt:        rec.CreatedAt,
		UpdatedAt:        rec.UpdatedAt,
	}
}

func normalizeState(state string) string {
	switch strings.TrimSpace(strings.ToLower(state)) {
	case StateRunning:
		return StateRunning
	case StateDetached:
		return StateDetached
	case StateTerminated:
		return StateTerminated
	case StateArchived:
		return StateArchived
	default:
		return ""
	}
}

func normalizeSource(source string) string {
	switch strings.TrimSpace(strings.ToLower(source)) {
	case SourceChannel:
		return SourceChannel
	case SourceWebUI:
		return SourceWebUI
	case "":
		return SourceWebUI
	default:
		return strings.TrimSpace(strings.ToLower(source))
	}
}

func normalizeAccessMode(mode string) string {
	switch strings.TrimSpace(strings.ToLower(mode)) {
	case AccessModeOneTime:
		return AccessModeOneTime
	case AccessModePermanent:
		return AccessModePermanent
	default:
		return AccessModeNone
	}
}

func hashAccessSecret(secret string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(secret)))
	return hex.EncodeToString(sum[:])
}

func compareAccessSecret(hashed, secret string) bool {
	left := strings.TrimSpace(strings.ToLower(hashed))
	right := hashAccessSecret(secret)
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

func (m *Manager) storeSessionOTP(sessionID, hash string, expiresAt time.Time) {
	m.otpMu.Lock()
	m.otpCodes[sessionID] = sessionOTP{hash: hash, expiresAt: expiresAt}
	m.otpMu.Unlock()
}

func (m *Manager) clearSessionOTP(sessionID string) {
	m.otpMu.Lock()
	delete(m.otpCodes, strings.TrimSpace(sessionID))
	m.otpMu.Unlock()
}

func (m *Manager) consumeSessionOTPIfMatch(sessionID, secret string) bool {
	id := strings.TrimSpace(sessionID)
	if id == "" || strings.TrimSpace(secret) == "" {
		return false
	}
	now := time.Now()
	m.otpMu.Lock()
	defer m.otpMu.Unlock()
	entry, ok := m.otpCodes[id]
	if !ok {
		return false
	}
	if now.After(entry.expiresAt) {
		delete(m.otpCodes, id)
		return false
	}
	if !compareAccessSecret(entry.hash, secret) {
		return false
	}
	delete(m.otpCodes, id)
	return true
}

func marshalJSON(v map[string]interface{}) (string, error) {
	if len(v) == 0 {
		return "", nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func parseJSON(raw string) map[string]interface{} {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	out := map[string]interface{}{}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

func randomToken(bytesLen int) (string, error) {
	if bytesLen < 16 {
		bytesLen = 16
	}
	buf := make([]byte, bytesLen)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func randomDigits(length int) (string, error) {
	if length <= 0 {
		length = 6
	}
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	out := make([]byte, length)
	for i := 0; i < length; i++ {
		out[i] = byte('0' + (buf[i] % 10))
	}
	return string(out), nil
}

func normalizeOTPTTLSeconds(seconds int) time.Duration {
	if seconds <= 0 {
		seconds = defaultOTPTTLSeconds
	}
	if seconds < minOTPTTLSeconds {
		seconds = minOTPTTLSeconds
	}
	if seconds > maxOTPTTLSeconds {
		seconds = maxOTPTTLSeconds
	}
	return time.Duration(seconds) * time.Second
}

func normalizeOTPTTLDuration(ttl time.Duration) time.Duration {
	if ttl <= 0 {
		return normalizeOTPTTLSeconds(defaultOTPTTLSeconds)
	}
	seconds := int(ttl.Round(time.Second) / time.Second)
	return normalizeOTPTTLSeconds(seconds)
}
