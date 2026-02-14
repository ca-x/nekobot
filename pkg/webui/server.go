// Package webui provides a web-based dashboard for nekobot.
// It uses Echo v5 for HTTP routing with JWT authentication,
// and serves an embedded SPA frontend for configuration management
// and chat playground.
package webui

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io/fs"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	echojwt "github.com/labstack/echo-jwt/v5"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"go.uber.org/zap"

	"nekobot/pkg/approval"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/webui/frontend"
)

// Server is the WebUI HTTP server.
type Server struct {
	echo       *echo.Echo
	httpServer *http.Server
	config     *config.Config
	logger     *logger.Logger
	approval   *approval.Manager
	port       int
}

// NewServer creates a new WebUI server.
func NewServer(cfg *config.Config, log *logger.Logger, approvalMgr *approval.Manager) *Server {
	port := cfg.WebUI.Port
	if port == 0 {
		port = cfg.Gateway.Port + 1
	}

	s := &Server{
		config:   cfg,
		logger:   log,
		approval: approvalMgr,
		port:     port,
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

	// Protected API routes
	api := e.Group("/api")
	secret := s.getJWTSecret()
	api.Use(echojwt.WithConfig(echojwt.Config{
		SigningKey: []byte(secret),
	}))

	// Provider routes
	api.GET("/providers", s.handleGetProviders)
	api.POST("/providers", s.handleCreateProvider)
	api.PUT("/providers/:name", s.handleUpdateProvider)
	api.DELETE("/providers/:name", s.handleDeleteProvider)

	// Channel routes
	api.GET("/channels", s.handleGetChannels)
	api.PUT("/channels/:name", s.handleUpdateChannel)
	api.POST("/channels/:name/test", s.handleTestChannel)

	// Config routes
	api.GET("/config", s.handleGetConfig)
	api.PUT("/config", s.handleSaveConfig)

	// Chat playground (WebSocket)
	api.GET("/chat/ws", s.handleChatWS)

	// Status
	api.GET("/status", s.handleStatus)

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
	initialized := s.config.WebUI.Password != ""
	return c.JSON(http.StatusOK, map[string]bool{"initialized": initialized})
}

func (s *Server) handleInitPassword(c *echo.Context) error {
	if s.config.WebUI.Password != "" {
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

	// Set credentials (in production, hash the password)
	if body.Username != "" {
		s.config.WebUI.Username = body.Username
	}
	s.config.WebUI.Password = body.Password

	// Generate JWT secret if not set
	if s.config.WebUI.Secret == "" {
		s.config.WebUI.Secret = generateSecret()
	}

	// TODO: persist config to file

	token, err := s.generateToken(s.config.WebUI.Username)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "token generation failed"})
	}

	return c.JSON(http.StatusOK, map[string]string{"token": token})
}

func (s *Server) handleLogin(c *echo.Context) error {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	if body.Username != s.config.WebUI.Username || body.Password != s.config.WebUI.Password {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
	}

	token, err := s.generateToken(body.Username)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "token generation failed"})
	}

	return c.JSON(http.StatusOK, map[string]string{"token": token})
}

// --- Provider Handlers ---

func (s *Server) handleGetProviders(c *echo.Context) error {
	// Return providers with API keys masked
	providers := make([]map[string]interface{}, len(s.config.Providers))
	for i, p := range s.config.Providers {
		providers[i] = map[string]interface{}{
			"name":          p.Name,
			"provider_kind": p.ProviderKind,
			"api_base":      p.APIBase,
			"proxy":         p.Proxy,
			"models":        p.Models,
			"default_model": p.DefaultModel,
			"timeout":       p.Timeout,
			"has_api_key":   p.APIKey != "",
		}
	}
	return c.JSON(http.StatusOK, providers)
}

func (s *Server) handleCreateProvider(c *echo.Context) error {
	var profile config.ProviderProfile
	if err := c.Bind(&profile); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	s.config.Providers = append(s.config.Providers, profile)
	return c.JSON(http.StatusCreated, map[string]string{"status": "created"})
}

func (s *Server) handleUpdateProvider(c *echo.Context) error {
	name := c.Param("name")
	var profile config.ProviderProfile
	if err := c.Bind(&profile); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	for i, p := range s.config.Providers {
		if p.Name == name {
			// Preserve API key if not provided in update
			if profile.APIKey == "" {
				profile.APIKey = p.APIKey
			}
			s.config.Providers[i] = profile
			return c.JSON(http.StatusOK, map[string]string{"status": "updated"})
		}
	}

	return c.JSON(http.StatusNotFound, map[string]string{"error": "provider not found"})
}

