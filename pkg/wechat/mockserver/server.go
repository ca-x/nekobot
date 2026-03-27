package mockserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"

	wxtypes "nekobot/pkg/wechat/types"
)

// Server exposes SDK-compatible mock iLink endpoints backed by an Engine.
type Server struct {
	engine *Engine
	server *httptest.Server
	mux    *http.ServeMux
}

// NewServer creates and starts a new mock iLink server.
func NewServer() *Server {
	s := &Server{
		engine: NewEngine(),
		mux:    http.NewServeMux(),
	}

	s.mux.HandleFunc("/ilink/bot/get_bot_qrcode", s.handleGetBotQRCode)
	s.mux.HandleFunc("/ilink/bot/get_qrcode_status", s.handleGetQRCodeStatus)
	s.mux.HandleFunc("/ilink/bot/getupdates", s.handleGetUpdates)
	s.mux.HandleFunc("/ilink/bot/sendmessage", s.handleSendMessage)
	s.mux.HandleFunc("/ilink/bot/getconfig", s.handleGetConfig)
	s.mux.HandleFunc("/ilink/bot/sendtyping", s.handleSendTyping)

	s.server = httptest.NewServer(s.mux)
	return s
}

// URL returns the mock server base URL.
func (s *Server) URL() string {
	return s.server.URL
}

// Engine returns the underlying in-memory engine.
func (s *Server) Engine() *Engine {
	return s.engine
}

// Close shuts down the mock server.
func (s *Server) Close() {
	s.server.Close()
}

func (s *Server) handleGetBotQRCode(w http.ResponseWriter, r *http.Request) {
	if err := requireMethod(r, http.MethodGet); err != nil {
		http.Error(w, err.Error(), http.StatusMethodNotAllowed)
		return
	}

	writeJSON(w, http.StatusOK, s.engine.FetchQRCode())
}

func (s *Server) handleGetQRCodeStatus(w http.ResponseWriter, r *http.Request) {
	if err := requireMethod(r, http.MethodGet); err != nil {
		http.Error(w, err.Error(), http.StatusMethodNotAllowed)
		return
	}

	writeJSON(w, http.StatusOK, s.engine.CheckQRStatus(r.URL.Query().Get("qrcode")))
}

func (s *Server) handleGetUpdates(w http.ResponseWriter, r *http.Request) {
	if err := requireMethod(r, http.MethodPost); err != nil {
		http.Error(w, err.Error(), http.StatusMethodNotAllowed)
		return
	}

	var req wxtypes.GetUpdatesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("decode getupdates request: %v", err), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, s.engine.GetUpdates(req.GetUpdatesBuf))
}

func (s *Server) handleSendMessage(w http.ResponseWriter, r *http.Request) {
	if err := requireMethod(r, http.MethodPost); err != nil {
		http.Error(w, err.Error(), http.StatusMethodNotAllowed)
		return
	}

	var req wxtypes.SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("decode sendmessage request: %v", err), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, s.engine.SendMessage(&req))
}

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	if err := requireMethod(r, http.MethodPost); err != nil {
		http.Error(w, err.Error(), http.StatusMethodNotAllowed)
		return
	}

	var req wxtypes.GetConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("decode getconfig request: %v", err), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, s.engine.GetConfig(req.ILinkUserID, req.ContextToken))
}

func (s *Server) handleSendTyping(w http.ResponseWriter, r *http.Request) {
	if err := requireMethod(r, http.MethodPost); err != nil {
		http.Error(w, err.Error(), http.StatusMethodNotAllowed)
		return
	}

	var req wxtypes.SendTypingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("decode sendtyping request: %v", err), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, s.engine.SendTyping(req.ILinkUserID, req.Status))
}

func requireMethod(r *http.Request, want string) error {
	if r.Method != want {
		return fmt.Errorf("method %s not allowed", r.Method)
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
