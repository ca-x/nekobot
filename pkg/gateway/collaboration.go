package gateway

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	daemonv1 "nekobot/gen/go/nekobot/daemon/v1"
	"nekobot/pkg/agent"
	"nekobot/pkg/audit"
	"nekobot/pkg/bus"
	"nekobot/pkg/channels"
	"nekobot/pkg/cron"
	"nekobot/pkg/daemonhost"
	"nekobot/pkg/runtimeagents"
	"nekobot/pkg/session"
	"nekobot/pkg/skills"
	"nekobot/pkg/tasks"
	"nekobot/pkg/version"
)

const (
	daemonFollowStoreKey   = "daemon.collaboration.followed_threads.v1"
	daemonAgentEnvStoreKey = "daemon.collaboration.agent_env.v1"
	daemonReminderMetaKey  = "daemon.collaboration.reminder_meta.v1"
	daemonActivityStoreKey = "daemon.collaboration.activity.v1"
	daemonAttachmentKey    = "daemon.collaboration.attachments.v1"

	maxDaemonAttachmentBytes = 32 << 20
)

type storedAttachment struct {
	Record        *daemonv1.AttachmentRecord `json:"record"`
	ContentBase64 string                     `json:"content_base64"`
}

func (s *Server) ListChannels(ctx context.Context, req *daemonv1.ListChannelsRequest) (*daemonv1.ListChannelsResponse, error) {
	limit := normalizedCollaborationLimit(req.GetLimit(), 200)
	items := []*daemonv1.ChannelRecord{
		{
			Target:      "#websocket",
			ChannelId:   "websocket",
			DisplayName: "WebSocket",
			ChannelType: "websocket",
			Enabled:     true,
		},
	}
	for _, account := range s.listChannelAccountRecords(ctx) {
		if len(items) >= limit {
			break
		}
		items = append(items, account)
	}
	return &daemonv1.ListChannelsResponse{Channels: items}, nil
}

func (s *Server) ListThreads(ctx context.Context, req *daemonv1.ListThreadsRequest) (*daemonv1.ListThreadsResponse, error) {
	if s == nil || s.sessionMgr == nil {
		return nil, fmt.Errorf("session manager not available")
	}
	limit := normalizedCollaborationLimit(req.GetLimit(), 200)
	targetPrefix := strings.TrimSpace(req.GetTargetPrefix())
	ids, err := s.sessionMgr.List()
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	followed := s.followedThreadSet(ctx)
	records := make([]*daemonv1.ThreadRecord, 0, len(ids))
	for _, id := range ids {
		target := threadTarget(id)
		if targetPrefix != "" && !strings.HasPrefix(target, targetPrefix) {
			continue
		}
		record, err := s.threadRecord(ctx, id, followed)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, err
		}
		records = append(records, record)
	}
	sort.SliceStable(records, func(i, j int) bool {
		if records[i].UpdatedTimeUnix == records[j].UpdatedTimeUnix {
			return records[i].Target < records[j].Target
		}
		return records[i].UpdatedTimeUnix > records[j].UpdatedTimeUnix
	})
	if len(records) > limit {
		records = records[:limit]
	}
	return &daemonv1.ListThreadsResponse{Threads: records}, nil
}

func (s *Server) GetThread(ctx context.Context, req *daemonv1.GetThreadRequest) (*daemonv1.GetThreadResponse, error) {
	sessionID, err := collaborationSessionID(req.GetTarget())
	if err != nil {
		return nil, err
	}
	record, err := s.threadRecord(ctx, sessionID, s.followedThreadSet(ctx))
	if err != nil {
		return nil, err
	}
	return &daemonv1.GetThreadResponse{Thread: record}, nil
}

func (s *Server) ReadMessages(ctx context.Context, req *daemonv1.ReadMessagesRequest) (*daemonv1.ReadMessagesResponse, error) {
	if s == nil || s.sessionMgr == nil {
		return nil, fmt.Errorf("session manager not available")
	}
	sessionID, err := collaborationSessionID(req.GetTarget())
	if err != nil {
		return nil, err
	}
	sess, err := s.sessionMgr.GetExisting(sessionID)
	if err != nil {
		return nil, fmt.Errorf("load thread %s: %w", sessionID, err)
	}
	messages := sess.GetMessages()
	limit := normalizedCollaborationLimit(req.GetLimit(), 200)
	if len(messages) > limit {
		messages = messages[len(messages)-limit:]
	}
	out := make([]*daemonv1.CollaborationMessage, 0, len(messages))
	target := threadTarget(sessionID)
	for i, msg := range messages {
		out = append(out, sessionMessageToProto(target, sessionID, i, msg, sess.GetUpdatedAt()))
	}
	return &daemonv1.ReadMessagesResponse{Messages: out}, nil
}

