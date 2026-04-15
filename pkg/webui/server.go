// Package webui provides a web-based dashboard for nekobot.
// It uses Echo v5 for HTTP routing with JWT authentication,
// and serves an embedded SPA frontend for configuration management
// and chat playground.
package webui

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	echojwt "github.com/labstack/echo-jwt/v5"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"go.uber.org/zap"

	daemonv1 "nekobot/gen/go/nekobot/daemon/v1"
	"nekobot/pkg/accountbindings"
	"nekobot/pkg/agent"
	"nekobot/pkg/approval"
	"nekobot/pkg/audit"
	"nekobot/pkg/bus"
	"nekobot/pkg/channelaccounts"
	"nekobot/pkg/channels"
	channelwechat "nekobot/pkg/channels/wechat"
	"nekobot/pkg/commands"
	"nekobot/pkg/config"
	"nekobot/pkg/cron"
	"nekobot/pkg/daemonhost"
	"nekobot/pkg/execenv"
	"nekobot/pkg/externalagent"
	"nekobot/pkg/gateway"
	"nekobot/pkg/ilinkauth"
	"nekobot/pkg/inboundrouter"
	"nekobot/pkg/logger"
	memoryqmd "nekobot/pkg/memory/qmd"
	"nekobot/pkg/modelroute"
	"nekobot/pkg/modelstore"
	"nekobot/pkg/permissionrules"
	"nekobot/pkg/policy"
	"nekobot/pkg/process"
	"nekobot/pkg/prompts"
	"nekobot/pkg/providerregistry"
	"nekobot/pkg/providers"
	"nekobot/pkg/providerstore"
	"nekobot/pkg/runtimeagents"
	"nekobot/pkg/runtimetopology"
	"nekobot/pkg/servicecontrol"
	"nekobot/pkg/session"
	"nekobot/pkg/skills"
	"nekobot/pkg/state"
	"nekobot/pkg/storage/ent"
	"nekobot/pkg/tasks"
	"nekobot/pkg/threads"
	"nekobot/pkg/toolsessions"
	"nekobot/pkg/userprefs"
	"nekobot/pkg/version"
	"nekobot/pkg/watch"
	"nekobot/pkg/webui/frontend"
	wxauth "nekobot/pkg/wechat/auth"
	wxtypes "nekobot/pkg/wechat/types"
	"nekobot/pkg/workspace"
	rscqr "rsc.io/qr"
)

// Server is the WebUI HTTP server.
type Server struct {
	echo               *echo.Echo
	httpServer         *http.Server
	config             *config.Config
	loader             *config.Loader
	logger             *logger.Logger
	agent              *agent.Agent
	approval           *approval.Manager
	channels           *channels.Manager
	bus                bus.Bus
	commands           *commands.Registry
	prefs              *userprefs.Manager
	toolSess           *toolsessions.Manager
	externalAgent      *externalagent.Manager
	sessionMgr         *session.Manager
	processMgr         *process.Manager
	prompts            *prompts.Manager
	providers          *providerstore.Manager
	runtimeMgr         *runtimeagents.Manager
	accountMgr         *channelaccounts.Manager
	bindingMgr         *accountbindings.Manager
	topologySvc        *runtimetopology.Service
	cronMgr            *cron.Manager
	skillsMgr          *skills.Manager
	workspace          *workspace.Manager
	entClient          *ent.Client
	snapshotMgr        *session.SnapshotManager
	auditLogger        *audit.Logger
	ilinkAuth          *ilinkauth.Service
	serviceCtrl        serviceController
	taskStore          *tasks.Store
	kvStore            state.KV
	threads            *threads.Manager
	chatEventMu        sync.RWMutex
	chatEventSubs      map[string]map[chan chatEvent]struct{}
	watcher            *watch.Watcher
	webhookTestHandler func(ctx context.Context, username, message string) (string, error)
	port               int
	startedAt          time.Time
}

type chatEvent struct {
	SessionID string `json:"session_id"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	Timestamp int64  `json:"timestamp"`
}

type serviceController interface {
	Status() (map[string]interface{}, error)
	Restart() error
	Reload() error
}

type gatewayServiceController struct {
	cfg        *config.Config
	loader     *config.Loader
	log        *logger.Logger
	configPath string
}

func (c *gatewayServiceController) Status() (map[string]interface{}, error) {
	status, err := servicecontrol.InspectGatewayService(c.configPath)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"name":        status.Name,
		"platform":    status.Platform,
		"config_path": status.ConfigPath,
		"arguments":   status.Arguments,
		"installed":   status.Installed,
		"status":      status.Status,
	}, nil
}

func (c *gatewayServiceController) Restart() error {
	return servicecontrol.RestartGatewayService(c.configPath)
}

func (c *gatewayServiceController) Reload() error {
	if c.cfg == nil || c.loader == nil || c.log == nil {
		return fmt.Errorf("gateway reload is not available")
	}
	return gateway.NewController(c.cfg, c.loader, c.log).ReloadConfig()
}

const defaultQMDNPMPackage = "@tobilu/qmd"

var defaultNPMCommand = "npm"

// NewServer creates a new WebUI server.
func NewServer(
	cfg *config.Config,
	loader *config.Loader,
	log *logger.Logger,
	ag *agent.Agent,
	approvalMgr *approval.Manager,
	chanMgr *channels.Manager,
	messageBus bus.Bus,
	cmdRegistry *commands.Registry,
	prefsMgr *userprefs.Manager,
	toolSessionMgr *toolsessions.Manager,
	sessionMgr *session.Manager,
	processManager *process.Manager,
	promptManager *prompts.Manager,
	providerStore *providerstore.Manager,
	cronManager *cron.Manager,
	skillsManager *skills.Manager,
	workspaceManager *workspace.Manager,
	kvStore state.KV,
	entClient *ent.Client,
	auditLogger *audit.Logger,
	watcher *watch.Watcher,
) *Server {
	port := cfg.WebUI.Port
	if port == 0 {
		port = cfg.Gateway.Port + 1
	}

	// Validate auth storage connectivity during startup.
	if _, err := config.LoadAdminCredential(entClient); err != nil {
		log.Warn("Failed to load admin credential from database", zap.Error(err))
	}

	s := &Server{
		config:   cfg,
		loader:   loader,
		logger:   log,
		agent:    ag,
		approval: approvalMgr,
		channels: chanMgr,
		bus:      messageBus,
		commands: cmdRegistry,
		prefs:    prefsMgr,
		toolSess: toolSessionMgr,
		externalAgent: func() *externalagent.Manager {
			if toolSessionMgr == nil {
				return nil
			}
			manager, err := externalagent.NewManager(cfg, toolSessionMgr)
			if err != nil {
				log.Warn("Failed to initialize external agent manager", zap.Error(err))
				return nil
			}
			return manager
		}(),
		sessionMgr:    sessionMgr,
		processMgr:    processManager,
		prompts:       promptManager,
		providers:     providerStore,
		cronMgr:       cronManager,
		skillsMgr:     skillsManager,
		workspace:     workspaceManager,
		kvStore:       kvStore,
		threads:       threads.NewManager(kvStore),
		chatEventSubs: map[string]map[chan chatEvent]struct{}{},
		entClient:     entClient,
		auditLogger:   auditLogger,
		snapshotMgr: func() *session.SnapshotManager {
			if ag == nil {
				return nil
			}
			return ag.SnapshotManager()
		}(),
		watcher: watcher,
		serviceCtrl: &gatewayServiceController{
			cfg:    cfg,
			loader: loader,
			log:    log,
			configPath: func() string {
				if loader == nil {
					return ""
				}
				return loader.GetConfigPath()
			}(),
		},
		port:      port,
		startedAt: time.Now(),
	}
	if ag != nil {
		s.taskStore = ag.TaskStore()
	}

	if entClient != nil {
		runtimeMgr, err := runtimeagents.NewManager(cfg, log, entClient)
		if err != nil {
			log.Warn("Failed to initialize runtime agent manager", zap.Error(err))
		} else {
			s.runtimeMgr = runtimeMgr
		}

		accountMgr, err := channelaccounts.NewManager(cfg, log, entClient)
		if err != nil {
			log.Warn("Failed to initialize channel account manager", zap.Error(err))
		} else {
			s.accountMgr = accountMgr
		}

		if s.runtimeMgr != nil && s.accountMgr != nil {
			bindingMgr, err := accountbindings.NewManager(cfg, log, entClient, s.runtimeMgr, s.accountMgr)
			if err != nil {
				log.Warn("Failed to initialize account binding manager", zap.Error(err))
			} else {
				s.bindingMgr = bindingMgr
			}
		}

		if s.runtimeMgr != nil && s.accountMgr != nil && s.bindingMgr != nil {
			topologySvc, err := runtimetopology.NewService(s.runtimeMgr, s.accountMgr, s.bindingMgr, s.taskStore)
			if err != nil {
				log.Warn("Failed to initialize runtime topology service", zap.Error(err))
			} else {
				s.topologySvc = topologySvc
			}
		}
	}

	ilinkStore, err := ilinkauth.NewStore(cfg)
	if err != nil {
		log.Warn("Failed to create iLink auth store", zap.Error(err))
	} else {
		s.ilinkAuth = ilinkauth.NewService(ilinkStore, webWechatLoginClient{})
	}

	s.setup()
	return s
}

func (s *Server) setup() {
	e := echo.New()

	// Middleware
	e.Use(middleware.Recover())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete},
	}))

	// Public routes
	e.POST("/api/auth/login", s.handleLogin)
	e.GET("/api/auth/init-status", s.handleInitStatus)
	e.POST("/api/auth/init", s.handleInitPassword)
	e.POST("/api/auth/init/repair-workspace", s.handleInitRepairWorkspace)

	// Chat WebSocket (auth handled inside via token query param)
	e.GET("/api/chat/ws", s.handleChatWS)
	e.GET("/api/chat/events", s.handleChatEvents)
	e.GET("/api/tool-sessions/ws", s.handleToolSessionWS)
	e.POST("/api/tool-sessions/access-login", s.handleToolSessionAccessLogin)

	// Protected API routes
	api := e.Group("/api")
	api.Use(echojwt.WithConfig(echojwt.Config{
		SigningKey: nil, // Use KeyFunc instead for dynamic secret.
		KeyFunc: func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method")
			}
			return []byte(s.getJWTSecret()), nil
		},
	}))

	// Provider routes
	api.GET("/provider-types", s.handleGetProviderTypes)
	api.GET("/providers", s.handleGetProviders)
	api.GET("/providers/runtime", s.handleGetProviderRuntime)
	api.POST("/providers", s.handleCreateProvider)
	api.POST("/providers/discover-models", s.handleDiscoverProviderModels)
	api.PUT("/providers/:name", s.handleUpdateProvider)
	api.DELETE("/providers/:name", s.handleDeleteProvider)
	api.GET("/models", s.handleGetModels)
	api.POST("/models", s.handleCreateModel)
	api.GET("/model-routes", s.handleGetModelRoutes)
	api.PUT("/model-routes/:modelID/:providerName", s.handleUpdateModelRoute)
	api.GET("/permission-rules", s.handleGetPermissionRules)
	api.POST("/permission-rules", s.handleCreatePermissionRule)
	api.PUT("/permission-rules/:id", s.handleUpdatePermissionRule)
	api.DELETE("/permission-rules/:id", s.handleDeletePermissionRule)
	api.GET("/policy/presets", s.handleGetPolicyPresets)
	api.POST("/policy/evaluate", s.handleEvaluatePolicy)

	// Channel routes
	api.GET("/channels", s.handleGetChannels)
	api.PUT("/channels/:name", s.handleUpdateChannel)
	api.POST("/channels/:name/test", s.handleTestChannel)
	api.GET("/channels/wechat/binding", s.handleGetWechatBindingStatus)
	api.POST("/channels/wechat/binding/start", s.handleStartWechatBinding)
	api.POST("/channels/wechat/binding/poll", s.handlePollWechatBinding)
	api.POST("/channels/wechat/binding/activate", s.handleActivateWechatBinding)
	api.DELETE("/channels/wechat/binding/accounts/:accountId", s.handleDeleteWechatBindingAccount)
	api.DELETE("/channels/wechat/binding", s.handleDeleteWechatBinding)

	// Config routes
	api.GET("/config", s.handleGetConfig)
	api.PUT("/config", s.handleSaveConfig)
	api.GET("/config/export", s.handleExportConfig)
	api.POST("/config/import", s.handleImportConfig)
	api.GET("/memory/qmd/status", s.handleGetQMDStatus)
	api.POST("/memory/qmd/install", s.handleInstallQMD)
	api.POST("/memory/qmd/update", s.handleUpdateQMD)
	api.POST("/memory/qmd/sessions/cleanup", s.handleCleanupQMDSessionExports)

	// Prompt routes
	api.GET("/prompts", s.handleListPrompts)
	api.POST("/prompts", s.handleCreatePrompt)
	api.PUT("/prompts/:id", s.handleUpdatePrompt)
	api.DELETE("/prompts/:id", s.handleDeletePrompt)
	api.GET("/prompts/bindings", s.handleListPromptBindings)
	api.POST("/prompts/bindings", s.handleCreatePromptBinding)
	api.PUT("/prompts/bindings/:id", s.handleUpdatePromptBinding)
	api.DELETE("/prompts/bindings/:id", s.handleDeletePromptBinding)
	api.POST("/prompts/resolve", s.handleResolvePrompts)
	api.POST("/prompts/context-sources", s.handlePreviewContextSources)
	api.GET("/chat/prompts/session/:id", s.handleGetChatSessionPrompts)
	api.PUT("/chat/prompts/session/:id", s.handlePutChatSessionPrompts)
	api.DELETE("/chat/prompts/session/:id", s.handleDeleteChatSessionPrompts)
	api.POST("/chat/session/:id/undo", s.handleUndoChatSession)

	// Multi-runtime foundation routes.
	api.GET("/runtime-agents", s.handleListRuntimeAgents)
	api.POST("/runtime-agents", s.handleCreateRuntimeAgent)
	api.PUT("/runtime-agents/:id", s.handleUpdateRuntimeAgent)
	api.DELETE("/runtime-agents/:id", s.handleDeleteRuntimeAgent)
	api.GET("/channel-accounts", s.handleListChannelAccounts)
	api.POST("/channel-accounts", s.handleCreateChannelAccount)
	api.PUT("/channel-accounts/:id", s.handleUpdateChannelAccount)
	api.DELETE("/channel-accounts/:id", s.handleDeleteChannelAccount)
	api.GET("/account-bindings", s.handleListAccountBindings)
	api.POST("/account-bindings", s.handleCreateAccountBinding)
	api.PUT("/account-bindings/:id", s.handleUpdateAccountBinding)
	api.DELETE("/account-bindings/:id", s.handleDeleteAccountBinding)
	api.GET("/runtime-topology", s.handleGetRuntimeTopology)

	// Status
	api.GET("/status", s.handleStatus)
	api.GET("/service", s.handleServiceStatus)
	api.POST("/service/restart", s.handleServiceRestart)
	api.POST("/service/reload", s.handleServiceReload)
	api.GET("/harness/watch", s.handleGetWatchStatus)
	api.POST("/harness/watch", s.handleUpdateWatchStatus)
	api.GET("/harness/audit", s.handleGetHarnessAudit)
	api.POST("/harness/audit/clear", s.handleClearHarnessAudit)

	// Cron routes
	api.GET("/cron/jobs", s.handleListCronJobs)
	api.POST("/cron/jobs", s.handleCreateCronJob)
	api.DELETE("/cron/jobs/:id", s.handleDeleteCronJob)
	api.POST("/cron/jobs/:id/enable", s.handleEnableCronJob)
	api.POST("/cron/jobs/:id/disable", s.handleDisableCronJob)
	api.POST("/cron/jobs/:id/run", s.handleRunCronJob)

	// Session routes
	api.GET("/sessions", s.handleListSessions)
	api.GET("/sessions/:id", s.handleGetSession)
	api.PUT("/sessions/:id/summary", s.handleUpdateSessionSummary)
	api.PUT("/sessions/:id/runtime", s.handleUpdateSessionRuntime)
	api.PUT("/sessions/:id/thread", s.handleUpdateSessionThread)
	api.DELETE("/sessions/:id", s.handleDeleteSession)
	api.GET("/threads", s.handleListThreads)
	api.GET("/threads/:id", s.handleGetThread)
	api.PUT("/threads/:id", s.handleUpdateThread)
	api.POST("/sessions/cleanup", s.handleCleanupSessions)

	// Marketplace routes
	api.GET("/marketplace/skills", s.handleListMarketplaceSkills)
	api.GET("/marketplace/skills/installed", s.handleListInstalledMarketplaceSkills)
	api.GET("/marketplace/skills/search", s.handleSearchMarketplaceSkills)
	api.GET("/marketplace/skills/items/:id", s.handleGetMarketplaceSkillItem)
	api.GET("/marketplace/skills/items/:id/content", s.handleGetMarketplaceSkillContent)
	api.POST("/marketplace/skills/install", s.handleInstallMarketplaceSkill)
	api.GET("/marketplace/skills/inventory", s.handleGetMarketplaceInventory)
	api.GET("/marketplace/skills/snapshots", s.handleListMarketplaceSkillSnapshots)
	api.POST("/marketplace/skills/snapshots", s.handleCreateMarketplaceSkillSnapshot)
	api.POST("/marketplace/skills/snapshots/prune", s.handlePruneMarketplaceSkillSnapshots)
	api.POST("/marketplace/skills/versions/cleanup", s.handleCleanupMarketplaceSkillVersions)
	api.POST("/marketplace/skills/snapshots/:id/restore", s.handleRestoreMarketplaceSkillSnapshot)
	api.DELETE("/marketplace/skills/snapshots/:id", s.handleDeleteMarketplaceSkillSnapshot)
	api.POST("/marketplace/skills/:id/enable", s.handleEnableMarketplaceSkill)
	api.POST("/marketplace/skills/:id/disable", s.handleDisableMarketplaceSkill)
	api.POST("/marketplace/skills/:id/install-deps", s.handleInstallMarketplaceSkillDependencies)
	api.GET("/workspace/status", s.handleGetWorkspaceStatus)
	api.POST("/workspace/repair", s.handleRepairWorkspace)
	api.POST("/webhooks/test", s.handleTestWebhook)
	s.registerConfiguredWebhookRoute(api)

	// Auth management (change password, profile)
	api.POST("/auth/change-password", s.handleChangePassword)
	api.GET("/auth/profile", s.handleGetProfile)
	api.GET("/auth/me", s.handleGetMe)
	api.PUT("/auth/profile", s.handleUpdateProfile)

	// Tool session routes
	api.GET("/tool-sessions", s.handleListToolSessions)
	api.POST("/tool-sessions", s.handleCreateToolSession)
	api.POST("/tool-sessions/:id/detach", s.handleDetachToolSession)
	api.POST("/tool-sessions/:id/terminate", s.handleTerminateToolSession)
	api.PUT("/tool-sessions/:id", s.handleUpdateToolSession)
	api.POST("/tool-sessions/:id/access", s.handleUpdateToolSessionAccess)
	api.POST("/tool-sessions/:id/otp", s.handleGenerateToolSessionOTP)
	api.POST("/tool-sessions/:id/restart", s.handleRestartToolSession)
	api.POST("/tool-sessions/:id/attach-token", s.handleCreateToolSessionAttachToken)
	api.POST("/tool-sessions/consume-token", s.handleConsumeToolSessionAttachToken)
	api.POST("/tool-sessions/spawn", s.handleSpawnToolSession)
	api.POST("/external-agents/resolve-session", s.handleResolveExternalAgentSession)
	api.GET("/external-agents/catalog", s.handleGetExternalAgentCatalog)
	api.GET("/daemon/registry", s.handleGetDaemonRegistry)
	api.GET("/daemon/bootstrap", s.handleGetDaemonBootstrap)
	api.POST("/daemon/register", s.handleRegisterDaemon)
	api.POST("/daemon/heartbeat", s.handleHeartbeatDaemon)
	api.POST("/daemon/tasks/fetch", s.handleFetchDaemonTasks)
	api.POST("/daemon/tasks/update", s.handleUpdateDaemonTaskStatus)
	api.POST("/daemon/explorer/tree", s.handleDaemonExplorerTree)
	api.POST("/daemon/explorer/file", s.handleDaemonExplorerFile)
	api.GET("/tool-sessions/:id/process/status", s.handleToolSessionProcessStatus)
	api.GET("/tool-sessions/:id/process/output", s.handleToolSessionProcessOutput)
	api.POST("/tool-sessions/:id/process/input", s.handleToolSessionProcessInput)
	api.POST("/tool-sessions/:id/process/kill", s.handleToolSessionProcessKill)
	api.POST("/tool-sessions/cleanup-terminated", s.handleCleanupTerminatedToolSessions)
	api.POST("/tool-sessions/events/cleanup", s.handleCleanupToolSessionEvents)

	// Approval routes
	api.GET("/approvals", s.handleGetApprovals)
	api.POST("/approvals/:id/approve", s.handleApproveRequest)
	api.POST("/approvals/:id/deny", s.handleDenyRequest)

	// Serve embedded frontend (SPA fallback)
	distFS, err := fs.Sub(frontend.Dist, "dist")
	if err == nil {
		fileServer := http.FileServer(http.FS(distFS))
		e.GET("/*", echo.WrapHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Try to serve static file first, fallback to index.html for SPA routing
			f, err := distFS.Open(r.URL.Path[1:]) // strip leading /
			if err != nil {
				// Serve index.html for SPA client-side routing
				r.URL.Path = "/"
			} else {
				_ = f.Close()
			}
			fileServer.ServeHTTP(w, r)
		})))
	}

	s.echo = e
}

func (s *Server) registerConfiguredWebhookRoute(api *echo.Group) {
	if s == nil || api == nil || s.config == nil {
		return
	}
	path := normalizeConfiguredWebhookPath(s.config.Webhook.Path)
	if path == "" || path == "/webhooks/test" {
		return
	}
	api.POST(path, s.handleTestWebhook)
}

func normalizeConfiguredWebhookPath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "/api/") {
		trimmed = strings.TrimPrefix(trimmed, "/api")
	}
	if !strings.HasPrefix(trimmed, "/") {
		trimmed = "/" + trimmed
	}
	return strings.TrimSpace(trimmed)
}

// Start starts the WebUI server.
func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.port)
	s.logger.Info("WebUI server starting",
		zap.String("addr", addr),
		zap.Int("port", s.port),
	)

	// Use http.Server directly so we can control shutdown from fx lifecycle
	// (Echo v5's e.Start() manages its own signal handling which conflicts with fx).
	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: s.echo,
	}

	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("WebUI server error", zap.Error(err))
		}
	}()

	return nil
}

// Stop gracefully stops the WebUI server.
func (s *Server) Stop(ctx context.Context) error {
	s.logger.Info("WebUI server stopping")
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

// --- Auth Handlers ---

func (s *Server) handleInitStatus(c *echo.Context) error {
	cred, err := config.LoadAdminCredential(s.entClient)
	if err != nil {
		s.logger.Warn("Failed to load admin credential", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load auth status"})
	}
	initialized := cred != nil && strings.TrimSpace(cred.PasswordHash) != ""

	configPath, err := s.resolveBootstrapConfigPath()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to resolve config path"})
	}
	workspaceStatus, err := workspace.NewManager(s.config.WorkspacePath(), s.logger).Inspect()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to inspect workspace"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"initialized": initialized,
		"bootstrap": map[string]interface{}{
			"config_path":      configPath,
			"db_dir":           s.config.Storage.DBDir,
			"workspace":        s.config.Agents.Defaults.Workspace,
			"logger":           s.config.Logger,
			"gateway":          s.config.Gateway,
			"webui":            s.config.WebUI,
			"webhook":          s.config.Webhook,
			"workspace_status": workspaceStatus,
		},
	})
}

func (s *Server) handleInitPassword(c *echo.Context) error {
	cred, err := config.LoadAdminCredential(s.entClient)
	if err != nil {
		s.logger.Warn("Failed to load admin credential", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load auth status"})
	}
	if cred != nil && strings.TrimSpace(cred.PasswordHash) != "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "already initialized"})
	}

	var body struct {
		Username  string `json:"username"`
		Password  string `json:"password"`
		Bootstrap *struct {
			Logger  *config.LoggerConfig  `json:"logger"`
			Gateway *config.GatewayConfig `json:"gateway"`
			WebUI   *config.WebUIConfig   `json:"webui"`
		} `json:"bootstrap"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	if body.Password == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "password required"})
	}

	hash, err := config.HashPassword(body.Password)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to hash password"})
	}

	username := strings.TrimSpace(body.Username)
	if username == "" {
		username = "admin"
	}

	restartSections := make([]string, 0, 3)
	if body.Bootstrap != nil {
		if body.Bootstrap.Logger != nil {
			s.config.Logger = *body.Bootstrap.Logger
			restartSections = append(restartSections, "logger")
		}
		if body.Bootstrap.Gateway != nil {
			s.config.Gateway = *body.Bootstrap.Gateway
			restartSections = append(restartSections, "gateway")
		}
		if body.Bootstrap.WebUI != nil {
			s.config.WebUI = *body.Bootstrap.WebUI
			restartSections = append(restartSections, "webui")
		}
		if err := config.ValidateConfig(s.config); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		if len(restartSections) > 0 {
			if err := s.saveBootstrapConfig(); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
		}
	}

	newCred := &config.AdminCredential{
		Username:     username,
		PasswordHash: hash,
		JWTSecret:    config.GenerateJWTSecret(),
	}

	if err := config.SaveAdminCredential(s.entClient, newCred); err != nil {
		s.logger.Error("Failed to persist admin credential", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to save credentials"})
	}

	profile, err := config.BuildAuthProfileByUsername(c.Request().Context(), s.entClient, username)
	if err != nil {
		s.logger.Error("Failed to load auth profile after init", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load profile"})
	}
	if err := config.RecordUserLogin(c.Request().Context(), s.entClient, profile.UserID); err != nil {
		s.logger.Warn("Failed to record init login time", zap.Error(err))
	}

	token, err := s.generateToken(profile)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "token generation failed"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"token":            token,
		"user":             s.authProfileResponse(profile),
		"restart_required": len(restartSections) > 0,
		"restart_sections": restartSections,
	})
}

func (s *Server) handleInitRepairWorkspace(c *echo.Context) error {
	if err := workspace.NewManager(s.config.WorkspacePath(), s.logger).Ensure(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to repair workspace"})
	}
	status, err := workspace.NewManager(s.config.WorkspacePath(), s.logger).Inspect()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to inspect workspace"})
	}
	return c.JSON(http.StatusOK, map[string]any{
		"status":           "repaired",
		"workspace_status": status,
	})
}

func (s *Server) handleLogin(c *echo.Context) error {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	loginUser, err := config.AuthenticateUser(c.Request().Context(), s.entClient, body.Username, body.Password)
	if err != nil {
		if errors.Is(err, config.ErrAdminNotInitialized) {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		}
		s.logger.Error("Failed to authenticate login", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "authentication failed"})
	}

	profile, err := config.BuildAuthProfileByUserID(c.Request().Context(), s.entClient, loginUser.ID)
	if err != nil {
		s.logger.Error("Failed to load auth profile", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load profile"})
	}
	if err := config.RecordUserLogin(c.Request().Context(), s.entClient, profile.UserID); err != nil {
		s.logger.Warn("Failed to record login timestamp", zap.Error(err))
	}

	token, err := s.generateToken(profile)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "token generation failed"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"token": token,
		"user":  s.authProfileResponse(profile),
	})
}

func (s *Server) handleChangePassword(c *echo.Context) error {
	var body struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	if body.NewPassword == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "new_password required"})
	}

	profile, err := s.authProfileFromContext(c)
	if err != nil {
		if errors.Is(err, config.ErrAdminNotInitialized) {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid token"})
		}
		s.logger.Error("Failed to resolve auth profile", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load profile"})
	}

	loginUser, err := config.AuthenticateUser(c.Request().Context(), s.entClient, profile.Username, body.OldPassword)
	if err != nil {
		if errors.Is(err, config.ErrAdminNotInitialized) {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "old password is incorrect"})
		}
		s.logger.Error("Failed to verify old password", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "password check failed"})
	}

	hash, err := config.HashPassword(body.NewPassword)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to hash password"})
	}

	if err := config.UpdateUserPassword(c.Request().Context(), s.entClient, loginUser.ID, hash); err != nil {
		s.logger.Error("Failed to update password", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to save credentials"})
	}

	newSecret := config.GenerateJWTSecret()
	if err := config.RotateJWTSecret(s.entClient, newSecret); err != nil {
		s.logger.Error("Failed to rotate jwt secret", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to rotate token secret"})
	}

	freshProfile, err := config.BuildAuthProfileByUserID(c.Request().Context(), s.entClient, loginUser.ID)
	if err != nil {
		s.logger.Error("Failed to reload profile", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load profile"})
	}

	token, err := s.generateToken(freshProfile)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "token generation failed"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status": "password changed",
		"token":  token,
		"user":   s.authProfileResponse(freshProfile),
	})
}

func (s *Server) handleGetProfile(c *echo.Context) error {
	profile, err := s.authProfileFromContext(c)
	if err != nil {
		if errors.Is(err, config.ErrAdminNotInitialized) {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "not initialized"})
		}
		s.logger.Error("Failed to load profile", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load profile"})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"username":    profile.Username,
		"nickname":    profile.Nickname,
		"role":        profile.Role,
		"tenant_id":   profile.TenantID,
		"tenant_slug": profile.TenantSlug,
		"user_id":     profile.UserID,
	})
}

func (s *Server) handleGetMe(c *echo.Context) error {
	profile, err := s.authProfileFromContext(c)
	if err != nil {
		if errors.Is(err, config.ErrAdminNotInitialized) {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid token"})
		}
		s.logger.Error("Failed to load current user", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load current user"})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"user": s.authProfileResponse(profile),
	})
}

