// Package gotify provides a Gotify push notification channel.
package gotify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.uber.org/zap"

	"nekobot/pkg/bus"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
)

const defaultRequestTimeout = 20 * time.Second

// Channel implements the channels.Channel interface for Gotify.
type Channel struct {
	log    *logger.Logger
	config config.GotifyConfig
	client *http.Client
}

// NewChannel creates a new Gotify channel.
func NewChannel(log *logger.Logger, cfg config.GotifyConfig) (*Channel, error) {
	serverURL := strings.TrimSpace(cfg.ServerURL)
	if serverURL == "" {
		return nil, fmt.Errorf("gotify server_url is required")
	}
	if strings.TrimSpace(cfg.AppToken) == "" {
		return nil, fmt.Errorf("gotify app_token is required")
	}
	if cfg.Priority < 1 || cfg.Priority > 10 {
		return nil, fmt.Errorf("gotify priority must be between 1 and 10")
	}

	return &Channel{
		log:    log,
		config: cfg,
		client: &http.Client{Timeout: defaultRequestTimeout},
	}, nil
}

// ID returns the stable channel identifier.
func (c *Channel) ID() string { return "gotify" }

// Name returns the human-readable channel name.
func (c *Channel) Name() string { return "Gotify" }

// IsEnabled reports whether the channel is enabled.
func (c *Channel) IsEnabled() bool { return c.config.Enabled }

// Start verifies that the Gotify server and token are reachable.
func (c *Channel) Start(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.currentApplicationURL(), nil)
	if err != nil {
		return fmt.Errorf("create gotify verify request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("verify gotify connection: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("gotify verify returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	c.log.Info("Gotify channel started", zap.String("server_url", strings.TrimSpace(c.config.ServerURL)))
	return nil
}

// Stop shuts down the channel.
func (c *Channel) Stop(ctx context.Context) error {
	return nil
}

// SendMessage sends a push notification to Gotify.
func (c *Channel) SendMessage(ctx context.Context, msg *bus.Message) error {
	payload := map[string]any{
		"title":    c.messageTitle(msg),
		"message":  msg.Content,
		"priority": c.config.Priority,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal gotify payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.messageURL(), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create gotify send request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("send gotify message: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("gotify send returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	return nil
}

func (c *Channel) messageTitle(msg *bus.Message) string {
	if msg == nil {
		return "Nekobot"
	}
	if title, ok := msg.Data["title"].(string); ok && strings.TrimSpace(title) != "" {
		return strings.TrimSpace(title)
	}
	if strings.TrimSpace(msg.Username) != "" {
		return msg.Username
	}
	if strings.TrimSpace(msg.ChannelID) != "" {
		return "Nekobot / " + strings.TrimSpace(msg.ChannelID)
	}
	return "Nekobot"
}

func (c *Channel) baseURL() string {
	return strings.TrimRight(strings.TrimSpace(c.config.ServerURL), "/")
}

func (c *Channel) currentApplicationURL() string {
	return fmt.Sprintf("%s/current/application?token=%s", c.baseURL(), url.QueryEscape(strings.TrimSpace(c.config.AppToken)))
}

func (c *Channel) messageURL() string {
	return fmt.Sprintf("%s/message?token=%s", c.baseURL(), url.QueryEscape(strings.TrimSpace(c.config.AppToken)))
}
