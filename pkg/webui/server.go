// Package webui provides a web-based dashboard for nekobot.
// It uses Echo v5 for HTTP routing with JWT authentication,
// and serves an embedded SPA frontend for configuration management
// and chat playground.
package webui

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"sort"
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
	"nekobot/pkg/providers"
	"nekobot/pkg/userprefs"
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
	port       int
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
) *Server {
	port := cfg.WebUI.Port
	if port == 0 {
		port = cfg.Gateway.Port + 1
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

	// Chat WebSocket (auth handled inside via token query param)
	e.GET("/api/chat/ws", s.handleChatWS)

	// Protected API routes
	api := e.Group("/api")
	secret := s.getJWTSecret()
	api.Use(echojwt.WithConfig(echojwt.Config{
		SigningKey: []byte(secret),
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

	// Persist config to file
	if err := s.persistConfig(); err != nil {
		s.logger.Warn("Failed to persist init config", zap.Error(err))
	}

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
	// Return provider profiles for dashboard editing.
	providers := make([]map[string]interface{}, len(s.config.Providers))
	for i, p := range s.config.Providers {
		providers[i] = map[string]interface{}{
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
	return c.JSON(http.StatusOK, providers)
}

func (s *Server) handleCreateProvider(c *echo.Context) error {
	var profile config.ProviderProfile
	if err := c.Bind(&profile); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	s.config.Providers = append(s.config.Providers, profile)

	if err := s.persistConfig(); err != nil {
		s.logger.Error("Failed to persist provider config", zap.Error(err))
	}

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

			if err := s.persistConfig(); err != nil {
				s.logger.Error("Failed to persist provider config", zap.Error(err))
			}

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

			if err := s.persistConfig(); err != nil {
				s.logger.Error("Failed to persist provider config", zap.Error(err))
			}

			return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
		}
	}

	return c.JSON(http.StatusNotFound, map[string]string{"error": "provider not found"})
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

	// Persist
	if err := s.persistConfig(); err != nil {
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

	// Validate
	if err := config.ValidateConfig(s.config); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	// Persist to file
	if err := s.persistConfig(); err != nil {
		s.logger.Error("Failed to persist config", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to save config"})
	}

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
	Type    string `json:"type"`    // "message", "ping", "clear"
	Content string `json:"content"` // User message text
	Model   string `json:"model"`   // Optional model override
}

type chatWSResponse struct {
	Type      string `json:"type"`                // "message", "thinking", "error", "system", "pong"
	Content   string `json:"content"`             // Response text
	Thinking  string `json:"thinking,omitempty"`  // Model's thinking (if extended thinking enabled)
	Timestamp int64  `json:"timestamp,omitempty"` // Unix timestamp
}

func (s *Server) handleChatWS(c *echo.Context) error {
	// Authenticate via token query param (since WebSocket can't use Authorization header easily)
	tokenStr := c.QueryParam("token")
	if tokenStr == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "token required"})
	}

	secret := s.getJWTSecret()
	parsed, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil || !parsed.Valid {
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

			// Add user message to session
			sess.AddMessage(agent.Message{
				Role:    "user",
				Content: content,
			})

			// Process with agent
			response, err := s.agent.ChatWithModel(context.Background(), sess, content, model)
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

// persistConfig saves the current config to the file it was loaded from.
func (s *Server) persistConfig() error {
	configPath := s.loader.GetConfigPath()
	if configPath == "" {
		// No config file loaded yet, save to default location
		home, err := config.GetConfigHome()
		if err != nil {
			return fmt.Errorf("getting config home: %w", err)
		}
		configPath = home + "/config.json"
	}
	return s.loader.Save(configPath, s.config)
}
