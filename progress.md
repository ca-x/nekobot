# Progress Log

## 2026-03-26

## 2026-04-03

- Completed gateway control-plane hardening phase 10 (endpoint-scoped REST auth):
  - refined the previous role gate into explicit read/manage scopes in `pkg/gateway/server.go` instead of treating every REST control-plane endpoint as equally privileged.
  - allowed `member` tokens to read `/api/v1/status`, `/api/v1/connections`, and `/api/v1/connections/{id}` while keeping `DELETE /api/v1/connections/{id}` restricted to `admin` / `owner`.
  - preserved websocket chat compatibility by continuing to allow any valid authenticated token there, and preserved legacy tokens without a `role` claim as admin-compatible.
- Verification run:
  - `go test -count=1 ./pkg/gateway -run 'Test(Gateway(StatusEndpointAllowsMemberRole|ConnectionsEndpointAllowsMemberRole)|DeleteConnectionEndpointRejectsMemberRole|GetConnectionEndpointAllowsMemberRole|AuthenticateRequestAllowsMemberRoleForWebsocketPath)$'` passed.
  - `go test -count=1 ./pkg/gateway ./pkg/config` passed.

- Continued backlog execution from `task_plan.md`:
  - re-read the persistent planning artifacts and GoalX run state.
  - confirmed the previous GoalX run is complete, while the next approved mainline remains `gateway control-plane hardening`.
  - narrowed the next minimal slice to `gateway.allowed_ips` so gateway gets a shared IP allowlist across websocket handshake and REST control-plane entrypoints.
- Completed gateway control-plane hardening phase 7 (IP allowlist):
  - added `gateway.allowed_ips` to `pkg/config.GatewayConfig` and kept it runtime-reloadable.
  - validated `allowed_ips` entries so blanks and non-IP literals fail fast in config validation instead of deferring to runtime surprises.
  - added a shared `checkClientIP()` gate in `pkg/gateway/server.go` and applied it to both websocket handshake and REST control-plane entrypoints.
  - kept the first version intentionally narrow: exact literal IP matching only, no CIDR, no forwarded-header trust chain, no pairing/scope semantics.
- Verification run:
  - `go test -count=1 ./pkg/gateway ./pkg/config -run 'Test(Gateway(CheckClientIP(AllowsRequestsWhenListUnset|AllowsConfiguredIP|RejectsUnconfiguredIP)|StatusEndpoint(RejectsDisallowedIP|AllowsConfiguredIP)|WSChatRejectsDisallowedIP)|ValidatorRejects(BlankGatewayAllowedIPs|InvalidGatewayAllowedIPs))$'` passed.
  - `go test -count=1 ./pkg/gateway ./pkg/config` passed.
- Completed gateway control-plane hardening phase 8 (per-IP rate limit):
  - added `gateway.rate_limit_per_minute` to `pkg/config.GatewayConfig` and kept it runtime-reloadable.
  - validated negative `rate_limit_per_minute` values as config errors.
  - added shared per-IP limiter state in `pkg/gateway/server.go` and enforced it at both REST control-plane and websocket handshake entrypoints.
  - kept the first version intentionally narrow: remote-IP buckets only, no session/user scope, no pairing integration, no advanced eviction policy.
- Verification run:
  - `go test -count=1 ./pkg/gateway ./pkg/config -run 'Test(Gateway(RateLimit(AllowsRequestsWhenUnset|RejectsSecondRequestFromSameIP|UsesPerIPBuckets)|StatusEndpointRejectsRateLimitedRequest|WSChatRejectsRateLimitedRequest)|ValidatorRejectsNegativeGatewayRateLimitPerMinute)$'` passed.
  - `go test -count=1 ./pkg/gateway ./pkg/config` passed.
- Completed gateway control-plane hardening phase 9 (control-plane auth scope):
  - added a shared gateway JWT parser in `pkg/gateway/server.go` so gateway can consistently read `sub` / `uid` / `role` from existing WebUI-issued tokens.
  - tightened REST control-plane authorization so `/api/v1/status` and connection-management endpoints now require `admin` / `owner`, instead of accepting any valid JWT.
  - preserved websocket chat compatibility by continuing to allow any valid authenticated token there, and preserved legacy control-plane tokens by treating missing `role` claims as admin-compatible.
- Verification run:
  - `go test -count=1 ./pkg/gateway -run 'TestGateway(StatusEndpointRejectsMemberRole|ConnectionsEndpointRejectsMemberRole|AuthenticateRequestAllowsMemberRoleForWebsocketPath)$'` passed.
  - `go test -count=1 ./pkg/gateway ./pkg/config` passed.
- Completed Slack interactive flow phase 3:
  - added a second real shortcut/modal business closure on top of the existing Slack interactive routing skeleton: `settings` shortcut now opens a modal and submission reuses the existing `/settings` command semantics.
  - kept the slice intentionally narrow by treating the modal as a thin UI shell over the already-existing settings command, instead of inventing a parallel Slack-only settings protocol.
  - added regression coverage for settings shortcut modal open and modal submission command execution.
- Verification run:
  - `go test -count=1 ./pkg/channels/slack -run 'TestHandle(ShortcutOpens(Settings|FindSkills)Modal|ViewSubmissionExecutes(Settings|FindSkills)Command)$'` passed.
  - `go test -count=1 ./pkg/channels/slack ./pkg/commands` passed.

- Completed context economy preflight execution telemetry slice:
  - added `preflight.applied` to the shared preflight decision shape so route metadata can distinguish "recommended" from "actually executed".
  - updated both legacy and blades orchestrators to mark `applied=true` only when `compact_before_run` really executes, while warning-only paths stay `false`.
  - extended websocket route payloads and Chat UI rendering so the frontend can show when preflight compaction was actually applied.
