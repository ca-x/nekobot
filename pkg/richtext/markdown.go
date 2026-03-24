package richtext

import (
	"html"
	"regexp"
	"strings"
)

var (
	reMarkdownImage   = regexp.MustCompile(`!\[[^\]]*\]\(([^)]+)\)`)
	reMarkdownLink    = regexp.MustCompile(`\[([^\]]+)\]\([^)]*\)`)
	reMarkdownCodeBlk = regexp.MustCompile("(?s)```[^\\n]*\\n?(.*?)```")
	reMarkdownCode    = regexp.MustCompile("`([^`]+)`")
	reMarkdownHead    = regexp.MustCompile(`(?m)^#{1,6}\s+`)
	reMarkdownBold    = regexp.MustCompile(`\*\*(.+?)\*\*|__(.+?)__`)
	reMarkdownStrike  = regexp.MustCompile(`~~(.+?)~~`)
	reMarkdownQuote   = regexp.MustCompile(`(?m)^>\s?`)
	reMarkdownList    = regexp.MustCompile(`(?m)^(\s*)[-*+]\s+`)
)

const longMarkdownThreshold = 400

// HasMarkdown reports whether the content contains meaningful markdown syntax.
func HasMarkdown(text string) bool {
	trimmed := strings.TrimSpace(text)
	return reMarkdownCodeBlk.MatchString(trimmed) ||
		reMarkdownHead.MatchString(trimmed) ||
		reMarkdownList.MatchString(trimmed) ||
		reMarkdownBold.MatchString(trimmed) ||
		reMarkdownCode.MatchString(trimmed)
}

// ShouldRenderAsImage reports whether markdown content is better rendered as an image.
func ShouldRenderAsImage(text string) bool {
	trimmed := strings.TrimSpace(text)
	return len(trimmed) > longMarkdownThreshold && strings.Contains(trimmed, "```")
}

// MarkdownToPlainText converts markdown content to readable plain text.
func MarkdownToPlainText(text string) string {
	result := text
	result = reMarkdownImage.ReplaceAllString(result, "")
	result = reMarkdownLink.ReplaceAllString(result, "$1")
	result = reMarkdownCodeBlk.ReplaceAllString(result, "$1")
	result = reMarkdownCode.ReplaceAllString(result, "$1")
	result = reMarkdownHead.ReplaceAllString(result, "")
	result = reMarkdownBold.ReplaceAllStringFunc(result, func(match string) string {
		groups := reMarkdownBold.FindStringSubmatch(match)
		if len(groups) < 3 {
			return match
		}
		if groups[1] != "" {
			return groups[1]
		}
		return groups[2]
	})
	result = reMarkdownStrike.ReplaceAllString(result, "$1")
	result = reMarkdownQuote.ReplaceAllString(result, "")
	result = reMarkdownList.ReplaceAllString(result, "${1}• ")
	result = regexp.MustCompile(`\n{3,}`).ReplaceAllString(result, "\n\n")
	return strings.TrimSpace(result)
}

// SplitPlainText breaks text into messaging-friendly chunks.
func SplitPlainText(text string, maxLen int) []string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil
	}
	if maxLen <= 0 || len(trimmed) <= maxLen {
		return []string{trimmed}
	}

	parts := make([]string, 0, (len(trimmed)/maxLen)+1)
	remaining := trimmed
	for len(remaining) > maxLen {
		cut := strings.LastIndex(remaining[:maxLen], "\n")
		if cut < maxLen/2 {
			cut = maxLen
		}
		parts = append(parts, strings.TrimSpace(remaining[:cut]))
		remaining = strings.TrimSpace(remaining[cut:])
	}
	if remaining != "" {
		parts = append(parts, remaining)
	}
	return parts
}

// BuildMarkdownHTML builds a screenshot-friendly HTML document for markdown-like content.
func BuildMarkdownHTML(markdown string) string {
	plain := html.EscapeString(strings.TrimSpace(markdown))
	plain = strings.ReplaceAll(plain, "\t", "    ")
	return `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8" />
<style>
  * { box-sizing: border-box; }
  body {
    margin: 0;
    width: 720px;
    padding: 28px 30px;
    background: linear-gradient(180deg, #fffaf8 0%, #fff 100%);
    color: #171212;
    font-family: "SFMono-Regular", "Cascadia Code", "JetBrains Mono", ui-monospace, monospace;
  }
  .frame {
    border: 1px solid #f0d7de;
    border-radius: 18px;
    background: #ffffff;
    box-shadow: 0 18px 48px -30px rgba(120, 55, 75, 0.45);
    overflow: hidden;
  }
  .header {
    padding: 14px 18px;
    background: linear-gradient(135deg, #fff1f4, #fff8ef);
    border-bottom: 1px solid #f4dde3;
    font: 600 12px/1.4 -apple-system, BlinkMacSystemFont, "PingFang SC", "Segoe UI", sans-serif;
    letter-spacing: 0.12em;
    text-transform: uppercase;
    color: #8f435f;
  }
  pre {
    margin: 0;
    padding: 20px 22px 24px;
    white-space: pre-wrap;
    word-break: break-word;
    font-size: 13px;
    line-height: 1.65;
    color: #241b1d;
  }
</style>
</head>
<body>
  <div class="frame">
    <div class="header">Nekobot WeChat Render</div>
    <pre>` + plain + `</pre>
  </div>
</body>
</html>`
}
