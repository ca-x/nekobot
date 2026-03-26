package tools

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mafredri/cdp/protocol/dom"
	"github.com/mafredri/cdp/protocol/emulation"
	"github.com/mafredri/cdp/protocol/input"
	"github.com/mafredri/cdp/protocol/page"
	"github.com/mafredri/cdp/protocol/runtime"
	"go.uber.org/zap"

	"nekobot/pkg/logger"
)

// BrowserTool provides browser automation using Chrome DevTools Protocol.
type BrowserTool struct {
	log       *logger.Logger
	headless  bool
	timeout   time.Duration
	outputDir string
}

// NewBrowserTool creates a new browser tool.
func NewBrowserTool(log *logger.Logger, headless bool, timeout int, outputDir string) *BrowserTool {
	var t time.Duration
	if timeout > 0 {
		t = time.Duration(timeout) * time.Second
	} else {
		t = 30 * time.Second
	}

	if outputDir == "" {
		homeDir, _ := os.UserHomeDir()
		outputDir = filepath.Join(homeDir, ".nekobot", "screenshots")
	}

	// Ensure output directory exists
	_ = os.MkdirAll(outputDir, 0755)

	return &BrowserTool{
		log:       log,
		headless:  headless,
		timeout:   t,
		outputDir: outputDir,
	}
}

// Name returns the tool name.
func (b *BrowserTool) Name() string {
	return "browser"
}

// Description returns the tool description.
func (b *BrowserTool) Description() string {
	return "Browser automation tool using Chrome DevTools Protocol. Supports navigation, screenshots, script execution, element interaction, and more."
}

// Parameters returns the tool parameters schema.
func (b *BrowserTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type": "string",
				"enum": []string{
					"navigate", "screenshot", "execute_script",
					"click", "type", "select", "get_html",
					"wait", "scroll", "go_back", "go_forward",
					"print_pdf", "extract_structured_data",
					"reload", "close",
				},
				"description": "Browser action to perform",
			},
			"url": map[string]interface{}{
				"type":        "string",
				"description": "URL to navigate to (for navigate action)",
			},
			"script": map[string]interface{}{
				"type":        "string",
				"description": "JavaScript code to execute (for execute_script action)",
			},
			"selector": map[string]interface{}{
				"type":        "string",
				"description": "CSS selector for element (for click, type, select actions)",
			},
			"text": map[string]interface{}{
				"type":        "string",
				"description": "Text to type (for type action)",
			},
			"width": map[string]interface{}{
				"type":        "integer",
				"description": "Screenshot width in pixels (default: 1920)",
			},
			"height": map[string]interface{}{
				"type":        "integer",
				"description": "Screenshot height in pixels (default: 1080)",
			},
			"duration": map[string]interface{}{
				"type":        "integer",
				"description": "Wait duration in milliseconds (for wait action)",
			},
			"mode": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"auto", "direct"},
				"description": "Browser session startup mode. auto reuses existing Chrome before launching; direct only uses direct CDP attach/launch.",
			},
			"landscape": map[string]interface{}{
				"type":        "boolean",
				"description": "Render PDF in landscape orientation (for print_pdf action).",
			},
			"display_header_footer": map[string]interface{}{
				"type":        "boolean",
				"description": "Include header and footer in generated PDF (for print_pdf action).",
			},
			"print_background": map[string]interface{}{
				"type":        "boolean",
				"description": "Include CSS backgrounds in generated PDF (for print_pdf action).",
			},
			"margin_top": map[string]interface{}{
				"type":        "number",
				"description": "Top PDF margin in inches (for print_pdf action).",
			},
			"margin_bottom": map[string]interface{}{
				"type":        "number",
				"description": "Bottom PDF margin in inches (for print_pdf action).",
			},
			"margin_left": map[string]interface{}{
				"type":        "number",
				"description": "Left PDF margin in inches (for print_pdf action).",
			},
			"margin_right": map[string]interface{}{
				"type":        "number",
				"description": "Right PDF margin in inches (for print_pdf action).",
			},
			"extract_type": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"all", "schema_org", "json_ld", "meta"},
				"description": "Structured data extraction mode for extract_structured_data.",
			},
		},
		"required": []string{"action"},
	}
}