- Verification run:
  - `go test -count=1 ./pkg/agent -run 'TestChatWithPromptContextDetailed_(IncludesContextPressurePreview|DoesNotAutoCompressWarningPreflightBeforeModelCall|AutoCompressesCriticalPreflightBeforeBlades)$'` passed.
  - `go test -count=1 ./pkg/webui -run '^TestChatRouteStateJSONIncludesContextPressureFields$'` passed.
  - `go test -count=1 ./pkg/agent ./pkg/webui` passed.

- Completed gateway control-plane hardening phase 2 (REST auth gate):
  - added a shared `requireAuthenticatedAPI()` gate in `pkg/gateway/server.go`.
  - changed `GET /api/v1/status` and `GET /api/v1/connections` to require the same JWT authentication path already used by websocket clients instead of exposing control-plane data anonymously.
  - added regression coverage in `pkg/gateway/server_test.go` for both unauthorized and authorized REST access paths.
- Verification run:
  - `go test -count=1 ./pkg/gateway -run 'Test(Gateway(StatusEndpointRequiresAuth|ConnectionsEndpointRequiresAuth)|StatusEndpoint|ConnectionsEndpoint)$'` failed first, then passed after the fix.
  - `go test -count=1 ./pkg/gateway` passed.
  - `go test -count=1 ./pkg/gateway ./pkg/config` passed.

- Completed gateway control-plane hardening phase 3 (connection delete endpoint):
  - added `DELETE /api/v1/connections/{id}` in `pkg/gateway/server.go` so the control plane can actively terminate a single websocket client connection.
  - kept the endpoint behind the same JWT auth gate used by the other control-plane REST endpoints.
  - added regression coverage in `pkg/gateway/server_test.go` for successful removal, unknown-client `404`, and unauthenticated `401`.
- Verification run:
  - `go test -count=1 ./pkg/gateway -run 'TestDeleteConnectionEndpoint(RemovesClient|ReturnsNotFoundForUnknownClient|RequiresAuth)$'` failed first, then passed after the fix.
  - `go test -count=1 ./pkg/gateway` passed.
  - `go test -count=1 ./pkg/gateway ./pkg/config` passed.

- Completed gateway control-plane hardening phase 4 (stable connection list metadata):
  - extended gateway clients with connection metadata capture for `connected_at` and `remote_addr`.
  - changed `GET /api/v1/connections` to return a stable, sorted connection list with `session_id`, `connected_at`, and `remote_addr` fields instead of depending on Go map iteration order.
  - added regression coverage in `pkg/gateway/server_test.go` for sorted connection output and status reporting with active connections.
- Verification run:
  - `go test -count=1 ./pkg/gateway -run 'Test(ConnectionsEndpoint|StatusEndpointCountsConnectionsDeterministically)$'` failed first, then passed after the fix.
  - `go test -count=1 ./pkg/gateway` passed.
  - `go test -count=1 ./pkg/gateway ./pkg/config` passed.

- Completed gateway control-plane hardening phase 5 (connection detail endpoint):
  - added `GET /api/v1/connections/{id}` in `pkg/gateway/server.go` so the control plane can inspect a single websocket connection without fetching the full list.
  - reused the same JWT auth gate and shared connection serialization shape as the list endpoint.
  - added regression coverage in `pkg/gateway/server_test.go` for success, unknown-client `404`, and unauthenticated `401`.
- Verification run:
  - `go test -count=1 ./pkg/gateway -run 'TestGetConnectionEndpoint(ReturnsConnectionDetails|ReturnsNotFoundForUnknownClient|RequiresAuth)$'` failed first, then passed after the fix.
  - `go test -count=1 ./pkg/gateway` passed.
  - `go test -count=1 ./pkg/gateway ./pkg/config` passed.

- Completed gateway control-plane hardening phase 6 (max connection limit):
  - added `gateway.max_connections` to `pkg/config.GatewayConfig` with `0` meaning unlimited.
  - validated negative `max_connections` values as config errors.
  - added a server-side `checkConnectionLimit()` guard so websocket upgrades are rejected once the configured limit is reached.
  - added regression coverage in `pkg/gateway/server_test.go` and `pkg/config/path_test.go` for limit enforcement and config validation.
- Verification run:
  - `go test -count=1 ./pkg/gateway ./pkg/config -run 'Test(Gateway(RejectsConnectionsAboveConfiguredLimit|AllowsConnectionsWhenLimitUnset)|ValidatorRejectsNegativeGatewayMaxConnections)$'` failed first, then passed after the fix.
  - `go test -count=1 ./pkg/gateway` passed.
  - `go test -count=1 ./pkg/gateway ./pkg/config` passed.

- Completed conversation/thread binding slice 2 (deterministic query order):
  - added `TestServiceBindingQueriesReturnDeterministicConversationOrder` in `pkg/conversationbindings/service_test.go` to lock a real missing contract with TDD.
  - confirmed RED first: `ListBindings()` returned `[chat-b chat-a]` when bindings were created in non-sorted order.
  - updated `pkg/conversationbindings/service.go` so `sessionToBindingRecords`, `ListBindings`, and `GetBindingsBySession` now return a stable order by `conversation_id` instead of leaking metadata write order.
  - verified the existing WeChat runtime consumer remained compatible without code changes.
- Verification run:
  - `go test -count=1 ./pkg/conversationbindings -run 'TestServiceBindingQueriesReturnDeterministicConversationOrder'` failed first, then passed after the fix.
  - `go test -count=1 ./pkg/conversationbindings` passed.
  - `go test -count=1 ./pkg/toolsessions ./pkg/conversationbindings ./pkg/channels/wechat` passed.

- Completed Slack interactive flow phase 2:
  - extended the Slack API abstraction with `OpenView(...)` so shortcut handlers can open modals through the existing channel abstraction.
  - implemented the first real shortcut/modal business flow: `find_skills` shortcut now opens a modal, and `find_skills_modal` submission re-runs the existing `find-skills` command with the submitted query.
  - kept the flow narrow by reusing the existing skill-install confirmation message path instead of inventing a second install-confirm mechanism for modals.
  - added regression coverage in `pkg/channels/slack/slack_test.go` for shortcut modal open and view-submission command execution.
