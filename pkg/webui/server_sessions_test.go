package webui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"

	"nekobot/pkg/agent"
	"nekobot/pkg/session"
)

func TestSessionHandlers_Return503WhenManagerUnavailable(t *testing.T) {
	s := &Server{}
	e := echo.New()

	tests := []struct {
		name       string
		handler    func(*echo.Context) error
		method     string
		target     string
		body       string
		path       string
		pathValues echo.PathValues
	}{
		{
			name:    "list",
			handler: s.handleListSessions,
			method:  http.MethodGet,
			target:  "/api/sessions",
		},
		{
			name:       "get",
			handler:    s.handleGetSession,
			method:     http.MethodGet,
			target:     "/api/sessions/s1",
			path:       "/api/sessions/:id",
			pathValues: echo.PathValues{{Name: "id", Value: "s1"}},
		},
		{
			name:       "update-summary",
			handler:    s.handleUpdateSessionSummary,
			method:     http.MethodPut,
			target:     "/api/sessions/s1/summary",
			body:       `{"summary":"x"}`,
			path:       "/api/sessions/:id/summary",
			pathValues: echo.PathValues{{Name: "id", Value: "s1"}},
		},
		{
			name:       "delete",
			handler:    s.handleDeleteSession,
			method:     http.MethodDelete,
			target:     "/api/sessions/s1",
			path:       "/api/sessions/:id",
			pathValues: echo.PathValues{{Name: "id", Value: "s1"}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var bodyReader *strings.Reader
			if tc.body != "" {
				bodyReader = strings.NewReader(tc.body)
			} else {
				bodyReader = strings.NewReader("")
			}

			req := httptest.NewRequest(tc.method, tc.target, bodyReader)
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			if tc.path != "" {
				c.SetPath(tc.path)
				c.SetPathValues(tc.pathValues)
			}

			if err := tc.handler(c); err != nil {
				t.Fatalf("handler failed: %v", err)
			}
			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
			}
			var payload map[string]string
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatalf("unmarshal response failed: %v", err)
			}
			if strings.TrimSpace(payload["error"]) == "" {
				t.Fatalf("expected non-empty error payload, got %s", rec.Body.String())
			}
		})
	}
}

func TestSessionHandlers_EndToEndFlow(t *testing.T) {
	sm := session.NewManager(t.TempDir())
	s := &Server{sessionMgr: sm}
	e := echo.New()

	const sessionID = "webui-e2e"
	sess, err := sm.Get(sessionID)
	if err != nil {
		t.Fatalf("create session failed: %v", err)
	}
	sess.AddMessage(agent.Message{Role: "user", Content: "hello"})
	sess.AddMessage(agent.Message{Role: "assistant", Content: "hi"})
	sess.AddMessage(agent.Message{Role: "tool", Content: "tool output", ToolCallID: "call-1"})
	sess.SetSummary("initial summary")
	if err := sm.Save(sess); err != nil {
		t.Fatalf("save session failed: %v", err)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/sessions", nil)
	listRec := httptest.NewRecorder()
	listCtx := e.NewContext(listReq, listRec)
	if err := s.handleListSessions(listCtx); err != nil {
		t.Fatalf("list handler failed: %v", err)
	}
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, listRec.Code)
	}

	var listed []map[string]json.RawMessage
	if err := json.Unmarshal(listRec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("unmarshal list response failed: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected 1 session, got %d", len(listed))
	}
	assertSessionSummaryShape(t, listed[0], sessionID, "initial summary", 3)

	getReq := httptest.NewRequest(http.MethodGet, "/api/sessions/"+sessionID, nil)
	getRec := httptest.NewRecorder()
	getCtx := e.NewContext(getReq, getRec)
	getCtx.SetPath("/api/sessions/:id")
	getCtx.SetPathValues(echo.PathValues{{Name: "id", Value: sessionID}})
	if err := s.handleGetSession(getCtx); err != nil {
		t.Fatalf("get handler failed: %v", err)
	}
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, getRec.Code)
	}

	var detail map[string]json.RawMessage
	if err := json.Unmarshal(getRec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("unmarshal detail response failed: %v", err)
	}
	assertSessionSummaryShape(t, detail, sessionID, "initial summary", 3)

	var messages []map[string]json.RawMessage
	if err := json.Unmarshal(detail["messages"], &messages); err != nil {
		t.Fatalf("unmarshal detail messages failed: %v", err)
	}
	if len(messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(messages))
	}
	assertMessageShape(t, messages[0], "user", "hello", "")
	assertMessageShape(t, messages[1], "assistant", "hi", "")
	assertMessageShape(t, messages[2], "tool", "tool output", "call-1")

	updateReq := httptest.NewRequest(http.MethodPut, "/api/sessions/"+sessionID+"/summary", strings.NewReader(`{"summary":"updated summary"}`))
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	updateCtx := e.NewContext(updateReq, updateRec)
	updateCtx.SetPath("/api/sessions/:id/summary")
	updateCtx.SetPathValues(echo.PathValues{{Name: "id", Value: sessionID}})
	if err := s.handleUpdateSessionSummary(updateCtx); err != nil {
		t.Fatalf("update handler failed: %v", err)
	}
	if updateRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, updateRec.Code)
	}
	assertStatusPayload(t, updateRec.Body.Bytes(), "updated")

	updated, err := sm.GetExisting(sessionID)
	if err != nil {
		t.Fatalf("get existing session after update failed: %v", err)
	}
	if got := updated.GetSummary(); got != "updated summary" {
		t.Fatalf("expected updated summary, got %q", got)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/sessions/"+sessionID, nil)
	deleteRec := httptest.NewRecorder()
	deleteCtx := e.NewContext(deleteReq, deleteRec)
	deleteCtx.SetPath("/api/sessions/:id")
	deleteCtx.SetPathValues(echo.PathValues{{Name: "id", Value: sessionID}})
	if err := s.handleDeleteSession(deleteCtx); err != nil {
		t.Fatalf("delete handler failed: %v", err)
	}
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, deleteRec.Code)
	}
	assertStatusPayload(t, deleteRec.Body.Bytes(), "deleted")

	if _, err := sm.GetExisting(sessionID); !os.IsNotExist(err) {
		t.Fatalf("expected deleted session to be missing, got err=%v", err)
	}
}