func (s *Server) SendMessage(ctx context.Context, req *daemonv1.SendMessageRequest) (*daemonv1.SendMessageResponse, error) {
	if s == nil || s.sessionMgr == nil {
		return nil, fmt.Errorf("session manager not available")
	}
	target, err := daemonhost.ValidateCollaborationTarget(req.GetTarget())
	if err != nil {
		return nil, err
	}
	content := strings.TrimSpace(req.GetContent())
	if content == "" {
		return nil, fmt.Errorf("content is required")
	}
	sessionID, err := collaborationSessionID(target)
	if err != nil {
		return nil, err
	}
	role := normalizeMessageRole(req.GetRole())
	sess, err := s.sessionMgr.GetWithSource(sessionID, session.SourceGateway)
	if err != nil {
		return nil, fmt.Errorf("load thread %s: %w", sessionID, err)
	}
	msg := agent.Message{Role: role, Content: content}
	sess.AddMessage(msg)
	if err := s.sessionMgr.Save(sess); err != nil {
		return nil, fmt.Errorf("save thread %s: %w", sessionID, err)
	}
	protoMsg := &daemonv1.CollaborationMessage{
		MessageId:         uuid.NewString(),
		Target:            target,
		ThreadId:          sessionID,
		Role:              role,
		Content:           content,
		SenderAgentId:     strings.TrimSpace(req.GetSenderAgentId()),
		SenderDisplayName: strings.TrimSpace(req.GetSenderDisplayName()),
		ReplyToMessageId:  strings.TrimSpace(req.GetReplyToMessageId()),
		CreatedTimeUnix:   time.Now().Unix(),
	}
	if req.GetEmitOutbound() && s.bus != nil {
		_ = s.bus.SendOutbound(&bus.Message{
			ID:        protoMsg.MessageId,
			ChannelID: targetChannelID(target),
			SessionID: sessionID,
			UserID:    protoMsg.SenderAgentId,
			Username:  protoMsg.SenderDisplayName,
			Type:      bus.MessageTypeText,
			Content:   content,
			Timestamp: time.Now(),
			ReplyTo:   protoMsg.ReplyToMessageId,
		})
	}
	return &daemonv1.SendMessageResponse{Accepted: true, Message: protoMsg}, nil
}

func (s *Server) FollowThread(ctx context.Context, req *daemonv1.FollowThreadRequest) (*daemonv1.FollowThreadResponse, error) {
	target, err := daemonhost.ValidateCollaborationTarget(req.GetTarget())
	if err != nil {
		return nil, err
	}
	if _, err := collaborationSessionID(target); err != nil {
		return nil, err
	}
	return &daemonv1.FollowThreadResponse{Accepted: true}, s.updateFollowedThread(ctx, target, true)
}

func (s *Server) UnfollowThread(ctx context.Context, req *daemonv1.UnfollowThreadRequest) (*daemonv1.UnfollowThreadResponse, error) {
	target, err := daemonhost.ValidateCollaborationTarget(req.GetTarget())
	if err != nil {
		return nil, err
	}
	return &daemonv1.UnfollowThreadResponse{Accepted: true}, s.updateFollowedThread(ctx, target, false)
}

func (s *Server) CreateCollaborationTask(ctx context.Context, req *daemonv1.CreateCollaborationTaskRequest) (*daemonv1.CreateCollaborationTaskResponse, error) {
	if s == nil || s.agent == nil || s.agent.TaskService() == nil {
		return nil, fmt.Errorf("task service unavailable")
	}
	target, err := daemonhost.ValidateCollaborationTarget(req.GetTarget())
	if err != nil {
		return nil, err
	}
	sessionID, err := collaborationSessionID(target)
	if err != nil {
		return nil, err
	}
	summary := strings.TrimSpace(req.GetSummary())
	if summary == "" {
		return nil, fmt.Errorf("summary is required")
	}
	task, err := s.agent.TaskService().Enqueue(tasks.Task{
		ID:        "collab-task-" + uuid.NewString(),
		Type:      tasks.TypeRemoteAgent,
		Summary:   summary,
		SessionID: sessionID,
		RuntimeID: strings.TrimSpace(req.GetAgentId()),
		Metadata: map[string]any{
			"target":             target,
			"created_by_user_id": strings.TrimSpace(req.GetCreatedByUserId()),
			"delivery":           "daemon-collaboration",
		},
	})
	if err != nil {
		return nil, err
	}
	return &daemonv1.CreateCollaborationTaskResponse{Task: daemonhost.CollaborationTaskToProto(task)}, nil
}

func (s *Server) ListCollaborationTasks(ctx context.Context, req *daemonv1.ListCollaborationTasksRequest) (*daemonv1.ListCollaborationTasksResponse, error) {
	if s == nil || s.agent == nil || s.agent.TaskService() == nil {
		return nil, fmt.Errorf("task service unavailable")
	}
	target := strings.TrimSpace(req.GetTarget())
	runtimeID := strings.TrimSpace(req.GetAgentId())
	limit := normalizedCollaborationLimit(req.GetLimit(), 200)
	out := make([]*daemonv1.Task, 0)
	for _, item := range s.agent.TaskService().List() {
		if target != "" && metadataString(item.Metadata, "target") != target && threadTarget(item.SessionID) != target {
			continue
		}
		if runtimeID != "" && strings.TrimSpace(item.RuntimeID) != runtimeID {
			continue
		}
		out = append(out, daemonhost.CollaborationTaskToProto(item))
		if len(out) >= limit {
			break
		}
	}
	return &daemonv1.ListCollaborationTasksResponse{Tasks: out}, nil
}