- Verification run:
  - `go test -count=1 ./pkg/channels/slack -run 'TestHandle(ShortcutOpensFindSkillsModal|ViewSubmissionExecutesFindSkillsCommand)'` failed first, then passed after the fix.
  - `go test -count=1 ./pkg/channels/slack` passed.
  - `go test -count=1 ./pkg/channels/slack ./pkg/commands` passed.

- Completed gateway control-plane hardening phase 1 (origin allowlist):
  - added `gateway.allowed_origins` to `pkg/config.GatewayConfig`.
  - replaced the fully-open websocket `CheckOrigin` behavior with a server-level allowlist check.
  - kept requests without an `Origin` header allowed so existing non-browser clients still work.
  - added regression coverage in `pkg/gateway/server_test.go` for allowed and blocked origins, and updated `pkg/config/path_test.go` for the new config field.
- Verification run:
  - `go test -count=1 ./pkg/gateway -run 'TestGatewayCheckOrigin(AllowsConfiguredOrigins|AllowsRequestsWithoutOrigin)'` failed first, then passed after the fix.
  - `go test -count=1 ./pkg/gateway ./pkg/config` passed.

- Completed channel capability matrix phase 1:
  - added `pkg/channels/capabilities.go` to import the core `goclaw` capability model: capability types, scope parsing, enablement checks, merge behavior, and per-channel default matrices.
  - kept this slice intentionally low risk by landing the shared declaration/evaluation layer first without forcing all channel runtimes to consume it in the same commit.
  - added regression coverage in `pkg/channels/capabilities_test.go` for default matrices, scope-based enablement, and override merge behavior.
- Verification run:
  - `go test -count=1 ./pkg/channels -run 'GetDefaultCapabilitiesForChannel|IsCapabilityEnabled|MergeCapabilities'` passed.
  - `go test -count=1 ./pkg/channels` passed.

- Completed browser advanced extraction phase 5 (`get_text`):
  - extended `pkg/tools/browser.go` so the `browser` tool now exposes a `get_text` action for plain-text extraction on top of the existing `get_html` path.
  - imported the lightweight `goclaw` HTML-to-text slice as a local helper instead of adding a new browser tool type or extra dependency surface.
  - added regression coverage in `pkg/tools/browser_test.go` for action exposure and HTML tag stripping.
- Verification run:
  - `go test -count=1 ./pkg/tools -run 'BrowserToolParametersIncludeGetText|HTMLToTextStripsTags'` passed.
  - `go test -count=1 ./pkg/tools` passed.

- Completed browser advanced extraction phase 4 (`extract_structured_data`):
  - extended `pkg/tools/browser.go` so the `browser` tool now exposes an `extract_structured_data` action with `extract_type` modes for `all`, `schema_org`, `json_ld`, and `meta`.
  - imported the `goclaw` structured-data extraction slice by adding reusable extraction-script builders and CDP result formatting directly into the existing `BrowserTool`.
  - added regression coverage in `pkg/tools/browser_test.go` for action exposure and extractor script composition.
- Verification run:
  - `go test -count=1 ./pkg/tools -run 'BrowserTool(ParametersIncludeExtractStructuredData|BuildExtractionScript)'` passed.
  - `go test -count=1 ./pkg/tools` passed.

- Completed browser advanced extraction phase 3 (`print_pdf`):
  - extended `pkg/tools/browser.go` so the `browser` tool now exposes a `print_pdf` action plus PDF-specific parameters for orientation, header/footer, background rendering, and margins.
  - imported the `goclaw` `BrowserPrintToPDF` slice in a `nekobot`-native form by reusing the existing session manager and output directory instead of adding a separate CDP tool type.
  - added regression coverage in `pkg/tools/browser_test.go` for `print_pdf` action exposure and PDF argument construction defaults/overrides.
- Verification run:
  - `go test -count=1 ./pkg/tools -run 'BrowserTool(ParametersIncludePrintPDF|BuildPrintToPDFArgs)'` passed.
  - `go test -count=1 ./pkg/tools` passed.

- Completed MaixCAM outbound response routing:
  - re-audited `pkg/channels/maixcam/maixcam.go` and found slash-command responses were already written back to the active device connection, so the real remaining gap was outbound bus replies being broadcast to every connected device.
  - updated `SendMessage` to parse `maixcam:<device>` session IDs and route outbound replies only to the targeted device connection while preserving broadcast behavior when no device target is present.
  - added regression coverage in `pkg/channels/maixcam/maixcam_test.go` for device-targeted send, broadcast fallback, and session parsing.
- Verification run:
  - `go test -count=1 ./pkg/channels/maixcam -run 'SendMessage(TargetsSessionDevice|BroadcastsWithoutTargetSession)|MaixcamDeviceIDFromSession'` failed first, then passed after the fix.
  - `go test -count=1 ./pkg/channels/maixcam` passed.

- Completed memory quality pack phase 4 (embedding cache):
  - added `pkg/memory/embedding_cache.go` to import the useful `goclaw` LRU cache idea in a `nekobot`-appropriate form: caching embedding vectors by input text instead of redundantly caching objects already held by the in-memory store.
  - updated `pkg/memory/manager.go` so both `Add` and `Search` reuse cached embedding vectors for repeated text, reducing duplicate provider calls while keeping the store and search interface unchanged.
  - added regression coverage in `pkg/memory/manager_cache_test.go` to prove repeated `Add` and repeated `Search` on the same text only invoke the embedding provider once.
- Verification run:
  - `go test -count=1 ./pkg/memory -run 'Manager(SearchCachesQueryEmbeddings|AddCachesEmbeddingsForRepeatedText)'` failed first, then passed after the fix.
  - `go test -count=1 ./pkg/memory` passed.

