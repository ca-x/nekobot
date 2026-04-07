package tools

import (
	"context"
	"strings"
	"testing"
)

func TestBrowserToolStartModeFromParams(t *testing.T) {
	tool := NewBrowserTool(newToolsTestLogger(t), true, 30, t.TempDir())

	got, err := tool.startMode(map[string]interface{}{})
	if err != nil {
		t.Fatalf("startMode returned error for default params: %v", err)
	}
	if got != BrowserModeAuto {
		t.Fatalf("expected default auto mode, got %q", got)
	}

	got, err = tool.startMode(map[string]interface{}{"mode": "direct"})
	if err != nil {
		t.Fatalf("startMode returned error for direct mode: %v", err)
	}
	if got != BrowserModeDirect {
		t.Fatalf("expected direct mode, got %q", got)
	}

	got, err = tool.startMode(map[string]interface{}{"mode": "relay"})
	if err != nil {
		t.Fatalf("startMode returned error for relay mode: %v", err)
	}
	if got != BrowserModeRelay {
		t.Fatalf("expected relay mode, got %q", got)
	}
}

func TestBrowserToolExecuteRejectsInvalidMode(t *testing.T) {
	tool := NewBrowserTool(newToolsTestLogger(t), true, 30, t.TempDir())

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "navigate",
		"url":    "https://example.com",
		"mode":   "invalid",
	})
	if err == nil {
		t.Fatal("expected invalid mode error")
	}
	if !strings.Contains(err.Error(), "invalid browser mode") {
		t.Fatalf("expected invalid mode error, got %v", err)
	}
}

func TestBrowserToolParametersIncludePrintPDF(t *testing.T) {
	tool := NewBrowserTool(newToolsTestLogger(t), true, 30, t.TempDir())

	params := tool.Parameters()
	properties, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected properties map, got %#v", params["properties"])
	}
	action, ok := properties["action"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected action schema, got %#v", properties["action"])
	}
	enumValues, ok := action["enum"].([]string)
	if !ok {
		t.Fatalf("expected enum values, got %#v", action["enum"])
	}
	found := false
	for _, value := range enumValues {
		if value == "print_pdf" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected print_pdf action in enum, got %#v", enumValues)
	}
}

func TestBrowserToolBuildPrintToPDFArgs(t *testing.T) {
	tool := NewBrowserTool(newToolsTestLogger(t), true, 30, t.TempDir())

	args := tool.buildPrintToPDFArgs(map[string]interface{}{
		"landscape":             true,
		"display_header_footer": true,
		"print_background":      true,
		"margin_top":            float64(0.2),
		"margin_bottom":         float64(0.4),
		"margin_left":           float64(0.6),
		"margin_right":          float64(0.8),
	})

	if args.Landscape == nil || !*args.Landscape {
		t.Fatalf("expected landscape true, got %#v", args.Landscape)
	}
	if args.DisplayHeaderFooter == nil || !*args.DisplayHeaderFooter {
		t.Fatalf("expected display header/footer true, got %#v", args.DisplayHeaderFooter)
	}
	if args.PrintBackground == nil || !*args.PrintBackground {
		t.Fatalf("expected print background true, got %#v", args.PrintBackground)
	}
	if args.PreferCSSPageSize == nil || !*args.PreferCSSPageSize {
		t.Fatalf("expected prefer css page size true, got %#v", args.PreferCSSPageSize)
	}
	if args.MarginTop == nil || *args.MarginTop != 0.2 {
		t.Fatalf("expected top margin 0.2, got %#v", args.MarginTop)
	}
	if args.MarginBottom == nil || *args.MarginBottom != 0.4 {
		t.Fatalf("expected bottom margin 0.4, got %#v", args.MarginBottom)
	}
	if args.MarginLeft == nil || *args.MarginLeft != 0.6 {
		t.Fatalf("expected left margin 0.6, got %#v", args.MarginLeft)
	}
	if args.MarginRight == nil || *args.MarginRight != 0.8 {
		t.Fatalf("expected right margin 0.8, got %#v", args.MarginRight)
	}
	if args.PaperWidth == nil || *args.PaperWidth != 8.27 {
		t.Fatalf("expected A4 paper width, got %#v", args.PaperWidth)
	}
	if args.PaperHeight == nil || *args.PaperHeight != 11.69 {
		t.Fatalf("expected A4 paper height, got %#v", args.PaperHeight)
	}
}

