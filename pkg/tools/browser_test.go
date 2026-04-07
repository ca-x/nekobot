package tools

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/mafredri/cdp/devtool"
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

func TestBrowserToolParametersIncludeGetImages(t *testing.T) {
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
		if value == "get_images" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected get_images action in enum, got %#v", enumValues)
	}
}

func TestBrowserToolParametersIncludeGetForms(t *testing.T) {
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
		if value == "get_forms" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected get_forms action in enum, got %#v", enumValues)
	}
}

func TestBrowserToolParametersIncludeGetButtons(t *testing.T) {
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
		if value == "get_buttons" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected get_buttons action in enum, got %#v", enumValues)
	}
}

func TestBrowserToolParametersIncludeGetTables(t *testing.T) {
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
		if value == "get_tables" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected get_tables action in enum, got %#v", enumValues)
	}
}

func TestBrowserToolParametersIncludeGetLists(t *testing.T) {
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
		if value == "get_lists" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected get_lists action in enum, got %#v", enumValues)
	}
}

func TestBrowserToolParametersIncludeGetInputs(t *testing.T) {
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
		if value == "get_inputs" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected get_inputs action in enum, got %#v", enumValues)
	}
}

func TestBrowserToolParametersIncludeGetSelects(t *testing.T) {
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
		if value == "get_selects" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected get_selects action in enum, got %#v", enumValues)
	}
}

func TestBrowserToolParametersIncludeGetTextareas(t *testing.T) {
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
		if value == "get_textareas" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected get_textareas action in enum, got %#v", enumValues)
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

func TestBrowserToolGetImagesRejectsRelativeURLBeforeNavigation(t *testing.T) {
	tool := NewBrowserTool(newToolsTestLogger(t), true, 30, t.TempDir())

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "get_images",
		"url":    "example.com/path",
	})
	if err == nil {
		t.Fatal("expected invalid URL error")
	}
	if !strings.Contains(err.Error(), "absolute URL is required") {
		t.Fatalf("expected absolute URL error, got %v", err)
	}
}

func TestBrowserToolGetFormsRejectsRelativeURLBeforeNavigation(t *testing.T) {
	tool := NewBrowserTool(newToolsTestLogger(t), true, 30, t.TempDir())

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "get_forms",
		"url":    "example.com/path",
	})
	if err == nil {
		t.Fatal("expected invalid URL error")
	}
	if !strings.Contains(err.Error(), "absolute URL is required") {
		t.Fatalf("expected absolute URL error, got %v", err)
	}
}

func TestBrowserToolGetButtonsRejectsRelativeURLBeforeNavigation(t *testing.T) {
	tool := NewBrowserTool(newToolsTestLogger(t), true, 30, t.TempDir())

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "get_buttons",
		"url":    "example.com/path",
	})
	if err == nil {
		t.Fatal("expected invalid URL error")
	}
	if !strings.Contains(err.Error(), "absolute URL is required") {
		t.Fatalf("expected absolute URL error, got %v", err)
	}
}

func TestBrowserToolGetTablesRejectsRelativeURLBeforeNavigation(t *testing.T) {
	tool := NewBrowserTool(newToolsTestLogger(t), true, 30, t.TempDir())

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "get_tables",
		"url":    "example.com/path",
	})
	if err == nil {
		t.Fatal("expected invalid URL error")
	}
	if !strings.Contains(err.Error(), "absolute URL is required") {
		t.Fatalf("expected absolute URL error, got %v", err)
	}
}

func TestBrowserToolGetListsRejectsRelativeURLBeforeNavigation(t *testing.T) {
	tool := NewBrowserTool(newToolsTestLogger(t), true, 30, t.TempDir())

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "get_lists",
		"url":    "example.com/path",
	})
	if err == nil {
		t.Fatal("expected invalid URL error")
	}
	if !strings.Contains(err.Error(), "absolute URL is required") {
		t.Fatalf("expected absolute URL error, got %v", err)
	}
}

func TestBrowserToolGetInputsRejectsRelativeURLBeforeNavigation(t *testing.T) {
	tool := NewBrowserTool(newToolsTestLogger(t), true, 30, t.TempDir())

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "get_inputs",
		"url":    "example.com/path",
	})
	if err == nil {
		t.Fatal("expected invalid URL error")
	}
	if !strings.Contains(err.Error(), "absolute URL is required") {
		t.Fatalf("expected absolute URL error, got %v", err)
	}
}

