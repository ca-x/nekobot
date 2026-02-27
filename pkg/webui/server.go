// Package webui provides a web-based dashboard for nekobot.
// It uses Echo v5 for HTTP routing with JWT authentication,
// and serves an embedded SPA frontend for configuration management
// and chat playground.
package webui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	echojwt "github.com/labstack/echo-jwt/v5"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"go.uber.org/zap"

	"nekobot/pkg/agent"
	"nekobot/pkg/approval"
	"nekobot/pkg/bus"
	"nekobot/pkg/channels"
	"nekobot/pkg/commands"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/process"
	"nekobot/pkg/providers"
	"nekobot/pkg/providerstore"
	"nekobot/pkg/storage/ent"
	"nekobot/pkg/toolsessions"
	"nekobot/pkg/userprefs"
	"nekobot/pkg/version"
	"nekobot/pkg/webui/frontend"
)

// Server is the WebUI HTTP server.
type Server struct {
	echo       *echo.Echo
	httpServer *http.Server
	config     *config.Config
	loader     *config.Loader
	logger     *logger.Logger
	agent      *agent.Agent
	approval   *approval.Manager
	channels   *channels.Manager
	bus        bus.Bus
	commands   *commands.Registry
	prefs      *userprefs.Manager
	toolSess   *toolsessions.Manager
	processMgr *process.Manager
	providers  *providerstore.Manager
	entClient  *ent.Client
	port       int
	startedAt  time.Time
}

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
	processManager *process.Manager,
	providerStore *providerstore.Manager,
	entClient *ent.Client,
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
		config:     cfg,
		loader:     loader,
		logger:     log,
		agent:      ag,
		approval:   approvalMgr,
		channels:   chanMgr,
		bus:        messageBus,
		commands:   cmdRegistry,
		prefs:      prefsMgr,
		toolSess:   toolSessionMgr,
		processMgr: processManager,
		providers:  providerStore,
		entClient:  entClient,
		port:       port,
		startedAt:  time.Now(),
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

	// Chat WebSocket (auth handled inside via token query param)
	e.GET("/api/chat/ws", s.handleChatWS)
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
	api.GET("/providers", s.handleGetProviders)
	api.POST("/providers", s.handleCreateProvider)
	api.POST("/providers/discover-models", s.handleDiscoverProviderModels)
	api.PUT("/providers/:name", s.handleUpdateProvider)
	api.DELETE("/providers/:name", s.handleDeleteProvider)

	// Channel routes
	api.GET("/channels", s.handleGetChannels)
	api.PUT("/channels/:name", s.handleUpdateChannel)
	api.POST("/channels/:name/test", s.handleTestChannel)

	// Config routes
	api.GET("/config", s.handleGetConfig)
	api.PUT("/config", s.handleSaveConfig)
	api.GET("/config/export", s.handleExportConfig)
	api.POST("/config/import", s.handleImportConfig)

	// Status
	api.GET("/status", s.handleStatus)

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
	api.GET("/tool-sessions/:id/process/status", s.handleToolSessionProcessStatus)
	api.GET("/tool-sessions/:id/process/output", s.handleToolSessionProcessOutput)
	api.POST("/tool-sessions/:id/process/input", s.handleToolSessionProcessInput)
	api.POST("/tool-sessions/:id/process/kill", s.handleToolSessionProcessKill)
	api.POST("/tool-sessions/cleanup-terminated", s.handleCleanupTerminatedToolSessions)

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
				f.Close()
			}
			fileServer.ServeHTTP(w, r)
		})))
	}

	s.echo = e
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
	return c.JSON(http.StatusOK, map[string]bool{"initialized": initialized})
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
		Username string `json:"username"`
		Password string `json:"password"`
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
		"token": token,
		"user":  s.authProfileResponse(profile),
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
		providers[i] = providerProfileToMap(p)
	}
	return c.JSON(http.StatusOK, providers)
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
		"provider": providerProfileToMap(*created),
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
		"provider": providerProfileToMap(*updated),
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
		"name":          p.Name,
		"provider_kind": p.ProviderKind,
		"api_key":       p.APIKey,
		"api_base":      p.APIBase,
		"proxy":         p.Proxy,
		"models":        p.Models,
		"default_model": p.DefaultModel,
		"timeout":       p.Timeout,
	}
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
	if defaultProvider != "" && !s.hasProvider(defaultProvider) {
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
		if !s.hasProvider(trimmed) {
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

	kind := strings.TrimSpace(profile.ProviderKind)
	if kind == "" {
		kind = strings.TrimSpace(profile.Name)
	}
	if kind == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "provider_kind is required"})
	}

	models, err := s.discoverModels(kind, &profile)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"provider_kind": kind,
		"models":        models,
	})
}