// Execute executes the browser tool.
func (b *BrowserTool) Execute(ctx context.Context, params map[string]interface{}) (string, error) {
	action, ok := params["action"].(string)
	if !ok {
		return "", fmt.Errorf("action parameter is required")
	}

	switch action {
	case "navigate":
		return b.navigate(ctx, params)
	case "screenshot":
		return b.screenshot(ctx, params)
	case "execute_script":
		return b.executeScript(ctx, params)
	case "click":
		return b.click(ctx, params)
	case "type":
		return b.typeText(ctx, params)
	case "get_html":
		return b.getHTML(ctx, params)
	case "wait":
		return b.wait(ctx, params)
	case "scroll":
		return b.scroll(ctx, params)
	case "go_back":
		return b.goBack(ctx)
	case "go_forward":
		return b.goForward(ctx)
	case "print_pdf":
		return b.printPDF(ctx, params)
	case "extract_structured_data":
		return b.extractStructuredData(ctx, params)
	case "reload":
		return b.reload(ctx)
	case "close":
		return b.close()
	default:
		return "", fmt.Errorf("unknown action: %s", action)
	}
}

// navigate navigates to a URL.
func (b *BrowserTool) navigate(ctx context.Context, params map[string]interface{}) (string, error) {
	urlStr, ok := params["url"].(string)
	if !ok {
		return "", fmt.Errorf("url parameter is required")
	}

	if _, err := url.Parse(urlStr); err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	b.log.Info("Browser navigating",
		zap.String("url", urlStr))

	sessionMgr := GetBrowserSession(b.log)
	mode, err := b.startMode(params)
	if err != nil {
		return "", err
	}
	if !sessionMgr.IsReady() {
		if err := sessionMgr.StartWithMode(b.timeout, mode); err != nil {
			return "", fmt.Errorf("failed to start browser: %w", err)
		}
	}

	client, err := sessionMgr.GetClient()
	if err != nil {
		return "", fmt.Errorf("failed to get client: %w", err)
	}

	nav, err := client.Page.Navigate(ctx, page.NewNavigateArgs(urlStr))
	if err != nil {
		return "", fmt.Errorf("failed to navigate: %w", err)
	}

	// Wait for page load
	domLoaded, err := client.Page.DOMContentEventFired(ctx)
	if err != nil {
		b.log.Warn("Failed to wait for DOMContentLoaded",
			zap.Error(err))
	} else {
		defer domLoaded.Close()
		_, _ = domLoaded.Recv()
	}

	return fmt.Sprintf("Navigated to: %s\nFrame ID: %s", urlStr, nav.FrameID), nil
}

func (b *BrowserTool) startMode(params map[string]interface{}) (BrowserConnectionMode, error) {
	rawMode, _ := params["mode"].(string)
	if strings.TrimSpace(rawMode) == "" {
		return BrowserModeAuto, nil
	}
	mode := resolveBrowserMode(rawMode)
	if mode == BrowserModeAuto && strings.TrimSpace(strings.ToLower(rawMode)) != string(BrowserModeAuto) {
		return "", fmt.Errorf("invalid browser mode: %s", rawMode)
	}
	return mode, nil
}

func (b *BrowserTool) buildPrintToPDFArgs(params map[string]interface{}) *page.PrintToPDFArgs {
	pdfArgs := page.NewPrintToPDFArgs().
		SetPreferCSSPageSize(true).
		SetPaperWidth(8.27).
		SetPaperHeight(11.69)

	if landscape, ok := params["landscape"].(bool); ok {
		pdfArgs.SetLandscape(landscape)
	}
	if displayHeaderFooter, ok := params["display_header_footer"].(bool); ok {
		pdfArgs.SetDisplayHeaderFooter(displayHeaderFooter)
	}
	if printBackground, ok := params["print_background"].(bool); ok {
		pdfArgs.SetPrintBackground(printBackground)
	}
	if marginTop, ok := params["margin_top"].(float64); ok {
		pdfArgs.SetMarginTop(marginTop)
	}
	if marginBottom, ok := params["margin_bottom"].(float64); ok {
		pdfArgs.SetMarginBottom(marginBottom)
	}
	if marginLeft, ok := params["margin_left"].(float64); ok {
		pdfArgs.SetMarginLeft(marginLeft)
	}
	if marginRight, ok := params["margin_right"].(float64); ok {
		pdfArgs.SetMarginRight(marginRight)
	}

	return pdfArgs
}

