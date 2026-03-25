package voice

import "nekobot/pkg/wechat/types"

// Transcription returns the WeChat built-in voice-to-text transcription.
func Transcription(item *types.VoiceItem) string {
	if item != nil && item.Text != "" {
		return item.Text
	}
	return ""
}
