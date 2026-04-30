# Slock-style Message, Board, and Attachment API Design

Status: next-step design from screenshots (task #33)

Related:
- `docs/superpowers/specs/2026-04-30-daemon-collaboration-protocol-v2.md`
- `docs/superpowers/specs/2026-04-30-daemon-idempotency-event-replay-design.md`
- task #24 channel task board aggregation
- task #26 saved chat messages
- task #30 attachment gap review

## Goal

The Slock screenshots show three user-facing collaboration surfaces that must become first-class Nekobot protocol and WebUI API concepts:

- Channel chat with per-message actions.
- Channel task board with status columns, creator/assignee visibility, and task-to-chat navigation.
- Attachments that are visible in chat, tasks, saved items, and agent replay.

These should not be bolted onto the current direct-message fix. They are the next protocol/API slice after Milestone 1 closes.

## Required User Experience

### Channel Shell

Each channel view has:

- Chat tab.
- Tasks tab.
- Saved entry in the left navigation.
- Channel/member context in the sidebar.

The protocol already has enough target grammar to represent this:

- `#channel`
- `#channel:thread`
- `dm:@agent_or_user`

The missing piece is a set of board and saved-message APIs that use the same target model consistently.

### Message Actions

Every rendered chat message should expose at least two actions:

- `reply in thread`
- `save message`

`reply in thread` reuses thread/reply semantics:

- It creates or opens the message thread.
- It should preserve `reply_to_message_id`.
- It should navigate to the thread target for the message.

`save message` is a separate saved/favorite state:

- It records a user-scoped saved item.
- It can be toggled without changing message content.
- It appears in the Saved view.
- It emits replayable events.

### Saved View

The Saved view lists user-saved messages across visible channels, threads, and DMs.

Each saved item should show:

- Channel/thread target.
- Message sender and sender kind.
- Content excerpt.
- Attachment preview metadata when present.
- Saved time and original message time.
- Link back to the original chat/thread/message.

### Task Board

The Tasks tab should support:

- All channels.
- Single channel.
- Single thread or DM.
- Status columns: `All`, `TODO`, `IN PROCESS`, `IN REVIEW`, `Done`.
- Counts per column.
- Creator and assignee visible on task cards.
- Clicking a task opens the assigned/origin chat thread.

Current Nekobot task states differ from the Slock board labels. The API should normalize them rather than forcing clients to guess:

| Board column | Nekobot states |
| --- | --- |
| TODO | `pending` |
| IN PROCESS | `claimed`, `running`, `requires_action` |
| IN REVIEW | new explicit state or `metadata.review_state = in_review` until state is added |
| Done | `completed`, `failed`, `canceled` |

`failed` and `canceled` may be shown as Done sub-states, but the API should retain the raw task state.

## Protocol Additions

### Stable Message Identity

Saved messages and thread replies require stable message IDs.

Current gap:

- `CollaborationMessage.message_id` exists.
- `SendMessage` creates a UUID for the response.
- Historical `ReadMessages` derives IDs as `sessionID:index`.
- WebUI session message response does not expose `message_id`.

Required target:

- All new messages persist a stable `message_id`.
- Historical messages may be backfilled as `sessionID:index` during migration.
- WebUI session responses and daemon `ReadMessages` return the same message ID for the same message.
- Message actions must never depend on a transient array index alone.

### Message Action RPCs

Add or expose server API equivalents for:

```text
ReplyInThread(message_id, target, content, request_id)
SaveMessage(message_id, target, request_id)
UnsaveMessage(message_id, target, request_id)
ListSavedMessages(target_filter?, limit, cursor)
```

`ReplyInThread` can be implemented as a thin wrapper over `SendMessage` once target derivation is stable.

`SaveMessage` and `UnsaveMessage` are mutations and must use `request_id` idempotency.

### Task Board RPC

Add a board-shaped read API instead of overloading every client with local grouping:

```text
ListTaskBoard(target_filter?, assignee_id?, created_by?, column?, limit, cursor)
```

Response shape:

```text
TaskBoardSnapshot {
  repeated TaskBoardColumn columns
  map<string,int64> counts
  EventCursor next_cursor
}

TaskBoardColumn {
  string column
  repeated Task tasks
  int64 total_count
}
```

Each task must include:

- `task_id`
- `summary`
- raw `state`
- normalized `board_column`
- `target`
- `thread_id`
- `created_by_user_id`
- `created_by_agent_id`
- `assignee_id`
- `runtime_id`
- `source`
- graph fields

Task click navigation uses `target` first, then `thread_id` fallback.

### Attachment Model

Existing protocol already has:

- `AttachmentRecord`
- `UploadAttachment`
- `GetAttachment`
- `SendMessage.attachment_ids`
- `CollaborationMessage.attachments`

Current gap:

- WebUI chat does not expose an end-to-end upload, bind, preview, and download path for collaboration messages.
- `SendMessage` accepts attachment IDs but does not reliably hydrate `CollaborationMessage.attachments` for replay.
- Event replay does not consistently include attachment upload or message attachment linkage.

Required server API:

```text
POST /api/daemon/attachments
GET  /api/daemon/attachments/{attachment_id}
POST /api/daemon/messages
```

The message send API must accept `attachment_ids` and return hydrated attachment metadata.

Attachments should be scoped by:

- owner/sender
- target/thread
- message_id once attached
- content type and size

## Event Log Requirements

The agent-awareness path is `ListEventsSince`, not ad hoc polling.

Add producers for:

```text
message.created
message.thread_replied
message.saved
message.unsaved
attachment.uploaded
attachment.attached
task.created
task.claimed
task.assigned
task.assignee_changed
task.status_changed
task.done
task.cancelled
task.board_moved
```

Event payloads must include enough indexed data for filtering:

- `target`
- `thread_id`
- `message_id`
- `task_id`
- `assignee_id`
- `created_by_user_id`
- `created_by_agent_id`
- `attachment_ids`
- `board_column`
- `old_state`
- `new_state`

Agents should be able to recover missed chat/task/attachment/save changes using only `ListEventsSince` plus targeted resource reads.

## WebUI API Boundaries

All WebUI endpoints for these surfaces must live under the authenticated `/api` group.

Do not add public routes for:

- sending direct messages
- saving messages
- reading saved items
- uploading or downloading private attachments
- listing task boards

Recommended HTTP wrappers:

```text
GET    /api/daemon/channels
GET    /api/daemon/channels/{channel}/threads
GET    /api/daemon/messages?target=...
POST   /api/daemon/messages
POST   /api/daemon/messages/{message_id}/reply
POST   /api/daemon/messages/{message_id}/save
DELETE /api/daemon/messages/{message_id}/save
GET    /api/daemon/saved-messages
POST   /api/daemon/attachments
GET    /api/daemon/attachments/{attachment_id}
GET    /api/daemon/task-board
```

HTTP wrappers must remain thin adapters over the daemon collaboration service. They must not introduce a second message, task, or attachment protocol.

## Implementation Slices

### Slice H: Task Board and Task-to-chat Navigation

Scope:

- Board read API.
- Channel/thread/DM filters.
- Column normalization and counts.
- Creator/assignee display fields.
- Task card navigation target.
- Task event producers.

Acceptance:

- `ListTaskBoard(target=#LightOsClub)` returns grouped columns and counts.
- Task card data includes `created_by_*`, `assignee_id`, `runtime_id`, `target`, and `thread_id`.
- Clicking a task can navigate to the associated chat/thread.
- `task.created`, `task.claimed`, and `task.status_changed` replay through `ListEventsSince`.

### Slice I: Saved Messages and Message Actions

Scope:

- Stable message IDs in WebUI and daemon reads.
- Per-message action row: `reply in thread`, `save message`.
- Saved message store and Saved page.
- Save/unsave idempotency and events.

Acceptance:

- Every chat message has visible `reply in thread` and `save message` actions.
- Saved page shows saved messages and links back to the original target.
- Saving the same message twice with the same request ID replays.
- `message.saved` and `message.unsaved` replay through `ListEventsSince`.

### Slice J: Attachment End-to-end

Scope:

- Authenticated WebUI upload/download routes.
- Chat composer attachment upload.
- Message send with `attachment_ids`.
- Chat rendering for image/file attachments.
- Agent `GetAttachment` path and event replay.

Acceptance:

- User can attach an image to a chat message and see a preview.
- Agent can read message attachment metadata and fetch content by `attachment_id`.
- Attachments are not exposed through public unauthenticated routes.
- `attachment.uploaded`, `attachment.attached`, and `message.created` replay through `ListEventsSince`.

## Current Risk Callouts

- Current #20 attempts must not leave direct-message routes public or under unproxied paths.
- Session-file backed messages make stable IDs and atomic event append harder. Slice I should either persist message IDs in session data or introduce a message resource store.
- Attachment storage currently uses a KV blob map. That is acceptable for a first slice but not enough for long-term retention, permissions, or large-file streaming.
- Task board `IN REVIEW` is not a native task state today. Add an explicit state or a documented metadata overlay before building UI expectations around it.
