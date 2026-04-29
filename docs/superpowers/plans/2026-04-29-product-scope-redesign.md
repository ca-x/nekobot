# Nekobot Product Scope Redesign

Date: 2026-04-29

## Goal

Reposition Nekobot from a broad all-in-one AI assistant platform into a focused multi-user collaboration hub for web chat, message channels, lightweight scheduled notifications, prompt management, built-in skills, and daemon-managed code agents.

This plan follows the latest product direction:

- Multi-user first.
- Web chat and channel/thread collaboration are the primary user surfaces.
- Message channels such as WeChat and Telegram remain first-class notification and reply paths.
- Daemon clients create computers, discover code agents on those computers, and register those agents as collaboration participants.
- Direct WebUI invocation of Codex, Claude Code, and web terminals should move out of the web product surface and be handled by daemon clients instead.
- Skills remain as built-in/private/shared operational capability, while the public skills marketplace is removed.

## Target Product Boundary

### Keep

| Area | Target shape |
| --- | --- |
| Users and tenancy | Multiple users, memberships, roles, and resource ownership. |
| Web chat | User-facing chat, channel/thread views, message history, mentions, and notifications. |
| Message channels | Existing WeChat, Telegram, Feishu, DingTalk, Slack, etc. as notification/reply transports. |
| Notification routing | Per user, per channel/thread, and per scheduled job notification targets. |
| Lightweight scheduled jobs | Simple reminders/scheduled bot prompts with selected notification channels. |
| Prompts | User-owned or shared prompt templates, bindable to chat/channel/thread/agent scopes. |
| Skills | Built-in skills plus private/shared local skills. No remote marketplace/install flow in the product UI. |
| Daemon | Computers, daemon clients, discovered code agents, agent profile/env/skills/DMs/activity/reminders. |
| Audit/activity | Lightweight activity timeline for daemon and collaboration operations. |

### Remove From Main Product Surface

| Area | Decision |
| --- | --- |
| Web terminal and tool sessions | Remove from WebUI. Interactive terminal belongs to daemon-side tools if needed. |
| Direct WebUI Codex/Claude Code launch | Remove from WebUI. Code agents are discovered and controlled through daemon clients. |
| Skills marketplace | Remove public marketplace/search/install/version UI. Keep built-in/private/shared skill management. |
| Goal-run/harness experiments | Remove from primary navigation and reassess as internal/dev-only tooling. |
| Provider/model tuning surface | Keep only the minimum needed for web chat and scheduled jobs; do not make it the center of the product. |
| Long-term memory/QMD | Default to off or remove from user-facing flow unless a concrete per-user value is proven. |
| Webhooks/policy/permission-rule advanced panels | Collapse or hide unless required by the simplified product surface. |

## Target Architecture

