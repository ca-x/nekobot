package toolsessions

import "time"

const (
	StateRunning    = "running"
	StateDetached   = "detached"
	StateTerminated = "terminated"
	StateArchived   = "archived"
)

const (
	SourceWebUI   = "webui"
	SourceChannel = "channel"
	SourceAgent   = "agent"
)

const (
	AccessModeNone      = "none"
	AccessModeOneTime   = "one_time"
	AccessModePermanent = "permanent"
)

// Session is the public model used by API and business logic.
type Session struct {
	ID               string                 `json:"id"`
	Owner            string                 `json:"owner"`
	Source           string                 `json:"source"`
	Channel          string                 `json:"channel,omitempty"`
	ConversationKey  string                 `json:"conversation_key,omitempty"`
	Tool             string                 `json:"tool"`
	Title            string                 `json:"title,omitempty"`
	Command          string                 `json:"command,omitempty"`
	Workdir          string                 `json:"workdir,omitempty"`
	State            string                 `json:"state"`
	AccessMode       string                 `json:"access_mode"`
	AccessOnceUsedAt *time.Time             `json:"access_once_used_at,omitempty"`
	Pinned           bool                   `json:"pinned"`
	LastActiveAt     time.Time              `json:"last_active_at"`
	DetachedAt       *time.Time             `json:"detached_at,omitempty"`
	TerminatedAt     *time.Time             `json:"terminated_at,omitempty"`
	ExpiresAt        *time.Time             `json:"expires_at,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt        time.Time              `json:"created_at"`
	UpdatedAt        time.Time              `json:"updated_at"`
}

// Event stores timeline entries for session state changes.
type Event struct {
	ID        string                 `json:"id"`
	SessionID string                 `json:"session_id"`
	Type      string                 `json:"type"`
	Payload   map[string]interface{} `json:"payload,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
}

// CreateSessionInput is used to create a new tool session.
type CreateSessionInput struct {
	Owner           string                 `json:"owner"`
	Source          string                 `json:"source"`
	Channel         string                 `json:"channel"`
	ConversationKey string                 `json:"conversation_key"`
	Tool            string                 `json:"tool"`
	Title           string                 `json:"title"`
	Command         string                 `json:"command"`
	Workdir         string                 `json:"workdir"`
	State           string                 `json:"state"`
	AccessMode      string                 `json:"access_mode"`
	AccessPassword  string                 `json:"access_password"`
	Pinned          bool                   `json:"pinned"`
	ExpiresAt       *time.Time             `json:"expires_at"`
	Metadata        map[string]interface{} `json:"metadata"`
}

// ListSessionsInput controls filtering and pagination.
type ListSessionsInput struct {
	Owner  string
	Source string
	State  string
	Limit  int
}

// LifecycleConfig controls automatic state transitions and cleanup.
type LifecycleConfig struct {
	SweepInterval       time.Duration
	RunningIdleTimeout  time.Duration
	DetachedTTL         time.Duration
	MaxLifetime         time.Duration
	TerminatedRetention time.Duration
}

// GCResult reports how many rows were transitioned by one cleanup cycle.
type GCResult struct {
	DetachedByIdle   int `json:"detached_by_idle"`
	TerminatedByTTL  int `json:"terminated_by_ttl"`
	TerminatedByLife int `json:"terminated_by_lifetime"`
	ArchivedOld      int `json:"archived_old"`
}

func defaultLifecycleConfig() LifecycleConfig {
	return LifecycleConfig{
		SweepInterval:       time.Minute,
		RunningIdleTimeout:  2 * time.Hour,
		DetachedTTL:         24 * time.Hour,
		MaxLifetime:         7 * 24 * time.Hour,
		TerminatedRetention: 48 * time.Hour,
	}
}