func (s *Server) ClaimCollaborationTask(ctx context.Context, req *daemonv1.ClaimCollaborationTaskRequest) (*daemonv1.ClaimCollaborationTaskResponse, error) {
	if s == nil || s.agent == nil || s.agent.TaskService() == nil {
		return nil, fmt.Errorf("task service unavailable")
	}
	task, err := s.agent.TaskService().Claim(req.GetTaskId(), req.GetAgentId())
	if err != nil {
		return nil, err
	}
	return &daemonv1.ClaimCollaborationTaskResponse{Task: daemonhost.CollaborationTaskToProto(task)}, nil
}

func (s *Server) GetServerInfo(ctx context.Context, req *daemonv1.GetServerInfoRequest) (*daemonv1.GetServerInfoResponse, error) {
	channels, err := s.ListChannels(ctx, &daemonv1.ListChannelsRequest{Limit: 200})
	if err != nil {
		return nil, err
	}
	agents, err := s.ListAgentProfiles(ctx, &daemonv1.ListAgentProfilesRequest{Limit: 200})
	if err != nil {
		return nil, err
	}
	return &daemonv1.GetServerInfoResponse{
		ServerName: "nekobot",
		Version:    version.GetVersion(),
		Channels:   channels.Channels,
		Agents:     agents.Profiles,
	}, nil
}

func (s *Server) GetAgentProfile(ctx context.Context, req *daemonv1.GetAgentProfileRequest) (*daemonv1.GetAgentProfileResponse, error) {
	runtimeID := strings.TrimSpace(req.GetAgentId())
	profiles, err := s.agentProfiles(ctx, 1000)
	if err != nil {
		return nil, err
	}
	for _, profile := range profiles {
		if runtimeID == "" || profile.AgentId == runtimeID || profile.Name == runtimeID {
			return &daemonv1.GetAgentProfileResponse{Profile: profile}, nil
		}
	}
	return nil, fmt.Errorf("agent profile not found: %s", runtimeID)
}

func (s *Server) SetAgentEnv(ctx context.Context, req *daemonv1.SetAgentEnvRequest) (*daemonv1.SetAgentEnvResponse, error) {
	runtimeID := strings.TrimSpace(req.GetAgentId())
	if runtimeID == "" {
		return nil, fmt.Errorf("runtime_id is required")
	}
	env := normalizeEnvVars(req.GetEnv())
	if err := s.storeAgentEnv(ctx, runtimeID, env); err != nil {
		return nil, err
	}
	profile, err := s.GetAgentProfile(ctx, &daemonv1.GetAgentProfileRequest{AgentId: runtimeID})
	if err != nil {
		return nil, err
	}
	return &daemonv1.SetAgentEnvResponse{Profile: profile.Profile}, nil
}

func (s *Server) ListAgentProfiles(ctx context.Context, req *daemonv1.ListAgentProfilesRequest) (*daemonv1.ListAgentProfilesResponse, error) {
	profiles, err := s.agentProfiles(ctx, normalizedCollaborationLimit(req.GetLimit(), 200))
	if err != nil {
		return nil, err
	}
	return &daemonv1.ListAgentProfilesResponse{Profiles: profiles}, nil
}

func (s *Server) ListAgentDMs(ctx context.Context, req *daemonv1.ListAgentDMsRequest) (*daemonv1.ListAgentDMsResponse, error) {
	profiles, err := s.agentProfiles(ctx, normalizedCollaborationLimit(req.GetLimit(), 200))
	if err != nil {
		return nil, err
	}
	self := strings.TrimSpace(req.GetAgentId())
	dms := make([]*daemonv1.ChannelRecord, 0, len(profiles))
	for _, profile := range profiles {
		if profile == nil || profile.AgentId == "" || profile.AgentId == self {
			continue
		}
		dms = append(dms, &daemonv1.ChannelRecord{
			Target:      agentDMTarget(profile.AgentId),
			ChannelId:   "dm:" + profile.AgentId,
			DisplayName: profile.DisplayName,
			ChannelType: "dm",
			Enabled:     profile.Enabled,
		})
	}
	return &daemonv1.ListAgentDMsResponse{Dms: dms}, nil
}

func (s *Server) ScheduleReminder(ctx context.Context, req *daemonv1.ScheduleReminderRequest) (*daemonv1.ScheduleReminderResponse, error) {
	if s == nil || s.cronMgr == nil {
		return nil, fmt.Errorf("reminder scheduler unavailable")
	}
	target, err := daemonhost.ValidateCollaborationTarget(req.GetTarget())
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.GetName())
	if name == "" {
		name = "daemon reminder"
	}
	prompt := strings.TrimSpace(req.GetPrompt())
	if prompt == "" {
		return nil, fmt.Errorf("prompt is required")
	}
	route := cron.RouteOptions{Skills: req.GetSkills()}
	var job *cron.Job
	switch cron.ScheduleKind(strings.ToLower(strings.TrimSpace(req.GetScheduleKind()))) {
	case "", cron.ScheduleCron:
		schedule := strings.TrimSpace(req.GetSchedule())
		if schedule == "" {
			return nil, fmt.Errorf("schedule is required")
		}
		job, err = s.cronMgr.AddCronJobWithRoute(name, schedule, prompt, route)
	case cron.ScheduleAt:
		at, parseErr := time.Parse(time.RFC3339, strings.TrimSpace(req.GetSchedule()))
		if parseErr != nil {
			return nil, fmt.Errorf("invalid at schedule, use RFC3339: %w", parseErr)
		}
		job, err = s.cronMgr.AddAtJobWithRoute(name, at, prompt, true, route)
	case cron.ScheduleEvery:
		schedule := strings.TrimSpace(req.GetSchedule())
		if schedule == "" {
			return nil, fmt.Errorf("schedule is required")
		}
		job, err = s.cronMgr.AddEveryJobWithRoute(name, schedule, prompt, route)
	default:
		return nil, fmt.Errorf("invalid schedule_kind")
	}
	if err != nil {
		return nil, err
	}
	if err := s.storeReminderTarget(ctx, job.ID, target); err != nil {
		return nil, err
	}
	return &daemonv1.ScheduleReminderResponse{Reminder: reminderToProto(job, target)}, nil
}

