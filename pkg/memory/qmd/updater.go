// Package qmd provides automatic update scheduler for QMD collections.
package qmd

import (
	"context"
	"time"

	"go.uber.org/zap"
	"nekobot/pkg/logger"
)

// Updater handles automatic updates of QMD collections.
type Updater struct {
	log      *logger.Logger
	manager  *Manager
	config   UpdateConfig
	stopChan chan struct{}
	interval time.Duration
}

// NewUpdater creates a new QMD updater.
func NewUpdater(log *logger.Logger, manager *Manager, config UpdateConfig) *Updater {
	// Parse interval
	interval := 30 * time.Minute // Default
	if config.Interval != "" {
		if d, err := time.ParseDuration(config.Interval); err == nil {
			interval = d
		}
	}

	return &Updater{
		log:      log,
		manager:  manager,
		config:   config,
		stopChan: make(chan struct{}),
		interval: interval,
	}
}

// Start starts the automatic update scheduler.
func (u *Updater) Start(ctx context.Context) error {
	if !u.manager.IsAvailable() {
		u.log.Debug("QMD not available, updater disabled")
		return nil
	}

	// Update on boot if configured
	if u.config.OnBoot {
		u.log.Info("Updating QMD collections on boot")
		if err := u.manager.UpdateAll(ctx); err != nil {
			u.log.Warn("Failed to update collections on boot", zap.Error(err))
		}
	}

	// Start scheduled updates
	go u.scheduleUpdates(ctx)

	u.log.Info("QMD updater started",
		zap.Duration("interval", u.interval))

	return nil
}

// Stop stops the updater.
func (u *Updater) Stop() error {
	close(u.stopChan)
	u.log.Info("QMD updater stopped")
	return nil
}

// scheduleUpdates runs periodic updates.
func (u *Updater) scheduleUpdates(ctx context.Context) {
	ticker := time.NewTicker(u.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			u.log.Debug("Running scheduled QMD update")
			if err := u.manager.UpdateAll(ctx); err != nil {
				u.log.Warn("Scheduled update failed", zap.Error(err))
			} else {
				u.log.Info("Scheduled QMD update completed")
			}

		case <-u.stopChan:
			return

		case <-ctx.Done():
			return
		}
	}
}

// TriggerUpdate manually triggers an update.
func (u *Updater) TriggerUpdate(ctx context.Context) error {
	u.log.Info("Manual QMD update triggered")
	return u.manager.UpdateAll(ctx)
}
