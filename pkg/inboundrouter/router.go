package inboundrouter

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"nekobot/pkg/accountbindings"
	"nekobot/pkg/agent"
	"nekobot/pkg/bus"
	"nekobot/pkg/channelaccounts"
	"nekobot/pkg/logger"
	"nekobot/pkg/runtimeagents"
	"nekobot/pkg/session"
)

const (
	// SessionPrefix isolates router-managed sessions from older direct-call channel sessions.
	SessionPrefix = "route"
)

// Router resolves inbound bus messages onto runtime agents and emits outbound replies.
type Router struct {
	log         *logger.Logger
	bus         bus.Bus
	agent       routingAgent
	sessionMgr  *session.Manager
	accounts    *channelaccounts.Manager
	bindings    *accountbindings.Manager
	runtimes    *runtimeagents.Manager
	channelKeys []string
}

type selectedBinding struct {
	binding accountbindings.AccountBinding
	runtime runtimeagents.AgentRuntime
}

type routingAgent interface {
	ChatWithPromptContextDetailed(
		ctx context.Context,
		sess agent.SessionInterface,
		userMessage string,
		promptCtx agent.PromptContext,
	) (string, agent.ChatRouteResult, error)
}

// New creates a router instance.
func New(
	log *logger.Logger,
	messageBus bus.Bus,
	ag routingAgent,
	sessionMgr *session.Manager,
	accountMgr *channelaccounts.Manager,
	bindingMgr *accountbindings.Manager,
	runtimeMgr *runtimeagents.Manager,
) (*Router, error) {
	if log == nil {
		return nil, fmt.Errorf("logger is nil")
	}
	if messageBus == nil {
		return nil, fmt.Errorf("bus is nil")
	}
	if ag == nil {
		return nil, fmt.Errorf("agent is nil")
	}
	if sessionMgr == nil {
		return nil, fmt.Errorf("session manager is nil")
	}
	if accountMgr == nil {
		return nil, fmt.Errorf("channel account manager is nil")
	}
	if bindingMgr == nil {
		return nil, fmt.Errorf("account binding manager is nil")
	}
	if runtimeMgr == nil {
		return nil, fmt.Errorf("runtime agent manager is nil")
	}

	return &Router{
		log:        log,
		bus:        messageBus,
		agent:      ag,
		sessionMgr: sessionMgr,
		accounts:   accountMgr,
		bindings:   bindingMgr,
		runtimes:   runtimeMgr,
	}, nil
}

// RegisterChannel registers one inbound channel identifier with the bus.
func (r *Router) RegisterChannel(channelID string) {
	channelID = strings.TrimSpace(channelID)
	if channelID == "" || r == nil || r.bus == nil {
		return
	}

	r.channelKeys = append(r.channelKeys, channelID)
	r.bus.RegisterInboundHandler(channelID, r.HandleInbound)
}

// UnregisterAll removes all registered inbound handlers.
func (r *Router) UnregisterAll() {
	if r == nil || r.bus == nil {
		return
	}
	for _, channelID := range r.channelKeys {
		r.bus.UnregisterInboundHandlers(channelID)
	}
	r.channelKeys = nil
}