func (s *Server) ListReminders(ctx context.Context, req *daemonv1.ListRemindersRequest) (*daemonv1.ListRemindersResponse, error) {
	if s == nil || s.cronMgr == nil {
		return nil, fmt.Errorf("reminder scheduler unavailable")
	}
	target := strings.TrimSpace(req.GetTarget())
	limit := normalizedCollaborationLimit(req.GetLimit(), 200)
	targets := s.reminderTargets(ctx)
	jobs := s.cronMgr.ListJobs()
	sort.SliceStable(jobs, func(i, j int) bool {
		if jobs[i] == nil || jobs[j] == nil {
			return jobs[j] == nil
		}
		return jobs[i].NextRun.Before(jobs[j].NextRun)
	})
	out := make([]*daemonv1.ReminderRecord, 0, len(jobs))
	for _, job := range jobs {
		if job == nil {
			continue
		}
		jobTarget := targets[job.ID]
		if target != "" && jobTarget != target {
			continue
		}
		out = append(out, reminderToProto(job, jobTarget))
		if len(out) >= limit {
			break
		}
	}
	return &daemonv1.ListRemindersResponse{Reminders: out}, nil
}

func (s *Server) CancelReminder(ctx context.Context, req *daemonv1.CancelReminderRequest) (*daemonv1.CancelReminderResponse, error) {
	if s == nil || s.cronMgr == nil {
		return nil, fmt.Errorf("reminder scheduler unavailable")
	}
	reminderID := strings.TrimSpace(req.GetReminderId())
	if reminderID == "" {
		return nil, fmt.Errorf("reminder_id is required")
	}
	if err := s.cronMgr.RemoveJob(reminderID); err != nil {
		return nil, err
	}
	_ = s.deleteReminderTarget(ctx, reminderID)
	return &daemonv1.CancelReminderResponse{Accepted: true}, nil
}

func (s *Server) LogActivity(ctx context.Context, req *daemonv1.LogActivityRequest) (*daemonv1.LogActivityResponse, error) {
	target, err := daemonhost.ValidateCollaborationTarget(req.GetTarget())
	if err != nil {
		return nil, err
	}
	activity := &daemonv1.ActivityRecord{
		ActivityId:      "activity-" + uuid.NewString(),
		Target:          target,
		AgentId:         strings.TrimSpace(req.GetAgentId()),
		Kind:            strings.TrimSpace(req.GetKind()),
		Summary:         strings.TrimSpace(req.GetSummary()),
		Detail:          strings.TrimSpace(req.GetDetail()),
		CreatedTimeUnix: time.Now().Unix(),
		RunId:           strings.TrimSpace(req.GetRunId()),
		StepId:          strings.TrimSpace(req.GetStepId()),
	}
	if activity.Kind == "" {
		activity.Kind = "event"
	}
	if activity.Summary == "" {
		return nil, fmt.Errorf("summary is required")
	}
	if err := s.appendActivity(ctx, activity); err != nil {
		return nil, err
	}
	if s.auditLogger != nil {
		s.auditLogger.Log(&audit.Entry{
			Timestamp:     time.Unix(activity.CreatedTimeUnix, 0),
			ToolName:      "daemon.activity." + activity.Kind,
			Arguments:     map[string]interface{}{"target": target, "runtime_id": activity.AgentId},
			Success:       true,
			ResultPreview: activity.Summary,
			SessionID:     target,
		})
	}
	return &daemonv1.LogActivityResponse{Activity: activity}, nil
}

func (s *Server) ListActivity(ctx context.Context, req *daemonv1.ListActivityRequest) (*daemonv1.ListActivityResponse, error) {
	target := strings.TrimSpace(req.GetTarget())
	runtimeID := strings.TrimSpace(req.GetAgentId())
	limit := normalizedCollaborationLimit(req.GetLimit(), 200)
	activities := s.activityLog(ctx)
	out := make([]*daemonv1.ActivityRecord, 0, len(activities))
	for i := len(activities) - 1; i >= 0 && len(out) < limit; i-- {
		item := activities[i]
		if item == nil {
			continue
		}
		if target != "" && item.Target != target {
			continue
		}
		if runtimeID != "" && item.AgentId != runtimeID {
			continue
		}
		out = append(out, item)
	}
	return &daemonv1.ListActivityResponse{Activities: out}, nil
}

