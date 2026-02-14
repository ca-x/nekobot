package commands

import (
	"regexp"
	"strings"
)

const (
	// InteractionTypeSkillInstallConfirm asks user to confirm skill installation.
	InteractionTypeSkillInstallConfirm = "skill_install_confirm"
)

var repoPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+/[A-Za-z0-9._-]+$`)

// SkillInstallProposal is a parsed skill install suggestion from model output.
type SkillInstallProposal struct {
	Repo    string
	Reason  string
	Message string
}

// ParseSkillInstallProposal parses standardized proposal lines from model output.
func ParseSkillInstallProposal(content string) (SkillInstallProposal, bool) {
	var p SkillInstallProposal
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "SKILL_INSTALL_PROPOSAL:"):
			p.Repo = strings.TrimSpace(strings.TrimPrefix(line, "SKILL_INSTALL_PROPOSAL:"))
		case strings.HasPrefix(line, "REASON:"):
			p.Reason = strings.TrimSpace(strings.TrimPrefix(line, "REASON:"))
		case strings.HasPrefix(line, "MESSAGE:"):
			p.Message = strings.TrimSpace(strings.TrimPrefix(line, "MESSAGE:"))
		}
	}

	if p.Repo == "" || !repoPattern.MatchString(p.Repo) {
		return SkillInstallProposal{}, false
	}
	return p, true
}
