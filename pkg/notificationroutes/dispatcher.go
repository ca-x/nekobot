package notificationroutes

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"nekobot/pkg/bus"
	"nekobot/pkg/channelaccounts"
	"nekobot/pkg/cron"
	"nekobot/pkg/logger"
)

// Dispatcher delivers notification events to configured channel accounts.
type Dispatcher struct {
	log      *logger.Logger
	routes   *Manager
	accounts *channelaccounts.Manager
	bus      bus.Bus
}

// NewDispatcher creates a notification dispatcher.
func NewDispatcher(log *logger.Logger, routes *Manager, accounts *channelaccounts.Manager, messageBus bus.Bus) *Dispatcher {
	return &Dispatcher{
		log:      log,
		routes:   routes,
		accounts: accounts,
		bus:      messageBus,
	}
}

// HandleCronJobEvent delivers cron completion events through the binding model.
func (d *Dispatcher) HandleCronJobEvent(ctx context.Context, event cron.JobEvent) {
	if d == nil || d.routes == nil || d.accounts == nil || d.bus == nil {
		return
	}
	if strings.TrimSpace(event.Job.ID) == "" || strings.TrimSpace(event.EventType) == "" {
		return
	}

	dispatchCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	binding, err := d.routes.FindBindingForTarget(dispatchCtx, ScopeCronJob, event.Job.ID)
	if err != nil {
		d.warn("Failed to find cron notification binding", zap.String("job_id", event.Job.ID), zap.Error(err))
		return
	}
	if binding == nil || !binding.Enabled || !bindingMatchesEvent(binding.EventTypesJSON, event.EventType) {
		return
	}

	route, err := d.routes.GetRoute(dispatchCtx, binding.RouteID)
	if err != nil {
		d.warn("Failed to load cron notification route", zap.String("route_id", binding.RouteID), zap.Error(err))
		return
	}
	if route == nil || !route.Enabled {
		return
	}

	account, err := d.accounts.Get(dispatchCtx, route.ChannelAccountID)
	if err != nil {
		d.warn("Failed to load notification channel account",
			zap.String("route_id", route.ID),
			zap.String("channel_account_id", route.ChannelAccountID),
			zap.Error(err))
		return
	}
	if account == nil || !account.Enabled {
		return
	}

	target, err := parseTargetConfig(route.TargetConfigJSON)
	if err != nil {
		d.warn("Invalid notification route target config", zap.String("route_id", route.ID), zap.Error(err))
		return
	}

	msg := buildCronNotificationMessage(event, *route, *account, target)
	if err := d.bus.SendOutbound(msg); err != nil {
		d.warn("Failed to send cron notification",
			zap.String("job_id", event.Job.ID),
			zap.String("route_id", route.ID),
			zap.String("channel_id", msg.ChannelID),
			zap.Error(err))
		return
	}

	if event.DeleteAfterRun {
		if err := d.routes.DeleteBindingsForTarget(dispatchCtx, ScopeCronJob, event.Job.ID); err != nil {
			d.warn("Failed to delete one-shot cron notification binding", zap.String("job_id", event.Job.ID), zap.Error(err))
		}
	}
}

func (d *Dispatcher) warn(message string, fields ...zap.Field) {
	if d != nil && d.log != nil {
		d.log.Warn(message, fields...)
	}
}

type targetConfig struct {
	SessionID    string                 `json:"session_id"`
	Target       string                 `json:"target"`
	ChatID       string                 `json:"chat_id"`
	UserID       string                 `json:"user_id"`
	Username     string                 `json:"username"`
	ReplyTo      string                 `json:"reply_to"`
	ContextToken string                 `json:"context_token"`
	Title        string                 `json:"title"`
	Extra        map[string]interface{} `json:"-"`
}

func parseTargetConfig(raw string) (targetConfig, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = "{}"
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return targetConfig{}, fmt.Errorf("decode target config: %w", err)
	}
	cfg := targetConfig{Extra: map[string]interface{}{}}
	for key, value := range decoded {
		text, _ := value.(string)
		text = strings.TrimSpace(text)
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "session_id":
			cfg.SessionID = text
		case "target":
			cfg.Target = text
		case "chat_id":
			cfg.ChatID = text
		case "user_id":
			cfg.UserID = text
		case "username":
			cfg.Username = text
		case "reply_to":
			cfg.ReplyTo = text
		case "context_token":
			cfg.ContextToken = text
		case "title":
			cfg.Title = text
		default:
			cfg.Extra[key] = value
		}
	}
	return cfg, nil
}

func buildCronNotificationMessage(
	event cron.JobEvent,
	route NotificationRoute,
	account channelaccounts.ChannelAccount,
	target targetConfig,
) *bus.Message {
	channelType := strings.TrimSpace(strings.ToLower(account.ChannelType))
	channelID := channelaccounts.RuntimeChannelID(account)
	sessionID := notificationSessionID(channelType, target)
	content := cronNotificationContent(event)
	data := map[string]interface{}{
		"source":   "cron",
		"scope":    ScopeCronJob,
		"event":    event.EventType,
		"job_id":   event.Job.ID,
		"job_name": event.Job.Name,
		"route_id": route.ID,
		"title":    notificationTitle(event, target),
	}
	for key, value := range target.Extra {
		data[key] = value
	}
	if target.ContextToken != "" {
		data["context_token"] = target.ContextToken
	}
	return &bus.Message{
		ID:        "notification:" + uuid.NewString(),
		ChannelID: channelID,
		SessionID: sessionID,
		UserID:    firstNonEmpty(target.UserID, target.ChatID, target.Target),
		Username:  target.Username,
		Type:      bus.MessageTypeText,
		Content:   content,
		Data:      data,
		Timestamp: event.FinishedAt,
		ReplyTo:   target.ReplyTo,
	}
}

func notificationSessionID(channelType string, target targetConfig) string {
	if target.SessionID != "" {
		return target.SessionID
	}
	raw := firstNonEmpty(target.Target, target.ChatID, target.UserID)
	if raw == "" {
		return channelType + ":notification"
	}
	if strings.Contains(raw, ":") {
		return raw
	}
	if channelType == "" {
		return raw
	}
	return channelType + ":" + raw
}

func cronNotificationContent(event cron.JobEvent) string {
	status := "succeeded"
	if event.EventType == EventCronFailed || strings.TrimSpace(event.Error) != "" {
		status = "failed"
	}
	lines := []string{
		fmt.Sprintf("Scheduled job %q %s.", event.Job.Name, status),
		fmt.Sprintf("Job ID: %s", event.Job.ID),
	}
	if !event.FinishedAt.IsZero() {
		lines = append(lines, "Finished at: "+event.FinishedAt.Format(time.RFC3339))
	}
	if strings.TrimSpace(event.Error) != "" {
		lines = append(lines, "Error: "+strings.TrimSpace(event.Error))
	} else if strings.TrimSpace(event.Response) != "" {
		lines = append(lines, "Result: "+truncate(event.Response, 800))
	}
	return strings.Join(lines, "\n")
}

func notificationTitle(event cron.JobEvent, target targetConfig) string {
	if target.Title != "" {
		return target.Title
	}
	if event.EventType == EventCronFailed || strings.TrimSpace(event.Error) != "" {
		return "Nekobot schedule failed"
	}
	return "Nekobot schedule completed"
}

func bindingMatchesEvent(raw, eventType string) bool {
	eventType = strings.TrimSpace(eventType)
	if eventType == "" {
		return false
	}
	var events []string
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &events); err != nil {
		return false
	}
	for _, event := range events {
		normalized := strings.TrimSpace(event)
		if normalized == "*" || normalized == eventType {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