func TestBrowserToolParametersIncludeExtractStructuredData(t *testing.T) {
	tool := NewBrowserTool(newToolsTestLogger(t), true, 30, t.TempDir())

	params := tool.Parameters()
	properties, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected properties map, got %#v", params["properties"])
	}
	action, ok := properties["action"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected action schema, got %#v", properties["action"])
	}
	enumValues, ok := action["enum"].([]string)
	if !ok {
		t.Fatalf("expected enum values, got %#v", action["enum"])
	}
	found := false
	for _, value := range enumValues {
		if value == "extract_structured_data" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected extract_structured_data action in enum, got %#v", enumValues)
	}
}

func TestBrowserToolBuildExtractionScript(t *testing.T) {
	tool := NewBrowserTool(newToolsTestLogger(t), true, 30, t.TempDir())

	jsonLD := tool.buildExtractionScript("json_ld")
	if !strings.Contains(jsonLD, "application/ld+json") {
		t.Fatalf("expected json_ld extractor script, got %q", jsonLD)
	}
	if strings.Contains(jsonLD, "open_graph") {
		t.Fatalf("did not expect open_graph extraction in json_ld mode, got %q", jsonLD)
	}

	all := tool.buildExtractionScript("all")
	if !strings.Contains(all, "schema_org") {
		t.Fatalf("expected schema_org extraction in all mode, got %q", all)
	}
	if !strings.Contains(all, "open_graph") {
		t.Fatalf("expected open_graph extraction in all mode, got %q", all)
	}
}

func TestBrowserToolParametersIncludeGetText(t *testing.T) {
	tool := NewBrowserTool(newToolsTestLogger(t), true, 30, t.TempDir())

	params := tool.Parameters()
	properties, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected properties map, got %#v", params["properties"])
	}
	action, ok := properties["action"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected action schema, got %#v", properties["action"])
	}
	enumValues, ok := action["enum"].([]string)
	if !ok {
		t.Fatalf("expected enum values, got %#v", action["enum"])
	}
	found := false
	for _, value := range enumValues {
		if value == "get_text" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected get_text action in enum, got %#v", enumValues)
	}
}

func TestBrowserToolParametersIncludeGetTitle(t *testing.T) {
	tool := NewBrowserTool(newToolsTestLogger(t), true, 30, t.TempDir())

	params := tool.Parameters()
	properties, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected properties map, got %#v", params["properties"])
	}
	action, ok := properties["action"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected action schema, got %#v", properties["action"])
	}
	enumValues, ok := action["enum"].([]string)
	if !ok {
		t.Fatalf("expected enum values, got %#v", action["enum"])
	}
	found := false
	for _, value := range enumValues {
		if value == "get_title" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected get_title action in enum, got %#v", enumValues)
	}
}

func TestBrowserToolParametersIncludeGetURL(t *testing.T) {
	tool := NewBrowserTool(newToolsTestLogger(t), true, 30, t.TempDir())

	params := tool.Parameters()
	properties, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected properties map, got %#v", params["properties"])
	}
	action, ok := properties["action"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected action schema, got %#v", properties["action"])
	}
	enumValues, ok := action["enum"].([]string)
	if !ok {
		t.Fatalf("expected enum values, got %#v", action["enum"])
	}
	found := false
	for _, value := range enumValues {
		if value == "get_url" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected get_url action in enum, got %#v", enumValues)
	}
}

func TestBrowserToolParametersIncludeGetLinks(t *testing.T) {
	tool := NewBrowserTool(newToolsTestLogger(t), true, 30, t.TempDir())

	params := tool.Parameters()
	properties, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected properties map, got %#v", params["properties"])
	}
	action, ok := properties["action"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected action schema, got %#v", properties["action"])
	}
	enumValues, ok := action["enum"].([]string)
	if !ok {
		t.Fatalf("expected enum values, got %#v", action["enum"])
	}
	found := false
	for _, value := range enumValues {
		if value == "get_links" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected get_links action in enum, got %#v", enumValues)
	}
}

func TestBrowserToolParametersIncludeGetCookies(t *testing.T) {
	tool := NewBrowserTool(newToolsTestLogger(t), true, 30, t.TempDir())

	params := tool.Parameters()
	properties, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected properties map, got %#v", params["properties"])
	}
	action, ok := properties["action"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected action schema, got %#v", properties["action"])
	}
	enumValues, ok := action["enum"].([]string)
	if !ok {
		t.Fatalf("expected enum values, got %#v", action["enum"])
	}
	found := false
	for _, value := range enumValues {
		if value == "get_cookies" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected get_cookies action in enum, got %#v", enumValues)
	}
}

