package webui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v5"

	"nekobot/pkg/config"
	"nekobot/pkg/goaldriven"
	goaldrivencriteria "nekobot/pkg/goaldriven/criteria"
	goaldrivenscope "nekobot/pkg/goaldriven/scope"
	"nekobot/pkg/logger"
	"nekobot/pkg/state"
	"nekobot/pkg/tools"
)

func TestHandleCreateGoalRunAndGetAndList(t *testing.T) {
	cfg := config.DefaultConfig()
	store := goaldriven.NewMemoryStore()
	svc := goaldriven.NewService(
		store,
		goaldrivencriteria.NewParser(),
		goaldrivencriteria.NewSchema(),
		goaldrivenscope.NewResolver(),
	)
	server := &Server{config: cfg, goalSvc: svc}
	e := echo.New()

	body := map[string]any{
		"name":                      "daemon rollout",
		"goal":                      "fix daemon host rollout",
		"natural_language_criteria": "confirm the daemon host rollout succeeded",
		"risk_level":                "balanced",
		"allow_auto_scope":          true,
	}
	payload, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/goal-runs", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	ctx.Set("user", testJWTToken("alice"))

	if err := server.handleCreateGoalRun(ctx); err != nil {
		t.Fatalf("handleCreateGoalRun failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var created struct {
		GoalRun goaldriven.GoalRun `json:"goal_run"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal create response: %v", err)
	}
	if created.GoalRun.ID == "" {
		t.Fatalf("expected goal run id, got %+v", created.GoalRun)
	}
	if created.GoalRun.Status != goaldriven.GoalStatusCriteriaPendingConfirm {
		t.Fatalf("expected criteria pending status, got %q", created.GoalRun.Status)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/goal-runs/"+created.GoalRun.ID, nil)
	getRec := httptest.NewRecorder()
	getCtx := e.NewContext(getReq, getRec)
	getCtx.SetPath("/api/goal-runs/:id")
	getCtx.SetPathValues(echo.PathValues{{Name: "id", Value: created.GoalRun.ID}})
	if err := server.handleGetGoalRun(getCtx); err != nil {
		t.Fatalf("handleGetGoalRun failed: %v", err)
	}
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", getRec.Code, getRec.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/goal-runs", nil)
	listRec := httptest.NewRecorder()
	listCtx := e.NewContext(listReq, listRec)
	if err := server.handleListGoalRuns(listCtx); err != nil {
		t.Fatalf("handleListGoalRuns failed: %v", err)
	}
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", listRec.Code, listRec.Body.String())
	}
	var listed struct {
		Items []goaldriven.GoalRun `json:"items"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("unmarshal list response: %v", err)
	}
	if len(listed.Items) != 1 {
		t.Fatalf("expected 1 goal run, got %+v", listed.Items)
	}
}

func TestHandleConfirmGoalRunCriteria(t *testing.T) {
	cfg := config.DefaultConfig()
	store := goaldriven.NewMemoryStore()
	svc := goaldriven.NewService(
		store,
		goaldrivencriteria.NewParser(),
		goaldrivencriteria.NewSchema(),
		goaldrivenscope.NewResolver(),
	)
	server := &Server{config: cfg, goalSvc: svc}
	e := echo.New()

	created, err := svc.CreateGoalRun(t.Context(), goaldriven.CreateGoalRunInput{
		Name:                    "server build",
		Goal:                    "verify local server build",
		NaturalLanguageCriteria: "go build ./cmd/nekobot",
		RiskLevel:               goaldriven.RiskBalanced,
		AllowAutoScope:          true,
		CreatedBy:               "alice",
	})
	if err != nil {
		t.Fatalf("create goal run: %v", err)
	}

	confirmBody := map[string]any{
		"criteria": map[string]any{
			"criteria": []map[string]any{
				{
					"id":       "build-pass",
					"title":    "Build passes",
					"type":     "command",
					"required": true,
					"scope": map[string]any{
						"kind":   "server",
						"source": "manual",
					},
					"definition": map[string]any{
						"command":          "go build ./cmd/nekobot",
						"expect_exit_code": 0,
					},
				},
			},
		},
		"selected_scope": map[string]any{
			"kind":   "server",
			"source": "manual",
		},
	}
	payload, _ := json.Marshal(confirmBody)
	req := httptest.NewRequest(http.MethodPost, "/api/goal-runs/"+created.GoalRun.ID+"/confirm-criteria", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	ctx.SetPath("/api/goal-runs/:id/confirm-criteria")
	ctx.SetPathValues(echo.PathValues{{Name: "id", Value: created.GoalRun.ID}})

	if err := server.handleConfirmGoalRunCriteria(ctx); err != nil {
		t.Fatalf("handleConfirmGoalRunCriteria failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	detail, ok, err := svc.GetGoalRunDetail(t.Context(), created.GoalRun.ID)
	if err != nil || !ok {
		t.Fatalf("GetGoalRunDetail failed: ok=%v err=%v", ok, err)
	}
	if detail.GoalRun.Status != goaldriven.GoalStatusReady {
		t.Fatalf("expected ready status, got %q", detail.GoalRun.Status)
	}
	if detail.GoalRun.SelectedScope == nil || detail.GoalRun.SelectedScope.Kind != goaldriven.ScopeServer {
		t.Fatalf("expected selected scope server, got %+v", detail.GoalRun.SelectedScope)
	}
}

func TestHandleStartStopCancelAndManualConfirmGoalRun(t *testing.T) {
	cfg := config.DefaultConfig()
	log, err := logger.New(&logger.Config{Level: logger.LevelInfo, Development: true})
	if err != nil {
		t.Fatalf("logger.New failed: %v", err)
	}
	store := goaldriven.NewMemoryStore()
	svc := goaldriven.NewService(
		store,
		goaldrivencriteria.NewParser(),
		goaldrivencriteria.NewSchema(),
		goaldrivenscope.NewResolver(),
	)
	svc.SetLogger(log)
	svc.SetServerRunner(func(_ context.Context, run goaldriven.GoalRun) (goaldriven.WorkerRef, error) {
		return goaldriven.WorkerRef{
			ID:              "gw_test",
			Name:            "server-worker",
			Status:          "completed",
			Scope:           *run.SelectedScope,
			TaskID:          "task_test",
			LastHeartbeatAt: time.Now().UTC(),
			LastProgressAt:  time.Now().UTC(),
			LeaseExpiresAt:  time.Now().UTC().Add(5 * time.Minute),
		}, nil
	})

	server := &Server{config: cfg, goalSvc: svc}
	e := echo.New()

	created, err := svc.CreateGoalRun(t.Context(), goaldriven.CreateGoalRunInput{
		Name:                    "server build",
		Goal:                    "verify local build",
		NaturalLanguageCriteria: "confirm the build result manually",
		RiskLevel:               goaldriven.RiskBalanced,
		AllowAutoScope:          true,
		CreatedBy:               "alice",
	})
	if err != nil {
		t.Fatalf("CreateGoalRun failed: %v", err)
	}
	_, err = svc.ConfirmCriteria(t.Context(), created.GoalRun.ID, goaldrivencriteria.Set{
		Criteria: []goaldrivencriteria.Item{
			{
				ID:       "manual-1",
				Title:    "Manual confirmation",
				Type:     goaldrivencriteria.TypeManualConfirmation,
				Scope:    goaldriven.ExecutionScope{Kind: goaldriven.ScopeServer, Source: "manual"},
				Required: true,
				Status:   goaldrivencriteria.StatusPending,
				Definition: map[string]any{
					"prompt": "Confirm success",
				},
				UpdatedAt: time.Now().UTC(),
			},
		},
	}, &goaldriven.ExecutionScope{Kind: goaldriven.ScopeServer, Source: "manual"})
	if err != nil {
		t.Fatalf("ConfirmCriteria failed: %v", err)
	}

	startReq := httptest.NewRequest(http.MethodPost, "/api/goal-runs/"+created.GoalRun.ID+"/start", nil)
	startRec := httptest.NewRecorder()
	startCtx := e.NewContext(startReq, startRec)
	startCtx.SetPath("/api/goal-runs/:id/start")
	startCtx.SetPathValues(echo.PathValues{{Name: "id", Value: created.GoalRun.ID}})
	if err := server.handleStartGoalRun(startCtx); err != nil {
		t.Fatalf("handleStartGoalRun failed: %v", err)
	}
	if startRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", startRec.Code, startRec.Body.String())
	}

	waitForGoalStatus(t, svc, created.GoalRun.ID, goaldriven.GoalStatusNeedsHumanConfirmation)

	confirmBody := map[string]any{
		"criterion_id": "manual-1",
		"approved":     true,
		"note":         "operator confirmed success",
	}
	payload, _ := json.Marshal(confirmBody)
	confirmReq := httptest.NewRequest(http.MethodPost, "/api/goal-runs/"+created.GoalRun.ID+"/confirm-manual", bytes.NewReader(payload))
	confirmReq.Header.Set("Content-Type", "application/json")
	confirmRec := httptest.NewRecorder()
	confirmCtx := e.NewContext(confirmReq, confirmRec)
	confirmCtx.SetPath("/api/goal-runs/:id/confirm-manual")
	confirmCtx.SetPathValues(echo.PathValues{{Name: "id", Value: created.GoalRun.ID}})
	if err := server.handleConfirmGoalRunManualCriterion(confirmCtx); err != nil {
		t.Fatalf("handleConfirmGoalRunManualCriterion failed: %v", err)
	}
	if confirmRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", confirmRec.Code, confirmRec.Body.String())
	}

	canceledCreate, err := svc.CreateGoalRun(t.Context(), goaldriven.CreateGoalRunInput{
		Name:                    "cancel me",
		Goal:                    "cancel path",
		NaturalLanguageCriteria: "confirm manually",
		RiskLevel:               goaldriven.RiskBalanced,
		AllowAutoScope:          true,
		CreatedBy:               "alice",
	})
	if err != nil {
		t.Fatalf("CreateGoalRun cancel case failed: %v", err)
	}
	_, err = svc.ConfirmCriteria(t.Context(), canceledCreate.GoalRun.ID, goaldrivencriteria.Set{
		Criteria: []goaldrivencriteria.Item{
			{
				ID:       "manual-2",
				Title:    "Manual confirmation",
				Type:     goaldrivencriteria.TypeManualConfirmation,
				Scope:    goaldriven.ExecutionScope{Kind: goaldriven.ScopeServer, Source: "manual"},
				Required: true,
				Status:   goaldrivencriteria.StatusPending,
				Definition: map[string]any{
					"prompt": "Confirm success",
				},
				UpdatedAt: time.Now().UTC(),
			},
		},
	}, &goaldriven.ExecutionScope{Kind: goaldriven.ScopeServer, Source: "manual"})
	if err != nil {
		t.Fatalf("ConfirmCriteria cancel case failed: %v", err)
	}

	stopReq := httptest.NewRequest(http.MethodPost, "/api/goal-runs/"+canceledCreate.GoalRun.ID+"/start", nil)
	stopRec := httptest.NewRecorder()
	stopCtx := e.NewContext(stopReq, stopRec)
	stopCtx.SetPath("/api/goal-runs/:id/start")
	stopCtx.SetPathValues(echo.PathValues{{Name: "id", Value: canceledCreate.GoalRun.ID}})
	if err := server.handleStartGoalRun(stopCtx); err != nil {
		t.Fatalf("start cancel case failed: %v", err)
	}
	stopNowReq := httptest.NewRequest(http.MethodPost, "/api/goal-runs/"+canceledCreate.GoalRun.ID+"/stop", nil)
	stopNowRec := httptest.NewRecorder()
	stopNowCtx := e.NewContext(stopNowReq, stopNowRec)
	stopNowCtx.SetPath("/api/goal-runs/:id/stop")
	stopNowCtx.SetPathValues(echo.PathValues{{Name: "id", Value: canceledCreate.GoalRun.ID}})
	if err := server.handleStopGoalRun(stopNowCtx); err != nil {
		t.Fatalf("handleStopGoalRun failed: %v", err)
	}
	if stopNowRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", stopNowRec.Code, stopNowRec.Body.String())
	}

	cancelReq := httptest.NewRequest(http.MethodPost, "/api/goal-runs/"+canceledCreate.GoalRun.ID+"/cancel", nil)
	cancelRec := httptest.NewRecorder()
	cancelCtx := e.NewContext(cancelReq, cancelRec)
	cancelCtx.SetPath("/api/goal-runs/:id/cancel")
	cancelCtx.SetPathValues(echo.PathValues{{Name: "id", Value: canceledCreate.GoalRun.ID}})
	if err := server.handleCancelGoalRun(cancelCtx); err != nil {
		t.Fatalf("handleCancelGoalRun failed: %v", err)
	}
	if cancelRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", cancelRec.Code, cancelRec.Body.String())
	}
}