func TestSessionHandlers_NotFoundBehavior(t *testing.T) {
	sm := session.NewManager(t.TempDir())
	s := &Server{sessionMgr: sm}
	e := echo.New()

	const missingID = "missing-session"

	getReq := httptest.NewRequest(http.MethodGet, "/api/sessions/"+missingID, nil)
	getRec := httptest.NewRecorder()
	getCtx := e.NewContext(getReq, getRec)
	getCtx.SetPath("/api/sessions/:id")
	getCtx.SetPathValues(echo.PathValues{{Name: "id", Value: missingID}})
	if err := s.handleGetSession(getCtx); err != nil {
		t.Fatalf("get handler failed: %v", err)
	}
	if getRec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, getRec.Code)
	}
	assertErrorPayload(t, getRec.Body.Bytes())
	if _, err := sm.GetExisting(missingID); !os.IsNotExist(err) {
		t.Fatalf("missing session was created implicitly by get, err=%v", err)
	}

	updateReq := httptest.NewRequest(http.MethodPut, "/api/sessions/"+missingID+"/summary", strings.NewReader(`{"summary":"x"}`))
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	updateCtx := e.NewContext(updateReq, updateRec)
	updateCtx.SetPath("/api/sessions/:id/summary")
	updateCtx.SetPathValues(echo.PathValues{{Name: "id", Value: missingID}})
	if err := s.handleUpdateSessionSummary(updateCtx); err != nil {
		t.Fatalf("update handler failed: %v", err)
	}
	if updateRec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, updateRec.Code)
	}
	assertErrorPayload(t, updateRec.Body.Bytes())

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/sessions/"+missingID, nil)
	deleteRec := httptest.NewRecorder()
	deleteCtx := e.NewContext(deleteReq, deleteRec)
	deleteCtx.SetPath("/api/sessions/:id")
	deleteCtx.SetPathValues(echo.PathValues{{Name: "id", Value: missingID}})
	if err := s.handleDeleteSession(deleteCtx); err != nil {
		t.Fatalf("delete handler failed: %v", err)
	}
	if deleteRec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, deleteRec.Code)
	}
	assertErrorPayload(t, deleteRec.Body.Bytes())
}

func assertSessionSummaryShape(t *testing.T, payload map[string]json.RawMessage, wantID string, wantSummary string, wantCount int) {
	t.Helper()

	required := []string{"id", "created_at", "updated_at", "summary", "message_count"}
	for _, key := range required {
		if _, ok := payload[key]; !ok {
			t.Fatalf("expected key %q in payload", key)
		}
	}

	var gotID string
	if err := json.Unmarshal(payload["id"], &gotID); err != nil {
		t.Fatalf("unmarshal id failed: %v", err)
	}
	if gotID != wantID {
		t.Fatalf("expected id %q, got %q", wantID, gotID)
	}

	var gotSummary string
	if err := json.Unmarshal(payload["summary"], &gotSummary); err != nil {
		t.Fatalf("unmarshal summary failed: %v", err)
	}
	if gotSummary != wantSummary {
		t.Fatalf("expected summary %q, got %q", wantSummary, gotSummary)
	}

	var gotCount int
	if err := json.Unmarshal(payload["message_count"], &gotCount); err != nil {
		t.Fatalf("unmarshal message_count failed: %v", err)
	}
	if gotCount != wantCount {
		t.Fatalf("expected message_count %d, got %d", wantCount, gotCount)
	}
}

func assertMessageShape(t *testing.T, payload map[string]json.RawMessage, wantRole string, wantContent string, wantToolCallID string) {
	t.Helper()

	required := []string{"role", "content", "tool_call_id"}
	for _, key := range required {
		if _, ok := payload[key]; !ok {
			t.Fatalf("expected key %q in message payload", key)
		}
	}

	var role string
	if err := json.Unmarshal(payload["role"], &role); err != nil {
		t.Fatalf("unmarshal role failed: %v", err)
	}
	if role != wantRole {
		t.Fatalf("expected role %q, got %q", wantRole, role)
	}

	var content string
	if err := json.Unmarshal(payload["content"], &content); err != nil {
		t.Fatalf("unmarshal content failed: %v", err)
	}
	if content != wantContent {
		t.Fatalf("expected content %q, got %q", wantContent, content)
	}

	var toolCallID string
	if err := json.Unmarshal(payload["tool_call_id"], &toolCallID); err != nil {
		t.Fatalf("unmarshal tool_call_id failed: %v", err)
	}
	if toolCallID != wantToolCallID {
		t.Fatalf("expected tool_call_id %q, got %q", wantToolCallID, toolCallID)
	}
}

func assertStatusPayload(t *testing.T, body []byte, wantStatus string) {
	t.Helper()

	var payload map[string]string
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("unmarshal status response failed: %v", err)
	}
	if payload["status"] != wantStatus {
		t.Fatalf("expected status %q, got %q", wantStatus, payload["status"])
	}
}

func assertErrorPayload(t *testing.T, body []byte) {
	t.Helper()

	var payload map[string]string
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("unmarshal error response failed: %v", err)
	}
	if strings.TrimSpace(payload["error"]) == "" {
		t.Fatalf("expected non-empty error payload, got %s", string(body))
	}
}