func (b *BrowserTool) buildExtractionScript(extractType string) string {
	baseScript := `(function() {
		const data = {};
	`

	switch extractType {
	case "schema_org":
		baseScript += `
		const schemaOrg = [];
		document.querySelectorAll('[itemscope]').forEach(item => {
			const schema = {};
			const itemType = item.getAttribute('itemtype');
			if (itemType) schema['@type'] = itemType;
			item.querySelectorAll('[itemprop]').forEach(prop => {
				const propName = prop.getAttribute('itemprop');
				schema[propName] = prop.textContent.trim() || prop.getAttribute('content') || prop.getAttribute('href');
			});
			if (Object.keys(schema).length > 0) schemaOrg.push(schema);
		});
		data.schema_org = schemaOrg;
		`
	case "json_ld":
		baseScript += `
		const jsonLd = [];
		document.querySelectorAll('script[type="application/ld+json"]').forEach(script => {
			try {
				jsonLd.push(JSON.parse(script.textContent));
			} catch (e) {}
		});
		data.json_ld = jsonLd;
		`
	case "meta":
		baseScript += `
		const meta = {};
		document.querySelectorAll('meta').forEach(tag => {
			const name = tag.getAttribute('name') || tag.getAttribute('property');
			const content = tag.getAttribute('content');
			if (name && content) meta[name] = content;
		});
		data.meta = meta;

		const og = {};
		document.querySelectorAll('meta[property^="og:"]').forEach(tag => {
			const prop = tag.getAttribute('property').substring(3);
			og[prop] = tag.getAttribute('content');
		});
		data.open_graph = og;

		const twitter = {};
		document.querySelectorAll('meta[name^="twitter:"]').forEach(tag => {
			const prop = tag.getAttribute('name').substring(8);
			twitter[prop] = tag.getAttribute('content');
		});
		data.twitter_card = twitter;
		`
	case "all":
		baseScript = `(function() {
		const data = {};
		const schemaOrg = [];
		document.querySelectorAll('[itemscope]').forEach(item => {
			const schema = {};
			const itemType = item.getAttribute('itemtype');
			if (itemType) schema['@type'] = itemType;
			item.querySelectorAll('[itemprop]').forEach(prop => {
				const propName = prop.getAttribute('itemprop');
				schema[propName] = prop.textContent.trim() || prop.getAttribute('content') || prop.getAttribute('href');
			});
			if (Object.keys(schema).length > 0) schemaOrg.push(schema);
		});
		data.schema_org = schemaOrg;

		const jsonLd = [];
		document.querySelectorAll('script[type="application/ld+json"]').forEach(script => {
			try {
				jsonLd.push(JSON.parse(script.textContent));
			} catch (e) {}
		});
		data.json_ld = jsonLd;

		const meta = {};
		document.querySelectorAll('meta').forEach(tag => {
			const name = tag.getAttribute('name') || tag.getAttribute('property');
			const content = tag.getAttribute('content');
			if (name && content) meta[name] = content;
		});
		data.meta = meta;

		const og = {};
		document.querySelectorAll('meta[property^="og:"]').forEach(tag => {
			const prop = tag.getAttribute('property').substring(3);
			og[prop] = tag.getAttribute('content');
		});
		data.open_graph = og;

		const twitter = {};
		document.querySelectorAll('meta[name^="twitter:"]').forEach(tag => {
			const prop = tag.getAttribute('name').substring(8);
			twitter[prop] = tag.getAttribute('content');
		});
		data.twitter_card = twitter;

		return JSON.stringify(data, null, 2);
	})();`
		return baseScript
	}

	baseScript += `
		return JSON.stringify(data, null, 2);
	})();`
	return baseScript
}