func testJWTToken(username string) *jwt.Token {
	return &jwt.Token{
		Claims: jwt.MapClaims{
			"sub": username,
		},
	}
}

func waitForGoalStatus(t *testing.T, svc *goaldriven.Service, goalRunID string, want goaldriven.GoalStatus) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		detail, ok, err := svc.GetGoalRunDetail(t.Context(), goalRunID)
		if err == nil && ok && detail.GoalRun.Status == want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	detail, _, _ := svc.GetGoalRunDetail(t.Context(), goalRunID)
	t.Fatalf("goal run %s never reached status %s, got %+v", goalRunID, want, detail.GoalRun)
}

func TestGoalRunRecoveryAcrossRealProcessRestart(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace")
	cfg.Gateway.Host = "127.0.0.1"
	cfg.Gateway.Port = mustPort(t, reserveGoalRunTCPAddr(t))
	cfg.WebUI.Port = mustPort(t, reserveGoalRunTCPAddr(t))

	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := config.SaveToFile(cfg, configPath); err != nil {
		t.Fatalf("SaveToFile failed: %v", err)
	}

	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	binPath := filepath.Join(t.TempDir(), "nekobot-goalrun-e2e")
	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/nekobot")
	buildCmd.Dir = repoRoot
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("build binary failed: %v\n%s", err, string(output))
	}

	startProcess := func(t *testing.T) *exec.Cmd {
		t.Helper()
		cmd := exec.Command(binPath, "--config", configPath, "gateway")
		cmd.Dir = repoRoot
		cmd.Env = append(os.Environ(), "HOME="+t.TempDir())
		if err := cmd.Start(); err != nil {
			t.Fatalf("start gateway process: %v", err)
		}
		waitForHTTPOK(t, "http://127.0.0.1:"+itoa(cfg.WebUI.Port)+"/api/auth/init-status", 15*time.Second)
		return cmd
	}

	stopProcess := func(t *testing.T, cmd *exec.Cmd) {
		t.Helper()
		if cmd == nil || cmd.Process == nil {
			return
		}
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}

	cmdA := startProcess(t)
	defer stopProcess(t, cmdA)

	token := initAdminAndGetToken(t, cfg.WebUI.Port)

	stopProcess(t, cmdA)
	cmdA = nil

	log, err := logger.New(&logger.Config{Level: logger.LevelInfo, Development: true})
	if err != nil {
		t.Fatalf("logger.New failed: %v", err)
	}
	loader := config.NewLoader()
	loadedCfg, err := loader.LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}
	kvStore, err := state.NewFileStore(log, &state.FileStoreConfig{
		FilePath: filepath.Join(loadedCfg.WorkspacePath(), "state.json"),
		AutoSave: true,
	})
	if err != nil {
		t.Fatalf("NewFileStore failed: %v", err)
	}
	goalStore := goaldriven.NewPersistentStore(kvStore)
	goalRunID := "gr-process-restart"
	now := time.Now().UTC()
	if _, err := goalStore.CreateGoalRun(t.Context(), goaldriven.GoalRun{
		ID:                      goalRunID,
		Name:                    "restart recovery e2e",
		Goal:                    "resume through real process restart",
		NaturalLanguageCriteria: "confirm manually",
		Status:                  goaldriven.GoalStatusVerifying,
		RiskLevel:               goaldriven.RiskBalanced,
		AllowAutoScope:          true,
		SelectedScope:           &goaldriven.ExecutionScope{Kind: goaldriven.ScopeServer, Source: "manual"},
		CreatedBy:               "admin",
		CreatedAt:               now,
		UpdatedAt:               now,
		StartedAt:               now,
	}); err != nil {
		t.Fatalf("CreateGoalRun failed: %v", err)
	}
	if err := goalStore.SaveCriteria(t.Context(), goalRunID, goaldrivencriteria.Set{
		Criteria: []goaldrivencriteria.Item{
			{
				ID:       "manual-1",
				Title:    "Manual confirmation",
				Type:     goaldrivencriteria.TypeManualConfirmation,
				Scope:    goaldriven.ExecutionScope{Kind: goaldriven.ScopeServer, Source: "manual"},
				Required: true,
				Status:   goaldrivencriteria.StatusPending,
				Definition: map[string]any{
					"prompt": "Confirm success",
				},
				UpdatedAt: now,
			},
		},
	}); err != nil {
		t.Fatalf("SaveCriteria failed: %v", err)
	}
	if err := kvStore.Close(); err != nil {
		t.Fatalf("Close kv store failed: %v", err)
	}

	cmdB := startProcess(t)
	defer stopProcess(t, cmdB)

	token = loginAndGetToken(t, cfg.WebUI.Port, "admin", "admin123456")
	waitForGoalRunStatusHTTP(t, cfg.WebUI.Port, token, goalRunID, string(goaldriven.GoalStatusNeedsHumanConfirmation), 15*time.Second)
	confirmManualCriterionHTTP(t, cfg.WebUI.Port, token, goalRunID, map[string]any{
		"criterion_id": "manual-1",
		"approved":     true,
		"note":         "restart recovery e2e confirmed",
	})
	waitForGoalRunStatusHTTP(t, cfg.WebUI.Port, token, goalRunID, string(goaldriven.GoalStatusCompleted), 15*time.Second)
}

