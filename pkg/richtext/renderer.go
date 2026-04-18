package richtext

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/mafredri/cdp/protocol/emulation"
	"github.com/mafredri/cdp/protocol/page"
	"go.uber.org/zap"

	"nekobot/pkg/logger"
	"nekobot/pkg/tools"
)

// MarkdownImageRenderer renders markdown-like content into an image file.
type MarkdownImageRenderer interface {
	RenderMarkdown(ctx context.Context, markdown string) (string, error)
}

// BrowserMarkdownRenderer renders markdown into PNG through the shared headless browser session.
type BrowserMarkdownRenderer struct {
	log       *logger.Logger
	outputDir string
	width     int
	height    int
}

// NewBrowserMarkdownRenderer creates a browser-backed markdown image renderer.
func NewBrowserMarkdownRenderer(log *logger.Logger, outputDir string) *BrowserMarkdownRenderer {
	if err := os.MkdirAll(outputDir, 0o755); err != nil && log != nil {
		log.Warn("Failed to create markdown render output directory",
			zap.String("output_dir", outputDir),
			zap.Error(err),
		)
	}
	return &BrowserMarkdownRenderer{
		log:       log,
		outputDir: outputDir,
		width:     760,
		height:    1200,
	}
}

// RenderMarkdown renders markdown into a PNG file and returns the local path.
func (r *BrowserMarkdownRenderer) RenderMarkdown(ctx context.Context, markdown string) (string, error) {
	if r == nil {
		return "", fmt.Errorf("renderer is nil")
	}

	session := tools.GetBrowserSession(r.log)
	if !session.IsReady() {
		if err := session.Start(30 * time.Second); err != nil {
			return "", fmt.Errorf("start browser session: %w", err)
		}
	}

	client, err := session.GetClient()
	if err != nil {
		return "", fmt.Errorf("get browser client: %w", err)
	}

	dataURL := "data:text/html;base64," + base64.StdEncoding.EncodeToString([]byte(BuildMarkdownHTML(markdown)))
	if _, err := client.Page.Navigate(ctx, page.NewNavigateArgs(dataURL)); err != nil {
		return "", fmt.Errorf("navigate render page: %w", err)
	}

	if err := client.Emulation.SetDeviceMetricsOverride(ctx, emulation.NewSetDeviceMetricsOverrideArgs(
		r.width,
		r.height,
		1.0,
		false,
	)); err != nil {
		return "", fmt.Errorf("set render viewport: %w", err)
	}

	time.Sleep(300 * time.Millisecond)

	screenshot, err := client.Page.CaptureScreenshot(ctx, page.NewCaptureScreenshotArgs().SetFormat("png"))
	if err != nil {
		return "", fmt.Errorf("capture markdown screenshot: %w", err)
	}

	filename := filepath.Join(r.outputDir, fmt.Sprintf("wechat-md-%d.png", time.Now().UnixNano()))
	if err := os.WriteFile(filename, screenshot.Data, 0o644); err != nil {
		return "", fmt.Errorf("write markdown screenshot: %w", err)
	}
	return filename, nil
}

// BuildMarkdownDataURL returns a data URL for the rendered markdown HTML.
func BuildMarkdownDataURL(markdown string) string {
	return "data:text/html;charset=utf-8," + url.PathEscape(BuildMarkdownHTML(markdown))
}