func (s *Server) discoverModels(kind string, profile *config.ProviderProfile) ([]string, error) {
	kind = strings.ToLower(strings.TrimSpace(kind))

	if kind == "openai" || kind == "generic" || kind == "openrouter" || kind == "groq" || kind == "vllm" || kind == "deepseek" || kind == "moonshot" || kind == "zhipu" || kind == "nvidia" {
		if models, err := discoverOpenAICompatibleModels(profile.APIBase, profile.APIKey, profile.Proxy, profile.Timeout); err == nil && len(models) > 0 {
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
	defer resp.Body.Close()

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
	s.tryKillTmuxSession(id)
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
		Tool           string                 `json:"tool"`
		Title          string                 `json:"title"`
		Command        string                 `json:"command"`
		CommandArgs    string                 `json:"command_args"`
		Workdir        string                 `json:"workdir"`
		Metadata       map[string]interface{} `json:"metadata"`
		AccessMode     string                 `json:"access_mode"`
		AccessPassword string                 `json:"access_password"`
		ProxyMode      string                 `json:"proxy_mode"`
		ProxyURL       string                 `json:"proxy_url"`
		PublicBaseURL  string                 `json:"public_base_url"`
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
	tmuxSession := ""
	if wrapped, sessionName := buildToolRuntimeCommand(launchCommand, sess.ID); sessionName != "" {
		launchCommand = wrapped
		tmuxSession = sessionName
	}

	if err := s.processMgr.Start(context.Background(), sess.ID, launchCommand, workdir); err != nil {
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

	_ = s.toolSess.AppendEvent(context.Background(), sess.ID, "process_started", map[string]interface{}{
		"command":      command,
		"launch_cmd":   launchCommand,
		"tmux_session": tmuxSession,
		"workdir":      workdir,
		"proxy_mode":   proxyMode,
	})
	return c.JSON(http.StatusCreated, map[string]interface{}{
		"session":         sess,
		"access_mode":     sess.AccessMode,
		"access_url":      accessURL,
		"access_password": accessPassword,
	})
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
		Tool           string `json:"tool"`
		Title          string `json:"title"`
		Command        string `json:"command"`
		CommandArgs    string `json:"command_args"`
		Workdir        string `json:"workdir"`
		AccessMode     string `json:"access_mode"`
		AccessPassword string `json:"access_password"`
		ProxyMode      string `json:"proxy_mode"`
		ProxyURL       string `json:"proxy_url"`
		PublicBaseURL  string `json:"public_base_url"`
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

	updated, err := s.toolSess.UpdateSessionConfig(c.Request().Context(), id, toolName, strings.TrimSpace(body.Title), command, workdir)
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
	updated, _ = s.toolSess.GetSession(c.Request().Context(), id)

	accessPassword := ""
	modeChanged := strings.TrimSpace(body.AccessMode) != "" || strings.TrimSpace(body.AccessPassword) != ""
	if modeChanged {
		mode := strings.TrimSpace(body.AccessMode)
		if mode == "" {
			mode = strings.TrimSpace(updated.AccessMode)
		}
		accessPassword, err = s.toolSess.ConfigureSessionAccess(c.Request().Context(), id, mode, body.AccessPassword)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "failed to configure session access: " + err.Error()})
		}
		updated, _ = s.toolSess.GetSession(c.Request().Context(), id)
	}

	accessURL := ""
	if strings.TrimSpace(updated.AccessMode) != "" && updated.AccessMode != toolsessions.AccessModeNone {
		accessURL = s.buildToolSessionAccessURL(c, id, strings.TrimSpace(body.PublicBaseURL))
	}
	_ = s.toolSess.AppendEvent(context.Background(), id, "session_updated", map[string]interface{}{
		"command":    command,
		"workdir":    workdir,
		"proxy_mode": proxyMode,
	})

	return c.JSON(http.StatusOK, map[string]interface{}{
		"session":         updated,
		"access_mode":     updated.AccessMode,
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
		Tool           string `json:"tool"`
		Title          string `json:"title"`
		Command        string `json:"command"`
		CommandArgs    string `json:"command_args"`
		Workdir        string `json:"workdir"`
		AccessMode     string `json:"access_mode"`
		AccessPassword string `json:"access_password"`
		ProxyMode      string `json:"proxy_mode"`
		ProxyURL       string `json:"proxy_url"`
		PublicBaseURL  string `json:"public_base_url"`
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

	launchCommand := applyToolProxyToCommand(command, proxyMode, proxyURL)
	tmuxSession := ""
	if wrapped, sessionName := buildToolRuntimeCommand(launchCommand, id); sessionName != "" {
		launchCommand = wrapped
		tmuxSession = sessionName
	}

	_ = s.processMgr.Reset(id)
	s.tryKillTmuxSession(id)
	if err := s.processMgr.Start(context.Background(), id, launchCommand, workdir); err != nil {
		_ = s.toolSess.TerminateSession(context.Background(), id, "failed to restart process: "+err.Error())
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "failed to restart tool process: " + err.Error()})
	}

	updated, err := s.toolSess.UpdateSessionLaunch(c.Request().Context(), id, toolName, strings.TrimSpace(body.Title), command, workdir)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	nextMetadata := withToolProxyMetadata(cloneMap(current.Metadata), proxyMode, proxyURL)
	nextMetadata["user_command"] = command
	nextMetadata["user_args"] = strings.TrimSpace(body.CommandArgs)
	if err := s.toolSess.UpdateSessionMetadata(c.Request().Context(), id, nextMetadata); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	updated, _ = s.toolSess.GetSession(c.Request().Context(), id)

	accessPassword := ""
	modeChanged := strings.TrimSpace(body.AccessMode) != "" || strings.TrimSpace(body.AccessPassword) != ""
	if modeChanged {
		mode := strings.TrimSpace(body.AccessMode)
		if mode == "" {
			mode = strings.TrimSpace(updated.AccessMode)
		}
		accessPassword, err = s.toolSess.ConfigureSessionAccess(c.Request().Context(), id, mode, body.AccessPassword)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "failed to configure session access: " + err.Error()})
		}
		updated, _ = s.toolSess.GetSession(c.Request().Context(), id)
	}

	accessURL := ""
	if strings.TrimSpace(updated.AccessMode) != "" && updated.AccessMode != toolsessions.AccessModeNone {
		accessURL = s.buildToolSessionAccessURL(c, id, strings.TrimSpace(body.PublicBaseURL))
	}

	_ = s.toolSess.AppendEvent(context.Background(), id, "process_restarted", map[string]interface{}{
		"command":      command,
		"launch_cmd":   launchCommand,
		"tmux_session": tmuxSession,
		"workdir":      workdir,
		"proxy_mode":   proxyMode,
	})

	return c.JSON(http.StatusOK, map[string]interface{}{
		"session":         updated,
		"access_mode":     updated.AccessMode,
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

	status, err := s.processMgr.GetStatus(id)
	if err != nil {
		if isProcessSessionNotFound(err) {
			return c.JSON(http.StatusOK, map[string]interface{}{
				"id":      id,
				"running": false,
				"missing": true,
			})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, status)
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
	s.tryKillTmuxSession(id)
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

	attachCmd, tmuxName, ok := buildToolReattachCommand(id)
	if !ok {
		return false
	}
	workdir := strings.TrimSpace(sess.Workdir)
	if workdir == "" {
		workdir = s.config.WorkspacePath()
	}
	if err := s.processMgr.Start(context.Background(), id, attachCmd, workdir); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "session already exists") {
			return true
		}
		s.logger.Warn("Failed to restore tool session runtime from tmux",
			zap.String("session_id", id),
			zap.String("tmux_session", tmuxName),
			zap.Error(err),
		)
		return false
	}

	_ = s.toolSess.AppendEvent(context.Background(), id, "process_restored", map[string]interface{}{
		"launch_cmd":   attachCmd,
		"tmux_session": tmuxName,
		"workdir":      workdir,
	})
	_ = s.toolSess.TouchSession(context.Background(), id, toolsessions.StateRunning)
	s.logger.Info("Restored tool session runtime from tmux",
		zap.String("session_id", id),
		zap.String("tmux_session", tmuxName),
	)
	return true
}

