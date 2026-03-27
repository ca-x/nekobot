package wechat

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	rscqr "rsc.io/qr"

	wxauth "nekobot/pkg/wechat/auth"
)

func (c *Channel) createShareQRCode(ctx context.Context) (string, error) {
	if c == nil {
		return "", fmt.Errorf("wechat channel is nil")
	}

	qrResp, err := wxauth.FetchQRCode(ctx)
	if err != nil {
		return "", fmt.Errorf("fetch WeChat share QR code: %w", err)
	}

	baseDir := os.TempDir()
	if strings.TrimSpace(c.workspace) != "" {
		baseDir = filepath.Join(c.workspace, "tmp")
	}

	return generateShareQRCodeImage(ctx, baseDir, qrResp.QRCodeImgContent)
}

func generateShareQRCodeImage(_ context.Context, baseDir, content string) (string, error) {
	trimmedContent := strings.TrimSpace(content)
	if trimmedContent == "" {
		return "", fmt.Errorf("share QR content is empty")
	}

	if strings.TrimSpace(baseDir) == "" {
		baseDir = os.TempDir()
	}
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return "", fmt.Errorf("create share QR directory: %w", err)
	}

	code, err := rscqr.Encode(trimmedContent, rscqr.M)
	if err != nil {
		return "", fmt.Errorf("encode share QR code: %w", err)
	}
	code.Scale = 8

	path := filepath.Join(baseDir, "wechat-share-"+uuid.NewString()+".png")
	if err := os.WriteFile(path, code.PNG(), 0o600); err != nil {
		return "", fmt.Errorf("write share QR image: %w", err)
	}

	return path, nil
}