// screenshot takes a screenshot of the current page.
func (b *BrowserTool) screenshot(ctx context.Context, params map[string]interface{}) (string, error) {
	width := 1920
	height := 1080

	if w, ok := params["width"].(float64); ok {
		width = int(w)
	}
	if h, ok := params["height"].(float64); ok {
		height = int(h)
	}

	// Navigate if URL provided
	if urlStr, ok := params["url"].(string); ok && urlStr != "" {
		if _, err := b.navigate(ctx, map[string]interface{}{"url": urlStr}); err != nil {
			return "", err
		}
		time.Sleep(1 * time.Second) // Wait for page to stabilize
	}

	sessionMgr := GetBrowserSession(b.log)
	if !sessionMgr.IsReady() {
		return "", fmt.Errorf("browser session not ready")
	}

	client, err := sessionMgr.GetClient()
	if err != nil {
		return "", err
	}

	// Set viewport size
	if err := client.Emulation.SetDeviceMetricsOverride(ctx, emulation.NewSetDeviceMetricsOverrideArgs(
		width, height, 1.0, false,
	)); err != nil {
		b.log.Warn("Failed to set viewport",
			zap.Error(err))
	}

	// Capture screenshot
	screenshotArgs := page.NewCaptureScreenshotArgs().SetFormat("png")
	screenshot, err := client.Page.CaptureScreenshot(ctx, screenshotArgs)
	if err != nil {
		return "", fmt.Errorf("failed to capture screenshot: %w", err)
	}

	// Get current URL
	frameTree, err := client.Page.GetFrameTree(ctx)
	currentURL := ""
	if err == nil {
		currentURL = frameTree.FrameTree.Frame.URL
	}

	// Save to file
	filename := fmt.Sprintf("screenshot_%d.png", time.Now().Unix())
	filepath := filepath.Join(b.outputDir, filename)
	if err := os.WriteFile(filepath, screenshot.Data, 0644); err != nil {
		return "", fmt.Errorf("failed to save screenshot: %w", err)
	}

	base64Str := base64.StdEncoding.EncodeToString(screenshot.Data)

	return fmt.Sprintf("Screenshot saved to: %s\nURL: %s\nSize: %d bytes\nBase64 length: %d",
		filepath, currentURL, len(screenshot.Data), len(base64Str)), nil
}

func (b *BrowserTool) printPDF(ctx context.Context, params map[string]interface{}) (string, error) {
	if urlStr, ok := params["url"].(string); ok && strings.TrimSpace(urlStr) != "" {
		if _, err := b.navigate(ctx, params); err != nil {
			return "", err
		}
		time.Sleep(2 * time.Second)
	}

	sessionMgr := GetBrowserSession(b.log)
	if !sessionMgr.IsReady() {
		mode, err := b.startMode(params)
		if err != nil {
			return "", err
		}
		if err := sessionMgr.StartWithMode(b.timeout, mode); err != nil {
			return "", fmt.Errorf("failed to start browser: %w", err)
		}
	}

	client, err := sessionMgr.GetClient()
	if err != nil {
		return "", err
	}

	pdfResult, err := client.Page.PrintToPDF(ctx, b.buildPrintToPDFArgs(params))
	if err != nil {
		return "", fmt.Errorf("failed to generate PDF: %w", err)
	}

	filename := fmt.Sprintf("page_%d.pdf", time.Now().Unix())
	path := filepath.Join(b.outputDir, filename)
	if err := os.WriteFile(path, pdfResult.Data, 0644); err != nil {
		return "", fmt.Errorf("failed to save PDF: %w", err)
	}

	return fmt.Sprintf("PDF saved to: %s\nSize: %d bytes", path, len(pdfResult.Data)), nil
}

