package externalagent

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"nekobot/pkg/approval"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/permissionrules"
	"nekobot/pkg/storage/ent"
	"nekobot/pkg/tasks"
	"nekobot/pkg/toolsessions"
)

// ResolveOrchestrator centralizes external-agent resolve policy preview and approval handling.
type ResolveOrchestrator struct {
	cfg       *config.Config
	log       *logger.Logger
	entClient *ent.Client
	approval  *approval.Manager
	taskStore *tasks.Store
}

func NewResolveOrchestrator(
	cfg *config.Config,
	log *logger.Logger,
	entClient *ent.Client,
	approvalMgr *approval.Manager,
	taskStore *tasks.Store,
) *ResolveOrchestrator {
	return &ResolveOrchestrator{
		cfg:       cfg,
		log:       log,
		entClient: entClient,
		approval:  approvalMgr,
		taskStore: taskStore,
	}
}

func (o *ResolveOrchestrator) PreviewLaunchPolicy(
	ctx context.Context,
	session *toolsessions.Session,
	agentKind string,
	requestedTool string,
) (map[string]any, error) {
	toolName := strings.TrimSpace(requestedTool)
	if toolName == "" && session != nil {
		toolName = strings.TrimSpace(session.Tool)
	}
	if toolName == "" {
		toolName = strings.TrimSpace(strings.ToLower(agentKind))
	}

	approvalMode := ""
	if o != nil && o.cfg != nil {
		approvalMode = strings.TrimSpace(o.cfg.Approval.Mode)
	}
	if approvalMode == "" {
		approvalMode = string(approval.ModeAuto)
	}

	permissionPreview := map[string]any{
		"matched": false,
		"action":  "",
		"source":  "",
	}
	if o != nil && o.entClient != nil && toolName != "" {
		manager, err := permissionrules.NewManager(o.cfg, o.log, o.entClient)
		if err != nil {
			return nil, err
		}
		sessionID := ""
		if session != nil {
			sessionID = strings.TrimSpace(session.ID)
		}
		result, err := manager.Evaluate(ctx, permissionrules.Input{
			ToolName:  toolName,
			SessionID: sessionID,
		})
		if err != nil {
			return nil, err
		}
		permissionPreview["matched"] = result.Matched
		permissionPreview["action"] = strings.TrimSpace(string(result.Action))
		permissionPreview["source"] = strings.TrimSpace(result.Explanation.Source)
	}

	return map[string]any{
		"tool_name":       toolName,
		"approval_mode":   approvalMode,
		"permission_rule": permissionPreview,
	}, nil
}

type ApprovalResult struct {
	Status  int
	Payload map[string]any
}

type ApprovalSummary struct {
	Status       string
	RequestID    string
	Reason       string
	LaunchPolicy map[string]any
}

type ResolveFlowResult struct {
	Session      *toolsessions.Session
	Created      bool
	LaunchPolicy map[string]any
	Approval     *ApprovalResult
	SessionState *tasks.SessionState
}

func (r ResolveFlowResult) ResponseBody() map[string]any {
	body := map[string]any{
		"created":       r.Created,
		"session":       r.Session,
		"launch_policy": r.LaunchPolicy,
	}
	if r.Approval != nil {
		body["approval"] = r.Approval.Payload
	}
	if r.SessionState != nil {
		body["session_runtime_state"] = r.SessionState
	}
	return body
}

func (r ResolveFlowResult) HTTPStatus() int {
	if r.Approval != nil {
		return r.Approval.Status
	}
	if r.Created {
		return http.StatusCreated
	}
	return http.StatusOK
}

