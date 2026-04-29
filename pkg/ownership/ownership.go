package ownership

import "strings"

const (
	VisibilityPrivate = "private"
	VisibilityShared  = "shared"
	VisibilitySystem  = "system"
)

func NormalizeVisibility(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case VisibilityPrivate:
		return VisibilityPrivate
	case VisibilitySystem:
		return VisibilitySystem
	default:
		return VisibilityShared
	}
}
