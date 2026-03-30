package accountbindings

import "time"

const (
	// ModeSingleAgent restricts one account to one active public runtime.
	ModeSingleAgent = "single_agent"
	// ModeMultiAgent allows one account to fan out to multiple runtimes.
	ModeMultiAgent = "multi_agent"
)

// AccountBinding defines one account-to-runtime contract.
type AccountBinding struct {
	ID               string                 `json:"id"`
	ChannelAccountID string                 `json:"channel_account_id"`
	AgentRuntimeID   string                 `json:"agent_runtime_id"`
	BindingMode      string                 `json:"binding_mode"`
	Enabled          bool                   `json:"enabled"`
	AllowPublicReply bool                   `json:"allow_public_reply"`
	ReplyLabel       string                 `json:"reply_label"`
	Priority         int                    `json:"priority"`
	Metadata         map[string]interface{} `json:"metadata"`
	CreatedAt        time.Time              `json:"created_at"`
	UpdatedAt        time.Time              `json:"updated_at"`
}
