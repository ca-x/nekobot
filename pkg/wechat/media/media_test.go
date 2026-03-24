package media

import (
	"context"
	"crypto/aes"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseAESKeySupportsRawAndHex(t *testing.T) {
	raw := []byte("1234567890abcdef")
	got, err := ParseAESKey(base64.StdEncoding.EncodeToString(raw))
	if err != nil {
		t.Fatalf("ParseAESKey(raw) failed: %v", err)
	}
	if string(got) != string(raw) {
		t.Fatalf("unexpected raw key: %q", got)
	}

	hexEncoded := base64.StdEncoding.EncodeToString([]byte("31323334353637383930616263646566"))
	got, err = ParseAESKey(hexEncoded)
	if err != nil {
		t.Fatalf("ParseAESKey(hex) failed: %v", err)
	}
	if string(got) != string(raw) {
		t.Fatalf("unexpected hex key: %q", got)
	}
}

func TestDecryptAESECB(t *testing.T) {
	key := []byte("1234567890abcdef")
	plain := []byte("hello wechat media")
	ciphertext := encryptAESECBForTest(plain, key)

	got, err := DecryptAESECB(ciphertext, key)
	if err != nil {
		t.Fatalf("DecryptAESECB failed: %v", err)
	}
	if string(got) != string(plain) {
		t.Fatalf("unexpected plaintext: %q", got)
	}
}

func TestDownloadFromItemDownloadsAndDecryptsFile(t *testing.T) {
	key := []byte("1234567890abcdef")
	plain := []byte("secret file content")
	ciphertext := encryptAESECBForTest(plain, key)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(ciphertext)
	}))
	defer server.Close()

	downloader := NewDownloader(t.TempDir())
	downloader.cdnBaseURL = server.URL

	result, err := downloader.DownloadFromItem(context.Background(), &Item{
		Type: ItemTypeFile,
		File: &FileItem{
			FileName: "doc.txt",
			Media: &CDNMedia{
				EncryptQueryParam: "abc123",
				AESKey:            base64.StdEncoding.EncodeToString(key),
			},
		},
	})
	if err != nil {
		t.Fatalf("DownloadFromItem failed: %v", err)
	}
	if result == nil {
		t.Fatal("expected media result")
	}
	data, err := os.ReadFile(result.FilePath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(data) != string(plain) {
		t.Fatalf("unexpected downloaded plaintext: %q", data)
	}
	if filepath.Ext(result.FilePath) != ".txt" {
		t.Fatalf("expected .txt extension, got %s", result.FilePath)
	}
}

func TestBuildBodyIncludesDownloadedMedia(t *testing.T) {
	body := BuildBody([]Item{
		{Type: ItemTypeText, Text: &TextItem{Text: "hello"}},
	}, &InboundMediaResult{
		FilePath: "/tmp/example.jpg",
		MIMEType: "image/jpeg",
	})
	if !strings.Contains(body, "hello") || !strings.Contains(body, "/tmp/example.jpg") {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestInboundProcessorTranscribesVoiceMedia(t *testing.T) {
	processor := NewInboundProcessor(fakeDownloader{
		result: &InboundMediaResult{FilePath: writeTestFile(t, "voice.silk", []byte("audio-bytes")), MIMEType: "audio/silk"},
	}, fakeTranscriber{text: "hello from voice"})

	body, err := processor.Process(context.Background(), []Item{{
		Type:  ItemTypeVoice,
		Voice: &VoiceItem{Media: &CDNMedia{EncryptQueryParam: "abc", AESKey: "key"}},
	}})
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if !strings.Contains(body, "语音转写: hello from voice") {
		t.Fatalf("expected transcription in body, got %q", body)
	}
}

func TestInboundProcessorReturnsBodyWhenTranscriptionFails(t *testing.T) {
	processor := NewInboundProcessor(fakeDownloader{
		result: &InboundMediaResult{FilePath: writeTestFile(t, "voice.silk", []byte("audio-bytes")), MIMEType: "audio/silk"},
	}, fakeTranscriber{err: fmt.Errorf("boom")})

	body, err := processor.Process(context.Background(), []Item{{
		Type:  ItemTypeVoice,
		Voice: &VoiceItem{Media: &CDNMedia{EncryptQueryParam: "abc", AESKey: "key"}},
	}})
	if err == nil {
		t.Fatal("expected transcription error")
	}
	if !strings.Contains(body, "音频已下载到本地") {
		t.Fatalf("expected fallback media body, got %q", body)
	}
}

type fakeDownloader struct {
	result *InboundMediaResult
	err    error
}

func (f fakeDownloader) DownloadFromItem(ctx context.Context, item *Item) (*InboundMediaResult, error) {
	return f.result, f.err
}

type fakeTranscriber struct {
	text string
	err  error
}

func (f fakeTranscriber) Transcribe(ctx context.Context, audio []byte, filename string) (string, error) {
	return f.text, f.err
}

func writeTestFile(t *testing.T, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	return path
}

func encryptAESECBForTest(plain, key []byte) []byte {
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err)
	}
	padding := block.BlockSize() - (len(plain) % block.BlockSize())
	if padding == 0 {
		padding = block.BlockSize()
	}
	padded := append([]byte{}, plain...)
	for range padding {
		padded = append(padded, byte(padding))
	}

	out := make([]byte, len(padded))
	for offset := 0; offset < len(padded); offset += block.BlockSize() {
		block.Encrypt(out[offset:offset+block.BlockSize()], padded[offset:offset+block.BlockSize()])
	}
	return out
}
