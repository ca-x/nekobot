package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mafredri/cdp"
	"github.com/mafredri/cdp/devtool"
	"github.com/mafredri/cdp/protocol/dom"
	"github.com/mafredri/cdp/protocol/emulation"
	"github.com/mafredri/cdp/protocol/input"
	cdpLog "github.com/mafredri/cdp/protocol/log"
	"github.com/mafredri/cdp/protocol/network"
	"github.com/mafredri/cdp/protocol/page"
	"github.com/mafredri/cdp/protocol/performance"
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
					"click", "type", "select", "get_html", "get_console", "get_network", "get_session", "start_session", "reset_session", "restart_session", "close_session",
					"get_text", "get_title", "get_url", "get_links", "get_cookies", "set_cookie", "clear_cookies", "get_meta", "get_images", "get_forms", "get_buttons", "get_tables", "get_lists", "get_inputs", "get_selects", "get_textareas", "get_headings", "wait", "scroll", "go_back", "go_forward",
					"list_pages", "new_page", "activate_page", "close_page",
					"get_storage", "set_storage", "remove_storage", "clear_storage",
					"print_pdf", "extract_structured_data",
					"reload", "get_metrics", "emulate_device", "set_viewport", "close",
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
				"description": "Text to type, select value, or cookie value depending on action",
			},
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Cookie name (for set_cookie action)",
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
				"enum":        []string{"auto", "direct", "relay"},
				"description": "Browser session startup mode. auto reuses existing Chrome before launching; direct only uses direct CDP attach/launch; relay only attaches to an existing browser and never launches a new one.",
			},
			"debug_port": map[string]interface{}{
				"type":        "integer",
				"description": "Optional Chrome DevTools port override. When set, browser attach prefers this port instead of the default 9222/9223/9224 scan.",
			},
			"debug_endpoint": map[string]interface{}{
				"type":        "string",
				"description": "Optional Chrome DevTools base URL such as http://host:9222. When set, attach attempts this endpoint before any port scan.",
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
			"secure": map[string]interface{}{
				"type":        "boolean",
				"description": "Set secure flag on cookie (for set_cookie action).",
			},
			"http_only": map[string]interface{}{
				"type":        "boolean",
				"description": "Set HttpOnly flag on cookie (for set_cookie action).",
			},
			"same_site": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"strict", "lax", "none"},
				"description": "Optional SameSite policy for set_cookie.",
			},
			"max_entries": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of collected console or network entries.",
			},
			"resource_type": map[string]interface{}{
				"type":        "string",
				"description": "Optional network resource type filter for get_network (for example document, xhr, fetch, image).",
			},
			"device": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"iphone", "ipad", "android", "desktop"},
				"description": "Device profile to emulate for emulate_device action.",
			},
			"target_id": map[string]interface{}{
				"type":        "string",
				"description": "Browser target/page ID for activate_page and close_page actions.",
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
	case "select":
		return b.selectOption(ctx, params)
	case "get_html":
		return b.getHTML(ctx, params)
	case "get_console":
		return b.getConsole(ctx, params)
	case "get_network":
		return b.getNetwork(ctx, params)
	case "get_session":
		return b.getSession(ctx, params)
	case "start_session":
		return b.startSession(params)
	case "reset_session":
		return b.resetSession()
	case "restart_session":
		return b.restartSession(params)
	case "close_session":
		return b.close()
	case "get_text":
		return b.getText(ctx, params)
	case "get_title":
		return b.getTitle(ctx, params)
	case "get_url":
		return b.getURL(ctx, params)
	case "get_links":
		return b.getLinks(ctx, params)
	case "get_cookies":
		return b.getCookies(ctx, params)
	case "set_cookie":
		return b.setCookie(ctx, params)
	case "clear_cookies":
		return b.clearCookies(ctx, params)
	case "get_meta":
		return b.getMeta(ctx, params)
	case "get_images":
		return b.getImages(ctx, params)
	case "get_forms":
		return b.getForms(ctx, params)
	case "get_buttons":
		return b.getButtons(ctx, params)
	case "get_tables":
		return b.getTables(ctx, params)
	case "get_lists":
		return b.getLists(ctx, params)
	case "get_inputs":
		return b.getInputs(ctx, params)
	case "get_selects":
		return b.getSelects(ctx, params)
	case "get_textareas":
		return b.getTextareas(ctx, params)
	case "get_headings":
		return b.getHeadings(ctx, params)
	case "wait":
		return b.wait(ctx, params)
	case "scroll":
		return b.scroll(ctx, params)
	case "go_back":
		return b.goBack(ctx)
	case "go_forward":
		return b.goForward(ctx)
	case "list_pages":
		return b.listPages(ctx, params)
	case "new_page":
		return b.newPage(ctx, params)
	case "activate_page":
		return b.activatePage(ctx, params)
	case "close_page":
		return b.closePage(ctx, params)
	case "print_pdf":
		return b.printPDF(ctx, params)
	case "extract_structured_data":
		return b.extractStructuredData(ctx, params)
	case "reload":
		return b.reload(ctx)
	case "get_metrics":
		return b.getMetrics(ctx, params)
	case "emulate_device":
		return b.emulateDevice(ctx, params)
	case "set_viewport":
		return b.setViewport(ctx, params)
	case "close":
		return b.close()
	default:
		return "", fmt.Errorf("unknown action: %s", action)
	}
}

type browserDeviceProfile struct {
	Name       string
	Width      int
	Height     int
	Scale      float64
	Mobile     bool
	UserAgent  string
	Platform   string
	Touch      bool
	MaxTouches int
}

type browserConsoleOptions struct {
	ErrorsOnly   bool
	WarningsOnly bool
	InfoOnly     bool
	MaxEntries   int
}

type browserNetworkOptions struct {
	MaxEntries   int
	Duration     time.Duration
	ResourceType string
}

type browserConsoleEntry struct {
	Source     string   `json:"source"`
	Level      string   `json:"level"`
	Type       string   `json:"type,omitempty"`
	Text       string   `json:"text"`
	Timestamp  string   `json:"timestamp,omitempty"`
	URL        string   `json:"url,omitempty"`
	Context    string   `json:"context,omitempty"`
	LineNumber *int     `json:"line_number,omitempty"`
	Args       []string `json:"args,omitempty"`
}

type browserNetworkEntry struct {
	RequestID   string `json:"request_id"`
	Phase       string `json:"phase"`
	Type        string `json:"type,omitempty"`
	Method      string `json:"method,omitempty"`
	URL         string `json:"url,omitempty"`
	Status      *int   `json:"status,omitempty"`
	StatusText  string `json:"status_text,omitempty"`
	MimeType    string `json:"mime_type,omitempty"`
	Protocol    string `json:"protocol,omitempty"`
	RemoteIP    string `json:"remote_ip,omitempty"`
	FromCache   bool   `json:"from_cache,omitempty"`
	EncodedSize *int64 `json:"encoded_size,omitempty"`
	ErrorText   string `json:"error_text,omitempty"`
	Timestamp   string `json:"timestamp,omitempty"`
}

type browserSessionStatus struct {
	Ready          bool   `json:"ready"`
	Mode           string `json:"mode"`
	Endpoint       string `json:"endpoint,omitempty"`
	DebugURL       string `json:"debug_url,omitempty"`
	ManagedProcess bool   `json:"managed_process"`
}