func (s *Server) UploadAttachment(ctx context.Context, req *daemonv1.UploadAttachmentRequest) (*daemonv1.UploadAttachmentResponse, error) {
	if s == nil || s.kvStore == nil {
		return nil, fmt.Errorf("attachment store unavailable")
	}
	target, err := daemonhost.ValidateCollaborationTarget(req.GetTarget())
	if err != nil {
		return nil, err
	}
	filename := strings.TrimSpace(req.GetFilename())
	if filename == "" {
		return nil, fmt.Errorf("filename is required")
	}
	content := req.GetContent()
	if len(content) == 0 {
		return nil, fmt.Errorf("content is required")
	}
	if len(content) > maxDaemonAttachmentBytes {
		return nil, fmt.Errorf("attachment exceeds %d bytes", maxDaemonAttachmentBytes)
	}
	record := &daemonv1.AttachmentRecord{
		AttachmentId:    "attachment-" + uuid.NewString(),
		Target:          target,
		OwnerId:         strings.TrimSpace(req.GetOwnerId()),
		Filename:        filename,
		MimeType:        strings.TrimSpace(req.GetMimeType()),
		SizeBytes:       int64(len(content)),
		StorageRef:      "kv:" + daemonAttachmentKey,
		CreatedTimeUnix: time.Now().Unix(),
	}
	if err := s.storeAttachment(ctx, record, content); err != nil {
		return nil, err
	}
	return &daemonv1.UploadAttachmentResponse{Attachment: record}, nil
}

func (s *Server) GetAttachment(ctx context.Context, req *daemonv1.GetAttachmentRequest) (*daemonv1.GetAttachmentResponse, error) {
	attachmentID := strings.TrimSpace(req.GetAttachmentId())
	if attachmentID == "" {
		return nil, fmt.Errorf("attachment_id is required")
	}
	record, content, err := s.loadAttachment(ctx, attachmentID)
	if err != nil {
		return nil, err
	}
	return &daemonv1.GetAttachmentResponse{Attachment: record, Content: content}, nil
}

func (s *Server) ListEventsSince(ctx context.Context, req *daemonv1.ListEventsSinceRequest) (*daemonv1.ListEventsSinceResponse, error) {
	limit := normalizedCollaborationLimit(req.GetLimit(), 200)
	cursor := req.GetCursor()
	afterActivityID := ""
	if cursor != nil {
		afterActivityID = strings.TrimSpace(cursor.GetLastActivityId())
	}
	activities := s.activityLog(ctx)
	events := make([]*daemonv1.CollaborationEvent, 0, len(activities))
	seenCursor := afterActivityID == ""
	for _, activity := range activities {
		if activity == nil {
			continue
		}
		if !seenCursor {
			if activity.GetActivityId() == afterActivityID {
				seenCursor = true
			}
			continue
		}
		if len(events) >= limit {
			break
		}
		events = append(events, activityToEvent(activity))
	}
	if afterActivityID != "" && !seenCursor {
		return nil, fmt.Errorf("event cursor expired or unknown: %s", afterActivityID)
	}
	next := &daemonv1.EventCursor{}
	if cursor != nil {
		next.Target = cursor.GetTarget()
	}
	if len(events) > 0 {
		last := events[len(events)-1]
		next.LastEventId = last.GetEventId()
		next.LastActivityId = last.GetActivityId()
		next.Cursor = last.GetEventId()
	} else if cursor != nil {
		next.Cursor = cursor.GetCursor()
		next.LastEventId = cursor.GetLastEventId()
		next.LastActivityId = cursor.GetLastActivityId()
	}
	return &daemonv1.ListEventsSinceResponse{Events: events, NextCursor: next}, nil
}

func (s *Server) listChannelAccountRecords(ctx context.Context) []*daemonv1.ChannelRecord {
	if s == nil || s.channels == nil {
		return nil
	}
	registered := s.channels.ListChannels()
	items := make([]*daemonv1.ChannelRecord, 0, len(registered))
	for _, ch := range registered {
		if ch == nil {
			continue
		}
		channelType := ch.ID()
		if typed, ok := ch.(channels.TypedChannel); ok {
			channelType = typed.ChannelType()
		}
		items = append(items, &daemonv1.ChannelRecord{
			Target:      "#" + strings.TrimPrefix(ch.ID(), "#"),
			ChannelId:   ch.ID(),
			DisplayName: ch.Name(),
			ChannelType: channelType,
			Enabled:     ch.IsEnabled(),
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Target < items[j].Target
	})
	return items
}

func (s *Server) agentProfiles(ctx context.Context, limit int) ([]*daemonv1.AgentProfile, error) {
	runtimes, err := s.runtimeDefinitions(ctx)
	if err != nil {
		return nil, err
	}
	envByRuntime := s.agentEnv(ctx)
	allSkills := s.skillRecords(nil)
	out := make([]*daemonv1.AgentProfile, 0, len(runtimes))
	for _, item := range runtimes {
		skillFilter := map[string]struct{}{}
		for _, id := range item.Skills {
			skillFilter[id] = struct{}{}
		}
		out = append(out, &daemonv1.AgentProfile{
			AgentId:     item.ID,
			Name:        item.Name,
			DisplayName: item.DisplayName,
			Description: item.Description,
			Enabled:     item.Enabled,
			Provider:    item.Provider,
			Model:       item.Model,
			Env:         profileEnvVars(envByRuntime[item.ID]),
			Skills:      s.skillRecords(skillFilter),
			DmTargets:   []string{agentDMTarget(item.ID)},
		})
		if len(out) >= limit {
			return out, nil
		}
	}
	if len(out) == 0 {
		out = append(out, &daemonv1.AgentProfile{
			AgentId:     "default",
			Name:        "default",
			DisplayName: "Default Agent",
			Enabled:     true,
			Env:         profileEnvVars(envByRuntime["default"]),
			Skills:      allSkills,
			DmTargets:   []string{agentDMTarget("default")},
		})
	}
	return out, nil
}

func (s *Server) runtimeDefinitions(ctx context.Context) ([]runtimeagents.AgentRuntime, error) {
	if s == nil || s.runtimeMgr == nil {
		return nil, nil
	}
	items, err := s.runtimeMgr.List(ctx)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Name == items[j].Name {
			return items[i].ID < items[j].ID
		}
		return items[i].Name < items[j].Name
	})
	return items, nil
}