func (s *Server) handleUpdateProfile(c *echo.Context) error {
	var body struct {
		Username string `json:"username"`
		Nickname string `json:"nickname"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	profile, err := s.authProfileFromContext(c)
	if err != nil {
		if errors.Is(err, config.ErrAdminNotInitialized) {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid token"})
		}
		s.logger.Error("Failed to resolve auth profile", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load profile"})
	}

	newUsername := strings.TrimSpace(body.Username)
	if newUsername == "" {
		newUsername = profile.Username
	}
	updated, err := config.UpdateUserProfile(c.Request().Context(), s.entClient, profile.UserID, newUsername, body.Nickname)
	if err != nil {
		if errors.Is(err, config.ErrUsernameAlreadyUsed) {
			return c.JSON(http.StatusConflict, map[string]string{"error": "username is already used"})
		}
		if errors.Is(err, config.ErrAdminNotInitialized) {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "not initialized"})
		}
		s.logger.Error("Failed to persist updated profile", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to save profile"})
	}

	freshProfile, err := config.BuildAuthProfileByUserID(c.Request().Context(), s.entClient, updated.ID)
	if err != nil {
		s.logger.Error("Failed to reload updated profile", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load profile"})
	}

	resp := map[string]interface{}{
		"user": s.authProfileResponse(freshProfile),
	}
	if freshProfile.Username != profile.Username {
		token, err := s.generateToken(freshProfile)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "token generation failed"})
		}
		resp["token"] = token
	}

	return c.JSON(http.StatusOK, resp)
}

// --- Provider Handlers ---

func (s *Server) handleGetProviders(c *echo.Context) error {
	loaded, err := s.providers.List(c.Request().Context())
	if err != nil {
		s.logger.Error("Failed to load providers from database", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load providers"})
	}

	providers := make([]map[string]interface{}, len(loaded))
	for i, p := range loaded {
		providers[i] = s.providerProfileToView(p)
	}
	return c.JSON(http.StatusOK, providers)
}

func (s *Server) handleGetProviderRuntime(c *echo.Context) error {
	if s.agent == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "agent not available"})
	}

	loaded, err := s.providers.List(c.Request().Context())
	if err != nil {
		s.logger.Error("Failed to load providers from database", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load providers"})
	}

	names := make([]string, 0, len(loaded))
	for _, provider := range loaded {
		trimmed := strings.TrimSpace(provider.Name)
		if trimmed == "" {
			continue
		}
		names = append(names, trimmed)
	}

	snapshots := s.agent.GetFailoverSnapshots(names)
	items := make([]map[string]interface{}, 0, len(names))
	for _, name := range names {
		profile := s.config.GetProviderConfig(name)
		snapshot := snapshots[name]
		failureCounts := make(map[string]int, len(snapshot.FailureCounts))
		for reason, count := range snapshot.FailureCounts {
			failureCounts[string(reason)] = count
		}

		item := map[string]interface{}{
			"name":                       name,
			"available":                  snapshot.Available,
			"in_cooldown":                snapshot.InCooldown,
			"error_count":                snapshot.ErrorCount,
			"cooldown_remaining_seconds": int(snapshot.CooldownRemaining.Seconds()),
			"failure_counts":             failureCounts,
			"disabled_reason":            string(snapshot.DisabledReason),
			"last_failure_unix":          int64(0),
			"cooldown_end_unix":          int64(0),
			"disabled_until_unix":        int64(0),
		}
		if !snapshot.LastFailure.IsZero() {
			item["last_failure_unix"] = snapshot.LastFailure.Unix()
		}
		if !snapshot.CooldownEnd.IsZero() {
			item["cooldown_end_unix"] = snapshot.CooldownEnd.Unix()
		}
		if !snapshot.DisabledUntil.IsZero() {
			item["disabled_until_unix"] = snapshot.DisabledUntil.Unix()
		}
		if !providerProfileUsableForExecution(profile) {
			item["available"] = false
			item["disabled_reason"] = "invalid_config"
		}

		items = append(items, item)
	}

	return c.JSON(http.StatusOK, items)
}

func (s *Server) handleCreateProvider(c *echo.Context) error {
	var profile config.ProviderProfile
	if err := c.Bind(&profile); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	created, err := s.providers.Create(c.Request().Context(), profile)
	if err != nil {
		return s.handleProviderStoreError(c, err)
	}
	if err := s.ensureRoutingProvidersValid(); err != nil {
		s.logger.Warn("Failed to persist routing config after provider create", zap.Error(err))
	}
	return c.JSON(http.StatusCreated, map[string]interface{}{
		"status":   "created",
		"provider": s.providerProfileToView(*created),
	})
}

func (s *Server) handleUpdateProvider(c *echo.Context) error {
	name := c.Param("name")
	var profile config.ProviderProfile
	if err := c.Bind(&profile); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	updated, err := s.providers.Update(c.Request().Context(), name, profile)
	if err != nil {
		return s.handleProviderStoreError(c, err)
	}
	if err := s.ensureRoutingProvidersValid(); err != nil {
		s.logger.Warn("Failed to persist routing config after provider update", zap.Error(err))
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":   "updated",
		"provider": s.providerProfileToView(*updated),
	})
}

func (s *Server) handleDeleteProvider(c *echo.Context) error {
	name := c.Param("name")

	if err := s.providers.Delete(c.Request().Context(), name); err != nil {
		return s.handleProviderStoreError(c, err)
	}
	if err := s.ensureRoutingProvidersValid(); err != nil {
		s.logger.Warn("Failed to persist routing config after provider delete", zap.Error(err))
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
}

func providerProfileToMap(p config.ProviderProfile) map[string]interface{} {
	return map[string]interface{}{
		"name":           p.Name,
		"provider_kind":  p.ProviderKind,
		"api_key":        p.APIKey,
		"api_base":       p.APIBase,
		"proxy":          p.Proxy,
		"default_weight": p.DefaultWeight,
		"enabled":        p.Enabled,
		"timeout":        p.Timeout,
	}
}

func (s *Server) providerProfileToView(p config.ProviderProfile) map[string]interface{} {
	apiKeySet := strings.TrimSpace(p.APIKey) != ""

	return map[string]interface{}{
		"name":               strings.TrimSpace(p.Name),
		"provider_kind":      strings.TrimSpace(p.ProviderKind),
		"api_key_set":        apiKeySet,
		"api_base":           strings.TrimSpace(p.APIBase),
		"proxy":              strings.TrimSpace(p.Proxy),
		"default_weight":     p.DefaultWeight,
		"enabled":            p.Enabled,
		"is_routing_default": strings.TrimSpace(s.config.Agents.Defaults.Provider) == strings.TrimSpace(p.Name),
		"supports_discovery": providerKindSupportsDiscovery(p.ProviderKind),
		"summary":            summarizeProviderProfile(p),
		"timeout":            p.Timeout,
	}
}

func providerProfileUsableForExecution(p *config.ProviderProfile) bool {
	if p == nil {
		return false
	}
	kind := strings.TrimSpace(strings.ToLower(p.ProviderKind))
	if kind == "" {
		return false
	}
	if meta, ok := providerregistry.Get(kind); ok {
		for _, field := range meta.AuthFields {
			if !field.Required {
				continue
			}
			switch field.Key {
			case "api_key":
				if strings.TrimSpace(p.APIKey) == "" {
					return false
				}
			}
		}
	}
	return true
}

func providerKindSupportsDiscovery(kind string) bool {
	item, ok := providerregistry.Get(strings.ToLower(strings.TrimSpace(kind)))
	return ok && item.SupportsDiscovery
}

func summarizeProviderProfile(p config.ProviderProfile) string {
	parts := make([]string, 0, 2)
	if p.Enabled {
		parts = append(parts, "Enabled")
	} else {
		parts = append(parts, "Disabled")
	}
	parts = append(parts, fmt.Sprintf("weight %d", p.DefaultWeight))
	return strings.Join(parts, " · ")
}

func (s *Server) handleGetProviderTypes(c *echo.Context) error {
	return c.JSON(http.StatusOK, providerregistry.List())
}

func (s *Server) handleProviderStoreError(c *echo.Context, err error) error {
	if err == nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "provider operation failed"})
	}
	switch {
	case errors.Is(err, providerstore.ErrProviderExists):
		return c.JSON(http.StatusConflict, map[string]string{"error": err.Error()})
	case errors.Is(err, providerstore.ErrProviderNotFound):
		return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
	default:
		s.logger.Error("Provider store operation failed", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
}

// ensureRoutingProvidersValid keeps default/fallback provider routing consistent after CRUD changes.
func (s *Server) ensureRoutingProvidersValid() error {
	changed := false

	defaultProvider := strings.TrimSpace(s.config.Agents.Defaults.Provider)
	if defaultProvider != "" && !s.hasRoutingTarget(defaultProvider) {
		s.config.Agents.Defaults.Provider = ""
		changed = true
	}
	if strings.TrimSpace(s.config.Agents.Defaults.Provider) == "" && len(s.config.Providers) > 0 {
		s.config.Agents.Defaults.Provider = strings.TrimSpace(s.config.Providers[0].Name)
		changed = true
	}

	filteredFallback := make([]string, 0, len(s.config.Agents.Defaults.Fallback))
	for _, name := range s.config.Agents.Defaults.Fallback {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		if !s.hasRoutingTarget(trimmed) {
			changed = true
			continue
		}
		filteredFallback = append(filteredFallback, trimmed)
	}
	filteredFallback = normalizeProviderNames(filteredFallback)
	if !reflect.DeepEqual(s.config.Agents.Defaults.Fallback, filteredFallback) {
		s.config.Agents.Defaults.Fallback = filteredFallback
		changed = true
	}

	if !changed {
		return nil
	}
	return config.SaveDatabaseSections(s.config, "agents")
}

func (s *Server) handleDiscoverProviderModels(c *echo.Context) error {
	var profile config.ProviderProfile
	if err := c.Bind(&profile); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	name := strings.TrimSpace(profile.Name)
	if name != "" {
		if existing, err := s.providers.Get(c.Request().Context(), name); err == nil {
			if strings.TrimSpace(profile.APIKey) == "" {
				profile.APIKey = existing.APIKey
			}
			if strings.TrimSpace(profile.APIBase) == "" {
				profile.APIBase = existing.APIBase
			}
			if profile.Timeout == 0 {
				profile.Timeout = existing.Timeout
			}
			if strings.TrimSpace(profile.Proxy) == "" {
				profile.Proxy = existing.Proxy
			}
			if strings.TrimSpace(profile.ProviderKind) == "" {
				profile.ProviderKind = existing.ProviderKind
			}
		}
	}

	kind := strings.TrimSpace(profile.ProviderKind)
	if kind == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "provider_kind is required"})
	}

	models, err := s.discoverModels(kind, &profile)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	if err := s.mergeDiscoveredModels(c.Request().Context(), profile, models); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"provider_kind": kind,
		"models":        models,
	})
}

func (s *Server) handleGetModels(c *echo.Context) error {
	manager, err := modelstore.NewManager(s.config, s.logger, s.entClient)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	items, err := manager.List(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, items)
}

func (s *Server) handleCreateModel(c *echo.Context) error {
	var input modelstore.ModelCatalog
	if err := c.Bind(&input); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	manager, err := modelstore.NewManager(s.config, s.logger, s.entClient)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	item, err := manager.Create(c.Request().Context(), input)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusCreated, map[string]interface{}{"status": "created", "model": item})
}

func (s *Server) handleGetModelRoutes(c *echo.Context) error {
	modelID := strings.TrimSpace(c.QueryParam("model_id"))
	if modelID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "model_id is required"})
	}
	manager, err := modelroute.NewManager(s.config, s.logger, s.entClient)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	items, err := manager.ListByModel(c.Request().Context(), modelID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, items)
}

func (s *Server) handleUpdateModelRoute(c *echo.Context) error {
	var input modelroute.ModelRoute
	if err := c.Bind(&input); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	modelID := strings.TrimSpace(c.Param("modelID"))
	providerName := strings.TrimSpace(c.Param("providerName"))
	manager, err := modelroute.NewManager(s.config, s.logger, s.entClient)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	item, err := manager.Update(c.Request().Context(), modelID, providerName, input)
	if errors.Is(err, modelroute.ErrRouteNotFound) {
		input.ModelID = modelID
		input.ProviderName = providerName
		item, err = manager.Create(c.Request().Context(), input)
	}
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{"status": "updated", "route": item})
}

func (s *Server) mergeDiscoveredModels(
	ctx context.Context,
	profile config.ProviderProfile,
	modelIDs []string,
) error {
	modelMgr, err := modelstore.NewManager(s.config, s.logger, s.entClient)
	if err != nil {
		return fmt.Errorf("create model manager: %w", err)
	}
	routeMgr, err := modelroute.NewManager(s.config, s.logger, s.entClient)
	if err != nil {
		return fmt.Errorf("create model route manager: %w", err)
	}

	for _, modelID := range modelIDs {
		trimmedModelID := strings.TrimSpace(modelID)
		if trimmedModelID == "" {
			continue
		}
		if _, err := modelMgr.Get(ctx, trimmedModelID); err != nil {
			if !errors.Is(err, modelstore.ErrModelNotFound) {
				return err
			}
			if _, err := modelMgr.Create(ctx, modelstore.ModelCatalog{
				ModelID:       trimmedModelID,
				DisplayName:   trimmedModelID,
				CatalogSource: "provider_discovery",
				Enabled:       true,
			}); err != nil && !errors.Is(err, modelstore.ErrModelExists) {
				return err
			}
		}

		routeInput := modelroute.ModelRoute{
			ModelID:      trimmedModelID,
			ProviderName: strings.TrimSpace(profile.Name),
			Enabled:      true,
			IsDefault:    false,
			Metadata: map[string]interface{}{
				"provider_model_id": trimmedModelID,
			},
		}
		if _, err := routeMgr.Create(ctx, routeInput); err != nil {
			if !errors.Is(err, modelroute.ErrRouteExists) {
				return err
			}
			if _, err := routeMgr.Update(ctx, routeInput.ModelID, routeInput.ProviderName, routeInput); err != nil {
				return err
			}
		}
	}

	return nil
}

// --- Cron Handlers ---

type createCronJobRequest struct {
	Name           string   `json:"name"`
	ScheduleKind   string   `json:"schedule_kind"`
	Schedule       string   `json:"schedule"`
	AtTime         string   `json:"at_time"`
	EveryDuration  string   `json:"every_duration"`
	Prompt         string   `json:"prompt"`
	Provider       string   `json:"provider"`
	Model          string   `json:"model"`
	Fallback       []string `json:"fallback"`
	DeleteAfterRun bool     `json:"delete_after_run"`
}

func (s *Server) handleListCronJobs(c *echo.Context) error {
	if s.cronMgr == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "cron manager unavailable"})
	}
	return c.JSON(http.StatusOK, s.cronMgr.ListJobs())
}

func (s *Server) handleCreateCronJob(c *echo.Context) error {
	if s.cronMgr == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "cron manager unavailable"})
	}

	var body createCronJobRequest
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	name := strings.TrimSpace(body.Name)
	prompt := strings.TrimSpace(body.Prompt)
	provider := strings.TrimSpace(body.Provider)
	model := strings.TrimSpace(body.Model)
	fallback := normalizeProviderNames(body.Fallback)
	if name == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "name is required"})
	}
	if prompt == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "prompt is required"})
	}
	if provider != "" && !s.hasRoutingTarget(provider) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("routing target not found: %s", provider)})
	}
	for _, item := range fallback {
		if !s.hasRoutingTarget(item) {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("fallback routing target not found: %s", item)})
		}
	}
	route := cron.RouteOptions{
		Provider: provider,
		Model:    model,
		Fallback: fallback,
	}

	kind := cron.ScheduleKind(strings.ToLower(strings.TrimSpace(body.ScheduleKind)))
	if kind == "" {
		kind = cron.ScheduleCron
	}

	var (
		job *cron.Job
		err error
	)

	switch kind {
	case cron.ScheduleCron:
		schedule := strings.TrimSpace(body.Schedule)
		if schedule == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "schedule is required for cron jobs"})
		}
		job, err = s.cronMgr.AddCronJobWithRoute(name, schedule, prompt, route)
	case cron.ScheduleAt:
		atRaw := strings.TrimSpace(body.AtTime)
		if atRaw == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "at_time is required for at jobs"})
		}
		at, parseErr := time.Parse(time.RFC3339, atRaw)
		if parseErr != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid at_time, must be RFC3339"})
		}
		job, err = s.cronMgr.AddAtJobWithRoute(name, at, prompt, body.DeleteAfterRun, route)
	case cron.ScheduleEvery:
		every := strings.TrimSpace(body.EveryDuration)
		if every == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "every_duration is required for every jobs"})
		}
		job, err = s.cronMgr.AddEveryJobWithRoute(name, every, prompt, route)
	default:
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid schedule_kind"})
	}

	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusCreated, map[string]interface{}{
		"status": "created",
		"job":    job,
	})
}

func (s *Server) handleDeleteCronJob(c *echo.Context) error {
	if s.cronMgr == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "cron manager unavailable"})
	}
	jobID := strings.TrimSpace(c.Param("id"))
	if jobID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "job id is required"})
	}
	if err := s.cronMgr.RemoveJob(jobID); err != nil {
		if strings.Contains(err.Error(), "job not found") {
			return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
		}
		s.logger.Error("Failed to delete cron job", zap.String("job_id", jobID), zap.Error(err))
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleEnableCronJob(c *echo.Context) error {
	if s.cronMgr == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "cron manager unavailable"})
	}
	jobID := strings.TrimSpace(c.Param("id"))
	if jobID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "job id is required"})
	}
	if err := s.cronMgr.EnableJob(jobID); err != nil {
		if strings.Contains(err.Error(), "job not found") {
			return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
		}
		s.logger.Error("Failed to enable cron job", zap.String("job_id", jobID), zap.Error(err))
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "enabled"})
}

func (s *Server) handleDisableCronJob(c *echo.Context) error {
	if s.cronMgr == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "cron manager unavailable"})
	}
	jobID := strings.TrimSpace(c.Param("id"))
	if jobID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "job id is required"})
	}
	if err := s.cronMgr.DisableJob(jobID); err != nil {
		if strings.Contains(err.Error(), "job not found") {
			return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
		}
		s.logger.Error("Failed to disable cron job", zap.String("job_id", jobID), zap.Error(err))
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "disabled"})
}

func (s *Server) handleRunCronJob(c *echo.Context) error {
	if s.cronMgr == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "cron manager unavailable"})
	}
	jobID := strings.TrimSpace(c.Param("id"))
	if jobID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "job id is required"})
	}
	if err := s.cronMgr.RunJob(jobID); err != nil {
		if strings.Contains(err.Error(), "job not found") {
			return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
		}
		s.logger.Error("Failed to run cron job", zap.String("job_id", jobID), zap.Error(err))
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "started"})
}

func (s *Server) discoverModels(kind string, profile *config.ProviderProfile) ([]string, error) {
	kind = strings.ToLower(strings.TrimSpace(kind))

	if kind == "openai" || kind == "generic" || kind == "openrouter" || kind == "groq" || kind == "vllm" || kind == "deepseek" || kind == "moonshot" || kind == "zhipu" || kind == "nvidia" {
		if models, err := discoverOpenAICompatibleModelsFunc(profile.APIBase, profile.APIKey, profile.Proxy, profile.Timeout); err == nil && len(models) > 0 {
			return models, nil
		}
	}

	client, err := providers.NewClient(kind, &providers.RelayInfo{
		ProviderName: kind,
		APIKey:       profile.APIKey,
		APIBase:      profile.APIBase,
		Proxy:        profile.Proxy,
		Timeout:      profile.GetTimeout(),
	})
	if err != nil {
		return nil, fmt.Errorf("init provider client failed: %w", err)
	}

	models, err := client.GetModelList()
	if err != nil {
		return nil, fmt.Errorf("discover models failed: %w", err)
	}
	sort.Strings(models)
	return dedupeStrings(models), nil
}

func discoverOpenAICompatibleModels(apiBase, apiKey, proxy string, timeout int) ([]string, error) {
	base := strings.TrimRight(strings.TrimSpace(apiBase), "/")
	if base == "" {
		return nil, fmt.Errorf("api_base is required for OpenAI-compatible model discovery")
	}

	client, err := providers.NewHTTPClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("setup proxy failed: %w", err)
	}
	if timeout <= 0 {
		timeout = 20
	}
	client.Timeout = time.Duration(timeout) * time.Second

	url := base + "/models"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request failed: %w", err)
	}
	if strings.TrimSpace(apiKey) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(apiKey))
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request /models failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("request /models failed: HTTP %d", resp.StatusCode)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("parse /models response failed: %w", err)
	}

	models := make([]string, 0)
	if data, ok := payload["data"].([]interface{}); ok {
		for _, item := range data {
			if m, ok := item.(map[string]interface{}); ok {
				if id, ok := m["id"].(string); ok && strings.TrimSpace(id) != "" {
					models = append(models, strings.TrimSpace(id))
				}
			}
		}
	}

	if data, ok := payload["models"].([]interface{}); ok {
		for _, item := range data {
			if m, ok := item.(map[string]interface{}); ok {
				if id, ok := m["id"].(string); ok && strings.TrimSpace(id) != "" {
					models = append(models, strings.TrimSpace(id))
					continue
				}
				if name, ok := m["name"].(string); ok && strings.TrimSpace(name) != "" {
					models = append(models, strings.TrimSpace(name))
				}
			}
		}
	}

	if len(models) == 0 {
		return nil, fmt.Errorf("no models found in /models response")
	}

	sort.Strings(models)
	return dedupeStrings(models), nil
}

var discoverOpenAICompatibleModelsFunc = discoverOpenAICompatibleModels

func dedupeStrings(items []string) []string {
	if len(items) == 0 {
		return items
	}
	out := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, it := range items {
		v := strings.TrimSpace(it)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

// --- Tool Session Handlers ---

func (s *Server) handleListToolSessions(c *echo.Context) error {
	if s.toolSess == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "tool session manager not available"})
	}

	owner := s.currentUsername(c)
	limit := 100
	if raw := strings.TrimSpace(c.QueryParam("limit")); raw != "" {
		var parsed int
		if _, err := fmt.Sscanf(raw, "%d", &parsed); err == nil && parsed > 0 && parsed <= 500 {
			limit = parsed
		}
	}

	sessions, err := s.toolSess.ListSessions(c.Request().Context(), toolsessions.ListSessionsInput{
		Owner:  owner,
		Source: strings.TrimSpace(c.QueryParam("source")),
		State:  strings.TrimSpace(c.QueryParam("state")),
		Limit:  limit,
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, sessions)
}

func (s *Server) handleCreateToolSession(c *echo.Context) error {
	if s.toolSess == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "tool session manager not available"})
	}

	var body toolsessions.CreateSessionInput
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	body.Owner = s.currentUsername(c)
	if strings.TrimSpace(body.Tool) == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "tool is required"})
	}
	if strings.TrimSpace(body.Source) == "" {
		body.Source = toolsessions.SourceWebUI
	}

	sess, err := s.toolSess.CreateSession(c.Request().Context(), body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusCreated, sess)
}

func (s *Server) handleDetachToolSession(c *echo.Context) error {
	if s.toolSess == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "tool session manager not available"})
	}
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "session id is required"})
	}
	if err := s.ensureSessionOwner(c, id); err != nil {
		return err
	}
	if err := s.toolSess.DetachSession(c.Request().Context(), id); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "session not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "detached"})
}

func (s *Server) handleTerminateToolSession(c *echo.Context) error {
	if s.toolSess == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "tool session manager not available"})
	}
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "session id is required"})
	}
	if err := s.ensureSessionOwner(c, id); err != nil {
		return err
	}
	var body struct {
		Reason string `json:"reason"`
	}
	_ = c.Bind(&body)
	if s.processMgr != nil {
		_ = s.processMgr.Kill(id)
	}
	s.tryKillRuntimeSession(c.Request().Context(), id)
	if err := s.toolSess.TerminateSession(c.Request().Context(), id, body.Reason); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "session not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "terminated"})
}

func (s *Server) handleCleanupTerminatedToolSessions(c *echo.Context) error {
	if s.toolSess == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "tool session manager not available"})
	}
	count, err := s.toolSess.ArchiveTerminatedSessions(c.Request().Context(), s.currentUsername(c))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]int{"archived": count})
}

func (s *Server) handleCleanupToolSessionEvents(c *echo.Context) error {
	if s.toolSess == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "tool session manager not available"})
	}
	count, err := s.toolSess.DeleteAllEvents(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]int{"deleted": count})
}

func (s *Server) handleCreateToolSessionAttachToken(c *echo.Context) error {
	if s.toolSess == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "tool session manager not available"})
	}
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "session id is required"})
	}
	if err := s.ensureSessionOwner(c, id); err != nil {
		return err
	}
	var body struct {
		TTLSeconds int `json:"ttl_seconds"`
	}
	_ = c.Bind(&body)
	ttl := time.Minute
	if body.TTLSeconds > 0 && body.TTLSeconds <= 600 {
		ttl = time.Duration(body.TTLSeconds) * time.Second
	}
	token, err := s.toolSess.CreateAttachToken(c.Request().Context(), id, s.currentUsername(c), ttl)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "session not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]string{"token": token})
}

func (s *Server) handleConsumeToolSessionAttachToken(c *echo.Context) error {
	if s.toolSess == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "tool session manager not available"})
	}
	var body struct {
		Token string `json:"token"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	sess, err := s.toolSess.ConsumeAttachToken(c.Request().Context(), body.Token, s.currentUsername(c))
	if err != nil {
		switch {
		case errors.Is(err, os.ErrNotExist):
			return c.JSON(http.StatusNotFound, map[string]string{"error": "token not found"})
		case errors.Is(err, os.ErrPermission):
			return c.JSON(http.StatusForbidden, map[string]string{"error": "token is not valid for this user"})
		case errors.Is(err, os.ErrDeadlineExceeded):
			return c.JSON(http.StatusGone, map[string]string{"error": "token expired"})
		default:
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
	}
	return c.JSON(http.StatusOK, sess)
}

func (s *Server) handleSpawnToolSession(c *echo.Context) error {
	if s.toolSess == nil || s.processMgr == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "tool runtime not available"})
	}

	var body struct {
		Tool             string                 `json:"tool"`
		Title            string                 `json:"title"`
		Command          string                 `json:"command"`
		CommandArgs      string                 `json:"command_args"`
		RuntimeTransport string                 `json:"runtime_transport"`
		Workdir          string                 `json:"workdir"`
		Metadata         map[string]interface{} `json:"metadata"`
		AccessMode       string                 `json:"access_mode"`
		AccessPassword   string                 `json:"access_password"`
		ProxyMode        string                 `json:"proxy_mode"`
		ProxyURL         string                 `json:"proxy_url"`
		PublicBaseURL    string                 `json:"public_base_url"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	toolName := strings.TrimSpace(body.Tool)
	if toolName == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "tool is required"})
	}
	command := resolveToolCommandWithArgs(toolName, body.Command, body.CommandArgs)
	if command == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "command is required"})
	}
	workdir := strings.TrimSpace(body.Workdir)
	if workdir == "" {
		workdir = s.config.WorkspacePath()
	}
	proxyMode, proxyURL, err := resolveToolProxyConfig("", "", body.ProxyMode, body.ProxyURL)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	metadata := cloneMap(body.Metadata)
	metadata = withToolProxyMetadata(metadata, proxyMode, proxyURL)
	metadata["user_command"] = command
	metadata["user_args"] = strings.TrimSpace(body.CommandArgs)
	transport := s.resolveSessionRuntimeTransport(metadata, body.RuntimeTransport)

	sess, err := s.toolSess.CreateSession(c.Request().Context(), toolsessions.CreateSessionInput{
		Owner:    s.currentUsername(c),
		Source:   toolsessions.SourceWebUI,
		Tool:     toolName,
		Title:    strings.TrimSpace(body.Title),
		Command:  command,
		Workdir:  workdir,
		State:    toolsessions.StateRunning,
		Metadata: metadata,
	})
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	launchCommand := applyToolProxyToCommand(command, proxyMode, proxyURL)
	runtimeSession := ""
	if wrapped, sessionName := buildToolRuntimeLaunchWithTransport(transport, launchCommand, sess.ID); sessionName != "" {
		launchCommand = wrapped
		runtimeSession = sessionName
		metadata = runtimeagents.ApplyLaunchMetadata(metadata, runtimeagents.LaunchInfo{
			TransportName: transport.Name(),
			SessionName:   runtimeSession,
			LaunchCommand: launchCommand,
		})
	}
	metadata[runtimeagents.MetadataLaunchCommand] = launchCommand
	if err := s.toolSess.UpdateSessionMetadata(c.Request().Context(), sess.ID, metadata); err != nil {
		_ = s.toolSess.TerminateSession(context.Background(), sess.ID, "failed to persist launch metadata: "+err.Error())
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to persist tool session metadata: " + err.Error()})
	}
	sess, _ = s.toolSess.GetSession(c.Request().Context(), sess.ID)

	spec := execenv.StartSpecFromContext(c.Request().Context(), sess.ID, launchCommand, workdir, metadata)
	if err := s.processMgr.StartWithSpec(context.Background(), spec); err != nil {
		_ = s.toolSess.TerminateSession(context.Background(), sess.ID, "failed to start process: "+err.Error())
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "failed to start tool process: " + err.Error()})
	}
	accessMode := strings.TrimSpace(body.AccessMode)
	accessPassword := ""
	if accessMode != "" && accessMode != toolsessions.AccessModeNone {
		accessPassword, err = s.toolSess.ConfigureSessionAccess(c.Request().Context(), sess.ID, accessMode, body.AccessPassword)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "failed to configure session access: " + err.Error()})
		}
		sess, _ = s.toolSess.GetSession(c.Request().Context(), sess.ID)
	}
	accessURL := ""
	if strings.TrimSpace(sess.AccessMode) != "" && sess.AccessMode != toolsessions.AccessModeNone {
		accessURL = s.buildToolSessionAccessURL(c, sess.ID, strings.TrimSpace(body.PublicBaseURL))
	}
	sess, _ = s.toolSess.GetSession(c.Request().Context(), sess.ID)

	_ = s.toolSess.AppendEvent(context.Background(), sess.ID, "process_started", map[string]interface{}{
		"command":         command,
		"launch_cmd":      launchCommand,
		"runtime_session": runtimeSession,
		"tmux_session":    runtimeSession,
		"workdir":         workdir,
		"proxy_mode":      proxyMode,
	})
	return c.JSON(http.StatusCreated, map[string]interface{}{
		"session":         sess,
		"access_mode":     sess.AccessMode,
		"access_url":      accessURL,
		"access_password": accessPassword,
	})
}

func (s *Server) handleResolveExternalAgentSession(c *echo.Context) error {
	if s.externalAgent == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "external agent manager not available"})
	}

	var body struct {
		AgentKind string `json:"agent_kind"`
		Workspace string `json:"workspace"`
		Tool      string `json:"tool"`
		Title     string `json:"title"`
		Command   string `json:"command"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	var probe externalAgentProcessProbe
	var starter externalagent.ProcessStarter
	if s.processMgr != nil {
		probe.mgr = s.processMgr
		starter = s.processMgr
	}
	result, err := externalagent.ExecuteResolveFlow(
		c.Request().Context(),
		s.externalAgent,
		externalagent.NewResolveOrchestrator(s.config, s.logger, s.entClient, s.approval, s.taskStore),
		s.config.WorkspacePath(),
		probe,
		starter,
		s.toolSess,
		runtimeagents.DefaultTransport(),
		externalagent.SessionSpec{
			Owner:     s.currentUsername(c),
			AgentKind: body.AgentKind,
			Workspace: body.Workspace,
			Tool:      body.Tool,
			Title:     body.Title,
			Command:   body.Command,
		},
	)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(result.HTTPStatus(), result.ResponseBody())
}

