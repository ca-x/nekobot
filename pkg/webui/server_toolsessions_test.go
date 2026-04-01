package webui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v5"

	"nekobot/pkg/config"
	"nekobot/pkg/execenv"
	"nekobot/pkg/process"
	"nekobot/pkg/toolsessions"
)

func TestToolSessionHandlers_Return503WhenManagerUnavailable(t *testing.T) {
	s := &Server{}
	e := echo.New()

	tests := []struct {
		name   string
		method string
		target string
		path   string
		body   string
		call   func(*echo.Context) error
	}{
		{
			name:   "list",
			method: http.MethodGet,
			target: "/api/tool-sessions",
			path:   "/api/tool-sessions",
			call:   s.handleListToolSessions,
		},
		{
			name:   "create",
			method: http.MethodPost,
			target: "/api/tool-sessions",
			path:   "/api/tool-sessions",
			body:   `{"tool":"codex"}`,
			call:   s.handleCreateToolSession,
		},
		{
			name:   "attach-token",
			method: http.MethodPost,
			target: "/api/tool-sessions/s1/attach-token",
			path:   "/api/tool-sessions/:id/attach-token",
			body:   `{"ttl_seconds":60}`,
			call:   s.handleCreateToolSessionAttachToken,
		},
		{
			name:   "consume-token",
			method: http.MethodPost,
			target: "/api/tool-sessions/consume-token",
			path:   "/api/tool-sessions/consume-token",
			body:   `{"token":"abc"}`,
			call:   s.handleConsumeToolSessionAttachToken,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.target, strings.NewReader(tc.body))
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			rec := httptest.NewRecorder()
			ctx := newAuthedContext(e, req, rec, "alice")
			ctx.SetPath(tc.path)
			if strings.Contains(tc.path, ":id") {
				ctx.SetPathValues(echo.PathValues{{Name: "id", Value: "s1"}})
			}

			if err := tc.call(ctx); err != nil {
				t.Fatalf("handler failed: %v", err)
			}
			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
			}
			assertErrorPayload(t, rec.Body.Bytes())
		})
	}
}