func (s *Server) skillRecords(filter map[string]struct{}) []*daemonv1.SkillRecord {
	if s == nil || s.skillsMgr == nil {
		return nil
	}
	eligible := map[string]struct{}{}
	for _, skill := range s.skillsMgr.ListEligibleEnabled() {
		if skill != nil {
			eligible[skill.ID] = struct{}{}
		}
	}
	all := s.skillsMgr.List()
	records := make([]*daemonv1.SkillRecord, 0, len(all))
	for _, skill := range all {
		if skill == nil {
			continue
		}
		if len(filter) > 0 {
			if _, ok := filter[skill.ID]; !ok {
				continue
			}
		}
		_, isEligible := eligible[skill.ID]
		records = append(records, skillToProto(skill, isEligible))
	}
	sort.SliceStable(records, func(i, j int) bool {
		return records[i].Id < records[j].Id
	})
	return records
}

func skillToProto(skill *skills.Skill, eligible bool) *daemonv1.SkillRecord {
	if skill == nil {
		return nil
	}
	return &daemonv1.SkillRecord{
		Id:          skill.ID,
		Name:        skill.Name,
		Description: skill.Description,
		Version:     skill.Version,
		Enabled:     skill.Enabled,
		Always:      skill.Always,
		Eligible:    eligible,
		FilePath:    skill.FilePath,
	}
}

func agentDMTarget(runtimeID string) string {
	runtimeID = strings.TrimSpace(runtimeID)
	if runtimeID == "" {
		runtimeID = "default"
	}
	return "dm:@" + runtimeID
}