func (s *Server) handleGetExternalAgentCatalog(c *echo.Context) error {
	registry := externalagent.NewRegistry()
	type adapterPayload struct {
		Kind            string   `json:"kind"`
		Tool            string   `json:"tool"`
		Command         string   `json:"command"`
		SupportsInstall bool     `json:"supports_install"`
		InstallHint     []string `json:"install_hint,omitempty"`
	}
	items := make([]adapterPayload, 0, len(registry.List()))
	for _, adapter := range registry.List() {
		payload := adapterPayload{
			Kind:            adapter.Kind(),
			Tool:            adapter.Tool(),
			Command:         adapter.Command(),
			SupportsInstall: adapter.SupportsAutoInstall(),
		}
		if adapter.SupportsAutoInstall() {
			payload.InstallHint = adapter.InstallCommand(runtime.GOOS)
		}
		items = append(items, payload)
	}
	installed, err := externalagent.DetectInstalled("")
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{
		"adapters":  items,
		"installed": installed,
	})
}

func (s *Server) handleGetDaemonRegistry(c *echo.Context) error {
	if s.kvStore == nil {
		return c.JSON(http.StatusOK, map[string]any{
			"machines": []daemonhost.MachineStatus{},
		})
	}
	snapshot, err := daemonhost.NewRegistry(s.kvStore).Snapshot(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{
		"machines": daemonhost.MachineStatuses(snapshot),
	})
}

func (s *Server) daemonClientForMachine(ctx context.Context, machineID string) (*daemonhost.Client, *daemonv1.RuntimeInventory, error) {
	if s == nil || s.kvStore == nil {
		return nil, nil, fmt.Errorf("daemon registry unavailable")
	}
	snapshot, err := daemonhost.NewRegistry(s.kvStore).Snapshot(ctx)
	if err != nil {
		return nil, nil, err
	}
	machineID = strings.TrimSpace(machineID)
	if machineID == "" {
		return nil, nil, fmt.Errorf("machine_id is required")
	}
	info := snapshot.Machines[machineID]
	if info == nil {
		return nil, nil, fmt.Errorf("daemon machine not found")
	}
	baseURL := strings.TrimSpace(info.DaemonUrl)
	if baseURL == "" {
		return nil, nil, fmt.Errorf("daemon machine has no reachable daemon_url")
	}
	inv := snapshot.Inventories[machineID]
	if inv == nil {
		inv = &daemonv1.RuntimeInventory{}
	}
	return daemonhost.NewClient(baseURL), inv, nil
}

func (s *Server) handleDaemonExplorerTree(c *echo.Context) error {
	var body struct {
		MachineID   string `json:"machine_id"`
		WorkspaceID string `json:"workspace_id"`
		Path        string `json:"path"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	client, inv, err := s.daemonClientForMachine(c.Request().Context(), body.MachineID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	workspaceID := strings.TrimSpace(body.WorkspaceID)
	if workspaceID == "" {
		for _, ws := range inv.Workspaces {
			if ws != nil && ws.IsDefault {
				workspaceID = strings.TrimSpace(ws.WorkspaceId)
				break
			}
		}
	}
	resp, err := client.ListWorkspaceTree(&daemonv1.ListWorkspaceTreeRequest{
		WorkspaceId: workspaceID,
		Path:        strings.TrimSpace(body.Path),
	})
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, resp)
}

func (s *Server) handleDaemonExplorerFile(c *echo.Context) error {
	var body struct {
		MachineID   string `json:"machine_id"`
		WorkspaceID string `json:"workspace_id"`
		Path        string `json:"path"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	client, _, err := s.daemonClientForMachine(c.Request().Context(), body.MachineID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	resp, err := client.ReadWorkspaceFile(&daemonv1.ReadWorkspaceFileRequest{
		WorkspaceId: strings.TrimSpace(body.WorkspaceID),
		Path:        strings.TrimSpace(body.Path),
	})
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, resp)
}

func (s *Server) handleGetDaemonBootstrap(c *echo.Context) error {
	serverURL := strings.TrimSpace(s.config.WebUI.PublicBaseURL)
	if serverURL == "" {
		scheme := "http"
		if c.Scheme() != "" {
			scheme = c.Scheme()
		}
		serverURL = fmt.Sprintf("%s://%s", scheme, c.Request().Host)
	}
	token := s.getDaemonToken()
	machineName := strings.TrimSpace(s.currentUsername(c))
	command := fmt.Sprintf("nekobot daemon run --server-url %s --token %s --machine-name %s", serverURL, token, machineName)
	return c.JSON(http.StatusOK, map[string]any{
		"server_url":   serverURL,
		"daemon_token": token,
		"command":      command,
	})
}

func (s *Server) handleRegisterDaemon(c *echo.Context) error {
	if !s.authorizeDaemonRequest(c) {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid daemon token"})
	}
	if s.kvStore == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "daemon registry unavailable"})
	}
	var req daemonv1.RegisterMachineRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	resp, err := daemonhost.NewRegistry(s.kvStore).Register(c.Request().Context(), &req)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, resp)
}

func (s *Server) handleHeartbeatDaemon(c *echo.Context) error {
	if !s.authorizeDaemonRequest(c) {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid daemon token"})
	}
	if s.kvStore == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "daemon registry unavailable"})
	}
	var req daemonv1.HeartbeatMachineRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	resp, err := daemonhost.NewRegistry(s.kvStore).Heartbeat(c.Request().Context(), &req)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, resp)
}

func (s *Server) handleFetchDaemonTasks(c *echo.Context) error {
	if !s.authorizeDaemonRequest(c) {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid daemon token"})
	}
	if s.agent == nil || s.agent.TaskService() == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "task service unavailable"})
	}
	var req daemonv1.FetchAssignedTasksRequest
	if err := daemonhost.DecodeProtoJSON(c.Request(), &req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	resp := daemonhost.BuildAssignedTasks(s.agent.TaskService(), req.MachineId, req.RuntimeIds, int(req.Limit))
	return c.JSON(http.StatusOK, resp)
}

func (s *Server) handleUpdateDaemonTaskStatus(c *echo.Context) error {
	if !s.authorizeDaemonRequest(c) {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid daemon token"})
	}
	if s.agent == nil || s.agent.TaskService() == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "task service unavailable"})
	}
	var req daemonv1.UpdateTaskStatusRequest
	if err := daemonhost.DecodeProtoJSON(c.Request(), &req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	resp, task, err := daemonhost.ApplyTaskStatusUpdate(s.agent.TaskService(), &req)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	if err := s.appendDaemonTaskSessionUpdate(c.Request().Context(), task, &req); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, resp)
}

func (s *Server) appendDaemonTaskSessionUpdate(ctx context.Context, task tasks.Task, req *daemonv1.UpdateTaskStatusRequest) error {
	if s == nil || s.sessionMgr == nil || req == nil {
		return nil
	}
	sessionID := strings.TrimSpace(task.SessionID)
	if sessionID == "" {
		return nil
	}

	state := strings.TrimSpace(req.State)
	messageRole := "system"
	messageContent := ""
	switch state {
	case string(tasks.StateClaimed):
		messageContent = fmt.Sprintf("Daemon task claimed.\nTask ID: %s", strings.TrimSpace(task.ID))
	case string(tasks.StateRunning):
		messageContent = fmt.Sprintf("Daemon task started.\nTask ID: %s", strings.TrimSpace(task.ID))
	case string(tasks.StateRequiresAction):
		reason := strings.TrimSpace(req.BlockedReason)
		if reason == "" {
			reason = strings.TrimSpace(task.PendingAction)
		}
		if reason == "" {
			reason = "Daemon task requires action."
		}
		messageContent = reason
	case string(tasks.StateCompleted):
		messageRole = "assistant"
		messageContent = strings.TrimSpace(req.ResultMessage)
		if messageContent == "" {
			messageContent = fmt.Sprintf("Daemon task completed.\nTask ID: %s", strings.TrimSpace(task.ID))
		}
	case string(tasks.StateFailed):
		reason := strings.TrimSpace(req.Error)
		if reason == "" {
			reason = strings.TrimSpace(task.LastError)
		}
		messageContent = strings.TrimSpace(req.ResultMessage)
		if messageContent == "" {
			messageContent = "Daemon task failed."
		}
		if reason != "" {
			messageContent = fmt.Sprintf("%s\nError: %s", messageContent, reason)
		}
	case string(tasks.StateCanceled):
		messageContent = fmt.Sprintf("Daemon task canceled.\nTask ID: %s", strings.TrimSpace(task.ID))
	default:
		return nil
	}
	if strings.TrimSpace(messageContent) == "" {
		return nil
	}

	sess, err := s.sessionMgr.GetWithSource(sessionID, session.SourceWebUI)
	if err != nil {
		return fmt.Errorf("load daemon task session %s: %w", sessionID, err)
	}
	sess.AddMessage(agent.Message{
		Role:    messageRole,
		Content: messageContent,
	})
	if err := s.sessionMgr.Save(sess); err != nil {
		return fmt.Errorf("save daemon task session %s: %w", sessionID, err)
	}
	s.publishChatEvent(chatEvent{
		SessionID: sessionID,
		Role:      messageRole,
		Content:   messageContent,
		Timestamp: time.Now().Unix(),
	})
	return nil
}

func (s *Server) handleChatEvents(c *echo.Context) error {
	tokenStr := strings.TrimSpace(c.QueryParam("token"))
	if tokenStr == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "token required"})
	}
	username, err := s.parseJWTSubject(tokenStr)
	if err != nil || strings.TrimSpace(username) == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid token"})
	}
	sessionID := strings.TrimSpace(c.QueryParam("session_id"))
	if sessionID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "session_id required"})
	}
	if runtimeID, isAlias := resolveWebUIChatRuntimeAlias(sessionID); isAlias {
		sessionID = webUIRuntimeChatSessionID(username, runtimeID)
	}

	res := c.Response()
	req := c.Request()
	res.Header().Set("Content-Type", "text/event-stream")
	res.Header().Set("Cache-Control", "no-cache")
	res.Header().Set("Connection", "keep-alive")

	ch := make(chan chatEvent, 16)
	s.registerChatEventSubscriber(sessionID, ch)
	defer s.unregisterChatEventSubscriber(sessionID, ch)

	flusher, ok := res.(http.Flusher)
	if !ok {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "streaming unsupported"})
	}

	_, _ = res.Write([]byte(": connected\n\n"))
	flusher.Flush()

	for {
		select {
		case <-req.Context().Done():
			return nil
		case event := <-ch:
			payload, _ := json.Marshal(event)
			_, _ = res.Write([]byte("data: "))
			_, _ = res.Write(payload)
			_, _ = res.Write([]byte("\n\n"))
			flusher.Flush()
		}
	}
}

func (s *Server) registerChatEventSubscriber(sessionID string, ch chan chatEvent) {
	if s == nil || ch == nil {
		return
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return
	}
	s.chatEventMu.Lock()
	defer s.chatEventMu.Unlock()
	if s.chatEventSubs == nil {
		s.chatEventSubs = map[string]map[chan chatEvent]struct{}{}
	}
	if s.chatEventSubs[sessionID] == nil {
		s.chatEventSubs[sessionID] = map[chan chatEvent]struct{}{}
	}
	s.chatEventSubs[sessionID][ch] = struct{}{}
}

func (s *Server) unregisterChatEventSubscriber(sessionID string, ch chan chatEvent) {
	if s == nil || ch == nil {
		return
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return
	}
	s.chatEventMu.Lock()
	defer s.chatEventMu.Unlock()
	if s.chatEventSubs == nil || s.chatEventSubs[sessionID] == nil {
		return
	}
	delete(s.chatEventSubs[sessionID], ch)
	if len(s.chatEventSubs[sessionID]) == 0 {
		delete(s.chatEventSubs, sessionID)
	}
	close(ch)
}

func (s *Server) publishChatEvent(event chatEvent) {
	if s == nil {
		return
	}
	sessionID := strings.TrimSpace(event.SessionID)
	if sessionID == "" {
		return
	}
	s.chatEventMu.RLock()
	defer s.chatEventMu.RUnlock()
	for ch := range s.chatEventSubs[sessionID] {
		select {
		case ch <- event:
		default:
		}
	}
}

func (s *Server) handleUpdateToolSession(c *echo.Context) error {
	if s.toolSess == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "tool session manager not available"})
	}
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "session id is required"})
	}
	if err := s.ensureSessionOwner(c, id); err != nil {
		return err
	}
	current, err := s.toolSess.GetSession(c.Request().Context(), id)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "session not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	var body struct {
		Tool             string `json:"tool"`
		Title            string `json:"title"`
		Command          string `json:"command"`
		CommandArgs      string `json:"command_args"`
		RuntimeTransport string `json:"runtime_transport"`
		Workdir          string `json:"workdir"`
		AccessMode       string `json:"access_mode"`
		AccessPassword   string `json:"access_password"`
		ProxyMode        string `json:"proxy_mode"`
		ProxyURL         string `json:"proxy_url"`
		PublicBaseURL    string `json:"public_base_url"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	toolName := strings.TrimSpace(body.Tool)
	if toolName == "" {
		toolName = strings.TrimSpace(current.Tool)
	}
	if toolName == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "tool is required"})
	}

	command := resolveToolCommandWithArgs(toolName, body.Command, body.CommandArgs)
	if command == "" {
		command = strings.TrimSpace(current.Command)
	}
	if command == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "command is required"})
	}

	workdir := strings.TrimSpace(body.Workdir)
	if workdir == "" {
		workdir = strings.TrimSpace(current.Workdir)
	}
	if workdir == "" {
		workdir = s.config.WorkspacePath()
	}

	_, err = s.toolSess.UpdateSessionConfig(c.Request().Context(), id, toolName, strings.TrimSpace(body.Title), command, workdir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "session not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	existingProxyMode, existingProxyURL := toolProxyFromMetadata(current.Metadata)
	proxyMode, proxyURL, err := resolveToolProxyConfig(existingProxyMode, existingProxyURL, body.ProxyMode, body.ProxyURL)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	nextMetadata := withToolProxyMetadata(cloneMap(current.Metadata), proxyMode, proxyURL)
	nextMetadata["user_command"] = command
	nextMetadata["user_args"] = strings.TrimSpace(body.CommandArgs)
	if err := s.toolSess.UpdateSessionMetadata(c.Request().Context(), id, nextMetadata); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	updatedSession, _ := s.toolSess.GetSession(c.Request().Context(), id)

	accessPassword := ""
	modeChanged := strings.TrimSpace(body.AccessMode) != "" || strings.TrimSpace(body.AccessPassword) != ""
	if modeChanged {
		mode := strings.TrimSpace(body.AccessMode)
		if mode == "" {
			mode = strings.TrimSpace(updatedSession.AccessMode)
		}
		accessPassword, err = s.toolSess.ConfigureSessionAccess(c.Request().Context(), id, mode, body.AccessPassword)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "failed to configure session access: " + err.Error()})
		}
		updatedSession, _ = s.toolSess.GetSession(c.Request().Context(), id)
	}

	accessURL := ""
	if strings.TrimSpace(updatedSession.AccessMode) != "" && updatedSession.AccessMode != toolsessions.AccessModeNone {
		accessURL = s.buildToolSessionAccessURL(c, id, strings.TrimSpace(body.PublicBaseURL))
	}
	_ = s.toolSess.AppendEvent(context.Background(), id, "session_updated", map[string]interface{}{
		"command":    command,
		"workdir":    workdir,
		"proxy_mode": proxyMode,
	})

	return c.JSON(http.StatusOK, map[string]interface{}{
		"session":         updatedSession,
		"access_mode":     updatedSession.AccessMode,
		"access_url":      accessURL,
		"access_password": accessPassword,
	})
}

func (s *Server) handleRestartToolSession(c *echo.Context) error {
	if s.toolSess == nil || s.processMgr == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "tool runtime not available"})
	}

	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "session id is required"})
	}
	if err := s.ensureSessionOwner(c, id); err != nil {
		return err
	}
	current, err := s.toolSess.GetSession(c.Request().Context(), id)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "session not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	var body struct {
		Tool             string `json:"tool"`
		Title            string `json:"title"`
		Command          string `json:"command"`
		CommandArgs      string `json:"command_args"`
		RuntimeTransport string `json:"runtime_transport"`
		Workdir          string `json:"workdir"`
		AccessMode       string `json:"access_mode"`
		AccessPassword   string `json:"access_password"`
		ProxyMode        string `json:"proxy_mode"`
		ProxyURL         string `json:"proxy_url"`
		PublicBaseURL    string `json:"public_base_url"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	toolName := strings.TrimSpace(body.Tool)
	if toolName == "" {
		toolName = strings.TrimSpace(current.Tool)
	}
	if toolName == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "tool is required"})
	}

	command := resolveToolCommandWithArgs(toolName, body.Command, body.CommandArgs)
	if command == "" {
		command = strings.TrimSpace(current.Command)
	}
	if command == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "command is required"})
	}

	workdir := strings.TrimSpace(body.Workdir)
	if workdir == "" {
		workdir = strings.TrimSpace(current.Workdir)
	}
	if workdir == "" {
		workdir = s.config.WorkspacePath()
	}
	existingProxyMode, existingProxyURL := toolProxyFromMetadata(current.Metadata)
	proxyMode, proxyURL, err := resolveToolProxyConfig(existingProxyMode, existingProxyURL, body.ProxyMode, body.ProxyURL)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	nextMetadata := withToolProxyMetadata(cloneMap(current.Metadata), proxyMode, proxyURL)
	nextMetadata["user_command"] = command
	nextMetadata["user_args"] = strings.TrimSpace(body.CommandArgs)
	transport := s.resolveSessionRuntimeTransport(nextMetadata, body.RuntimeTransport)

	launchCommand := applyToolProxyToCommand(command, proxyMode, proxyURL)
	runtimeSession := ""
	if wrapped, sessionName := buildToolRuntimeLaunchWithTransport(transport, launchCommand, id); sessionName != "" {
		launchCommand = wrapped
		runtimeSession = sessionName
		nextMetadata = runtimeagents.ApplyLaunchMetadata(nextMetadata, runtimeagents.LaunchInfo{
			TransportName: transport.Name(),
			SessionName:   runtimeSession,
			LaunchCommand: launchCommand,
		})
	} else {
		delete(nextMetadata, runtimeagents.MetadataRuntimeTransport)
		delete(nextMetadata, runtimeagents.MetadataRuntimeSession)
		delete(nextMetadata, runtimeagents.MetadataTmuxSession)
	}
	nextMetadata[runtimeagents.MetadataLaunchCommand] = launchCommand

	_ = s.processMgr.Reset(id)
	s.tryKillRuntimeSession(c.Request().Context(), id)
	spec := execenv.StartSpecFromContext(c.Request().Context(), id, launchCommand, workdir, nextMetadata)
	if err := s.processMgr.StartWithSpec(context.Background(), spec); err != nil {
		_ = s.toolSess.TerminateSession(context.Background(), id, "failed to restart process: "+err.Error())
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "failed to restart tool process: " + err.Error()})
	}

	_, err = s.toolSess.UpdateSessionLaunch(c.Request().Context(), id, toolName, strings.TrimSpace(body.Title), command, workdir)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	if err := s.toolSess.UpdateSessionMetadata(c.Request().Context(), id, nextMetadata); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	updatedSession, _ := s.toolSess.GetSession(c.Request().Context(), id)

	accessPassword := ""
	modeChanged := strings.TrimSpace(body.AccessMode) != "" || strings.TrimSpace(body.AccessPassword) != ""
	if modeChanged {
		mode := strings.TrimSpace(body.AccessMode)
		if mode == "" {
			mode = strings.TrimSpace(updatedSession.AccessMode)
		}
		accessPassword, err = s.toolSess.ConfigureSessionAccess(c.Request().Context(), id, mode, body.AccessPassword)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "failed to configure session access: " + err.Error()})
		}
		updatedSession, _ = s.toolSess.GetSession(c.Request().Context(), id)
	}

	accessURL := ""
	if strings.TrimSpace(updatedSession.AccessMode) != "" && updatedSession.AccessMode != toolsessions.AccessModeNone {
		accessURL = s.buildToolSessionAccessURL(c, id, strings.TrimSpace(body.PublicBaseURL))
	}
	updatedSession, _ = s.toolSess.GetSession(c.Request().Context(), id)

	_ = s.toolSess.AppendEvent(context.Background(), id, "process_restarted", map[string]interface{}{
		"command":         command,
		"launch_cmd":      launchCommand,
		"runtime_session": runtimeSession,
		"tmux_session":    runtimeSession,
		"workdir":         workdir,
		"proxy_mode":      proxyMode,
	})

	return c.JSON(http.StatusOK, map[string]interface{}{
		"session":         updatedSession,
		"access_mode":     updatedSession.AccessMode,
		"access_url":      accessURL,
		"access_password": accessPassword,
	})
}

func (s *Server) handleToolSessionProcessStatus(c *echo.Context) error {
	if s.processMgr == nil || s.toolSess == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "tool runtime not available"})
	}
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "session id is required"})
	}
	if err := s.ensureSessionOwner(c, id); err != nil {
		return err
	}
	s.tryRestoreToolSessionRuntime(c.Request().Context(), id)
	sess, sessErr := s.toolSess.GetSession(c.Request().Context(), id)
	if sessErr != nil {
		if errors.Is(sessErr, os.ErrNotExist) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "session not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": sessErr.Error()})
	}

	status, err := s.processMgr.GetStatus(id)
	if err != nil {
		if isProcessSessionNotFound(err) {
			return c.JSON(http.StatusOK, map[string]interface{}{
				"id":                id,
				"running":           false,
				"missing":           true,
				"runtime_transport": metadataString(sess.Metadata, runtimeagents.MetadataRuntimeTransport),
				"runtime_session":   metadataString(sess.Metadata, runtimeagents.MetadataRuntimeSession),
				"tmux_session":      metadataString(sess.Metadata, runtimeagents.MetadataTmuxSession),
				"launch_cmd":        metadataString(sess.Metadata, runtimeagents.MetadataLaunchCommand),
			})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"id":                status.ID,
		"running":           status.Running,
		"exit_code":         status.ExitCode,
		"runtime_transport": metadataString(sess.Metadata, runtimeagents.MetadataRuntimeTransport),
		"runtime_session":   metadataString(sess.Metadata, runtimeagents.MetadataRuntimeSession),
		"tmux_session":      metadataString(sess.Metadata, runtimeagents.MetadataTmuxSession),
		"launch_cmd":        metadataString(sess.Metadata, runtimeagents.MetadataLaunchCommand),
		"observation":       status.Observation,
	})
}

func (s *Server) handleToolSessionProcessOutput(c *echo.Context) error {
	if s.processMgr == nil || s.toolSess == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "tool runtime not available"})
	}
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "session id is required"})
	}
	if err := s.ensureSessionOwner(c, id); err != nil {
		return err
	}
	s.tryRestoreToolSessionRuntime(c.Request().Context(), id)

	offset := 0
	limit := 300
	if raw := strings.TrimSpace(c.QueryParam("offset")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v >= 0 {
			offset = v
		}
	}
	if raw := strings.TrimSpace(c.QueryParam("limit")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 && v <= 2000 {
			limit = v
		}
	}

	lines, total, err := s.processMgr.GetOutput(id, offset, limit)
	if err != nil {
		if isProcessSessionNotFound(err) {
			return c.JSON(http.StatusOK, map[string]interface{}{
				"lines":   []string{},
				"total":   0,
				"running": false,
				"missing": true,
			})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	status, statusErr := s.processMgr.GetStatus(id)
	if statusErr == nil && status.Running {
		_ = s.toolSess.TouchSession(c.Request().Context(), id, toolsessions.StateRunning)
	} else if statusErr == nil && !status.Running {
		if sess, err := s.toolSess.GetSession(c.Request().Context(), id); err == nil &&
			sess.State != toolsessions.StateTerminated &&
			sess.State != toolsessions.StateArchived {
			_ = s.toolSess.TerminateSession(c.Request().Context(), id, fmt.Sprintf("process exited with code %d", status.ExitCode))
		}
	}
	running := statusErr == nil && status.Running
	exitCode := 0
	if statusErr == nil {
		exitCode = status.ExitCode
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"lines":     lines,
		"total":     total,
		"running":   running,
		"exit_code": exitCode,
	})
}

func (s *Server) handleToolSessionProcessInput(c *echo.Context) error {
	if s.processMgr == nil || s.toolSess == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "tool runtime not available"})
	}
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "session id is required"})
	}
	if err := s.ensureSessionOwner(c, id); err != nil {
		return err
	}
	s.tryRestoreToolSessionRuntime(c.Request().Context(), id)

	var body struct {
		Data string `json:"data"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if body.Data == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "input data is required"})
	}

	if err := s.processMgr.Write(id, body.Data); err != nil {
		if isProcessSessionNotFound(err) {
			if s.tryRestoreToolSessionRuntime(c.Request().Context(), id) {
				retryErr := s.processMgr.Write(id, body.Data)
				if retryErr == nil {
					_ = s.toolSess.TouchSession(c.Request().Context(), id, toolsessions.StateRunning)
					return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
				}
				err = retryErr
			}
		}
		if isProcessSessionNotFound(err) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "tool process not found"})
		}
		if strings.Contains(strings.ToLower(err.Error()), "not running") {
			return c.JSON(http.StatusConflict, map[string]string{"error": "tool process is not running"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	_ = s.toolSess.TouchSession(c.Request().Context(), id, toolsessions.StateRunning)
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleToolSessionProcessKill(c *echo.Context) error {
	if s.processMgr == nil || s.toolSess == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "tool runtime not available"})
	}
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "session id is required"})
	}
	if err := s.ensureSessionOwner(c, id); err != nil {
		return err
	}

	err := s.processMgr.Kill(id)
	if err != nil && !isProcessSessionNotFound(err) && !strings.Contains(strings.ToLower(err.Error()), "not running") {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	s.tryKillRuntimeSession(c.Request().Context(), id)
	_ = s.toolSess.TerminateSession(c.Request().Context(), id, "killed from webui")
	return c.JSON(http.StatusOK, map[string]string{"status": "killed"})
}

func (s *Server) handleUpdateToolSessionAccess(c *echo.Context) error {
	if s.toolSess == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "tool session manager not available"})
	}
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "session id is required"})
	}
	if err := s.ensureSessionOwner(c, id); err != nil {
		return err
	}
	var body struct {
		Mode          string `json:"mode"`
		Password      string `json:"password"`
		PublicBaseURL string `json:"public_base_url"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	mode := strings.TrimSpace(body.Mode)
	if mode == "" {
		mode = toolsessions.AccessModeNone
	}
	password, err := s.toolSess.ConfigureSessionAccess(c.Request().Context(), id, mode, body.Password)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "session not found"})
		}
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	sess, err := s.toolSess.GetSession(c.Request().Context(), id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	accessURL := ""
	if sess.AccessMode != toolsessions.AccessModeNone {
		accessURL = s.buildToolSessionAccessURL(c, id, body.PublicBaseURL)
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"session":         sess,
		"access_mode":     sess.AccessMode,
		"access_url":      accessURL,
		"access_password": password,
	})
}

func (s *Server) handleGenerateToolSessionOTP(c *echo.Context) error {
	if s.toolSess == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "tool session manager not available"})
	}
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "session id is required"})
	}
	if err := s.ensureSessionOwner(c, id); err != nil {
		return err
	}

	var body struct {
		TTLSeconds int `json:"ttl_seconds"`
	}
	_ = c.Bind(&body)
	var ttl time.Duration
	if body.TTLSeconds > 0 {
		ttl = time.Duration(body.TTLSeconds) * time.Second
	}

	code, expiresAt, err := s.toolSess.GenerateSessionOTP(c.Request().Context(), id, ttl)
	if err != nil {
		switch {
		case errors.Is(err, os.ErrNotExist):
			return c.JSON(http.StatusNotFound, map[string]string{"error": "session not found"})
		case errors.Is(err, os.ErrPermission):
			return c.JSON(http.StatusConflict, map[string]string{"error": "external access is disabled for this session"})
		default:
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"session_id":   id,
		"otp_code":     code,
		"expires_at":   expiresAt.Unix(),
		"ttl_seconds":  int(time.Until(expiresAt).Seconds()),
		"generated_at": time.Now().Unix(),
	})
}

