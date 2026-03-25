package monitor

import (
	"context"
	"time"

	"nekobot/pkg/wechat/types"
)

const (
	initialBackoff = 3 * time.Second
	maxBackoff     = 60 * time.Second
	errCodeExpired = -14
)

// Handler is called for each incoming message.
type Handler func(ctx context.Context, msg types.WeixinMessage)

// MonitorOption configures a Monitor.
type MonitorOption func(*Monitor)

// Monitor long-polls for new messages and dispatches them to a handler.
type Monitor struct {
	client    updatesClient
	handler   Handler
	syncState SyncState
	guard     *SessionGuard
}

type updatesClient interface {
	GetUpdates(ctx context.Context, buf string) (*types.GetUpdatesResponse, error)
}

// NewMonitor creates a Monitor for the given client and handler.
func NewMonitor(c updatesClient, handler Handler, opts ...MonitorOption) *Monitor {
	m := &Monitor{
		client:  c,
		handler: handler,
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// WithSyncState sets the sync state persistence backend.
func WithSyncState(s SyncState) MonitorOption {
	return func(m *Monitor) {
		m.syncState = s
	}
}

// WithSessionGuard sets the session guard for pause-on-expiry behavior.
func WithSessionGuard(g *SessionGuard) MonitorOption {
	return func(m *Monitor) {
		m.guard = g
	}
}

// Run starts the long-poll loop. Blocks until ctx is canceled.
func (m *Monitor) Run(ctx context.Context) error {
	var buf string
	backoff := initialBackoff

	if m.syncState != nil {
		if loaded, err := m.syncState.Load(); err == nil {
			buf = loaded
		}
	}

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		if m.guard != nil {
			if err := m.guard.Check(); err != nil {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(5 * time.Second):
				}
				continue
			}
		}

		resp, err := m.client.GetUpdates(ctx, buf)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
			backoff = min(backoff*2, maxBackoff)
			continue
		}

		backoff = initialBackoff

		if resp.ErrCode == errCodeExpired {
			if m.guard != nil {
				m.guard.Trigger()
			}
			buf = ""
			if m.syncState != nil {
				_ = m.syncState.Save("")
			}
			continue
		}

		if resp.Ret != 0 || resp.ErrCode != 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
			backoff = min(backoff*2, maxBackoff)
			continue
		}

		if resp.GetUpdatesBuf != "" {
			buf = resp.GetUpdatesBuf
		}

		if m.syncState != nil && buf != "" {
			_ = m.syncState.Save(buf)
		}

		for _, msg := range resp.Msgs {
			m.handler(ctx, msg)
		}
	}
}