func (o *ResolveOrchestrator) HandleLaunchApproval(
	session *toolsessions.Session,
	launchPolicy map[string]any,
) (ApprovalResult, bool, error) {
	if launchPolicy == nil {
		return ApprovalResult{}, false, nil
	}
	toolName, _ := launchPolicy["tool_name"].(string)
	if strings.TrimSpace(toolName) == "" {
		return ApprovalResult{}, false, nil
	}
	sessionID := ""
	if session != nil {
		sessionID = strings.TrimSpace(session.ID)
	}

	if permissionRule, _ := launchPolicy["permission_rule"].(map[string]any); permissionRule != nil {
		matched, _ := permissionRule["matched"].(bool)
		action, _ := permissionRule["action"].(string)
		if matched {
			switch strings.TrimSpace(action) {
			case string(permissionrules.ActionDeny):
				if o != nil && o.taskStore != nil {
					o.taskStore.ClearSessionPendingAction(sessionID)
				}
				return ApprovalResult{
					Status: http.StatusForbidden,
					Payload: map[string]any{
						"status": "denied",
						"reason": "denied by permission rule",
					},
				}, true, nil
			case string(permissionrules.ActionAsk):
				if o == nil || o.approval == nil {
					return ApprovalResult{}, false, fmt.Errorf("approval manager is unavailable")
				}
				requestID, err := o.approval.EnqueueRequest(toolName, map[string]any{
					"tool_name":  toolName,
					"session_id": sessionID,
				}, sessionID)
				if err != nil {
					return ApprovalResult{}, false, err
				}
				if o.taskStore != nil {
					o.taskStore.SetSessionPendingAction(sessionID, toolName, requestID)
				}
				return ApprovalResult{
					Status: http.StatusAccepted,
					Payload: map[string]any{
						"status":     "pending",
						"request_id": requestID,
					},
				}, true, nil
			}
		}
	}

	if o == nil || o.approval == nil {
		return ApprovalResult{}, false, nil
	}
	decision, requestID, err := o.approval.CheckApproval(toolName, map[string]any{
		"tool_name":  toolName,
		"session_id": sessionID,
	}, sessionID)
	if err != nil {
		return ApprovalResult{}, false, err
	}
	switch decision {
	case approval.Denied:
		if o.taskStore != nil {
			o.taskStore.ClearSessionPendingAction(sessionID)
		}
		return ApprovalResult{
			Status: http.StatusForbidden,
			Payload: map[string]any{
				"status": "denied",
				"reason": "denied by approval policy",
			},
		}, true, nil
	case approval.Pending:
		if o.taskStore != nil {
			o.taskStore.SetSessionPendingAction(sessionID, toolName, requestID)
			if mode, ok := o.approval.GetSessionMode(sessionID); ok {
				o.taskStore.SetSessionPermissionMode(sessionID, string(mode))
			} else if o.cfg != nil && strings.TrimSpace(o.cfg.Approval.Mode) != "" {
				o.taskStore.SetSessionPermissionMode(sessionID, strings.TrimSpace(o.cfg.Approval.Mode))
			}
		}
		return ApprovalResult{
			Status: http.StatusAccepted,
			Payload: map[string]any{
				"status":     "pending",
				"request_id": requestID,
			},
		}, true, nil
	case approval.Approved:
		if o.taskStore != nil {
			o.taskStore.ClearSessionPendingAction(sessionID)
		}
	}
	return ApprovalResult{}, false, nil
}

func ApprovalSummaryFromResult(approvalResult ApprovalResult, launchPolicy map[string]any) ApprovalSummary {
	summary := ApprovalSummary{
		LaunchPolicy: launchPolicy,
	}
	if payload := approvalResult.Payload; payload != nil {
		if status, _ := payload["status"].(string); strings.TrimSpace(status) != "" {
			summary.Status = strings.TrimSpace(status)
		}
		if requestID, _ := payload["request_id"].(string); strings.TrimSpace(requestID) != "" {
			summary.RequestID = strings.TrimSpace(requestID)
		}
		if reason, _ := payload["reason"].(string); strings.TrimSpace(reason) != "" {
			summary.Reason = strings.TrimSpace(reason)
		}
	}
	if summary.Status == "" {
		summary.Status = strings.TrimSpace(httpStatusApprovalLabel(approvalResult.Status))
	}
	return summary
}

func ExecuteResolveFlow(
	ctx context.Context,
	manager *Manager,
	orchestrator *ResolveOrchestrator,
	workspacePath string,
	processProbe ProcessProbe,
	processMgr ProcessStarter,
	sessionMgr SessionUpdater,
	transport RuntimeTransport,
	spec SessionSpec,
) (ResolveFlowResult, error) {
	if manager == nil {
		return ResolveFlowResult{}, fmt.Errorf("external agent manager is unavailable")
	}
	session, created, err := manager.ResolveSession(ctx, spec)
	if err != nil {
		return ResolveFlowResult{}, err
	}
	if orchestrator == nil {
		return ResolveFlowResult{}, fmt.Errorf("external agent orchestrator is unavailable")
	}
	launchPolicy, err := orchestrator.PreviewLaunchPolicy(ctx, session, spec.AgentKind, spec.Tool)
	if err != nil {
		return ResolveFlowResult{}, err
	}
	approvalResult, handled, err := orchestrator.HandleLaunchApproval(session, launchPolicy)
	if err != nil {
		return ResolveFlowResult{}, err
	}
	if handled {
		return ResolveFlowResult{
			Session:      session,
			Created:      created,
			LaunchPolicy: launchPolicy,
			Approval:     &approvalResult,
			SessionState: orchestrator.currentSessionState(session),
		}, nil
	}
	if err := EnsureProcess(ctx, workspacePath, processProbe, processMgr, sessionMgr, transport, session); err != nil {
		return ResolveFlowResult{}, err
	}
	return ResolveFlowResult{
		Session:      session,
		Created:      created,
		LaunchPolicy: launchPolicy,
		SessionState: orchestrator.currentSessionState(session),
	}, nil
}

func httpStatusApprovalLabel(status int) string {
	switch status {
	case http.StatusAccepted:
		return "pending"
	case http.StatusForbidden:
		return "denied"
	default:
		return ""
	}
}

func (o *ResolveOrchestrator) currentSessionState(session *toolsessions.Session) *tasks.SessionState {
	if o == nil || o.taskStore == nil || session == nil {
		return nil
	}
	state, ok := o.taskStore.GetSessionState(strings.TrimSpace(session.ID))
	if !ok {
		return nil
	}
	return &state
}