func (b *BrowserTool) extractStructuredData(ctx context.Context, params map[string]interface{}) (string, error) {
	urlStr, ok := params["url"].(string)
	if !ok || strings.TrimSpace(urlStr) == "" {
		return "", fmt.Errorf("url parameter is required")
	}

	extractType := "all"
	if rawType, ok := params["extract_type"].(string); ok && strings.TrimSpace(rawType) != "" {
		extractType = strings.TrimSpace(strings.ToLower(rawType))
	}

	switch extractType {
	case "all", "schema_org", "json_ld", "meta":
	default:
		return "", fmt.Errorf("invalid extract_type: %s", extractType)
	}

	if _, err := b.navigate(ctx, params); err != nil {
		return "", err
	}

	sessionMgr := GetBrowserSession(b.log)
	client, err := sessionMgr.GetClient()
	if err != nil {
		return "", err
	}

	evalArgs := runtime.NewEvaluateArgs(b.buildExtractionScript(extractType)).
		SetReturnByValue(true)
	result, err := client.Runtime.Evaluate(ctx, evalArgs)
	if err != nil {
		return "", fmt.Errorf("failed to execute extraction script: %w", err)
	}
	if result.ExceptionDetails != nil {
		return "", fmt.Errorf("extraction script error: %s", result.ExceptionDetails.Text)
	}

	return formatCDPResult(&result.Result)
}

// executeScript executes JavaScript in the browser.
func (b *BrowserTool) executeScript(ctx context.Context, params map[string]interface{}) (string, error) {
	script, ok := params["script"].(string)
	if !ok {
		return "", fmt.Errorf("script parameter is required")
	}

	// Navigate if URL provided
	if urlStr, ok := params["url"].(string); ok && urlStr != "" {
		if _, err := b.navigate(ctx, map[string]interface{}{"url": urlStr}); err != nil {
			return "", err
		}
	}

	sessionMgr := GetBrowserSession(b.log)
	if !sessionMgr.IsReady() {
		return "", fmt.Errorf("browser session not ready")
	}

	client, err := sessionMgr.GetClient()
	if err != nil {
		return "", err
	}

	b.log.Info("Executing JavaScript",
		zap.String("script", script[:min(len(script), 100)]))

	evalArgs := runtime.NewEvaluateArgs(script).
		SetReturnByValue(true).
		SetAwaitPromise(true)

	result, err := client.Runtime.Evaluate(ctx, evalArgs)
	if err != nil {
		return "", fmt.Errorf("failed to execute script: %w", err)
	}

	if result.ExceptionDetails != nil {
		return "", fmt.Errorf("script error: %s", result.ExceptionDetails.Text)
	}

	return fmt.Sprintf("Script executed successfully\nResult: %v", result.Result.Value), nil
}

// click clicks an element by CSS selector.
func (b *BrowserTool) click(ctx context.Context, params map[string]interface{}) (string, error) {
	selector, ok := params["selector"].(string)
	if !ok {
		return "", fmt.Errorf("selector parameter is required")
	}

	sessionMgr := GetBrowserSession(b.log)
	if !sessionMgr.IsReady() {
		return "", fmt.Errorf("browser session not ready")
	}

	client, err := sessionMgr.GetClient()
	if err != nil {
		return "", err
	}

	// Get document root
	doc, err := client.DOM.GetDocument(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get document: %w", err)
	}

	// Query selector
	node, err := client.DOM.QuerySelector(ctx, &dom.QuerySelectorArgs{
		NodeID:   doc.Root.NodeID,
		Selector: selector,
	})
	if err != nil {
		return "", fmt.Errorf("failed to find element: %w", err)
	}

	// Get box model to get click coordinates
	boxModel, err := client.DOM.GetBoxModel(ctx, &dom.GetBoxModelArgs{
		NodeID: &node.NodeID,
	})
	if err != nil {
		return "", fmt.Errorf("failed to get element position: %w", err)
	}

	// Calculate center of element
	x := (boxModel.Model.Border[0] + boxModel.Model.Border[2]) / 2
	y := (boxModel.Model.Border[1] + boxModel.Model.Border[5]) / 2

	// Click at position
	mouseArgs := input.NewDispatchMouseEventArgs("mousePressed", x, y).
		SetButton(input.MouseButtonLeft).
		SetClickCount(1)
	if err := client.Input.DispatchMouseEvent(ctx, mouseArgs); err != nil {
		return "", fmt.Errorf("failed to click: %w", err)
	}

	releaseArgs := input.NewDispatchMouseEventArgs("mouseReleased", x, y).
		SetButton(input.MouseButtonLeft).
		SetClickCount(1)
	if err := client.Input.DispatchMouseEvent(ctx, releaseArgs); err != nil {
		return "", fmt.Errorf("failed to release click: %w", err)
	}

	return fmt.Sprintf("Clicked element: %s at (%f, %f)", selector, x, y), nil
}