func (s *Server) handleToolSessionAccessLogin(c *echo.Context) error {
	if s.toolSess == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "tool session manager not available"})
	}
	var body struct {
		SessionID string `json:"session_id"`
		Password  string `json:"password"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	sess, err := s.toolSess.VerifySessionAccess(c.Request().Context(), body.SessionID, body.Password)
	if err != nil {
		switch {
		case errors.Is(err, os.ErrNotExist):
			return c.JSON(http.StatusNotFound, map[string]string{"error": "session not found"})
		case errors.Is(err, os.ErrPermission):
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid or expired password"})
		default:
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
	}
	username := strings.TrimSpace(sess.Owner)
	if username == "" {
		username = "tool:" + sess.ID
	}
	token, err := s.generateToken(&config.AuthProfile{Username: username, UserID: sess.Owner, Role: "member"})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "token generation failed"})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"token":      token,
		"session_id": sess.ID,
	})
}

func (s *Server) ensureSessionOwner(c *echo.Context, sessionID string) error {
	sess, err := s.toolSess.GetSession(c.Request().Context(), sessionID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "session not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	owner := s.currentUsername(c)
	if strings.TrimSpace(sess.Owner) != "" && sess.Owner != owner {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "session does not belong to current user"})
	}
	return nil
}

func (s *Server) authProfileFromContext(c *echo.Context) (*config.AuthProfile, error) {
	userID := s.currentUserID(c)
	if strings.TrimSpace(userID) != "" {
		profile, err := config.BuildAuthProfileByUserID(c.Request().Context(), s.entClient, userID)
		if err == nil {
			return profile, nil
		}
		if !errors.Is(err, config.ErrAdminNotInitialized) {
			return nil, err
		}
	}

	username := s.currentUsername(c)
	if strings.TrimSpace(username) == "" {
		return nil, config.ErrAdminNotInitialized
	}
	return config.BuildAuthProfileByUsername(c.Request().Context(), s.entClient, username)
}

func (s *Server) authProfileResponse(profile *config.AuthProfile) map[string]interface{} {
	if profile == nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"id":          profile.UserID,
		"username":    profile.Username,
		"nickname":    profile.Nickname,
		"role":        profile.Role,
		"tenant_id":   profile.TenantID,
		"tenant_slug": profile.TenantSlug,
	}
}

func (s *Server) currentUserID(c *echo.Context) string {
	user := c.Get("user")
	token, ok := user.(*jwt.Token)
	if !ok || token == nil {
		return ""
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return ""
	}
	uid, _ := claims["uid"].(string)
	return strings.TrimSpace(uid)
}

func (s *Server) currentUsername(c *echo.Context) string {
	user := c.Get("user")
	token, ok := user.(*jwt.Token)
	if !ok || token == nil {
		return ""
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return ""
	}
	sub, _ := claims["sub"].(string)
	return strings.TrimSpace(sub)
}

func resolveToolCommand(toolName, command string) string {
	cmd := strings.TrimSpace(command)
	if cmd != "" {
		return cmd
	}
	tool := strings.TrimSpace(strings.ToLower(toolName))
	switch tool {
	case "codex":
		return "codex"
	case "claude":
		return "claude"
	case "opencode":
		return "opencode"
	case "aider":
		return "aider"
	default:
		return strings.TrimSpace(toolName)
	}
}

func resolveToolCommandWithArgs(toolName, command, commandArgs string) string {
	args := strings.TrimSpace(commandArgs)
	if args == "" {
		return resolveToolCommand(toolName, command)
	}
	base := resolveToolCommand(toolName, "")
	if base == "" {
		base = strings.TrimSpace(toolName)
	}
	if base == "" {
		return ""
	}
	return strings.TrimSpace(base + " " + args)
}

const (
	toolProxyModeInherit = "inherit"
	toolProxyModeClear   = "clear"
	toolProxyModeCustom  = "custom"
)

func normalizeToolProxyMode(mode string) string {
	switch strings.TrimSpace(strings.ToLower(mode)) {
	case toolProxyModeClear:
		return toolProxyModeClear
	case toolProxyModeCustom:
		return toolProxyModeCustom
	default:
		return toolProxyModeInherit
	}
}

func cloneMap(src map[string]interface{}) map[string]interface{} {
	if len(src) == 0 {
		return map[string]interface{}{}
	}
	dst := make(map[string]interface{}, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func toolProxyFromMetadata(metadata map[string]interface{}) (string, string) {
	if len(metadata) == 0 {
		return toolProxyModeInherit, ""
	}
	rawMode, _ := metadata["proxy_mode"].(string)
	mode := normalizeToolProxyMode(rawMode)
	rawURL, _ := metadata["proxy_url"].(string)
	if mode != toolProxyModeCustom {
		return mode, ""
	}
	return mode, strings.TrimSpace(rawURL)
}

func withToolProxyMetadata(metadata map[string]interface{}, mode, proxyURL string) map[string]interface{} {
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	mode = normalizeToolProxyMode(mode)
	metadata["proxy_mode"] = mode
	if mode == toolProxyModeCustom && strings.TrimSpace(proxyURL) != "" {
		metadata["proxy_url"] = strings.TrimSpace(proxyURL)
	} else {
		delete(metadata, "proxy_url")
	}
	return metadata
}

func resolveToolProxyConfig(existingMode, existingURL, inputMode, inputURL string) (string, string, error) {
	mode := normalizeToolProxyMode(existingMode)
	urlValue := strings.TrimSpace(existingURL)

	if strings.TrimSpace(inputMode) != "" {
		mode = normalizeToolProxyMode(inputMode)
	}
	if strings.TrimSpace(inputURL) != "" {
		urlValue = strings.TrimSpace(inputURL)
	}

	if mode != toolProxyModeCustom {
		return mode, "", nil
	}
	if urlValue == "" {
		return "", "", fmt.Errorf("proxy url is required when proxy mode is custom")
	}
	parsed, err := url.Parse(urlValue)
	if err != nil || strings.TrimSpace(parsed.Scheme) == "" {
		return "", "", fmt.Errorf("invalid proxy url")
	}
	return mode, urlValue, nil
}

func applyToolProxyToCommand(command, proxyMode, proxyURL string) string {
	cmd := strings.TrimSpace(command)
	mode := normalizeToolProxyMode(proxyMode)
	switch mode {
	case toolProxyModeClear:
		return "env -u HTTP_PROXY -u HTTPS_PROXY -u ALL_PROXY -u NO_PROXY -u http_proxy -u https_proxy -u all_proxy -u no_proxy " + cmd
	case toolProxyModeCustom:
		value := strconv.Quote(strings.TrimSpace(proxyURL))
		return fmt.Sprintf("env HTTP_PROXY=%s HTTPS_PROXY=%s ALL_PROXY=%s http_proxy=%s https_proxy=%s all_proxy=%s %s",
			value, value, value, value, value, value, cmd)
	default:
		return cmd
	}
}

func (s *Server) tryRestoreToolSessionRuntime(ctx context.Context, sessionID string) bool {
	if s.processMgr == nil || s.toolSess == nil {
		return false
	}
	id := strings.TrimSpace(sessionID)
	if id == "" {
		return false
	}
	if _, err := s.processMgr.GetStatus(id); err == nil {
		return true
	} else if !isProcessSessionNotFound(err) {
		return false
	}

	sess, err := s.toolSess.GetSession(ctx, id)
	if err != nil {
		return false
	}
	if sess.State == toolsessions.StateTerminated || sess.State == toolsessions.StateArchived {
		return false
	}

	transport := s.resolveSessionRuntimeTransport(sess.Metadata, "")
	attachCmd, runtimeSession, ok := buildToolReattachLaunchWithTransport(transport, id)
	if !ok {
		return false
	}
	workdir := strings.TrimSpace(sess.Workdir)
	if workdir == "" {
		workdir = s.config.WorkspacePath()
	}
	spec := execenv.StartSpecFromContext(ctx, id, attachCmd, workdir, sess.Metadata)
	if err := s.processMgr.StartWithSpec(context.Background(), spec); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "session already exists") {
			return true
		}
		s.logger.Warn("Failed to restore tool session runtime",
			zap.String("session_id", id),
			zap.String("runtime_session", runtimeSession),
			zap.Error(err),
		)
		return false
	}

	metadata := cloneMap(sess.Metadata)
	metadata = runtimeagents.ApplyLaunchMetadata(metadata, runtimeagents.LaunchInfo{
		TransportName: transport.Name(),
		SessionName:   runtimeSession,
		LaunchCommand: attachCmd,
	})
	if err := s.toolSess.UpdateSessionMetadata(context.Background(), id, metadata); err != nil {
		s.logger.Warn("Failed to persist restored tool session metadata",
			zap.String("session_id", id),
			zap.String("runtime_session", runtimeSession),
			zap.Error(err),
		)
	}

	_ = s.toolSess.AppendEvent(context.Background(), id, "process_restored", map[string]interface{}{
		"launch_cmd":      attachCmd,
		"runtime_session": runtimeSession,
		"tmux_session":    runtimeSession,
		"workdir":         workdir,
	})
	_ = s.toolSess.TouchSession(context.Background(), id, toolsessions.StateRunning)
	s.logger.Info("Restored tool session runtime",
		zap.String("session_id", id),
		zap.String("runtime_session", runtimeSession),
	)
	return true
}

func runtimeTransportAvailable() bool {
	return runtimeagents.DefaultTransport().Available()
}

func buildToolRuntimeLaunch(command, sessionID string) (string, string) {
	launchInfo := runtimeagents.DefaultTransport().WrapStart(command, sessionID)
	return launchInfo.LaunchCommand, launchInfo.SessionName
}

func buildToolReattachLaunch(sessionID string) (string, string, bool) {
	reattachInfo, ok := runtimeagents.DefaultTransport().BuildReattach(sessionID)
	return reattachInfo.LaunchCommand, reattachInfo.SessionName, ok
}

func buildToolRuntimeLaunchWithTransport(transport runtimeagents.RuntimeTransport, command, sessionID string) (string, string) {
	if transport == nil {
		transport = runtimeagents.DefaultTransport()
	}
	launchInfo := transport.WrapStart(command, sessionID)
	return launchInfo.LaunchCommand, launchInfo.SessionName
}

func buildToolReattachLaunchWithTransport(transport runtimeagents.RuntimeTransport, sessionID string) (string, string, bool) {
	if transport == nil {
		transport = runtimeagents.DefaultTransport()
	}
	reattachInfo, ok := transport.BuildReattach(sessionID)
	return reattachInfo.LaunchCommand, reattachInfo.SessionName, ok
}

func metadataString(values map[string]interface{}, key string) string {
	if len(values) == 0 {
		return ""
	}
	value, _ := values[key].(string)
	return strings.TrimSpace(value)
}

func runtimeTransportFromName(name string) runtimeagents.RuntimeTransport {
	return runtimeagents.TransportByName(name)
}

func (s *Server) defaultRuntimeTransport() runtimeagents.RuntimeTransport {
	if s == nil || s.config == nil {
		return runtimeagents.DefaultTransport()
	}
	name := strings.TrimSpace(s.config.WebUI.ToolSessionRuntimeTransport)
	if name == "" {
		return runtimeagents.DefaultTransport()
	}
	return runtimeTransportFromName(name)
}

func (s *Server) resolveSessionRuntimeTransport(metadata map[string]interface{}, requested string) runtimeagents.RuntimeTransport {
	requested = strings.TrimSpace(requested)
	if requested != "" {
		return runtimeTransportFromName(requested)
	}
	stored := runtimeagents.MetadataString(metadata, runtimeagents.MetadataRuntimeTransport)
	if stored != "" {
		return runtimeTransportFromName(stored)
	}
	return s.defaultRuntimeTransport()
}

func (s *Server) tryKillRuntimeSession(ctx context.Context, sessionID string) {
	transport := s.defaultRuntimeTransport()
	if s != nil && s.toolSess != nil {
		if sess, err := s.toolSess.GetSession(ctx, sessionID); err == nil && sess != nil {
			transport = s.resolveSessionRuntimeTransport(sess.Metadata, "")
		}
	}
	transport.KillSession(sessionID)
}

func isProcessSessionNotFound(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "session not found")
}

func (s *Server) buildToolSessionAccessURL(c *echo.Context, sessionID, overrideBase string) string {
	base := strings.TrimSpace(overrideBase)
	if base == "" {
		base = strings.TrimSpace(s.config.WebUI.PublicBaseURL)
	}
	if base == "" {
		base = requestScheme(c) + "://" + c.Request().Host
	}
	if !strings.Contains(base, "://") {
		base = requestScheme(c) + "://" + strings.TrimPrefix(base, "/")
	}
	base = strings.TrimRight(base, "/")
	values := url.Values{}
	values.Set("tab", "tools")
	values.Set("tool_session", strings.TrimSpace(sessionID))
	return base + "/?" + values.Encode()
}

func requestScheme(c *echo.Context) string {
	scheme := "http"
	if c.Request().TLS != nil {
		scheme = "https"
	}
	if forwardedProto := strings.TrimSpace(c.Request().Header.Get("X-Forwarded-Proto")); forwardedProto != "" {
		scheme = forwardedProto
	}
	return scheme
}

func (s *Server) resolveMarketplaceSkill(id string) (*skills.Skill, bool) {
	trimmedID := strings.TrimSpace(id)
	if trimmedID == "" {
		return nil, false
	}

	for _, skill := range s.skillsMgr.List() {
		if strings.TrimSpace(skill.ID) == trimmedID {
			return skill, true
		}
	}

	return nil, false
}

// --- Marketplace Handlers ---

func (s *Server) handleListMarketplaceSkills(c *echo.Context) error {
	if s.skillsMgr == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "skills manager not available"})
	}

	skillsList := s.skillsMgr.List()
	sort.Slice(skillsList, func(i, j int) bool {
		left := strings.TrimSpace(skillsList[i].ID)
		right := strings.TrimSpace(skillsList[j].ID)
		if left == right {
			return strings.TrimSpace(skillsList[i].Name) < strings.TrimSpace(skillsList[j].Name)
		}
		return left < right
	})

	items := make([]map[string]interface{}, 0, len(skillsList))
	for _, skill := range skillsList {
		items = append(items, s.marketplaceSkillItem(skill))
	}

	return c.JSON(http.StatusOK, items)
}

func (s *Server) handleListInstalledMarketplaceSkills(c *echo.Context) error {
	if s.skillsMgr == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "skills manager not available"})
	}

	skillsList := s.skillsMgr.ListEnabled()
	sort.Slice(skillsList, func(i, j int) bool {
		left := strings.TrimSpace(skillsList[i].ID)
		right := strings.TrimSpace(skillsList[j].ID)
		if left == right {
			return strings.TrimSpace(skillsList[i].Name) < strings.TrimSpace(skillsList[j].Name)
		}
		return left < right
	})

	records := make([]map[string]interface{}, 0, len(skillsList))
	for _, skill := range skillsList {
		if !s.marketplaceSkillIsInstalled(skill) {
			continue
		}
		records = append(records, s.marketplaceSkillItem(skill))
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"total":   len(records),
		"records": records,
	})
}

func (s *Server) handleSearchMarketplaceSkills(c *echo.Context) error {
	if s.skillsMgr == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "skills manager not available"})
	}

	query := strings.TrimSpace(c.QueryParam("q"))
	if query == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "query is required"})
	}

	output, err := s.skillsMgr.SearchRegistry(c.Request().Context(), query)
	success := err == nil
	return c.JSON(http.StatusOK, map[string]interface{}{
		"query":      query,
		"success":    success,
		"proxy":      s.skillsMgr.SkillsProxy(),
		"output":     strings.TrimSpace(output),
		"error":      errorString(err),
		"has_output": strings.TrimSpace(output) != "",
	})
}

func (s *Server) marketplaceSkillIsInstalled(skill *skills.Skill) bool {
	if skill == nil {
		return false
	}

	filePath := strings.TrimSpace(skill.FilePath)
	if filePath == "" || strings.HasPrefix(filePath, "builtin://") {
		return false
	}
	if s.skillsMgr == nil {
		return true
	}

	skillsDir := strings.TrimSpace(s.skillsMgr.SkillsDir())
	if skillsDir == "" {
		return true
	}

	relPath, err := filepath.Rel(filepath.Clean(skillsDir), filepath.Clean(filePath))
	if err != nil {
		return false
	}
	if relPath == "." {
		return true
	}
	return relPath != ".." && !strings.HasPrefix(relPath, ".."+string(os.PathSeparator))
}

func (s *Server) handleGetMarketplaceSkillItem(c *echo.Context) error {
	if s.skillsMgr == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "skills manager not available"})
	}

	skillID := strings.TrimSpace(c.Param("id"))
	skill, ok := s.resolveMarketplaceSkill(skillID)
	if !ok {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "skill not found"})
	}

	return c.JSON(http.StatusOK, s.marketplaceSkillItem(skill))
}

func (s *Server) handleGetMarketplaceSkillContent(c *echo.Context) error {
	if s.skillsMgr == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "skills manager not available"})
	}

	skillID := strings.TrimSpace(c.Param("id"))
	skill, ok := s.resolveMarketplaceSkill(skillID)
	if !ok {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "skill not found"})
	}

	raw, bodyRaw, err := readMarketplaceSkillContent(skill)
	if err != nil {
		s.logger.Error("Failed to read marketplace skill content",
			zap.String("skill_id", skillID),
			zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to read skill content"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"id":        strings.TrimSpace(skill.ID),
		"name":      strings.TrimSpace(skill.Name),
		"file_path": strings.TrimSpace(skill.FilePath),
		"raw":       raw,
		"body_raw":  bodyRaw,
	})
}

func (s *Server) handleInstallMarketplaceSkill(c *echo.Context) error {
	if s.skillsMgr == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "skills manager not available"})
	}

	var body struct {
		Source string `json:"source"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	source := strings.TrimSpace(body.Source)
	if source == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "source is required"})
	}

	targetPath, err := s.skillsMgr.InstallSkill(c.Request().Context(), source)
	if err != nil {
		s.logger.Error("Failed to install marketplace skill",
			zap.String("source", source),
			zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to install skill"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"source":    source,
		"target":    targetPath,
		"proxy":     s.skillsMgr.SkillsProxy(),
		"installed": true,
		"refreshed": true,
	})
}

func (s *Server) handleGetMarketplaceInventory(c *echo.Context) error {
	if s.skillsMgr == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "skills manager not available"})
	}

	sources := s.skillsMgr.GetSkillSources()
	sourceItems := make([]map[string]interface{}, 0, len(sources))
	for _, source := range sources {
		exists := true
		if source.Type != skills.SourceBuiltin {
			if _, err := os.Stat(source.Path); err != nil {
				if os.IsNotExist(err) {
					exists = false
				} else {
					return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to inspect skill source"})
				}
			}
		}

		sourceItems = append(sourceItems, map[string]interface{}{
			"path":     strings.TrimSpace(source.Path),
			"priority": source.Priority,
			"type":     string(source.Type),
			"exists":   exists,
			"builtin":  source.Type == skills.SourceBuiltin,
		})
	}

	enabledCount := 0
	alwaysCount := 0
	for _, skill := range s.skillsMgr.List() {
		if skill.Enabled {
			enabledCount++
		}
		if skill.Always {
			alwaysCount++
		}
	}
	versionStats := s.skillsMgr.VersionHistoryStats()
	versionPolicy := s.skillVersionPolicy()

	return c.JSON(http.StatusOK, map[string]interface{}{
		"writable_dir":  strings.TrimSpace(s.skillsMgr.SkillsDir()),
		"proxy":         strings.TrimSpace(s.skillsMgr.SkillsProxy()),
		"source_count":  len(sourceItems),
		"enabled_count": enabledCount,
		"always_count":  alwaysCount,
		"version_history": map[string]interface{}{
			"enabled":       versionPolicy.Enabled,
			"max_count":     versionPolicy.MaxCount,
			"skill_count":   versionStats["skill_count"],
			"version_count": versionStats["version_count"],
		},
		"sources": sourceItems,
	})
}