// navigate navigates to a URL.
func (b *BrowserTool) navigate(ctx context.Context, params map[string]interface{}) (string, error) {
	urlStr, ok := params["url"].(string)
	if !ok {
		return "", fmt.Errorf("url parameter is required")
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}
	if !parsedURL.IsAbs() || strings.TrimSpace(parsedURL.Scheme) == "" || strings.TrimSpace(parsedURL.Host) == "" {
		return "", fmt.Errorf("absolute URL is required")
	}

	b.log.Info("Browser navigating",
		zap.String("url", urlStr))

	sessionMgr := GetBrowserSession(b.log)
	opts, err := b.startOptions(params)
	if err != nil {
		return "", err
	}
	if !sessionMgr.IsReady() {
		if err := sessionMgr.StartWithOptions(b.timeout, opts); err != nil {
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

	// Wait for page load using a bounded readyState poll rather than a CDP event
	// stream. This avoids CI flakes where the event listener can hang indefinitely.
	if err := waitForPageReadyState(ctx, client, b.timeout); err != nil {
		return "", fmt.Errorf("failed to wait for page ready state: %w", err)
	}

	return fmt.Sprintf("Navigated to: %s\nFrame ID: %s", urlStr, nav.FrameID), nil
}

func waitForPageReadyState(parent context.Context, client *cdp.Client, timeout time.Duration) error {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	waitCtx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	for {
		evalArgs := runtime.NewEvaluateArgs("document.readyState").SetReturnByValue(true)
		result, err := client.Runtime.Evaluate(waitCtx, evalArgs)
		if err == nil && result.ExceptionDetails == nil {
			var state string
			if unmarshalErr := json.Unmarshal(result.Result.Value, &state); unmarshalErr == nil {
				if state == "interactive" || state == "complete" {
					return nil
				}
			}
		}

		select {
		case <-waitCtx.Done():
			if err != nil {
				return err
			}
			return waitCtx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func (b *BrowserTool) startMode(params map[string]interface{}) (BrowserConnectionMode, error) {
	opts, err := b.startOptions(params)
	if err != nil {
		return "", err
	}
	return opts.Mode, nil
}

func (b *BrowserTool) startOptions(params map[string]interface{}) (BrowserStartOptions, error) {
	rawMode, _ := params["mode"].(string)
	if strings.TrimSpace(rawMode) == "" {
		rawMode = string(BrowserModeAuto)
	}
	mode := resolveBrowserMode(rawMode)
	if mode == BrowserModeAuto && strings.TrimSpace(strings.ToLower(rawMode)) != string(BrowserModeAuto) {
		return BrowserStartOptions{}, fmt.Errorf("invalid browser mode: %s", rawMode)
	}

	var ports []int
	if rawPort, ok := params["debug_port"].(float64); ok {
		port := int(rawPort)
		if float64(port) != rawPort || port <= 0 || port > 65535 {
			return BrowserStartOptions{}, fmt.Errorf("invalid debug_port: %v", rawPort)
		}
		ports = []int{port}
	}

	rawEndpoint, _ := params["debug_endpoint"].(string)
	endpoint, err := normalizeBrowserEndpoint(rawEndpoint)
	if err != nil {
		return BrowserStartOptions{}, err
	}

	return BrowserStartOptions{
		Mode:     mode,
		Ports:    ports,
		Endpoint: endpoint,
	}, nil
}

func (b *BrowserTool) navigationParams(params map[string]interface{}, urlStr string) map[string]interface{} {
	navigateParams := map[string]interface{}{
		"url": urlStr,
	}
	if rawMode, ok := params["mode"].(string); ok && strings.TrimSpace(rawMode) != "" {
		navigateParams["mode"] = rawMode
	}
	if rawPort, ok := params["debug_port"].(float64); ok {
		navigateParams["debug_port"] = rawPort
	}
	if rawEndpoint, ok := params["debug_endpoint"].(string); ok && strings.TrimSpace(rawEndpoint) != "" {
		navigateParams["debug_endpoint"] = rawEndpoint
	}
	return navigateParams
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

func (b *BrowserTool) buildDeviceProfile(params map[string]interface{}) (browserDeviceProfile, error) {
	device, _ := params["device"].(string)
	switch strings.TrimSpace(strings.ToLower(device)) {
	case "iphone":
		return browserDeviceProfile{Name: "iphone", Width: 390, Height: 844, Scale: 3, Mobile: true, UserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1", Platform: "iPhone", Touch: true, MaxTouches: 5}, nil
	case "ipad":
		return browserDeviceProfile{Name: "ipad", Width: 820, Height: 1180, Scale: 2, Mobile: true, UserAgent: "Mozilla/5.0 (iPad; CPU OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1", Platform: "iPad", Touch: true, MaxTouches: 5}, nil
	case "android":
		return browserDeviceProfile{Name: "android", Width: 412, Height: 915, Scale: 2.625, Mobile: true, UserAgent: "Mozilla/5.0 (Linux; Android 14; Pixel 7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Mobile Safari/537.36", Platform: "Android", Touch: true, MaxTouches: 5}, nil
	case "desktop":
		return browserDeviceProfile{Name: "desktop", Width: 1440, Height: 900, Scale: 1, Mobile: false, UserAgent: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36", Platform: "Linux x86_64", Touch: false, MaxTouches: 0}, nil
	default:
		return browserDeviceProfile{}, fmt.Errorf("invalid device: %s", device)
	}
}

func (b *BrowserTool) buildSetCookieArgs(params map[string]interface{}) (*network.SetCookieArgs, error) {
	name := strings.TrimSpace(stringParam(params, "name"))
	if name == "" {
		return nil, fmt.Errorf("name parameter is required")
	}
	value, ok := params["text"].(string)
	if !ok {
		return nil, fmt.Errorf("text parameter is required")
	}
	urlStr := strings.TrimSpace(stringParam(params, "url"))
	if urlStr == "" {
		return nil, fmt.Errorf("url parameter is required")
	}
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	if !parsedURL.IsAbs() || strings.TrimSpace(parsedURL.Scheme) == "" || strings.TrimSpace(parsedURL.Host) == "" {
		return nil, fmt.Errorf("absolute URL is required")
	}
	args := network.NewSetCookieArgs(name, value).SetURL(urlStr)
	if secure, ok := params["secure"].(bool); ok {
		args.SetSecure(secure)
	}
	if httpOnly, ok := params["http_only"].(bool); ok {
		args.SetHTTPOnly(httpOnly)
	}
	switch strings.TrimSpace(strings.ToLower(stringParam(params, "same_site"))) {
	case "":
	case "strict":
		args.SameSite = network.CookieSameSiteStrict
	case "lax":
		args.SameSite = network.CookieSameSiteLax
	case "none":
		args.SameSite = network.CookieSameSiteNone
	default:
		return nil, fmt.Errorf("invalid same_site: %s", stringParam(params, "same_site"))
	}
	return args, nil
}

func (b *BrowserTool) buildViewport(params map[string]interface{}) (int, int, error) {
	rawWidth, ok := params["width"].(float64)
	if !ok {
		return 0, 0, fmt.Errorf("width parameter is required")
	}
	rawHeight, ok := params["height"].(float64)
	if !ok {
		return 0, 0, fmt.Errorf("height parameter is required")
	}
	width := int(rawWidth)
	height := int(rawHeight)
	if rawWidth != float64(width) || width <= 0 {
		return 0, 0, fmt.Errorf("invalid width: %v", rawWidth)
	}
	if rawHeight != float64(height) || height <= 0 {
		return 0, 0, fmt.Errorf("invalid height: %v", rawHeight)
	}
	return width, height, nil
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
		if _, err := b.navigate(ctx, b.navigationParams(params, urlStr)); err != nil {
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
		opts, err := b.startOptions(params)
		if err != nil {
			return "", err
		}
		if err := sessionMgr.StartWithOptions(b.timeout, opts); err != nil {
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
		if _, err := b.navigate(ctx, b.navigationParams(params, urlStr)); err != nil {
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

func (b *BrowserTool) selectOption(ctx context.Context, params map[string]interface{}) (string, error) {
	selector, ok := params["selector"].(string)
	if !ok {
		return "", fmt.Errorf("selector parameter is required")
	}

	value, ok := params["value"].(string)
	if !ok {
		return "", fmt.Errorf("value parameter is required")
	}

	result, err := b.executeScript(ctx, map[string]interface{}{
		"script": b.buildSelectScript(selector, value),
	})
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Selected option %q in %s\n%s", value, selector, result), nil
}

func (b *BrowserTool) buildSelectScript(selector, value string) string {
	return fmt.Sprintf(`(() => {
		const element = document.querySelector(%q);
		if (!element) {
			throw new Error("element not found");
		}
		if (element.tagName !== "SELECT") {
			throw new Error("element is not a select");
		}
		const optionExists = Array.from(element.options).some(option => option.value === %q);
		if (!optionExists) {
			throw new Error("option not found");
		}
		element.value = %q;
		element.dispatchEvent(new Event("input", { bubbles: true }));
		element.dispatchEvent(new Event("change", { bubbles: true }));
		return element.value;
	})()`, selector, value, value)
}

// getHTML gets the HTML content of the page.
func (b *BrowserTool) getHTML(ctx context.Context, params map[string]interface{}) (string, error) {
	if urlStr, ok := params["url"].(string); ok && strings.TrimSpace(urlStr) != "" {
		if _, err := b.navigate(ctx, params); err != nil {
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

func (b *BrowserTool) getSession(ctx context.Context, params map[string]interface{}) (string, error) {
	sessionMgr := GetBrowserSession(b.log)
	status := sessionMgr.Status()
	data, err := json.Marshal(status)
	if err != nil {
		return "", fmt.Errorf("marshal browser session status: %w", err)
	}
	return string(data), nil
}

func (b *BrowserTool) startSession(params map[string]interface{}) (string, error) {
	sessionMgr := GetBrowserSession(b.log)
	if !sessionMgr.IsReady() {
		opts, err := b.startOptions(params)
		if err != nil {
			return "", err
		}
		if err := sessionMgr.StartWithOptions(b.timeout, opts); err != nil {
			return "", fmt.Errorf("failed to start browser: %w", err)
		}
	}

	status := sessionMgr.Status()
	data, err := json.Marshal(status)
	if err != nil {
		return "", fmt.Errorf("marshal browser session status: %w", err)
	}
	return string(data), nil
}

func (b *BrowserTool) resetSession() (string, error) {
	sessionMgr := GetBrowserSession(b.log)
	if err := sessionMgr.Stop(); err != nil {
		return "", err
	}
	return "Browser session reset", nil
}

func (b *BrowserTool) restartSession(params map[string]interface{}) (string, error) {
	sessionMgr := GetBrowserSession(b.log)
	if err := sessionMgr.Stop(); err != nil {
		return "", err
	}
	opts, err := b.startOptions(params)
	if err != nil {
		return "", err
	}
	if err := sessionMgr.StartWithOptions(b.timeout, opts); err != nil {
		return "", fmt.Errorf("failed to restart browser: %w", err)
	}
	return "Browser session restarted", nil
}

func (b *BrowserTool) getNetwork(ctx context.Context, params map[string]interface{}) (string, error) {
	if urlStr, ok := params["url"].(string); ok && strings.TrimSpace(urlStr) != "" {
		if _, err := b.navigate(ctx, params); err != nil {
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
	if err := client.Network.Enable(ctx, nil); err != nil {
		return "", fmt.Errorf("failed to enable browser network domain: %w", err)
	}

	requests, err := client.Network.RequestWillBeSent(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to subscribe to network requests: %w", err)
	}
	defer requests.Close()
	responses, err := client.Network.ResponseReceived(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to subscribe to network responses: %w", err)
	}
	defer responses.Close()
	finished, err := client.Network.LoadingFinished(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to subscribe to network completion: %w", err)
	}
	defer finished.Close()
	failed, err := client.Network.LoadingFailed(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to subscribe to network failures: %w", err)
	}
	defer failed.Close()

	opts := b.buildNetworkOptions(params)
	entries := collectBrowserNetworkEntries(ctx, requests, responses, finished, failed, opts)
	data, err := json.Marshal(entries)
	if err != nil {
		return "", fmt.Errorf("marshal browser network entries: %w", err)
	}
	return string(data), nil
}

func (b *BrowserTool) getConsole(ctx context.Context, params map[string]interface{}) (string, error) {
	if urlStr, ok := params["url"].(string); ok && strings.TrimSpace(urlStr) != "" {
		if _, err := b.navigate(ctx, params); err != nil {
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

	opts := b.buildConsoleOptions(params)
	if err := client.Log.Enable(ctx); err != nil {
		return "", fmt.Errorf("failed to enable browser log domain: %w", err)
	}
	if err := client.Runtime.Enable(ctx); err != nil {
		return "", fmt.Errorf("failed to enable browser runtime domain: %w", err)
	}

	entryAdded, err := client.Log.EntryAdded(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to subscribe to browser log entries: %w", err)
	}
	defer entryAdded.Close()

	consoleCalled, err := client.Runtime.ConsoleAPICalled(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to subscribe to browser console api calls: %w", err)
	}
	defer consoleCalled.Close()

	timeout := 500 * time.Millisecond
	if deadline, ok := ctx.Deadline(); ok {
		if remaining := time.Until(deadline); remaining > 0 && remaining < timeout {
			timeout = remaining
		}
	}

	entries := collectBrowserConsoleEntries(ctx, entryAdded, consoleCalled, opts.MaxEntries, timeout)
	filtered := make([]browserConsoleEntry, 0, len(entries))
	for _, entry := range entries {
		if !browserConsoleEntryMatches(entry.Level, opts) {
			continue
		}
		filtered = append(filtered, entry)
		if len(filtered) >= opts.MaxEntries {
			break
		}
	}

	data, err := json.Marshal(filtered)
	if err != nil {
		return "", fmt.Errorf("marshal browser console entries: %w", err)
	}
	return string(data), nil
}

func (b *BrowserTool) getText(ctx context.Context, params map[string]interface{}) (string, error) {
	if urlStr, ok := params["url"].(string); ok && strings.TrimSpace(urlStr) != "" {
		if _, err := b.navigate(ctx, params); err != nil {
			return "", err
		}
	}

	html, err := b.getHTML(ctx, params)
	if err != nil {
		return "", err
	}

	return htmlToText(html), nil
}

func (b *BrowserTool) getTitle(ctx context.Context, params map[string]interface{}) (string, error) {
	if urlStr, ok := params["url"].(string); ok && strings.TrimSpace(urlStr) != "" {
		if _, err := b.navigate(ctx, params); err != nil {
			return "", err
		}
	}

	result, err := b.executeScript(ctx, map[string]interface{}{
		"script": "document.title",
	})
	if err != nil {
		return "", err
	}
	const prefix = "Script executed successfully\nResult: "
	return strings.TrimPrefix(result, prefix), nil
}

func (b *BrowserTool) getURL(ctx context.Context, params map[string]interface{}) (string, error) {
	if urlStr, ok := params["url"].(string); ok && strings.TrimSpace(urlStr) != "" {
		if _, err := b.navigate(ctx, params); err != nil {
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

	frameTree, err := client.Page.GetFrameTree(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get frame tree: %w", err)
	}
	return frameTree.FrameTree.Frame.URL, nil
}

func (b *BrowserTool) getLinks(ctx context.Context, params map[string]interface{}) (string, error) {
	if urlStr, ok := params["url"].(string); ok && strings.TrimSpace(urlStr) != "" {
		if _, err := b.navigate(ctx, params); err != nil {
			return "", err
		}
	}

	result, err := b.executeScript(ctx, map[string]interface{}{
		"script": `JSON.stringify(Array.from(document.querySelectorAll('a[href]')).map(a => ({
  text: (a.textContent || '').trim(),
  href: a.href
})))`,
	})
	if err != nil {
		return "", err
	}
	const prefix = "Script executed successfully\nResult: "
	return strings.TrimPrefix(result, prefix), nil
}

func (b *BrowserTool) getCookies(ctx context.Context, params map[string]interface{}) (string, error) {
	if urlStr, ok := params["url"].(string); ok && strings.TrimSpace(urlStr) != "" {
		if _, err := b.navigate(ctx, params); err != nil {
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

	cookies, err := client.Network.GetCookies(ctx, network.NewGetCookiesArgs())
	if err != nil {
		return "", fmt.Errorf("failed to get cookies: %w", err)
	}

	result := make([]map[string]any, 0, len(cookies.Cookies))
	for _, cookie := range cookies.Cookies {
		result = append(result, map[string]any{
			"name":   cookie.Name,
			"value":  cookie.Value,
			"domain": cookie.Domain,
			"path":   cookie.Path,
		})
	}
	data, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("marshal cookies: %w", err)
	}
	return string(data), nil
}

func (b *BrowserTool) setCookie(ctx context.Context, params map[string]interface{}) (string, error) {
	args, err := b.buildSetCookieArgs(params)
	if err != nil {
		return "", err
	}
	sessionMgr := GetBrowserSession(b.log)
	if !sessionMgr.IsReady() {
		opts, err := b.startOptions(params)
		if err != nil {
			return "", err
		}
		if err := sessionMgr.StartWithOptions(b.timeout, opts); err != nil {
			return "", fmt.Errorf("failed to start browser: %w", err)
		}
	}
	client, err := sessionMgr.GetClient()
	if err != nil {
		return "", err
	}
	reply, err := client.Network.SetCookie(ctx, args)
	if err != nil {
		return "", fmt.Errorf("failed to set cookie: %w", err)
	}
	if reply != nil && !reply.Success {
		return "", fmt.Errorf("failed to set cookie")
	}
	payload := map[string]any{"name": args.Name, "url": *args.URL}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal cookie result: %w", err)
	}
	return string(data), nil
}

func (b *BrowserTool) clearCookies(ctx context.Context, params map[string]interface{}) (string, error) {
	sessionMgr := GetBrowserSession(b.log)
	if !sessionMgr.IsReady() {
		opts, err := b.startOptions(params)
		if err != nil {
			return "", err
		}
		if err := sessionMgr.StartWithOptions(b.timeout, opts); err != nil {
			return "", fmt.Errorf("failed to start browser: %w", err)
		}
	}
	client, err := sessionMgr.GetClient()
	if err != nil {
		return "", err
	}
	if err := client.Network.ClearBrowserCookies(ctx); err != nil {
		return "", fmt.Errorf("failed to clear cookies: %w", err)
	}
	return "Browser cookies cleared", nil
}

func (b *BrowserTool) getMeta(ctx context.Context, params map[string]interface{}) (string, error) {
	if urlStr, ok := params["url"].(string); ok && strings.TrimSpace(urlStr) != "" {
		if _, err := b.navigate(ctx, params); err != nil {
			return "", err
		}
	}

	result, err := b.executeScript(ctx, map[string]interface{}{
		"script": `JSON.stringify(Array.from(document.querySelectorAll('meta')).reduce((acc, tag) => {
  const key = tag.getAttribute('name') || tag.getAttribute('property');
  const value = tag.getAttribute('content');
  if (key && value) acc[key] = value;
  return acc;
}, {}))`,
	})
	if err != nil {
		return "", err
	}
	const prefix = "Script executed successfully\nResult: "
	return strings.TrimPrefix(result, prefix), nil
}

func (b *BrowserTool) getImages(ctx context.Context, params map[string]interface{}) (string, error) {
	if urlStr, ok := params["url"].(string); ok && strings.TrimSpace(urlStr) != "" {
		if _, err := b.navigate(ctx, params); err != nil {
			return "", err
		}
	}

	result, err := b.executeScript(ctx, map[string]interface{}{
		"script": `JSON.stringify(Array.from(document.querySelectorAll('img')).map(img => ({
  alt: (img.getAttribute('alt') || '').trim(),
  src: img.src
})))`,
	})
	if err != nil {
		return "", err
	}
	const prefix = "Script executed successfully\nResult: "
	return strings.TrimPrefix(result, prefix), nil
}

func (b *BrowserTool) getForms(ctx context.Context, params map[string]interface{}) (string, error) {
	if urlStr, ok := params["url"].(string); ok && strings.TrimSpace(urlStr) != "" {
		if _, err := b.navigate(ctx, params); err != nil {
			return "", err
		}
	}

	result, err := b.executeScript(ctx, map[string]interface{}{
		"script": `JSON.stringify(Array.from(document.querySelectorAll('form')).map((form, idx) => {
  const fields = Array.from(form.querySelectorAll('input, select, textarea')).map(field => {
    const info = {
      tag: field.tagName.toLowerCase(),
      type: field.type || null,
      name: field.name || null,
      id: field.id || null,
      value: field.value || null,
      placeholder: field.placeholder || null,
      required: field.required || false
    };
    if (field.tagName === 'SELECT') {
      info.options = Array.from(field.options).map(opt => ({
        value: opt.value,
        text: opt.textContent.trim(),
        selected: opt.selected
      }));
    }
    return info;
  });
  return {
    index: idx,
    id: form.id || null,
    name: form.name || null,
    action: form.action || null,
    method: form.method || 'get',
    fields: fields
  };
})))`,
	})
	if err != nil {
		return "", err
	}
	const prefix = "Script executed successfully\nResult: "
	return strings.TrimPrefix(result, prefix), nil
}

func (b *BrowserTool) getButtons(ctx context.Context, params map[string]interface{}) (string, error) {
	if urlStr, ok := params["url"].(string); ok && strings.TrimSpace(urlStr) != "" {
		if _, err := b.navigate(ctx, params); err != nil {
			return "", err
		}
	}

	result, err := b.executeScript(ctx, map[string]interface{}{
		"script": `JSON.stringify(Array.from(document.querySelectorAll('button, input[type="button"], input[type="submit"], input[type="reset"]')).map((btn, idx) => ({
  index: idx,
  tag: btn.tagName.toLowerCase(),
  type: btn.type || 'button',
  id: btn.id || null,
  name: btn.name || null,
  text: btn.tagName === 'BUTTON' ? (btn.textContent || '').trim() : null,
  value: btn.value || null,
  disabled: btn.disabled || false
})))`,
	})
	if err != nil {
		return "", err
	}
	const prefix = "Script executed successfully\nResult: "
	return strings.TrimPrefix(result, prefix), nil
}

func (b *BrowserTool) getTables(ctx context.Context, params map[string]interface{}) (string, error) {
	if urlStr, ok := params["url"].(string); ok && strings.TrimSpace(urlStr) != "" {
		if _, err := b.navigate(ctx, params); err != nil {
			return "", err
		}
	}

	result, err := b.executeScript(ctx, map[string]interface{}{
		"script": `JSON.stringify(Array.from(document.querySelectorAll('table')).map((table, idx) => {
  const headers = Array.from(table.querySelectorAll('th')).map(th => ({
    text: (th.textContent || '').trim(),
    scope: th.scope || null
  }));
  const rows = Array.from(table.querySelectorAll('tr')).map(tr =>
    Array.from(tr.querySelectorAll('td')).map(td => ({
      text: (td.textContent || '').trim(),
      colSpan: td.colSpan || 1,
      rowSpan: td.rowSpan || 1
    }))
  ).filter(row => row.length > 0);
  return {
    index: idx,
    id: table.id || null,
    caption: table.querySelector('caption') ? (table.querySelector('caption').textContent || '').trim() : null,
    headers: headers,
    rows: rows,
    rowCount: rows.length,
    columnCount: headers.length || (rows[0] ? rows[0].length : 0)
  };
})))`,
	})
	if err != nil {
		return "", err
	}
	const prefix = "Script executed successfully\nResult: "
	return strings.TrimPrefix(result, prefix), nil
}

func (b *BrowserTool) getLists(ctx context.Context, params map[string]interface{}) (string, error) {
	if urlStr, ok := params["url"].(string); ok && strings.TrimSpace(urlStr) != "" {
		if _, err := b.navigate(ctx, params); err != nil {
			return "", err
		}
	}

	result, err := b.executeScript(ctx, map[string]interface{}{
		"script": `JSON.stringify(Array.from(document.querySelectorAll('ul, ol')).map((list, idx) => {
  const items = Array.from(list.querySelectorAll(':scope > li')).map(li => ({
    text: (li.textContent || '').trim(),
    nested: li.querySelector('ul, ol') ? true : false
  }));
  return {
    index: idx,
    tag: list.tagName.toLowerCase(),
    id: list.id || null,
    class: list.className || null,
    itemCount: items.length,
    items: items
  };
})))`,
	})
	if err != nil {
		return "", err
	}
	const prefix = "Script executed successfully\nResult: "
	return strings.TrimPrefix(result, prefix), nil
}

func (b *BrowserTool) getInputs(ctx context.Context, params map[string]interface{}) (string, error) {
	if urlStr, ok := params["url"].(string); ok && strings.TrimSpace(urlStr) != "" {
		if _, err := b.navigate(ctx, params); err != nil {
			return "", err
		}
	}

	result, err := b.executeScript(ctx, map[string]interface{}{
		"script": `JSON.stringify(Array.from(document.querySelectorAll('input, textarea, select')).map((input, idx) => {
  const info = {
    index: idx,
    tag: input.tagName.toLowerCase(),
    type: input.type || null,
    id: input.id || null,
    name: input.name || null,
    value: input.value || null,
    placeholder: input.placeholder || null,
    required: input.required || false,
    disabled: input.disabled || false,
    readonly: input.readOnly || false
  };
  if (input.tagName === 'SELECT') {
    info.options = Array.from(input.options).map(opt => ({
      value: opt.value,
      text: opt.textContent.trim(),
      selected: opt.selected
    }));
  }
  return info;
})))`,
	})
	if err != nil {
		return "", err
	}
	const prefix = "Script executed successfully\nResult: "
	return strings.TrimPrefix(result, prefix), nil
}

func (b *BrowserTool) getSelects(ctx context.Context, params map[string]interface{}) (string, error) {
	if urlStr, ok := params["url"].(string); ok && strings.TrimSpace(urlStr) != "" {
		if _, err := b.navigate(ctx, params); err != nil {
			return "", err
		}
	}

	result, err := b.executeScript(ctx, map[string]interface{}{
		"script": `JSON.stringify(Array.from(document.querySelectorAll('select')).map((select, idx) => ({
  index: idx,
  id: select.id || null,
  name: select.name || null,
  multiple: select.multiple || false,
  disabled: select.disabled || false,
  required: select.required || false,
  size: select.size || 1,
  selectedCount: Array.from(select.options).filter(opt => opt.selected).length,
  options: Array.from(select.options).map(opt => ({
    value: opt.value,
    text: opt.textContent.trim(),
    selected: opt.selected,
    disabled: opt.disabled,
    index: opt.index
  }))
})))`,
	})
	if err != nil {
		return "", err
	}
	const prefix = "Script executed successfully\nResult: "
	return strings.TrimPrefix(result, prefix), nil
}

func (b *BrowserTool) getTextareas(ctx context.Context, params map[string]interface{}) (string, error) {
	if urlStr, ok := params["url"].(string); ok && strings.TrimSpace(urlStr) != "" {
		if _, err := b.navigate(ctx, params); err != nil {
			return "", err
		}
	}

	result, err := b.executeScript(ctx, map[string]interface{}{
		"script": `JSON.stringify(Array.from(document.querySelectorAll('textarea')).map((textarea, idx) => ({
  index: idx,
  id: textarea.id || null,
  name: textarea.name || null,
  rows: textarea.rows || null,
  cols: textarea.cols || null,
  maxlength: textarea.maxLength || null,
  placeholder: textarea.placeholder || null,
  value: textarea.value || null,
  textLength: (textarea.value || '').length,
  required: textarea.required || false,
  disabled: textarea.disabled || false,
  readonly: textarea.readOnly || false
})))`,
	})
	if err != nil {
		return "", err
	}
	const prefix = "Script executed successfully\nResult: "
	return strings.TrimPrefix(result, prefix), nil
}

func (b *BrowserTool) getHeadings(ctx context.Context, params map[string]interface{}) (string, error) {
	if urlStr, ok := params["url"].(string); ok && strings.TrimSpace(urlStr) != "" {
		if _, err := b.navigate(ctx, params); err != nil {
			return "", err
		}
	}

	result, err := b.executeScript(ctx, map[string]interface{}{
		"script": `JSON.stringify(Array.from(document.querySelectorAll('h1,h2,h3,h4,h5,h6')).map(h => ({
  level: h.tagName.toLowerCase(),
  text: (h.textContent || '').trim()
})))`,
	})
	if err != nil {
		return "", err
	}
	const prefix = "Script executed successfully\nResult: "
	return strings.TrimPrefix(result, prefix), nil
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

func (b *BrowserTool) getMetrics(ctx context.Context, params map[string]interface{}) (string, error) {
	if urlStr, ok := params["url"].(string); ok && strings.TrimSpace(urlStr) != "" {
		if _, err := b.navigate(ctx, params); err != nil {
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
	if err := client.Performance.Enable(ctx, performance.NewEnableArgs()); err != nil {
		return "", fmt.Errorf("failed to enable performance metrics: %w", err)
	}
	metrics, err := client.Performance.GetMetrics(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get performance metrics: %w", err)
	}
	payload := make(map[string]float64, len(metrics.Metrics))
	for _, metric := range metrics.Metrics {
		payload[metric.Name] = metric.Value
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal metrics: %w", err)
	}
	return string(data), nil
}

func (b *BrowserTool) emulateDevice(ctx context.Context, params map[string]interface{}) (string, error) {
	profile, err := b.buildDeviceProfile(params)
	if err != nil {
		return "", err
	}
	sessionMgr := GetBrowserSession(b.log)
	if !sessionMgr.IsReady() {
		opts, err := b.startOptions(params)
		if err != nil {
			return "", err
		}
		if err := sessionMgr.StartWithOptions(b.timeout, opts); err != nil {
			return "", fmt.Errorf("failed to start browser: %w", err)
		}
	}
	client, err := sessionMgr.GetClient()
	if err != nil {
		return "", err
	}
	if err := client.Emulation.SetDeviceMetricsOverride(ctx, emulation.NewSetDeviceMetricsOverrideArgs(profile.Width, profile.Height, profile.Scale, profile.Mobile)); err != nil {
		return "", fmt.Errorf("failed to set device metrics override: %w", err)
	}
	if err := client.Emulation.SetUserAgentOverride(ctx, emulation.NewSetUserAgentOverrideArgs(profile.UserAgent).SetPlatform(profile.Platform)); err != nil {
		return "", fmt.Errorf("failed to set user agent override: %w", err)
	}
	if err := client.Emulation.SetTouchEmulationEnabled(ctx, emulation.NewSetTouchEmulationEnabledArgs(profile.Touch).SetMaxTouchPoints(profile.MaxTouches)); err != nil {
		return "", fmt.Errorf("failed to set touch emulation: %w", err)
	}
	payload := map[string]any{"device": profile.Name, "width": profile.Width, "height": profile.Height, "scale": profile.Scale, "mobile": profile.Mobile, "platform": profile.Platform, "touch": profile.Touch, "max_touch_points": profile.MaxTouches}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal emulation profile: %w", err)
	}
	return string(data), nil
}

func (b *BrowserTool) setViewport(ctx context.Context, params map[string]interface{}) (string, error) {
	width, height, err := b.buildViewport(params)
	if err != nil {
		return "", err
	}
	sessionMgr := GetBrowserSession(b.log)
	if !sessionMgr.IsReady() {
		opts, err := b.startOptions(params)
		if err != nil {
			return "", err
		}
		if err := sessionMgr.StartWithOptions(b.timeout, opts); err != nil {
			return "", fmt.Errorf("failed to start browser: %w", err)
		}
	}
	client, err := sessionMgr.GetClient()
	if err != nil {
		return "", err
	}
	if err := client.Emulation.SetDeviceMetricsOverride(ctx, emulation.NewSetDeviceMetricsOverrideArgs(width, height, 1.0, false)); err != nil {
		return "", fmt.Errorf("failed to set viewport: %w", err)
	}
	payload := map[string]any{"width": width, "height": height}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal viewport: %w", err)
	}
	return string(data), nil
}

func (b *BrowserTool) listPages(ctx context.Context, params map[string]interface{}) (string, error) {
	devtools, err := b.ensureDevTools(ctx, params)
	if err != nil {
		return "", err
	}

	targets, err := devtools.List(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list browser pages: %w", err)
	}

	result := make([]map[string]any, 0, len(targets))
	for _, target := range targets {
		if target == nil {
			continue
		}
		result = append(result, map[string]any{
			"id":       target.ID,
			"type":     string(target.Type),
			"title":    target.Title,
			"url":      target.URL,
			"devtools": target.DevToolsFrontendURL,
			"ws_debug": target.WebSocketDebuggerURL,
		})
	}

	data, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("marshal browser pages: %w", err)
	}
	return string(data), nil
}

func (b *BrowserTool) newPage(ctx context.Context, params map[string]interface{}) (string, error) {
	devtools, err := b.ensureDevTools(ctx, params)
	if err != nil {
		return "", err
	}

	target, err := b.createPageTarget(ctx, devtools, stringParam(params, "url"))
	if err != nil {
		return "", err
	}
	return marshalBrowserTarget(target)
}

func (b *BrowserTool) activatePage(ctx context.Context, params map[string]interface{}) (string, error) {
	devtools, err := b.ensureDevTools(ctx, params)
	if err != nil {
		return "", err
	}

	target, err := requireTargetID(params)
	if err != nil {
		return "", err
	}
	if err := devtools.Activate(ctx, target); err != nil {
		return "", fmt.Errorf("failed to activate browser page %s: %w", target.ID, err)
	}
	return fmt.Sprintf("Activated browser page: %s", target.ID), nil
}

func (b *BrowserTool) closePage(ctx context.Context, params map[string]interface{}) (string, error) {
	devtools, err := b.ensureDevTools(ctx, params)
	if err != nil {
		return "", err
	}

	target, err := requireTargetID(params)
	if err != nil {
		return "", err
	}
	if err := devtools.Close(ctx, target); err != nil {
		return "", fmt.Errorf("failed to close browser page %s: %w", target.ID, err)
	}
	return fmt.Sprintf("Closed browser page: %s", target.ID), nil
}

func (b *BrowserTool) ensureDevTools(ctx context.Context, params map[string]interface{}) (browserDevTools, error) {
	sessionMgr := GetBrowserSession(b.log)
	if !sessionMgr.IsReady() {
		opts, err := b.startOptions(params)
		if err != nil {
			return nil, err
		}
		if err := sessionMgr.StartWithOptions(b.timeout, opts); err != nil {
			return nil, fmt.Errorf("failed to start browser: %w", err)
		}
	}
	return sessionMgr.GetDevTools()
}

func (b *BrowserTool) createPageTarget(ctx context.Context, devtools browserDevTools, rawURL string) (*devtool.Target, error) {
	if strings.TrimSpace(rawURL) == "" {
		target, err := devtools.Create(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to create browser page: %w", err)
		}
		return target, nil
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	if !parsedURL.IsAbs() || strings.TrimSpace(parsedURL.Scheme) == "" || strings.TrimSpace(parsedURL.Host) == "" {
		return nil, fmt.Errorf("absolute URL is required")
	}

	target, err := devtools.CreateURL(ctx, rawURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create browser page: %w", err)
	}
	return target, nil
}

func (b *BrowserTool) browserStorageAction(ctx context.Context, params map[string]interface{}) (string, error) {
	action, _ := params["action"].(string)
	script, err := b.buildStorageScript(action, params)
	if err != nil {
		return "", err
	}
	return b.executeScript(ctx, map[string]interface{}{"script": script})
}

func (b *BrowserTool) buildStorageScript(action string, params map[string]interface{}) (string, error) {
	storageScope := stringParam(params, "storage")
	if storageScope == "" {
		return "", fmt.Errorf("storage parameter is required")
	}

	var storageExpr string
	switch storageScope {
	case "local":
		storageExpr = "window.localStorage"
	case "session":
		storageExpr = "window.sessionStorage"
	default:
		return "", fmt.Errorf("invalid storage: %s", storageScope)
	}

	switch action {
	case "get_storage":
		key := stringParam(params, "key")
		if key != "" {
			return fmt.Sprintf(`(() => {
  const storage = %s;
  return JSON.stringify({key: %q, value: storage.getItem(%q)});
})()`, storageExpr, key, key), nil
		}
		return fmt.Sprintf(`(() => {
  const storage = %s;
  const out = {};
  Object.keys(storage).forEach(key => {
    out[key] = storage.getItem(key);
  });
  return JSON.stringify(out);
})()`, storageExpr), nil
	case "set_storage":
		key := stringParam(params, "key")
		if key == "" {
			return "", fmt.Errorf("key parameter is required")
		}
		value, ok := params["value"].(string)
		if !ok {
			return "", fmt.Errorf("value parameter is required")
		}
		return fmt.Sprintf(`(() => {
  const storage = %s;
  storage.setItem(%q, %q);
  return storage.getItem(%q);
})()`, storageExpr, key, value, key), nil
	case "remove_storage":
		key := stringParam(params, "key")
		if key == "" {
			return "", fmt.Errorf("key parameter is required")
		}
		return fmt.Sprintf(`(() => {
  const storage = %s;
  storage.removeItem(%q);
  return JSON.stringify({removed: %q});
})()`, storageExpr, key, key), nil
	case "clear_storage":
		return fmt.Sprintf(`(() => {
  const storage = %s;
  storage.clear();
  return JSON.stringify({cleared: true});
})()`, storageExpr), nil
	default:
		return "", fmt.Errorf("unsupported storage action: %s", action)
	}
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

func htmlToText(html string) string {
	var text strings.Builder
	inTag := false

	for i := 0; i < len(html); i++ {
		switch html[i] {
		case '<':
			inTag = true
		case '>':
			inTag = false
		default:
			if !inTag {
				text.WriteByte(html[i])
			}
		}
	}

	return text.String()
}

func stringParam(params map[string]interface{}, key string) string {
	value, _ := params[key].(string)
	return strings.TrimSpace(value)
}

func requireTargetID(params map[string]interface{}) (*devtool.Target, error) {
	targetID := stringParam(params, "target_id")
	if targetID == "" {
		return nil, fmt.Errorf("target_id parameter is required")
	}
	return &devtool.Target{ID: targetID}, nil
}

func marshalBrowserTarget(target *devtool.Target) (string, error) {
	if target == nil {
		return "", fmt.Errorf("browser page target unavailable")
	}

	data, err := json.Marshal(map[string]any{
		"id":       target.ID,
		"type":     string(target.Type),
		"title":    target.Title,
		"url":      target.URL,
		"devtools": target.DevToolsFrontendURL,
		"ws_debug": target.WebSocketDebuggerURL,
	})
	if err != nil {
		return "", fmt.Errorf("marshal browser page: %w", err)
	}
	return string(data), nil
}

func (b *BrowserTool) buildNetworkOptions(params map[string]interface{}) browserNetworkOptions {
	opts := browserNetworkOptions{
		MaxEntries: 100,
		Duration:   500 * time.Millisecond,
	}
	if value, ok := params["max_entries"].(float64); ok {
		maxEntries := int(value)
		if value == float64(maxEntries) && maxEntries > 0 {
			opts.MaxEntries = maxEntries
		}
	}
	if value, ok := params["duration"].(float64); ok {
		duration := int(value)
		if value == float64(duration) && duration > 0 {
			opts.Duration = time.Duration(duration) * time.Millisecond
		}
	}
	opts.ResourceType = strings.TrimSpace(strings.ToLower(stringParam(params, "resource_type")))
	return opts
}

func (b *BrowserTool) buildConsoleOptions(params map[string]interface{}) browserConsoleOptions {
	opts := browserConsoleOptions{MaxEntries: 100}
	if value, ok := params["errors_only"].(bool); ok {
		opts.ErrorsOnly = value
	}
	if value, ok := params["warnings_only"].(bool); ok {
		opts.WarningsOnly = value
	}
	if value, ok := params["info_only"].(bool); ok {
		opts.InfoOnly = value
	}
	if value, ok := params["max_entries"].(float64); ok {
		maxEntries := int(value)
		if value == float64(maxEntries) && maxEntries > 0 {
			opts.MaxEntries = maxEntries
		}
	}
	return opts
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

func browserNetworkEntryFromRequest(entry *network.RequestWillBeSentReply) browserNetworkEntry {
	if entry == nil {
		return browserNetworkEntry{}
	}
	return browserNetworkEntry{
		RequestID: string(entry.RequestID),
		Phase:     "request",
		Type:      strings.TrimSpace(entry.Type.String()),
		Method:    strings.TrimSpace(entry.Request.Method),
		URL:       strings.TrimSpace(entry.Request.URL),
		Timestamp: browserMonotonicTimestampString(entry.Timestamp),
	}
}

func browserNetworkEntryFromResponse(entry *network.ResponseReceivedReply) browserNetworkEntry {
	if entry == nil {
		return browserNetworkEntry{}
	}
	status := entry.Response.Status
	result := browserNetworkEntry{
		RequestID:  string(entry.RequestID),
		Phase:      "response",
		Type:       strings.TrimSpace(entry.Type.String()),
		URL:        strings.TrimSpace(entry.Response.URL),
		Status:     &status,
		StatusText: strings.TrimSpace(entry.Response.StatusText),
		MimeType:   strings.TrimSpace(entry.Response.MimeType),
		Timestamp:  browserMonotonicTimestampString(entry.Timestamp),
	}
	if entry.Response.Protocol != nil {
		result.Protocol = strings.TrimSpace(*entry.Response.Protocol)
	}
	if entry.Response.RemoteIPAddress != nil {
		result.RemoteIP = strings.TrimSpace(*entry.Response.RemoteIPAddress)
	}
	if entry.Response.FromDiskCache != nil && *entry.Response.FromDiskCache {
		result.FromCache = true
	}
	if entry.Response.FromPrefetchCache != nil && *entry.Response.FromPrefetchCache {
		result.FromCache = true
	}
	return result
}

func browserNetworkEntryFromFinished(entry *network.LoadingFinishedReply) browserNetworkEntry {
	if entry == nil {
		return browserNetworkEntry{}
	}
	size := int64(entry.EncodedDataLength)
	return browserNetworkEntry{
		RequestID:   string(entry.RequestID),
		Phase:       "finished",
		EncodedSize: &size,
		Timestamp:   browserMonotonicTimestampString(entry.Timestamp),
	}
}

func browserNetworkEntryFromFailed(entry *network.LoadingFailedReply) browserNetworkEntry {
	if entry == nil {
		return browserNetworkEntry{}
	}
	return browserNetworkEntry{
		RequestID: string(entry.RequestID),
		Phase:     "failed",
		Type:      strings.TrimSpace(entry.Type.String()),
		ErrorText: strings.TrimSpace(entry.ErrorText),
		Timestamp: browserMonotonicTimestampString(entry.Timestamp),
	}
}

func browserNetworkEntryMatches(entry browserNetworkEntry, opts browserNetworkOptions) bool {
	if strings.TrimSpace(opts.ResourceType) == "" {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(entry.Type), strings.TrimSpace(opts.ResourceType))
}

func browserMonotonicTimestampString(ts network.MonotonicTime) string {
	value := float64(ts)
	if value <= 0 {
		return ""
	}
	return time.Unix(0, int64(value*float64(time.Second))).UTC().Format(time.RFC3339Nano)
}

func collectBrowserNetworkEntries(
	ctx context.Context,
	requests network.RequestWillBeSentClient,
	responses network.ResponseReceivedClient,
	finished network.LoadingFinishedClient,
	failed network.LoadingFailedClient,
	opts browserNetworkOptions,
) []browserNetworkEntry {
	if opts.MaxEntries <= 0 {
		opts.MaxEntries = 100
	}
	if opts.Duration <= 0 {
		opts.Duration = 500 * time.Millisecond
	}

	deadline := time.Now().Add(opts.Duration)
	entries := make([]browserNetworkEntry, 0, min(opts.MaxEntries, 16))
	for len(entries) < opts.MaxEntries && time.Now().Before(deadline) {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		wait := remaining
		if wait > 25*time.Millisecond {
			wait = 25 * time.Millisecond
		}
		if requests != nil {
			if item, ok := recvNetworkRequestWithin(ctx, requests, wait); ok && item != nil {
				entry := browserNetworkEntryFromRequest(item)
				if browserNetworkEntryMatches(entry, opts) {
					entries = append(entries, entry)
				}
			}
		}
		if len(entries) >= opts.MaxEntries || time.Now().After(deadline) {
			break
		}
		if responses != nil {
			if item, ok := recvNetworkResponseWithin(ctx, responses, wait); ok && item != nil {
				entry := browserNetworkEntryFromResponse(item)
				if browserNetworkEntryMatches(entry, opts) {
					entries = append(entries, entry)
				}
			}
		}
		if len(entries) >= opts.MaxEntries || time.Now().After(deadline) {
			break
		}
		if finished != nil {
			if item, ok := recvNetworkFinishedWithin(ctx, finished, wait); ok && item != nil {
				entry := browserNetworkEntryFromFinished(item)
				if browserNetworkEntryMatches(entry, opts) {
					entries = append(entries, entry)
				}
			}
		}
		if len(entries) >= opts.MaxEntries || time.Now().After(deadline) {
			break
		}
		if failed != nil {
			if item, ok := recvNetworkFailedWithin(ctx, failed, wait); ok && item != nil {
				entry := browserNetworkEntryFromFailed(item)
				if browserNetworkEntryMatches(entry, opts) {
					entries = append(entries, entry)
				}
			}
		}
	}
	return entries
}

func recvNetworkRequestWithin(ctx context.Context, client network.RequestWillBeSentClient, wait time.Duration) (*network.RequestWillBeSentReply, bool) {
	type result struct {
		item *network.RequestWillBeSentReply
		err  error
	}
	ch := make(chan result, 1)
	go func() { item, err := client.Recv(); ch <- result{item: item, err: err} }()
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return nil, false
	case <-timer.C:
		return nil, false
	case res := <-ch:
		return res.item, res.err == nil
	}
}

func recvNetworkResponseWithin(ctx context.Context, client network.ResponseReceivedClient, wait time.Duration) (*network.ResponseReceivedReply, bool) {
	type result struct {
		item *network.ResponseReceivedReply
		err  error
	}
	ch := make(chan result, 1)
	go func() { item, err := client.Recv(); ch <- result{item: item, err: err} }()
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return nil, false
	case <-timer.C:
		return nil, false
	case res := <-ch:
		return res.item, res.err == nil
	}
}

func recvNetworkFinishedWithin(ctx context.Context, client network.LoadingFinishedClient, wait time.Duration) (*network.LoadingFinishedReply, bool) {
	type result struct {
		item *network.LoadingFinishedReply
		err  error
	}
	ch := make(chan result, 1)
	go func() { item, err := client.Recv(); ch <- result{item: item, err: err} }()
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return nil, false
	case <-timer.C:
		return nil, false
	case res := <-ch:
		return res.item, res.err == nil
	}
}

func recvNetworkFailedWithin(ctx context.Context, client network.LoadingFailedClient, wait time.Duration) (*network.LoadingFailedReply, bool) {
	type result struct {
		item *network.LoadingFailedReply
		err  error
	}
	ch := make(chan result, 1)
	go func() { item, err := client.Recv(); ch <- result{item: item, err: err} }()
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return nil, false
	case <-timer.C:
		return nil, false
	case res := <-ch:
		return res.item, res.err == nil
	}
}

func browserConsoleEntryMatches(level string, opts browserConsoleOptions) bool {
	normalized := strings.TrimSpace(strings.ToLower(level))
	switch {
	case opts.ErrorsOnly:
		return normalized == "error"
	case opts.WarningsOnly:
		return normalized == "warning"
	case opts.InfoOnly:
		return normalized == "info" || normalized == "log"
	default:
		return true
	}
}

func browserTimestampString(ts runtime.Timestamp) string {
	value := float64(ts)
	if value <= 0 {
		return ""
	}
	return time.Unix(0, int64(value*float64(time.Millisecond))).UTC().Format(time.RFC3339Nano)
}

func browserConsoleArgs(args []runtime.RemoteObject) []string {
	if len(args) == 0 {
		return nil
	}
	values := make([]string, 0, len(args))
	for _, arg := range args {
		value, err := formatCDPResult(&arg)
		if err != nil {
			continue
		}
		if strings.TrimSpace(value) == "" {
			continue
		}
		values = append(values, value)
	}
	return values
}

func browserConsoleText(args []string, fallback string) string {
	text := strings.TrimSpace(strings.Join(args, " "))
	if text != "" {
		return text
	}
	return strings.TrimSpace(fallback)
}

func browserConsoleEntryFromLog(entry *cdpLog.EntryAddedReply) browserConsoleEntry {
	if entry == nil {
		return browserConsoleEntry{}
	}
	result := browserConsoleEntry{
		Source:    strings.TrimSpace(entry.Entry.Source),
		Level:     strings.TrimSpace(entry.Entry.Level),
		Timestamp: browserTimestampString(entry.Entry.Timestamp),
		Args:      browserConsoleArgs(entry.Entry.Args),
	}
	if entry.Entry.URL != nil {
		result.URL = strings.TrimSpace(*entry.Entry.URL)
	}
	if entry.Entry.LineNumber != nil {
		line := *entry.Entry.LineNumber
		result.LineNumber = &line
	}
	result.Text = browserConsoleText(result.Args, entry.Entry.Text)
	return result
}

func browserConsoleEntryFromRuntime(entry *runtime.ConsoleAPICalledReply) browserConsoleEntry {
	if entry == nil {
		return browserConsoleEntry{}
	}
	result := browserConsoleEntry{
		Source:    "console",
		Level:     strings.TrimSpace(entry.Type),
		Type:      strings.TrimSpace(entry.Type),
		Timestamp: browserTimestampString(entry.Timestamp),
		Args:      browserConsoleArgs(entry.Args),
	}
	if entry.Context != nil {
		result.Context = strings.TrimSpace(*entry.Context)
	}
	result.Text = browserConsoleText(result.Args, strings.TrimSpace(entry.Type))
	return result
}

func collectBrowserConsoleEntries(
	ctx context.Context,
	entryAdded cdpLog.EntryAddedClient,
	consoleCalled runtime.ConsoleAPICalledClient,
	maxEntries int,
	timeout time.Duration,
) []browserConsoleEntry {
	if maxEntries <= 0 {
		maxEntries = 100
	}
	if timeout <= 0 {
		timeout = 500 * time.Millisecond
	}

	deadline := time.Now().Add(timeout)
	entries := make([]browserConsoleEntry, 0, min(maxEntries, 16))
	for len(entries) < maxEntries && time.Now().Before(deadline) {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		wait := remaining
		if wait > 25*time.Millisecond {
			wait = 25 * time.Millisecond
		}
		if entryAdded != nil {
			if item, ok := recvLogEntryWithin(ctx, entryAdded, wait); ok && item != nil {
				entry := browserConsoleEntryFromLog(item)
				if entry.Text != "" || len(entry.Args) > 0 {
					entries = append(entries, entry)
				}
			}
		}
		if len(entries) >= maxEntries || time.Now().After(deadline) {
			break
		}
		if consoleCalled != nil {
			if item, ok := recvRuntimeConsoleWithin(ctx, consoleCalled, wait); ok && item != nil {
				entry := browserConsoleEntryFromRuntime(item)
				if entry.Text != "" || len(entry.Args) > 0 {
					entries = append(entries, entry)
				}
			}
		}
	}
	return entries
}

func recvLogEntryWithin(ctx context.Context, client cdpLog.EntryAddedClient, wait time.Duration) (*cdpLog.EntryAddedReply, bool) {
	type result struct {
		item *cdpLog.EntryAddedReply
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		item, err := client.Recv()
		ch <- result{item: item, err: err}
	}()

	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return nil, false
	case <-timer.C:
		return nil, false
	case res := <-ch:
		return res.item, res.err == nil
	}
}

func recvRuntimeConsoleWithin(ctx context.Context, client runtime.ConsoleAPICalledClient, wait time.Duration) (*runtime.ConsoleAPICalledReply, bool) {
	type result struct {
		item *runtime.ConsoleAPICalledReply
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		item, err := client.Recv()
		ch <- result{item: item, err: err}
	}()

	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return nil, false
	case <-timer.C:
		return nil, false
	case res := <-ch:
		return res.item, res.err == nil
	}
}
