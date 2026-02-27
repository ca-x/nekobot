// Package gateway provides a WebSocket/REST gateway for external clients
// to communicate with the nekobot agent. It runs on the configured gateway
// port and supports authenticated WebSocket connections for real-time chat,
// plus REST endpoints for status and session management.
package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"nekobot/pkg/agent"
	"nekobot/pkg/bus"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/storage/ent"
	"nekobot/pkg/version"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// WSMessage is the JSON format for WebSocket messages.
type WSMessage struct {
	Type      string `json:"type"`                 // "message", "ping", "error", "system"
	Content   string `json:"content"`              // Text content
	SessionID string `json:"session_id,omitempty"` // Conversation session
	MessageID string `json:"message_id,omitempty"` // Unique message ID
	Timestamp int64  `json:"timestamp,omitempty"`  // Unix timestamp
}

// Client represents a connected WebSocket client.
type Client struct {
	id       string
	conn     *websocket.Conn
	send     chan []byte
	session  *simpleSession
	userID   string
	username string
}

// simpleSession implements agent.SessionInterface for WS clients.
type simpleSession struct {
	messages []agent.Message
	mu       sync.RWMutex
}

func (s *simpleSession) GetMessages() []agent.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.messages
}

func (s *simpleSession) AddMessage(msg agent.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, msg)
}

// Server is the WebSocket/REST gateway server.
type Server struct {
	config    *config.Config
	logger    *logger.Logger
	agent     *agent.Agent
	bus       bus.Bus
	entClient *ent.Client
	mux       *http.ServeMux
	server    *http.Server
	clients   map[string]*Client
	mu        sync.RWMutex
}

// NewServer creates a new gateway server.
func NewServer(cfg *config.Config, log *logger.Logger, ag *agent.Agent, messageBus bus.Bus, entClient *ent.Client) *Server {
	s := &Server{
		config:    cfg,
		logger:    log,
		agent:     ag,
		bus:       messageBus,
		entClient: entClient,
		clients:   make(map[string]*Client),
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

	// Health check
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

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
		client.conn.Close()
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
	// Authenticate via token query param or Authorization header
	userID, username, err := s.authenticateWS(r)
	if err != nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	// Upgrade to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("WebSocket upgrade failed", zap.Error(err))
		return
	}

	clientID := uuid.New().String()
	client := &Client{
		id:       clientID,
		conn:     conn,
		send:     make(chan []byte, 256),
		session:  &simpleSession{},
		userID:   userID,
		username: username,
	}

	s.mu.Lock()
	s.clients[clientID] = client
	s.mu.Unlock()

	s.logger.Info("WebSocket client connected",
		zap.String("client_id", clientID),
		zap.String("user", username),
	)

	// Send welcome message
	welcome := WSMessage{
		Type:      "system",
		Content:   "Connected to nekobot gateway",
		SessionID: clientID,
		Timestamp: time.Now().Unix(),
	}
	if data, err := json.Marshal(welcome); err == nil {
		client.send <- data
	}

	// Start reader and writer goroutines
	go s.readPump(client)
	go s.writePump(client)
}

func (s *Server) readPump(client *Client) {
	defer func() {
		s.removeClient(client)
		client.conn.Close()
	}()

	client.conn.SetReadLimit(65536)
	client.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	client.conn.SetPongHandler(func(string) error {
		client.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
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
		client.conn.Close()
	}()

	for {
		select {
		case message, ok := <-client.send:
			client.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				client.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := client.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			client.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := client.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (s *Server) processMessage(client *Client, wsMsg WSMessage) {
	// Add user message to session
	client.session.AddMessage(agent.Message{
		Role:    "user",
		Content: wsMsg.Content,
	})

	// Also send via bus for logging/routing
	busMsg := &bus.Message{
		ID:        uuid.New().String(),
		ChannelID: "websocket",
		SessionID: client.id,
		UserID:    client.userID,
		Username:  client.username,
		Type:      bus.MessageTypeText,
		Content:   wsMsg.Content,
		Timestamp: time.Now(),
	}
	_ = s.bus.SendInbound(busMsg)

	// Process with agent
	response, err := s.agent.Chat(context.Background(), client.session, wsMsg.Content)
	if err != nil {
		s.sendError(client, fmt.Sprintf("agent error: %v", err))
		return
	}

	// Add assistant response to session
	client.session.AddMessage(agent.Message{
		Role:    "assistant",
		Content: response,
	})

	// Send response to client
	respMsg := WSMessage{
		Type:      "message",
		Content:   response,
		SessionID: client.id,
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
	s.mu.RLock()
	connCount := len(s.clients)
	s.mu.RUnlock()

	status := map[string]interface{}{
		"version":     version.GetVersion(),
		"connections": connCount,
		"bus_metrics": s.bus.GetMetrics(),
		"gateway": map[string]interface{}{
			"host": s.config.Gateway.Host,
			"port": s.config.Gateway.Port,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func (s *Server) handleConnections(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	conns := make([]map[string]string, 0, len(s.clients))
	for _, client := range s.clients {
		conns = append(conns, map[string]string{
			"id":       client.id,
			"user_id":  client.userID,
			"username": client.username,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(conns)
}

// --- Auth ---

func (s *Server) authenticateWS(r *http.Request) (userID, username string, err error) {
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
		return "", "", fmt.Errorf("no token provided")
	}

	// Validate JWT using auth secret from database.
	secret, secretErr := config.GetJWTSecret(s.entClient)
	if secretErr != nil {
		return "", "", fmt.Errorf("server not initialized")
	}

	parsed, err := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return "", "", fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok || !parsed.Valid {
		return "", "", fmt.Errorf("invalid claims")
	}

	sub, _ := claims["sub"].(string)
	if sub == "" {
		sub = "anonymous"
	}

	return sub, sub, nil
}