func TestBrowserToolGetSelectsRejectsRelativeURLBeforeNavigation(t *testing.T) {
	tool := NewBrowserTool(newToolsTestLogger(t), true, 30, t.TempDir())

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "get_selects",
		"url":    "example.com/path",
	})
	if err == nil {
		t.Fatal("expected invalid URL error")
	}
	if !strings.Contains(err.Error(), "absolute URL is required") {
		t.Fatalf("expected absolute URL error, got %v", err)
	}
}

func TestBrowserToolGetTextareasRejectsRelativeURLBeforeNavigation(t *testing.T) {
	tool := NewBrowserTool(newToolsTestLogger(t), true, 30, t.TempDir())

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "get_textareas",
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

func TestBrowserToolParametersIncludeGetMetrics(t *testing.T) {
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
		if value == "get_metrics" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected get_metrics action in enum, got %#v", enumValues)
	}
}

func TestBrowserToolParametersIncludeEmulateDevice(t *testing.T) {
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
		if value == "emulate_device" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected emulate_device action in enum, got %#v", enumValues)
	}
}

func TestBrowserToolParametersIncludeSetViewport(t *testing.T) {
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
		if value == "set_viewport" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected set_viewport action in enum, got %#v", enumValues)
	}
}

func TestBrowserToolBuildDeviceProfile(t *testing.T) {
	tool := NewBrowserTool(newToolsTestLogger(t), true, 30, t.TempDir())

	profile, err := tool.buildDeviceProfile(map[string]interface{}{"device": "iphone"})
	if err != nil {
		t.Fatalf("buildDeviceProfile returned error: %v", err)
	}
	if profile.Name != "iphone" || profile.Width != 390 || profile.Height != 844 || !profile.Mobile {
		t.Fatalf("unexpected iphone profile: %+v", profile)
	}

	profile, err = tool.buildDeviceProfile(map[string]interface{}{"device": "desktop"})
	if err != nil {
		t.Fatalf("buildDeviceProfile returned error: %v", err)
	}
	if profile.Mobile {
		t.Fatalf("expected desktop profile to be non-mobile, got %+v", profile)
	}
}

func TestBrowserToolBuildDeviceProfileRejectsInvalidDevice(t *testing.T) {
	tool := NewBrowserTool(newToolsTestLogger(t), true, 30, t.TempDir())

	_, err := tool.buildDeviceProfile(map[string]interface{}{"device": "watch"})
	if err == nil {
		t.Fatal("expected invalid device error")
	}
	if !strings.Contains(err.Error(), "invalid device") {
		t.Fatalf("expected invalid device error, got %v", err)
	}
}

func TestBrowserToolBuildViewportRejectsMissingWidth(t *testing.T) {
	tool := NewBrowserTool(newToolsTestLogger(t), true, 30, t.TempDir())

	_, _, err := tool.buildViewport(map[string]interface{}{"height": float64(768)})
	if err == nil {
		t.Fatal("expected missing width error")
	}
	if !strings.Contains(err.Error(), "width parameter is required") {
		t.Fatalf("expected width required error, got %v", err)
	}
}

func TestBrowserToolBuildViewportRejectsInvalidHeight(t *testing.T) {
	tool := NewBrowserTool(newToolsTestLogger(t), true, 30, t.TempDir())

	_, _, err := tool.buildViewport(map[string]interface{}{"width": float64(1280), "height": float64(0)})
	if err == nil {
		t.Fatal("expected invalid height error")
	}
	if !strings.Contains(err.Error(), "invalid height") {
		t.Fatalf("expected invalid height error, got %v", err)
	}
}

func TestBrowserToolBuildViewportAcceptsPositiveDimensions(t *testing.T) {
	tool := NewBrowserTool(newToolsTestLogger(t), true, 30, t.TempDir())

	width, height, err := tool.buildViewport(map[string]interface{}{"width": float64(1280), "height": float64(720)})
	if err != nil {
		t.Fatalf("buildViewport returned error: %v", err)
	}
	if width != 1280 || height != 720 {
		t.Fatalf("unexpected viewport: %d x %d", width, height)
	}
}

func TestRequireTargetIDRejectsMissingTargetID(t *testing.T) {
	_, err := requireTargetID(map[string]interface{}{})
	if err == nil {
		t.Fatal("expected missing target_id error")
	}
	if !strings.Contains(err.Error(), "target_id parameter is required") {
		t.Fatalf("expected target_id error, got %v", err)
	}
}

