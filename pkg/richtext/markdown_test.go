package richtext

import (
	"strings"
	"testing"
)

func TestMarkdownToPlainText(t *testing.T) {
	input := "# Title\n\n- item\n\n`code`\n\n[link](https://example.com)"
	got := MarkdownToPlainText(input)
	if strings.Contains(got, "[") || strings.Contains(got, "https://example.com") {
		t.Fatalf("expected markdown links removed, got %q", got)
	}
	if !strings.Contains(got, "Title") || !strings.Contains(got, "• item") || !strings.Contains(got, "code") {
		t.Fatalf("unexpected plain text conversion: %q", got)
	}
}

func TestSplitPlainText(t *testing.T) {
	text := "line1\nline2\nline3\nline4"
	parts := SplitPlainText(text, 8)
	if len(parts) < 2 {
		t.Fatalf("expected text to split, got %#v", parts)
	}
	if strings.Join(parts, "\n") == "" {
		t.Fatal("expected non-empty parts")
	}
}

func TestBuildMarkdownHTML(t *testing.T) {
	html := BuildMarkdownHTML("## Hello\n\n`world`")
	if !strings.Contains(html, "Nekobot WeChat Render") {
		t.Fatalf("expected render header, got %q", html)
	}
	if !strings.Contains(html, "Hello") || !strings.Contains(html, "world") {
		t.Fatalf("expected markdown content in html, got %q", html)
	}
}