- Completed browser session migration phase 2:
  - extended `pkg/tools/browser.go` so the `browser` tool schema now exposes a `mode` parameter with `auto/direct` options instead of hiding session startup strategy inside tool internals.
  - wired `navigate` to pass the resolved startup mode into `BrowserSession.StartWithMode`, so callers can explicitly request direct attach semantics while keeping auto-mode reuse as the default.
  - added regression coverage in `pkg/tools/browser_test.go` for default/direct mode parsing and explicit rejection of unsupported modes like `relay` before any browser startup happens.
- Verification run:
  - `go test -count=1 ./pkg/tools -run 'BrowserToolStartMode|BrowserToolExecuteRejectsInvalidMode|BrowserSession|ResolveBrowserMode'` passed.
  - `go test -count=1 ./pkg/tools` passed.

- Completed browser session migration phase 1:
  - extended `pkg/tools/browser_session.go` with explicit `auto/direct` connection modes instead of only a fixed single-path startup flow.
  - added a reuse-first strategy so browser sessions now try to attach to existing Chrome debug ports before falling back to launching a new headless instance.
  - added regression coverage in `pkg/tools/browser_session_test.go` for mode parsing, auto-mode fallback-to-launch, and direct-mode reuse of an existing browser instance.
- Verification run:
  - `go test -count=1 ./pkg/tools -run 'BrowserSession|ResolveBrowserMode'` passed.
  - `go test -count=1 ./pkg/tools` passed.

- Completed memory quality pack phase 3 (temporal decay):
  - added `pkg/memory/temporal_decay.go` to import the core `goclaw` time-aware ranking slice for builtin memory search.
  - extended `pkg/memory/types.go` with `TemporalDecayConfig` and `SearchOptions.TemporalDecay`, then applied temporal decay inside `pkg/memory/manager.go` before MMR so age-adjusted scores feed later diversity re-ranking.
  - added regression coverage in `pkg/memory/search_manager_test.go` for pure decay ordering and manager-level search behavior with temporal decay enabled.
- Verification run:
  - `go test -count=1 ./pkg/memory` passed.

- Completed memory quality pack phase 2 (MMR):
  - added `pkg/memory/mmr.go` to import the core `goclaw` MMR re-ranking slice for builtin memory search.
  - extended `pkg/memory/types.go` with `MMRConfig` and `SearchOptions.MMR`, then applied MMR inside `pkg/memory/manager.go` after raw store search so diversity re-ranking stays isolated from storage code.
  - added regression coverage in `pkg/memory/search_manager_test.go` for direct MMR ordering and manager-level search behavior with MMR enabled.
- Verification run:
  - `go test -count=1 ./pkg/memory` passed.

- Completed memory quality pack phase 1 (citations):
  - added `pkg/memory/citations.go` to import the useful citation-formatting slice from `goclaw` in a way that fits `nekobot`'s existing memory types.
  - extended `pkg/memory/types.go` with `EndLineNumber`, `Timestamp`, and result-level `Citation` / `AgeInDays` fields so later memory-quality slices have a compatible shape.
  - updated `pkg/memory/manager.go` and `pkg/tools/memory.go` so both direct memory context rendering and the memory tool render unified citation strings like `path#Lx-Ly` instead of bare file paths.
  - added regression coverage in `pkg/memory/search_manager_test.go` and `pkg/tools/memory_test.go` for citation decoration and display formatting.
- Verification run:
  - `go test -count=1 ./pkg/memory ./pkg/tools` passed.

- Completed conversation/thread binding migration phase 1:
  - extended `pkg/conversationbindings/service.go` from a thin bind/resolve wrapper into a reusable binding layer with `BindWithOptions`, rich `BindingRecord` views, `GetBinding`, `ListBindings`, `GetBindingsBySession`, and `CleanupExpired`.
  - kept persistence on top of existing tool-session records to avoid schema churn while still importing the useful `goclaw` ideas: binding metadata, target kind/placement, conversation view, and expiry cleanup.
  - tightened `List` behavior so the service only returns sessions that actually match the configured channel + prefix instead of every session from the same source.
  - added regression coverage for filtered listing, metadata-bearing binds, session-based lookup, and expired-binding cleanup; verified WeChat runtime binding tests remain green.
- Verification run:
  - `go test -count=1 ./pkg/conversationbindings` passed.
  - `go test -count=1 ./pkg/channels/wechat` passed.
  - `go test -count=1 ./pkg/toolsessions` passed.

- Completed Tool Sessions / QMD runtime admin smoke pack:
  - added `pkg/webui/server_toolsessions_test.go` as a dedicated WebUI regression pack for tool sessions.
  - covered owner isolation, attach-token create/consume flow, OTP generation + access login, one-time password consumption, process status/input/output/kill flow, terminated-session archival, and tool-event cleanup.
  - re-used existing QMD handler coverage in `pkg/webui/server_status_test.go` as the backend smoke baseline for status/update/install/cleanup behavior, so Batch C now has both prompts and runtime-admin smoke coverage recorded.
- Verification run:
  - `go test -count=1 ./pkg/webui -run 'ToolSession|QMD|Status|Session'` passed.
  - `go test -count=1 ./...` passed.
  - `npm --prefix pkg/webui/frontend run build` could not run in the current shell because `npm` is missing; `pnpm` is present on disk but also fails because `node` is not available on `PATH`.

- Completed Runtime Prompts regression pack and checklist:
  - added `pkg/prompts/manager_test.go` to cover scope override semantics, disabled prompt/binding filtering, session binding replacement, and render-context separation between `system_text` and `user_text`.
  - found and fixed a real bug in `pkg/prompts/manager.go`: when the same prompt was bound in multiple scopes, `Resolve` previously let earlier query order win, so `global` could incorrectly override `channel` or `session`; resolution now explicitly prefers narrower scope, then lower priority.
  - added WebUI regression coverage in `pkg/webui/server_prompts_test.go` for scope override plus render-context fields (`channel`, `route`, `workspace`, `custom`).
  - added `docs/RUNTIME_PROMPTS.md` with behavior notes and a reusable smoke checklist, and linked it from `README.md`.
