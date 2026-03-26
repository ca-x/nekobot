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
}

func TestBrowserToolExecuteRejectsInvalidMode(t *testing.T) {
	tool := NewBrowserTool(newToolsTestLogger(t), true, 30, t.TempDir())

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "navigate",
		"url":    "https://example.com",
		"mode":   "relay",
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