func TestRequireTargetIDAcceptsTrimmedTargetID(t *testing.T) {
	target, err := requireTargetID(map[string]interface{}{"target_id": "  target-123  "})
	if err != nil {
		t.Fatalf("requireTargetID returned error: %v", err)
	}
	if target.ID != "target-123" {
		t.Fatalf("expected trimmed target id, got %q", target.ID)
	}
}

func TestMarshalBrowserTargetRejectsNilTarget(t *testing.T) {
	_, err := marshalBrowserTarget(nil)
	if err == nil {
		t.Fatal("expected nil target error")
	}
	if !strings.Contains(err.Error(), "browser page target unavailable") {
		t.Fatalf("expected nil target error, got %v", err)
	}
}

func TestMarshalBrowserTargetIncludesTargetFields(t *testing.T) {
	out, err := marshalBrowserTarget(&devtool.Target{
		ID:                   "page-1",
		Type:                 devtool.Page,
		Title:                "Example",
		URL:                  "https://example.com",
		DevToolsFrontendURL:  "/devtools/inspector.html?page-1",
		WebSocketDebuggerURL: "ws://chrome.internal/page-1",
	})
	if err != nil {
		t.Fatalf("marshalBrowserTarget returned error: %v", err)
	}
	if !strings.Contains(out, "\"id\":\"page-1\"") {
		t.Fatalf("expected id in marshaled target, got %s", out)
	}
	if !strings.Contains(out, "\"type\":\"page\"") {
		t.Fatalf("expected type in marshaled target, got %s", out)
	}
	if !strings.Contains(out, "\"url\":\"https://example.com\"") {
		t.Fatalf("expected url in marshaled target, got %s", out)
	}
}

func TestBrowserToolParametersIncludePageManagementActions(t *testing.T) {
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

	for _, want := range []string{"list_pages", "new_page", "activate_page", "close_page"} {
		found := false
		for _, value := range enumValues {
			if value == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected %s action in enum, got %#v", want, enumValues)
		}
	}
}

func TestBrowserToolCreatePageTargetRejectsRelativeURL(t *testing.T) {
	tool := NewBrowserTool(newToolsTestLogger(t), true, 30, t.TempDir())

	_, err := tool.createPageTarget(context.Background(), &stubBrowserDevTools{}, "example.com/path")
	if err == nil {
		t.Fatal("expected relative URL error")
	}
	if !strings.Contains(err.Error(), "absolute URL is required") {
		t.Fatalf("expected absolute URL error, got %v", err)
	}
}

func TestBrowserToolNewPageUsesCreateURL(t *testing.T) {
	tool := NewBrowserTool(newToolsTestLogger(t), true, 30, t.TempDir())
	session := installStubBrowserSession(t)
	devtools := &stubBrowserDevTools{
		createURLTarget: &devtool.Target{
			ID:    "page-2",
			Type:  devtool.Page,
			Title: "Example",
			URL:   "https://example.com",
		},
	}
	session.ready = true
	session.endpoint = "http://stub:9222"
	session.devtoolsFactory = func(endpoint string) browserDevTools {
		if endpoint != "http://stub:9222" {
			t.Fatalf("unexpected endpoint %q", endpoint)
		}
		return devtools
	}

	out, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "new_page",
		"url":    "https://example.com",
	})
	if err != nil {
		t.Fatalf("new_page returned error: %v", err)
	}
	if devtools.lastCreateURL != "https://example.com" {
		t.Fatalf("expected CreateURL call, got %q", devtools.lastCreateURL)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("unmarshal new_page response: %v", err)
	}
	if payload["id"] != "page-2" {
		t.Fatalf("expected page id page-2, got %v", payload["id"])
	}
	if payload["url"] != "https://example.com" {
		t.Fatalf("expected page url https://example.com, got %v", payload["url"])
	}
}

func TestBrowserToolListPagesReturnsJSON(t *testing.T) {
	tool := NewBrowserTool(newToolsTestLogger(t), true, 30, t.TempDir())
	session := installStubBrowserSession(t)
	devtools := &stubBrowserDevTools{
		listTargets: []*devtool.Target{
			{ID: "page-1", Type: devtool.Page, Title: "One", URL: "https://one.example"},
			{ID: "page-2", Type: devtool.Page, Title: "Two", URL: "https://two.example"},
		},
	}
	session.ready = true
	session.endpoint = "http://stub:9222"
	session.devtoolsFactory = func(string) browserDevTools { return devtools }

	out, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "list_pages",
	})
	if err != nil {
		t.Fatalf("list_pages returned error: %v", err)
	}

	var payload []map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("unmarshal list_pages response: %v", err)
	}
	if len(payload) != 2 {
		t.Fatalf("expected 2 pages, got %d", len(payload))
	}
	if payload[0]["id"] != "page-1" || payload[1]["id"] != "page-2" {
		t.Fatalf("unexpected page payload: %+v", payload)
	}
}

