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
	var builder strings.Builder
	builder.WriteString(wechatPresenterInstructions)

	if trimmedWorkspace := strings.TrimSpace(workspace); trimmedWorkspace != "" {
		builder.WriteString("\n\n[Workspace]\n")
		_, _ = fmt.Fprintf(&builder, "Preferred workspace root: %s", filepath.Clean(trimmedWorkspace))
	}

	if trimmedContent == "" {
		return strings.TrimSpace(builder.String())
	}

	builderWithMessage := strings.Builder{}
	builderWithMessage.WriteString("[WeChat Channel Instructions]\n")
	builderWithMessage.WriteString(builder.String())

	builderWithMessage.WriteString("\n\n[User Message]\n")
	builderWithMessage.WriteString(trimmedContent)

	return builderWithMessage.String()
}
