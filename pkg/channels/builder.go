package channels

import (
	"strings"

	"nekobot/pkg/agent"
	"nekobot/pkg/bus"
	"nekobot/pkg/commands"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/process"
	"nekobot/pkg/toolsessions"
	"nekobot/pkg/userprefs"
)

// BuildChannel creates a channel instance from the current config.
func BuildChannel(
	name string,
	log *logger.Logger,
	messageBus bus.Bus,
	ag *agent.Agent,
	cmdRegistry *commands.Registry,
	prefsMgr *userprefs.Manager,
	toolSessionMgr *toolsessions.Manager,
	processMgr *process.Manager,
	cfg *config.Config,
) (Channel, error) {
	descriptor, err := getChannelDescriptor(strings.ToLower(strings.TrimSpace(name)))
	if err != nil {
		return nil, err
	}
	return descriptor.build(log, messageBus, ag, cmdRegistry, prefsMgr, toolSessionMgr, processMgr, cfg)
}

// IsChannelEnabled checks whether a channel is enabled in config.
func IsChannelEnabled(name string, cfg *config.Config) (bool, error) {
	descriptor, err := getChannelDescriptor(strings.ToLower(strings.TrimSpace(name)))
	if err != nil {
		return false, err
	}
	return descriptor.enabled(cfg), nil
}
