package tools

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mafredri/cdp/devtool"
	cdpLog "github.com/mafredri/cdp/protocol/log"
	"github.com/mafredri/cdp/protocol/network"
	"github.com/mafredri/cdp/protocol/runtime"
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

func TestBrowserToolParametersIncludeCookieControlActions(t *testing.T) {
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
	for _, want := range []string{"set_cookie", "clear_cookies"} {
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

func TestBrowserToolBuildSetCookieArgs(t *testing.T) {
	tool := NewBrowserTool(newToolsTestLogger(t), true, 30, t.TempDir())
	args, err := tool.buildSetCookieArgs(map[string]interface{}{
		"name":      "session",
		"text":      "token-1",
		"url":       "https://example.com/app",
		"secure":    true,
		"http_only": true,
		"same_site": "lax",
	})
	if err != nil {
		t.Fatalf("buildSetCookieArgs returned error: %v", err)
	}
	if args.Name != "session" || args.Value != "token-1" {
		t.Fatalf("unexpected cookie args: %+v", args)
	}
	if args.URL == nil || *args.URL != "https://example.com/app" {
		t.Fatalf("expected cookie url, got %+v", args.URL)
	}
	if args.Secure == nil || !*args.Secure {
		t.Fatalf("expected secure flag, got %+v", args.Secure)
	}
	if args.HTTPOnly == nil || !*args.HTTPOnly {
		t.Fatalf("expected httpOnly flag, got %+v", args.HTTPOnly)
	}
	if args.SameSite != network.CookieSameSiteLax {
		t.Fatalf("expected same_site lax, got %v", args.SameSite)
	}
}

func TestBrowserToolBuildSetCookieArgsRejectsMissingName(t *testing.T) {
	tool := NewBrowserTool(newToolsTestLogger(t), true, 30, t.TempDir())
	_, err := tool.buildSetCookieArgs(map[string]interface{}{
		"text": "token-1",
		"url":  "https://example.com",
	})
	if err == nil {
		t.Fatal("expected missing name error")
	}
	if !strings.Contains(err.Error(), "name parameter is required") {
		t.Fatalf("expected name required error, got %v", err)
	}
}

func TestBrowserToolBuildSetCookieArgsRejectsRelativeURL(t *testing.T) {
	tool := NewBrowserTool(newToolsTestLogger(t), true, 30, t.TempDir())
	_, err := tool.buildSetCookieArgs(map[string]interface{}{
		"name": "session",
		"text": "token-1",
		"url":  "example.com/app",
	})
	if err == nil {
		t.Fatal("expected absolute URL error")
	}
	if !strings.Contains(err.Error(), "absolute URL is required") {
		t.Fatalf("expected absolute URL error, got %v", err)
	}
}

func TestBrowserToolBuildSetCookieArgsRejectsInvalidSameSite(t *testing.T) {
	tool := NewBrowserTool(newToolsTestLogger(t), true, 30, t.TempDir())
	_, err := tool.buildSetCookieArgs(map[string]interface{}{
		"name":      "session",
		"text":      "token-1",
		"url":       "https://example.com",
		"same_site": "weird",
	})
	if err == nil {
		t.Fatal("expected invalid same_site error")
	}
	if !strings.Contains(err.Error(), "invalid same_site") {
		t.Fatalf("expected invalid same_site error, got %v", err)
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

func TestBrowserToolBuildNetworkOptions(t *testing.T) {
	tool := NewBrowserTool(newToolsTestLogger(t), true, 30, t.TempDir())
	defaults := tool.buildNetworkOptions(map[string]interface{}{})
	if defaults.MaxEntries != 100 || defaults.Duration != 500*time.Millisecond || defaults.ResourceType != "" {
		t.Fatalf("unexpected default network options: %+v", defaults)
	}
	opts := tool.buildNetworkOptions(map[string]interface{}{
		"max_entries":   float64(12),
		"duration":      float64(750),
		"resource_type": "XHR",
	})
	if opts.MaxEntries != 12 || opts.Duration != 750*time.Millisecond || opts.ResourceType != "xhr" {
		t.Fatalf("unexpected custom network options: %+v", opts)
	}
}

func TestBrowserNetworkEntryMatchesResourceTypeFilter(t *testing.T) {
	entry := browserNetworkEntry{Type: "XHR"}
	if !browserNetworkEntryMatches(entry, browserNetworkOptions{}) {
		t.Fatal("expected empty filter to match")
	}
	if !browserNetworkEntryMatches(entry, browserNetworkOptions{ResourceType: "xhr"}) {
		t.Fatal("expected xhr filter to match regardless of case")
	}
	if browserNetworkEntryMatches(entry, browserNetworkOptions{ResourceType: "document"}) {
		t.Fatal("expected document filter to reject xhr entry")
	}
}

func TestBrowserToolParametersIncludeGetNetwork(t *testing.T) {
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
		if value == "get_network" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected get_network action in enum, got %#v", enumValues)
	}
}

func TestBrowserNetworkEntryFromRequest(t *testing.T) {
	entry := browserNetworkEntryFromRequest(&network.RequestWillBeSentReply{
		RequestID: "req-1",
		Type:      network.ResourceTypeDocument,
		Request: network.Request{
			Method: "GET",
			URL:    "https://example.com",
		},
	})
	if entry.RequestID != "req-1" || entry.Phase != "request" || entry.Method != "GET" || entry.URL != "https://example.com" {
		t.Fatalf("unexpected request entry: %+v", entry)
	}
}

func TestBrowserNetworkEntryFromResponse(t *testing.T) {
	proto := "h2"
	ip := "203.0.113.10"
	fromCache := true
	entry := browserNetworkEntryFromResponse(&network.ResponseReceivedReply{
		RequestID: "req-2",
		Type:      network.ResourceTypeXHR,
		Response: network.Response{
			URL:             "https://example.com/api",
			Status:          200,
			StatusText:      "OK",
			MimeType:        "application/json",
			Protocol:        &proto,
			RemoteIPAddress: &ip,
			FromDiskCache:   &fromCache,
		},
	})
	if entry.RequestID != "req-2" || entry.Phase != "response" || entry.Status == nil || *entry.Status != 200 {
		t.Fatalf("unexpected response entry: %+v", entry)
	}
	if !entry.FromCache || entry.Protocol != "h2" || entry.RemoteIP != ip {
		t.Fatalf("expected cache/protocol/ip fields, got %+v", entry)
	}
}

func TestBrowserNetworkEntryFromFinishedAndFailed(t *testing.T) {
	finished := browserNetworkEntryFromFinished(&network.LoadingFinishedReply{RequestID: "req-3", EncodedDataLength: 512})
	if finished.Phase != "finished" || finished.EncodedSize == nil || *finished.EncodedSize != 512 {
		t.Fatalf("unexpected finished entry: %+v", finished)
	}
	failed := browserNetworkEntryFromFailed(&network.LoadingFailedReply{RequestID: "req-4", ErrorText: "net::ERR_ABORTED", Type: network.ResourceTypeImage})
	if failed.Phase != "failed" || failed.ErrorText != "net::ERR_ABORTED" {
		t.Fatalf("unexpected failed entry: %+v", failed)
	}
}

func TestBrowserToolExecuteDispatchesAdvancedBrowserActions(t *testing.T) {
	tool := NewBrowserTool(newToolsTestLogger(t), true, 30, t.TempDir())
	actions := map[string]string{
		"get_console":    "getConsole",
		"get_network":    "getNetwork",
		"get_storage":    "browserStorageAction",
		"set_storage":    "browserStorageAction",
		"remove_storage": "browserStorageAction",
		"clear_storage":  "browserStorageAction",
	}
	for action := range actions {
		_, err := tool.Execute(context.Background(), map[string]interface{}{"action": action})
		if err == nil {
			t.Fatalf("expected %s to reach handler and fail on missing runtime inputs", action)
		}
	}
}

func TestBrowserToolParametersIncludeGetConsole(t *testing.T) {
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
		if value == "get_console" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected get_console action in enum, got %#v", enumValues)
	}
}

func TestBrowserToolBuildConsoleOptionsDefaultsAndFilters(t *testing.T) {
	tool := NewBrowserTool(newToolsTestLogger(t), true, 30, t.TempDir())

	defaults := tool.buildConsoleOptions(map[string]interface{}{})
	if defaults.MaxEntries != 100 || defaults.ErrorsOnly || defaults.WarningsOnly || defaults.InfoOnly {
		t.Fatalf("unexpected default console options: %+v", defaults)
	}

	opts := tool.buildConsoleOptions(map[string]interface{}{
		"errors_only": true,
		"max_entries": float64(12),
	})
	if !opts.ErrorsOnly || opts.MaxEntries != 12 {
		t.Fatalf("unexpected custom console options: %+v", opts)
	}
}

func TestBrowserConsoleEntryMatchesRespectsPriority(t *testing.T) {
	if !browserConsoleEntryMatches("error", browserConsoleOptions{ErrorsOnly: true}) {
		t.Fatal("expected error filter to match error")
	}
	if browserConsoleEntryMatches("warning", browserConsoleOptions{ErrorsOnly: true}) {
		t.Fatal("expected error filter to reject warning")
	}
	if !browserConsoleEntryMatches("warning", browserConsoleOptions{WarningsOnly: true}) {
		t.Fatal("expected warning filter to match warning")
	}
	if !browserConsoleEntryMatches("info", browserConsoleOptions{InfoOnly: true}) {
		t.Fatal("expected info filter to match info")
	}
	if !browserConsoleEntryMatches("log", browserConsoleOptions{InfoOnly: true}) {
		t.Fatal("expected info filter to match log")
	}
}

func TestBrowserConsoleEntryFromLogUsesArgsAsText(t *testing.T) {
	line := 17
	url := "https://example.com/app.js"
	entry := browserConsoleEntryFromLog(&cdpLog.EntryAddedReply{
		Entry: cdpLog.Entry{
			Source:     "javascript",
			Level:      "warning",
			Text:       "fallback",
			Timestamp:  runtime.Timestamp(1000),
			URL:        &url,
			LineNumber: &line,
			Args:       []runtime.RemoteObject{{Value: json.RawMessage(`"warn"`)}, {Description: stringPtr("details")}},
		},
	})
	if entry.Source != "javascript" || entry.Level != "warning" {
		t.Fatalf("unexpected log entry identity: %+v", entry)
	}
	if entry.Text != "\"warn\" details" {
		t.Fatalf("expected joined args text, got %q", entry.Text)
	}
	if entry.URL != url || entry.LineNumber == nil || *entry.LineNumber != line {
		t.Fatalf("expected url/line preserved, got %+v", entry)
	}
	if len(entry.Args) != 2 {
		t.Fatalf("expected 2 args, got %+v", entry.Args)
	}
}

func TestBrowserConsoleEntryFromRuntimeUsesContext(t *testing.T) {
	ctxValue := "named-console"
	entry := browserConsoleEntryFromRuntime(&runtime.ConsoleAPICalledReply{
		Type:      "error",
		Timestamp: runtime.Timestamp(2000),
		Context:   &ctxValue,
		Args:      []runtime.RemoteObject{{Description: stringPtr("boom")}},
	})
	if entry.Source != "console" || entry.Level != "error" || entry.Type != "error" {
		t.Fatalf("unexpected runtime entry identity: %+v", entry)
	}
	if entry.Context != ctxValue {
		t.Fatalf("expected context %q, got %+v", ctxValue, entry)
	}
	if entry.Text != "boom" {
		t.Fatalf("expected text boom, got %q", entry.Text)
	}
}

func stringPtr(value string) *string {
	return &value
}