func tmuxAvailable() bool {
	_, err := exec.LookPath("tmux")
	return err == nil
}

func toolShellPath() string {
	candidates := []string{
		"/bin/sh",
		"/usr/bin/sh",
		"/bin/bash",
		"/usr/bin/bash",
		"/usr/local/bin/bash",
		"/bin/zsh",
		"/usr/bin/zsh",
		"/usr/local/bin/zsh",
		"/bin/ash",
		"/usr/bin/ash",
		"/system/bin/sh",
		"/usr/bin/fish",
		"/bin/fish",
		"/usr/local/bin/fish",
	}
	for _, path := range candidates {
		if !isExecutableShell(path) {
			continue
		}
		return path
	}
	lookupNames := []string{"sh", "bash", "zsh", "ash", "fish"}
	for _, name := range lookupNames {
		lookedUp, err := exec.LookPath(name)
		if err != nil {
			continue
		}
		if isExecutableShell(lookedUp) {
			return lookedUp
		}
	}
	return "sh"
}

func isExecutableShell(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return info.Mode()&0o111 != 0
}

func buildToolRuntimeCommand(command, sessionID string) (string, string) {
	if !tmuxAvailable() {
		return command, ""
	}
	name := tmuxSessionName(sessionID)
	// Run the requested command inside tmux so the terminal session can survive reconnects.
	wrapped := fmt.Sprintf("tmux new-session -A -s %s %s -c %s", name, strconv.Quote(toolShellPath()), strconv.Quote(command))
	return wrapped, name
}