func normalizeEnvVars(items []*daemonv1.EnvVar) []*daemonv1.EnvVar {
	out := make([]*daemonv1.EnvVar, 0, len(items))
	seen := map[string]int{}
	for _, item := range items {
		if item == nil {
			continue
		}
		name := strings.TrimSpace(item.Name)
		if name == "" || strings.ContainsAny(name, "\x00\r\n=") {
			continue
		}
		env := &daemonv1.EnvVar{Name: name, Value: item.Value, Secret: item.Secret}
		if idx, ok := seen[name]; ok {
			out[idx] = env
			continue
		}
		seen[name] = len(out)
		out = append(out, env)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

func cloneEnvVars(items []*daemonv1.EnvVar) []*daemonv1.EnvVar {
	out := make([]*daemonv1.EnvVar, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		out = append(out, &daemonv1.EnvVar{Name: item.Name, Value: item.Value, Secret: item.Secret})
	}
	return out
}

func profileEnvVars(items []*daemonv1.EnvVar) []*daemonv1.EnvVar {
	out := make([]*daemonv1.EnvVar, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		value := item.Value
		if item.Secret {
			value = "********"
		}
		out = append(out, &daemonv1.EnvVar{Name: item.Name, Value: value, Secret: item.Secret})
	}
	return out
}

func (s *Server) agentEnv(ctx context.Context) map[string][]*daemonv1.EnvVar {
	result := map[string][]*daemonv1.EnvVar{}
	if s == nil || s.kvStore == nil {
		return result
	}
	value, ok, err := s.kvStore.Get(ctx, daemonAgentEnvStoreKey)
	if err != nil || !ok {
		return result
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return result
	}
	_ = json.Unmarshal(payload, &result)
	if result == nil {
		return map[string][]*daemonv1.EnvVar{}
	}
	return result
}

func (s *Server) storeAgentEnv(ctx context.Context, runtimeID string, env []*daemonv1.EnvVar) error {
	if s == nil || s.kvStore == nil {
		return nil
	}
	return s.kvStore.UpdateFunc(ctx, daemonAgentEnvStoreKey, func(current interface{}) interface{} {
		result := map[string][]*daemonv1.EnvVar{}
		payload, err := json.Marshal(current)
		if err == nil {
			_ = json.Unmarshal(payload, &result)
		}
		if result == nil {
			result = map[string][]*daemonv1.EnvVar{}
		}
		result[runtimeID] = cloneEnvVars(env)
		return result
	})
}

func reminderToProto(job *cron.Job, target string) *daemonv1.ReminderRecord {
	if job == nil {
		return nil
	}
	return &daemonv1.ReminderRecord{
		ReminderId:   job.ID,
		Target:       target,
		ScheduleKind: string(job.ScheduleKind),
		Schedule:     reminderSchedule(job),
		Prompt:       job.Prompt,
		Enabled:      job.Enabled,
		NextRunUnix:  unixOrZero(job.NextRun),
		LastRunUnix:  unixOrZero(job.LastRun),
		RunCount:     uint32(job.RunCount),
		LastError:    job.LastError,
	}
}

func reminderSchedule(job *cron.Job) string {
	switch job.ScheduleKind {
	case cron.ScheduleAt:
		if job.AtTime == nil {
			return ""
		}
		return job.AtTime.Format(time.RFC3339)
	case cron.ScheduleEvery:
		return job.EveryDuration
	default:
		return job.Schedule
	}
}

func unixOrZero(ts time.Time) int64 {
	if ts.IsZero() {
		return 0
	}
	return ts.Unix()
}

func (s *Server) reminderTargets(ctx context.Context) map[string]string {
	result := map[string]string{}
	if s == nil || s.kvStore == nil {
		return result
	}
	value, ok, err := s.kvStore.Get(ctx, daemonReminderMetaKey)
	if err != nil || !ok {
		return result
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return result
	}
	_ = json.Unmarshal(payload, &result)
	if result == nil {
		return map[string]string{}
	}
	return result
}

func (s *Server) storeReminderTarget(ctx context.Context, reminderID, target string) error {
	if s == nil || s.kvStore == nil {
		return nil
	}
	return s.kvStore.UpdateFunc(ctx, daemonReminderMetaKey, func(current interface{}) interface{} {
		result := map[string]string{}
		payload, err := json.Marshal(current)
		if err == nil {
			_ = json.Unmarshal(payload, &result)
		}
		if result == nil {
			result = map[string]string{}
		}
		result[reminderID] = target
		return result
	})
}

func (s *Server) deleteReminderTarget(ctx context.Context, reminderID string) error {
	if s == nil || s.kvStore == nil {
		return nil
	}
	return s.kvStore.UpdateFunc(ctx, daemonReminderMetaKey, func(current interface{}) interface{} {
		result := map[string]string{}
		payload, err := json.Marshal(current)
		if err == nil {
			_ = json.Unmarshal(payload, &result)
		}
		if result == nil {
			result = map[string]string{}
		}
		delete(result, reminderID)
		return result
	})
}

func (s *Server) appendActivity(ctx context.Context, activity *daemonv1.ActivityRecord) error {
	if s == nil || s.kvStore == nil || activity == nil {
		return nil
	}
	return s.kvStore.UpdateFunc(ctx, daemonActivityStoreKey, func(current interface{}) interface{} {
		activities := []*daemonv1.ActivityRecord{}
		payload, err := json.Marshal(current)
		if err == nil {
			_ = json.Unmarshal(payload, &activities)
		}
		if activities == nil {
			activities = []*daemonv1.ActivityRecord{}
		}
		activities = append(activities, activity)
		if len(activities) > 1000 {
			activities = activities[len(activities)-1000:]
		}
		return activities
	})
}

func (s *Server) activityLog(ctx context.Context) []*daemonv1.ActivityRecord {
	if s == nil || s.kvStore == nil {
		return nil
	}
	value, ok, err := s.kvStore.Get(ctx, daemonActivityStoreKey)
	if err != nil || !ok {
		return nil
	}
	activities := []*daemonv1.ActivityRecord{}
	payload, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	_ = json.Unmarshal(payload, &activities)
	if activities == nil {
		return nil
	}
	return activities
}

func activityToEvent(activity *daemonv1.ActivityRecord) *daemonv1.CollaborationEvent {
	if activity == nil {
		return nil
	}
	eventID := "event-" + activity.GetActivityId()
	return &daemonv1.CollaborationEvent{
		EventId:         eventID,
		Target:          activity.GetTarget(),
		Kind:            "activity." + activity.GetKind(),
		ActivityId:      activity.GetActivityId(),
		RunId:           activity.GetRunId(),
		CreatedTimeUnix: activity.GetCreatedTimeUnix(),
	}
}

func (s *Server) storeAttachment(ctx context.Context, record *daemonv1.AttachmentRecord, content []byte) error {
	if s == nil || s.kvStore == nil || record == nil {
		return fmt.Errorf("attachment store unavailable")
	}
	return s.kvStore.UpdateFunc(ctx, daemonAttachmentKey, func(current interface{}) interface{} {
		attachments := map[string]storedAttachment{}
		payload, err := json.Marshal(current)
		if err == nil {
			_ = json.Unmarshal(payload, &attachments)
		}
		if attachments == nil {
			attachments = map[string]storedAttachment{}
		}
		attachments[record.GetAttachmentId()] = storedAttachment{
			Record:        record,
			ContentBase64: base64.StdEncoding.EncodeToString(content),
		}
		return attachments
	})
}

func (s *Server) loadAttachment(ctx context.Context, attachmentID string) (*daemonv1.AttachmentRecord, []byte, error) {
	if s == nil || s.kvStore == nil {
		return nil, nil, fmt.Errorf("attachment store unavailable")
	}
	value, ok, err := s.kvStore.Get(ctx, daemonAttachmentKey)
	if err != nil {
		return nil, nil, err
	}
	if !ok {
		return nil, nil, fmt.Errorf("attachment not found: %s", attachmentID)
	}
	attachments := map[string]storedAttachment{}
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, nil, err
	}
	if err := json.Unmarshal(payload, &attachments); err != nil {
		return nil, nil, err
	}
	item, ok := attachments[attachmentID]
	if !ok || item.Record == nil {
		return nil, nil, fmt.Errorf("attachment not found: %s", attachmentID)
	}
	content, err := base64.StdEncoding.DecodeString(item.ContentBase64)
	if err != nil {
		return nil, nil, fmt.Errorf("decode attachment %s: %w", attachmentID, err)
	}
	return item.Record, content, nil
}

func (s *Server) threadRecord(ctx context.Context, sessionID string, followed map[string]struct{}) (*daemonv1.ThreadRecord, error) {
	sess, err := s.sessionMgr.GetExisting(sessionID)
	if err != nil {
		return nil, err
	}
	messages := sess.GetMessages()
	target := threadTarget(sessionID)
	_, isFollowed := followed[target]
	return &daemonv1.ThreadRecord{
		Target:          target,
		ThreadId:        sessionID,
		Summary:         sess.GetSummary(),
		RuntimeId:       "",
		MessageCount:    uint32(len(messages)),
		CreatedTimeUnix: sess.GetCreatedAt().Unix(),
		UpdatedTimeUnix: sess.GetUpdatedAt().Unix(),
		Followed:        isFollowed,
	}, nil
}

func (s *Server) followedThreadSet(ctx context.Context) map[string]struct{} {
	result := map[string]struct{}{}
	if s == nil || s.kvStore == nil {
		return result
	}
	value, ok, err := s.kvStore.Get(ctx, daemonFollowStoreKey)
	if err != nil || !ok {
		return result
	}
	raw, ok := value.(map[string]interface{})
	if !ok {
		return result
	}
	for target, enabled := range raw {
		if b, _ := enabled.(bool); b {
			result[strings.TrimSpace(target)] = struct{}{}
		}
	}
	return result
}

func (s *Server) updateFollowedThread(ctx context.Context, target string, followed bool) error {
	if s == nil || s.kvStore == nil {
		return nil
	}
	return s.kvStore.UpdateFunc(ctx, daemonFollowStoreKey, func(current interface{}) interface{} {
		raw, _ := current.(map[string]interface{})
		next := map[string]interface{}{}
		for key, value := range raw {
			next[key] = value
		}
		if followed {
			next[target] = true
		} else {
			delete(next, target)
		}
		return next
	})
}

func collaborationSessionID(target string) (string, error) {
	target, err := daemonhost.ValidateCollaborationTarget(target)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(target, "dm:@") {
		return "dm_" + strings.TrimPrefix(target, "dm:"), nil
	}
	if strings.Contains(target, ":") {
		_, threadID, ok := strings.Cut(target, ":")
		if !ok || strings.TrimSpace(threadID) == "" {
			return "", fmt.Errorf("thread target must be #channel:thread")
		}
		return strings.TrimSpace(threadID), nil
	}
	if strings.HasPrefix(target, "#") {
		return strings.TrimPrefix(target, "#"), nil
	}
	return target, nil
}

func threadTarget(sessionID string) string {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return ""
	}
	if strings.HasPrefix(sessionID, "dm_@") {
		return "dm:" + strings.TrimPrefix(sessionID, "dm_")
	}
	return "#websocket:" + sessionID
}

