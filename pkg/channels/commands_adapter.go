package channels

import "nekobot/pkg/commands"

type commandChannelAdapter struct {
	manager *Manager
}

func newCommandChannelAdapter(manager *Manager) commands.ChannelManager {
	return &commandChannelAdapter{manager: manager}
}

func (a *commandChannelAdapter) GetEnabledChannels() []commands.Channel {
	enabled := a.manager.GetEnabledChannels()
	result := make([]commands.Channel, 0, len(enabled))
	for _, ch := range enabled {
		result = append(result, ch)
	}
	return result
}
