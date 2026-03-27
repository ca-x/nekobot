package mockserver

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"

	wxtypes "nekobot/pkg/wechat/types"
)

// SentMessage records an outbound bot message captured by the mock server.
type SentMessage struct {
	Recipient    string
	Text         string
	ContextToken string
}

// Engine provides in-memory iLink behavior for SDK and binding tests.
type Engine struct {
	mu            sync.Mutex
	qrCode        string
	qrContent     string
	qrStatus      string
	confirmedCred *wxtypes.Credentials
	inbound       []wxtypes.WeixinMessage
	sent          []SentMessage
	typing        map[string]bool
}

// NewEngine creates a new mock engine.
func NewEngine() *Engine {
	return &Engine{
		typing: make(map[string]bool),
	}
}

// FetchQRCode creates a fresh QR code session.
func (e *Engine) FetchQRCode() *wxtypes.QRCodeResponse {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.qrCode = "qr-" + randomHex(8)
	e.qrContent = "mock-qr:" + e.qrCode
	e.qrStatus = wxtypes.QRStatusWait
	e.confirmedCred = nil

	return &wxtypes.QRCodeResponse{
		QRCode:           e.qrCode,
		QRCodeImgContent: e.qrContent,
	}
}

// CheckQRStatus returns the current QR status for the active QR code.
func (e *Engine) CheckQRStatus(qrCode string) *wxtypes.QRStatusResponse {
	e.mu.Lock()
	defer e.mu.Unlock()

	resp := &wxtypes.QRStatusResponse{Status: wxtypes.QRStatusExpired}
	if qrCode == "" || qrCode != e.qrCode {
		return resp
	}

	resp.Status = e.qrStatus
	if e.confirmedCred != nil {
		resp.BotToken = e.confirmedCred.BotToken
		resp.ILinkBotID = e.confirmedCred.ILinkBotID
		resp.BaseURL = e.confirmedCred.BaseURL
		resp.ILinkUserID = e.confirmedCred.ILinkUserID
	}
	return resp
}

// ScanQR marks the current QR code as scanned.
func (e *Engine) ScanQR() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.qrCode == "" {
		return
	}
	e.qrStatus = wxtypes.QRStatusScanned
}

// ConfirmQR marks the current QR code as confirmed and saves credentials.
func (e *Engine) ConfirmQR(creds *wxtypes.Credentials) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.qrCode == "" || creds == nil {
		return
	}
	copied := *creds
	e.confirmedCred = &copied
	e.qrStatus = wxtypes.QRStatusConfirmed
}

// InjectInbound appends an inbound message for the next getupdates call.
func (e *Engine) InjectInbound(msg wxtypes.WeixinMessage) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.inbound = append(e.inbound, msg)
}

// GetUpdates returns queued inbound messages and clears the queue.
func (e *Engine) GetUpdates(buf string) *wxtypes.GetUpdatesResponse {
	e.mu.Lock()
	defer e.mu.Unlock()

	msgs := append([]wxtypes.WeixinMessage(nil), e.inbound...)
	e.inbound = nil

	return &wxtypes.GetUpdatesResponse{
		Ret:           0,
		Msgs:          msgs,
		GetUpdatesBuf: "sync-" + randomHex(4),
	}
}

// SendMessage records an outbound message.
func (e *Engine) SendMessage(req *wxtypes.SendMessageRequest) *wxtypes.SendMessageResponse {
	e.mu.Lock()
	defer e.mu.Unlock()

	text := ""
	for _, item := range req.Msg.ItemList {
		if item.TextItem != nil {
			text = item.TextItem.Text
			break
		}
	}

	e.sent = append(e.sent, SentMessage{
		Recipient:    req.Msg.ToUserID,
		Text:         text,
		ContextToken: req.Msg.ContextToken,
	})

	return &wxtypes.SendMessageResponse{Ret: 0}
}

// SentMessages returns a copy of all sent messages.
func (e *Engine) SentMessages() []SentMessage {
	e.mu.Lock()
	defer e.mu.Unlock()

	out := make([]SentMessage, len(e.sent))
	copy(out, e.sent)
	return out
}

// GetConfig returns a stable typing ticket for the recipient.
func (e *Engine) GetConfig(userID, contextToken string) *wxtypes.GetConfigResponse {
	return &wxtypes.GetConfigResponse{
		Ret:          0,
		TypingTicket: fmt.Sprintf("typing:%s:%s", userID, contextToken),
	}
}

// SendTyping stores whether typing is enabled for a user.
func (e *Engine) SendTyping(userID string, status int) *wxtypes.SendTypingResponse {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.typing[userID] = status == wxtypes.TypingStatusTyping
	return &wxtypes.SendTypingResponse{Ret: 0}
}

// TypingEnabled reports the last typing state for the user.
func (e *Engine) TypingEnabled(userID string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.typing[userID]
}

func randomHex(size int) string {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "fallback"
	}
	return hex.EncodeToString(buf)
}