// ChatWebsocket routes a gateway/websocket turn through runtime bindings when configured.
// When no websocket runtime mapping exists yet, it preserves legacy default-agent behavior.
func (r *Router) ChatWebsocket(
	ctx context.Context,
	userID, username, upstreamSessionID, content, runtimeID string,
) (string, map[string]any, error) {
	if r == nil {
		return "", nil, fmt.Errorf("router is nil")
	}

	websocketAccount, err := r.accounts.FindByChannelTypeAndAccountKey(ctx, "websocket", "default")
	if err != nil {
		if !errors.Is(err, channelaccounts.ErrAccountNotFound) {
			return "", nil, err
		}
		if strings.TrimSpace(runtimeID) != "" {
			return "", nil, fmt.Errorf("runtime %s is not available for websocket chat", strings.TrimSpace(runtimeID))
		}
		sess, sessErr := r.sessionMgr.GetWithSource(upstreamSessionID, session.SourceGateway)
		if sessErr != nil {
			return "", nil, fmt.Errorf("get gateway session %s: %w", upstreamSessionID, sessErr)
		}
		response, _, chatErr := r.agent.ChatWithPromptContextDetailed(ctx, sess, content, agent.PromptContext{
			Channel:   "websocket",
			SessionID: upstreamSessionID,
			UserID:    userID,
			Username:  username,
		})
		if chatErr != nil {
			return "", nil, chatErr
		}
		return response, nil, nil
	}
	if websocketAccount == nil || !websocketAccount.Enabled {
		if strings.TrimSpace(runtimeID) != "" {
			return "", nil, fmt.Errorf("runtime %s is not available for websocket chat", strings.TrimSpace(runtimeID))
		}
		return "", nil, nil
	}

	selectedBindings, err := r.selectBindings(ctx, websocketAccount.ID, runtimeID)
	if err != nil {
		return "", nil, err
	}
	if len(selectedBindings) == 0 {
		if strings.TrimSpace(runtimeID) != "" {
			return "", nil, fmt.Errorf(
				"runtime %s is not bound to websocket account %s",
				strings.TrimSpace(runtimeID),
				websocketAccount.ID,
			)
		}
		sess, sessErr := r.sessionMgr.GetWithSource(upstreamSessionID, session.SourceGateway)
		if sessErr != nil {
			return "", nil, fmt.Errorf("get gateway session %s: %w", upstreamSessionID, sessErr)
		}
		response, _, chatErr := r.agent.ChatWithPromptContextDetailed(ctx, sess, content, agent.PromptContext{
			Channel:   "websocket",
			SessionID: upstreamSessionID,
			UserID:    userID,
			Username:  username,
		})
		if chatErr != nil {
			return "", nil, chatErr
		}
		return response, nil, nil
	}

	selection := selectedBindings[0]
	if !selection.runtime.Enabled {
		if strings.TrimSpace(runtimeID) != "" {
			return "", nil, fmt.Errorf("runtime %s is disabled", strings.TrimSpace(runtimeID))
		}
		return "", nil, nil
	}

	reply, metadata, err := r.chatWithRuntime(ctx, &bus.Message{
		ChannelID: "websocket",
		SessionID: upstreamSessionID,
		UserID:    userID,
		Username:  username,
		Type:      bus.MessageTypeText,
		Content:   content,
	}, *websocketAccount, selection.binding, selection.runtime, session.SourceGateway)
	if err != nil {
		return "", nil, err
	}
	return reply, metadata, nil
}

// HandleInbound routes one inbound message through account bindings.
func (r *Router) HandleInbound(ctx context.Context, msg *bus.Message) error {
	if msg == nil {
		return fmt.Errorf("message is nil")
	}

	account, err := r.accounts.ResolveForChannelID(ctx, msg.ChannelID)
	if err != nil {
		if account == nil {
			if errors.Is(err, channelaccounts.ErrAccountNotFound) {
				return r.handleLegacyInbound(ctx, msg)
			}
			r.log.Debug("No channel account mapping for inbound message",
				zap.String("channel_id", msg.ChannelID),
				zap.String("session_id", msg.SessionID),
				zap.Error(err))
			return nil
		}
		return err
	}
	if !account.Enabled {
		r.log.Debug("Skipping inbound message for disabled account",
			zap.String("channel_id", msg.ChannelID),
			zap.String("account_id", account.ID))
		return nil
	}

	selectedBindings, err := r.selectBindings(ctx, account.ID, "")
	if err != nil {
		return err
	}
	if len(selectedBindings) == 0 {
		r.log.Debug("No enabled bindings for channel account",
			zap.String("channel_id", msg.ChannelID),
			zap.String("account_id", account.ID))
		return nil
	}

	for _, item := range selectedBindings {
		if !item.runtime.Enabled {
			continue
		}

		if err := r.dispatchToRuntime(ctx, msg, *account, item.binding, item.runtime); err != nil {
			return err
		}
	}

	return nil
}