func TestToolSessionHandlers_SmokeFlow(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.WebUI.ToolSessionEvents.Enabled = true
	cfg.WebUI.ToolSessionEvents.RetentionDays = 14

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close ent client: %v", err)
		}
	})

	toolMgr, err := toolsessions.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new tool session manager: %v", err)
	}

	server := &Server{
		config:     cfg,
		logger:     log,
		toolSess:   toolMgr,
		processMgr: process.NewManager(log),
	}
	e := echo.New()

	createReq := httptest.NewRequest(http.MethodPost, "/api/tool-sessions", strings.NewReader(
		`{"tool":"codex","title":"Smoke Session","command":"cat","workdir":"`+cfg.WorkspacePath()+`","access_mode":"permanent","access_password":"perm-pass","metadata":{"suite":"smoke"}}`,
	))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	createCtx := newAuthedContext(e, createReq, createRec, "alice")
	createCtx.SetPath("/api/tool-sessions")
	if err := server.handleCreateToolSession(createCtx); err != nil {
		t.Fatalf("create handler failed: %v", err)
	}
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, createRec.Code, createRec.Body.String())
	}

	var created toolsessions.Session
	decodeJSON(t, createRec.Body.Bytes(), &created)
	if created.ID == "" {
		t.Fatalf("expected created session id, got %+v", created)
	}
	if created.Owner != "alice" {
		t.Fatalf("expected owner alice, got %q", created.Owner)
	}
	if created.AccessMode != toolsessions.AccessModePermanent {
		t.Fatalf("expected permanent access mode, got %q", created.AccessMode)
	}

	listAliceReq := httptest.NewRequest(http.MethodGet, "/api/tool-sessions", nil)
	listAliceRec := httptest.NewRecorder()
	listAliceCtx := newAuthedContext(e, listAliceReq, listAliceRec, "alice")
	listAliceCtx.SetPath("/api/tool-sessions")
	if err := server.handleListToolSessions(listAliceCtx); err != nil {
		t.Fatalf("list alice handler failed: %v", err)
	}
	if listAliceRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, listAliceRec.Code)
	}
	var aliceSessions []toolsessions.Session
	decodeJSON(t, listAliceRec.Body.Bytes(), &aliceSessions)
	if len(aliceSessions) != 1 || aliceSessions[0].ID != created.ID {
		t.Fatalf("expected alice to see created session, got %+v", aliceSessions)
	}

	listBobReq := httptest.NewRequest(http.MethodGet, "/api/tool-sessions", nil)
	listBobRec := httptest.NewRecorder()
	listBobCtx := newAuthedContext(e, listBobReq, listBobRec, "bob")
	listBobCtx.SetPath("/api/tool-sessions")
	if err := server.handleListToolSessions(listBobCtx); err != nil {
		t.Fatalf("list bob handler failed: %v", err)
	}
	if listBobRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, listBobRec.Code)
	}
	var bobSessions []toolsessions.Session
	decodeJSON(t, listBobRec.Body.Bytes(), &bobSessions)
	if len(bobSessions) != 0 {
		t.Fatalf("expected bob to see no sessions, got %+v", bobSessions)
	}

	otpReq := httptest.NewRequest(http.MethodPost, "/api/tool-sessions/"+created.ID+"/otp", strings.NewReader(`{"ttl_seconds":90}`))
	otpReq.Header.Set("Content-Type", "application/json")
	otpRec := httptest.NewRecorder()
	otpCtx := newAuthedContext(e, otpReq, otpRec, "alice")
	otpCtx.SetPath("/api/tool-sessions/:id/otp")
	otpCtx.SetPathValues(echo.PathValues{{Name: "id", Value: created.ID}})
	if err := server.handleGenerateToolSessionOTP(otpCtx); err != nil {
		t.Fatalf("generate otp handler failed: %v", err)
	}
	if otpRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, otpRec.Code, otpRec.Body.String())
	}
	var otpPayload struct {
		OTPCode string `json:"otp_code"`
	}
	decodeJSON(t, otpRec.Body.Bytes(), &otpPayload)
	if otpPayload.OTPCode == "" {
		t.Fatalf("expected otp code, got %s", otpRec.Body.String())
	}

	loginOTPReq := httptest.NewRequest(
		http.MethodPost,
		"/api/tool-sessions/access-login",
		strings.NewReader(`{"session_id":"`+created.ID+`","password":"`+otpPayload.OTPCode+`"}`),
	)
	loginOTPReq.Header.Set("Content-Type", "application/json")
	loginOTPRec := httptest.NewRecorder()
	loginOTPCtx := e.NewContext(loginOTPReq, loginOTPRec)
	if err := server.handleToolSessionAccessLogin(loginOTPCtx); err != nil {
		t.Fatalf("otp access login handler failed: %v", err)
	}
	if loginOTPRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, loginOTPRec.Code, loginOTPRec.Body.String())
	}
	var loginOTPPayload map[string]string
	decodeJSON(t, loginOTPRec.Body.Bytes(), &loginOTPPayload)
	if strings.TrimSpace(loginOTPPayload["token"]) == "" {
		t.Fatalf("expected login token, got %s", loginOTPRec.Body.String())
	}

	attachReq := httptest.NewRequest(
		http.MethodPost,
		"/api/tool-sessions/"+created.ID+"/attach-token",
		strings.NewReader(`{"ttl_seconds":120}`),
	)
	attachReq.Header.Set("Content-Type", "application/json")
	attachRec := httptest.NewRecorder()
	attachCtx := newAuthedContext(e, attachReq, attachRec, "alice")
	attachCtx.SetPath("/api/tool-sessions/:id/attach-token")
	attachCtx.SetPathValues(echo.PathValues{{Name: "id", Value: created.ID}})
	if err := server.handleCreateToolSessionAttachToken(attachCtx); err != nil {
		t.Fatalf("create attach token handler failed: %v", err)
	}
	if attachRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, attachRec.Code, attachRec.Body.String())
	}
	var attachPayload map[string]string
	decodeJSON(t, attachRec.Body.Bytes(), &attachPayload)
	attachToken := strings.TrimSpace(attachPayload["token"])
	if attachToken == "" {
		t.Fatalf("expected attach token, got %s", attachRec.Body.String())
	}

	consumeForbiddenReq := httptest.NewRequest(
		http.MethodPost,
		"/api/tool-sessions/consume-token",
		strings.NewReader(`{"token":"`+attachToken+`"}`),
	)
	consumeForbiddenReq.Header.Set("Content-Type", "application/json")
	consumeForbiddenRec := httptest.NewRecorder()
	consumeForbiddenCtx := newAuthedContext(e, consumeForbiddenReq, consumeForbiddenRec, "bob")
	consumeForbiddenCtx.SetPath("/api/tool-sessions/consume-token")
	if err := server.handleConsumeToolSessionAttachToken(consumeForbiddenCtx); err != nil {
		t.Fatalf("consume attach token forbidden handler failed: %v", err)
	}
	if consumeForbiddenRec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d: %s", http.StatusForbidden, consumeForbiddenRec.Code, consumeForbiddenRec.Body.String())
	}

	consumeReq := httptest.NewRequest(
		http.MethodPost,
		"/api/tool-sessions/consume-token",
		strings.NewReader(`{"token":"`+attachToken+`"}`),
	)
	consumeReq.Header.Set("Content-Type", "application/json")
	consumeRec := httptest.NewRecorder()
	consumeCtx := newAuthedContext(e, consumeReq, consumeRec, "alice")
	consumeCtx.SetPath("/api/tool-sessions/consume-token")
	if err := server.handleConsumeToolSessionAttachToken(consumeCtx); err != nil {
		t.Fatalf("consume attach token handler failed: %v", err)
	}
	if consumeRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, consumeRec.Code, consumeRec.Body.String())
	}
	var consumed toolsessions.Session
	decodeJSON(t, consumeRec.Body.Bytes(), &consumed)
	if consumed.ID != created.ID {
		t.Fatalf("expected consumed session id %q, got %q", created.ID, consumed.ID)
	}

	updateAccessReq := httptest.NewRequest(
		http.MethodPost,
		"/api/tool-sessions/"+created.ID+"/access",
		strings.NewReader(`{"mode":"one_time","password":"one-shot","public_base_url":"https://ui.example.com"}`),
	)
	updateAccessReq.Header.Set("Content-Type", "application/json")
	updateAccessRec := httptest.NewRecorder()
	updateAccessCtx := newAuthedContext(e, updateAccessReq, updateAccessRec, "alice")
	updateAccessCtx.SetPath("/api/tool-sessions/:id/access")
	updateAccessCtx.SetPathValues(echo.PathValues{{Name: "id", Value: created.ID}})
	if err := server.handleUpdateToolSessionAccess(updateAccessCtx); err != nil {
		t.Fatalf("update access handler failed: %v", err)
	}
	if updateAccessRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, updateAccessRec.Code, updateAccessRec.Body.String())
	}
	var accessPayload map[string]interface{}
	decodeJSON(t, updateAccessRec.Body.Bytes(), &accessPayload)
	if got, _ := accessPayload["access_mode"].(string); got != toolsessions.AccessModeOneTime {
		t.Fatalf("expected one_time access mode, got %+v", accessPayload)
	}

	loginReq := httptest.NewRequest(
		http.MethodPost,
		"/api/tool-sessions/access-login",
		strings.NewReader(`{"session_id":"`+created.ID+`","password":"one-shot"}`),
	)
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	loginCtx := e.NewContext(loginReq, loginRec)
	if err := server.handleToolSessionAccessLogin(loginCtx); err != nil {
		t.Fatalf("access login handler failed: %v", err)
	}
	if loginRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, loginRec.Code, loginRec.Body.String())
	}

	loginAgainReq := httptest.NewRequest(
		http.MethodPost,
		"/api/tool-sessions/access-login",
		strings.NewReader(`{"session_id":"`+created.ID+`","password":"one-shot"}`),
	)
	loginAgainReq.Header.Set("Content-Type", "application/json")
	loginAgainRec := httptest.NewRecorder()
	loginAgainCtx := e.NewContext(loginAgainReq, loginAgainRec)
	if err := server.handleToolSessionAccessLogin(loginAgainCtx); err != nil {
		t.Fatalf("second access login handler failed: %v", err)
	}
	if loginAgainRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d: %s", http.StatusUnauthorized, loginAgainRec.Code, loginAgainRec.Body.String())
	}

	if err := server.processMgr.Start(context.Background(), created.ID, "cat", cfg.WorkspacePath()); err != nil {
		t.Fatalf("start process failed: %v", err)
	}

	statusBobReq := httptest.NewRequest(http.MethodGet, "/api/tool-sessions/"+created.ID+"/process/status", nil)
	statusBobRec := httptest.NewRecorder()
	statusBobCtx := newAuthedContext(e, statusBobReq, statusBobRec, "bob")
	statusBobCtx.SetPath("/api/tool-sessions/:id/process/status")
	statusBobCtx.SetPathValues(echo.PathValues{{Name: "id", Value: created.ID}})
	if err := server.handleToolSessionProcessStatus(statusBobCtx); err != nil {
		t.Fatalf("process status bob handler failed: %v", err)
	}
	if statusBobRec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d: %s", http.StatusForbidden, statusBobRec.Code, statusBobRec.Body.String())
	}

	statusReq := httptest.NewRequest(http.MethodGet, "/api/tool-sessions/"+created.ID+"/process/status", nil)
	statusRec := httptest.NewRecorder()
	statusCtx := newAuthedContext(e, statusReq, statusRec, "alice")
	statusCtx.SetPath("/api/tool-sessions/:id/process/status")
	statusCtx.SetPathValues(echo.PathValues{{Name: "id", Value: created.ID}})
	if err := server.handleToolSessionProcessStatus(statusCtx); err != nil {
		t.Fatalf("process status handler failed: %v", err)
	}
	if statusRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, statusRec.Code, statusRec.Body.String())
	}
	var statusPayload struct {
		ID      string `json:"id"`
		Running bool   `json:"running"`
	}
	decodeJSON(t, statusRec.Body.Bytes(), &statusPayload)
	if statusPayload.ID != created.ID || !statusPayload.Running {
		t.Fatalf("expected running process status, got %+v", statusPayload)
	}

	inputReq := httptest.NewRequest(
		http.MethodPost,
		"/api/tool-sessions/"+created.ID+"/process/input",
		strings.NewReader(`{"data":"ping from webui\n"}`),
	)
	inputReq.Header.Set("Content-Type", "application/json")
	inputRec := httptest.NewRecorder()
	inputCtx := newAuthedContext(e, inputReq, inputRec, "alice")
	inputCtx.SetPath("/api/tool-sessions/:id/process/input")
	inputCtx.SetPathValues(echo.PathValues{{Name: "id", Value: created.ID}})
	if err := server.handleToolSessionProcessInput(inputCtx); err != nil {
		t.Fatalf("process input handler failed: %v", err)
	}
	if inputRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, inputRec.Code, inputRec.Body.String())
	}
	assertStatusPayload(t, inputRec.Body.Bytes(), "ok")

	waitForProcessOutput(t, func() (bool, error) {
		outputReq := httptest.NewRequest(http.MethodGet, "/api/tool-sessions/"+created.ID+"/process/output?offset=0&limit=50", nil)
		outputRec := httptest.NewRecorder()
		outputCtx := newAuthedContext(e, outputReq, outputRec, "alice")
		outputCtx.SetPath("/api/tool-sessions/:id/process/output")
		outputCtx.SetPathValues(echo.PathValues{{Name: "id", Value: created.ID}})
		if err := server.handleToolSessionProcessOutput(outputCtx); err != nil {
			return false, err
		}
		if outputRec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d: %s", http.StatusOK, outputRec.Code, outputRec.Body.String())
		}
		var outputPayload struct {
			Lines []string `json:"lines"`
		}
		decodeJSON(t, outputRec.Body.Bytes(), &outputPayload)
		for _, line := range outputPayload.Lines {
			if strings.Contains(line, "ping from webui") {
				return true, nil
			}
		}
		return false, nil
	})

	killReq := httptest.NewRequest(http.MethodPost, "/api/tool-sessions/"+created.ID+"/process/kill", nil)
	killRec := httptest.NewRecorder()
	killCtx := newAuthedContext(e, killReq, killRec, "alice")
	killCtx.SetPath("/api/tool-sessions/:id/process/kill")
	killCtx.SetPathValues(echo.PathValues{{Name: "id", Value: created.ID}})
	if err := server.handleToolSessionProcessKill(killCtx); err != nil {
		t.Fatalf("process kill handler failed: %v", err)
	}
	if killRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, killRec.Code, killRec.Body.String())
	}
	assertStatusPayload(t, killRec.Body.Bytes(), "killed")

	waitForSessionState(t, toolMgr, created.ID, toolsessions.StateTerminated)

	cleanupReq := httptest.NewRequest(http.MethodPost, "/api/tool-sessions/cleanup-terminated", nil)
	cleanupRec := httptest.NewRecorder()
	cleanupCtx := newAuthedContext(e, cleanupReq, cleanupRec, "alice")
	cleanupCtx.SetPath("/api/tool-sessions/cleanup-terminated")
	if err := server.handleCleanupTerminatedToolSessions(cleanupCtx); err != nil {
		t.Fatalf("cleanup terminated handler failed: %v", err)
	}
	if cleanupRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, cleanupRec.Code, cleanupRec.Body.String())
	}
	var cleanupPayload map[string]int
	decodeJSON(t, cleanupRec.Body.Bytes(), &cleanupPayload)
	if cleanupPayload["archived"] != 1 {
		t.Fatalf("expected archived=1, got %+v", cleanupPayload)
	}
	waitForSessionState(t, toolMgr, created.ID, toolsessions.StateArchived)

	eventsCleanupReq := httptest.NewRequest(http.MethodPost, "/api/tool-sessions/events/cleanup", nil)
	eventsCleanupRec := httptest.NewRecorder()
	eventsCleanupCtx := newAuthedContext(e, eventsCleanupReq, eventsCleanupRec, "alice")
	eventsCleanupCtx.SetPath("/api/tool-sessions/events/cleanup")
	if err := server.handleCleanupToolSessionEvents(eventsCleanupCtx); err != nil {
		t.Fatalf("cleanup events handler failed: %v", err)
	}
	if eventsCleanupRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, eventsCleanupRec.Code, eventsCleanupRec.Body.String())
	}
	var deletedPayload map[string]int
	decodeJSON(t, eventsCleanupRec.Body.Bytes(), &deletedPayload)
	if deletedPayload["deleted"] < 1 {
		t.Fatalf("expected deleted events > 0, got %+v", deletedPayload)
	}
}

