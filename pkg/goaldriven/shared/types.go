package shared

// RiskLevel defines how aggressively a GoalRun may act.
type RiskLevel string

// ExecutionScopeKind identifies where a GoalRun should execute.
type ExecutionScopeKind string

const (
	RiskConservative RiskLevel = "conservative"
	RiskBalanced     RiskLevel = "balanced"
	RiskAggressive   RiskLevel = "aggressive"
)

const (
	ScopeServer ExecutionScopeKind = "server"
	ScopeDaemon ExecutionScopeKind = "daemon"
)

// ExecutionScope describes the selected execution surface for one GoalRun.
type ExecutionScope struct {
	Kind      ExecutionScopeKind `json:"kind"`
	MachineID string             `json:"machine_id,omitempty"`
	Source    string             `json:"source"` // auto | manual
	Reason    string             `json:"reason,omitempty"`
}
