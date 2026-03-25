# Runtime Prompts

`Runtime Prompts` 用于在运行时管理可复用 prompt 模板，以及把它们绑定到：

- `global`: 全局生效。
- `channel`: 对单一 channel 生效。
- `session`: 对单一 chat session 生效。

解析结果会拆分成：

- `system_text`
- `user_text`
- `applied`

同一个 prompt 同时存在多层绑定时，作用域优先级为：

1. `session`
2. `channel`
3. `global`

同一作用域内再按 `priority` 升序生效。

## 回归范围

当前回归重点覆盖：

- Prompt CRUD。
- Binding CRUD。
- Session binding replace / cleanup。
- 模板渲染上下文。
- 同一 prompt 的多作用域覆盖优先级。
- disabled prompt / disabled binding 忽略行为。

自动化回归入口：

```bash
GOPROXY=https://goproxy.cn,direct go test -count=1 ./pkg/prompts ./pkg/webui
```

## Smoke Checklist

每次修改 `pkg/prompts/*`、`pkg/webui` prompts API、或前端 Prompts 页面后，至少执行以下检查：

1. 创建一个 `system` prompt，模板中包含 `{{channel.id}}`、`{{session.id}}`、`{{route.provider}}`、`{{workspace.path}}`、`{{custom.xxx}}`。
2. 创建一个 `user` prompt，确认 resolve 后进入 `user_text` 而不是 `system_text`。
3. 为同一个 prompt 同时创建 `global` 和 `channel` 绑定，确认目标 channel 下最终只保留一份 `applied`，且命中更窄的 `channel` 绑定。
4. 将一个 binding 设为 disabled，确认 resolve 结果不再包含该 binding。
5. 将一个 prompt 设为 disabled，确认 resolve 结果不再包含该 prompt。
6. 对 `session` prompt 集合执行 replace，确认旧绑定被清空、新绑定按 system/user 分组重建。
7. 删除 prompt 后，确认其关联 bindings 被一并清理。
8. 删除 chat session 后，确认 session-scoped prompt bindings 被清理。
9. WebUI Prompts 页面确认可以：
   - 列出 prompt。
   - 创建 prompt。
   - 创建 binding。
   - 删除 prompt / binding。
10. 如本次涉及前端改动，再执行：

```bash
npm --prefix pkg/webui/frontend run build
```

## 备注

- `webui-chat` 是一个别名，后端会解析为当前登录用户对应的真实 WebUI session ID。
- 模板渲染采用 `missingkey=error`，新增模板字段时必须同步更新渲染上下文，否则 resolve 会直接报错。