- Verification run:
  - `go test -count=1 ./pkg/prompts` passed.
  - `GOPROXY=https://goproxy.cn,direct go test -count=1 ./pkg/prompts ./pkg/webui` passed.

## 2026-03-25

- Completed Slack interactive flow phase 1:
  - added Slack-side pending interaction state for skill install confirmations, aligned with Telegram/Discord semantics.
  - changed Slack slash-command skill install confirmation from “send a pseudo inbound message” to “store pending interaction, require same-user confirm/cancel, expire after 15 minutes, re-run the original command with confirmation metadata, and update the original Slack message with the result”.
  - introduced a narrow Slack API interface to make the channel logic testable without live Slack I/O.
  - added placeholder shortcut / view-submission routing hooks so later modal/shortcut flows have a stable entry point instead of being hard-coded into the event switch.
  - added regression tests for pending-state storage, confirm execution path, cancel path, and expiry cleanup in `pkg/channels/slack/slack_test.go`.
- Verification run:
  - `go test -count=1 ./pkg/channels/slack` passed.
  - `GOPROXY=https://goproxy.cn,direct go test -count=1 ./pkg/channels/slack ./pkg/commands` passed.

- Re-baselined the post-WeChat migration plan:
  - confirmed the latest WeChat migration commits are already pushed to `origin/main`.
  - updated `task_plan.md` to mark the WeChat workstream as stage-complete for now and narrowed the next execution target to Slack interactive flow completion.
  - rewrote the Slack backlog item to reflect the real gap: missing pending state, expiry cleanup, message update path, and extensible shortcut/modal routing rather than only “callback exists / not exists”.

- Added WeChat presenter-style output guidance for agent turns:
  - prepended WeChat-specific output rules before user messages so the agent is explicitly told to avoid Markdown and prefer local absolute file paths for rich content.
  - included workspace-root hints in the injected WeChat instructions so generated attachment files have a stable preferred location.
  - added regression tests for presenter prompt assembly.
- Verification run:
  - `GOPROXY=https://goproxy.cn,direct go test -count=1 ./pkg/channels/wechat ./pkg/wechat/... ./pkg/webui` passed.

- Added the first WeChat weak-interaction slice:
  - wired command responses with `commands.InteractionTypeSkillInstallConfirm` into the WeChat channel.
  - added pending interaction state per WeChat user and command-style confirmation handling for `/yes`, `/no`, and `/cancel`.
  - aligned the confirmation execution path with Telegram/Discord by re-running the command with `__confirm_install__ <repo>` and `skill_install_confirmed_repo` metadata.
  - added regression tests for action parsing, pending interaction expiry, confirm execution, deny handling, and prompt rendering.
- Verification run:
  - `GOPROXY=https://goproxy.cn,direct go test -count=1 ./pkg/channels/wechat ./pkg/wechat/... ./pkg/webui` passed.

- Removed the obsolete channel-local WeChat protocol layer:
  - deleted `pkg/channels/wechat/protocol.go` after confirming channel runtime, send path, and WebUI QR bind flow all use shared `pkg/wechat` packages.
  - simplified `pkg/channels/wechat/channel.go` to keep only bot-backed channel glue instead of duplicated client/credential protocol state.
- Verification run:
  - `GOPROXY=https://goproxy.cn,direct go test -count=1 ./pkg/channels/wechat ./pkg/wechat/... ./pkg/webui` passed.

- Completed WeChat channel shared-SDK migration and attachment send pipeline:
  - switched `pkg/channels/wechat` runtime monitor, typing keepalive, outbound text/image/file/video sending, and QR binding helpers to shared `pkg/wechat` SDK primitives.
  - replaced rendered markdown image sending from channel-local inline payloads with shared uploader-based image sending.
  - added outbound file-path extraction/cleanup so reply text can promote local absolute paths into WeChat image/video/file attachments while removing those paths from the final text body.
  - aligned credential storage with shared `pkg/wechat/types.Credentials`.
  - added regression tests for file-path extraction and attachment classification in `pkg/channels/wechat/attachments_test.go`.
- Verification run:
  - `GOPROXY=https://goproxy.cn,direct go test -count=1 ./pkg/channels/wechat ./pkg/wechat/... ./pkg/webui` passed.

- Completed `gua/libc/wechat` SDK baseline migration into `nekobot/pkg/wechat`:
  - added shared `types / client / auth / cdn / messaging / monitor / parse / typing / voice / bot` packages under `pkg/wechat`.
  - kept existing `pkg/channels/wechat` working while introducing the new shared SDK layer, so follow-up channel enhancements can build on stable primitives instead of channel-local protocol code.
- Verification run:
  - `GOPROXY=https://goproxy.cn,direct go test -count=1 ./pkg/wechat/... ./pkg/channels/wechat` passed.

- Re-ordered the WeChat workstream per latest requirement:
  - promoted `gua/libc/wechat` SDK full migration into `nekobot/pkg/wechat` as the current feature slice.
  - moved WeChat attachment/file-path send-path enhancement behind the shared SDK migration.

- Re-scoped the next channel migration slice to WeChat SDK/send-path improvements:
  - switched reference source from `goclaw` to `gua` for WeChat-specific presenter / formatter / upload behavior.
  - identified the highest-value low-risk gap in `nekobot`: outbound replies cannot yet turn local file paths into WeChat image/video/file attachments.
  - updated `task_plan.md` to prioritize a WeChat attachment send pipeline before broader Slack interaction work.

