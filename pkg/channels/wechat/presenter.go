package wechat

import (
	"fmt"
	"path/filepath"
	"strings"
)

const wechatPresenterInstructions = `WeChat does not render Markdown. Use plain text only.
For rich content such as code blocks longer than 5 lines, tables, SVG, Mermaid, or formatted reports, write the content to a local file under the current workspace or /tmp and include the absolute file path in your reply.
When a local absolute file path appears in your reply, the system may send it as a WeChat attachment automatically.
Short plain text can be returned directly.`

func buildWeChatAgentInput(content, workspace string) string {
	trimmedContent := strings.TrimSpace(content)
	if trimmedContent == "" {
		return strings.TrimSpace(wechatPresenterInstructions)
	}

	var builder strings.Builder
	builder.WriteString("[WeChat Channel Instructions]\n")
	builder.WriteString(wechatPresenterInstructions)

	if trimmedWorkspace := strings.TrimSpace(workspace); trimmedWorkspace != "" {
		builder.WriteString("\n\n[Workspace]\n")
		builder.WriteString(fmt.Sprintf("Preferred workspace root: %s", filepath.Clean(trimmedWorkspace)))
	}

	builder.WriteString("\n\n[User Message]\n")
	builder.WriteString(trimmedContent)

	return builder.String()
}