func targetChannelID(target string) string {
	if strings.HasPrefix(strings.TrimSpace(target), "dm:@") {
		return "dm"
	}
	target = strings.TrimSpace(strings.TrimPrefix(target, "#"))
	if channel, _, ok := strings.Cut(target, ":"); ok {
		return channel
	}
	return target
}

func normalizedCollaborationLimit(limit uint32, fallback int) int {
	if limit == 0 {
		return fallback
	}
	if limit > 1000 {
		return 1000
	}
	return int(limit)
}

func normalizeMessageRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "system", "user", "assistant", "tool":
		return strings.ToLower(strings.TrimSpace(role))
	default:
		return "assistant"
	}
}

func sessionMessageToProto(target, sessionID string, index int, msg session.Message, updatedAt time.Time) *daemonv1.CollaborationMessage {
	return &daemonv1.CollaborationMessage{
		MessageId:       fmt.Sprintf("%s:%d", sessionID, index),
		Target:          target,
		ThreadId:        sessionID,
		Role:            msg.Role,
		Content:         msg.Content,
		CreatedTimeUnix: updatedAt.Unix(),
	}
}

func metadataString(values map[string]any, key string) string {
	if len(values) == 0 {
		return ""
	}
	value, _ := values[key].(string)
	return strings.TrimSpace(value)
}