func TestManualOnlyGoalRunHTTPFlowCompletesOnFreshProcess(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace")
	cfg.Gateway.Host = "127.0.0.1"
	cfg.Gateway.Port = mustPort(t, reserveGoalRunTCPAddr(t))
	cfg.WebUI.Port = mustPort(t, reserveGoalRunTCPAddr(t))

	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := config.SaveToFile(cfg, configPath); err != nil {
		t.Fatalf("SaveToFile failed: %v", err)
	}

	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	binPath := filepath.Join(t.TempDir(), "nekobot-goalrun-manual-flow")
	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/nekobot")
	buildCmd.Dir = repoRoot
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("build binary failed: %v\n%s", err, string(output))
	}

	cmd := exec.Command(binPath, "--config", configPath, "gateway")
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	if err := cmd.Start(); err != nil {
		t.Fatalf("start gateway process: %v", err)
	}
	defer func() {
		if cmd.Process == nil {
			return
		}
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()

	waitForHTTPOK(t, "http://127.0.0.1:"+itoa(cfg.WebUI.Port)+"/api/auth/init-status", 15*time.Second)
	token := initAdminAndGetToken(t, cfg.WebUI.Port)

	created := createGoalRunHTTP(t, cfg.WebUI.Port, token, map[string]any{
		"name":                      "manual-only e2e",
		"goal":                      "verify a manual-only Goal Run on a fresh process",
		"natural_language_criteria": "confirm manually",
		"risk_level":                "balanced",
		"allow_auto_scope":          true,
	})
	goalRunID := nestedString(t, created, "goal_run", "id")
	if goalRunID == "" {
		t.Fatalf("expected created goal run id, got %+v", created)
	}

	confirmGoalRunCriteriaHTTP(t, cfg.WebUI.Port, token, goalRunID, map[string]any{
		"criteria": map[string]any{
			"criteria": []map[string]any{
				{
					"id":       "manual-1",
					"title":    "Manual confirmation",
					"type":     "manual_confirmation",
					"required": true,
					"scope": map[string]any{
						"kind":   "server",
						"source": "manual",
					},
					"definition": map[string]any{
						"prompt": "Confirm success",
					},
				},
			},
		},
		"selected_scope": map[string]any{
			"kind":   "server",
			"source": "manual",
		},
	})

	startGoalRunHTTP(t, cfg.WebUI.Port, token, goalRunID)
	waitForGoalRunStatusHTTP(t, cfg.WebUI.Port, token, goalRunID, string(goaldriven.GoalStatusNeedsHumanConfirmation), 15*time.Second)

	detail := getJSON(t, "http://127.0.0.1:"+itoa(cfg.WebUI.Port)+"/api/goal-runs/"+goalRunID, token)
	workers, _ := detail["workers"].([]any)
	if len(workers) != 0 {
		t.Fatalf("expected manual-only flow to avoid workers, got %+v", workers)
	}

	confirmManualCriterionHTTP(t, cfg.WebUI.Port, token, goalRunID, map[string]any{
		"criterion_id": "manual-1",
		"approved":     true,
		"note":         "manual-only e2e confirmed",
	})
	waitForGoalRunStatusHTTP(t, cfg.WebUI.Port, token, goalRunID, string(goaldriven.GoalStatusCompleted), 15*time.Second)
}