func (s *Server) handleListMarketplaceSkillSnapshots(c *echo.Context) error {
	if s.skillsMgr == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "skills manager not available"})
	}

	snapshotList, err := s.skillsMgr.ListSnapshots()
	if err != nil {
		s.logger.Error("Failed to list skill snapshots", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to list skill snapshots"})
	}

	snapshotPolicy := s.skillSnapshotPolicy()

	items := make([]map[string]interface{}, 0, len(snapshotList))
	for _, snapshot := range snapshotList {
		enabledCount := 0
		for _, item := range snapshot.Skills {
			if item.Enabled {
				enabledCount++
			}
		}

		items = append(items, map[string]interface{}{
			"id":            snapshot.ID,
			"timestamp":     snapshot.Timestamp.Format(time.RFC3339),
			"skill_count":   len(snapshot.Skills),
			"enabled_count": enabledCount,
			"metadata":      snapshot.Metadata,
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"total":      len(items),
		"snapshots":  items,
		"auto_prune": snapshotPolicy.AutoPrune,
		"max_count":  snapshotPolicy.MaxCount,
	})
}

func (s *Server) handleCreateMarketplaceSkillSnapshot(c *echo.Context) error {
	if s.skillsMgr == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "skills manager not available"})
	}

	var body struct {
		Label string `json:"label"`
		Note  string `json:"note"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	metadata := map[string]string{
		"source": "webui",
	}
	if label := strings.TrimSpace(body.Label); label != "" {
		metadata["label"] = label
	}
	if note := strings.TrimSpace(body.Note); note != "" {
		metadata["note"] = note
	}

	snapshot, err := s.skillsMgr.CreateSnapshot(metadata)
	if err != nil {
		s.logger.Error("Failed to create skill snapshot", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create skill snapshot"})
	}

	enabledCount := 0
	for _, item := range snapshot.Skills {
		if item.Enabled {
			enabledCount++
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"id":            snapshot.ID,
		"timestamp":     snapshot.Timestamp.Format(time.RFC3339),
		"skill_count":   len(snapshot.Skills),
		"enabled_count": enabledCount,
		"metadata":      snapshot.Metadata,
	})
}

func (s *Server) handlePruneMarketplaceSkillSnapshots(c *echo.Context) error {
	if s.skillsMgr == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "skills manager not available"})
	}

	snapshotPolicy := s.skillSnapshotPolicy()
	maxCount := snapshotPolicy.MaxCount
	if maxCount < 1 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "skill snapshot max_count must be at least 1"})
	}

	deleted, err := s.skillsMgr.PruneSnapshots(maxCount)
	if err != nil {
		s.logger.Error("Failed to prune skill snapshots", zap.Int("max_count", maxCount), zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to prune skill snapshots"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"deleted":    deleted,
		"max_count":  maxCount,
		"auto_prune": snapshotPolicy.AutoPrune,
	})
}

func (s *Server) handleCleanupMarketplaceSkillVersions(c *echo.Context) error {
	if s.skillsMgr == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "skills manager not available"})
	}

	policy := s.skillVersionPolicy()
	if !policy.Enabled {
		deleted, err := s.skillsMgr.ClearVersionHistory()
		if err != nil {
			s.logger.Error("Failed to clear skill version history", zap.Error(err))
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to clear skill version history"})
		}
		return c.JSON(http.StatusOK, map[string]interface{}{
			"deleted":   deleted,
			"max_count": policy.MaxCount,
			"enabled":   policy.Enabled,
			"mode":      "clear_all",
		})
	}

	if policy.MaxCount < 1 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "skill version history max_count must be at least 1"})
	}

	if err := s.skillsMgr.PruneVersionHistory(policy.MaxCount); err != nil {
		s.logger.Error("Failed to prune skill version history", zap.Int("max_count", policy.MaxCount), zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to prune skill version history"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"deleted":   0,
		"max_count": policy.MaxCount,
		"enabled":   policy.Enabled,
		"mode":      "prune_to_policy",
	})
}

func (s *Server) skillSnapshotPolicy() config.SkillSnapshotsConfig {
	if s.config == nil {
		return config.DefaultConfig().WebUI.SkillSnapshots
	}
	return s.config.WebUI.SkillSnapshots
}

func (s *Server) skillVersionPolicy() config.SkillVersionsConfig {
	if s.config == nil {
		return config.DefaultConfig().WebUI.SkillVersions
	}
	return s.config.WebUI.SkillVersions
}

func (s *Server) handleRestoreMarketplaceSkillSnapshot(c *echo.Context) error {
	if s.skillsMgr == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "skills manager not available"})
	}

	snapshotID := strings.TrimSpace(c.Param("id"))
	if snapshotID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "snapshot id is required"})
	}

	if err := s.skillsMgr.RestoreSnapshot(snapshotID); err != nil {
		if os.IsNotExist(err) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "snapshot not found"})
		}
		s.logger.Error("Failed to restore skill snapshot",
			zap.String("snapshot_id", snapshotID),
			zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to restore skill snapshot"})
	}

	return c.JSON(http.StatusOK, map[string]string{
		"id":     snapshotID,
		"status": "restored",
	})
}

func (s *Server) handleDeleteMarketplaceSkillSnapshot(c *echo.Context) error {
	if s.skillsMgr == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "skills manager not available"})
	}

	snapshotID := strings.TrimSpace(c.Param("id"))
	if snapshotID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "snapshot id is required"})
	}

	if err := s.skillsMgr.DeleteSnapshot(snapshotID); err != nil {
		if os.IsNotExist(err) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "snapshot not found"})
		}
		s.logger.Error("Failed to delete skill snapshot",
			zap.String("snapshot_id", snapshotID),
			zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to delete skill snapshot"})
	}

	return c.JSON(http.StatusOK, map[string]string{
		"id":     snapshotID,
		"status": "deleted",
	})
}

func (s *Server) marketplaceSkillItem(skill *skills.Skill) map[string]interface{} {
	var report *skills.SkillEntry
	if s != nil && s.skillsMgr != nil && skill != nil {
		report, _ = s.skillsMgr.CheckRequirementsReport(skill.ID)
	}
	if report == nil {
		report = &skills.SkillEntry{
			Skill:     skill,
			Eligible:  true,
			Installed: skill != nil && !strings.HasPrefix(strings.TrimSpace(skill.FilePath), "builtin://"),
		}
		if skill != nil && skill.Requirements != nil {
			eligible, reasons := skills.CheckEligibility(skill)
			report.Eligible = eligible
			report.Reasons = append([]string(nil), reasons...)
		}
	}

	tags := append([]string(nil), skill.Tags...)
	return map[string]interface{}{
		"id":                    strings.TrimSpace(skill.ID),
		"name":                  strings.TrimSpace(skill.Name),
		"description":           strings.TrimSpace(skill.Description),
		"version":               strings.TrimSpace(skill.Version),
		"author":                strings.TrimSpace(skill.Author),
		"enabled":               skill.Enabled,
		"always":                skill.Always,
		"file_path":             strings.TrimSpace(skill.FilePath),
		"tags":                  tags,
		"eligible":              report.Eligible,
		"ineligibility_reasons": report.Reasons,
		"missing_requirements": map[string]interface{}{
			"binaries":        nonNilStrings(report.MissingBinaries),
			"any_binaries":    nonNilStrings(report.MissingAnyBinaries),
			"env":             nonNilStrings(report.MissingEnvVars),
			"config_paths":    nonNilStrings(report.MissingPaths),
			"python_packages": nonNilStrings(report.MissingPythonPackages),
			"node_packages":   nonNilStrings(report.MissingNodePackages),
		},
		"install_specs": marketplaceInstallSpecs(skill),
		"is_installed":  report.Installed,
	}
}

func (s *Server) handleGetWorkspaceStatus(c *echo.Context) error {
	if s.workspace == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "workspace manager not available"})
	}

	status, err := s.workspace.Inspect()
	if err != nil {
		s.logger.Error("Failed to inspect workspace", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to inspect workspace"})
	}

	return c.JSON(http.StatusOK, status)
}

func (s *Server) handleRepairWorkspace(c *echo.Context) error {
	if s.workspace == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "workspace manager not available"})
	}

	if err := s.workspace.Ensure(); err != nil {
		s.logger.Error("Failed to repair workspace", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to repair workspace"})
	}

	status, err := s.workspace.Inspect()
	if err != nil {
		s.logger.Error("Failed to inspect workspace after repair", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to inspect workspace"})
	}

	return c.JSON(http.StatusOK, status)
}

func (s *Server) handleGetQMDStatus(c *echo.Context) error {
	qmdMgr, resolvedCommand, commandSource := s.newQMDManager()
	status := qmdMgr.GetStatus()
	exportDir := s.qmdResolvedExportDir()
	exportCount, _ := s.countQMDExportFiles(exportDir)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"enabled":                   s.config.Memory.QMD.Enabled,
		"command":                   strings.TrimSpace(s.config.Memory.QMD.Command),
		"resolved_command":          resolvedCommand,
		"command_source":            commandSource,
		"persistent_command":        s.persistentQMDBinary(),
		"include_default":           s.config.Memory.QMD.IncludeDefault,
		"available":                 status.Available,
		"version":                   status.Version,
		"error":                     status.Error,
		"last_update":               status.LastUpdate.Format(time.RFC3339),
		"collections":               status.Collections,
		"sessions_enabled":          s.config.Memory.QMD.Sessions.Enabled,
		"session_export_dir":        exportDir,
		"session_retention_days":    s.config.Memory.QMD.Sessions.RetentionDays,
		"session_export_file_count": exportCount,
	})
}

func (s *Server) handleInstallQMD(c *echo.Context) error {
	prefix := s.persistentQMDPrefix()
	if err := os.MkdirAll(prefix, 0o755); err != nil {
		s.logger.Error("Failed to create persistent qmd directory", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to prepare qmd install directory"})
	}

	cmd := exec.CommandContext(
		c.Request().Context(),
		defaultNPMCommand,
		"install",
		"-g",
		"--prefix",
		prefix,
		defaultQMDNPMPackage,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		s.logger.Error("Failed to install qmd",
			zap.String("prefix", prefix),
			zap.ByteString("output", output),
			zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":  "failed to install qmd",
			"output": strings.TrimSpace(string(output)),
		})
	}

	qmdMgr, resolvedCommand, commandSource := s.newQMDManager()
	status := qmdMgr.GetStatus()
	return c.JSON(http.StatusOK, map[string]interface{}{
		"installed":          true,
		"package":            defaultQMDNPMPackage,
		"prefix":             prefix,
		"binary":             s.persistentQMDBinary(),
		"command":            strings.TrimSpace(s.config.Memory.QMD.Command),
		"resolved_command":   resolvedCommand,
		"command_source":     commandSource,
		"persistent_command": s.persistentQMDBinary(),
		"available":          status.Available,
		"version":            status.Version,
		"error":              status.Error,
		"output":             strings.TrimSpace(string(output)),
	})
}

func (s *Server) handleUpdateQMD(c *echo.Context) error {
	qmdMgr, resolvedCommand, commandSource := s.newQMDManager()
	if !qmdMgr.IsAvailable() {
		status := qmdMgr.GetStatus()
		return c.JSON(http.StatusServiceUnavailable, map[string]interface{}{
			"error":            "qmd not available",
			"available":        false,
			"detail":           status.Error,
			"resolved_command": resolvedCommand,
			"command_source":   commandSource,
		})
	}

	ctx := c.Request().Context()
	if err := qmdMgr.Initialize(ctx, s.config.WorkspacePath()); err != nil {
		s.logger.Error("Failed to initialize qmd", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to initialize qmd"})
	}
	if err := qmdMgr.UpdateAll(ctx); err != nil {
		s.logger.Error("Failed to update qmd", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update qmd"})
	}

	status := qmdMgr.GetStatus()
	exportDir := s.qmdResolvedExportDir()
	exportCount, _ := s.countQMDExportFiles(exportDir)
	return c.JSON(http.StatusOK, map[string]interface{}{
		"enabled":                   s.config.Memory.QMD.Enabled,
		"command":                   strings.TrimSpace(s.config.Memory.QMD.Command),
		"resolved_command":          resolvedCommand,
		"command_source":            commandSource,
		"persistent_command":        s.persistentQMDBinary(),
		"include_default":           s.config.Memory.QMD.IncludeDefault,
		"available":                 status.Available,
		"version":                   status.Version,
		"error":                     status.Error,
		"last_update":               status.LastUpdate.Format(time.RFC3339),
		"collections":               status.Collections,
		"sessions_enabled":          s.config.Memory.QMD.Sessions.Enabled,
		"session_export_dir":        exportDir,
		"session_retention_days":    s.config.Memory.QMD.Sessions.RetentionDays,
		"session_export_file_count": exportCount,
	})
}

func (s *Server) handleCleanupQMDSessionExports(c *echo.Context) error {
	if !s.config.Memory.QMD.Sessions.Enabled {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "qmd session export is disabled"})
	}
	if s.config.Memory.QMD.Sessions.RetentionDays < 1 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "qmd session export retention_days must be at least 1"})
	}

	exportDir := s.qmdResolvedExportDir()
	exporter := memoryqmd.NewSessionExporter(s.logger, exportDir, s.config.Memory.QMD.Sessions.RetentionDays)
	deleted, err := exporter.CleanupOldExportsCount(c.Request().Context())
	if err != nil {
		s.logger.Error("Failed to cleanup qmd session exports", zap.String("export_dir", exportDir), zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to cleanup qmd session exports"})
	}

	remaining, countErr := s.countQMDExportFiles(exportDir)
	if countErr != nil {
		s.logger.Warn("Failed to count qmd session exports after cleanup", zap.String("export_dir", exportDir), zap.Error(countErr))
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"deleted":                deleted,
		"remaining":              remaining,
		"session_export_dir":     exportDir,
		"session_retention_days": s.config.Memory.QMD.Sessions.RetentionDays,
	})
}

func (s *Server) newQMDManager() (*memoryqmd.Manager, string, string) {
	qmdCfg := memoryqmd.ConfigFromConfigWithWorkspace(s.config.Memory.QMD, s.config.WorkspacePath())
	resolvedCommand, commandSource := s.resolveQMDCommand()
	qmdCfg.Command = resolvedCommand
	return memoryqmd.NewManager(s.logger, qmdCfg), resolvedCommand, commandSource
}

func (s *Server) persistentQMDPrefix() string {
	return filepath.Join(s.config.WorkspacePath(), ".nekobot", "runtime", "qmd")
}

func (s *Server) qmdResolvedExportDir() string {
	return memoryqmd.ConfigFromConfigWithWorkspace(s.config.Memory.QMD, s.config.WorkspacePath()).Sessions.ExportDir
}

func (s *Server) countQMDExportFiles(exportDir string) (int, error) {
	if strings.TrimSpace(exportDir) == "" {
		return 0, nil
	}

	entries, err := os.ReadDir(exportDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	count := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".md") {
			continue
		}
		count++
	}
	return count, nil
}

func (s *Server) persistentQMDBinary() string {
	return filepath.Join(s.persistentQMDPrefix(), "bin", "qmd")
}

func (s *Server) resolveQMDCommand() (string, string) {
	configured := strings.TrimSpace(s.config.Memory.QMD.Command)
	if configured != "" && configured != "qmd" {
		return configured, "config"
	}

	persistentBinary := s.persistentQMDBinary()
	if info, err := os.Stat(persistentBinary); err == nil && !info.IsDir() {
		return persistentBinary, "workspace"
	}

	if configured != "" {
		return configured, "config"
	}

	return "qmd", "path"
}

func marketplaceInstallSpecs(skill *skills.Skill) []map[string]interface{} {
	if skill == nil || skill.Requirements == nil {
		return nil
	}

	specs := skills.ParseRequirementsToSpecs(skill.Requirements)
	if len(specs) == 0 {
		return nil
	}

	out := make([]map[string]interface{}, 0, len(specs))
	for _, spec := range specs {
		out = append(out, map[string]interface{}{
			"method":    strings.TrimSpace(spec.Method),
			"package":   strings.TrimSpace(spec.Package),
			"version":   strings.TrimSpace(spec.Version),
			"post_hook": strings.TrimSpace(spec.PostHook),
			"options":   spec.Options,
		})
	}
	return out
}

func readMarketplaceSkillContent(skill *skills.Skill) (string, string, error) {
	filePath := strings.TrimSpace(skill.FilePath)
	var raw string
	if strings.HasPrefix(filePath, "builtin://") {
		builtinRaw, err := skills.ReadBuiltinSkillContent(filePath)
		if err != nil {
			return "", "", err
		}
		raw = builtinRaw
	} else {
		rawBytes, err := os.ReadFile(filePath)
		if err != nil {
			return "", "", fmt.Errorf("read skill file %s: %w", filePath, err)
		}
		raw = string(rawBytes)
	}

	bodyRaw := strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "---\n") {
		parts := strings.SplitN(raw[4:], "\n---\n", 2)
		if len(parts) == 2 {
			bodyRaw = strings.TrimSpace(parts[1])
		}
	}

	return raw, bodyRaw, nil
}

func (s *Server) handleEnableMarketplaceSkill(c *echo.Context) error {
	if s.skillsMgr == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "skills manager not available"})
	}

	skillID := strings.TrimSpace(c.Param("id"))
	if _, ok := s.resolveMarketplaceSkill(skillID); !ok {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "skill not found"})
	}
	if err := s.skillsMgr.Enable(skillID); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to enable skill"})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "enabled"})
}

func (s *Server) handleDisableMarketplaceSkill(c *echo.Context) error {
	if s.skillsMgr == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "skills manager not available"})
	}

	skillID := strings.TrimSpace(c.Param("id"))
	if _, ok := s.resolveMarketplaceSkill(skillID); !ok {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "skill not found"})
	}
	if err := s.skillsMgr.Disable(skillID); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to disable skill"})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "disabled"})
}

func (s *Server) handleInstallMarketplaceSkillDependencies(c *echo.Context) error {
	if s.skillsMgr == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "skills manager not available"})
	}

	skillID := strings.TrimSpace(c.Param("id"))
	if _, ok := s.resolveMarketplaceSkill(skillID); !ok {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "skill not found"})
	}

	results, err := s.skillsMgr.InstallDependencies(c.Request().Context(), skillID)
	if err != nil {
		s.logger.Error("Failed to install marketplace skill dependencies",
			zap.String("skill_id", skillID),
			zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to install dependencies"})
	}

	success := true
	payload := make([]map[string]interface{}, 0, len(results))
	for _, result := range results {
		if !result.Success {
			success = false
		}
		payload = append(payload, map[string]interface{}{
			"success":      result.Success,
			"method":       result.Method,
			"package":      result.Package,
			"output":       result.Output,
			"error":        errorString(result.Error),
			"duration_ms":  result.Duration.Milliseconds(),
			"installed_at": result.InstalledAt,
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"skill_id": skillID,
		"success":  success,
		"results":  payload,
	})
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

// --- Channel Handlers ---

func (s *Server) handleGetChannels(c *echo.Context) error {
	payload := map[string]interface{}{}
	for name, cfg := range channels.ListChannelConfigs(s.config) {
		payload[name] = cfg
	}
	payload["_instances"] = s.listChannelInstances()
	return c.JSON(http.StatusOK, payload)
}

func (s *Server) handleUpdateChannel(c *echo.Context) error {
	name := c.Param("name")

	var body map[string]interface{}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	// Marshal the body to JSON, then unmarshal into the appropriate channel config.
	data, err := json.Marshal(body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	nextConfig, err := cloneConfigSnapshot(s.config)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	if err := channels.ApplyChannelConfig(nextConfig, name, data); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	if err := config.ValidateConfig(nextConfig); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	enabled, err := channels.IsChannelEnabled(name, nextConfig)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	var rebuilt channels.Channel
	if enabled {
		rebuilt, err = channels.BuildChannel(
			name,
			s.logger,
			s.bus,
			s.agent,
			s.commands,
			s.prefs,
			s.toolSess,
			s.processMgr,
			nextConfig,
		)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
	}

	// Persist runtime channel config to database.
	if err := config.SaveDatabaseSections(nextConfig, "channels"); err != nil {
		s.logger.Error("Failed to persist channel config", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to save config"})
	}

	if enabled {
		if err := s.channels.ReloadChannel(rebuilt); err != nil {
			s.logger.Error("Failed to reload channel", zap.String("channel", name), zap.Error(err))
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "channel config saved but reload failed: " + err.Error(),
			})
		}
	} else {
		if err := s.channels.StopChannel(name); err != nil {
			s.logger.Error("Failed to stop channel", zap.String("channel", name), zap.Error(err))
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "channel config saved but reload failed: " + err.Error(),
			})
		}
	}
	s.config.Channels = nextConfig.Channels

	return c.JSON(http.StatusOK, map[string]string{"status": "updated", "channel": name, "reload": "ok"})
}

func (s *Server) listChannelInstances() []map[string]interface{} {
	if s.channels == nil {
		return []map[string]interface{}{}
	}

	items := s.channels.ListChannels()
	result := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		entry := map[string]interface{}{
			"id":      item.ID(),
			"name":    item.Name(),
			"enabled": item.IsEnabled(),
			"type":    item.ID(),
		}
		if typed, ok := item.(channels.TypedChannel); ok {
			entry["type"] = typed.ChannelType()
		}
		result = append(result, entry)
	}
	return result
}

func (s *Server) reloadChannel(name string) error {
	enabled, err := channels.IsChannelEnabled(name, s.config)
	if err != nil {
		return err
	}

	if !enabled {
		return s.channels.StopChannel(name)
	}

	ch, err := channels.BuildChannel(name, s.logger, s.bus, s.agent, s.commands, s.prefs, s.toolSess, s.processMgr, s.config)
	if err != nil {
		return err
	}

	return s.channels.ReloadChannel(ch)
}

func (s *Server) reloadChannelsByType(channelType string) error {
	if s == nil || s.channels == nil {
		return fmt.Errorf("channel manager is unavailable")
	}

	channelType = strings.TrimSpace(channelType)
	if channelType == "" {
		return fmt.Errorf("channel type is required")
	}

	if s.accountMgr != nil {
		accounts, err := s.accountMgr.List(context.Background())
		if err != nil {
			return fmt.Errorf("list channel accounts for %s: %w", channelType, err)
		}

		desiredIDs := make(map[string]struct{})
		desiredAccounts := make([]channelaccounts.ChannelAccount, 0)
		for _, account := range accounts {
			if account.ChannelType != channelType || !account.Enabled {
				continue
			}
			runtimeID := strings.TrimSpace(account.ChannelType) + ":" + strings.TrimSpace(account.AccountKey)
			desiredIDs[runtimeID] = struct{}{}
			desiredAccounts = append(desiredAccounts, account)
		}
		if channelType == "wechat" {
			desiredAccounts = s.prioritizeActiveWechatAccount(desiredAccounts)
		}

		existingChannels := s.channels.ListChannelsByType(channelType)
		if len(desiredAccounts) > 0 {
			for _, existing := range existingChannels {
				if _, ok := desiredIDs[existing.ID()]; ok {
					continue
				}
				if err := s.channels.StopChannel(existing.ID()); err != nil {
					return fmt.Errorf("stop stale %s runtime %s: %w", channelType, existing.ID(), err)
				}
			}

			for _, account := range desiredAccounts {
				ch, err := channels.BuildChannelFromAccount(
					account,
					s.logger,
					s.bus,
					s.agent,
					s.commands,
					s.prefs,
					s.toolSess,
					s.processMgr,
					s.config,
				)
				if err != nil {
					return fmt.Errorf("build %s account runtime %s: %w", channelType, account.AccountKey, err)
				}
				if err := s.channels.ReloadChannel(ch); err != nil {
					return fmt.Errorf("reload %s account runtime %s: %w", channelType, ch.ID(), err)
				}
			}
			return nil
		}

		for _, existing := range existingChannels {
			if existing.ID() == channelType {
				continue
			}
			if err := s.channels.StopChannel(existing.ID()); err != nil {
				return fmt.Errorf("stop stale %s runtime %s: %w", channelType, existing.ID(), err)
			}
		}
	}

	return s.reloadChannel(channelType)
}

func (s *Server) prioritizeActiveWechatAccount(
	accounts []channelaccounts.ChannelAccount,
) []channelaccounts.ChannelAccount {
	if len(accounts) < 2 || s == nil || s.config == nil {
		return accounts
	}

	store, err := channelwechat.NewCredentialStore(s.config)
	if err != nil {
		return accounts
	}
	creds, err := store.LoadCredentials()
	if err != nil || creds == nil {
		return accounts
	}

	activeBotID := strings.TrimSpace(creds.ILinkBotID)
	if activeBotID == "" {
		return accounts
	}

	activeIndex := -1
	for i, account := range accounts {
		if strings.TrimSpace(account.AccountKey) == activeBotID {
			activeIndex = i
			break
		}
	}
	if activeIndex <= 0 {
		return accounts
	}

	prioritized := make([]channelaccounts.ChannelAccount, 0, len(accounts))
	prioritized = append(prioritized, accounts[activeIndex])
	prioritized = append(prioritized, accounts[:activeIndex]...)
	prioritized = append(prioritized, accounts[activeIndex+1:]...)
	return prioritized
}

func (s *Server) reloadChannelForAccount(channelType, accountID string) error {
	channelType = strings.TrimSpace(channelType)
	accountID = strings.TrimSpace(accountID)
	if channelType == "" || accountID == "" || s.accountMgr == nil {
		return s.reloadChannel(channelType)
	}

	account, err := s.accountMgr.Get(context.Background(), accountID)
	if err != nil {
		return s.reloadChannel(channelType)
	}
	if account == nil || strings.TrimSpace(account.ChannelType) != channelType {
		return s.reloadChannel(channelType)
	}

	runtimeID := strings.TrimSpace(account.ChannelType) + ":" + strings.TrimSpace(account.AccountKey)
	if !account.Enabled {
		return s.channels.StopChannel(runtimeID)
	}

	ch, err := channels.BuildChannelFromAccount(*account, s.logger, s.bus, s.agent, s.commands, s.prefs, s.toolSess, s.processMgr, s.config)
	if err != nil {
		return err
	}

	return s.channels.ReloadChannel(ch)
}

func (s *Server) handleTestChannel(c *echo.Context) error {
	name := c.Param("name")

	ch, err := s.channels.GetChannel(name)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"channel": name,
			"status":  "not_found",
			"error":   err.Error(),
		})
	}

	result := map[string]interface{}{
		"channel":   ch.Name(),
		"id":        ch.ID(),
		"enabled":   ch.IsEnabled(),
		"reachable": false,
	}

	if !ch.IsEnabled() {
		result["status"] = "disabled"
		return c.JSON(http.StatusOK, result)
	}

	healthChecker, ok := ch.(channels.HealthChecker)
	if !ok {
		result["status"] = "configured"
		return c.JSON(http.StatusOK, result)
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 10*time.Second)
	defer cancel()

	if err := healthChecker.HealthCheck(ctx); err != nil {
		result["status"] = "unreachable"
		result["error"] = err.Error()
		return c.JSON(http.StatusOK, result)
	}

	result["reachable"] = true
	result["status"] = "ok"
	return c.JSON(http.StatusOK, result)
}

type webWechatLoginClient struct{}

func (webWechatLoginClient) FetchQRCode(ctx context.Context) (*wxtypes.QRCodeResponse, error) {
	return wxauth.FetchQRCode(ctx)
}

func (webWechatLoginClient) CheckQRStatus(ctx context.Context, qrcode string) (*wxtypes.QRStatusResponse, error) {
	return wxauth.CheckQRStatus(ctx, qrcode)
}

func (s *Server) ilinkAuthService() (*ilinkauth.Service, error) {
	if s.ilinkAuth != nil {
		return s.ilinkAuth, nil
	}

	store, err := ilinkauth.NewStore(s.config)
	if err != nil {
		return nil, fmt.Errorf("create ilink auth store: %w", err)
	}
	s.ilinkAuth = ilinkauth.NewService(store, webWechatLoginClient{})
	return s.ilinkAuth, nil
}

func (s *Server) currentAuthUser(c *echo.Context) (*config.AuthProfile, error) {
	profile, err := s.authProfileFromContext(c)
	if err == nil {
		return profile, nil
	}

	userID := strings.TrimSpace(s.currentUserID(c))
	username := strings.TrimSpace(s.currentUsername(c))
	if userID == "" && username == "" {
		return nil, err
	}

	return &config.AuthProfile{
		UserID:   userID,
		Username: username,
		Role:     "member",
	}, nil
}

func (s *Server) handleGetWechatBindingStatus(c *echo.Context) error {
	authSvc, err := s.ilinkAuthService()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	profile, err := s.currentAuthUser(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": err.Error()})
	}

	payload, err := s.buildWechatBindingPayload(authSvc, profile.UserID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, payload)
}

func (s *Server) handleStartWechatBinding(c *echo.Context) error {
	authSvc, err := s.ilinkAuthService()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	profile, err := s.currentAuthUser(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": err.Error()})
	}

	if _, err := authSvc.StartBinding(c.Request().Context(), profile.UserID); err != nil {
		return c.JSON(http.StatusBadGateway, map[string]string{"error": err.Error()})
	}

	payload, err := s.buildWechatBindingPayload(authSvc, profile.UserID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, payload)
}

func (s *Server) handlePollWechatBinding(c *echo.Context) error {
	authSvc, err := s.ilinkAuthService()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	profile, err := s.currentAuthUser(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": err.Error()})
	}

	if _, err := authSvc.PollBinding(c.Request().Context(), profile.UserID); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no active ilink binding") {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "no active wechat binding"})
		}
		return c.JSON(http.StatusBadGateway, map[string]string{"error": err.Error()})
	}

	if err := s.syncWechatBindingToAccounts(c.Request().Context(), authSvc, profile.UserID); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	if s.config.Channels.WeChat.Enabled {
		if err := s.reloadChannelsByType("wechat"); err != nil {
			s.logger.Error("Failed to reload WeChat after binding", zap.Error(err))
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
	}

	payload, err := s.buildWechatBindingPayload(authSvc, profile.UserID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, payload)
}

func (s *Server) handleDeleteWechatBinding(c *echo.Context) error {
	authSvc, err := s.ilinkAuthService()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	profile, err := s.currentAuthUser(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": err.Error()})
	}

	if _, err := authSvc.GetBinding(profile.UserID); err != nil {
		if errors.Is(err, ilinkauth.ErrBindingNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "no active wechat binding"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	if err := authSvc.DeleteBinding(profile.UserID); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	if err := s.deleteWechatChannelAccount(c.Request().Context(), profile.UserID); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	if s.config.Channels.WeChat.Enabled {
		if err := s.reloadChannelsByType("wechat"); err != nil {
			s.logger.Error("Failed to reload WeChat after unbind", zap.Error(err))
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleActivateWechatBinding(c *echo.Context) error {
	if s.accountMgr == nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "channel account manager is unavailable"})
	}

	accountID := strings.TrimSpace(c.FormValue("account_id"))
	if accountID == "" {
		var body struct {
			AccountID string `json:"account_id"`
		}
		if err := c.Bind(&body); err == nil {
			accountID = strings.TrimSpace(body.AccountID)
		}
	}
	if accountID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "account_id is required"})
	}

	profile, err := s.currentAuthUser(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": err.Error()})
	}

	if err := s.activateWechatChannelAccount(c.Request().Context(), profile.UserID, accountID); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	if s.config.Channels.WeChat.Enabled {
		if err := s.reloadChannelsByType("wechat"); err != nil {
			s.logger.Error("Failed to reload WeChat after activation", zap.Error(err))
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
	}

	authSvc, err := s.ilinkAuthService()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	payload, err := s.buildWechatBindingPayload(authSvc, profile.UserID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, payload)
}

func (s *Server) handleDeleteWechatBindingAccount(c *echo.Context) error {
	if s.accountMgr == nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "channel account manager is unavailable"})
	}

	accountID := strings.TrimSpace(c.Param("accountId"))
	if accountID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "accountId is required"})
	}
	profile, err := s.currentAuthUser(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": err.Error()})
	}
	if err := s.removeWechatChannelAccount(c.Request().Context(), profile.UserID, accountID); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	if s.config.Channels.WeChat.Enabled {
		if err := s.reloadChannelsByType("wechat"); err != nil {
			s.logger.Error("Failed to reload WeChat after account delete", zap.Error(err))
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
	}

	authSvc, err := s.ilinkAuthService()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	payload, err := s.buildWechatBindingPayload(authSvc, profile.UserID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, payload)
}

func (s *Server) buildWechatBindingPayload(authSvc *ilinkauth.Service, userID string) (map[string]interface{}, error) {
	payload := map[string]interface{}{
		"bound": false,
	}

	binding, err := authSvc.GetBinding(userID)
	if err != nil && !errors.Is(err, ilinkauth.ErrBindingNotFound) {
		return nil, fmt.Errorf("load ilink binding: %w", err)
	}
	activeAccountBotID := ""
	if binding != nil {
		payload["bound"] = true
		activeAccountBotID = strings.TrimSpace(binding.Credentials.ILinkBotID)
		payload["account"] = map[string]interface{}{
			"bot_id":  binding.Credentials.ILinkBotID,
			"user_id": binding.Credentials.ILinkUserID,
		}
	}

	if s.accountMgr != nil {
		accounts, err := s.accountMgr.List(context.Background())
		if err != nil {
			return nil, fmt.Errorf("list wechat channel accounts: %w", err)
		}
		items := make([]map[string]interface{}, 0, len(accounts))
		for _, account := range accounts {
			if account.ChannelType != "wechat" {
				continue
			}
			if owner, _ := account.Metadata["owner_user_id"].(string); strings.TrimSpace(owner) != strings.TrimSpace(userID) {
				continue
			}
			botID, _ := account.Config["ilink_bot_id"].(string)
			ilinkUserID, _ := account.Config["ilink_user_id"].(string)
			items = append(items, map[string]interface{}{
				"account_id": account.ID,
				"bot_id":     botID,
				"user_id":    ilinkUserID,
				"active":     activeAccountBotID != "" && strings.TrimSpace(botID) == activeAccountBotID,
			})
			if activeAccountBotID != "" && strings.TrimSpace(botID) == activeAccountBotID {
				payload["active_account_id"] = account.ID
			}
		}
		payload["accounts"] = items
	}

	state, err := authSvc.LoadBindSession(userID)
	if err != nil {
		return nil, fmt.Errorf("load ilink bind session: %w", err)
	}
	if state != nil {
		bind := map[string]interface{}{
			"status":         state.Status,
			"qrcode_content": state.QRCodeContent,
			"updated_at":     state.UpdatedAt,
			"bot_id":         state.BotID,
			"user_id":        state.ILinkUserID,
			"error":          state.Error,
		}
		if strings.TrimSpace(state.QRCodeContent) != "" {
			if dataURL, err := encodeQRCodeDataURL(state.QRCodeContent); err == nil {
				bind["qr_png_data_url"] = dataURL
			}
		}
		payload["binding"] = bind
	}

	return payload, nil
}

func (s *Server) syncWechatBindingToAccounts(ctx context.Context, authSvc *ilinkauth.Service, userID string) error {
	if s == nil || s.accountMgr == nil {
		return nil
	}

	binding, err := authSvc.GetBinding(userID)
	if err != nil {
		if errors.Is(err, ilinkauth.ErrBindingNotFound) {
			return nil
		}
		return fmt.Errorf("get ilink binding: %w", err)
	}
	if binding == nil {
		return nil
	}

	store, err := channelwechat.NewCredentialStore(s.config)
	if err != nil {
		return fmt.Errorf("create wechat credential store: %w", err)
	}
	if err := store.SaveCredentials((*channelwechat.Credentials)(&binding.Credentials), true); err != nil {
		return fmt.Errorf("save wechat credentials: %w", err)
	}

	accountKey := strings.TrimSpace(binding.Credentials.ILinkBotID)
	if accountKey == "" {
		return fmt.Errorf("ilink bot id is empty")
	}

	accounts, err := s.accountMgr.List(ctx)
	if err != nil {
		return fmt.Errorf("list channel accounts: %w", err)
	}

	payload := channelaccounts.ChannelAccount{
		ChannelType: "wechat",
		AccountKey:  accountKey,
		DisplayName: accountKey,
		Enabled:     true,
		Config: map[string]interface{}{
			"enabled":       true,
			"ilink_bot_id":  binding.Credentials.ILinkBotID,
			"ilink_user_id": binding.Credentials.ILinkUserID,
			"base_url":      binding.Credentials.BaseURL,
			"bot_token":     binding.Credentials.BotToken,
		},
		Metadata: map[string]interface{}{
			"owner_user_id": strings.TrimSpace(userID),
			"source":        "ilinkauth",
		},
	}

	for _, item := range accounts {
		if item.ChannelType != "wechat" || item.AccountKey != accountKey {
			continue
		}
		_, err := s.accountMgr.Update(ctx, item.ID, payload)
		if err != nil {
			return fmt.Errorf("update wechat channel account %s: %w", item.ID, err)
		}
		return nil
	}

	if _, err := s.accountMgr.Create(ctx, payload); err != nil {
		return fmt.Errorf("create wechat channel account: %w", err)
	}
	return nil
}

func (s *Server) deleteWechatChannelAccount(ctx context.Context, userID string) error {
	if s == nil || s.accountMgr == nil {
		return nil
	}
	accounts, err := s.accountMgr.List(ctx)
	if err != nil {
		return fmt.Errorf("list wechat channel accounts: %w", err)
	}
	for _, item := range accounts {
		if item.ChannelType != "wechat" {
			continue
		}
		if owner, _ := item.Metadata["owner_user_id"].(string); strings.TrimSpace(owner) != strings.TrimSpace(userID) {
			continue
		}
		if s.bindingMgr != nil {
			if err := s.bindingMgr.DeleteByChannelAccountID(ctx, item.ID); err != nil {
				return fmt.Errorf("delete wechat bindings for channel account %s: %w", item.ID, err)
			}
		}
		if err := s.accountMgr.Delete(ctx, item.ID); err != nil {
			return fmt.Errorf("delete wechat channel account %s: %w", item.ID, err)
		}
	}
	return nil
}

func (s *Server) activateWechatChannelAccount(ctx context.Context, userID, accountID string) error {
	authSvc, err := s.ilinkAuthService()
	if err != nil {
		return err
	}
	account, err := s.accountMgr.Get(ctx, accountID)
	if err != nil {
		return err
	}
	if account == nil || account.ChannelType != "wechat" {
		return fmt.Errorf("wechat account not found")
	}
	if owner, _ := account.Metadata["owner_user_id"].(string); strings.TrimSpace(owner) != strings.TrimSpace(userID) {
		return fmt.Errorf("wechat account does not belong to current user")
	}

	botID, _ := account.Config["ilink_bot_id"].(string)
	botToken, _ := account.Config["bot_token"].(string)
	ilinkUserID, _ := account.Config["ilink_user_id"].(string)
	baseURL, _ := account.Config["base_url"].(string)
	if strings.TrimSpace(botID) == "" || strings.TrimSpace(botToken) == "" {
		return fmt.Errorf("wechat account credentials are incomplete")
	}

	store, err := channelwechat.NewCredentialStore(s.config)
	if err != nil {
		return fmt.Errorf("create wechat credential store: %w", err)
	}
	if err := store.SetActiveAccount(botID); err != nil {
		return fmt.Errorf("set active wechat account: %w", err)
	}

	if err := authSvc.DeleteBinding(userID); err != nil {
		return err
	}

	return authSvc.SaveBinding(&ilinkauth.Binding{
		UserID: strings.TrimSpace(userID),
		Credentials: wxtypes.Credentials{
			BotToken:    botToken,
			ILinkBotID:  botID,
			BaseURL:     baseURL,
			ILinkUserID: ilinkUserID,
		},
	})
}

func (s *Server) removeWechatChannelAccount(ctx context.Context, userID, accountID string) error {
	authSvc, err := s.ilinkAuthService()
	if err != nil {
		return err
	}
	account, err := s.accountMgr.Get(ctx, accountID)
	if err != nil {
		return err
	}
	if account == nil || account.ChannelType != "wechat" {
		return fmt.Errorf("wechat account not found")
	}
	if owner, _ := account.Metadata["owner_user_id"].(string); strings.TrimSpace(owner) != strings.TrimSpace(userID) {
		return fmt.Errorf("wechat account does not belong to current user")
	}
	if s.bindingMgr != nil {
		if err := s.bindingMgr.DeleteByChannelAccountID(ctx, account.ID); err != nil {
			return err
		}
	}
	if err := s.accountMgr.Delete(ctx, account.ID); err != nil {
		return err
	}

	store, err := channelwechat.NewCredentialStore(s.config)
	if err != nil {
		return fmt.Errorf("create wechat credential store: %w", err)
	}
	if err := store.DeleteCredentials(account.AccountKey); err != nil {
		return fmt.Errorf("delete wechat local credentials: %w", err)
	}

	active, err := authSvc.GetBinding(userID)
	if err != nil && !errors.Is(err, ilinkauth.ErrBindingNotFound) {
		return err
	}
	if active != nil && strings.TrimSpace(active.Credentials.ILinkBotID) == strings.TrimSpace(account.AccountKey) {
		if err := authSvc.DeleteBinding(userID); err != nil {
			return err
		}

		remaining, listErr := s.accountMgr.List(ctx)
		if listErr != nil {
			return listErr
		}
		for _, item := range remaining {
			if item.ChannelType != "wechat" {
				continue
			}
			if owner, _ := item.Metadata["owner_user_id"].(string); strings.TrimSpace(owner) != strings.TrimSpace(userID) {
				continue
			}
			return s.activateWechatChannelAccount(ctx, userID, item.ID)
		}
	}
	return nil
}

func encodeQRCodeDataURL(content string) (string, error) {
	code, err := rscqr.Encode(content, rscqr.M)
	if err != nil {
		return "", fmt.Errorf("encode QR code: %w", err)
	}
	code.Scale = 8
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(code.PNG()), nil
}

// --- Config Handlers ---

func (s *Server) handleGetConfig(c *echo.Context) error {
	// Return sanitized config (no secrets)
	return c.JSON(http.StatusOK, map[string]interface{}{
		"storage":       s.config.Storage,
		"agents":        s.config.Agents,
		"gateway":       s.config.Gateway,
		"tools":         s.config.Tools,
		"heartbeat":     s.config.Heartbeat,
		"webhook":       s.config.Webhook,
		"redis":         s.config.Redis,
		"state":         s.config.State,
		"bus":           s.config.Bus,
		"approval":      s.config.Approval,
		"logger":        s.config.Logger,
		"memory":        s.config.Memory,
		"sessions":      s.config.Sessions,
		"webui":         s.config.WebUI,
		"transcription": s.config.Transcription,
		"audit":         s.config.Audit,
		"undo":          s.config.Undo,
		"preprocess":    s.config.Preprocess,
		"learnings":     s.config.Learnings,
		"watch":         s.config.Watch,
	})
}

func (s *Server) handleSaveConfig(c *echo.Context) error {
	previousStorage := s.config.Storage
	oldRuntimeDBPath := ""
	if currentPath, err := config.RuntimeDBPath(s.config); err == nil {
		oldRuntimeDBPath = currentPath
	}

	var body struct {
		Storage       *config.StorageConfig       `json:"storage"`
		Agents        *config.AgentsConfig        `json:"agents"`
		Gateway       *config.GatewayConfig       `json:"gateway"`
		Tools         *config.ToolsConfig         `json:"tools"`
		Heartbeat     *config.HeartbeatConfig     `json:"heartbeat"`
		Webhook       *config.WebhookConfig       `json:"webhook"`
		Redis         *config.RedisConfig         `json:"redis"`
		State         *config.StateConfig         `json:"state"`
		Bus           *config.BusConfig           `json:"bus"`
		Approval      *config.ApprovalConfig      `json:"approval"`
		Logger        *config.LoggerConfig        `json:"logger"`
		Memory        *config.MemoryConfig        `json:"memory"`
		Sessions      *config.SessionsConfig      `json:"sessions"`
		WebUI         *config.WebUIConfig         `json:"webui"`
		Transcription *config.TranscriptionConfig `json:"transcription"`
		Audit         *config.AuditConfig         `json:"audit"`
		Undo          *config.UndoConfig          `json:"undo"`
		Preprocess    *config.PreprocessConfig    `json:"preprocess"`
		Learnings     *config.LearningsConfig     `json:"learnings"`
		Watch         *config.WatchConfig         `json:"watch"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	// Apply partial updates
	if body.Storage != nil {
		s.config.Storage = *body.Storage
	}
	if body.Agents != nil {
		s.config.Agents = *body.Agents
	}
	if body.Gateway != nil {
		s.config.Gateway = *body.Gateway
	}
	if body.Tools != nil {
		s.config.Tools = *body.Tools
	}
	if body.Heartbeat != nil {
		s.config.Heartbeat = *body.Heartbeat
	}
	if body.Webhook != nil {
		s.config.Webhook = *body.Webhook
	}
	if body.Redis != nil {
		s.config.Redis = *body.Redis
	}
	if body.State != nil {
		s.config.State = *body.State
	}
	if body.Bus != nil {
		s.config.Bus = *body.Bus
	}
	if body.Approval != nil {
		s.config.Approval = *body.Approval
	}
	if body.Logger != nil {
		s.config.Logger = *body.Logger
	}
	if body.Memory != nil {
		s.config.Memory = *body.Memory
	}
	if body.Sessions != nil {
		s.config.Sessions = *body.Sessions
	}
	if body.WebUI != nil {
		s.config.WebUI = *body.WebUI
		if s.toolSess != nil {
			s.toolSess.SetEventConfig(s.config.WebUI.ToolSessionEvents)
		}
		if s.skillsMgr != nil {
			s.skillsMgr.SetSnapshotRetention(skills.SnapshotRetentionConfig{
				AutoPrune: s.config.WebUI.SkillSnapshots.AutoPrune,
				MaxCount:  s.config.WebUI.SkillSnapshots.MaxCount,
			})
			s.skillsMgr.SetVersionRetention(skills.VersionRetentionConfig{
				Enabled:  s.config.WebUI.SkillVersions.Enabled,
				MaxCount: s.config.WebUI.SkillVersions.MaxCount,
			})
		}
	}
	if body.Transcription != nil {
		s.config.Transcription = *body.Transcription
	}
	if body.Audit != nil {
		s.config.Audit = *body.Audit
	}
	if body.Undo != nil {
		s.config.Undo = *body.Undo
	}
	if body.Preprocess != nil {
		s.config.Preprocess = *body.Preprocess
	}
	if body.Learnings != nil {
		s.config.Learnings = *body.Learnings
	}
	if body.Watch != nil {
		s.config.Watch = *body.Watch
	}

	// Validate
	if err := config.ValidateConfig(s.config); err != nil {
		s.config.Storage = previousStorage
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	restartSections := make([]string, 0, 5)
	if body.Storage != nil || body.Logger != nil || body.Gateway != nil || body.WebUI != nil || body.Webhook != nil {
		if body.Storage != nil {
			newRuntimeDBPath, err := config.RuntimeDBPath(s.config)
			if err != nil {
				s.config.Storage = previousStorage
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			if err := config.MigrateRuntimeDB(oldRuntimeDBPath, newRuntimeDBPath); err != nil {
				s.config.Storage = previousStorage
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
		}
		if err := s.saveBootstrapConfig(); err != nil {
			s.config.Storage = previousStorage
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		if body.Storage != nil {
			restartSections = append(restartSections, "storage")
		}
		if body.Logger != nil {
			restartSections = append(restartSections, "logger")
		}
		if body.Gateway != nil {
			restartSections = append(restartSections, "gateway")
		}
		if body.WebUI != nil {
			restartSections = append(restartSections, "webui")
		}
		if body.Webhook != nil {
			restartSections = append(restartSections, "webhook")
		}
	}

	sections := make([]string, 0, 19)
	if body.Agents != nil {
		sections = append(sections, "agents")
	}
	if body.Gateway != nil {
		sections = append(sections, "gateway")
	}
	if body.Tools != nil {
		sections = append(sections, "tools")
	}
	if body.Heartbeat != nil {
		sections = append(sections, "heartbeat")
	}
	if body.Webhook != nil {
		sections = append(sections, "webhook")
	}
	if body.Redis != nil {
		sections = append(sections, "redis")
	}
	if body.State != nil {
		sections = append(sections, "state")
	}
	if body.Bus != nil {
		sections = append(sections, "bus")
	}
	if body.Approval != nil {
		sections = append(sections, "approval")
	}
	if body.Logger != nil {
		sections = append(sections, "logger")
	}
	if body.Memory != nil {
		sections = append(sections, "memory")
	}
	if body.Sessions != nil {
		sections = append(sections, "sessions")
	}
	if body.WebUI != nil {
		sections = append(sections, "webui")
	}
	if body.Transcription != nil {
		sections = append(sections, "transcription")
	}
	if body.Audit != nil {
		sections = append(sections, "audit")
	}
	if body.Undo != nil {
		sections = append(sections, "undo")
	}
	if body.Preprocess != nil {
		sections = append(sections, "preprocess")
	}
	if body.Learnings != nil {
		sections = append(sections, "learnings")
	}
	if body.Watch != nil {
		sections = append(sections, "watch")
	}

	// Persist runtime config sections to database.
	if len(sections) > 0 {
		if err := config.SaveDatabaseSections(s.config, sections...); err != nil {
			s.logger.Error("Failed to persist config sections", zap.Error(err), zap.Strings("sections", sections))
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to save config"})
		}
	}
	if body.Watch != nil {
		if err := s.syncWatchRuntime(); err != nil {
			if s.logger != nil {
				s.logger.Error("Failed to sync watch runtime after config save", zap.Error(err))
			}
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to sync watch runtime"})
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":           "saved",
		"sections_saved":   len(sections),
		"restart_required": len(restartSections) > 0,
		"restart_sections": restartSections,
	})
}

func (s *Server) handleExportConfig(c *echo.Context) error {
	// Collect providers from the store
	providerProfiles, err := s.providers.List(c.Request().Context())
	if err != nil {
		s.logger.Error("Failed to export providers", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load providers"})
	}
	providerList := make([]map[string]interface{}, len(providerProfiles))
	for i, p := range providerProfiles {
		providerList[i] = providerProfileToMap(p)
	}

	export := map[string]interface{}{
		"storage":       s.config.Storage,
		"agents":        s.config.Agents,
		"gateway":       s.config.Gateway,
		"tools":         s.config.Tools,
		"heartbeat":     s.config.Heartbeat,
		"redis":         s.config.Redis,
		"state":         s.config.State,
		"bus":           s.config.Bus,
		"approval":      s.config.Approval,
		"logger":        s.config.Logger,
		"memory":        s.config.Memory,
		"sessions":      s.config.Sessions,
		"webui":         s.config.WebUI,
		"transcription": s.config.Transcription,
		"audit":         s.config.Audit,
		"undo":          s.config.Undo,
		"preprocess":    s.config.Preprocess,
		"learnings":     s.config.Learnings,
		"watch":         s.config.Watch,
		"providers":     providerList,
	}

	c.Response().Header().Set("Content-Disposition", `attachment; filename="nekobot-config-export.json"`)
	return c.JSON(http.StatusOK, export)
}

func (s *Server) handleImportConfig(c *echo.Context) error {
	previousStorage := s.config.Storage
	oldRuntimeDBPath := ""
	if currentPath, err := config.RuntimeDBPath(s.config); err == nil {
		oldRuntimeDBPath = currentPath
	}

	var body struct {
		Storage       *config.StorageConfig       `json:"storage"`
		Agents        *config.AgentsConfig        `json:"agents"`
		Gateway       *config.GatewayConfig       `json:"gateway"`
		Tools         *config.ToolsConfig         `json:"tools"`
		Heartbeat     *config.HeartbeatConfig     `json:"heartbeat"`
		Webhook       *config.WebhookConfig       `json:"webhook"`
		Redis         *config.RedisConfig         `json:"redis"`
		State         *config.StateConfig         `json:"state"`
		Bus           *config.BusConfig           `json:"bus"`
		Approval      *config.ApprovalConfig      `json:"approval"`
		Logger        *config.LoggerConfig        `json:"logger"`
		Memory        *config.MemoryConfig        `json:"memory"`
		Sessions      *config.SessionsConfig      `json:"sessions"`
		WebUI         *config.WebUIConfig         `json:"webui"`
		Transcription *config.TranscriptionConfig `json:"transcription"`
		Audit         *config.AuditConfig         `json:"audit"`
		Undo          *config.UndoConfig          `json:"undo"`
		Preprocess    *config.PreprocessConfig    `json:"preprocess"`
		Learnings     *config.LearningsConfig     `json:"learnings"`
		Watch         *config.WatchConfig         `json:"watch"`
		Providers     []config.ProviderProfile    `json:"providers"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
	}

	// Apply config sections
	if body.Storage != nil {
		s.config.Storage = *body.Storage
	}
	if body.Agents != nil {
		s.config.Agents = *body.Agents
	}
	if body.Gateway != nil {
		s.config.Gateway = *body.Gateway
	}
	if body.Tools != nil {
		s.config.Tools = *body.Tools
	}
	if body.Heartbeat != nil {
		s.config.Heartbeat = *body.Heartbeat
	}
	if body.Webhook != nil {
		s.config.Webhook = *body.Webhook
	}
	if body.Redis != nil {
		s.config.Redis = *body.Redis
	}
	if body.State != nil {
		s.config.State = *body.State
	}
	if body.Bus != nil {
		s.config.Bus = *body.Bus
	}
	if body.Approval != nil {
		s.config.Approval = *body.Approval
	}
	if body.Logger != nil {
		s.config.Logger = *body.Logger
	}
	if body.Memory != nil {
		s.config.Memory = *body.Memory
	}
	if body.Sessions != nil {
		s.config.Sessions = *body.Sessions
	}
	if body.WebUI != nil {
		s.config.WebUI = *body.WebUI
		if s.toolSess != nil {
			s.toolSess.SetEventConfig(s.config.WebUI.ToolSessionEvents)
		}
		if s.skillsMgr != nil {
			s.skillsMgr.SetSnapshotRetention(skills.SnapshotRetentionConfig{
				AutoPrune: s.config.WebUI.SkillSnapshots.AutoPrune,
				MaxCount:  s.config.WebUI.SkillSnapshots.MaxCount,
			})
			s.skillsMgr.SetVersionRetention(skills.VersionRetentionConfig{
				Enabled:  s.config.WebUI.SkillVersions.Enabled,
				MaxCount: s.config.WebUI.SkillVersions.MaxCount,
			})
		}
	}
	if body.Transcription != nil {
		s.config.Transcription = *body.Transcription
	}
	if body.Audit != nil {
		s.config.Audit = *body.Audit
	}
	if body.Undo != nil {
		s.config.Undo = *body.Undo
	}
	if body.Preprocess != nil {
		s.config.Preprocess = *body.Preprocess
	}
	if body.Learnings != nil {
		s.config.Learnings = *body.Learnings
	}
	if body.Watch != nil {
		s.config.Watch = *body.Watch
	}

	// Validate
	if err := config.ValidateConfig(s.config); err != nil {
		s.config.Storage = previousStorage
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	restartSections := make([]string, 0, 5)
	if body.Storage != nil || body.Logger != nil || body.Gateway != nil || body.WebUI != nil || body.Webhook != nil {
		if body.Storage != nil {
			newRuntimeDBPath, err := config.RuntimeDBPath(s.config)
			if err != nil {
				s.config.Storage = previousStorage
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			if err := config.MigrateRuntimeDB(oldRuntimeDBPath, newRuntimeDBPath); err != nil {
				s.config.Storage = previousStorage
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
		}
		if err := s.saveBootstrapConfig(); err != nil {
			s.config.Storage = previousStorage
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		if body.Storage != nil {
			restartSections = append(restartSections, "storage")
		}
		if body.Logger != nil {
			restartSections = append(restartSections, "logger")
		}
		if body.Gateway != nil {
			restartSections = append(restartSections, "gateway")
		}
		if body.WebUI != nil {
			restartSections = append(restartSections, "webui")
		}
		if body.Webhook != nil {
			restartSections = append(restartSections, "webhook")
		}
	}

	// Persist runtime sections to database
	sections := make([]string, 0, 19)
	if body.Agents != nil {
		sections = append(sections, "agents")
	}
	if body.Gateway != nil {
		sections = append(sections, "gateway")
	}
	if body.Tools != nil {
		sections = append(sections, "tools")
	}
	if body.Heartbeat != nil {
		sections = append(sections, "heartbeat")
	}
	if body.Webhook != nil {
		sections = append(sections, "webhook")
	}
	if body.Redis != nil {
		sections = append(sections, "redis")
	}
	if body.State != nil {
		sections = append(sections, "state")
	}
	if body.Bus != nil {
		sections = append(sections, "bus")
	}
	if body.Approval != nil {
		sections = append(sections, "approval")
	}
	if body.Logger != nil {
		sections = append(sections, "logger")
	}
	if body.Memory != nil {
		sections = append(sections, "memory")
	}
	if body.Sessions != nil {
		sections = append(sections, "sessions")
	}
	if body.WebUI != nil {
		sections = append(sections, "webui")
	}
	if body.Transcription != nil {
		sections = append(sections, "transcription")
	}
	if body.Audit != nil {
		sections = append(sections, "audit")
	}
	if body.Undo != nil {
		sections = append(sections, "undo")
	}
	if body.Preprocess != nil {
		sections = append(sections, "preprocess")
	}
	if body.Learnings != nil {
		sections = append(sections, "learnings")
	}
	if body.Watch != nil {
		sections = append(sections, "watch")
	}
	if len(sections) > 0 {
		if err := config.SaveDatabaseSections(s.config, sections...); err != nil {
			s.logger.Error("Failed to persist imported config sections", zap.Error(err), zap.Strings("sections", sections))
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to save config sections"})
		}
	}
	if body.Watch != nil {
		if err := s.syncWatchRuntime(); err != nil {
			if s.logger != nil {
				s.logger.Error("Failed to sync watch runtime after config import", zap.Error(err))
			}
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to sync watch runtime"})
		}
	}

	// Import providers
	importedProviders := 0
	if len(body.Providers) > 0 {
		ctx := c.Request().Context()
		for _, profile := range body.Providers {
			if profile.Name == "" {
				continue
			}
			// Try update first, then create
			_, err := s.providers.Update(ctx, profile.Name, profile)
			if err != nil {
				_, createErr := s.providers.Create(ctx, profile)
				if createErr != nil {
					s.logger.Warn("Failed to import provider", zap.String("name", profile.Name), zap.Error(createErr))
					continue
				}
			}
			importedProviders++
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":             "imported",
		"sections_saved":     len(sections),
		"providers_imported": importedProviders,
		"restart_required":   len(restartSections) > 0,
		"restart_sections":   restartSections,
	})
}

func (s *Server) handleTestWebhook(c *echo.Context) error {
	if !s.config.Webhook.Enabled {
		return c.JSON(http.StatusConflict, map[string]string{"error": "webhook trigger is disabled"})
	}
	if s.agent == nil && s.webhookTestHandler == nil || s.sessionMgr == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "agent runtime not available"})
	}

	var body struct {
		Message string `json:"message"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	messageText := strings.TrimSpace(body.Message)
	if messageText == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "message is required"})
	}

	username := strings.TrimSpace(s.currentUsername(c))
	if username == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	sess, err := s.sessionMgr.GetWithSource("webhook:"+username, session.SourceWebUI)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	var reply string
	if s.webhookTestHandler != nil {
		reply, err = s.webhookTestHandler(c.Request().Context(), username, messageText)
	} else {
		reply, err = s.agent.Chat(c.Request().Context(), sess, messageText)
	}
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{
		"status":     "ok",
		"reply":      reply,
		"session_id": sess.ID,
	})
}

func (s *Server) saveBootstrapConfig() error {
	if s == nil || s.config == nil {
		return fmt.Errorf("config is unavailable")
	}

	configPath, err := s.resolveBootstrapConfigPath()
	if err != nil {
		return err
	}

	if err := config.SaveToFile(s.config, configPath); err != nil {
		return fmt.Errorf("save bootstrap config: %w", err)
	}
	return nil
}

func (s *Server) syncWatchRuntime() error {
	if s == nil || s.config == nil || s.watcher == nil {
		return nil
	}
	if err := s.watcher.ApplyConfig(s.config.Watch); err != nil {
		return fmt.Errorf("sync watch runtime: %w", err)
	}
	return nil
}

func cloneConfigSnapshot(cfg *config.Config) (*config.Config, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is unavailable")
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshal config snapshot: %w", err)
	}

	var cloned config.Config
	if err := json.Unmarshal(data, &cloned); err != nil {
		return nil, fmt.Errorf("unmarshal config snapshot: %w", err)
	}

	return &cloned, nil
}

func (s *Server) clearChatSession(sessionID string) error {
	if s == nil || s.sessionMgr == nil {
		return fmt.Errorf("session manager not available")
	}

	if err := s.sessionMgr.Delete(sessionID); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("delete session %q: %w", sessionID, err)
	}
	if s.snapshotMgr != nil {
		if err := s.snapshotMgr.RemoveStore(sessionID); err != nil {
			return fmt.Errorf("clear undo snapshots for %q: %w", sessionID, err)
		}
	}
	if _, err := s.getOrCreateChatSession(sessionID); err != nil {
		return fmt.Errorf("recreate session %q: %w", sessionID, err)
	}

	return nil
}

