// Package gateway provides a WebSocket/REST gateway for external clients
// to communicate with the nekobot agent. It runs on the configured gateway
// port and supports authenticated WebSocket connections for real-time chat,
// plus REST endpoints for status and session management.
package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"golang.org/x/time/rate"

	"nekobot/pkg/agent"
	"nekobot/pkg/approval"
	"nekobot/pkg/bus"
	"nekobot/pkg/config"
	"nekobot/pkg/externalagent"
	"nekobot/pkg/inboundrouter"
	"nekobot/pkg/logger"
	"nekobot/pkg/process"
	"nekobot/pkg/runtimeagents"
	"nekobot/pkg/session"
	"nekobot/pkg/storage/ent"
	"nekobot/pkg/tasks"
	"nekobot/pkg/toolsessions"
	"nekobot/pkg/version"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// WSMessage is the JSON format for WebSocket messages.
type WSMessage struct {
	Type      string `json:"type"`                 // "message", "ping", "error", "system"
	Content   string `json:"content"`              // Text content
	SessionID string `json:"session_id,omitempty"` // Conversation session
	MessageID string `json:"message_id,omitempty"` // Unique message ID
	Timestamp int64  `json:"timestamp,omitempty"`  // Unix timestamp
	RuntimeID string `json:"runtime_id,omitempty"` // Explicit runtime selection
}

type websocketRouter interface {
	ChatWebsocket(
		ctx context.Context,
		userID, username, upstreamSessionID, content, runtimeID string,
	) (string, map[string]any, error)
}

// Client represents a connected WebSocket client.
type Client struct {
	id                 string
	conn               *websocket.Conn
	send               chan []byte
	session            agent.SessionInterface
	userID             string
	username           string
	connectedAt        time.Time
	remoteAddr         string
	sessionSource      string
	requestedSessionID string
}

// Server is the WebSocket/REST gateway server.
type Server struct {
	config                    *config.Config
	logger                    *logger.Logger
	agent                     *agent.Agent
	bus                       bus.Bus
	router                    websocketRouter
	externalAgent             *externalagent.Manager
	approval                  *approval.Manager
	toolSess                  *toolsessions.Manager
	processMgr                *process.Manager
	sessionMgr                *session.Manager
	entClient                 *ent.Client
	taskStore                 *tasks.Store
	mux                       *http.ServeMux
	server                    *http.Server
	clients                   map[string]*Client
	reservedPairingSessionIDs map[string]struct{}
	rateLimiters              map[string]*rate.Limiter
	beforeWSUpgrade           func(sessionID string)
	beforeWelcomeSend         func(client *Client)
	mu                        sync.RWMutex
}

type connectionStatus struct {
	ID                 string  `json:"id"`
	UserID             string  `json:"user_id"`
	Username           string  `json:"username"`
	SessionID          *string `json:"session_id"`
	Paired             bool    `json:"paired"`
	PairedID           *string `json:"paired_session_id"`
	SessionSource      *string `json:"session_source,omitempty"`
	RequestedSessionID *string `json:"requested_session_id,omitempty"`
	ConnectedAt        string  `json:"connected_at"`
	RemoteAddr         string  `json:"remote_addr"`
}

type authContext struct {
	userID   string
	username string
	role     string
}

type gatewayControlPlaneScope string

const (
	gatewayControlPlaneScopeRead   gatewayControlPlaneScope = "read"
	gatewayControlPlaneScopeManage gatewayControlPlaneScope = "manage"
)