func TestGoalRunBrowserToolManualFlowE2E(t *testing.T) {
	browserPath := tools.FindChromeForTesting()
	if browserPath == "" {
		t.Skip("browser e2e requires chromium or chrome in PATH")
	}

	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace")
	cfg.Gateway.Host = "127.0.0.1"
	cfg.Gateway.Port = mustPort(t, reserveGoalRunTCPAddr(t))
	cfg.WebUI.Port = mustPort(t, reserveGoalRunTCPAddr(t))

	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := config.SaveToFile(cfg, configPath); err != nil {
		t.Fatalf("SaveToFile failed: %v", err)
	}

	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	binPath := filepath.Join(t.TempDir(), "nekobot-goalrun-browser-e2e")
	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/nekobot")
	buildCmd.Dir = repoRoot
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("build binary failed: %v\n%s", err, string(output))
	}

	cmd := exec.Command(binPath, "--config", configPath, "gateway")
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	if err := cmd.Start(); err != nil {
		t.Fatalf("start gateway process: %v", err)
	}
	defer func() {
		if cmd.Process == nil {
			return
		}
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()

	waitForHTTPOK(t, "http://127.0.0.1:"+itoa(cfg.WebUI.Port)+"/api/auth/init-status", 15*time.Second)
	token := initAdminAndGetToken(t, cfg.WebUI.Port)

	tool := tools.NewBrowserTool(logOrFatal(t), true, 30, t.TempDir())
	if _, err := tool.Execute(t.Context(), map[string]interface{}{
		"action": "start_session",
		"mode":   "direct",
	}); err != nil {
		t.Fatalf("start browser session: %v", err)
	}
	defer func() {
		_, _ = tool.Execute(context.Background(), map[string]interface{}{"action": "close_session"})
	}()

	baseURL := "http://127.0.0.1:" + itoa(cfg.WebUI.Port)
	if _, err := tool.Execute(t.Context(), map[string]interface{}{
		"action": "navigate",
		"url":    baseURL + "/goal-runs",
	}); err != nil {
		t.Fatalf("navigate goal-runs: %v", err)
	}
	if _, err := tool.Execute(t.Context(), map[string]interface{}{
		"action": "execute_script",
		"script": fmt.Sprintf(`localStorage.setItem("nekobot_token", %q);`, token),
	}); err != nil {
		t.Fatalf("seed auth token: %v", err)
	}
	if _, err := tool.Execute(t.Context(), map[string]interface{}{
		"action": "navigate",
		"url":    baseURL + "/goal-runs",
	}); err != nil {
		t.Fatalf("reload goal-runs after auth: %v", err)
	}

	steps := []map[string]interface{}{
		{"action": "execute_script", "script": `Array.from(document.querySelectorAll('button')).find(btn => btn.textContent?.includes('New goal run'))?.click(); true;`},
		{"action": "execute_script", "script": `(function(){ const set = (selector, value) => { const el = document.querySelector(selector); if (!el) throw new Error('missing '+selector); el.value = value; el.dispatchEvent(new Event('input', { bubbles: true })); }; set('#goal-run-name', 'Browser tool e2e'); set('#goal-run-goal', 'Verify browser-tool Goal Runs flow'); set('#goal-run-criteria', 'confirm manually'); return true; })();`},
		{"action": "execute_script", "script": `Array.from(document.querySelectorAll('button')).find(btn => btn.textContent?.includes('Create run'))?.click(); true;`},
		{"action": "wait", "duration": float64(1200)},
		{"action": "execute_script", "script": `Array.from(document.querySelectorAll('button')).find(btn => btn.textContent?.includes('Confirm criteria'))?.click(); true;`},
		{"action": "wait", "duration": float64(600)},
		{"action": "execute_script", "script": `Array.from(document.querySelectorAll('button')).find(btn => btn.textContent?.includes('Start run'))?.click(); true;`},
		{"action": "wait", "duration": float64(1200)},
		{"action": "execute_script", "script": `(function(){ const areas = Array.from(document.querySelectorAll('textarea')); if (areas.length === 0) throw new Error('missing textarea'); const el = areas[areas.length - 1]; el.value = 'browser tool e2e confirmed'; el.dispatchEvent(new Event('input', { bubbles: true })); return true; })();`},
		{"action": "execute_script", "script": `Array.from(document.querySelectorAll('button')).find(btn => btn.textContent?.includes('Approve criterion'))?.click(); true;`},
		{"action": "wait", "duration": float64(1200)},
	}
	for _, step := range steps {
		if _, err := tool.Execute(t.Context(), step); err != nil {
			t.Fatalf("browser step failed (%v): %v", step, err)
		}
	}

	textOut, err := tool.Execute(t.Context(), map[string]interface{}{"action": "get_text"})
	if err != nil {
		t.Fatalf("get_text failed: %v", err)
	}
	if !strings.Contains(textOut, "Completed") {
		t.Fatalf("expected Completed in browser text, got %s", textOut)
	}
	if !strings.Contains(textOut, "Browser tool e2e") {
		t.Fatalf("expected created goal run name in browser text, got %s", textOut)
	}
}

