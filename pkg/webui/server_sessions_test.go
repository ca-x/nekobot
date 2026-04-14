package webui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"

	"nekobot/pkg/agent"
	"nekobot/pkg/config"
	"nekobot/pkg/session"
	"nekobot/pkg/state"
	"nekobot/pkg/threads"
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
		{
			name:       "update-runtime",
			handler:    s.handleUpdateSessionRuntime,
			method:     http.MethodPut,
			target:     "/api/sessions/s1/runtime",
			body:       `{"runtime_id":"rt-1"}`,
			path:       "/api/sessions/:id/runtime",
			pathValues: echo.PathValues{{Name: "id", Value: "s1"}},
		},
		{
			name:       "update-thread",
			handler:    s.handleUpdateSessionThread,
			method:     http.MethodPut,
			target:     "/api/sessions/s1/thread",
			body:       `{"topic":"ops triage"}`,
			path:       "/api/sessions/:id/thread",
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
	cfg := config.DefaultConfig()
	sm := session.NewManager(t.TempDir(), cfg.Sessions)
	log := newTestLogger(t)
	store, err := state.NewFileStore(log, &state.FileStoreConfig{FilePath: filepath.Join(t.TempDir(), "session-state.json")})
	if err != nil {
		t.Fatalf("new file store failed: %v", err)
	}
	defer func() { _ = store.Close() }()
	s := &Server{sessionMgr: sm, kvStore: store, threads: threads.NewManager(store)}
	e := echo.New()

	const sessionID = "webui-e2e"
	sess, err := sm.GetWithSource(sessionID, session.SourceWebUI)
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

	runtimeReq := httptest.NewRequest(http.MethodPut, "/api/sessions/"+sessionID+"/runtime", strings.NewReader(`{"runtime_id":"daemon-runtime-1"}`))
	runtimeReq.Header.Set("Content-Type", "application/json")
	runtimeRec := httptest.NewRecorder()
	runtimeCtx := e.NewContext(runtimeReq, runtimeRec)
	runtimeCtx.SetPath("/api/sessions/:id/runtime")
	runtimeCtx.SetPathValues(echo.PathValues{{Name: "id", Value: sessionID}})
	if err := s.handleUpdateSessionRuntime(runtimeCtx); err != nil {
		t.Fatalf("runtime update handler failed: %v", err)
	}
	if runtimeRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, runtimeRec.Code)
	}
	assertStatusPayload(t, runtimeRec.Body.Bytes(), "updated")

	getAfterRuntimeReq := httptest.NewRequest(http.MethodGet, "/api/sessions/"+sessionID, nil)
	getAfterRuntimeRec := httptest.NewRecorder()
	getAfterRuntimeCtx := e.NewContext(getAfterRuntimeReq, getAfterRuntimeRec)
	getAfterRuntimeCtx.SetPath("/api/sessions/:id")
	getAfterRuntimeCtx.SetPathValues(echo.PathValues{{Name: "id", Value: sessionID}})
	if err := s.handleGetSession(getAfterRuntimeCtx); err != nil {
		t.Fatalf("get-after-runtime handler failed: %v", err)
	}
	var detailAfterRuntime map[string]json.RawMessage
	if err := json.Unmarshal(getAfterRuntimeRec.Body.Bytes(), &detailAfterRuntime); err != nil {
		t.Fatalf("unmarshal runtime detail response failed: %v", err)
	}
	var runtimeID string
	if err := json.Unmarshal(detailAfterRuntime["runtime_id"], &runtimeID); err != nil {
		t.Fatalf("unmarshal runtime_id failed: %v", err)
	}
	if runtimeID != "daemon-runtime-1" {
		t.Fatalf("expected runtime binding daemon-runtime-1, got %q", runtimeID)
	}

	threadReq := httptest.NewRequest(http.MethodPut, "/api/sessions/"+sessionID+"/thread", strings.NewReader(`{"topic":"ops triage"}`))
	threadReq.Header.Set("Content-Type", "application/json")
	threadRec := httptest.NewRecorder()
	threadCtx := e.NewContext(threadReq, threadRec)
	threadCtx.SetPath("/api/sessions/:id/thread")
	threadCtx.SetPathValues(echo.PathValues{{Name: "id", Value: sessionID}})
	if err := s.handleUpdateSessionThread(threadCtx); err != nil {
		t.Fatalf("thread update handler failed: %v", err)
	}
	if threadRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, threadRec.Code)
	}
	assertStatusPayload(t, threadRec.Body.Bytes(), "updated")

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
	cfg := config.DefaultConfig()
	sm := session.NewManager(t.TempDir(), cfg.Sessions)
	log := newTestLogger(t)
	store, err := state.NewFileStore(log, &state.FileStoreConfig{FilePath: filepath.Join(t.TempDir(), "session-state.json")})
	if err != nil {
		t.Fatalf("new file store failed: %v", err)
	}
	defer func() { _ = store.Close() }()
	s := &Server{sessionMgr: sm, kvStore: store, threads: threads.NewManager(store)}
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

	runtimeReq := httptest.NewRequest(http.MethodPut, "/api/sessions/"+missingID+"/runtime", strings.NewReader(`{"runtime_id":"rt-1"}`))
	runtimeReq.Header.Set("Content-Type", "application/json")
	runtimeRec := httptest.NewRecorder()
	runtimeCtx := e.NewContext(runtimeReq, runtimeRec)
	runtimeCtx.SetPath("/api/sessions/:id/runtime")
	runtimeCtx.SetPathValues(echo.PathValues{{Name: "id", Value: missingID}})
	if err := s.handleUpdateSessionRuntime(runtimeCtx); err != nil {
		t.Fatalf("runtime update handler failed: %v", err)
	}
	if runtimeRec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, runtimeRec.Code)
	}
	assertErrorPayload(t, runtimeRec.Body.Bytes())

	threadReq := httptest.NewRequest(http.MethodPut, "/api/sessions/"+missingID+"/thread", strings.NewReader(`{"topic":"ops triage"}`))
	threadReq.Header.Set("Content-Type", "application/json")
	threadRec := httptest.NewRecorder()
	threadCtx := e.NewContext(threadReq, threadRec)
	threadCtx.SetPath("/api/sessions/:id/thread")
	threadCtx.SetPathValues(echo.PathValues{{Name: "id", Value: missingID}})
	if err := s.handleUpdateSessionThread(threadCtx); err != nil {
		t.Fatalf("thread update handler failed: %v", err)
	}
	if threadRec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, threadRec.Code)
	}
	assertErrorPayload(t, threadRec.Body.Bytes())

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