func (s *Server) resolveBootstrapConfigPath() (string, error) {
	configPath := ""
	if s != nil && s.loader != nil {
		configPath = strings.TrimSpace(s.loader.GetConfigPath())
	}
	if configPath == "" {
		configPath = strings.TrimSpace(os.Getenv(config.ConfigPathEnv))
	}
	if configPath == "" {
		home, err := config.GetConfigHome()
		if err != nil {
			return "", fmt.Errorf("resolve config home: %w", err)
		}
		configPath = filepath.Join(home, "config.json")
	}
	return configPath, nil
}

// --- Session Handlers ---

type sessionSummaryResponse struct {
	ID           string    `json:"id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Summary      string    `json:"summary"`
	MessageCount int       `json:"message_count"`
	RuntimeID    string    `json:"runtime_id"`
	Topic        string    `json:"topic"`
}

type sessionMessageResponse struct {
	Role       string `json:"role"`
	Content    string `json:"content"`
	ToolCallID string `json:"tool_call_id"`
}

type sessionDetailResponse struct {
	ID           string                   `json:"id"`
	CreatedAt    time.Time                `json:"created_at"`
	UpdatedAt    time.Time                `json:"updated_at"`
	Summary      string                   `json:"summary"`
	MessageCount int                      `json:"message_count"`
	RuntimeID    string                   `json:"runtime_id"`
	Topic        string                   `json:"topic"`
	Messages     []sessionMessageResponse `json:"messages"`
}

type threadSummaryResponse struct {
	ID           string    `json:"id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Summary      string    `json:"summary"`
	MessageCount int       `json:"message_count"`
	RuntimeID    string    `json:"runtime_id"`
	Topic        string    `json:"topic"`
}

type threadDetailResponse struct {
	ID           string                   `json:"id"`
	CreatedAt    time.Time                `json:"created_at"`
	UpdatedAt    time.Time                `json:"updated_at"`
	Summary      string                   `json:"summary"`
	MessageCount int                      `json:"message_count"`
	RuntimeID    string                   `json:"runtime_id"`
	Topic        string                   `json:"topic"`
	Messages     []sessionMessageResponse `json:"messages"`
}

func buildSessionMessageResponses(messages []session.Message) []sessionMessageResponse {
	respMessages := make([]sessionMessageResponse, len(messages))
	for i, msg := range messages {
		respMessages[i] = sessionMessageResponse{
			Role:       msg.Role,
			Content:    msg.Content,
			ToolCallID: msg.ToolCallID,
		}
	}
	return respMessages
}

func (s *Server) handleListSessions(c *echo.Context) error {
	if s.sessionMgr == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "session manager not available"})
	}

	ids, err := s.sessionMgr.List()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to list sessions: %v", err)})
	}

	summaries := make([]sessionSummaryResponse, 0, len(ids))
	for _, id := range ids {
		sess, err := s.sessionMgr.GetExisting(id)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to load session %q: %v", id, err)})
		}
		messages := sess.GetMessages()
		summaries = append(summaries, sessionSummaryResponse{
			ID:           sess.GetID(),
			CreatedAt:    sess.GetCreatedAt(),
			UpdatedAt:    sess.GetUpdatedAt(),
			Summary:      sess.GetSummary(),
			MessageCount: len(messages),
			RuntimeID:    s.getThreadRuntimeBinding(id),
			Topic:        s.getThreadTopic(id),
		})
	}

	sort.SliceStable(summaries, func(i, j int) bool {
		return summaries[i].UpdatedAt.After(summaries[j].UpdatedAt)
	})

	return c.JSON(http.StatusOK, summaries)
}

func (s *Server) handleGetSession(c *echo.Context) error {
	if s.sessionMgr == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "session manager not available"})
	}

	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "session id is required"})
	}

	sess, err := s.sessionMgr.GetExisting(id)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "session not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to load session: %v", err)})
	}

	messages := sess.GetMessages()

	resp := sessionDetailResponse{
		ID:           sess.GetID(),
		CreatedAt:    sess.GetCreatedAt(),
		UpdatedAt:    sess.GetUpdatedAt(),
		Summary:      sess.GetSummary(),
		MessageCount: len(messages),
		RuntimeID:    s.getThreadRuntimeBinding(id),
		Topic:        s.getThreadTopic(id),
		Messages:     buildSessionMessageResponses(messages),
	}
	return c.JSON(http.StatusOK, resp)
}

func (s *Server) handleUpdateSessionSummary(c *echo.Context) error {
	if s.sessionMgr == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "session manager not available"})
	}

	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "session id is required"})
	}

	var body struct {
		Summary string `json:"summary"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	sess, err := s.sessionMgr.GetExisting(id)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "session not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to load session: %v", err)})
	}

	sess.SetSummary(body.Summary)
	if err := s.sessionMgr.Save(sess); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to save session summary: %v", err)})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "updated"})
}

func (s *Server) handleUpdateSessionRuntime(c *echo.Context) error {
	if s.sessionMgr == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "session manager not available"})
	}
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "session id is required"})
	}
	if _, err := s.sessionMgr.GetExisting(id); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "session not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to load session: %v", err)})
	}
	var body struct {
		RuntimeID string `json:"runtime_id"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if err := s.setThreadRuntimeBinding(id, body.RuntimeID); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "updated"})
}

func (s *Server) handleUpdateSessionThread(c *echo.Context) error {
	if s.sessionMgr == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "session manager not available"})
	}
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "session id is required"})
	}
	if _, err := s.sessionMgr.GetExisting(id); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "session not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to load session: %v", err)})
	}
	var body struct {
		Topic string `json:"topic"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if err := s.setThreadTopic(id, body.Topic); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "updated"})
}

func (s *Server) handleDeleteSession(c *echo.Context) error {
	if s.sessionMgr == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "session manager not available"})
	}

	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "session id is required"})
	}

	if _, err := s.sessionMgr.GetExisting(id); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "session not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to load session: %v", err)})
	}
	if err := s.sessionMgr.Delete(id); err != nil && !errors.Is(err, os.ErrNotExist) {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to delete session: %v", err)})
	}
	_ = s.deleteThread(id)
	if s.prompts != nil {
		if err := s.prompts.ClearSessionBindings(c.Request().Context(), id); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to delete session prompts: %v", err)})
		}
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleCleanupSessions(c *echo.Context) error {
	if s.sessionMgr == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "session manager not available"})
	}
	if err := session.CleanupPersistedSessions(s.config, s.sessionMgr); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	if s.prompts != nil {
		listed, err := s.sessionMgr.List()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("list sessions after cleanup: %v", err)})
		}
		active := make(map[string]struct{}, len(listed))
		for _, item := range listed {
			active[item] = struct{}{}
		}
		bindings, err := s.prompts.ListBindings(c.Request().Context(), prompts.ScopeSession, "")
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("list session prompt bindings: %v", err)})
		}
		for _, binding := range bindings {
			if _, ok := active[binding.Target]; ok {
				continue
			}
			if err := s.prompts.ClearSessionBindings(c.Request().Context(), binding.Target); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("cleanup session prompts: %v", err)})
			}
		}
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "cleaned"})
}

func (s *Server) handleListThreads(c *echo.Context) error {
	if s.sessionMgr == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "session manager not available"})
	}

	ids, err := s.sessionMgr.List()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to list threads: %v", err)})
	}

	items := make([]threadSummaryResponse, 0, len(ids))
	for _, id := range ids {
		sess, err := s.sessionMgr.GetExisting(id)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to load thread %q: %v", id, err)})
		}
		messages := sess.GetMessages()
		items = append(items, threadSummaryResponse{
			ID:           sess.GetID(),
			CreatedAt:    sess.GetCreatedAt(),
			UpdatedAt:    sess.GetUpdatedAt(),
			Summary:      sess.GetSummary(),
			MessageCount: len(messages),
			RuntimeID:    s.getThreadRuntimeBinding(id),
			Topic:        s.getThreadTopic(id),
		})
	}

	sort.SliceStable(items, func(i, j int) bool {
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})

	return c.JSON(http.StatusOK, items)
}

func (s *Server) handleGetThread(c *echo.Context) error {
	if s.sessionMgr == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "session manager not available"})
	}

	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "thread id is required"})
	}

	sess, err := s.sessionMgr.GetExisting(id)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "thread not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to load thread: %v", err)})
	}

	messages := sess.GetMessages()
	return c.JSON(http.StatusOK, threadDetailResponse{
		ID:           sess.GetID(),
		CreatedAt:    sess.GetCreatedAt(),
		UpdatedAt:    sess.GetUpdatedAt(),
		Summary:      sess.GetSummary(),
		MessageCount: len(messages),
		RuntimeID:    s.getThreadRuntimeBinding(id),
		Topic:        s.getThreadTopic(id),
		Messages:     buildSessionMessageResponses(messages),
	})
}

func (s *Server) handleUpdateThread(c *echo.Context) error {
	if s.sessionMgr == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "session manager not available"})
	}
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "thread id is required"})
	}
	sess, err := s.sessionMgr.GetExisting(id)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "thread not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to load thread: %v", err)})
	}
	var body struct {
		Summary   string `json:"summary"`
		RuntimeID string `json:"runtime_id"`
		Topic     string `json:"topic"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if strings.TrimSpace(body.Summary) != "" || body.Summary == "" {
		sess.SetSummary(body.Summary)
		if err := s.sessionMgr.Save(sess); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to save thread summary: %v", err)})
		}
	}
	if err := s.setThreadRuntimeBinding(id, body.RuntimeID); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	if err := s.setThreadTopic(id, body.Topic); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "updated"})
}

// --- Status Handler ---

func (s *Server) handleStatus(c *echo.Context) error {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	uptime := time.Since(s.startedAt)
	configPath, err := s.resolveBootstrapConfigPath()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	runtimeDBPath, err := config.RuntimeDBPath(s.config)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	taskSnapshots := s.listTaskSnapshots()
	recentTasks, stateCounts := summarizeTasks(taskSnapshots, 5)
	recentCronJobs := s.listRecentCronJobs(5)
	runtimeStates, err := s.deriveRuntimeStatuses(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	sessionStates := s.listSessionStates()

	return c.JSON(http.StatusOK, map[string]interface{}{
		"version":            version.GetVersion(),
		"commit":             version.GitCommit,
		"build_time":         version.BuildTime,
		"os":                 runtime.GOOS,
		"arch":               runtime.GOARCH,
		"go_version":         runtime.Version(),
		"pid":                os.Getpid(),
		"uptime":             uptime.Round(time.Second).String(),
		"uptime_seconds":     int64(uptime.Seconds()),
		"memory_alloc_bytes": mem.Alloc,
		"memory_sys_bytes":   mem.Sys,
		"provider_count":     len(s.config.Providers),
		"config_path":        configPath,
		"database_dir":       s.config.Storage.DBDir,
		"runtime_db_path":    runtimeDBPath,
		"workspace_path":     s.config.Agents.Defaults.Workspace,
		"task_count":         len(taskSnapshots),
		"task_state_counts":  stateCounts,
		"recent_tasks":       recentTasks,
		"recent_cron_jobs":   recentCronJobs,
		"runtime_states":     runtimeStates,
		"daemon_machines": func() interface{} {
			if s.kvStore == nil {
				return []daemonhost.MachineStatus{}
			}
			snapshot, err := daemonhost.NewRegistry(s.kvStore).Snapshot(c.Request().Context())
			if err != nil || snapshot == nil {
				return []daemonhost.MachineStatus{}
			}
			return daemonhost.MachineStatuses(snapshot)
		}(),
		"session_runtime_states": sessionStates,
		"agent_definition": func() interface{} {
			if s.agent == nil {
				return nil
			}
			return s.agent.Definition()
		}(),
		"gateway_host": s.config.Gateway.Host,
		"gateway_port": s.config.Gateway.Port,
		"gateway": map[string]interface{}{
			"host": s.config.Gateway.Host,
			"port": s.config.Gateway.Port,
		},
	})
}

func (s *Server) listTaskSnapshots() []tasks.Task {
	if s == nil || s.taskStore == nil {
		return nil
	}
	return s.taskStore.List()
}

func (s *Server) listRecentCronJobs(limit int) []cron.Job {
	if s == nil || s.cronMgr == nil {
		return []cron.Job{}
	}

	jobs := s.cronMgr.ListJobs()
	if len(jobs) == 0 {
		return []cron.Job{}
	}

	filtered := make([]cron.Job, 0, len(jobs))
	for _, job := range jobs {
		if job == nil || job.LastRun.IsZero() {
			continue
		}
		filtered = append(filtered, *job)
	}
	if len(filtered) == 0 {
		return []cron.Job{}
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		return filtered[i].LastRun.After(filtered[j].LastRun)
	})
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
	}
	return filtered
}

func (s *Server) listSessionStates() []tasks.SessionState {
	if s == nil || s.taskStore == nil {
		return []tasks.SessionState{}
	}
	return s.taskStore.ListSessionStates()
}

func (s *Server) deriveRuntimeStatuses(ctx context.Context) ([]runtimeagents.AgentRuntime, error) {
	if s == nil || s.runtimeMgr == nil {
		return []runtimeagents.AgentRuntime{}, nil
	}

	items, err := s.runtimeMgr.List(ctx)
	if err != nil {
		return nil, err
	}
	return s.attachRuntimeStatuses(ctx, items)
}