- Implemented subagent completion notification flow and spawn context propagation:
  - Added `pkg/subagent` notification payload + outbound sender abstraction so finished tasks can render origin-channel notifications without coupling the package to the bus implementation.
  - Wired agent startup to enable subagents and bridge notifications into the message bus outbound path.
  - Registered the `spawn` tool in agent runtime and propagated channel/session route context into spawn tool execution.
  - Updated direct channel agent call sites (Telegram / ServerChan / WeChat) to use `ChatWithPromptContext` so tool execution has origin channel/session metadata.
- Added regression tests for:
  - subagent notification rendering/sending (`pkg/subagent/notify_test.go`)
  - spawn context route propagation (`pkg/tools/spawn_test.go`)
  - agent spawn tool registration (`pkg/agent/agent_test.go`)
- Verification run:
  - `GOPROXY=https://goproxy.cn,direct go test -count=1 ./pkg/subagent ./pkg/tools ./pkg/agent` passed.
  - `GOPROXY=https://goproxy.cn,direct go test -count=1 ./...` passed.
- Updated planning artifacts after completing this feature:
  - marked `Subagent 完成通知真正回推 origin channel` complete in `task_plan.md`
  - marked Batch B `Subagent origin notify 接线` complete

- Re-audited `nekobot` against current code, `~/code/goclaw`, and `~/code/gua`, then rewrote the task backlog to distinguish completed baseline vs actual remaining gaps.
- Cleared stale backlog items that are already implemented:
  - `/gateway restart` and `/gateway reload` are implemented in `pkg/gateway/controller.go`.
  - memory hybrid text similarity already exists in `pkg/memory/store.go`.
  - skills version/tool comparison already exists in `pkg/skills/eligibility.go`.
  - cron `at` / `every` / `delete_after_run` / `run-now` already exist in `pkg/cron/*`, WebUI, and CLI.
- Confirmed current stable baseline now includes:
  - Web-first runtime admin for prompts / tool sessions / QMD / skills runtime status
  - provider fallback + cooldown + route override
  - session history sanitize / safe history / context compression
  - multi-path skills with snapshots/versioning
- Added new migration backlog sourced from `goclaw`:
  - general thread/conversation binding layer
  - memory quality pack (MMR / temporal decay / citations / cache)
  - gateway control-plane hardening
  - browser dual-mode session and advanced extraction
  - OAuth credential manager
- Added new migration backlog sourced from `gua`:
  - user-scoped external agent runtime foundation
  - permission / elicitation bridge for external agents
  - WeChat presenter and attachment-output pipeline
  - runtime prompt detection / tmux-style interactive control
  - channel interaction model for weak-interaction platforms
- Updated `task_plan.md` to reflect:
  - completed capabilities
  - real unfinished gaps
  - new Batch A-E execution order
  - rule that each completed feature must be committed and pushed individually

- Added runtime-backed prompt management with Ent schemas for `prompt` and `prompt_binding`, including CRUD, binding resolution, and render helpers in `pkg/prompts`.
- Wired prompt manager into FX/runtime startup and exposed WebUI prompt APIs with server-side tests in `pkg/webui/server_prompts_test.go`.
- Added frontend Prompts page and `usePrompts` hook, plus supporting textarea component and i18n entries.
- Expanded runtime admin flows around tool sessions, config, providers, marketplace, QMD inspection, and status endpoints to support the web-first dashboard model.
- Added workspace-aware QMD path resolution and improved session export defaults/visibility, including resolved export directory and cleanup support.
- Improved skills runtime metadata handling with snapshot/version coverage and added regression tests for snapshot/version behavior.
- Added provider cooldown tests and related runtime integration updates.
- Updated README and QMD docs to reflect the current Web-first setup and Docker/QMD behavior.
- Created and pushed commit `58877a5` (`feat(runtime): add web-managed prompts and tool session controls`).
- Follow-up needed on next device/session:
  - Run `go test -count=1 ./...`
  - Run `npm --prefix pkg/webui/frontend run build`
  - Manually smoke test Prompts page, tool session controls, and QMD admin flow in WebUI

## 2026-02-15

- Initialized planning artifacts for provider DB migration task.
- Inspected current provider backend/frontend implementation paths.
- Confirmed provider CRUD currently depends on config file persistence and draft-based frontend flow.
- Next: implement DB-backed provider store and wire into WebUI handlers.
- Implemented `pkg/providerstore` with SQLite provider CRUD and runtime config sync.
- Wired provider store into WebUI APIs and gateway/CLI startup module graphs.
- Updated dashboard provider dialog: clicking dialog Apply now directly persists provider changes.
- Adjusted storage to reuse the existing single DB file `tool_sessions.db` per user request.
- Verification: `go test ./...` passed.
- Refactored Ent location to a single shared path: moved generated code from `pkg/toolsessions/ent` to `pkg/storage/ent` and updated all imports.
- Re-ran verification after Ent refactor: `go test ./...` passed.
- Implemented runtime config DB store (`config_sections`) on shared `tool_sessions.db` and startup overlay logic.
- Migrated WebUI save paths for init password, channel updates, global config save, and chat routing persistence from file writes to DB writes.
- Added config DB store tests and verified with `go test ./...`.
- Updated `config/config.example.json`, `pkg/config/config.example.json`, and `docs/CONFIG.md` to match latest minimal-file + DB-runtime behavior.
- Replanned config UX: switched dashboard Config from whole-document JSON editing to section-scoped editing with section selector/reset/save.
- Removed outdated provider-in-config snippets from README/docs and aligned examples to bootstrap-only config.
- Added WebUI config section into `/api/config` read/write path and persisted via DB section storage.
- Validation run: `go test ./...` passed.

## 2026-02-28