// NewServer creates a new gateway server.
func NewServer(
	cfg *config.Config,
	log *logger.Logger,
	ag *agent.Agent,
	messageBus bus.Bus,
	router *inboundrouter.Router,
	approvalMgr *approval.Manager,
	toolSessionMgr *toolsessions.Manager,
	processManager *process.Manager,
	sessionMgr *session.Manager,
	entClient *ent.Client,
) *Server {
	s := &Server{
		config:   cfg,
		logger:   log,
		agent:    ag,
		bus:      messageBus,
		router:   router,
		approval: approvalMgr,
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
		toolSess:   toolSessionMgr,
		processMgr: processManager,
		sessionMgr: sessionMgr,
		entClient:  entClient,
		taskStore: func() *tasks.Store {
			if ag == nil {
				return nil
			}
			return ag.TaskStore()
		}(),
		clients:                   make(map[string]*Client),
		reservedPairingSessionIDs: make(map[string]struct{}),
		rateLimiters:              make(map[string]*rate.Limiter),
	}

	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	mux := http.NewServeMux()

	// WebSocket endpoint
	mux.HandleFunc("/ws/chat", s.handleWSChat)

	// REST endpoints
	mux.HandleFunc("GET /api/v1/status", s.handleStatus)
	mux.HandleFunc("GET /api/v1/connections", s.handleConnections)
	mux.HandleFunc("DELETE /api/v1/connections", s.handleDeleteConnections)
	mux.HandleFunc("GET /api/v1/connections/{id}", s.handleConnection)
	mux.HandleFunc("DELETE /api/v1/connections/{id}", s.handleDeleteConnection)
	mux.HandleFunc("POST /api/v1/external-agents/resolve-session", s.handleResolveExternalAgentSession)
	mux.HandleFunc("GET /api/v1/approvals", s.handleGetApprovals)
	mux.HandleFunc("POST /api/v1/approvals/{id}/approve", s.handleApproveRequest)
	mux.HandleFunc("POST /api/v1/approvals/{id}/deny", s.handleDenyRequest)

	// Health check
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"status":"ok"}`)); err != nil {
			s.logger.Warn("Failed to write health response", zap.Error(err))
		}
	})

	// Metrics endpoint (Prometheus-style)
	mux.HandleFunc("GET /metrics", s.handleMetrics)

	s.mux = mux
}

// Start starts the gateway server.
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.config.Gateway.Host, s.config.Gateway.Port)
	s.logger.Info("Gateway server starting",
		zap.String("addr", addr),
	)

	s.server = &http.Server{
		Addr:    addr,
		Handler: s.mux,
	}

	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("Gateway server error", zap.Error(err))
		}
	}()

	return nil
}

// Stop gracefully shuts down the gateway server.
func (s *Server) Stop(ctx context.Context) error {
	s.logger.Info("Gateway server stopping")

	// Close all client connections
	s.mu.Lock()
	for id, client := range s.clients {
		close(client.send)
		if err := client.conn.Close(); err != nil {
			s.logger.Warn("Failed to close gateway websocket", zap.String("client_id", id), zap.Error(err))
		}
		delete(s.clients, id)
	}
	s.mu.Unlock()

	if s.server != nil {
		return s.server.Shutdown(ctx)
	}
	return nil
}

// --- WebSocket Handler ---

func (s *Server) handleWSChat(w http.ResponseWriter, r *http.Request) {
	if err := s.checkClientIP(r); err != nil {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	if err := s.checkRateLimit(r); err != nil {
		http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
		return
	}

	// Authenticate via token query param or Authorization header
	authCtx, err := s.authenticateRequest(r)
	if err != nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	if err := s.checkConnectionLimit(); err != nil {
		http.Error(w, `{"error":"connection limit exceeded"}`, http.StatusServiceUnavailable)
		return
	}

	clientID := uuid.New().String()
	requestedSessionID := strings.TrimSpace(r.URL.Query().Get("session_id"))
	sessionID, err := s.resolveGatewaySessionID(r, clientID)
	if err != nil {
		http.Error(w, `{"error":"invalid session_id"}`, http.StatusBadRequest)
		return
	}
	if s.sessionMgr == nil {
		http.Error(w, `{"error":"session unavailable"}`, http.StatusInternalServerError)
		return
	}
	releasePairingReservation, err := s.reservePairingSessionID(sessionID, requestedSessionID != "")
	if err != nil {
		http.Error(w, `{"error":"session already attached"}`, http.StatusConflict)
		return
	}
	defer releasePairingReservation()
	if s.beforeWSUpgrade != nil {
		s.beforeWSUpgrade(sessionID)
	}

	// Upgrade to WebSocket
	upgrader.CheckOrigin = s.checkOrigin
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("WebSocket upgrade failed", zap.Error(err))
		return
	}
	sess, err := s.getOrCreateSession(sessionID)
	if err != nil {
		s.logger.Error("Create gateway session failed", zap.Error(err))
		http.Error(w, `{"error":"session unavailable"}`, http.StatusInternalServerError)
		return
	}
	client := &Client{
		id:                 clientID,
		conn:               conn,
		send:               make(chan []byte, 256),
		session:            sess,
		userID:             authCtx.userID,
		username:           authCtx.username,
		connectedAt:        time.Now().UTC(),
		remoteAddr:         r.RemoteAddr,
		sessionSource:      classifyGatewaySessionSource(sess, requestedSessionID),
		requestedSessionID: strings.TrimSpace(requestedSessionID),
	}

	s.mu.Lock()
	s.clients[clientID] = client
	s.mu.Unlock()

	s.logger.Info("WebSocket client connected",
		zap.String("client_id", clientID),
		zap.String("session_id", sessionID),
		zap.String("user", authCtx.username),
	)

	// Send welcome message
	if s.beforeWelcomeSend != nil {
		s.beforeWelcomeSend(client)
	}
	welcome := WSMessage{
		Type:      "system",
		Content:   "Connected to nekobot gateway",
		SessionID: sessionID,
		Timestamp: time.Now().Unix(),
	}
	if data, err := json.Marshal(welcome); err == nil {
		client.send <- data
	}

	// Start reader and writer goroutines
	go s.readPump(client)
	go s.writePump(client)
}

func (s *Server) getOrCreateSession(sessionID string) (agent.SessionInterface, error) {
	if s.sessionMgr == nil {
		return nil, fmt.Errorf("session manager not available")
	}
	return s.sessionMgr.GetWithSource(sessionID, session.SourceGateway)
}

func (s *Server) resolveGatewaySessionID(r *http.Request, fallbackSessionID string) (string, error) {
	requestedSessionID := strings.TrimSpace(r.URL.Query().Get("session_id"))
	if requestedSessionID == "" {
		return fallbackSessionID, nil
	}
	if s.sessionMgr == nil {
		return "", fmt.Errorf("session manager not available")
	}
	existing, err := s.sessionMgr.GetExisting(requestedSessionID)
	if err != nil {
		return "", fmt.Errorf("lookup session %q: %w", requestedSessionID, err)
	}
	if existing == nil {
		return "", fmt.Errorf("session %q unavailable", requestedSessionID)
	}
	existingSource := strings.TrimSpace(existing.Source)
	if existingSource != "" && existingSource != session.SourceGateway {
		return "", fmt.Errorf("session %q is not a gateway session", requestedSessionID)
	}
	return requestedSessionID, nil
}

func (s *Server) reservePairingSessionID(sessionID string, requested bool) (func(), error) {
	if !requested || strings.TrimSpace(sessionID) == "" {
		return func() {}, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.reservedPairingSessionIDs == nil {
		s.reservedPairingSessionIDs = make(map[string]struct{})
	}
	if _, exists := s.reservedPairingSessionIDs[sessionID]; exists {
		return nil, fmt.Errorf("session %q already reserved for attach", sessionID)
	}
	for _, client := range s.clients {
		if gatewaySessionID(client) == sessionID {
			return nil, fmt.Errorf("session %q already attached to an active websocket client", sessionID)
		}
	}

	s.reservedPairingSessionIDs[sessionID] = struct{}{}
	return func() {
		s.mu.Lock()
		delete(s.reservedPairingSessionIDs, sessionID)
		s.mu.Unlock()
	}, nil
}

func (s *Server) readPump(client *Client) {
	defer func() {
		s.removeClient(client)
		if err := client.conn.Close(); err != nil {
			s.logger.Warn("Failed to close read websocket", zap.String("client_id", client.id), zap.Error(err))
		}
	}()

	client.conn.SetReadLimit(65536)
	if err := client.conn.SetReadDeadline(time.Now().Add(60 * time.Second)); err != nil {
		s.logger.Warn("Failed to set gateway read deadline", zap.String("client_id", client.id), zap.Error(err))
	}
	client.conn.SetPongHandler(func(string) error {
		if err := client.conn.SetReadDeadline(time.Now().Add(60 * time.Second)); err != nil {
			s.logger.Warn("Failed to refresh gateway read deadline", zap.String("client_id", client.id), zap.Error(err))
		}
		return nil
	})

	for {
		_, message, err := client.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				s.logger.Warn("WebSocket read error",
					zap.String("client_id", client.id),
					zap.Error(err),
				)
			}
			return
		}

		// Parse incoming message
		var wsMsg WSMessage
		if err := json.Unmarshal(message, &wsMsg); err != nil {
			s.sendError(client, "invalid message format")
			continue
		}

		// Handle ping
		if wsMsg.Type == "ping" {
			pong := WSMessage{Type: "pong", Timestamp: time.Now().Unix()}
			if data, err := json.Marshal(pong); err == nil {
				client.send <- data
			}
			continue
		}

		// Handle chat message
		if wsMsg.Type == "message" && wsMsg.Content != "" {
			go s.processMessage(client, wsMsg)
		}
	}
}

func (s *Server) writePump(client *Client) {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		if err := client.conn.Close(); err != nil {
			s.logger.Warn("Failed to close write websocket", zap.String("client_id", client.id), zap.Error(err))
		}
	}()

	for {
		select {
		case message, ok := <-client.send:
			if err := client.conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
				s.logger.Warn("Failed to set gateway write deadline", zap.String("client_id", client.id), zap.Error(err))
			}
			if !ok {
				if err := client.conn.WriteMessage(websocket.CloseMessage, []byte{}); err != nil {
					s.logger.Warn("Failed to write close frame", zap.String("client_id", client.id), zap.Error(err))
				}
				return
			}

			if err := client.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			if err := client.conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
				s.logger.Warn("Failed to set gateway ping deadline", zap.String("client_id", client.id), zap.Error(err))
			}
			if err := client.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (s *Server) processMessage(client *Client, wsMsg WSMessage) {
	activeSessionID := gatewaySessionID(client)
	if msgSessionID := strings.TrimSpace(wsMsg.SessionID); msgSessionID != "" && msgSessionID != activeSessionID {
		s.sendError(client, fmt.Sprintf("session mismatch: active=%s message=%s", activeSessionID, msgSessionID))
		return
	}

	response := ""
	routerHandled := false
	if s.router != nil {
		routerHandled = true
		var err error
		response, _, err = s.router.ChatWebsocket(
			context.Background(),
			client.userID,
			client.username,
			activeSessionID,
			wsMsg.Content,
			wsMsg.RuntimeID,
		)
		if err != nil {
			s.sendError(client, fmt.Sprintf("agent error: %v", err))
			return
		}
	}
	if response == "" && routerHandled {
		s.sendError(client, "agent error: websocket chat returned empty response")
		return
	}
	if response == "" {
		busMsg := &bus.Message{
			ID:        uuid.New().String(),
			ChannelID: "websocket",
			SessionID: activeSessionID,
			UserID:    client.userID,
			Username:  client.username,
			Type:      bus.MessageTypeText,
			Content:   wsMsg.Content,
			Timestamp: time.Now(),
		}
		if err := s.bus.SendInbound(busMsg); err != nil {
			s.logger.Warn("Failed to publish inbound bus message", zap.Error(err))
		}
		var err error
		response, err = s.agent.Chat(context.Background(), client.session, wsMsg.Content)
		if err != nil {
			s.sendError(client, fmt.Sprintf("agent error: %v", err))
			return
		}
	}

	// Persist the user-visible gateway transcript after the turn completes.
	client.session.AddMessage(agent.Message{
		Role:    "user",
		Content: wsMsg.Content,
	})
	client.session.AddMessage(agent.Message{
		Role:    "assistant",
		Content: response,
	})

	// Send response to client
	respMsg := WSMessage{
		Type:      "message",
		Content:   response,
		SessionID: activeSessionID,
		MessageID: uuid.New().String(),
		Timestamp: time.Now().Unix(),
	}
	data, err := json.Marshal(respMsg)
	if err != nil {
		return
	}

	select {
	case client.send <- data:
	default:
		s.removeClient(client)
	}
}

func (s *Server) sendError(client *Client, errMsg string) {
	msg := WSMessage{
		Type:      "error",
		Content:   errMsg,
		Timestamp: time.Now().Unix(),
	}
	data, _ := json.Marshal(msg)
	select {
	case client.send <- data:
	default:
	}
}

func (s *Server) removeClient(client *Client) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.clients[client.id]; ok {
		close(client.send)
		delete(s.clients, client.id)
		s.logger.Info("WebSocket client disconnected",
			zap.String("client_id", client.id),
		)
	}
}

// --- REST Handlers ---

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAuthenticatedAPI(w, r, gatewayControlPlaneScopeManage); !ok {
		return
	}

	s.mu.RLock()
	connCount := len(s.clients)
	pairedCount := 0
	pairedGeneratedCount := 0
	pairedRequestedCount := 0
	pairedLegacyCount := 0
	for _, client := range s.clients {
		conn := describeConnection(client)
		if !conn.Paired {
			continue
		}
		pairedCount++
		source := "generated"
		if conn.SessionSource != nil && strings.TrimSpace(*conn.SessionSource) != "" {
			source = strings.TrimSpace(*conn.SessionSource)
		}
		switch source {
		case "requested":
			pairedRequestedCount++
		case "legacy":
			pairedLegacyCount++
		default:
			pairedGeneratedCount++
		}
	}
	s.mu.RUnlock()

	status := map[string]interface{}{
		"version":                      version.GetVersion(),
		"connections":                  connCount,
		"paired_connections":           pairedCount,
		"unpaired_connections":         connCount - pairedCount,
		"paired_generated_connections": pairedGeneratedCount,
		"paired_requested_connections": pairedRequestedCount,
		"paired_legacy_connections":    pairedLegacyCount,
		"bus_metrics":                  s.bus.GetMetrics(),
		"gateway": map[string]interface{}{
			"host": s.config.Gateway.Host,
			"port": s.config.Gateway.Port,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(status); err != nil {
		s.logger.Warn("Failed to encode gateway status", zap.Error(err))
	}
}

func (s *Server) handleConnections(w http.ResponseWriter, r *http.Request) {
	authCtx, ok := s.requireAuthenticatedAPI(w, r, gatewayControlPlaneScopeRead)
	if !ok {
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	conns := make([]connectionStatus, 0, len(s.clients))
	for _, client := range s.clients {
		if !gatewayControlPlaneCanReadConnection(authCtx, client) {
			continue
		}
		conns = append(conns, describeConnectionForAuth(authCtx, client))
	}
	sort.Slice(conns, func(i, j int) bool {
		return conns[i].ID < conns[j].ID
	})

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(conns); err != nil {
		s.logger.Warn("Failed to encode gateway connections", zap.Error(err))
	}
}

func (s *Server) handleConnection(w http.ResponseWriter, r *http.Request) {
	authCtx, ok := s.requireAuthenticatedAPI(w, r, gatewayControlPlaneScopeRead)
	if !ok {
		return
	}

	clientID := strings.TrimSpace(r.PathValue("id"))
	if clientID == "" {
		http.NotFound(w, r)
		return
	}

	s.mu.RLock()
	client, ok := s.clients[clientID]
	s.mu.RUnlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	if !gatewayControlPlaneCanReadConnection(authCtx, client) {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(describeConnectionForAuth(authCtx, client)); err != nil {
		s.logger.Warn("Failed to encode gateway connection", zap.Error(err))
	}
}

func (s *Server) handleDeleteConnection(w http.ResponseWriter, r *http.Request) {
	authCtx, err := s.authenticateDeleteConnections(r)
	if err != nil {
		s.writeDeleteConnectionsAuthError(w, err)
		return
	}

	clientID := strings.TrimSpace(r.PathValue("id"))
	if clientID == "" {
		http.NotFound(w, r)
		return
	}

	s.mu.RLock()
	client, ok := s.clients[clientID]
	s.mu.RUnlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	if !gatewayControlPlaneCanManageConnection(authCtx, client) {
		if strings.EqualFold(strings.TrimSpace(authCtx.role), "member") {
			http.NotFound(w, r)
			return
		}
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	s.removeClient(client)

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDeleteConnections(w http.ResponseWriter, r *http.Request) {
	authCtx, err := s.authenticateDeleteConnections(r)
	if err != nil {
		s.writeDeleteConnectionsAuthError(w, err)
		return
	}

	s.mu.RLock()
	targets := make([]*Client, 0, len(s.clients))
	for _, client := range s.clients {
		if !gatewayControlPlaneCanManageConnection(authCtx, client) {
			continue
		}
		targets = append(targets, client)
	}
	s.mu.RUnlock()

	for _, client := range targets {
		s.removeClient(client)
	}

	s.mu.RLock()
	remaining := 0
	for _, client := range s.clients {
		if !gatewayControlPlaneCanReadConnection(authCtx, client) {
			continue
		}
		remaining++
	}
	s.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]int{"deleted": len(targets), "remaining": remaining}); err != nil {
		s.logger.Warn("Failed to encode deleted connection response", zap.Error(err))
	}
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAuthenticatedAPI(w, r, gatewayControlPlaneScopeManage); !ok {
		return
	}

	s.mu.RLock()
	totalConns := len(s.clients)
	pairedCount := 0
	pairedGeneratedCount := 0
	pairedRequestedCount := 0
	pairedLegacyCount := 0
	for _, client := range s.clients {
		details := describeConnection(client)
		if !details.Paired {
			continue
		}
		pairedCount++
		source := "generated"
		if details.SessionSource != nil && strings.TrimSpace(*details.SessionSource) != "" {
			source = strings.TrimSpace(*details.SessionSource)
		}
		switch source {
		case "requested":
			pairedRequestedCount++
		case "legacy":
			pairedLegacyCount++
		default:
			pairedGeneratedCount++
		}
	}
	s.mu.RUnlock()

	metrics := map[string]interface{}{
		"gateway_connections_total":            totalConns,
		"gateway_connections_paired":           pairedCount,
		"gateway_connections_unpaired":         totalConns - pairedCount,
		"gateway_connections_paired_generated": pairedGeneratedCount,
		"gateway_connections_paired_requested": pairedRequestedCount,
		"gateway_connections_paired_legacy":    pairedLegacyCount,
		"gateway_rate_limit_per_minute":        s.config.Gateway.RateLimitPerMinute,
		"gateway_max_connections":              s.config.Gateway.MaxConnections,
		"gateway_allowed_origins_count":        len(s.config.Gateway.AllowedOrigins),
		"gateway_allowed_ips_count":            len(s.config.Gateway.AllowedIPs),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(metrics); err != nil {
		s.logger.Warn("Failed to encode gateway metrics", zap.Error(err))
	}
}

func (s *Server) handleResolveExternalAgentSession(w http.ResponseWriter, r *http.Request) {
	if s.externalAgent == nil {
		http.Error(w, `{"error":"external agent manager not available"}`, http.StatusServiceUnavailable)
		return
	}

	authCtx, err := s.authenticateDeleteConnections(r)
	if err != nil {
		s.writeDeleteConnectionsAuthError(w, err)
		return
	}

	var body struct {
		AgentKind string `json:"agent_kind"`
		Workspace string `json:"workspace"`
		Tool      string `json:"tool"`
		Title     string `json:"title"`
		Command   string `json:"command"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	var probe gatewayExternalAgentProcessProbe
	var starter externalagent.ProcessStarter
	if s.processMgr != nil {
		probe.mgr = s.processMgr
		starter = s.processMgr
	}
	result, err := externalagent.ExecuteResolveFlow(
		r.Context(),
		s.externalAgent,
		externalagent.NewResolveOrchestrator(s.config, s.logger, s.entClient, s.approval, s.taskStore),
		s.config.WorkspacePath(),
		probe,
		starter,
		s.toolSess,
		runtimeagents.DefaultTransport(),
		externalagent.SessionSpec{
			Owner:     authCtx.username,
			AgentKind: body.AgentKind,
			Workspace: body.Workspace,
			Tool:      body.Tool,
			Title:     body.Title,
			Command:   body.Command,
		},
	)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(result.HTTPStatus())
	if err := json.NewEncoder(w).Encode(result.ResponseBody()); err != nil {
		s.logger.Warn("Failed to encode external agent session", zap.Error(err))
	}
}