func (s *Server) handleDeleteProvider(c *echo.Context) error {
	name := c.Param("name")

	for i, p := range s.config.Providers {
		if p.Name == name {
			s.config.Providers = append(s.config.Providers[:i], s.config.Providers[i+1:]...)
			return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
		}
	}

	return c.JSON(http.StatusNotFound, map[string]string{"error": "provider not found"})
}

// --- Channel Handlers ---

func (s *Server) handleGetChannels(c *echo.Context) error {
	// Return channel configs (with secrets masked)
	channels := map[string]interface{}{
		"telegram":   map[string]interface{}{"enabled": s.config.Channels.Telegram.Enabled, "has_token": s.config.Channels.Telegram.Token != ""},
		"discord":    map[string]interface{}{"enabled": s.config.Channels.Discord.Enabled, "has_token": s.config.Channels.Discord.Token != ""},
		"slack":      map[string]interface{}{"enabled": s.config.Channels.Slack.Enabled, "has_token": s.config.Channels.Slack.BotToken != ""},
		"whatsapp":   map[string]interface{}{"enabled": s.config.Channels.WhatsApp.Enabled, "bridge_url": s.config.Channels.WhatsApp.BridgeURL},
		"feishu":     map[string]interface{}{"enabled": s.config.Channels.Feishu.Enabled, "has_app_id": s.config.Channels.Feishu.AppID != ""},
		"dingtalk":   map[string]interface{}{"enabled": s.config.Channels.DingTalk.Enabled, "has_client_id": s.config.Channels.DingTalk.ClientID != ""},
		"qq":         map[string]interface{}{"enabled": s.config.Channels.QQ.Enabled, "has_app_id": s.config.Channels.QQ.AppID != ""},
		"wework":     map[string]interface{}{"enabled": s.config.Channels.WeWork.Enabled, "has_corp_id": s.config.Channels.WeWork.CorpID != ""},
		"serverchan": map[string]interface{}{"enabled": s.config.Channels.ServerChan.Enabled},
		"googlechat": map[string]interface{}{"enabled": s.config.Channels.GoogleChat.Enabled},
		"maixcam":    map[string]interface{}{"enabled": s.config.Channels.MaixCam.Enabled, "host": s.config.Channels.MaixCam.Host, "port": s.config.Channels.MaixCam.Port},
	}
	return c.JSON(http.StatusOK, channels)
}

func (s *Server) handleUpdateChannel(c *echo.Context) error {
	// Channel update will be implemented with reflection or a switch on channel name
	name := c.Param("name")
	return c.JSON(http.StatusOK, map[string]string{"status": "updated", "channel": name})
}

func (s *Server) handleTestChannel(c *echo.Context) error {
	name := c.Param("name")
	// TODO: implement channel connectivity test
	return c.JSON(http.StatusOK, map[string]interface{}{
		"channel": name,
		"status":  "test not yet implemented",
	})
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
	})
}

func (s *Server) handleSaveConfig(c *echo.Context) error {
	// TODO: persist config changes to disk
	return c.JSON(http.StatusOK, map[string]string{"status": "saved"})
}

// --- Status Handler ---

func (s *Server) handleStatus(c *echo.Context) error {
	return c.JSON(http.StatusOK, map[string]interface{}{
		"version":        "0.11.0-alpha",
		"uptime":         time.Since(time.Now()).String(), // placeholder
		"provider_count": len(s.config.Providers),
		"gateway": map[string]interface{}{
			"host": s.config.Gateway.Host,
			"port": s.config.Gateway.Port,
		},
	})
}

// --- Chat WebSocket (placeholder) ---

func (s *Server) handleChatWS(c *echo.Context) error {
	// TODO: implement WebSocket chat playground
	return c.JSON(http.StatusNotImplemented, map[string]string{"error": "chat WebSocket not yet implemented"})
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

func (s *Server) getJWTSecret() string {
	if s.config.WebUI.Secret != "" {
		return s.config.WebUI.Secret
	}
	// Generate a temporary secret (will be lost on restart if not persisted)
	secret := generateSecret()
	s.config.WebUI.Secret = secret
	return secret
}

func (s *Server) generateToken(username string) (string, error) {
	claims := jwt.MapClaims{
		"sub": username,
		"exp": time.Now().Add(24 * time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.getJWTSecret()))
}

func generateSecret() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