- Completed memory storage abstraction hardening with `MemoryBackend` implementations for file/db/kv/noop.
- Fixed file backend I/O in `pkg/agent/memory_backend.go` to use `os.MkdirAll` + `os.ReadFile` + `os.IsNotExist` while keeping atomic writes.
- Fixed `NewMemoryStore` fallback typing in `pkg/agent/memory.go` to safely degrade to noop backend when file backend init fails.
- Added `pkg/agent/memory_backend_test.go` to verify KV backend selection, DB backend selection, and KV-unavailable fallback to file backend.
- Verification run: `go test ./pkg/agent ./pkg/config ./cmd/nekobot` passed.
- Added ACP stdio entrypoint command `nekobot acp` with FX wiring and lifecycle management in `cmd/nekobot/acp.go`.
- Extended ACP session state and adapter mapping so `session/new` `mcpServers` are converted into `config.MCPServerConfig` and stored per ACP session.
- Updated blades orchestrator tool resolver path to honor ACP session-level MCP overrides, while keeping existing provider fallback and tool execution flow unchanged.
- Added MCP transport compatibility for ACP `sse` by mapping to blades HTTP transport and expanded config validation to accept `sse` transport values.
- Added ACP adapter tests for session creation/mode/cancel/prompt validation and MCP mapping coverage in `pkg/agent/acp_adapter_test.go`.
- Added ACP `session/update` bridge in adapter `Prompt` flow to emit agent text chunks via ACP connection while preserving existing provider fallback and tool execution semantics.
- Wired ACP adapter to `AgentSideConnection` in CLI startup so session update notifications are available in real ACP runtime.
- Expanded ACP adapter tests to cover session update emission, session update failure/cancel handling, and connection-detach behavior.
- Verification run: `go test ./pkg/agent ./pkg/config ./cmd/nekobot` passed.
- Added ACP `session/load` support in adapter with absolute CWD validation, in-memory session bootstrap, and per-session MCP override mapping.
- Updated ACP initialize capability to advertise `loadSession=true` so ACP clients can restore existing session IDs.
- Added ACP adapter tests for `Initialize` loadSession capability plus `LoadSession` success and validation failure paths.
- Added ACP session model state exposure in `session/new` and `session/load` responses to reflect the session’s active model.
- Implemented ACP experimental `session/set_model` handling with per-session model override updates and validation.
- Expanded ACP adapter tests to cover loaded/new session model state plus `session/set_model` success and invalid-param cases.
- Verification run: `go test -count=1 ./pkg/agent ./pkg/config ./cmd/nekobot` passed.
- Added ACP `current_mode_update` session notifications in `session/set_mode` so clients receive mode-change updates immediately.
- Added ACP adapter tests for `session/set_mode` notification emission plus update failure/cancel handling.
- Verification run: `go test -count=1 ./pkg/agent ./pkg/config ./cmd/nekobot` passed.
- Added trailing user-message dedup normalization in `pkg/agent/context.go` (`BuildMessages`) to avoid double-injecting the current prompt when callers pre-append user turns.
- Applied the same trailing user-message normalization in `pkg/agent/blades_runtime.go` before hydrating blades session history, keeping blades runtime behavior aligned with legacy prompt construction semantics.
- Added regression tests in `pkg/agent/agent_test.go` for trailing-current-user dedup and non-matching history preservation.
- Verification run: `go test -count=1 ./pkg/agent ./pkg/gateway ./pkg/webui ./cmd/nekobot ./pkg/config` passed.
- Note: full `go test -count=1 ./...` still fails in `pkg/cron` with known upstream `fatal error: concurrent map writes` in Ent atlas migration path (unchanged by this batch).
- Fixed runtime Ent schema migration race by serializing `EnsureRuntimeEntSchema` calls with a package-level mutex in `pkg/config/runtime_client.go`.
- Added regression test `TestEnsureRuntimeEntSchemaConcurrentCalls` in `pkg/config/db_store_test.go` to verify concurrent schema init no longer fails.
- Verification run: `go test -count=1 ./pkg/config ./pkg/cron` passed.
- Verification run: `go test -count=1 ./...` passed.
- Added deterministic tool description ordering in `pkg/agent/context.go` by sorting tool descriptions before assembling the tools section, improving prompt cache stability.
- Added regression test `TestBuildToolsSection_SortsToolDescriptionsDeterministically` in `pkg/agent/agent_test.go`.
- Verification run: `go test -count=1 ./pkg/agent` passed.
- Verification run: `go test -count=1 ./...` passed.
- Wired `Agent.callLLMWithFallback` to provider failover semantics (`providers.ClassifyError` + shared `CooldownTracker`) so retriable failures continue fallback and non-retriable format errors fail fast.
- Added provider cooldown skip behavior in agent fallback path, including contextual logging for skip reason and remaining cooldown window.
- Added agent failover regression tests for retriable fallback continuation, non-retriable short-circuit behavior, and cooldown-based skip on subsequent attempts (`pkg/agent/agent_test.go`).
- Added provider failover/cooldown regression tests covering cooldown skipping, non-retriable stop, reason tracking, all-cooldown exhaustion, and 24h failure-window reset (`pkg/providers/loadbalancer_test.go`).
- Added `always` frontmatter support in skills metadata (`pkg/skills/manager.go`) with eligibility-aware always-skill selection.
- Updated skill prompt assembly to emit a dedicated `Always Skills` XML section and keep regular skills in deterministic name order.
- Added compatibility parsing for `metadata.openclaw.always` in `pkg/skills/loader.go`.
- Extended validation with `ValidateAlways` to warn when always-on skills are disabled.
- Added regression tests for always-skill loading, prompt rendering, and validation (`pkg/skills/manager_test.go`, `pkg/skills/loader_test.go`).
- Updated docs with `always` field and Always Skills behavior (`docs/CONFIG.md`).
- Verification run: `go test -count=1 ./pkg/skills ./pkg/agent` passed.
- Continued Skills follow-up: switched `# Available Skills` from full markdown bodies to compact XML summary (`<skills><skill ... /></skills>`) in `pkg/skills/manager.go`, keeping Always Skills full XML instructions unchanged.
- Added `instructions_length` summary field using rune count for deterministic lightweight metadata and better token budgeting hints.
- Added/updated regression tests for compact summary output and non-ASCII length handling in `pkg/skills/manager_test.go`.
- Verification run: `go test -count=1 ./pkg/skills ./pkg/agent` passed.
- Verification run: `go test -count=1 ./...` passed.
- Added WebUI Cron API routes and handlers in `pkg/webui/server.go` for list/create/delete/enable/disable/run-now operations.
- Added structured error logging for Cron mutation failures in WebUI handlers (delete/enable/disable/run).
- Added WebUI Cron handler tests in `pkg/webui/server_cron_test.go` covering unavailable manager, CRUD flow, invalid RFC3339 `at_time`, not-found run-now, and disabled-job run-now behavior.
- Added frontend Cron integration with new hooks and page (`pkg/webui/frontend/src/hooks/useCron.ts`, `pkg/webui/frontend/src/pages/CronPage.tsx`), plus routing/sidebar wiring in `pkg/webui/frontend/src/App.tsx` and `pkg/webui/frontend/src/components/layout/Sidebar.tsx`.
- Added Cron i18n strings in `pkg/webui/frontend/public/i18n/en.json`, `pkg/webui/frontend/public/i18n/zh-CN.json`, and `pkg/webui/frontend/public/i18n/ja.json`.
- Verification run: `go test ./pkg/webui ./pkg/cron ./pkg/agent ./pkg/config ./cmd/nekobot` passed.
- Verification run: `go test ./...` passed.
- Verification run: `npm --prefix pkg/webui/frontend run build` passed (after installing frontend deps with `npm --prefix pkg/webui/frontend ci`).
- Added CLI command `nekobot cron run <job-id>` to trigger immediate execution for existing jobs.
- Aligned blades runtime tool error semantics with legacy orchestrator: tool execution failures now return `Error: ...` tool results instead of aborting the whole run.
- Added blades runtime regression tests for tool-error result fallback and role/parts mapping (`pkg/agent/agent_test.go`).
- Updated architecture docs for Cron capabilities to reflect DB-backed persistence and run-now support.
- Verification run: `go test ./pkg/agent ./pkg/cron ./cmd/nekobot` passed.
- Verification run: `go test ./...` passed.
- Added CLI regression tests for `nekobot cron run` command wiring and arg validation in `cmd/nekobot/cron_test.go`.
- Verification run: `go test ./cmd/nekobot` passed.
- Verification run: `go test ./...` passed.
- Fixed blades tool-result history conversion in `pkg/agent/blades_runtime.go`: when hydrating prior `RoleTool` messages, each `blades.ToolPart` now maps to its own `providers.UnifiedMessage` so multiple tool results in one blades message are preserved for provider context reconstruction.
- Added regression tests for blades tool history conversion in `pkg/agent/agent_test.go`:
  - `TestBladesModelProvider_ConvertMessagesPreservesMultipleToolResults`
  - `TestBladesModelProvider_ConvertMessagesToolFallbackToRequest`