func buildToolReattachCommand(sessionID string) (string, string, bool) {
	if !tmuxAvailable() {
		return "", "", false
	}
	name := tmuxSessionName(sessionID)
	if !tmuxSessionExists(name) {
		return "", "", false
	}
	return fmt.Sprintf("tmux attach-session -t %s", name), name, true
}

func tmuxSessionExists(name string) bool {
	if !tmuxAvailable() {
		return false
	}
	return exec.Command("tmux", "has-session", "-t", strings.TrimSpace(name)).Run() == nil
}

func tmuxSessionName(sessionID string) string {
	raw := strings.TrimSpace(strings.ToLower(sessionID))
	if raw == "" {
		return "nekobot_session"
	}
	var b strings.Builder
	b.WriteString("nekobot_")
	for _, r := range raw {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	name := b.String()
	if len(name) > 40 {
		name = name[:40]
	}
	if name == "nekobot_" {
		return "nekobot_session"
	}
	return name
}

func (s *Server) tryKillTmuxSession(sessionID string) {
	if !tmuxAvailable() {
		return
	}
	_ = exec.Command("tmux", "kill-session", "-t", tmuxSessionName(sessionID)).Run()
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

// --- Channel Handlers ---

func (s *Server) handleGetChannels(c *echo.Context) error {
	// Return editable channel configs for dashboard.
	channels := map[string]interface{}{
		"telegram":   s.config.Channels.Telegram,
		"discord":    s.config.Channels.Discord,
		"slack":      s.config.Channels.Slack,
		"whatsapp":   s.config.Channels.WhatsApp,
		"feishu":     s.config.Channels.Feishu,
		"dingtalk":   s.config.Channels.DingTalk,
		"qq":         s.config.Channels.QQ,
		"wework":     s.config.Channels.WeWork,
		"serverchan": s.config.Channels.ServerChan,
		"googlechat": s.config.Channels.GoogleChat,
		"maixcam":    s.config.Channels.MaixCam,
		"teams":      s.config.Channels.Teams,
		"infoflow":   s.config.Channels.Infoflow,
	}
	return c.JSON(http.StatusOK, channels)
}

func (s *Server) handleUpdateChannel(c *echo.Context) error {
	name := c.Param("name")

	var body map[string]interface{}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	// Marshal the body to JSON, then unmarshal into the appropriate channel config
	data, err := json.Marshal(body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	switch name {
	case "telegram":
		if err := json.Unmarshal(data, &s.config.Channels.Telegram); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
	case "discord":
		if err := json.Unmarshal(data, &s.config.Channels.Discord); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
	case "slack":
		if err := json.Unmarshal(data, &s.config.Channels.Slack); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
	case "whatsapp":
		if err := json.Unmarshal(data, &s.config.Channels.WhatsApp); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
	case "feishu":
		if err := json.Unmarshal(data, &s.config.Channels.Feishu); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
	case "dingtalk":
		if err := json.Unmarshal(data, &s.config.Channels.DingTalk); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
	case "qq":
		if err := json.Unmarshal(data, &s.config.Channels.QQ); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
	case "wework":
		if err := json.Unmarshal(data, &s.config.Channels.WeWork); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
	case "serverchan":
		if err := json.Unmarshal(data, &s.config.Channels.ServerChan); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
	case "googlechat":
		if err := json.Unmarshal(data, &s.config.Channels.GoogleChat); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
	case "maixcam":
		if err := json.Unmarshal(data, &s.config.Channels.MaixCam); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
	case "teams":
		if err := json.Unmarshal(data, &s.config.Channels.Teams); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
	case "infoflow":
		if err := json.Unmarshal(data, &s.config.Channels.Infoflow); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
	default:
		return c.JSON(http.StatusNotFound, map[string]string{"error": "unknown channel: " + name})
	}

	// Persist runtime channel config to database.
	if err := config.SaveDatabaseSections(s.config, "channels"); err != nil {
		s.logger.Error("Failed to persist channel config", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to save config"})
	}

	if err := s.reloadChannel(name); err != nil {
		s.logger.Error("Failed to reload channel", zap.String("channel", name), zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "channel config saved but reload failed: " + err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "updated", "channel": name, "reload": "ok"})
}

func (s *Server) reloadChannel(name string) error {
	enabled, err := channels.IsChannelEnabled(name, s.config)
	if err != nil {
		return err
	}

	if !enabled {
		return s.channels.StopChannel(name)
	}

	ch, err := channels.BuildChannel(name, s.logger, s.bus, s.agent, s.commands, s.prefs, s.config)
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

	result["reachable"] = true
	result["status"] = "ok"
	return c.JSON(http.StatusOK, result)
}

// --- Config Handlers ---

func (s *Server) handleGetConfig(c *echo.Context) error {
	// Return sanitized config (no secrets)
	return c.JSON(http.StatusOK, map[string]interface{}{
		"agents":    s.config.Agents,
		"gateway":   s.config.Gateway,
		"tools":     s.config.Tools,
		"heartbeat": s.config.Heartbeat,
		"approval":  s.config.Approval,
		"logger":    s.config.Logger,
		"memory":    s.config.Memory,
		"webui":     s.config.WebUI,
	})
}

func (s *Server) handleSaveConfig(c *echo.Context) error {
	var body struct {
		Agents    *config.AgentsConfig    `json:"agents"`
		Gateway   *config.GatewayConfig   `json:"gateway"`
		Tools     *config.ToolsConfig     `json:"tools"`
		Heartbeat *config.HeartbeatConfig `json:"heartbeat"`
		Approval  *config.ApprovalConfig  `json:"approval"`
		Logger    *config.LoggerConfig    `json:"logger"`
		Memory    *config.MemoryConfig    `json:"memory"`
		WebUI     *config.WebUIConfig     `json:"webui"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	// Apply partial updates
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
	if body.Approval != nil {
		s.config.Approval = *body.Approval
	}
	if body.Logger != nil {
		s.config.Logger = *body.Logger
	}
	if body.Memory != nil {
		s.config.Memory = *body.Memory
	}
	if body.WebUI != nil {
		s.config.WebUI = *body.WebUI
	}

	// Validate
	if err := config.ValidateConfig(s.config); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	sections := make([]string, 0, 8)
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
	if body.Approval != nil {
		sections = append(sections, "approval")
	}
	if body.Logger != nil {
		sections = append(sections, "logger")
	}
	if body.Memory != nil {
		sections = append(sections, "memory")
	}
	if body.WebUI != nil {
		sections = append(sections, "webui")
	}

	// Persist runtime config sections to database.
	if len(sections) > 0 {
		if err := config.SaveDatabaseSections(s.config, sections...); err != nil {
			s.logger.Error("Failed to persist config sections", zap.Error(err), zap.Strings("sections", sections))
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to save config"})
		}
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "saved"})
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
		"agents":    s.config.Agents,
		"gateway":   s.config.Gateway,
		"tools":     s.config.Tools,
		"heartbeat": s.config.Heartbeat,
		"approval":  s.config.Approval,
		"logger":    s.config.Logger,
		"memory":    s.config.Memory,
		"webui":     s.config.WebUI,
		"providers": providerList,
	}

	c.Response().Header().Set("Content-Disposition", `attachment; filename="nekobot-config-export.json"`)
	return c.JSON(http.StatusOK, export)
}

func (s *Server) handleImportConfig(c *echo.Context) error {
	var body struct {
		Agents    *config.AgentsConfig     `json:"agents"`
		Gateway   *config.GatewayConfig    `json:"gateway"`
		Tools     *config.ToolsConfig      `json:"tools"`
		Heartbeat *config.HeartbeatConfig  `json:"heartbeat"`
		Approval  *config.ApprovalConfig   `json:"approval"`
		Logger    *config.LoggerConfig     `json:"logger"`
		Memory    *config.MemoryConfig     `json:"memory"`
		WebUI     *config.WebUIConfig      `json:"webui"`
		Providers []config.ProviderProfile `json:"providers"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
	}

	// Apply config sections
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
	if body.Approval != nil {
		s.config.Approval = *body.Approval
	}
	if body.Logger != nil {
		s.config.Logger = *body.Logger
	}
	if body.Memory != nil {
		s.config.Memory = *body.Memory
	}
	if body.WebUI != nil {
		s.config.WebUI = *body.WebUI
	}

	// Validate
	if err := config.ValidateConfig(s.config); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	// Persist runtime sections to database
	sections := make([]string, 0, 8)
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
	if body.Approval != nil {
		sections = append(sections, "approval")
	}
	if body.Logger != nil {
		sections = append(sections, "logger")
	}
	if body.Memory != nil {
		sections = append(sections, "memory")
	}
	if body.WebUI != nil {
		sections = append(sections, "webui")
	}
	if len(sections) > 0 {
		if err := config.SaveDatabaseSections(s.config, sections...); err != nil {
			s.logger.Error("Failed to persist imported config sections", zap.Error(err), zap.Strings("sections", sections))
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to save config sections"})
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
	})
}

// --- Status Handler ---

func (s *Server) handleStatus(c *echo.Context) error {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	uptime := time.Since(s.startedAt)

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
		"gateway_host":       s.config.Gateway.Host,
		"gateway_port":       s.config.Gateway.Port,
		"gateway": map[string]interface{}{
			"host": s.config.Gateway.Host,
			"port": s.config.Gateway.Port,
		},
	})
}

// --- Chat WebSocket Playground ---

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// chatSession implements agent.SessionInterface for WebUI chat playground.
type chatSession struct {
	messages []agent.Message
	mu       sync.RWMutex
}

func (cs *chatSession) GetMessages() []agent.Message {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.messages
}

func (cs *chatSession) AddMessage(msg agent.Message) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.messages = append(cs.messages, msg)
}

type chatWSMessage struct {
	Type     string   `json:"type"`               // "message", "ping", "clear"
	Content  string   `json:"content"`            // User message text
	Model    string   `json:"model"`              // Optional model override
	Provider string   `json:"provider,omitempty"` // Optional provider override
	Fallback []string `json:"fallback,omitempty"` // Optional fallback provider order
}

type chatWSResponse struct {
	Type      string `json:"type"`                // "message", "thinking", "error", "system", "pong"
	Content   string `json:"content"`             // Response text
	Thinking  string `json:"thinking,omitempty"`  // Model's thinking (if extended thinking enabled)
	Timestamp int64  `json:"timestamp,omitempty"` // Unix timestamp
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

func (s *Server) handleChatWS(c *echo.Context) error {
	// Authenticate via token query param (since WebSocket can't use Authorization header easily)
	tokenStr := c.QueryParam("token")
	if tokenStr == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "token required"})
	}
	if _, err := s.parseJWTSubject(tokenStr); err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid token"})
	}

	// Upgrade to WebSocket
	conn, err := wsUpgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		s.logger.Error("WebUI chat WS upgrade failed", zap.Error(err))
		return nil
	}
	defer conn.Close()

	sess := &chatSession{}

	// Send welcome
	welcome := chatWSResponse{
		Type:      "system",
		Content:   "Connected to chat playground",
		Timestamp: time.Now().Unix(),
	}
	if data, err := json.Marshal(welcome); err == nil {
		conn.WriteMessage(websocket.TextMessage, data)
	}

	// Read loop
	conn.SetReadLimit(65536)
	conn.SetReadDeadline(time.Now().Add(120 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(120 * time.Second))
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
				conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
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
			sendWSError(conn, "invalid message format")
			continue
		}

		switch msg.Type {
		case "ping":
			resp := chatWSResponse{Type: "pong", Timestamp: time.Now().Unix()}
			if data, err := json.Marshal(resp); err == nil {
				conn.WriteMessage(websocket.TextMessage, data)
			}

		case "clear":
			sess = &chatSession{}
			resp := chatWSResponse{Type: "system", Content: "Session cleared", Timestamp: time.Now().Unix()}
			if data, err := json.Marshal(resp); err == nil {
				conn.WriteMessage(websocket.TextMessage, data)
			}

		case "message":
			content := strings.TrimSpace(msg.Content)
			if content == "" {
				continue
			}
			model := strings.TrimSpace(msg.Model)
			provider := strings.TrimSpace(msg.Provider)
			fallback := normalizeProviderNames(msg.Fallback)

			// Keep provider/fallback choices in sync with the saved config so restarts preserve them.
			if err := s.persistChatRouting(provider, fallback); err != nil {
				sendWSError(conn, fmt.Sprintf("persist chat routing failed: %v", err))
				continue
			}

			// Add user message to session
			sess.AddMessage(agent.Message{
				Role:    "user",
				Content: content,
			})

			// Process with agent
			response, err := s.agent.ChatWithProviderModelAndFallback(context.Background(), sess, content, provider, model, fallback)
			if err != nil {
				sendWSError(conn, fmt.Sprintf("agent error: %v", err))
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
			}
			if data, err := json.Marshal(resp); err == nil {
				conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
				conn.WriteMessage(websocket.TextMessage, data)
			}
		}
	}
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
	defer conn.Close()

	var writeMu sync.Mutex
	writeJSON := func(v interface{}) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		return conn.WriteJSON(v)
	}

	_ = writeJSON(toolWSResponse{
		Type:      "ready",
		SessionID: sessionID,
		Running:   true,
	})

	conn.SetReadLimit(65536)
	conn.SetReadDeadline(time.Now().Add(120 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(120 * time.Second))
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
				s.tryKillTmuxSession(sessionID)
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
						err = s.processMgr.Write(sessionID, msg.Data)
					}
				}
				if err != nil {
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
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
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

func sendWSError(conn *websocket.Conn, errMsg string) {
	resp := chatWSResponse{
		Type:      "error",
		Content:   errMsg,
		Timestamp: time.Now().Unix(),
	}
	if data, err := json.Marshal(resp); err == nil {
		conn.WriteMessage(websocket.TextMessage, data)
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
		if strings.TrimSpace(p.Name) == name {
			return true
		}
	}
	return false
}

func (s *Server) persistChatRouting(provider string, fallback []string) error {
	changed := false

	if provider != "" {
		if !s.hasProvider(provider) {
			return fmt.Errorf("provider not found: %s", provider)
		}
		if strings.TrimSpace(s.config.Agents.Defaults.Provider) != provider {
			s.config.Agents.Defaults.Provider = provider
			changed = true
		}
	}

	for _, name := range fallback {
		if !s.hasProvider(name) {
			return fmt.Errorf("fallback provider not found: %s", name)
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

// --- Approval Handlers ---

func (s *Server) handleGetApprovals(c *echo.Context) error {
	pending := s.approval.GetPending()
	return c.JSON(http.StatusOK, pending)
}

func (s *Server) handleApproveRequest(c *echo.Context) error {
	id := c.Param("id")
	if err := s.approval.Approve(id); err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "approved", "id": id})
}

func (s *Server) handleDenyRequest(c *echo.Context) error {
	id := c.Param("id")
	var body struct {
		Reason string `json:"reason"`
	}
	_ = c.Bind(&body) // reason is optional

	if err := s.approval.Deny(id, body.Reason); err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "denied", "id": id})
}

// --- Helpers ---

func (s *Server) parseJWTSubject(tokenStr string) (string, error) {
	parsed, err := jwt.Parse(strings.TrimSpace(tokenStr), func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		secret, secretErr := config.GetJWTSecret(s.entClient)
		if secretErr != nil {
			return nil, secretErr
		}
		return []byte(secret), nil
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

func (s *Server) getJWTSecret() string {
	secret, err := config.GetJWTSecret(s.entClient)
	if err == nil && strings.TrimSpace(secret) != "" {
		return secret
	}
	// No credential stored yet — generate an ephemeral secret.
	// It will be replaced once the admin initializes their password.
	return "nekobot-ephemeral-secret"
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