func TestBrowserToolParametersIncludeGetMeta(t *testing.T) {
	tool := NewBrowserTool(newToolsTestLogger(t), true, 30, t.TempDir())

	params := tool.Parameters()
	properties, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected properties map, got %#v", params["properties"])
	}
	action, ok := properties["action"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected action schema, got %#v", properties["action"])
	}
	enumValues, ok := action["enum"].([]string)
	if !ok {
		t.Fatalf("expected enum values, got %#v", action["enum"])
	}
	found := false
	for _, value := range enumValues {
		if value == "get_meta" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected get_meta action in enum, got %#v", enumValues)
	}
}

func TestBrowserToolParametersIncludeGetHeadings(t *testing.T) {
	tool := NewBrowserTool(newToolsTestLogger(t), true, 30, t.TempDir())

	params := tool.Parameters()
	properties, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected properties map, got %#v", params["properties"])
	}
	action, ok := properties["action"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected action schema, got %#v", properties["action"])
	}
	enumValues, ok := action["enum"].([]string)
	if !ok {
		t.Fatalf("expected enum values, got %#v", action["enum"])
	}
	found := false
	for _, value := range enumValues {
		if value == "get_headings" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected get_headings action in enum, got %#v", enumValues)
	}
}

func TestBrowserToolExecuteRejectsMissingSelectValue(t *testing.T) {
	tool := NewBrowserTool(newToolsTestLogger(t), true, 30, t.TempDir())

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "select",
		"selector": "select[name=country]",
	})
	if err == nil {
		t.Fatal("expected select value error")
	}
	if !strings.Contains(err.Error(), "value parameter is required") {
		t.Fatalf("expected missing value error, got %v", err)
	}
}

func TestBrowserToolBuildSelectScriptRejectsMissingOption(t *testing.T) {
	tool := NewBrowserTool(newToolsTestLogger(t), true, 30, t.TempDir())

	script := tool.buildSelectScript("select[name=country]", "missing")
	if !strings.Contains(script, "option not found") {
		t.Fatalf("expected missing-option guard in script, got %q", script)
	}
}

func TestBrowserToolNavigationParamsPreserveMode(t *testing.T) {
	tool := NewBrowserTool(newToolsTestLogger(t), true, 30, t.TempDir())

	params := tool.navigationParams(map[string]interface{}{
		"url":            "https://example.com",
		"mode":           "relay",
		"debug_port":     float64(9555),
		"debug_endpoint": "http://chrome.internal:9333",
	}, "https://override.example.com")

	if got, _ := params["url"].(string); got != "https://override.example.com" {
		t.Fatalf("expected overridden url, got %#v", params["url"])
	}
	if got, _ := params["mode"].(string); got != "relay" {
		t.Fatalf("expected relay mode preserved, got %#v", params["mode"])
	}
	if got, _ := params["debug_port"].(float64); got != 9555 {
		t.Fatalf("expected debug_port preserved, got %#v", params["debug_port"])
	}
	if got, _ := params["debug_endpoint"].(string); got != "http://chrome.internal:9333" {
		t.Fatalf("expected debug_endpoint preserved, got %#v", params["debug_endpoint"])
	}
}

func TestBrowserToolStartOptionsFromParams(t *testing.T) {
	tool := NewBrowserTool(newToolsTestLogger(t), true, 30, t.TempDir())

	opts, err := tool.startOptions(map[string]interface{}{
		"mode":           "relay",
		"debug_port":     float64(9555),
		"debug_endpoint": "http://chrome.internal:9333",
	})
	if err != nil {
		t.Fatalf("startOptions returned error: %v", err)
	}
	if opts.Mode != BrowserModeRelay {
		t.Fatalf("expected relay mode, got %+v", opts)
	}
	if len(opts.Ports) != 1 || opts.Ports[0] != 9555 {
		t.Fatalf("expected custom debug port 9555, got %+v", opts)
	}
	if opts.Endpoint != "http://chrome.internal:9333" {
		t.Fatalf("expected custom endpoint, got %+v", opts)
	}
}

func TestBrowserToolStartOptionsRejectsInvalidDebugPort(t *testing.T) {
	tool := NewBrowserTool(newToolsTestLogger(t), true, 30, t.TempDir())

	_, err := tool.startOptions(map[string]interface{}{
		"debug_port": float64(0),
	})
	if err == nil {
		t.Fatal("expected invalid debug_port error")
	}
	if !strings.Contains(err.Error(), "invalid debug_port") {
		t.Fatalf("expected invalid debug_port error, got %v", err)
	}
}