func (s *Server) attachRuntimeStatuses(ctx context.Context, items []runtimeagents.AgentRuntime) ([]runtimeagents.AgentRuntime, error) {
	if len(items) == 0 {
		return []runtimeagents.AgentRuntime{}, nil
	}

	bindingsByRuntimeID := make(map[string]int, len(items))
	enabledBindingsByRuntimeID := make(map[string]int, len(items))
	accountEnabledByID := map[string]bool{}
	if s.accountMgr != nil {
		accounts, err := s.accountMgr.List(ctx)
		if err != nil {
			return nil, fmt.Errorf("list channel accounts for runtime status: %w", err)
		}
		for _, account := range accounts {
			accountEnabledByID[account.ID] = account.Enabled
		}
	}
	if s.bindingMgr != nil {
		bindings, err := s.bindingMgr.List(ctx)
		if err != nil {
			return nil, fmt.Errorf("list account bindings for runtime status: %w", err)
		}
		for _, binding := range bindings {
			bindingsByRuntimeID[binding.AgentRuntimeID]++
			if binding.Enabled && accountEnabledByID[binding.ChannelAccountID] {
				enabledBindingsByRuntimeID[binding.AgentRuntimeID]++
			}
		}
	}

	tasksByRuntimeID := make(map[string]int, len(items))
	lastSeenByRuntimeID := make(map[string]time.Time, len(items))
	for _, task := range s.listTaskSnapshots() {
		runtimeID := strings.TrimSpace(task.RuntimeID)
		if runtimeID == "" {
			continue
		}
		if !tasks.IsFinal(task.State) {
			tasksByRuntimeID[runtimeID]++
		}
		taskTime := taskSortTime(task)
		if taskTime.After(lastSeenByRuntimeID[runtimeID]) {
			lastSeenByRuntimeID[runtimeID] = taskTime
		}
	}

	result := make([]runtimeagents.AgentRuntime, 0, len(items))
	for _, item := range items {
		status := &runtimeagents.RuntimeDerivedStatus{
			EffectiveAvailable:  item.Enabled && enabledBindingsByRuntimeID[item.ID] > 0,
			BoundAccountCount:   bindingsByRuntimeID[item.ID],
			EnabledBindingCount: enabledBindingsByRuntimeID[item.ID],
			CurrentTaskCount:    tasksByRuntimeID[item.ID],
			LastSeenAt:          runtimeagents.NormalizeTimestamp(lastSeenByRuntimeID[item.ID]),
		}
		switch {
		case !item.Enabled:
			status.AvailabilityReason = "runtime_disabled"
		case enabledBindingsByRuntimeID[item.ID] == 0:
			if bindingsByRuntimeID[item.ID] == 0 {
				status.AvailabilityReason = "unbound"
			} else {
				status.AvailabilityReason = "no_enabled_bindings"
			}
		default:
			status.AvailabilityReason = "available"
		}
		item.Status = status
		result = append(result, item)
	}
	return result, nil
}

func summarizeTasks(all []tasks.Task, limit int) ([]tasks.Task, map[string]int) {
	stateCounts := make(map[string]int)
	if len(all) == 0 {
		return []tasks.Task{}, stateCounts
	}

	snapshots := append([]tasks.Task(nil), all...)
	for _, task := range snapshots {
		stateCounts[string(task.State)]++
	}
	sort.SliceStable(snapshots, func(i, j int) bool {
		return taskSortTime(snapshots[i]).After(taskSortTime(snapshots[j]))
	})
	if limit > 0 && len(snapshots) > limit {
		snapshots = snapshots[:limit]
	}
	return snapshots, stateCounts
}

func taskSortTime(task tasks.Task) time.Time {
	switch {
	case !task.CompletedAt.IsZero():
		return task.CompletedAt
	case !task.StartedAt.IsZero():
		return task.StartedAt
	default:
		return task.CreatedAt
	}
}

func (s *Server) handleServiceStatus(c *echo.Context) error {
	if s.serviceCtrl == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "service control not available"})
	}

	status, err := s.serviceCtrl.Status()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, status)
}

func (s *Server) handleServiceRestart(c *echo.Context) error {
	if s.serviceCtrl == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "service control not available"})
	}

	if err := s.serviceCtrl.Restart(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "restarting"})
}

func (s *Server) handleServiceReload(c *echo.Context) error {
	if s.serviceCtrl == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "service control not available"})
	}

	if err := s.serviceCtrl.Reload(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "reloaded"})
}

func (s *Server) handleGetWatchStatus(c *echo.Context) error {
	if s.config == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "config unavailable"})
	}

	if s.watcher == nil {
		return c.JSON(http.StatusOK, watch.StatusSnapshot{
			Enabled:    s.config.Watch.Enabled,
			Running:    false,
			DebounceMs: s.config.Watch.DebounceMs,
			Patterns:   append([]config.WatchPattern(nil), s.config.Watch.Patterns...),
		})
	}

	return c.JSON(http.StatusOK, s.watcher.Status())
}

func (s *Server) handleUpdateWatchStatus(c *echo.Context) error {
	if s.config == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "config unavailable"})
	}

	var body struct {
		Enabled    *bool                 `json:"enabled"`
		DebounceMs *int                  `json:"debounce_ms"`
		Patterns   []config.WatchPattern `json:"patterns"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	if body.Enabled != nil {
		s.config.Watch.Enabled = *body.Enabled
	}
	if body.DebounceMs != nil {
		s.config.Watch.DebounceMs = *body.DebounceMs
	}
	if body.Patterns != nil {
		s.config.Watch.Patterns = append([]config.WatchPattern(nil), body.Patterns...)
	}

	if err := config.SaveDatabaseSections(s.config, "watch"); err != nil {
		if s.logger != nil {
			s.logger.Error("Failed to persist watch config", zap.Error(err))
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to save watch config"})
	}
	if err := s.syncWatchRuntime(); err != nil {
		if s.logger != nil {
			s.logger.Error("Failed to sync watch runtime", zap.Error(err))
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to sync watch runtime"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":           "saved",
		"restart_required": true,
		"watch": func() watch.StatusSnapshot {
			if s.watcher != nil {
				return s.watcher.Status()
			}
			return watch.StatusSnapshot{
				Enabled:    s.config.Watch.Enabled,
				Running:    false,
				DebounceMs: s.config.Watch.DebounceMs,
				Patterns:   append([]config.WatchPattern(nil), s.config.Watch.Patterns...),
			}
		}(),
	})
}

func (s *Server) handleGetHarnessAudit(c *echo.Context) error {
	if s.auditLogger == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "audit log unavailable"})
	}

	limit := 100
	if rawLimit := strings.TrimSpace(c.QueryParam("limit")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid limit"})
		}
		if parsed > 500 {
			parsed = 500
		}
		limit = parsed
	}

	entries, err := s.auditLogger.ReadLast(limit)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("Failed to read audit log", zap.Error(err))
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to read audit log"})
	}

	stats, err := s.auditLogger.Stats()
	if err != nil {
		if s.logger != nil {
			s.logger.Error("Failed to read audit stats", zap.Error(err))
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to read audit stats"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"entries": entries,
		"stats":   stats,
		"limit":   limit,
	})
}

func (s *Server) handleClearHarnessAudit(c *echo.Context) error {
	if s.auditLogger == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "audit log unavailable"})
	}

	if err := s.auditLogger.Clear(); err != nil && !os.IsNotExist(err) {
		if s.logger != nil {
			s.logger.Error("Failed to clear audit log", zap.Error(err))
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to clear audit log"})
	}

	stats, err := s.auditLogger.Stats()
	if err != nil {
		if s.logger != nil {
			s.logger.Error("Failed to refresh audit stats after clear", zap.Error(err))
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to read audit stats"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status": "cleared",
		"stats":  stats,
	})
}

// --- Chat WebSocket Playground ---

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type chatWSMessage struct {
	Type            string   `json:"type"`                        // "message", "ping", "clear"
	Content         string   `json:"content"`                     // User message text
	Model           string   `json:"model"`                       // Optional model override
	Provider        string   `json:"provider,omitempty"`          // Optional provider override
	Fallback        []string `json:"fallback,omitempty"`          // Optional fallback provider order
	SystemPromptIDs []string `json:"system_prompt_ids,omitempty"` // Optional session prompt overlays
	UserPromptIDs   []string `json:"user_prompt_ids,omitempty"`   // Optional session prompt overlays
	RuntimeID       string   `json:"runtime_id,omitempty"`        // Optional explicit runtime selection
}

type chatWSResponse struct {
	Type      string          `json:"type"`                 // "message", "thinking", "error", "system", "pong", "route_result"
	Content   string          `json:"content"`              // Response text
	Thinking  string          `json:"thinking,omitempty"`   // Model's thinking (if extended thinking enabled)
	Timestamp int64           `json:"timestamp,omitempty"`  // Unix timestamp
	SessionID string          `json:"session_id,omitempty"` // Routed chat session
	Route     *chatRouteState `json:"route,omitempty"`
	Meta      interface{}     `json:"meta,omitempty"`
}

type fileMentionFeedback struct {
	Count    int      `json:"count"`
	Paths    []string `json:"paths,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

type chatRouteSettings struct {
	Provider string   `json:"provider"`
	Model    string   `json:"model"`
	Fallback []string `json:"fallback"`
}

type chatRouteState struct {
	RequestedProvider     string                   `json:"requested_provider"`
	RequestedModel        string                   `json:"requested_model"`
	RequestedFallback     []string                 `json:"requested_fallback"`
	ResolvedOrder         []string                 `json:"resolved_order"`
	ActualProvider        string                   `json:"actual_provider"`
	ActualModel           string                   `json:"actual_model"`
	Preflight             *chatRoutePreflightState `json:"preflight,omitempty"`
	ContextBudgetStatus   string                   `json:"context_budget_status,omitempty"`
	ContextBudgetReasons  []string                 `json:"context_budget_reasons,omitempty"`
	CompactionRecommended bool                     `json:"compaction_recommended,omitempty"`
	CompactionStrategy    string                   `json:"compaction_strategy,omitempty"`
	RuntimeID             string                   `json:"runtime_id,omitempty"`
}

type chatRoutePreflightState struct {
	Action        string                   `json:"action,omitempty"`
	Applied       bool                     `json:"applied"`
	BudgetStatus  string                   `json:"budget_status,omitempty"`
	BudgetReasons []string                 `json:"budget_reasons,omitempty"`
	Compaction    chatRouteCompactionState `json:"compaction"`
}

type chatRouteCompactionState struct {
	Recommended         bool     `json:"recommended"`
	Strategy            string   `json:"strategy,omitempty"`
	Reasons             []string `json:"reasons,omitempty"`
	EstimatedCharsSaved int      `json:"estimated_chars_saved,omitempty"`
}

func buildChatRouteWSResponse(sessionID, runtimeID string, routeResult agent.ChatRouteResult) chatWSResponse {
	return chatWSResponse{
		Type:      "route_result",
		Timestamp: time.Now().Unix(),
		SessionID: strings.TrimSpace(sessionID),
		Route: &chatRouteState{
			RequestedProvider: routeResult.RequestedProvider,
			RequestedModel:    routeResult.RequestedModel,
			RequestedFallback: append([]string(nil), routeResult.RequestedFallback...),
			ResolvedOrder:     append([]string(nil), routeResult.ResolvedOrder...),
			ActualProvider:    routeResult.ActualProvider,
			ActualModel:       routeResult.ActualModel,
			Preflight: &chatRoutePreflightState{
				Action:        routeResult.Preflight.Action,
				Applied:       routeResult.Preflight.Applied,
				BudgetStatus:  routeResult.Preflight.BudgetStatus,
				BudgetReasons: append([]string(nil), routeResult.Preflight.BudgetReasons...),
				Compaction: chatRouteCompactionState{
					Recommended:         routeResult.Preflight.Compaction.Recommended,
					Strategy:            routeResult.Preflight.Compaction.Strategy,
					Reasons:             append([]string(nil), routeResult.Preflight.Compaction.Reasons...),
					EstimatedCharsSaved: routeResult.Preflight.Compaction.EstimatedCharsSaved,
				},
			},
			ContextBudgetStatus:   routeResult.ContextBudgetStatus,
			ContextBudgetReasons:  append([]string(nil), routeResult.ContextBudgetReasons...),
			CompactionRecommended: routeResult.CompactionRecommended,
			CompactionStrategy:    routeResult.CompactionStrategy,
			RuntimeID:             runtimeID,
		},
	}
}

type toolWSMessage struct {
	Type string `json:"type"` // "input", "ping", "kill", "resize"
	Data string `json:"data,omitempty"`
	Cols int    `json:"cols,omitempty"`
	Rows int    `json:"rows,omitempty"`
}

type toolWSResponse struct {
	Type      string `json:"type"`                 // "ready", "output", "status", "error", "pong"
	SessionID string `json:"session_id,omitempty"` // for ready
	Data      string `json:"data,omitempty"`       // terminal output
	Total     int    `json:"total,omitempty"`      // output chunk cursor
	Running   bool   `json:"running,omitempty"`
	ExitCode  int    `json:"exit_code,omitempty"`
	Missing   bool   `json:"missing,omitempty"`
	Message   string `json:"message,omitempty"`
}

func webUIChatSessionID(username string) string {
	username = strings.TrimSpace(username)
	if username == "" {
		return "webui-chat"
	}
	return "webui-chat:" + username
}

func webUIRuntimeChatSessionID(username, runtimeID string) string {
	username = strings.TrimSpace(username)
	runtimeID = strings.TrimSpace(runtimeID)
	baseSessionID := webUIChatSessionID(username)
	if runtimeID == "" {
		return baseSessionID
	}
	return inboundrouter.SessionPrefix + ":" + runtimeID + ":" + baseSessionID
}

func webUIClientChatSessionID(runtimeID string) string {
	runtimeID = strings.TrimSpace(runtimeID)
	if runtimeID == "" {
		return "webui-chat"
	}
	return inboundrouter.SessionPrefix + ":" + runtimeID + ":webui-chat"
}

func buildWebUIChatPromptContext(
	sessionID string,
	username string,
	provider string,
	model string,
	fallback []string,
	explicitPromptIDs []string,
	runtimeID string,
) agent.PromptContext {
	promptCtx := agent.PromptContext{
		Channel:           session.SourceWebUI,
		SessionID:         strings.TrimSpace(sessionID),
		UserID:            strings.TrimSpace(username),
		Username:          strings.TrimSpace(username),
		RequestedProvider: strings.TrimSpace(provider),
		RequestedModel:    strings.TrimSpace(model),
		RequestedFallback: append([]string(nil), fallback...),
		ExplicitPromptIDs: append([]string(nil), explicitPromptIDs...),
	}
	if strings.TrimSpace(runtimeID) != "" {
		promptCtx.Custom = map[string]any{
			"runtime_id": strings.TrimSpace(runtimeID),
		}
	}
	return promptCtx
}

func (s *Server) resolveWebUIRuntimeSelection(
	ctx context.Context,
	runtimeID string,
	provider string,
	model string,
	fallback []string,
) (string, string, []string, []string, error) {
	runtimeID = strings.TrimSpace(runtimeID)
	if runtimeID == "" {
		return strings.TrimSpace(provider), strings.TrimSpace(model), append([]string(nil), fallback...), nil, nil
	}
	if s == nil || s.runtimeMgr == nil {
		return "", "", nil, nil, fmt.Errorf("runtime manager not available")
	}
	if s.accountMgr == nil || s.bindingMgr == nil {
		return "", "", nil, nil, fmt.Errorf("chat runtime topology is not available")
	}

	runtimeItem, err := s.runtimeMgr.Get(ctx, runtimeID)
	if err != nil {
		return "", "", nil, nil, fmt.Errorf("get runtime %s: %w", runtimeID, err)
	}
	if runtimeItem == nil || !runtimeItem.Enabled {
		return "", "", nil, nil, fmt.Errorf("runtime %s is not available", runtimeID)
	}
	websocketAccount, err := s.accountMgr.FindByChannelTypeAndAccountKey(ctx, "websocket", "default")
	if err != nil {
		return "", "", nil, nil, fmt.Errorf("runtime %s is not available for websocket chat", runtimeID)
	}
	if websocketAccount == nil || !websocketAccount.Enabled {
		return "", "", nil, nil, fmt.Errorf("runtime %s is not available for websocket chat", runtimeID)
	}
	bindings, err := s.bindingMgr.ListEnabledByChannelAccountID(ctx, websocketAccount.ID)
	if err != nil {
		return "", "", nil, nil, fmt.Errorf("list websocket bindings: %w", err)
	}
	bound := false
	for _, binding := range bindings {
		if strings.TrimSpace(binding.AgentRuntimeID) == runtimeID {
			bound = true
			break
		}
	}
	if !bound {
		return "", "", nil, nil, fmt.Errorf("runtime %s is not available for websocket chat", runtimeID)
	}

	return strings.TrimSpace(runtimeItem.Provider),
		strings.TrimSpace(runtimeItem.Model),
		nil,
		runtimePromptIDs(runtimeItem.PromptID),
		nil
}

func (s *Server) handleChatWS(c *echo.Context) error {
	// Authenticate via token query param (since WebSocket can't use Authorization header easily)
	tokenStr := c.QueryParam("token")
	if tokenStr == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "token required"})
	}
	username, err := s.parseJWTSubject(tokenStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid token"})
	}

	// Upgrade to WebSocket
	conn, err := wsUpgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		s.logger.Error("WebUI chat WS upgrade failed", zap.Error(err))
		return nil
	}
	defer func() {
		_ = conn.Close()
	}()

	baseSessionID := webUIChatSessionID(username)
	baseClientSessionID := webUIClientChatSessionID("")
	sess, err := s.getOrCreateChatSession(baseSessionID)
	if err != nil {
		sendWSError(conn, fmt.Sprintf("session error: %v", err), baseClientSessionID)
		return nil
	}

	// Send welcome
	welcome := chatWSResponse{
		Type:      "system",
		Content:   "Connected to chat playground",
		Timestamp: time.Now().Unix(),
		SessionID: baseClientSessionID,
	}
	if data, err := json.Marshal(welcome); err == nil {
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			s.logger.Warn("Failed to send chat welcome", zap.Error(err))
		}
	}
	routing := chatRouteSettings{
		Provider: strings.TrimSpace(s.config.Agents.Defaults.Provider),
		Model:    strings.TrimSpace(s.config.Agents.Defaults.Model),
		Fallback: append([]string(nil), s.config.Agents.Defaults.Fallback...),
	}
	if data, err := json.Marshal(chatWSResponse{
		Type:      "routing",
		Content:   mustMarshalChatRouting(routing),
		Timestamp: time.Now().Unix(),
		SessionID: baseClientSessionID,
	}); err == nil {
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			s.logger.Warn("Failed to send chat routing", zap.Error(err))
		}
	}

	// Read loop
	conn.SetReadLimit(65536)
	if err := conn.SetReadDeadline(time.Now().Add(120 * time.Second)); err != nil {
		s.logger.Warn("Failed to set chat read deadline", zap.Error(err))
	}
	conn.SetPongHandler(func(string) error {
		if err := conn.SetReadDeadline(time.Now().Add(120 * time.Second)); err != nil {
			s.logger.Warn("Failed to refresh chat read deadline", zap.Error(err))
		}
		return nil
	})

	// Ping ticker
	pingDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
					s.logger.Warn("Failed to set chat ping deadline", zap.Error(err))
				}
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			case <-pingDone:
				return
			}
		}
	}()
	defer close(pingDone)

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				s.logger.Warn("WebUI chat WS read error", zap.Error(err))
			}
			return nil
		}

		var msg chatWSMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			sendWSError(conn, "invalid message format", baseClientSessionID)
			continue
		}

		switch msg.Type {
		case "ping":
			resp := chatWSResponse{
				Type:      "pong",
				Timestamp: time.Now().Unix(),
				SessionID: baseClientSessionID,
			}
			if data, err := json.Marshal(resp); err == nil {
				if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
					s.logger.Warn("Failed to send chat pong", zap.Error(err))
				}
			}

		case "clear":
			clearSessionID := webUIRuntimeChatSessionID(username, msg.RuntimeID)
			clearClientSessionID := webUIClientChatSessionID(msg.RuntimeID)
			if err := s.clearChatSession(clearSessionID); err != nil {
				sendWSError(conn, fmt.Sprintf("session reset failed: %v", err), clearClientSessionID)
				continue
			}
			sess, err = s.sessionMgr.GetExisting(clearSessionID)
			if err != nil {
				sendWSError(conn, fmt.Sprintf("session error: %v", err), clearClientSessionID)
				continue
			}
			resp := chatWSResponse{
				Type:      "system",
				Content:   "Session cleared",
				Timestamp: time.Now().Unix(),
				SessionID: clearClientSessionID,
			}
			if data, err := json.Marshal(resp); err == nil {
				if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
					s.logger.Warn("Failed to send chat cleared event", zap.Error(err))
				}
			}

		case "message":
			content := strings.TrimSpace(msg.Content)
			if content == "" {
				continue
			}
			runtimeID := strings.TrimSpace(msg.RuntimeID)
			if runtimeID == "" {
				runtimeID = s.getThreadRuntimeBinding(webUIChatSessionID(username))
			}
			sessionID := webUIRuntimeChatSessionID(username, runtimeID)
			clientSessionID := webUIClientChatSessionID(runtimeID)
			requestedModel := strings.TrimSpace(msg.Model)
			requestedProvider := strings.TrimSpace(msg.Provider)
			requestedFallback := normalizeProviderNames(msg.Fallback)

			if runtimeID == "" {
				// Keep provider/fallback choices in sync with the saved config so restarts preserve them.
				if err := s.persistChatRouting(requestedProvider, requestedModel, requestedFallback); err != nil {
					sendWSError(conn, fmt.Sprintf("persist chat routing failed: %v", err), clientSessionID)
					continue
				}
			} else {
				_ = s.setThreadRuntimeBinding(webUIChatSessionID(username), runtimeID)
			}

			provider, model, fallback, explicitPromptIDs, err := s.resolveWebUIRuntimeSelection(
				context.Background(),
				runtimeID,
				requestedProvider,
				requestedModel,
				requestedFallback,
			)
			if err != nil {
				sendWSError(conn, fmt.Sprintf("runtime selection failed: %v", err), clientSessionID)
				continue
			}
			if s.prompts != nil {
				if _, err := s.prompts.ReplaceSessionBindings(
					context.Background(),
					sessionID,
					msg.SystemPromptIDs,
					msg.UserPromptIDs,
				); err != nil {
					sendWSError(conn, fmt.Sprintf("save session prompts failed: %v", err), clientSessionID)
					continue
				}
			}
			sess, err = s.getOrCreateChatSession(sessionID)
			if err != nil {
				sendWSError(conn, fmt.Sprintf("session error: %v", err), clientSessionID)
				continue
			}

			if s.agent != nil {
				preview, err := s.agent.PreviewPreprocessedInput(content)
				if err == nil && preview != nil && (len(preview.Mentions) > 0 || len(preview.Warnings) > 0) {
					paths := make([]string, 0, len(preview.Mentions))
					for _, mention := range preview.Mentions {
						path := strings.TrimSpace(mention.Path)
						if path == "" {
							continue
						}
						if mention.StartLine > 0 && mention.EndLine >= mention.StartLine {
							path = fmt.Sprintf("%s:%d-%d", path, mention.StartLine, mention.EndLine)
						}
						paths = append(paths, path)
					}
					feedback := fileMentionFeedback{
						Count:    len(preview.Mentions),
						Paths:    paths,
						Warnings: append([]string(nil), preview.Warnings...),
					}
					systemText := fmt.Sprintf("Inlined %d file reference(s)", feedback.Count)
					if len(feedback.Warnings) > 0 {
						systemText = fmt.Sprintf("%s (%d warning(s))", systemText, len(feedback.Warnings))
					}
					if data, marshalErr := json.Marshal(chatWSResponse{
						Type:      "system",
						Content:   systemText,
						Timestamp: time.Now().Unix(),
						SessionID: clientSessionID,
						Meta: map[string]interface{}{
							"kind": "file_mentions",
							"data": feedback,
						},
					}); marshalErr == nil {
						if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
							s.logger.Warn("Failed to send file mention feedback", zap.Error(err))
						}
					}
				}
			}

			// Add user message to session
			sess.AddMessage(agent.Message{
				Role:    "user",
				Content: content,
			})

			if daemonHandled, daemonReply, daemonErr := s.handleDaemonRuntimeChatMessage(
				context.Background(),
				username,
				runtimeID,
				sessionID,
				content,
			); daemonHandled {
				if daemonErr != nil {
					sendWSError(conn, fmt.Sprintf("daemon task error: %v", daemonErr), clientSessionID)
					continue
				}
				sess.AddMessage(agent.Message{
					Role:    "assistant",
					Content: daemonReply,
				})
				resp := chatWSResponse{
					Type:      "message",
					Content:   daemonReply,
					Timestamp: time.Now().Unix(),
					SessionID: clientSessionID,
				}
				if data, err := json.Marshal(resp); err == nil {
					if err := conn.SetWriteDeadline(time.Now().Add(30 * time.Second)); err != nil {
						s.logger.Warn("Failed to set daemon chat response deadline", zap.Error(err))
					}
					if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
						s.logger.Warn("Failed to send daemon chat response", zap.Error(err))
					}
				}
				continue
			}

			// Process with agent.
			response, routeResult, err := s.agent.ChatWithPromptContextDetailed(
				context.Background(),
				sess,
				content,
				buildWebUIChatPromptContext(sessionID, username, provider, model, fallback, explicitPromptIDs, runtimeID),
			)
			if err != nil {
				routeResp := buildChatRouteWSResponse(clientSessionID, runtimeID, routeResult)
				if data, marshalErr := json.Marshal(routeResp); marshalErr == nil {
					if err := conn.SetWriteDeadline(time.Now().Add(30 * time.Second)); err != nil {
						s.logger.Warn("Failed to set chat route deadline", zap.Error(err))
					}
					if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
						s.logger.Warn("Failed to send chat route result", zap.Error(err))
					}
				}
				sendWSError(conn, fmt.Sprintf("agent error: %v", err), clientSessionID)
				continue
			}

			// Add assistant response to session
			sess.AddMessage(agent.Message{
				Role:    "assistant",
				Content: response,
			})

			resp := chatWSResponse{
				Type:      "message",
				Content:   response,
				Timestamp: time.Now().Unix(),
				SessionID: clientSessionID,
			}
			if data, err := json.Marshal(resp); err == nil {
				if err := conn.SetWriteDeadline(time.Now().Add(30 * time.Second)); err != nil {
					s.logger.Warn("Failed to set chat response deadline", zap.Error(err))
				}
				if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
					s.logger.Warn("Failed to send chat response", zap.Error(err))
				}
			}

			routeResp := buildChatRouteWSResponse(clientSessionID, runtimeID, routeResult)
			if data, err := json.Marshal(routeResp); err == nil {
				if err := conn.SetWriteDeadline(time.Now().Add(30 * time.Second)); err != nil {
					s.logger.Warn("Failed to set chat route deadline", zap.Error(err))
				}
				if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
					s.logger.Warn("Failed to send chat route result", zap.Error(err))
				}
			}
		}
	}
}

func (s *Server) handleDaemonRuntimeChatMessage(
	ctx context.Context,
	username, runtimeID, sessionID, content string,
) (bool, string, error) {
	if s == nil || s.runtimeMgr == nil || s.agent == nil || s.agent.TaskService() == nil {
		return false, "", nil
	}
	runtimeID = strings.TrimSpace(runtimeID)
	if runtimeID == "" {
		return false, "", nil
	}
	runtimeItem, err := s.runtimeMgr.Get(ctx, runtimeID)
	if err != nil || runtimeItem == nil {
		return false, "", nil
	}
	if !runtimeIsDaemonBacked(runtimeItem) {
		return false, "", nil
	}
	machineID, _ := runtimeItem.Policy["daemon_machine_id"].(string)
	workspaceID, _ := runtimeItem.Policy["daemon_workspace_id"].(string)
	if strings.TrimSpace(machineID) == "" {
		return false, "", fmt.Errorf("daemon-backed runtime %s is missing daemon_machine_id policy", runtimeID)
	}
	taskID := "daemon-task-" + uuid.NewString()
	_, err = s.agent.TaskService().Enqueue(tasks.Task{
		ID:        taskID,
		Type:      tasks.TypeRemoteAgent,
		Summary:   content,
		SessionID: sessionID,
		RuntimeID: runtimeID,
		Metadata: map[string]any{
			"machine_id":         strings.TrimSpace(machineID),
			"workspace_id":       strings.TrimSpace(workspaceID),
			"created_by_user_id": strings.TrimSpace(username),
			"delivery":           "daemon",
		},
	})
	if err != nil {
		return true, "", err
	}
	return true, fmt.Sprintf("Daemon task queued.\nTask ID: %s\nRuntime: %s\nMachine: %s", taskID, runtimeID, machineID), nil
}

func runtimeIsDaemonBacked(item *runtimeagents.AgentRuntime) bool {
	if item == nil {
		return false
	}
	if item.Policy == nil {
		return false
	}
	value, _ := item.Policy["daemon_enabled"].(bool)
	return value
}

func (s *Server) getOrCreateChatSession(sessionID string) (agent.SessionInterface, error) {
	if s.sessionMgr == nil {
		return nil, fmt.Errorf("session manager not available")
	}
	return s.sessionMgr.GetWithSource(sessionID, session.SourceWebUI)
}

