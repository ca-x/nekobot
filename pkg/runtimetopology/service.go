package runtimetopology

import (
	"context"
	"fmt"
	"strings"
	"time"

	"nekobot/pkg/accountbindings"
	"nekobot/pkg/channelaccounts"
	"nekobot/pkg/runtimeagents"
	"nekobot/pkg/tasks"
)

// Summary contains top-level topology counts.
type Summary struct {
	RuntimeCount        int `json:"runtime_count"`
	ChannelAccountCount int `json:"channel_account_count"`
	BindingCount        int `json:"binding_count"`
	SingleAgentAccounts int `json:"single_agent_accounts"`
	MultiAgentAccounts  int `json:"multi_agent_accounts"`
}

// RuntimeNode is the runtime projection for topology views.
type RuntimeNode struct {
	Runtime           runtimeagents.AgentRuntime `json:"runtime"`
	BoundAccountCount int                        `json:"bound_account_count"`
}

// AccountNode is the account projection for topology views.
type AccountNode struct {
	Account           channelaccounts.ChannelAccount `json:"account"`
	BindingMode       string                         `json:"binding_mode"`
	BoundRuntimeCount int                            `json:"bound_runtime_count"`
}

// BindingEdge is the topology edge projection.
type BindingEdge struct {
	Binding          accountbindings.AccountBinding `json:"binding"`
	RuntimeName      string                         `json:"runtime_name"`
	AccountLabel     string                         `json:"account_label"`
	ChannelType      string                         `json:"channel_type"`
	EffectiveEnabled bool                           `json:"effective_enabled"`
	DisabledReason   string                         `json:"disabled_reason,omitempty"`
}

// Snapshot is the full Round 1 runtime topology shape.
type Snapshot struct {
	Summary  Summary       `json:"summary"`
	Runtimes []RuntimeNode `json:"runtimes"`
	Accounts []AccountNode `json:"accounts"`
	Bindings []BindingEdge `json:"bindings"`
}

// Service aggregates runtime/account/binding data into a topology view.
type Service struct {
	runtimes  *runtimeagents.Manager
	accounts  *channelaccounts.Manager
	bindings  *accountbindings.Manager
	taskStore interface {
		List() []tasks.Task
	}
}

// NewService creates a new topology service.
func NewService(
	runtimes *runtimeagents.Manager,
	accounts *channelaccounts.Manager,
	bindings *accountbindings.Manager,
	taskStore interface {
		List() []tasks.Task
	},
) (*Service, error) {
	if runtimes == nil {
		return nil, fmt.Errorf("runtime manager is nil")
	}
	if accounts == nil {
		return nil, fmt.Errorf("channel account manager is nil")
	}
	if bindings == nil {
		return nil, fmt.Errorf("account binding manager is nil")
	}
	return &Service{
		runtimes:  runtimes,
		accounts:  accounts,
		bindings:  bindings,
		taskStore: taskStore,
	}, nil
}