func TestThreadHandlers_EndToEndFlow(t *testing.T) {
	cfg := config.DefaultConfig()
	sm := session.NewManager(t.TempDir(), cfg.Sessions)
	log := newTestLogger(t)
	store, err := state.NewFileStore(log, &state.FileStoreConfig{FilePath: filepath.Join(t.TempDir(), "thread-state.json")})
	if err != nil {
		t.Fatalf("new file store failed: %v", err)
	}
	defer func() { _ = store.Close() }()
	s := &Server{sessionMgr: sm, kvStore: store, threads: threads.NewManager(store)}
	e := echo.New()

	const threadID = "thread-e2e"
	sess, err := sm.GetWithSource(threadID, session.SourceWebUI)
	if err != nil {
		t.Fatalf("create thread failed: %v", err)
	}
	sess.AddMessage(agent.Message{Role: "user", Content: "hello"})
	sess.SetSummary("initial thread")
	if err := sm.Save(sess); err != nil {
		t.Fatalf("save thread failed: %v", err)
	}
	if err := s.setThreadRuntimeBinding(threadID, "daemon-runtime-1"); err != nil {
		t.Fatalf("set runtime binding failed: %v", err)
	}
	if err := s.setThreadTopic(threadID, "ops triage"); err != nil {
		t.Fatalf("set thread topic failed: %v", err)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/threads", nil)
	listRec := httptest.NewRecorder()
	listCtx := e.NewContext(listReq, listRec)
	if err := s.handleListThreads(listCtx); err != nil {
		t.Fatalf("list threads failed: %v", err)
	}
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, listRec.Code)
	}

	var listed []map[string]json.RawMessage
	if err := json.Unmarshal(listRec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("unmarshal list response failed: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected 1 thread, got %d", len(listed))
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/threads/"+threadID, nil)
	getRec := httptest.NewRecorder()
	getCtx := e.NewContext(getReq, getRec)
	getCtx.SetPath("/api/threads/:id")
	getCtx.SetPathValues(echo.PathValues{{Name: "id", Value: threadID}})
	if err := s.handleGetThread(getCtx); err != nil {
		t.Fatalf("get thread failed: %v", err)
	}
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, getRec.Code)
	}
	var detail map[string]json.RawMessage
	if err := json.Unmarshal(getRec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("unmarshal thread detail failed: %v", err)
	}
	var topic string
	if err := json.Unmarshal(detail["topic"], &topic); err != nil {
		t.Fatalf("unmarshal topic failed: %v", err)
	}
	if topic != "ops triage" {
		t.Fatalf("expected topic ops triage, got %q", topic)
	}

	updateReq := httptest.NewRequest(http.MethodPut, "/api/threads/"+threadID, strings.NewReader(`{"summary":"updated thread","runtime_id":"daemon-runtime-2","topic":"incident response"}`))
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	updateCtx := e.NewContext(updateReq, updateRec)
	updateCtx.SetPath("/api/threads/:id")
	updateCtx.SetPathValues(echo.PathValues{{Name: "id", Value: threadID}})
	if err := s.handleUpdateThread(updateCtx); err != nil {
		t.Fatalf("update thread failed: %v", err)
	}
	if updateRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, updateRec.Code)
	}
	assertStatusPayload(t, updateRec.Body.Bytes(), "updated")

	getAfterReq := httptest.NewRequest(http.MethodGet, "/api/threads/"+threadID, nil)
	getAfterRec := httptest.NewRecorder()
	getAfterCtx := e.NewContext(getAfterReq, getAfterRec)
	getAfterCtx.SetPath("/api/threads/:id")
	getAfterCtx.SetPathValues(echo.PathValues{{Name: "id", Value: threadID}})
	if err := s.handleGetThread(getAfterCtx); err != nil {
		t.Fatalf("get thread after update failed: %v", err)
	}
	var detailAfter map[string]json.RawMessage
	if err := json.Unmarshal(getAfterRec.Body.Bytes(), &detailAfter); err != nil {
		t.Fatalf("unmarshal thread detail after update failed: %v", err)
	}
	var runtimeID string
	if err := json.Unmarshal(detailAfter["runtime_id"], &runtimeID); err != nil {
		t.Fatalf("unmarshal runtime_id failed: %v", err)
	}
	if runtimeID != "daemon-runtime-2" {
		t.Fatalf("expected runtime daemon-runtime-2, got %q", runtimeID)
	}
	if err := json.Unmarshal(detailAfter["topic"], &topic); err != nil {
		t.Fatalf("unmarshal topic after update failed: %v", err)
	}
	if topic != "incident response" {
		t.Fatalf("expected topic incident response, got %q", topic)
	}
}

func TestHandleCleanupSessions(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Sessions.Enabled = true
	cfg.Sessions.Cleanup.Enabled = true
	cfg.Sessions.Cleanup.MaxAgeDays = 1
	sm := session.NewManager(t.TempDir(), cfg.Sessions)
	s := &Server{sessionMgr: sm, config: cfg}
	e := echo.New()

	req := httptest.NewRequest(http.MethodPost, "/api/sessions/cleanup", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := s.handleCleanupSessions(c); err != nil {
		t.Fatalf("cleanup handler failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	assertStatusPayload(t, rec.Body.Bytes(), "cleaned")
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