func (s *Server) handleToolSessionWS(c *echo.Context) error {
	if s.processMgr == nil || s.toolSess == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "tool runtime not available"})
	}

	tokenStr := strings.TrimSpace(c.QueryParam("token"))
	if tokenStr == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "token required"})
	}
	username, err := s.parseJWTSubject(tokenStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid token"})
	}

	sessionID := strings.TrimSpace(c.QueryParam("session_id"))
	if sessionID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "session_id required"})
	}
	sess, err := s.toolSess.GetSession(c.Request().Context(), sessionID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "session not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	if strings.TrimSpace(sess.Owner) != "" && sess.Owner != username {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "session does not belong to current user"})
	}
	s.tryRestoreToolSessionRuntime(c.Request().Context(), sessionID)

	conn, err := wsUpgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		s.logger.Error("Tool session WS upgrade failed", zap.Error(err))
		return nil
	}
	defer func() {
		_ = conn.Close()
	}()

	var writeMu sync.Mutex
	writeJSON := func(v interface{}) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		if err := conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
			s.logger.Warn("Failed to set tool session WS write deadline", zap.Error(err))
		}
		return conn.WriteJSON(v)
	}

	_ = writeJSON(toolWSResponse{
		Type:      "ready",
		SessionID: sessionID,
		Running:   true,
	})

	conn.SetReadLimit(65536)
	if err := conn.SetReadDeadline(time.Now().Add(120 * time.Second)); err != nil {
		s.logger.Warn("Failed to set tool session WS read deadline", zap.Error(err))
	}
	conn.SetPongHandler(func(string) error {
		if err := conn.SetReadDeadline(time.Now().Add(120 * time.Second)); err != nil {
			s.logger.Warn("Failed to refresh tool session WS read deadline", zap.Error(err))
		}
		return nil
	})

	done := make(chan struct{})
	var doneOnce sync.Once
	markDone := func() {
		doneOnce.Do(func() { close(done) })
	}
	defer markDone()

	go func() {
		defer markDone()
		for {
			_, payload, err := conn.ReadMessage()
			if err != nil {
				return
			}

			var msg toolWSMessage
			if err := json.Unmarshal(payload, &msg); err != nil {
				_ = writeJSON(toolWSResponse{Type: "error", Message: "invalid message format"})
				continue
			}
			switch msg.Type {
			case "ping":
				_ = writeJSON(toolWSResponse{Type: "pong"})
			case "kill":
				if s.processMgr != nil {
					_ = s.processMgr.Kill(sessionID)
				}
				s.tryKillRuntimeSession(context.Background(), sessionID)
				_ = s.toolSess.TerminateSession(context.Background(), sessionID, "killed from tool ws")
			case "resize":
				if msg.Cols <= 0 || msg.Rows <= 0 {
					continue
				}
				if err := s.processMgr.Resize(sessionID, msg.Cols, msg.Rows); err != nil && isProcessSessionNotFound(err) {
					if s.tryRestoreToolSessionRuntime(context.Background(), sessionID) {
						_ = s.processMgr.Resize(sessionID, msg.Cols, msg.Rows)
					}
				}
			case "input":
				if msg.Data == "" {
					continue
				}
				if err := s.processMgr.Write(sessionID, msg.Data); err != nil {
					if isProcessSessionNotFound(err) && s.tryRestoreToolSessionRuntime(context.Background(), sessionID) {
						if retryErr := s.processMgr.Write(sessionID, msg.Data); retryErr == nil {
							_ = s.toolSess.TouchSession(context.Background(), sessionID, toolsessions.StateRunning)
							continue
						} else {
							err = retryErr
						}
					}
					_ = writeJSON(toolWSResponse{Type: "error", Message: err.Error()})
					continue
				}
				_ = s.toolSess.TouchSession(context.Background(), sessionID, toolsessions.StateRunning)
			}
		}
	}()

	offset := 0
	lastRunning := true
	lastExit := 0
	lastMissing := false
	statusInit := false

	ticker := time.NewTicker(220 * time.Millisecond)
	defer ticker.Stop()

	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	for {
		select {
		case <-done:
			return nil
		case <-pingTicker.C:
			writeMu.Lock()
			if err := conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
				writeMu.Unlock()
				return nil
			}
			err := conn.WriteMessage(websocket.PingMessage, nil)
			writeMu.Unlock()
			if err != nil {
				return nil
			}
		case <-ticker.C:
			chunks, total, outErr := s.processMgr.GetOutput(sessionID, offset, 500)
			if outErr == nil {
				offset = total
				if len(chunks) > 0 {
					_ = writeJSON(toolWSResponse{
						Type:  "output",
						Data:  strings.Join(chunks, ""),
						Total: total,
					})
				}
			}

			status, statusErr := s.processMgr.GetStatus(sessionID)
			if statusErr != nil {
				missing := isProcessSessionNotFound(statusErr)
				if !statusInit || missing != lastMissing {
					_ = writeJSON(toolWSResponse{
						Type:    "status",
						Running: false,
						Missing: missing,
					})
					statusInit = true
					lastMissing = missing
				}
				continue
			}

			if status.Running {
				_ = s.toolSess.TouchSession(context.Background(), sessionID, toolsessions.StateRunning)
			} else {
				if rec, err := s.toolSess.GetSession(context.Background(), sessionID); err == nil &&
					rec.State != toolsessions.StateTerminated &&
					rec.State != toolsessions.StateArchived {
					_ = s.toolSess.TerminateSession(context.Background(), sessionID, fmt.Sprintf("process exited with code %d", status.ExitCode))
				}
			}

			if !statusInit || status.Running != lastRunning || status.ExitCode != lastExit || lastMissing {
				_ = writeJSON(toolWSResponse{
					Type:     "status",
					Running:  status.Running,
					ExitCode: status.ExitCode,
					Missing:  false,
				})
				statusInit = true
				lastRunning = status.Running
				lastExit = status.ExitCode
				lastMissing = false
			}
		}
	}
}

func sendWSError(conn *websocket.Conn, errMsg string, sessionID ...string) {
	targetSessionID := ""
	if len(sessionID) > 0 {
		targetSessionID = strings.TrimSpace(sessionID[0])
	}
	resp := chatWSResponse{
		Type:      "error",
		Content:   errMsg,
		Timestamp: time.Now().Unix(),
		SessionID: targetSessionID,
	}
	if data, err := json.Marshal(resp); err == nil {
		_ = conn.WriteMessage(websocket.TextMessage, data)
	}
}

func normalizeProviderNames(names []string) []string {
	if len(names) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(names))
	result := make([]string, 0, len(names))
	for _, name := range names {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func (s *Server) hasProvider(name string) bool {
	for _, p := range s.config.Providers {
		if strings.TrimSpace(p.Name) == name && providerHasRequiredAuthFields(p) {
			return true
		}
	}
	return false
}

func providerHasRequiredAuthFields(profile config.ProviderProfile) bool {
	meta, ok := providerregistry.Get(strings.ToLower(strings.TrimSpace(profile.ProviderKind)))
	if !ok {
		return true
	}
	for _, field := range meta.AuthFields {
		if !field.Required {
			continue
		}
		switch field.Key {
		case "api_key":
			if strings.TrimSpace(profile.APIKey) == "" {
				return false
			}
		}
	}
	return true
}

func (s *Server) hasProviderGroup(name string) bool {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return false
	}
	for _, group := range s.config.Agents.Defaults.ProviderGroups {
		if strings.TrimSpace(group.Name) == trimmed {
			return true
		}
	}
	return false
}

func (s *Server) hasRoutingTarget(name string) bool {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return false
	}
	return s.hasProvider(trimmed) || s.hasProviderGroup(trimmed)
}

func (s *Server) persistChatRouting(provider, model string, fallback []string) error {
	changed := false

	if provider != "" {
		if !s.hasRoutingTarget(provider) {
			return fmt.Errorf("routing target not found: %s", provider)
		}
	}
	if strings.TrimSpace(s.config.Agents.Defaults.Provider) != provider {
		s.config.Agents.Defaults.Provider = provider
		changed = true
	}
	if trimmedModel := strings.TrimSpace(model); s.config.Agents.Defaults.Model != trimmedModel {
		s.config.Agents.Defaults.Model = trimmedModel
		changed = true
	}

	for _, name := range fallback {
		if !s.hasRoutingTarget(name) {
			return fmt.Errorf("fallback routing target not found: %s", name)
		}
	}
	if !reflect.DeepEqual(s.config.Agents.Defaults.Fallback, fallback) {
		s.config.Agents.Defaults.Fallback = fallback
		changed = true
	}

	if !changed {
		return nil
	}

	if err := config.ValidateConfig(s.config); err != nil {
		return err
	}

	return config.SaveDatabaseSections(s.config, "agents")
}

func mustMarshalChatRouting(routing chatRouteSettings) string {
	data, err := json.Marshal(routing)
	if err != nil {
		return `{"provider":"","model":"","fallback":[]}`
	}
	return string(data)
}

func (s *Server) handleListPrompts(c *echo.Context) error {
	if s.prompts == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "prompt manager not available"})
	}
	items, err := s.prompts.ListPrompts(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, items)
}

func (s *Server) handleCreatePrompt(c *echo.Context) error {
	if s.prompts == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "prompt manager not available"})
	}
	var body prompts.Prompt
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	item, err := s.prompts.CreatePrompt(c.Request().Context(), body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, item)
}

func (s *Server) handleUpdatePrompt(c *echo.Context) error {
	if s.prompts == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "prompt manager not available"})
	}
	var body prompts.Prompt
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	item, err := s.prompts.UpdatePrompt(c.Request().Context(), c.Param("id"), body)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, prompts.ErrPromptNotFound) {
			status = http.StatusNotFound
		}
		return c.JSON(status, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, item)
}

func (s *Server) handleDeletePrompt(c *echo.Context) error {
	if s.prompts == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "prompt manager not available"})
	}
	err := s.prompts.DeletePrompt(c.Request().Context(), c.Param("id"))
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, prompts.ErrPromptNotFound) {
			status = http.StatusNotFound
		}
		return c.JSON(status, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleListPromptBindings(c *echo.Context) error {
	if s.prompts == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "prompt manager not available"})
	}
	items, err := s.prompts.ListBindings(
		c.Request().Context(),
		c.QueryParam("scope"),
		c.QueryParam("target"),
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, items)
}

func (s *Server) handleCreatePromptBinding(c *echo.Context) error {
	if s.prompts == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "prompt manager not available"})
	}
	var body prompts.Binding
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	item, err := s.prompts.CreateBinding(c.Request().Context(), body)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, prompts.ErrPromptNotFound) {
			status = http.StatusNotFound
		}
		return c.JSON(status, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, item)
}

func (s *Server) handleUpdatePromptBinding(c *echo.Context) error {
	if s.prompts == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "prompt manager not available"})
	}
	var body prompts.Binding
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	item, err := s.prompts.UpdateBinding(c.Request().Context(), c.Param("id"), body)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, prompts.ErrPromptNotFound) || errors.Is(err, prompts.ErrBindingNotFound) {
			status = http.StatusNotFound
		}
		return c.JSON(status, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, item)
}

func (s *Server) handleDeletePromptBinding(c *echo.Context) error {
	if s.prompts == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "prompt manager not available"})
	}
	err := s.prompts.DeleteBinding(c.Request().Context(), c.Param("id"))
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, prompts.ErrBindingNotFound) {
			status = http.StatusNotFound
		}
		return c.JSON(status, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleResolvePrompts(c *echo.Context) error {
	if s.prompts == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "prompt manager not available"})
	}
	var body prompts.ResolveInput
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if strings.TrimSpace(body.Workspace) == "" && s.config != nil {
		body.Workspace = s.config.WorkspacePath()
	}
	resolved, err := s.prompts.Resolve(c.Request().Context(), body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, resolved)
}

func (s *Server) handlePreviewContextSources(c *echo.Context) error {
	if s.agent == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "agent not available"})
	}
	var body struct {
		prompts.ResolveInput
		UserMessage string `json:"user_message"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	preview, err := s.agent.PreviewContextSources(c.Request().Context(), agent.PromptContext{
		Channel:           body.Channel,
		SessionID:         body.SessionID,
		UserID:            body.UserID,
		Username:          body.Username,
		RequestedProvider: body.RequestedProvider,
		RequestedModel:    body.RequestedModel,
		RequestedFallback: body.RequestedFallback,
		ExplicitPromptIDs: body.ExplicitPromptIDs,
		Custom:            body.Custom,
	}, body.UserMessage)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, preview)
}

func (s *Server) handleListRuntimeAgents(c *echo.Context) error {
	if s.runtimeMgr == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "runtime agent manager not available"})
	}
	items, err := s.deriveRuntimeStatuses(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, items)
}

func (s *Server) handleCreateRuntimeAgent(c *echo.Context) error {
	if s.runtimeMgr == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "runtime agent manager not available"})
	}
	var body runtimeagents.AgentRuntime
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	item, err := s.runtimeMgr.Create(c.Request().Context(), body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, item)
}

func (s *Server) handleUpdateRuntimeAgent(c *echo.Context) error {
	if s.runtimeMgr == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "runtime agent manager not available"})
	}
	var body runtimeagents.AgentRuntime
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	item, err := s.runtimeMgr.Update(c.Request().Context(), c.Param("id"), body)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, runtimeagents.ErrRuntimeNotFound) {
			status = http.StatusNotFound
		}
		return c.JSON(status, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, item)
}

func (s *Server) handleDeleteRuntimeAgent(c *echo.Context) error {
	if s.runtimeMgr == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "runtime agent manager not available"})
	}
	runtimeID := strings.TrimSpace(c.Param("id"))
	if runtimeID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "runtime id is required"})
	}
	if s.bindingMgr != nil {
		if err := s.bindingMgr.DeleteByRuntimeID(c.Request().Context(), runtimeID); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
	}
	err := s.runtimeMgr.Delete(c.Request().Context(), runtimeID)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, runtimeagents.ErrRuntimeNotFound) {
			status = http.StatusNotFound
		}
		return c.JSON(status, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleListChannelAccounts(c *echo.Context) error {
	if s.accountMgr == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "channel account manager not available"})
	}
	items, err := s.accountMgr.List(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, items)
}

func (s *Server) handleCreateChannelAccount(c *echo.Context) error {
	if s.accountMgr == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "channel account manager not available"})
	}
	var body channelaccounts.ChannelAccount
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if err := validateChannelAccountInput(body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	item, err := s.accountMgr.Create(c.Request().Context(), body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	if s.channels != nil {
		if err := s.reloadChannelsByType(item.ChannelType); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "channel account created but runtime reload failed: " + err.Error(),
			})
		}
	}
	return c.JSON(http.StatusOK, item)
}

func (s *Server) handleUpdateChannelAccount(c *echo.Context) error {
	if s.accountMgr == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "channel account manager not available"})
	}
	existing, err := s.accountMgr.Get(c.Request().Context(), c.Param("id"))
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, channelaccounts.ErrAccountNotFound) {
			status = http.StatusNotFound
		}
		return c.JSON(status, map[string]string{"error": err.Error()})
	}
	if existing == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "channel account not found"})
	}
	var body channelaccounts.ChannelAccount
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if err := validateChannelAccountInput(body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	item, err := s.accountMgr.Update(c.Request().Context(), c.Param("id"), body)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, channelaccounts.ErrAccountNotFound) {
			status = http.StatusNotFound
		}
		return c.JSON(status, map[string]string{"error": err.Error()})
	}
	previousType := strings.TrimSpace(existing.ChannelType)
	currentType := strings.TrimSpace(item.ChannelType)
	if s.channels != nil {
		if previousType != "" && previousType != currentType {
			if err := s.reloadChannelsByType(previousType); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{
					"error": "channel account updated but previous runtime reload failed: " + err.Error(),
				})
			}
		}
		if err := s.reloadChannelsByType(currentType); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "channel account updated but runtime reload failed: " + err.Error(),
			})
		}
	}
	return c.JSON(http.StatusOK, item)
}

func (s *Server) handleDeleteChannelAccount(c *echo.Context) error {
	if s.accountMgr == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "channel account manager not available"})
	}
	existing, err := s.accountMgr.Get(c.Request().Context(), c.Param("id"))
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, channelaccounts.ErrAccountNotFound) {
			status = http.StatusNotFound
		}
		return c.JSON(status, map[string]string{"error": err.Error()})
	}
	if existing == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "channel account not found"})
	}
	if s.bindingMgr != nil {
		if err := s.bindingMgr.DeleteByChannelAccountID(c.Request().Context(), existing.ID); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
	}
	err = s.accountMgr.Delete(c.Request().Context(), c.Param("id"))
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, channelaccounts.ErrAccountNotFound) {
			status = http.StatusNotFound
		}
		return c.JSON(status, map[string]string{"error": err.Error()})
	}
	if s.channels != nil {
		if err := s.reloadChannelsByType(existing.ChannelType); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "channel account deleted but runtime reload failed: " + err.Error(),
			})
		}
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
}

func validateChannelAccountInput(item channelaccounts.ChannelAccount) error {
	if !item.Enabled {
		return nil
	}

	switch strings.TrimSpace(strings.ToLower(item.ChannelType)) {
	case "wechat":
		botToken, _ := item.Config["bot_token"].(string)
		botID, _ := item.Config["ilink_bot_id"].(string)
		if strings.TrimSpace(botToken) == "" || strings.TrimSpace(botID) == "" {
			return fmt.Errorf("enabled wechat account requires config.bot_token and config.ilink_bot_id")
		}
	case "gotify":
		serverURL, _ := item.Config["server_url"].(string)
		appToken, _ := item.Config["app_token"].(string)
		if strings.TrimSpace(serverURL) == "" || strings.TrimSpace(appToken) == "" {
			return fmt.Errorf("enabled gotify account requires config.server_url and config.app_token")
		}
	}

	return nil
}

func (s *Server) handleListAccountBindings(c *echo.Context) error {
	if s.bindingMgr == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "account binding manager not available"})
	}
	items, err := s.bindingMgr.List(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, items)
}

func (s *Server) handleCreateAccountBinding(c *echo.Context) error {
	if s.bindingMgr == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "account binding manager not available"})
	}
	var body accountbindings.AccountBinding
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	item, err := s.bindingMgr.Create(c.Request().Context(), body)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, accountbindings.ErrBindingNotFound) {
			status = http.StatusNotFound
		}
		return c.JSON(status, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, item)
}

func (s *Server) handleUpdateAccountBinding(c *echo.Context) error {
	if s.bindingMgr == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "account binding manager not available"})
	}
	var body accountbindings.AccountBinding
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	item, err := s.bindingMgr.Update(c.Request().Context(), c.Param("id"), body)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, accountbindings.ErrBindingNotFound) {
			status = http.StatusNotFound
		}
		return c.JSON(status, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, item)
}

func (s *Server) handleDeleteAccountBinding(c *echo.Context) error {
	if s.bindingMgr == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "account binding manager not available"})
	}
	err := s.bindingMgr.Delete(c.Request().Context(), c.Param("id"))
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, accountbindings.ErrBindingNotFound) {
			status = http.StatusNotFound
		}
		return c.JSON(status, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleGetRuntimeTopology(c *echo.Context) error {
	if s.topologySvc == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "runtime topology service not available"})
	}
	snapshot, err := s.topologySvc.Snapshot(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, snapshot)
}

func (s *Server) handleGetChatSessionPrompts(c *echo.Context) error {
	if s.prompts == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "prompt manager not available"})
	}
	sessionID, err := s.resolveWebUIChatSessionAlias(c, c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": err.Error()})
	}
	result, err := s.prompts.GetSessionBindingSet(c.Request().Context(), sessionID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, result)
}

func (s *Server) handlePutChatSessionPrompts(c *echo.Context) error {
	if s.prompts == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "prompt manager not available"})
	}
	var body struct {
		SystemPromptIDs []string `json:"system_prompt_ids"`
		UserPromptIDs   []string `json:"user_prompt_ids"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	sessionID, err := s.resolveWebUIChatSessionAlias(c, c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": err.Error()})
	}
	result, err := s.prompts.ReplaceSessionBindings(c.Request().Context(), sessionID, body.SystemPromptIDs, body.UserPromptIDs)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, prompts.ErrPromptNotFound) {
			status = http.StatusNotFound
		}
		return c.JSON(status, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, result)
}

func (s *Server) handleDeleteChatSessionPrompts(c *echo.Context) error {
	if s.prompts == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "prompt manager not available"})
	}
	sessionID, err := s.resolveWebUIChatSessionAlias(c, c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": err.Error()})
	}
	if err := s.prompts.ClearSessionBindings(c.Request().Context(), sessionID); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleUndoChatSession(c *echo.Context) error {
	if s.sessionMgr == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "session manager not available"})
	}
	if s.snapshotMgr == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "undo is not available"})
	}

	sessionID, err := s.resolveWebUIChatSessionAlias(c, c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": err.Error()})
	}

	var body struct {
		Steps int `json:"steps"`
	}
	if err := c.Bind(&body); err != nil && !errors.Is(err, io.EOF) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	steps := body.Steps
	if steps <= 0 {
		steps = 1
	}

	sess, err := s.sessionMgr.GetExisting(sessionID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "session not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to load session: %v", err)})
	}

	store := s.snapshotMgr.GetStore(sessionID)
	if err := store.LoadSnapshots(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to load undo snapshots: %v", err)})
	}

	undone := 0
	var reverted []session.MessageSnapshot
	for undone < steps && store.CanUndo() {
		reverted, err = store.Undo()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("undo failed: %v", err)})
		}
		undone++
	}

	if undone == 0 {
		current := sess.GetMessages()
		return c.JSON(http.StatusOK, map[string]interface{}{
			"undone_steps":    0,
			"remaining_turns": store.GetTurnCount(),
			"message_count":   len(current),
			"messages":        buildSessionMessageResponses(current),
		})
	}

	messages := session.MessageSnapshotsToMessages(reverted)
	sess.ReplaceMessages(messages)
	if err := s.sessionMgr.Save(sess); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to save reverted session: %v", err)})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"undone_steps":    undone,
		"remaining_turns": store.GetTurnCount(),
		"message_count":   len(messages),
		"messages":        buildSessionMessageResponses(messages),
	})
}

// --- Approval Handlers ---

func (s *Server) handleGetApprovals(c *echo.Context) error {
	pending := s.approval.GetPending()
	return c.JSON(http.StatusOK, pending)
}

func (s *Server) handleApproveRequest(c *echo.Context) error {
	id := c.Param("id")
	req, _ := s.approval.GetRequest(id)
	if err := s.approval.Approve(id); err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
	}
	if req != nil && s.approval != nil && isExternalAgentApprovalRequest(req) {
		s.approval.SetSessionMode(req.SessionID, approval.ModeAuto)
		if s.toolSess != nil {
			sess, err := s.toolSess.GetSession(c.Request().Context(), req.SessionID)
			if err == nil && sess != nil {
				if err := s.ensureExternalAgentProcess(c.Request().Context(), sess); err != nil {
					return c.JSON(http.StatusBadRequest, map[string]string{"error": "failed to continue external agent launch: " + err.Error()})
				}
			}
		}
	}
	if req != nil {
		if pendingToolCall, ok := approval.PendingToolCallForRequest(id); ok {
			if s.agent != nil {
				s.approval.SetSessionMode(req.SessionID, approval.ModeAuto)
				if _, err := s.agent.ReplayApprovedToolCall(c.Request().Context(), pendingToolCall.SessionID, pendingToolCall.Call); err != nil {
					return c.JSON(http.StatusBadRequest, map[string]string{"error": "failed to replay approved tool call: " + err.Error()})
				}
			}
			approval.ClearPendingToolCall(id)
		}
	}
	if req != nil && s.taskStore != nil {
		s.taskStore.ClearSessionPendingAction(req.SessionID)
		if mode, ok := s.approval.GetSessionMode(req.SessionID); ok {
			s.taskStore.SetSessionPermissionMode(req.SessionID, string(mode))
		}
	}
	body := map[string]any{"status": "approved", "id": id}
	if req != nil {
		if state := s.currentSessionRuntimeState(req.SessionID); state != nil {
			body["session_runtime_state"] = state
		}
	}
	return c.JSON(http.StatusOK, body)
}

func (s *Server) handleDenyRequest(c *echo.Context) error {
	id := c.Param("id")
	var body struct {
		Reason string `json:"reason"`
	}
	_ = c.Bind(&body) // reason is optional

	req, _ := s.approval.GetRequest(id)
	if err := s.approval.Deny(id, body.Reason); err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
	}
	approval.ClearPendingToolCall(id)
	if req != nil && s.taskStore != nil {
		s.taskStore.ClearSessionPendingAction(req.SessionID)
	}
	response := map[string]any{"status": "denied", "id": id}
	if req != nil {
		if state := s.currentSessionRuntimeState(req.SessionID); state != nil {
			response["session_runtime_state"] = state
		}
	}
	return c.JSON(http.StatusOK, response)
}

func (s *Server) currentSessionRuntimeState(sessionID string) *tasks.SessionState {
	if s == nil || s.taskStore == nil {
		return nil
	}
	state, ok := s.taskStore.GetSessionState(sessionID)
	if !ok {
		return nil
	}
	return &state
}

func isExternalAgentApprovalRequest(req *approval.Request) bool {
	if req == nil {
		return false
	}
	if strings.TrimSpace(req.ToolName) == "" {
		return false
	}
	toolName, _ := req.Arguments["tool_name"].(string)
	sessionID, _ := req.Arguments["session_id"].(string)
	return strings.TrimSpace(toolName) == strings.TrimSpace(req.ToolName) &&
		strings.TrimSpace(sessionID) == strings.TrimSpace(req.SessionID)
}

func (s *Server) ensureExternalAgentProcess(ctx context.Context, sess *toolsessions.Session) error {
	if s == nil {
		return nil
	}
	var probe externalAgentProcessProbe
	var starter externalagent.ProcessStarter
	if s.processMgr != nil {
		probe.mgr = s.processMgr
		starter = s.processMgr
	}
	return externalagent.EnsureProcess(
		ctx,
		s.config.WorkspacePath(),
		probe,
		starter,
		s.toolSess,
		runtimeagents.DefaultTransport(),
		sess,
	)
}

type externalAgentProcessProbe struct {
	mgr *process.Manager
}

func (p externalAgentProcessProbe) HasProcess(sessionID string) bool {
	if p.mgr == nil {
		return false
	}
	_, err := p.mgr.GetStatus(sessionID)
	return err == nil
}

func (s *Server) handleGetPermissionRules(c *echo.Context) error {
	manager, err := permissionrules.NewManager(s.config, s.logger, s.entClient)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	items, err := manager.List(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, items)
}

func (s *Server) handleCreatePermissionRule(c *echo.Context) error {
	var input permissionrules.Rule
	if err := c.Bind(&input); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	manager, err := permissionrules.NewManager(s.config, s.logger, s.entClient)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	item, err := manager.Create(c.Request().Context(), input)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusCreated, map[string]interface{}{"status": "created", "rule": item})
}

func (s *Server) handleUpdatePermissionRule(c *echo.Context) error {
	var input permissionrules.Rule
	if err := c.Bind(&input); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	manager, err := permissionrules.NewManager(s.config, s.logger, s.entClient)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	item, err := manager.Update(c.Request().Context(), strings.TrimSpace(c.Param("id")), input)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{"status": "updated", "rule": item})
}

func (s *Server) handleDeletePermissionRule(c *echo.Context) error {
	manager, err := permissionrules.NewManager(s.config, s.logger, s.entClient)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	id := strings.TrimSpace(c.Param("id"))
	if err := manager.Delete(c.Request().Context(), id); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "deleted", "id": id})
}

func (s *Server) handleGetPolicyPresets(c *echo.Context) error {
	return c.JSON(http.StatusOK, policy.Presets())
}

func (s *Server) handleEvaluatePolicy(c *echo.Context) error {
	var body struct {
		Policy policy.Policy          `json:"policy"`
		Input  policy.EvaluationInput `json:"input"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	return c.JSON(http.StatusOK, policy.Evaluate(body.Policy, body.Input))
}

// --- Helpers ---

func (s *Server) parseJWTSubject(tokenStr string) (string, error) {
	parsed, err := jwt.Parse(strings.TrimSpace(tokenStr), func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(s.getJWTSecret()), nil
	})
	if err != nil || parsed == nil || !parsed.Valid {
		return "", fmt.Errorf("invalid token")
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return "", fmt.Errorf("invalid token claims")
	}
	sub, _ := claims["sub"].(string)
	sub = strings.TrimSpace(sub)
	if sub == "" {
		return "", fmt.Errorf("subject is empty")
	}
	return sub, nil
}

func (s *Server) resolveWebUIChatSessionAlias(c *echo.Context, rawID string) (string, error) {
	sessionID := strings.TrimSpace(rawID)
	runtimeID, isAlias := resolveWebUIChatRuntimeAlias(sessionID)
	if !isAlias {
		return sessionID, nil
	}
	authHeader := strings.TrimSpace(c.Request().Header.Get("Authorization"))
	if authHeader == "" {
		return "", fmt.Errorf("authorization required")
	}
	tokenStr := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer"))
	if tokenStr == "" {
		return "", fmt.Errorf("authorization required")
	}
	username, err := s.parseJWTSubject(tokenStr)
	if err != nil {
		return "", fmt.Errorf("invalid token")
	}
	return webUIRuntimeChatSessionID(username, runtimeID), nil
}

func resolveWebUIChatRuntimeAlias(sessionID string) (string, bool) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "webui-chat" {
		return "", true
	}

	prefix := inboundrouter.SessionPrefix + ":"
	if !strings.HasPrefix(sessionID, prefix) {
		return "", false
	}

	parts := strings.Split(sessionID, ":")
	if len(parts) != 3 {
		return "", false
	}
	if parts[0] != inboundrouter.SessionPrefix || parts[2] != "webui-chat" {
		return "", false
	}

	runtimeID := strings.TrimSpace(parts[1])
	if runtimeID == "" {
		return "", false
	}
	return runtimeID, true
}

func runtimePromptIDs(promptID string) []string {
	promptID = strings.TrimSpace(promptID)
	if promptID == "" {
		return nil
	}
	return []string{promptID}
}

func (s *Server) getJWTSecret() string {
	secret, err := config.GetJWTSecret(s.entClient)
	if err == nil && strings.TrimSpace(secret) != "" {
		return secret
	}
	// No credential stored yet — generate an ephemeral secret.
	// It will be replaced once the admin initializes their password.
	return "nekobot-ephemeral-secret"
}

func (s *Server) getDaemonToken() string {
	if s == nil || s.kvStore == nil {
		return "nekobot-daemon-ephemeral-token"
	}
	ctx := context.Background()
	if value, ok, err := s.kvStore.GetString(ctx, "daemonhost.auth.token"); err == nil && ok && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	token := "daemon-" + config.GenerateJWTSecret()
	_ = s.kvStore.Set(ctx, "daemonhost.auth.token", token)
	return token
}

func (s *Server) sessionRuntimeBindingKey(sessionID string) string {
	return "webui.session_runtime." + strings.TrimSpace(sessionID)
}

func (s *Server) sessionThreadTopicKey(sessionID string) string {
	return "webui.session_thread.topic." + strings.TrimSpace(sessionID)
}

func (s *Server) getThreadRuntimeBinding(sessionID string) string {
	if s == nil || s.threads == nil {
		return ""
	}
	record, ok, err := s.threads.Get(context.Background(), sessionID)
	if err != nil || !ok {
		return ""
	}
	return strings.TrimSpace(record.RuntimeID)
}

func (s *Server) setThreadRuntimeBinding(sessionID, runtimeID string) error {
	if s == nil || s.threads == nil {
		return nil
	}
	record, _, _ := s.threads.Get(context.Background(), sessionID)
	return s.threads.Upsert(context.Background(), sessionID, strings.TrimSpace(runtimeID), record.Topic)
}

func (s *Server) getThreadTopic(sessionID string) string {
	if s == nil || s.threads == nil {
		return ""
	}
	record, ok, err := s.threads.Get(context.Background(), sessionID)
	if err != nil || !ok {
		return ""
	}
	return strings.TrimSpace(record.Topic)
}

func (s *Server) setThreadTopic(sessionID, topic string) error {
	if s == nil || s.threads == nil {
		return nil
	}
	record, _, _ := s.threads.Get(context.Background(), sessionID)
	return s.threads.Upsert(context.Background(), sessionID, record.RuntimeID, strings.TrimSpace(topic))
}

func (s *Server) deleteThread(sessionID string) error {
	if s == nil || s.threads == nil {
		return nil
	}
	return s.threads.Delete(context.Background(), sessionID)
}

func (s *Server) authorizeDaemonRequest(c *echo.Context) bool {
	authHeader := strings.TrimSpace(c.Request().Header.Get("Authorization"))
	if authHeader == "" {
		return false
	}
	token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer"))
	if token == "" {
		return false
	}
	return token == s.getDaemonToken()
}

func (s *Server) generateToken(profile *config.AuthProfile) (string, error) {
	if profile == nil {
		return "", fmt.Errorf("auth profile is nil")
	}
	now := time.Now()
	claims := jwt.MapClaims{
		"sub":  profile.Username,
		"uid":  profile.UserID,
		"role": profile.Role,
		"tid":  profile.TenantID,
		"ts":   profile.TenantSlug,
		"exp":  now.Add(24 * time.Hour).Unix(),
		"iat":  now.Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.getJWTSecret()))
}