<style scoped>
.scope-arch{font-family:Inter,ui-sans-serif,system-ui,sans-serif;border:1px solid #d8dee9;border-radius:10px;padding:14px;background:#f8fafc;color:#172033}.scope-layer{border-radius:8px;border:1px solid #cbd5e1;background:white;margin:10px 0;padding:12px}.scope-title{font-weight:700;font-size:14px;margin-bottom:8px}.scope-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:8px}.scope-box{border:1px solid #d8dee9;border-radius:7px;background:#f8fafc;padding:8px;font-size:12px;line-height:1.35}.scope-box strong{display:block;font-size:12px;margin-bottom:3px}.scope-user{border-left:5px solid #2563eb}.scope-app{border-left:5px solid #059669}.scope-agent{border-left:5px solid #7c3aed}.scope-data{border-left:5px solid #d97706}.scope-ext{border-left:5px solid #64748b}
</style>
<div class="scope-arch"><div class="scope-layer scope-user"><div class="scope-title">User and Channel Layer</div><div class="scope-grid"><div class="scope-box"><strong>Web Users</strong>Chat, threads, prompts, schedules, channel settings</div><div class="scope-box"><strong>External Channel Users</strong>WeChat, Telegram, Feishu, DingTalk replies mapped back to targets</div><div class="scope-box"><strong>Daemon Agents</strong>Code agents represented as collaboration identities</div></div></div><div class="scope-layer scope-app"><div class="scope-title">Collaboration Core</div><div class="scope-grid"><div class="scope-box"><strong>Targets</strong>#channel, #channel:thread, dm:@agent, dm:@user</div><div class="scope-box"><strong>Messages</strong>Read, send, search, attachments, mentions</div><div class="scope-box"><strong>Tasks</strong>Create, claim, update, review workflow</div><div class="scope-box"><strong>Notifications</strong>User/channel/thread/job routing rules</div><div class="scope-box"><strong>Prompts</strong>Private/shared templates and scope bindings</div><div class="scope-box"><strong>Schedules</strong>Reminder and simple bot notification jobs</div></div></div><div class="scope-layer scope-agent"><div class="scope-title">Daemon Control Plane</div><div class="scope-grid"><div class="scope-box"><strong>Computers</strong>Daemon install, registration, heartbeat, inventory</div><div class="scope-box"><strong>Code Agents</strong>Discovered runtimes, profiles, env, installed skills</div><div class="scope-box"><strong>Protocol</strong>Slock-like server info, messages, tasks, reminders, activity</div></div></div><div class="scope-layer scope-data"><div class="scope-title">Data and Policy Layer</div><div class="scope-grid"><div class="scope-box"><strong>Tenant/User Scope</strong>Owner, tenant, visibility, role checks on resources</div><div class="scope-box"><strong>Resource Store</strong>Channels, threads, messages, prompts, jobs, daemon state</div><div class="scope-box"><strong>Audit</strong>Activity log and security-relevant changes</div></div></div><div class="scope-layer scope-ext"><div class="scope-title">External Services</div><div class="scope-grid"><div class="scope-box"><strong>LLM Providers</strong>Minimal web-chat/job execution backend</div><div class="scope-box"><strong>Channel APIs</strong>Messaging and notification delivery</div><div class="scope-box"><strong>Daemon Machines</strong>Local code-agent execution environment</div></div></div></div>

## Domain Model Changes

### User and Ownership

Current state already has `User`, `Tenant`, and `Membership`, but most runtime resources are still global. The redesign should make ownership explicit:

- Add `tenant_id`, `owner_user_id`, and `visibility` where appropriate.
- Visibility values: `private`, `shared`, `system`.
- Shared resources belong to a tenant, not to the whole install.
- System resources are built-in and read-only unless the user copies them.

Resources needing ownership:

- Channel accounts.
- Prompt templates and prompt bindings.
- Cron/reminder jobs.
- Skills metadata for private/shared local skills.
- Daemon computers and agents.
- Threads, channels, tasks, messages, and notification routes.

### Collaboration Targets

Use one target grammar everywhere:

- `#channel`
- `#channel:thread`
- `dm:@user`
- `dm:@agent`

Targets are stable external identifiers. Internally each target should map to a row with:

- `target`
- `kind`
- `tenant_id`
- `owner_user_id` where applicable
- `notification_policy_id` optional
- `created_at`, `updated_at`

### Notification Routes

Notifications should be a first-class resource, not ad-hoc fields inside each feature.

Suggested model:

- `notification_route`: owner, tenant, name, channel_account_id, target_config_json, enabled.
- `notification_binding`: scope (`user`, `channel`, `thread`, `job`, `agent`), target, route_id, event types, enabled.
- Incoming replies from the channel account must resolve back to the bound collaboration target and append a message there.

This gives the user one mental model: configure channel accounts once, then bind them where notifications are needed.

### Scheduling

Cron should be renamed in the product surface to scheduled notifications or reminders. Keep the backend cron engine, but reduce the UI contract:

- Name.
- Schedule: once, interval, cron.
- Message/prompt.
- Optional agent/bot runner.
- Notification routes.
- Enable/disable/delete.
- Last run and next run.

Provider/model/fallback details should be hidden behind defaults unless advanced mode is explicitly enabled.

### Skills

Keep:

- Built-in embedded skills.
- Private skills owned by a user.
- Shared skills in a tenant.
- Skill enable/disable and requirement visibility.

Remove:

- Remote marketplace page.
- Remote search/install/version cleanup as a first-class WebUI feature.

The loader can keep local multi-path behavior, but the WebUI should present skills as built-in/private/shared, not as a marketplace.

### Memory

Memory is not part of the core product positioning right now. Recommended decision:

- Keep session/thread history because it is core collaboration data.
- Disable long-term memory and QMD by default.
- Hide memory configuration from primary settings.
- Reassess later only if there is a clear per-user workflow such as "remember my preferences across web chats".

## WebUI Navigation Target

Primary navigation should collapse to:

1. Chat
2. Channels
3. Threads
4. Schedules
5. Prompts
6. Skills
7. Daemon
8. Settings
9. System

Remove or hide from normal navigation:

- Tool sessions.
- Marketplace.
- Providers and models as standalone primary pages.
- Permission rules.
- Harness audit.
- Runtime topology.
- Webhooks.
- Policy.
- Goal runs.

Provider/model settings can move under Settings as a compact "AI backend" section.

Daemon should become its own primary page instead of being buried in System. The page should show:

- Install/bootstrap command.
- Computers.
- Discovered code-agent runtimes per computer.
- Agent profiles, env, skills.
- Agent DMs and activity.
- Thread/channel access and invitation status.

## Daemon Protocol Direction

The new `daemon.proto` work should remain the foundation. The next iteration should harden it around product concepts:

- Treat computer registration and agent registration as separate resources.
- Agents have identity, display name, skills, env, activity, status, and membership in channels/threads.
- Agent access to a thread/channel is granted by invite link or explicit user action.
- Every daemon operation should use target-based addressing.
- Replies through WeChat/Telegram/etc. should land in the same thread/channel target as web replies.

Do not preserve old RPCs if they conflict with the collaboration model. There are no existing external installs, so compatibility is not a constraint.

## Implementation Phases

### Phase 0: Lock Product Contract

Deliverables:

- This scope document.
- A route/resource inventory marking keep, move, hide, remove.
- A schema ownership inventory for tenant/user scoping gaps.

Verification:

- No code behavior changes.
- Human review confirms product boundary.

### Phase 1: Navigation and Surface Cleanup

Deliverables:

- Add feature flag or product mode for the simplified navigation.
- Move provider/model settings under Settings.
- Hide/remove Tool Sessions, Marketplace, Harness, Runtime Topology, Webhooks, Policy, Goal Runs from primary nav.
- Create dedicated Daemon page skeleton using current daemon/system data.

Tests:

- Frontend route smoke tests where available.
- `npm run build`.
- `go test ./pkg/webui ./pkg/gateway`.

### Phase 2: Multi-User Resource Scoping

Deliverables:

- Add owner/tenant/visibility fields to runtime resources.
- Add request-scoped auth context helpers.
- Apply tenant/user filters to prompts, channels, schedules, daemon computers/agents, tasks, and messages.
- Add migration defaults for existing global data to the initial owner/default tenant.

Tests:

- API tests for user A cannot read/write user B private resources.
- Shared resources are visible to users in the same tenant.
- Admin/owner role can manage tenant-shared resources.

### Phase 3: Notification Routing

Deliverables:

- Add notification route and binding models.
- Allow scheduled jobs, channels, threads, and daemon events to select notification routes.
- Map inbound replies from message channels back to collaboration targets.

Tests:

- A scheduled job posts to the selected target and sends through selected channel account.
- A reply from Telegram/WeChat appends to the original thread/channel.
- Disabled route stops delivery without deleting history.

### Phase 4: Skills Simplification

Deliverables:

- Remove marketplace WebUI and API endpoints from primary product.
- Keep built-in/private/shared local skills.
- Add visibility/ownership to skill records or skill metadata.
- Keep the loader simple and local.

Tests:

- Built-in skills visible to all users.
- Private skills visible only to owner.
- Shared skills visible to tenant members.
- No marketplace API is required by the frontend bundle.

### Phase 5: Daemon Productization

Deliverables:

- Dedicated Daemon page for computers and agents.
- Agent invite/access flow for channels and threads.
- Agent DMs and activity timeline.
- Explicit daemon token model without prefix heuristics.
- Thread ownership model that does not assume `#websocket`.

Tests:

- Computer registers, heartbeats, and reports inventory.
- Code agents are discovered and shown under the computer.
- Agent can read/send messages to an authorized thread.
- Agent cannot access unauthorized targets.

### Phase 6: Remove Heavy Legacy Surfaces

Deliverables:

- Delete unused frontend pages/components/hooks for removed surfaces.
- Delete backend endpoints and modules only after verifying no retained surface imports them.
- Keep provider/model core only if required for web chat/schedules.
- Decide whether memory/QMD stays disabled-hidden or is removed.

Tests:

- `go test ./...`.
- Frontend typecheck/build.
- Search confirms removed routes are not linked.
- Smoke test login, chat, channels, schedules, prompts, daemon.

## Risk Notes

- Multi-user support is partially present but not consistently enforced. The biggest risk is accidentally leaving global resources readable across users.
- Existing channel account ownership is mixed into metadata in some paths; it should become first-class schema.
- Removing Web terminal and direct code-tool launch is a product simplification, but daemon must provide a complete replacement path for code-agent control.
- Memory/QMD should not be deleted before deciding whether thread history needs any summarization or preference recall.
- Provider/model internals may still be needed by web chat and scheduled jobs, even if their standalone pages are removed.

## Acceptance Criteria

The redesign is complete when:

- A normal user can log in, configure their channels, create prompts, create scheduled notifications, chat in WebUI, and manage daemon agents without seeing terminal/marketplace/provider complexity.
- A daemon agent can participate in channels/threads through the same target/message/task/reminder/activity model used by web users.
- Notifications can be bound to users, threads, channels, scheduled jobs, and daemon events.
- Private vs shared resources are enforced consistently by tests.
- Removed surfaces have no dead navigation, stale frontend routes, or unused backend endpoints exposed in normal product mode.

## Appendix A: Current Route Inventory

| Route | Current page | Decision | Notes |
| --- | --- | --- | --- |
| `/chat` | ChatPage | Keep | Primary web chat surface. |
| `/threads` | ThreadsPage | Keep and elevate | Should become the center of channel/thread collaboration. |
| `/sessions` | SessionsPage | Keep only if mapped to threads | Avoid separate mental model; sessions should be implementation detail or history view. |
| `/tools` | ToolSessionsPage | Remove from product nav | Web terminal/tool sessions move to daemon-side code-agent control. |
| `/providers` | ProvidersPage | Move under Settings | Needed as backend config, not primary product surface. |
| `/models` | ModelsPage | Move under Settings | Same as providers. |
| `/permission-rules` | PermissionRulesPage | Hide/internal | Reassess after daemon permission model settles. |
| `/channels` | ChannelsPage | Keep | User/channel account configuration remains core. |
| `/marketplace` | MarketplacePage | Remove | Public skills marketplace is out of scope. |
| `/prompts` | PromptsPage | Keep | Add private/shared/system visibility model. |
| `/config` | ConfigPage | Keep as Settings | Reduce sections to product-relevant settings. |
| `/harness/audit` | HarnessAuditPage | Hide/internal | Not part of end-user product. |
| `/runtime-topology` | RuntimeTopologyPage | Replace with Daemon page | Runtime topology should be expressed as computers and code agents. |
| `/cron` | CronPage | Keep but rename | Product label should be Schedules or Reminders. |
| `/webhooks` | WebhooksPage | Hide/remove | Only keep if notification routing requires it later. |
| `/policy` | PolicyPage | Hide/internal | Policy should be embedded in daemon/channel permission flows. |
| `/goal-runs` | GoalRunsPage | Remove from primary product | Too broad for the new scope. |
| `/goal-runs/:id` | GoalRunDetailPage | Remove from primary product | Same as goal runs. |
| `/system` | SystemPage | Keep for ops status | Move daemon UX out to dedicated page. |

## Appendix B: Resource Ownership Inventory

| Resource/schema | Current ownership state | Required change |
| --- | --- | --- |
| `User`, `Tenant`, `Membership` | Present | Use consistently in API filters and resource writes. |
| `ChannelAccount` | Global with some owner metadata in channel-specific flows | Add first-class `tenant_id`, `owner_user_id`, `visibility`. |
| `AgentRuntime` | Global | Add computer relationship, tenant/user ownership, and visibility. |
| `CronJob` | Global | Add tenant/user ownership and notification-route bindings. |
| `Prompt` | Global | Add tenant/user ownership and private/shared/system visibility. |
| `PromptBinding` | Global target binding | Add tenant scope and validate target access. |
| `Provider`, `ModelRoute`, `ModelCatalog` | Global | Keep global/admin-level unless per-user AI backend configuration becomes required. |
| `PermissionRule` | Global with runtime/session fields | Reassess after daemon authorization model; hide until then. |
| `ToolSession`, `ToolEvent` | Existing but out of product scope | Remove/hide WebUI surface; keep only if daemon internals still use it. |
| Session/thread/message stores | Mixed file/db managers | Normalize around collaboration targets and tenant/user access. |
| Skills | File paths and manager state | Add metadata for built-in/private/shared visibility; remove marketplace dependency from UI. |

## Appendix C: First Code Change Candidates

1. Add a `PRODUCT_SCOPE.md` link from README once this plan is accepted.
2. Introduce a frontend `PRODUCT_NAV_ITEMS` list matching the target navigation before deleting pages.
3. Create a dedicated `/daemon` route using current System daemon data, then remove daemon panels from System.
4. Add schema fields for `tenant_id`, `owner_user_id`, and `visibility` to `Prompt`, `ChannelAccount`, `CronJob`, and `AgentRuntime`.
5. Add API tests for private/shared prompt and channel-account visibility before broadening the pattern.
