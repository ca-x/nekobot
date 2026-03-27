package wechat

import (
	"context"
	"os"
	"testing"
)

func TestGenerateShareQRCodeImageWritesPNG(t *testing.T) {
	t.Parallel()

	path, err := generateShareQRCodeImage(context.Background(), t.TempDir(), "https://example.com/share")
	if err != nil {
		t.Fatalf("generateShareQRCodeImage failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if len(data) < 8 {
		t.Fatalf("expected PNG data, got %d bytes", len(data))
	}
	if string(data[:8]) != "\x89PNG\r\n\x1a\n" {
		t.Fatalf("unexpected PNG header: %q", string(data[:8]))
	}
}

func TestGenerateShareQRCodeImageRejectsEmptyContent(t *testing.T) {
	t.Parallel()

	if _, err := generateShareQRCodeImage(context.Background(), t.TempDir(), "   "); err == nil {
		t.Fatal("expected error for empty QR content")
	}
}