// Snapshot returns the current topology snapshot.
func (s *Service) Snapshot(ctx context.Context) (*Snapshot, error) {
	runtimeList, err := s.runtimes.List(ctx)
	if err != nil {
		return nil, err
	}
	accountList, err := s.accounts.List(ctx)
	if err != nil {
		return nil, err
	}
	bindingList, err := s.bindings.List(ctx)
	if err != nil {
		return nil, err
	}

	runtimeNameByID := make(map[string]string, len(runtimeList))
	runtimeEnabledByID := make(map[string]bool, len(runtimeList))
	runtimeCountByID := make(map[string]int, len(runtimeList))
	for _, item := range runtimeList {
		runtimeNameByID[item.ID] = item.DisplayName
		runtimeEnabledByID[item.ID] = item.Enabled
	}
	accountLabelByID := make(map[string]string, len(accountList))
	accountTypeByID := make(map[string]string, len(accountList))
	accountEnabledByID := make(map[string]bool, len(accountList))
	accountCountByID := make(map[string]int, len(accountList))
	accountModeByID := make(map[string]string, len(accountList))
	for _, item := range accountList {
		label := item.DisplayName
		if label == "" {
			label = item.AccountKey
		}
		accountLabelByID[item.ID] = label
		accountTypeByID[item.ID] = item.ChannelType
		accountEnabledByID[item.ID] = item.Enabled
	}

	edges := make([]BindingEdge, 0, len(bindingList))
	for _, item := range bindingList {
		runtimeCountByID[item.AgentRuntimeID]++
		accountCountByID[item.ChannelAccountID]++
		if accountModeByID[item.ChannelAccountID] == "" {
			accountModeByID[item.ChannelAccountID] = item.BindingMode
		}
		effectiveEnabled, disabledReason := bindingEffectiveState(
			item,
			accountEnabledByID[item.ChannelAccountID],
			runtimeEnabledByID[item.AgentRuntimeID],
		)
		edges = append(edges, BindingEdge{
			Binding:          item,
			RuntimeName:      runtimeNameByID[item.AgentRuntimeID],
			AccountLabel:     accountLabelByID[item.ChannelAccountID],
			ChannelType:      accountTypeByID[item.ChannelAccountID],
			EffectiveEnabled: effectiveEnabled,
			DisabledReason:   disabledReason,
		})
	}

	taskCountByRuntimeID := make(map[string]int, len(runtimeList))
	lastSeenByRuntimeID := make(map[string]time.Time, len(runtimeList))
	if s.taskStore != nil {
		for _, task := range s.taskStore.List() {
			runtimeID := strings.TrimSpace(task.RuntimeID)
			if runtimeID == "" {
				continue
			}
			if !tasks.IsFinal(task.State) {
				taskCountByRuntimeID[runtimeID]++
			}
			taskTime := task.CreatedAt
			if !task.CompletedAt.IsZero() {
				taskTime = task.CompletedAt
			} else if !task.StartedAt.IsZero() {
				taskTime = task.StartedAt
			}
			if taskTime.After(lastSeenByRuntimeID[runtimeID]) {
				lastSeenByRuntimeID[runtimeID] = taskTime
			}
		}
	}

	runtimes := make([]RuntimeNode, 0, len(runtimeList))
	for _, item := range runtimeList {
		status := &runtimeagents.RuntimeDerivedStatus{
			EffectiveAvailable:  item.Enabled && runtimeEnabledByID[item.ID] && countEffectiveBindings(edges, item.ID) > 0,
			AvailabilityReason:  deriveAvailabilityReason(item.Enabled, runtimeCountByID[item.ID], countEffectiveBindings(edges, item.ID)),
			BoundAccountCount:   runtimeCountByID[item.ID],
			EnabledBindingCount: countEffectiveBindings(edges, item.ID),
			CurrentTaskCount:    taskCountByRuntimeID[item.ID],
			LastSeenAt:          runtimeagents.NormalizeTimestamp(lastSeenByRuntimeID[item.ID]),
		}
		item.Status = status
		runtimes = append(runtimes, RuntimeNode{
			Runtime:           item,
			BoundAccountCount: runtimeCountByID[item.ID],
		})
	}

	summary := Summary{
		RuntimeCount:        len(runtimeList),
		ChannelAccountCount: len(accountList),
		BindingCount:        len(bindingList),
	}
	accounts := make([]AccountNode, 0, len(accountList))
	for _, item := range accountList {
		mode := accountModeByID[item.ID]
		if mode == "" {
			mode = accountbindings.ModeSingleAgent
		}
		if mode == accountbindings.ModeMultiAgent {
			summary.MultiAgentAccounts++
		} else {
			summary.SingleAgentAccounts++
		}
		accounts = append(accounts, AccountNode{
			Account:           item,
			BindingMode:       mode,
			BoundRuntimeCount: accountCountByID[item.ID],
		})
	}

	return &Snapshot{
		Summary:  summary,
		Runtimes: runtimes,
		Accounts: accounts,
		Bindings: edges,
	}, nil
}

func bindingEffectiveState(
	item accountbindings.AccountBinding,
	accountEnabled bool,
	runtimeEnabled bool,
) (bool, string) {
	if !item.Enabled {
		return false, "binding_disabled"
	}
	if !accountEnabled {
		return false, "account_disabled"
	}
	if !runtimeEnabled {
		return false, "runtime_disabled"
	}
	return true, ""
}

func countEffectiveBindings(edges []BindingEdge, runtimeID string) int {
	count := 0
	for _, edge := range edges {
		if edge.Binding.AgentRuntimeID == runtimeID && edge.EffectiveEnabled {
			count++
		}
	}
	return count
}

func deriveAvailabilityReason(enabled bool, boundCount int, enabledBindingCount int) string {
	switch {
	case !enabled:
		return "runtime_disabled"
	case enabledBindingCount > 0:
		return "available"
	case boundCount > 0:
		return "no_enabled_bindings"
	default:
		return "unbound"
	}
}