- Verification run: `go test ./pkg/agent` passed.
- Verification run: `go test ./...` passed.
- Feature Batch #2 收口：`chatWithBladesOrchestrator` 会话历史注入现在保留 assistant 的 tool-calls turns（即使 text 为空），避免在重建 blades history 时丢失工具调用上下文，保证与 legacy 的 tool 执行链路语义一致。
- Added `hasBladesHistoryContent` + enhanced `toBladesMessage` in `pkg/agent/blades_runtime.go` to preserve assistant tool call metadata when hydrating history into blades session.
- Added regression tests in `pkg/agent/agent_test.go`:
  - `TestToBladesMessage_AssistantToolCallsPreserved`
  - `TestHasBladesHistoryContent`
- Verification run: `go test -count=1 ./pkg/agent` passed.
- Implemented static prompt caching in `pkg/agent/context.go` with file-state/tool-signature invalidation and dynamic current-time substitution to reduce repeated full prompt rebuilds while keeping fresh time output.
- Added context prompt regression tests in `pkg/agent/agent_test.go` for current-time placeholder replacement plus cache invalidation on bootstrap file and tool-description changes.
- Verification run: `go test -count=1 ./pkg/agent` passed.
- Verification run: `go test -count=1 ./...` passed.

## 2026-03-29

- Completed harness feature review and fixes:
  - Reviewed 5 recent commits: `1b7c3d0`, `580741d`, `46026ac`, `583245d`, `c409cf1`
  - Fixed snapshot incremental delta calculation to avoid message duplication
  - Added snapshot JSONL rewrite after `Undo()` for persistence
  - Added session ID passthrough for streaming updates
  - Added explicit fallback notice for streaming without handler
  - Changed watcher path matching to use RLock for better concurrency
  - Updated ConfigPage section metadata with proper i18n labels
  - Committed and pushed all fixes as `7412fdd`
- Verification run:
  - `GOPROXY=https://goproxy.cn,direct go test -count=1 ./...` passed
  - `npm --prefix pkg/webui/frontend run build` passed
- Updated task_plan.md:
  - Marked Batch A verification complete
  - Marked Batch C frontend build and backend test complete
  - Marked 2026-03-29 review batch Phase 4 complete

- Added missing i18n labels for harness config sections:
  - Added audit, undo, preprocess, learnings, watch section labels
  - Added translations for en, zh-CN, ja
  - Committed as `28a8093`
- Added line range support for @file mentions:
  - Support @file.txt:50-100 syntax for extracting specific line ranges
  - Added extractLineRange method for line-based content extraction
  - Updated formatFileReference to show line range in output
  - Added test cases for line range functionality
  - Committed as `6707d22`
- Verification run:
  - `go test -count=1 ./pkg/preprocess` passed
  - `npm --prefix pkg/webui/frontend run build` passed
