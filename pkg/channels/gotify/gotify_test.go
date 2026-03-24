package gotify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"nekobot/pkg/bus"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
)

func TestChannelStartAndSendMessage(t *testing.T) {
	log, err := logger.New(&logger.Config{Level: "error"})
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}

	var verifyCalled bool
	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/current/application":
			verifyCalled = true
			if got := r.URL.Query().Get("token"); got != "test-token" {
				t.Fatalf("unexpected token in verify request: %q", got)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":1}`))
		case "/message":
			if got := r.URL.Query().Get("token"); got != "test-token" {
				t.Fatalf("unexpected token in message request: %q", got)
			}
			if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
				t.Fatalf("decode payload: %v", err)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":2}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	ch, err := NewChannel(log, config.GotifyConfig{
		Enabled:   true,
		ServerURL: server.URL,
		AppToken:  "test-token",
		Priority:  6,
	})
	if err != nil {
		t.Fatalf("new channel: %v", err)
	}

	if err := ch.Start(context.Background()); err != nil {
		t.Fatalf("start channel: %v", err)
	}
	if !verifyCalled {
		t.Fatalf("expected verify request to be called")
	}

	err = ch.SendMessage(context.Background(), &bus.Message{
		ChannelID: "gotify",
		Content:   "hello gotify",
		Username:  "tester",
		Data: map[string]any{
			"title": "Custom Title",
		},
	})
	if err != nil {
		t.Fatalf("send message: %v", err)
	}

	if gotPayload["title"] != "Custom Title" {
		t.Fatalf("expected title Custom Title, got %#v", gotPayload["title"])
	}
	if gotPayload["message"] != "hello gotify" {
		t.Fatalf("expected message hello gotify, got %#v", gotPayload["message"])
	}
	if gotPayload["priority"] != float64(6) {
		t.Fatalf("expected priority 6, got %#v", gotPayload["priority"])
	}
}