func TestDaemonBackedGoalRunRecoveryAcrossRealProcessRestart(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace")
	cfg.Gateway.Host = "127.0.0.1"
	cfg.Gateway.Port = mustPort(t, reserveGoalRunTCPAddr(t))
	cfg.WebUI.Port = mustPort(t, reserveGoalRunTCPAddr(t))

	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := config.SaveToFile(cfg, configPath); err != nil {
		t.Fatalf("SaveToFile failed: %v", err)
	}

	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	binPath := filepath.Join(t.TempDir(), "nekobot-goalrun-daemon-e2e")
	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/nekobot")
	buildCmd.Dir = repoRoot
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("build binary failed: %v\n%s", err, string(output))
	}

	startGateway := func(t *testing.T) *exec.Cmd {
		t.Helper()
		cmd := exec.Command(binPath, "--config", configPath, "gateway")
		cmd.Dir = repoRoot
		cmd.Env = append(os.Environ(), "HOME="+t.TempDir())
		if err := cmd.Start(); err != nil {
			t.Fatalf("start gateway process: %v", err)
		}
		waitForHTTPOK(t, "http://127.0.0.1:"+itoa(cfg.WebUI.Port)+"/api/auth/init-status", 15*time.Second)
		return cmd
	}
	stopProcess := func(t *testing.T, cmd *exec.Cmd) {
		t.Helper()
		if cmd == nil || cmd.Process == nil {
			return
		}
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}

	cmdA := startGateway(t)
	defer stopProcess(t, cmdA)
	token := initAdminAndGetToken(t, cfg.WebUI.Port)
	bootstrap := getJSON(t, "http://127.0.0.1:"+itoa(cfg.WebUI.Port)+"/api/daemon/bootstrap", token)
	daemonToken := stringField(t, bootstrap, "daemon_token")

	registerDaemonMachineHTTP(t, cfg.WebUI.Port, daemonToken, map[string]any{
		"info": map[string]any{
			"daemon_id":      "daemon-goalrun",
			"machine_id":     "machine-goalrun",
			"machine_name":   "machine-goalrun",
			"status":         "online",
			"last_seen_unix": time.Now().Unix(),
			"daemon_url":     "http://127.0.0.1:9999",
		},
		"inventory": map[string]any{
			"workspaces": []map[string]any{
				{
					"workspace_id": "machine-goalrun:default",
					"machine_id":   "machine-goalrun",
					"path":         filepath.Join(t.TempDir(), "daemon-workspace"),
					"display_name": "default",
					"is_default":   true,
				},
			},
			"runtimes": []map[string]any{
				{
					"runtime_id":   "machine-goalrun:default:claude",
					"machine_id":   "machine-goalrun",
					"workspace_id": "machine-goalrun:default",
					"kind":         "claude",
					"display_name": "Claude",
					"installed":    true,
					"healthy":      true,
				},
			},
		},
	})
	waitForDaemonMachineHTTP(t, cfg.WebUI.Port, token, "machine-goalrun", 15*time.Second)

	stopProcess(t, cmdA)
	cmdA = nil

	log, err := logger.New(&logger.Config{Level: logger.LevelInfo, Development: true})
	if err != nil {
		t.Fatalf("logger.New failed: %v", err)
	}
	loader := config.NewLoader()
	loadedCfg, err := loader.LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}
	kvStore, err := state.NewFileStore(log, &state.FileStoreConfig{
		FilePath: filepath.Join(loadedCfg.WorkspacePath(), "state.json"),
		AutoSave: true,
	})
	if err != nil {
		t.Fatalf("NewFileStore failed: %v", err)
	}
	goalStore := goaldriven.NewPersistentStore(kvStore)
	goalRunID := "gr-daemon-process-restart"
	now := time.Now().UTC()
	if _, err := goalStore.CreateGoalRun(t.Context(), goaldriven.GoalRun{
		ID:                      goalRunID,
		Name:                    "daemon restart recovery e2e",
		Goal:                    "resume daemon-backed goal through real process restart",
		NaturalLanguageCriteria: "confirm manually",
		Status:                  goaldriven.GoalStatusVerifying,
		RiskLevel:               goaldriven.RiskBalanced,
		AllowAutoScope:          true,
		SelectedScope:           &goaldriven.ExecutionScope{Kind: goaldriven.ScopeDaemon, MachineID: "machine-goalrun", Source: "manual"},
		CreatedBy:               "admin",
		CreatedAt:               now,
		UpdatedAt:               now,
		StartedAt:               now,
	}); err != nil {
		t.Fatalf("CreateGoalRun failed: %v", err)
	}
	if err := goalStore.SaveCriteria(t.Context(), goalRunID, goaldrivencriteria.Set{
		Criteria: []goaldrivencriteria.Item{
			{
				ID:       "manual-1",
				Title:    "Manual confirmation",
				Type:     goaldrivencriteria.TypeManualConfirmation,
				Scope:    goaldriven.ExecutionScope{Kind: goaldriven.ScopeDaemon, MachineID: "machine-goalrun", Source: "manual"},
				Required: true,
				Status:   goaldrivencriteria.StatusPending,
				Definition: map[string]any{
					"prompt": "Confirm success",
				},
				UpdatedAt: now,
			},
		},
	}); err != nil {
		t.Fatalf("SaveCriteria failed: %v", err)
	}
	if err := kvStore.Close(); err != nil {
		t.Fatalf("Close kv store failed: %v", err)
	}

	cmdB := startGateway(t)
	defer stopProcess(t, cmdB)

	token = loginAndGetToken(t, cfg.WebUI.Port, "admin", "admin123456")
	waitForGoalRunStatusHTTP(t, cfg.WebUI.Port, token, goalRunID, string(goaldriven.GoalStatusNeedsHumanConfirmation), 15*time.Second)
	confirmManualCriterionHTTP(t, cfg.WebUI.Port, token, goalRunID, map[string]any{
		"criterion_id": "manual-1",
		"approved":     true,
		"note":         "daemon restart recovery e2e confirmed",
	})
	waitForGoalRunStatusHTTP(t, cfg.WebUI.Port, token, goalRunID, string(goaldriven.GoalStatusCompleted), 15*time.Second)
}

