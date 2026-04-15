package webui

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v5"

	"nekobot/pkg/config"
	"nekobot/pkg/goaldriven"
	goaldrivencriteria "nekobot/pkg/goaldriven/criteria"
	goaldrivenscope "nekobot/pkg/goaldriven/scope"
	"nekobot/pkg/logger"
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