func newAuthedContext(e *echo.Echo, req *http.Request, rec *httptest.ResponseRecorder, username string) *echo.Context {
	ctx := e.NewContext(req, rec)
	ctx.Set("user", jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": username,
	}))
	return ctx
}

func decodeJSON(t *testing.T, body []byte, target interface{}) {
	t.Helper()
	if err := json.Unmarshal(body, target); err != nil {
		t.Fatalf("unmarshal json failed: %v; body=%s", err, string(body))
	}
}

func waitForProcessOutput(t *testing.T, check func() (bool, error)) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		ok, err := check()
		if err != nil {
			t.Fatalf("check process output: %v", err)
		}
		if ok {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for process output")
}

func waitForSessionState(t *testing.T, mgr *toolsessions.Manager, sessionID, wantState string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		sess, err := mgr.GetSession(context.Background(), sessionID)
		if err == nil && sess.State == wantState {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	sess, err := mgr.GetSession(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("get session after wait: %v", err)
	}
	t.Fatalf("expected state %q, got %q", wantState, sess.State)
}

func TestHandleSpawnToolSessionPassesMetadataToExecenv(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() { _ = client.Close() })

	toolMgr, err := toolsessions.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new tool session manager: %v", err)
	}

	preparer := &captureWebUITestPreparer{}
	pm := process.NewManager(log)
	pm.SetPreparer(preparer)
	server := &Server{
		config:     cfg,
		logger:     log,
		toolSess:   toolMgr,
		processMgr: pm,
	}
	e := echo.New()

	req := httptest.NewRequest(http.MethodPost, "/api/tool-sessions/spawn", strings.NewReader(
		`{"tool":"codex","command":"sleep 5","workdir":"`+cfg.WorkspacePath()+`","metadata":{"runtime_id":"runtime-webui","task_id":"task-webui"}}`,
	))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ctx := newAuthedContext(e, req, rec, "alice")
	ctx.SetPath("/api/tool-sessions/spawn")
	if err := server.handleSpawnToolSession(ctx); err != nil {
		t.Fatalf("spawn handler failed: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, rec.Code, rec.Body.String())
	}
	if preparer.last.RuntimeID != "runtime-webui" {
		t.Fatalf("expected runtime id to propagate, got %q", preparer.last.RuntimeID)
	}
	if preparer.last.TaskID != "task-webui" {
		t.Fatalf("expected task id to propagate, got %q", preparer.last.TaskID)
	}
	if preparer.last.SessionID == "" {
		t.Fatal("expected spawned session id")
	}
	if err := pm.Reset(preparer.last.SessionID); err != nil {
		t.Fatalf("reset spawned process: %v", err)
	}
}

type captureWebUITestPreparer struct {
	last execenv.StartSpec
}

func (c *captureWebUITestPreparer) Prepare(_ context.Context, spec execenv.StartSpec) (execenv.Prepared, error) {
	c.last = spec
	return execenv.Prepared{
		Workdir: spec.Workdir,
		Env:     append([]string{}, spec.Env...),
		Cleanup: func() error { return nil },
	}, nil
}