func TestBrowserToolExecuteRejectsRelativeNavigateURL(t *testing.T) {
	tool := NewBrowserTool(newToolsTestLogger(t), true, 30, t.TempDir())

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "navigate",
		"url":    "example.com/path",
	})
	if err == nil {
		t.Fatal("expected invalid URL error")
	}
	if !strings.Contains(err.Error(), "absolute URL is required") {
		t.Fatalf("expected absolute URL error, got %v", err)
	}
}

func TestBrowserToolGetTextRejectsRelativeURLBeforeNavigation(t *testing.T) {
	tool := NewBrowserTool(newToolsTestLogger(t), true, 30, t.TempDir())

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "get_text",
		"url":    "example.com/path",
	})
	if err == nil {
		t.Fatal("expected invalid URL error")
	}
	if !strings.Contains(err.Error(), "absolute URL is required") {
		t.Fatalf("expected absolute URL error, got %v", err)
	}
}

func TestBrowserToolGetHTMLRejectsRelativeURLBeforeNavigation(t *testing.T) {
	tool := NewBrowserTool(newToolsTestLogger(t), true, 30, t.TempDir())

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "get_html",
		"url":    "example.com/path",
	})
	if err == nil {
		t.Fatal("expected invalid URL error")
	}
	if !strings.Contains(err.Error(), "absolute URL is required") {
		t.Fatalf("expected absolute URL error, got %v", err)
	}
}

func TestBrowserToolGetTitleRejectsRelativeURLBeforeNavigation(t *testing.T) {
	tool := NewBrowserTool(newToolsTestLogger(t), true, 30, t.TempDir())

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "get_title",
		"url":    "example.com/path",
	})
	if err == nil {
		t.Fatal("expected invalid URL error")
	}
	if !strings.Contains(err.Error(), "absolute URL is required") {
		t.Fatalf("expected absolute URL error, got %v", err)
	}
}

func TestBrowserToolGetURLRejectsRelativeURLBeforeNavigation(t *testing.T) {
	tool := NewBrowserTool(newToolsTestLogger(t), true, 30, t.TempDir())

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "get_url",
		"url":    "example.com/path",
	})
	if err == nil {
		t.Fatal("expected invalid URL error")
	}
	if !strings.Contains(err.Error(), "absolute URL is required") {
		t.Fatalf("expected absolute URL error, got %v", err)
	}
}

func TestBrowserToolGetLinksRejectsRelativeURLBeforeNavigation(t *testing.T) {
	tool := NewBrowserTool(newToolsTestLogger(t), true, 30, t.TempDir())

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "get_links",
		"url":    "example.com/path",
	})
	if err == nil {
		t.Fatal("expected invalid URL error")
	}
	if !strings.Contains(err.Error(), "absolute URL is required") {
		t.Fatalf("expected absolute URL error, got %v", err)
	}
}

func TestBrowserToolGetCookiesRejectsRelativeURLBeforeNavigation(t *testing.T) {
	tool := NewBrowserTool(newToolsTestLogger(t), true, 30, t.TempDir())

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "get_cookies",
		"url":    "example.com/path",
	})
	if err == nil {
		t.Fatal("expected invalid URL error")
	}
	if !strings.Contains(err.Error(), "absolute URL is required") {
		t.Fatalf("expected absolute URL error, got %v", err)
	}
}

func TestBrowserToolGetMetaRejectsRelativeURLBeforeNavigation(t *testing.T) {
	tool := NewBrowserTool(newToolsTestLogger(t), true, 30, t.TempDir())

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "get_meta",
		"url":    "example.com/path",
	})
	if err == nil {
		t.Fatal("expected invalid URL error")
	}
	if !strings.Contains(err.Error(), "absolute URL is required") {
		t.Fatalf("expected absolute URL error, got %v", err)
	}
}

func TestBrowserToolGetHeadingsRejectsRelativeURLBeforeNavigation(t *testing.T) {
	tool := NewBrowserTool(newToolsTestLogger(t), true, 30, t.TempDir())

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "get_headings",
		"url":    "example.com/path",
	})
	if err == nil {
		t.Fatal("expected invalid URL error")
	}
	if !strings.Contains(err.Error(), "absolute URL is required") {
		t.Fatalf("expected absolute URL error, got %v", err)
	}
}
func TestHTMLToTextStripsTags(t *testing.T) {
	got := htmlToText("<html><body><h1>Hello</h1><p>World</p></body></html>")
	if got != "HelloWorld" {
		t.Fatalf("expected stripped text, got %q", got)
	}
}