func logOrFatal(t *testing.T) *logger.Logger {
	t.Helper()
	log, err := logger.New(&logger.Config{Level: logger.LevelInfo, Development: true})
	if err != nil {
		t.Fatalf("logger.New failed: %v", err)
	}
	return log
}

func reserveGoalRunTCPAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve tcp addr: %v", err)
	}
	defer ln.Close()
	return ln.Addr().String()
}

func mustPort(t *testing.T, addr string) int {
	t.Helper()
	_, portText, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	var port int
	if _, err := fmt.Sscanf(portText, "%d", &port); err != nil {
		t.Fatalf("parse port: %v", err)
	}
	return port
}

func itoa(v int) string {
	return fmt.Sprintf("%d", v)
}

func waitForHTTPOK(t *testing.T, url string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("endpoint never became ready: %s", url)
}

func initAdminAndGetToken(t *testing.T, webuiPort int) string {
	t.Helper()
	body := map[string]any{
		"username": "admin",
		"password": "admin123456",
	}
	resp := postJSON(t, "http://127.0.0.1:"+itoa(webuiPort)+"/api/auth/init", "", body)
	return stringField(t, resp, "token")
}

func loginAndGetToken(t *testing.T, webuiPort int, username, password string) string {
	t.Helper()
	resp := postJSON(t, "http://127.0.0.1:"+itoa(webuiPort)+"/api/auth/login", "", map[string]any{
		"username": username,
		"password": password,
	})
	return stringField(t, resp, "token")
}