func (r *Router) handleLegacyInbound(ctx context.Context, msg *bus.Message) error {
	sess, err := r.sessionMgr.GetWithSource(msg.SessionID, session.SourceChannels)
	if err != nil {
		return fmt.Errorf("get legacy channel session %s: %w", msg.SessionID, err)
	}

	response, _, err := r.agent.ChatWithPromptContextDetailed(ctx, sess, msg.Content, agent.PromptContext{
		Channel:   msg.ChannelID,
		SessionID: msg.SessionID,
		UserID:    msg.UserID,
		Username:  msg.Username,
	})
	if err != nil {
		return fmt.Errorf("legacy channel %s chat: %w", msg.ChannelID, err)
	}

	outbound := &bus.Message{
		ChannelID: msg.ChannelID,
		SessionID: msg.SessionID,
		UserID:    msg.UserID,
		Username:  msg.Username,
		Type:      bus.MessageTypeText,
		Content:   response,
		Data:      cloneMessageData(msg.Data),
		ReplyTo:   msg.ReplyTo,
	}
	if err := r.bus.SendOutbound(outbound); err != nil {
		return fmt.Errorf("send legacy outbound reply for %s: %w", msg.ChannelID, err)
	}

	return nil
}

func (r *Router) selectBindings(
	ctx context.Context,
	channelAccountID string,
	explicitRuntimeID string,
) ([]selectedBinding, error) {
	items, err := r.bindings.ListEnabledByChannelAccountID(ctx, channelAccountID)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}

	if explicitRuntimeID != "" {
		for _, item := range items {
			if item.AgentRuntimeID != explicitRuntimeID {
				continue
			}
			runtimeItem, err := r.runtimes.Get(ctx, item.AgentRuntimeID)
			if err != nil {
				return nil, fmt.Errorf("get runtime %s: %w", item.AgentRuntimeID, err)
			}
			if runtimeItem == nil {
				return nil, nil
			}
			return []selectedBinding{{
				binding: item,
				runtime: *runtimeItem,
			}}, nil
		}
		return nil, nil
	}

	mode := strings.TrimSpace(items[0].BindingMode)
	if mode != accountbindings.ModeMultiAgent {
		runtimeItem, err := r.runtimes.Get(ctx, items[0].AgentRuntimeID)
		if err != nil {
			return nil, fmt.Errorf("get runtime %s: %w", items[0].AgentRuntimeID, err)
		}
		if runtimeItem == nil {
			return nil, nil
		}
		return []selectedBinding{{
			binding: items[0],
			runtime: *runtimeItem,
		}}, nil
	}

	selected := make([]selectedBinding, 0, len(items))
	for _, item := range items {
		runtimeItem, err := r.runtimes.Get(ctx, item.AgentRuntimeID)
		if err != nil {
			return nil, fmt.Errorf("get runtime %s: %w", item.AgentRuntimeID, err)
		}
		if runtimeItem == nil {
			continue
		}
		selected = append(selected, selectedBinding{
			binding: item,
			runtime: *runtimeItem,
		})
	}
	return selected, nil
}

