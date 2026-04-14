package approval

import (
	"fmt"
	"sync"

	"nekobot/pkg/providers"
)

// PendingToolCall stores resumable ordinary tool-call context for approve-then-retry flows.
type PendingToolCall struct {
	SessionID string
	Call      providers.UnifiedToolCall
}

var (
	pendingToolCallsMu sync.RWMutex
	pendingToolCalls   = map[string]PendingToolCall{}
)

// RememberPendingToolCall stores pending tool-call context for an approval request id.
func RememberPendingToolCall(requestID, sessionID string, call providers.UnifiedToolCall) error {
	if requestID == "" {
		return fmt.Errorf("request id is required")
	}
	pendingToolCallsMu.Lock()
	defer pendingToolCallsMu.Unlock()
	pendingToolCalls[requestID] = PendingToolCall{SessionID: sessionID, Call: call}
	return nil
}

// PendingToolCallForRequest returns stored pending tool-call context.
func PendingToolCallForRequest(requestID string) (PendingToolCall, bool) {
	pendingToolCallsMu.RLock()
	defer pendingToolCallsMu.RUnlock()
	call, ok := pendingToolCalls[requestID]
	return call, ok
}

// ClearPendingToolCall removes stored tool-call context for an approval request id.
func ClearPendingToolCall(requestID string) {
	pendingToolCallsMu.Lock()
	defer pendingToolCallsMu.Unlock()
	delete(pendingToolCalls, requestID)
}