func createGoalRunHTTP(t *testing.T, webuiPort int, token string, body map[string]any) map[string]any {
	t.Helper()
	return postJSON(t, "http://127.0.0.1:"+itoa(webuiPort)+"/api/goal-runs", token, body)
}

func confirmGoalRunCriteriaHTTP(t *testing.T, webuiPort int, token, goalRunID string, body map[string]any) map[string]any {
	t.Helper()
	return postJSON(t, "http://127.0.0.1:"+itoa(webuiPort)+"/api/goal-runs/"+goalRunID+"/confirm-criteria", token, body)
}

func startGoalRunHTTP(t *testing.T, webuiPort int, token, goalRunID string) map[string]any {
	t.Helper()
	return postJSON(t, "http://127.0.0.1:"+itoa(webuiPort)+"/api/goal-runs/"+goalRunID+"/start", token, map[string]any{})
}

func confirmManualCriterionHTTP(t *testing.T, webuiPort int, token, goalRunID string, body map[string]any) map[string]any {
	t.Helper()
	return postJSON(t, "http://127.0.0.1:"+itoa(webuiPort)+"/api/goal-runs/"+goalRunID+"/confirm-manual", token, body)
}

func waitForGoalRunStatusHTTP(t *testing.T, webuiPort int, token, goalRunID, want string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp := getJSON(t, "http://127.0.0.1:"+itoa(webuiPort)+"/api/goal-runs/"+goalRunID, token)
		got := nestedString(t, resp, "goal_run", "status")
		if got == want {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("goal run %s never reached status %s", goalRunID, want)
}

func waitForDaemonMachineHTTP(t *testing.T, webuiPort int, token, machineID string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp := getJSON(t, "http://127.0.0.1:"+itoa(webuiPort)+"/api/status", token)
		items, _ := resp["daemon_machines"].([]any)
		for _, item := range items {
			machine, _ := item.(map[string]any)
			info, _ := machine["info"].(map[string]any)
			gotID, _ := info["machine_id"].(string)
			if gotID == machineID {
				return
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("daemon machine %s never appeared in status", machineID)
}

func registerDaemonMachineHTTP(t *testing.T, webuiPort int, daemonToken string, body map[string]any) {
	t.Helper()
	postJSON(t, "http://127.0.0.1:"+itoa(webuiPort)+"/api/daemon/register", daemonToken, body)
}

func postJSON(t *testing.T, url, token string, body map[string]any) map[string]any {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post %s failed: %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		var text bytes.Buffer
		_, _ = text.ReadFrom(resp.Body)
		t.Fatalf("post %s status %d: %s", url, resp.StatusCode, text.String())
	}
	var decoded map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return decoded
}

func getJSON(t *testing.T, url, token string) map[string]any {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get %s failed: %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		var text bytes.Buffer
		_, _ = text.ReadFrom(resp.Body)
		t.Fatalf("get %s status %d: %s", url, resp.StatusCode, text.String())
	}
	var decoded map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return decoded
}

func stringField(t *testing.T, payload map[string]any, key string) string {
	t.Helper()
	value, _ := payload[key].(string)
	if strings.TrimSpace(value) == "" {
		t.Fatalf("expected string field %q in payload: %+v", key, payload)
	}
	return value
}

func nestedString(t *testing.T, payload map[string]any, path ...string) string {
	t.Helper()
	var current any = payload
	for _, key := range path {
		node, ok := current.(map[string]any)
		if !ok {
			t.Fatalf("expected map at %q in payload: %+v", key, payload)
		}
		current = node[key]
	}
	value, _ := current.(string)
	if strings.TrimSpace(value) == "" {
		t.Fatalf("expected non-empty string at %v in payload: %+v", path, payload)
	}
	return value
}