// typeText types text into an element.
func (b *BrowserTool) typeText(ctx context.Context, params map[string]interface{}) (string, error) {
	selector, ok := params["selector"].(string)
	if !ok {
		return "", fmt.Errorf("selector parameter is required")
	}

	text, ok := params["text"].(string)
	if !ok {
		return "", fmt.Errorf("text parameter is required")
	}

	// Click element first to focus
	if _, err := b.click(ctx, params); err != nil {
		return "", err
	}

	sessionMgr := GetBrowserSession(b.log)
	client, err := sessionMgr.GetClient()
	if err != nil {
		return "", err
	}

	// Type each character
	for _, char := range text {
		keyArgs := input.NewDispatchKeyEventArgs("char").
			SetText(string(char))
		if err := client.Input.DispatchKeyEvent(ctx, keyArgs); err != nil {
			return "", fmt.Errorf("failed to type character: %w", err)
		}
		time.Sleep(10 * time.Millisecond) // Small delay between characters
	}

	return fmt.Sprintf("Typed text into %s: %s", selector, text), nil
}

// getHTML gets the HTML content of the page.
func (b *BrowserTool) getHTML(ctx context.Context, params map[string]interface{}) (string, error) {
	sessionMgr := GetBrowserSession(b.log)
	if !sessionMgr.IsReady() {
		return "", fmt.Errorf("browser session not ready")
	}

	client, err := sessionMgr.GetClient()
	if err != nil {
		return "", err
	}

	doc, err := client.DOM.GetDocument(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get document: %w", err)
	}

	html, err := client.DOM.GetOuterHTML(ctx, &dom.GetOuterHTMLArgs{
		NodeID: &doc.Root.NodeID,
	})
	if err != nil {
		return "", fmt.Errorf("failed to get HTML: %w", err)
	}

	return html.OuterHTML, nil
}

// wait waits for a specified duration.
func (b *BrowserTool) wait(ctx context.Context, params map[string]interface{}) (string, error) {
	duration := 1000 // default 1 second

	if d, ok := params["duration"].(float64); ok {
		duration = int(d)
	}

	time.Sleep(time.Duration(duration) * time.Millisecond)
	return fmt.Sprintf("Waited for %d milliseconds", duration), nil
}

// scroll scrolls the page.
func (b *BrowserTool) scroll(ctx context.Context, params map[string]interface{}) (string, error) {
	script := "window.scrollTo(0, document.body.scrollHeight);"
	return b.executeScript(ctx, map[string]interface{}{"script": script})
}

// goBack navigates back in history.
func (b *BrowserTool) goBack(ctx context.Context) (string, error) {
	script := "window.history.back();"
	return b.executeScript(ctx, map[string]interface{}{"script": script})
}

// goForward navigates forward in history.
func (b *BrowserTool) goForward(ctx context.Context) (string, error) {
	script := "window.history.forward();"
	return b.executeScript(ctx, map[string]interface{}{"script": script})
}

// reload reloads the current page.
func (b *BrowserTool) reload(ctx context.Context) (string, error) {
	sessionMgr := GetBrowserSession(b.log)
	if !sessionMgr.IsReady() {
		return "", fmt.Errorf("browser session not ready")
	}

	client, err := sessionMgr.GetClient()
	if err != nil {
		return "", err
	}

	if err := client.Page.Reload(ctx, nil); err != nil {
		return "", fmt.Errorf("failed to reload: %w", err)
	}

	return "Page reloaded", nil
}

// close closes the browser session.
func (b *BrowserTool) close() (string, error) {
	sessionMgr := GetBrowserSession(b.log)
	if err := sessionMgr.Stop(); err != nil {
		return "", err
	}
	return "Browser session closed", nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func formatCDPResult(result *runtime.RemoteObject) (string, error) {
	if result == nil {
		return "null", nil
	}
	if result.Value != nil {
		return string(result.Value), nil
	}
	if result.Description != nil {
		return *result.Description, nil
	}
	return "", nil
}
