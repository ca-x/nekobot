// Package transcription provides speech-to-text integrations.
package transcription

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"

	"nekobot/pkg/config"
	"nekobot/pkg/logger"
)

const (
	defaultGroqBase  = "https://api.groq.com/openai/v1"
	defaultGroqModel = "whisper-large-v3-turbo"
)

// Transcriber is the shared interface used by channels.
type Transcriber interface {
	Transcribe(ctx context.Context, audio []byte, filename string) (string, error)
}

// WhisperClient is a Groq Whisper API client.
type WhisperClient struct {
	log        *logger.Logger
	apiKey     string
	apiBase    string
	model      string
	httpClient *http.Client
}

// NewWhisperClient creates a Groq Whisper client.
func NewWhisperClient(log *logger.Logger, apiKey, apiBase, model string, timeout time.Duration) *WhisperClient {
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	if strings.TrimSpace(apiBase) == "" {
		apiBase = defaultGroqBase
	}
	if strings.TrimSpace(model) == "" {
		model = defaultGroqModel
	}

	return &WhisperClient{
		log:     log,
		apiKey:  strings.TrimSpace(apiKey),
		apiBase: strings.TrimRight(strings.TrimSpace(apiBase), "/"),
		model:   strings.TrimSpace(model),
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// NewFromConfig creates a Whisper transcriber from global config.
// Returns nil when transcription is disabled or no API key can be resolved.
func NewFromConfig(log *logger.Logger, cfg *config.Config) Transcriber {
	if cfg == nil {
		return nil
	}
	if !cfg.Transcription.Enabled {
		return nil
	}

	apiKey := strings.TrimSpace(cfg.Transcription.APIKey)
	if apiKey == "" {
		// Reuse provider API key, prefer explicit provider first.
		if p := cfg.GetProviderConfig(cfg.Transcription.Provider); p != nil && p.APIKey != "" {
			apiKey = p.APIKey
		}
	}
	if apiKey == "" {
		// Fallback to provider profile named "groq".
		if p := cfg.GetProviderConfig("groq"); p != nil && p.APIKey != "" {
			apiKey = p.APIKey
		}
	}
	if apiKey == "" {
		for i := range cfg.Providers {
			p := &cfg.Providers[i]
			if strings.Contains(strings.ToLower(p.APIBase), "api.groq.com") && p.APIKey != "" {
				apiKey = p.APIKey
				break
			}
		}
	}
	if apiKey == "" {
		log.Warn("Transcription enabled but no API key found (set transcription.api_key or groq provider api_key)")
		return nil
	}

	apiBase := cfg.Transcription.APIBase
	if apiBase == "" {
		if p := cfg.GetProviderConfig(cfg.Transcription.Provider); p != nil && p.APIBase != "" {
			apiBase = p.APIBase
		}
	}
	if apiBase == "" {
		if p := cfg.GetProviderConfig("groq"); p != nil && p.APIBase != "" {
			apiBase = p.APIBase
		}
	}
	timeout := time.Duration(cfg.Transcription.TimeoutSeconds) * time.Second
	return NewWhisperClient(log, apiKey, apiBase, cfg.Transcription.Model, timeout)
}

// Transcribe sends audio bytes to Groq Whisper and returns transcribed text.
func (c *WhisperClient) Transcribe(ctx context.Context, audio []byte, filename string) (string, error) {
	if len(audio) == 0 {
		return "", fmt.Errorf("audio is empty")
	}
	if c.apiKey == "" {
		return "", fmt.Errorf("transcription api key is empty")
	}
	if strings.TrimSpace(filename) == "" {
		filename = "audio.ogg"
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	if err := writer.WriteField("model", c.model); err != nil {
		return "", fmt.Errorf("writing model field: %w", err)
	}

	part, err := writer.CreateFormFile("file", filepath.Base(filename))
	if err != nil {
		return "", fmt.Errorf("creating file part: %w", err)
	}
	if _, err := part.Write(audio); err != nil {
		return "", fmt.Errorf("writing audio payload: %w", err)
	}
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("closing multipart writer: %w", err)
	}

	url := c.apiBase + "/audio/transcriptions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &body)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("calling whisper api: %w", err)
	}
	defer resp.Body.Close()

	rawResp, _ := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("whisper api status %d: %s", resp.StatusCode, strings.TrimSpace(string(rawResp)))
	}

	var payload struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(rawResp, &payload); err != nil {
		return "", fmt.Errorf("decoding whisper response: %w", err)
	}
	text := strings.TrimSpace(payload.Text)
	if text == "" {
		c.log.Warn("Whisper transcription returned empty text")
	}
	c.log.Debug("Whisper transcription complete", zap.Int("text_len", len(text)))
	return text, nil
}
