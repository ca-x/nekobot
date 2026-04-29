package channelaccounts

import "time"

// ChannelAccount defines one channel-agnostic account endpoint.
type ChannelAccount struct {
	ID          string                 `json:"id"`
	ChannelType string                 `json:"channel_type"`
	AccountKey  string                 `json:"account_key"`
	DisplayName string                 `json:"display_name"`
	Description string                 `json:"description"`
	Enabled     bool                   `json:"enabled"`
	Config      map[string]interface{} `json:"config"`
	Metadata    map[string]interface{} `json:"metadata"`
	TenantID    string                 `json:"tenant_id"`
	OwnerUserID string                 `json:"owner_user_id"`
	Visibility  string                 `json:"visibility"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}
