package acpstate

import "strings"

const MetadataKey = "acp_sessions"

// SessionMap extracts persisted ACP conversation-to-session mappings from metadata.
func SessionMap(metadata map[string]interface{}) map[string]string {
	if len(metadata) == 0 {
		return map[string]string{}
	}
	raw, ok := metadata[MetadataKey]
	if !ok || raw == nil {
		return map[string]string{}
	}

	result := map[string]string{}
	switch typed := raw.(type) {
	case map[string]string:
		for conversationID, sessionID := range typed {
			conversationID = strings.TrimSpace(conversationID)
			sessionID = strings.TrimSpace(sessionID)
			if conversationID == "" || sessionID == "" {
				continue
			}
			result[conversationID] = sessionID
		}
	case map[string]interface{}:
		for conversationID, value := range typed {
			conversationID = strings.TrimSpace(conversationID)
			sessionID, _ := value.(string)
			sessionID = strings.TrimSpace(sessionID)
			if conversationID == "" || sessionID == "" {
				continue
			}
			result[conversationID] = sessionID
		}
	}
	return result
}

// SetConversationSession returns a copied metadata map with the ACP session mapping updated.
func SetConversationSession(metadata map[string]interface{}, conversationID, sessionID string) map[string]interface{} {
	conversationID = strings.TrimSpace(conversationID)
	sessionID = strings.TrimSpace(sessionID)
	cloned := cloneMetadata(metadata)
	if conversationID == "" || sessionID == "" {
		return cloned
	}

	sessions := SessionMap(metadata)
	sessions[conversationID] = sessionID
	stored := make(map[string]interface{}, len(sessions))
	for key, value := range sessions {
		stored[key] = value
	}
	cloned[MetadataKey] = stored
	return cloned
}

func cloneMetadata(metadata map[string]interface{}) map[string]interface{} {
	if len(metadata) == 0 {
		return map[string]interface{}{}
	}
	cloned := make(map[string]interface{}, len(metadata))
	for key, value := range metadata {
		cloned[key] = value
	}
	return cloned
}
