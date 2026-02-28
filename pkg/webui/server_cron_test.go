package webui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"

	"nekobot/pkg/config"
	"nekobot/pkg/cron"
)

func TestCronHandlers_RequireCronManager(t *testing.T) {
	s := &Server{}
	e := echo.New()

	tests := []struct {
		name   string
		method string
		path   string
		target string
		call   func(*Server, *echo.Context) error
	}{
		{
			name:   "list",
			method: http.MethodGet,
			path:   "/api/cron/jobs",
			target: "/api/cron/jobs",
			call:   (*Server).handleListCronJobs,
		},
		{
			name:   "create",
			method: http.MethodPost,
			path:   "/api/cron/jobs",
			target: "/api/cron/jobs",
			call:   (*Server).handleCreateCronJob,
		},
		{
			name:   "delete",
			method: http.MethodDelete,
			path:   "/api/cron/jobs/:id",
			target: "/api/cron/jobs/job-1",
			call:   (*Server).handleDeleteCronJob,
		},
		{
			name:   "enable",
			method: http.MethodPost,
			path:   "/api/cron/jobs/:id/enable",
			target: "/api/cron/jobs/job-1/enable",
			call:   (*Server).handleEnableCronJob,
		},
		{
			name:   "disable",
			method: http.MethodPost,
			path:   "/api/cron/jobs/:id/disable",
			target: "/api/cron/jobs/job-1/disable",
			call:   (*Server).handleDisableCronJob,
		},
		{
			name:   "run",
			method: http.MethodPost,
			path:   "/api/cron/jobs/:id/run",
			target: "/api/cron/jobs/job-1/run",
			call:   (*Server).handleRunCronJob,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.target, nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.SetPath(tc.path)
			if strings.Contains(tc.path, ":id") {
				c.SetPathValues(echo.PathValues{{Name: "id", Value: "job-1"}})
			}

			if err := tc.call(s, c); err != nil {
				t.Fatalf("handler returned error: %v", err)
			}
			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
			}
		})
	}
}

func TestCronHandlers_Flow(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	defer client.Close()

	manager := cron.New(log, nil, client)
	s := &Server{
		config:  cfg,
		logger:  log,
		cronMgr: manager,
	}
	e := echo.New()

	createBody := `{"name":"cron-job","schedule_kind":"cron","schedule":"*/5 * * * *","prompt":"hello cron"}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/cron/jobs", strings.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	createCtx := e.NewContext(createReq, createRec)
	if err := s.handleCreateCronJob(createCtx); err != nil {
		t.Fatalf("handleCreateCronJob failed: %v", err)
	}
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, createRec.Code, createRec.Body.String())
	}

	var createdResp struct {
		Status string    `json:"status"`
		Job    *cron.Job `json:"job"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &createdResp); err != nil {
		t.Fatalf("unmarshal create response: %v", err)
	}
	if createdResp.Job == nil || createdResp.Job.ID == "" {
		t.Fatalf("expected created job id, got: %s", createRec.Body.String())
	}
	if createdResp.Status != "created" {
		t.Fatalf("expected created status, got %q", createdResp.Status)
	}
	jobID := createdResp.Job.ID

	listReq := httptest.NewRequest(http.MethodGet, "/api/cron/jobs", nil)
	listRec := httptest.NewRecorder()
	listCtx := e.NewContext(listReq, listRec)
	if err := s.handleListCronJobs(listCtx); err != nil {
		t.Fatalf("handleListCronJobs failed: %v", err)
	}
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, listRec.Code)
	}
	var jobs []*cron.Job
	if err := json.Unmarshal(listRec.Body.Bytes(), &jobs); err != nil {
		t.Fatalf("unmarshal list response: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected one job, got %d", len(jobs))
	}
	if jobs[0].ID != jobID {
		t.Fatalf("expected listed job id %q, got %q", jobID, jobs[0].ID)
	}

	disableReq := httptest.NewRequest(http.MethodPost, "/api/cron/jobs/"+jobID+"/disable", nil)
	disableRec := httptest.NewRecorder()
	disableCtx := e.NewContext(disableReq, disableRec)
	disableCtx.SetPath("/api/cron/jobs/:id/disable")
	disableCtx.SetPathValues(echo.PathValues{{Name: "id", Value: jobID}})
	if err := s.handleDisableCronJob(disableCtx); err != nil {
		t.Fatalf("handleDisableCronJob failed: %v", err)
	}
	if disableRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, disableRec.Code)
	}

	jobAfterDisable, err := manager.GetJob(jobID)
	if err != nil {
		t.Fatalf("get job after disable: %v", err)
	}
	if jobAfterDisable.Enabled {
		t.Fatalf("expected disabled job")
	}

	runReq := httptest.NewRequest(http.MethodPost, "/api/cron/jobs/"+jobID+"/run", nil)
	runRec := httptest.NewRecorder()
	runCtx := e.NewContext(runReq, runRec)
	runCtx.SetPath("/api/cron/jobs/:id/run")
	runCtx.SetPathValues(echo.PathValues{{Name: "id", Value: jobID}})
	if err := s.handleRunCronJob(runCtx); err != nil {
		t.Fatalf("handleRunCronJob failed: %v", err)
	}
	if runRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, runRec.Code)
	}

	jobAfterRun, err := manager.GetJob(jobID)
	if err != nil {
		t.Fatalf("get job after run-now: %v", err)
	}
	if jobAfterRun.RunCount != 0 {
		t.Fatalf("expected run-now to skip disabled job, run_count=%d", jobAfterRun.RunCount)
	}

	enableReq := httptest.NewRequest(http.MethodPost, "/api/cron/jobs/"+jobID+"/enable", nil)
	enableRec := httptest.NewRecorder()
	enableCtx := e.NewContext(enableReq, enableRec)
	enableCtx.SetPath("/api/cron/jobs/:id/enable")
	enableCtx.SetPathValues(echo.PathValues{{Name: "id", Value: jobID}})
	if err := s.handleEnableCronJob(enableCtx); err != nil {
		t.Fatalf("handleEnableCronJob failed: %v", err)
	}
	if enableRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, enableRec.Code)
	}

	jobAfterEnable, err := manager.GetJob(jobID)
	if err != nil {
		t.Fatalf("get job after enable: %v", err)
	}
	if !jobAfterEnable.Enabled {
		t.Fatalf("expected enabled job")
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/cron/jobs/"+jobID, nil)
	deleteRec := httptest.NewRecorder()
	deleteCtx := e.NewContext(deleteReq, deleteRec)
	deleteCtx.SetPath("/api/cron/jobs/:id")
	deleteCtx.SetPathValues(echo.PathValues{{Name: "id", Value: jobID}})
	if err := s.handleDeleteCronJob(deleteCtx); err != nil {
		t.Fatalf("handleDeleteCronJob failed: %v", err)
	}
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, deleteRec.Code)
	}

	if _, err := manager.GetJob(jobID); err == nil || !strings.Contains(err.Error(), "job not found") {
		t.Fatalf("expected removed job error, got %v", err)
	}
}

