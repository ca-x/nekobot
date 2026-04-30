package notificationroutes

import "time"

const (
	// ScopeCronJob binds notifications for a specific cron job ID.
	ScopeCronJob = "cron_job"

	// EventCronSucceeded fires when a cron job run succeeds.
	EventCronSucceeded = "cron.succeeded"
	// EventCronFailed fires when a cron job run fails.
	EventCronFailed = "cron.failed"
)

// NotificationRoute defines a named notification routing target (e.g. a channel account + config).
type NotificationRoute struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Description      string    `json:"description"`
	Enabled          bool      `json:"enabled"`
	ChannelAccountID string    `json:"channel_account_id"`
	TargetConfigJSON string    `json:"target_config_json"`
	TenantID         string    `json:"tenant_id"`
	OwnerUserID      string    `json:"owner_user_id"`
	Visibility       string    `json:"visibility"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// NotificationBinding connects event types in a given scope to a notification route.
type NotificationBinding struct {
	ID             string    `json:"id"`
	Scope          string    `json:"scope"`
	Target         string    `json:"target"`
	RouteID        string    `json:"route_id"`
	EventTypesJSON string    `json:"event_types_json"`
	Enabled        bool      `json:"enabled"`
	TenantID       string    `json:"tenant_id"`
	OwnerUserID    string    `json:"owner_user_id"`
	Visibility     string    `json:"visibility"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