func (s *Server) ensureExternalAgentProcess(ctx context.Context, sess *toolsessions.Session) error {
	if s == nil {
		return nil
	}
	var probe gatewayExternalAgentProcessProbe
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

func (s *Server) handleGetApprovals(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAuthenticatedAPI(w, r, gatewayControlPlaneScopeManage); !ok {
		return
	}
	if s.approval == nil {
		http.Error(w, `{"error":"approval manager not available"}`, http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(s.approval.GetPending()); err != nil {
		s.logger.Warn("Failed to encode approvals response", zap.Error(err))
	}
}

func (s *Server) handleApproveRequest(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAuthenticatedAPI(w, r, gatewayControlPlaneScopeManage); !ok {
		return
	}
	if s.approval == nil {
		http.Error(w, `{"error":"approval manager not available"}`, http.StatusServiceUnavailable)
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	req, _ := s.approval.GetRequest(id)
	if err := s.approval.Approve(id); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusNotFound)
		return
	}
	if req != nil && isGatewayExternalAgentApprovalRequest(req) {
		s.approval.SetSessionMode(req.SessionID, approval.ModeAuto)
		if s.toolSess != nil {
			sess, err := s.toolSess.GetSession(r.Context(), req.SessionID)
			if err == nil && sess != nil {
				if err := s.ensureExternalAgentProcess(r.Context(), sess); err != nil {
					http.Error(w, fmt.Sprintf(`{"error":%q}`, "failed to continue external agent launch: "+err.Error()), http.StatusBadRequest)
					return
				}
			}
		}
	}
	if req != nil {
		if pendingToolCall, ok := approval.PendingToolCallForRequest(id); ok {
			if s.agent != nil {
				s.approval.SetSessionMode(req.SessionID, approval.ModeAuto)
				if _, err := s.agent.ReplayApprovedToolCall(r.Context(), pendingToolCall.SessionID, pendingToolCall.Call); err != nil {
					http.Error(w, fmt.Sprintf(`{"error":%q}`, "failed to replay approved tool call: "+err.Error()), http.StatusBadRequest)
					return
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
		s.taskStore.SetSessionLifecycleState(req.SessionID, tasks.SessionLifecycleIdle, "")
	}
	body := map[string]any{"status": "approved", "id": id}
	if req != nil {
		if state := s.currentSessionRuntimeState(req.SessionID); state != nil {
			body["session_runtime_state"] = state
		}
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(body); err != nil {
		s.logger.Warn("Failed to encode approval response", zap.Error(err))
	}
}

func (s *Server) handleDenyRequest(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAuthenticatedAPI(w, r, gatewayControlPlaneScopeManage); !ok {
		return
	}
	if s.approval == nil {
		http.Error(w, `{"error":"approval manager not available"}`, http.StatusServiceUnavailable)
		return
	}
	var body struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		http.Error(w, fmt.Sprintf(`{"error":%q}`, "invalid request"), http.StatusBadRequest)
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	req, _ := s.approval.GetRequest(id)
	if err := s.approval.Deny(id, body.Reason); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusNotFound)
		return
	}
	approval.ClearPendingToolCall(id)
	if req != nil && s.taskStore != nil {
		s.taskStore.ClearSessionPendingAction(req.SessionID)
		s.taskStore.SetSessionLifecycleState(req.SessionID, tasks.SessionLifecycleIdle, "")
	}
	response := map[string]any{"status": "denied", "id": id}
	if req != nil {
		if state := s.currentSessionRuntimeState(req.SessionID); state != nil {
			response["session_runtime_state"] = state
		}
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Warn("Failed to encode deny response", zap.Error(err))
	}
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

func isGatewayExternalAgentApprovalRequest(req *approval.Request) bool {
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

type gatewayExternalAgentProcessProbe struct {
	mgr *process.Manager
}

func (p gatewayExternalAgentProcessProbe) HasProcess(sessionID string) bool {
	if p.mgr == nil {
		return false
	}
	_, err := p.mgr.GetStatus(sessionID)
	return err == nil
}

func (s *Server) authenticateDeleteConnections(r *http.Request) (*authContext, error) {
	if err := s.checkClientIP(r); err != nil {
		return nil, fmt.Errorf("forbidden")
	}
	if err := s.checkRateLimit(r); err != nil {
		return nil, fmt.Errorf("rate_limit")
	}
	authCtx, err := s.authenticateRequest(r)
	if err != nil {
		return nil, fmt.Errorf("unauthorized")
	}
	role := strings.ToLower(strings.TrimSpace(authCtx.role))
	if role == "member" || role == "admin" || role == "owner" {
		return authCtx, nil
	}
	return nil, fmt.Errorf("forbidden")
}

func (s *Server) writeDeleteConnectionsAuthError(w http.ResponseWriter, err error) {
	switch err.Error() {
	case "unauthorized":
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
	case "rate_limit":
		http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
	default:
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
	}
}

func (s *Server) requireAuthenticatedAPI(
	w http.ResponseWriter,
	r *http.Request,
	scope gatewayControlPlaneScope,
) (*authContext, bool) {
	if err := s.checkClientIP(r); err != nil {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return nil, false
	}
	if err := s.checkRateLimit(r); err != nil {
		http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
		return nil, false
	}

	authCtx, err := s.authenticateRequest(r)
	if err != nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return nil, false
	}
	if !isGatewayControlPlaneRoleAllowed(authCtx.role, scope) {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return nil, false
	}
	return authCtx, true
}

func (s *Server) checkClientIP(r *http.Request) error {
	if s == nil || s.config == nil || len(s.config.Gateway.AllowedIPs) == 0 {
		return nil
	}

	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil {
		return fmt.Errorf("parse remote addr: %w", err)
	}
	if net.ParseIP(host) == nil {
		return fmt.Errorf("remote addr %q does not contain a valid ip", r.RemoteAddr)
	}

	for _, allowedIP := range s.config.Gateway.AllowedIPs {
		if strings.TrimSpace(allowedIP) == host {
			return nil
		}
	}

	return fmt.Errorf("ip %s not allowed", host)
}

func (s *Server) checkConnectionLimit() error {
	if s == nil || s.config == nil || s.config.Gateway.MaxConnections == 0 {
		return nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.clients) >= s.config.Gateway.MaxConnections {
		return fmt.Errorf("gateway connection limit exceeded")
	}
	return nil
}

func (s *Server) checkRateLimit(r *http.Request) error {
	if s == nil || s.config == nil || s.config.Gateway.RateLimitPerMinute == 0 {
		return nil
	}

	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil {
		return fmt.Errorf("parse remote addr for rate limit: %w", err)
	}
	if net.ParseIP(host) == nil {
		return fmt.Errorf("remote addr %q does not contain a valid ip", r.RemoteAddr)
	}

	limiter := s.getOrCreateRateLimiter(host)
	if limiter.Allow() {
		return nil
	}

	return fmt.Errorf("rate limit exceeded for ip %s", host)
}

func (s *Server) getOrCreateRateLimiter(host string) *rate.Limiter {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.rateLimiters == nil {
		s.rateLimiters = make(map[string]*rate.Limiter)
	}

	if limiter, ok := s.rateLimiters[host]; ok {
		return limiter
	}

	limitPerMinute := s.config.Gateway.RateLimitPerMinute
	limiter := rate.NewLimiter(rate.Every(time.Minute/time.Duration(limitPerMinute)), limitPerMinute)
	s.rateLimiters[host] = limiter
	return limiter
}

func describeConnection(client *Client) connectionStatus {
	var sessionID *string
	paired := false
	var pairedID *string
	var sessionSource *string
	var requestedSessionID *string
	if identifiable, ok := client.session.(interface{ GetID() string }); ok {
		id := strings.TrimSpace(identifiable.GetID())
		if id != "" {
			sessionID = &id
			paired = true
			pairedID = &id
		}
	}
	if value := strings.TrimSpace(client.sessionSource); value != "" {
		sessionSource = &value
	} else if paired {
		value := inferGatewaySessionSource(client)
		if value != "" {
			sessionSource = &value
		}
	}
	if value := strings.TrimSpace(client.requestedSessionID); value != "" {
		requestedSessionID = &value
	}

	return connectionStatus{
		ID:                 client.id,
		UserID:             client.userID,
		Username:           client.username,
		SessionID:          sessionID,
		Paired:             paired,
		PairedID:           pairedID,
		SessionSource:      sessionSource,
		RequestedSessionID: requestedSessionID,
		ConnectedAt:        client.connectedAt.UTC().Format(time.RFC3339),
		RemoteAddr:         client.remoteAddr,
	}
}

func describeConnectionForAuth(authCtx *authContext, client *Client) connectionStatus {
	conn := describeConnection(client)
	if authCtx == nil {
		return conn
	}
	if strings.EqualFold(strings.TrimSpace(authCtx.role), "member") {
		conn.RemoteAddr = ""
		conn.SessionSource = nil
		conn.RequestedSessionID = nil
	}
	return conn
}

func classifyGatewaySessionSource(sess agent.SessionInterface, requestedSessionID string) string {
	if strings.TrimSpace(requestedSessionID) == "" {
		if sess != nil {
			return "generated"
		}
		return ""
	}
	if managed, ok := sess.(*session.Session); ok && strings.TrimSpace(managed.Source) == "" {
		return "legacy"
	}
	return "requested"
}

func inferGatewaySessionSource(client *Client) string {
	if client == nil || client.session == nil {
		return ""
	}
	if managed, ok := client.session.(*session.Session); ok && strings.TrimSpace(managed.Source) == "" {
		return "legacy"
	}
	if gatewaySessionID(client) != strings.TrimSpace(client.id) {
		return "requested"
	}
	return "generated"
}

func gatewaySessionID(client *Client) string {
	if client == nil {
		return ""
	}
	if identifiable, ok := client.session.(interface{ GetID() string }); ok {
		id := strings.TrimSpace(identifiable.GetID())
		if id != "" {
			return id
		}
	}
	return client.id
}

// --- Auth ---

func (s *Server) authenticateRequest(r *http.Request) (*authContext, error) {
	// Try token from query parameter
	token := r.URL.Query().Get("token")
	if token == "" {
		// Try Authorization header
		auth := r.Header.Get("Authorization")
		if len(auth) > 7 && auth[:7] == "Bearer " {
			token = auth[7:]
		}
	}

	if token == "" {
		return nil, fmt.Errorf("no token provided")
	}

	// Validate JWT using auth secret from database.
	secret, secretErr := config.GetJWTSecret(s.entClient)
	if secretErr != nil {
		return nil, fmt.Errorf("server not initialized")
	}

	parsed, err := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok || !parsed.Valid {
		return nil, fmt.Errorf("invalid claims")
	}

	sub, _ := claims["sub"].(string)
	username := strings.TrimSpace(sub)
	if username == "" {
		username = "anonymous"
	}

	uid, _ := claims["uid"].(string)
	userID := strings.TrimSpace(uid)
	if userID == "" {
		userID = username
	}

	role, _ := claims["role"].(string)
	role = strings.TrimSpace(role)
	if role == "" {
		role = "admin"
	}

	return &authContext{
		userID:   userID,
		username: username,
		role:     role,
	}, nil
}

func isGatewayControlPlaneRoleAllowed(role string, scope gatewayControlPlaneScope) bool {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "admin", "owner":
		return true
	case "member":
		return scope == gatewayControlPlaneScopeRead
	default:
		return false
	}
}

func gatewayControlPlaneCanReadConnection(authCtx *authContext, client *Client) bool {
	if authCtx == nil || client == nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(authCtx.role)) {
	case "admin", "owner":
		return true
	case "member":
		return strings.TrimSpace(authCtx.userID) != "" && strings.TrimSpace(authCtx.userID) == strings.TrimSpace(client.userID)
	default:
		return false
	}
}

func gatewayControlPlaneCanManageConnection(authCtx *authContext, client *Client) bool {
	if authCtx == nil || client == nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(authCtx.role)) {
	case "admin", "owner":
		return true
	case "member":
		return strings.TrimSpace(authCtx.userID) != "" && strings.TrimSpace(authCtx.userID) == strings.TrimSpace(client.userID)
	default:
		return false
	}
}

func (s *Server) checkOrigin(r *http.Request) bool {
	if s == nil || s.config == nil {
		return true
	}

	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}

	allowedOrigins := s.config.Gateway.AllowedOrigins
	if len(allowedOrigins) == 0 {
		return true
	}

	for _, allowed := range allowedOrigins {
		if strings.TrimSpace(allowed) == origin {
			return true
		}
	}

	return false
}