func TestHandleCreateCronJob_AtScheduleValidatesRFC3339(t *testing.T) {
	s := &Server{cronMgr: cron.New(newTestLogger(t), nil, nil)}
	e := echo.New()

	body := `{"name":"once","schedule_kind":"at","at_time":"invalid","prompt":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/api/cron/jobs", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := s.handleCreateCronJob(c); err != nil {
		t.Fatalf("handleCreateCronJob failed: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "invalid at_time") {
		t.Fatalf("expected invalid at_time error, got %s", rec.Body.String())
	}
}

func TestHandleRunCronJob_NotFound(t *testing.T) {
	s := &Server{cronMgr: cron.New(newTestLogger(t), nil, nil)}
	e := echo.New()

	req := httptest.NewRequest(http.MethodPost, "/api/cron/jobs/missing/run", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/cron/jobs/:id/run")
	c.SetPathValues(echo.PathValues{{Name: "id", Value: "missing"}})

	if err := s.handleRunCronJob(c); err != nil {
		t.Fatalf("handleRunCronJob failed: %v", err)
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNotFound, rec.Code, rec.Body.String())
	}
}

func TestHandleRunCronJob_DisabledJobDoesNotExecute(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	defer client.Close()

	manager := cron.New(log, nil, client)
	job, err := manager.AddCronJob("disabled-job", "*/5 * * * *", "hello")
	if err != nil {
		t.Fatalf("add cron job: %v", err)
	}
	if err := manager.DisableJob(job.ID); err != nil {
		t.Fatalf("disable job: %v", err)
	}

	s := &Server{cronMgr: manager}
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/cron/jobs/"+job.ID+"/run", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/cron/jobs/:id/run")
	c.SetPathValues(echo.PathValues{{Name: "id", Value: job.ID}})

	if err := s.handleRunCronJob(c); err != nil {
		t.Fatalf("handleRunCronJob failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	updated, err := manager.GetJob(job.ID)
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if updated.RunCount != 0 {
		t.Fatalf("expected disabled run_count to remain 0, got %d", updated.RunCount)
	}
}