func TestBrowserToolActivatePageRequiresTargetID(t *testing.T) {
	tool := NewBrowserTool(newToolsTestLogger(t), true, 30, t.TempDir())
	session := installStubBrowserSession(t)
	session.ready = true
	session.endpoint = "http://stub:9222"
	session.devtoolsFactory = func(string) browserDevTools { return &stubBrowserDevTools{} }

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "activate_page",
	})
	if err == nil {
		t.Fatal("expected missing target_id error")
	}
	if !strings.Contains(err.Error(), "target_id parameter is required") {
		t.Fatalf("expected missing target_id error, got %v", err)
	}
}

func TestBrowserToolActivateAndClosePageUseTargetID(t *testing.T) {
	tool := NewBrowserTool(newToolsTestLogger(t), true, 30, t.TempDir())
	session := installStubBrowserSession(t)
	devtools := &stubBrowserDevTools{}
	session.ready = true
	session.endpoint = "http://stub:9222"
	session.devtoolsFactory = func(string) browserDevTools { return devtools }

	out, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":    "activate_page",
		"target_id": "page-9",
	})
	if err != nil {
		t.Fatalf("activate_page returned error: %v", err)
	}
	if devtools.activatedTargetID != "page-9" {
		t.Fatalf("expected activate target page-9, got %q", devtools.activatedTargetID)
	}
	if !strings.Contains(out, "page-9") {
		t.Fatalf("expected activate response to mention target id, got %q", out)
	}

	out, err = tool.Execute(context.Background(), map[string]interface{}{
		"action":    "close_page",
		"target_id": "page-9",
	})
	if err != nil {
		t.Fatalf("close_page returned error: %v", err)
	}
	if devtools.closedTargetID != "page-9" {
		t.Fatalf("expected close target page-9, got %q", devtools.closedTargetID)
	}
	if !strings.Contains(out, "page-9") {
		t.Fatalf("expected close response to mention target id, got %q", out)
	}
}

type stubBrowserDevTools struct {
	listTargets       []*devtool.Target
	createTarget      *devtool.Target
	createURLTarget   *devtool.Target
	listErr           error
	createErr         error
	createURLErr      error
	activateErr       error
	closeErr          error
	lastCreateURL     string
	activatedTargetID string
	closedTargetID    string
}

func (s *stubBrowserDevTools) List(context.Context) ([]*devtool.Target, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.listTargets, nil
}

func (s *stubBrowserDevTools) Create(context.Context) (*devtool.Target, error) {
	if s.createErr != nil {
		return nil, s.createErr
	}
	if s.createTarget != nil {
		return s.createTarget, nil
	}
	return &devtool.Target{ID: "new-page", Type: devtool.Page}, nil
}

func (s *stubBrowserDevTools) CreateURL(_ context.Context, openURL string) (*devtool.Target, error) {
	s.lastCreateURL = openURL
	if s.createURLErr != nil {
		return nil, s.createURLErr
	}
	if s.createURLTarget != nil {
		return s.createURLTarget, nil
	}
	return &devtool.Target{ID: "new-page", Type: devtool.Page, URL: openURL}, nil
}

func (s *stubBrowserDevTools) Activate(_ context.Context, t *devtool.Target) error {
	if s.activateErr != nil {
		return s.activateErr
	}
	if t != nil {
		s.activatedTargetID = t.ID
	}
	return nil
}

func (s *stubBrowserDevTools) Close(_ context.Context, t *devtool.Target) error {
	if s.closeErr != nil {
		return s.closeErr
	}
	if t != nil {
		s.closedTargetID = t.ID
	}
	return nil
}

func installStubBrowserSession(t *testing.T) *BrowserSession {
	t.Helper()

	oldSession := browserSession

	browserSession = nil
	browserSessionOnce = sync.Once{}
	session := GetBrowserSession(newToolsTestLogger(t))

	t.Cleanup(func() {
		browserSession = oldSession
		browserSessionOnce = sync.Once{}
	})

	return session
}

var _ browserDevTools = (*stubBrowserDevTools)(nil)