func (r *Router) dispatchToRuntime(
	ctx context.Context,
	msg *bus.Message,
	account channelaccounts.ChannelAccount,
	binding accountbindings.AccountBinding,
	runtimeItem runtimeagents.AgentRuntime,
) error {
	response, metadata, err := r.chatWithRuntime(
		ctx,
		msg,
		account,
		binding,
		runtimeItem,
		session.SourceChannels,
	)
	if err != nil {
		return err
	}

	outbound := &bus.Message{
		ChannelID: msg.ChannelID,
		SessionID: msg.SessionID,
		UserID:    msg.UserID,
		Username:  msg.Username,
		Type:      bus.MessageTypeText,
		Content:   response,
		Data:      mergeMessageData(msg.Data, metadata),
		ReplyTo:   msg.ReplyTo,
	}

	if err := r.bus.SendOutbound(outbound); err != nil {
		return fmt.Errorf("send outbound reply for runtime %s: %w", runtimeItem.ID, err)
	}

	return nil
}

func (r *Router) chatWithRuntime(
	ctx context.Context,
	msg *bus.Message,
	account channelaccounts.ChannelAccount,
	binding accountbindings.AccountBinding,
	runtimeItem runtimeagents.AgentRuntime,
	source string,
) (string, map[string]any, error) {
	sessionID := routedSessionID(runtimeItem.ID, msg.SessionID)
	sess, err := r.sessionMgr.GetWithSource(sessionID, source)
	if err != nil {
		return "", nil, fmt.Errorf("get routed session %s: %w", sessionID, err)
	}

	response, _, err := r.agent.ChatWithPromptContextDetailed(ctx, sess, msg.Content, agent.PromptContext{
		Channel:           msg.ChannelID,
		SessionID:         sessionID,
		UserID:            msg.UserID,
		Username:          msg.Username,
		RequestedProvider: strings.TrimSpace(runtimeItem.Provider),
		RequestedModel:    strings.TrimSpace(runtimeItem.Model),
		ExplicitPromptIDs: runtimePromptIDs(runtimeItem.PromptID),
		Custom: map[string]any{
			"runtime_id":         runtimeItem.ID,
			"runtime_name":       runtimeItem.Name,
			"channel_account_id": account.ID,
			"channel_type":       account.ChannelType,
			"binding_id":         binding.ID,
			"binding_mode":       binding.BindingMode,
			"reply_label":        binding.ReplyLabel,
		},
	})
	if err != nil {
		return "", nil, fmt.Errorf("runtime %s chat: %w", runtimeItem.ID, err)
	}

	replyContent := response
	if strings.TrimSpace(binding.ReplyLabel) != "" {
		replyContent = fmt.Sprintf("[%s] %s", binding.ReplyLabel, response)
	} else if binding.BindingMode == accountbindings.ModeMultiAgent {
		replyContent = fmt.Sprintf("[%s] %s", firstNonEmpty(runtimeItem.DisplayName, runtimeItem.Name, runtimeItem.ID), response)
	}

	return replyContent, map[string]any{
		"runtime_id":   runtimeItem.ID,
		"runtime_name": firstNonEmpty(runtimeItem.DisplayName, runtimeItem.Name, runtimeItem.ID),
		"binding_id":   binding.ID,
		"account_id":   account.ID,
	}, nil
}

func routedSessionID(runtimeID, upstreamSessionID string) string {
	runtimeID = strings.TrimSpace(runtimeID)
	upstreamSessionID = strings.TrimSpace(upstreamSessionID)
	if runtimeID == "" {
		return upstreamSessionID
	}
	if upstreamSessionID == "" {
		return SessionPrefix + ":" + runtimeID
	}
	return SessionPrefix + ":" + runtimeID + ":" + upstreamSessionID
}

func runtimePromptIDs(promptID string) []string {
	promptID = strings.TrimSpace(promptID)
	if promptID == "" {
		return nil
	}
	return []string{promptID}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func cloneMessageData(input map[string]interface{}) map[string]interface{} {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]interface{}, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func mergeMessageData(base, overlay map[string]interface{}) map[string]interface{} {
	if len(base) == 0 && len(overlay) == 0 {
		return nil
	}
	out := cloneMessageData(base)
	if out == nil {
		out = make(map[string]interface{}, len(overlay))
	}
	for key, value := range overlay {
		out[key] = value
	}
	return out
}
