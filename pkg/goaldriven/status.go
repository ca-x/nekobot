package goaldriven

import "fmt"

// CanTransition reports whether one GoalStatus may move to another.
func CanTransition(from, to GoalStatus) bool {
	switch from {
	case GoalStatusDraft:
		return to == GoalStatusCriteriaPendingConfirm || to == GoalStatusCanceled
	case GoalStatusCriteriaPendingConfirm:
		return to == GoalStatusReady || to == GoalStatusCanceled
	case GoalStatusReady:
		return to == GoalStatusRunning || to == GoalStatusCanceled
	case GoalStatusRunning:
		return to == GoalStatusVerifying || to == GoalStatusNeedsApproval || to == GoalStatusCanceled || to == GoalStatusFailed
	case GoalStatusVerifying:
		return to == GoalStatusRunning || to == GoalStatusCompleted || to == GoalStatusNeedsHumanConfirmation || to == GoalStatusFailed
	case GoalStatusNeedsApproval:
		return to == GoalStatusRunning || to == GoalStatusCanceled || to == GoalStatusFailed
	case GoalStatusNeedsHumanConfirmation:
		return to == GoalStatusCompleted || to == GoalStatusRunning || to == GoalStatusCanceled
	default:
		return false
	}
}

// ValidateTransition rejects invalid GoalStatus transitions.
func ValidateTransition(from, to GoalStatus) error {
	if !CanTransition(from, to) {
		return fmt.Errorf("invalid goal status transition: %s -> %s", from, to)
	}
	return nil
}
