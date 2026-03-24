package skills

import (
	"strings"
	"testing"

	"nekobot/pkg/logger"
)

func newSkillsTestManager(t *testing.T) *Manager {
	t.Helper()

	log, err := logger.New(&logger.Config{Level: logger.LevelError})
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}

	return &Manager{
		log:              log,
		skills:           make(map[string]*Skill),
		eligibilityCheck: NewEligibilityChecker(),
	}
}

func TestListAlwaysEligible(t *testing.T) {
	mgr := newSkillsTestManager(t)
	mgr.skills["always-enabled"] = &Skill{
		ID:           "always-enabled",
		Name:         "Always Enabled",
		Enabled:      true,
		Always:       true,
		Instructions: "always instructions",
	}
	mgr.skills["regular-enabled"] = &Skill{
		ID:           "regular-enabled",
		Name:         "Regular Enabled",
		Enabled:      true,
		Instructions: "regular instructions",
	}
	mgr.skills["always-disabled"] = &Skill{
		ID:           "always-disabled",
		Name:         "Always Disabled",
		Enabled:      false,
		Always:       true,
		Instructions: "disabled always instructions",
	}

	always := mgr.ListAlwaysEligible()
	if len(always) != 1 {
		t.Fatalf("expected 1 always eligible skill, got %d", len(always))
	}
	if always[0].ID != "always-enabled" {
		t.Fatalf("expected always-enabled skill, got %s", always[0].ID)
	}
}

func TestGetInstructionsIncludesAlwaysSection(t *testing.T) {
	mgr := newSkillsTestManager(t)
	mgr.skills["always-a"] = &Skill{
		ID:           "always-a",
		Name:         "Always A",
		Enabled:      true,
		Always:       true,
		Description:  "Always skill description",
		Instructions: "Use always behavior.",
	}
	mgr.skills["regular-z"] = &Skill{
		ID:           "regular-z",
		Name:         "Regular Z",
		Enabled:      true,
		Description:  "Regular skill description",
		Instructions: "Use regular behavior.",
	}

	instructions := mgr.GetInstructions()
	if instructions == "" {
		t.Fatalf("expected non-empty instructions")
	}

	expected := strings.TrimSpace(`# Always Skills

<skill id="always-a" name="Always A" always="true">
  <description>Always skill description</description>
  <instructions>
Use always behavior.
  </instructions>
</skill>

---

# Available Skills

<skills>
  <skill id="regular-z" name="Regular Z" instructions_length="21">
    <description>Regular skill description</description>
  </skill>
</skills>

Use the skill tool with action "invoke" and the skill_id to load detailed instructions when needed.`)
	if instructions != expected {
		t.Fatalf("unexpected instructions:\n%s", instructions)
	}
}

func TestGetInstructionsSkipsDisabledAlwaysSkill(t *testing.T) {
	mgr := newSkillsTestManager(t)
	mgr.skills["always-disabled"] = &Skill{
		ID:           "always-disabled",
		Name:         "Always Disabled",
		Enabled:      false,
		Always:       true,
		Instructions: "should not appear",
	}
	mgr.skills["regular-enabled"] = &Skill{
		ID:           "regular-enabled",
		Name:         "Regular Enabled",
		Enabled:      true,
		Instructions: "regular only",
	}

	instructions := mgr.GetInstructions()
	if strings.Contains(instructions, "always-disabled") {
		t.Fatalf("disabled always skill should not be included: %s", instructions)
	}
	if strings.Contains(instructions, "# Always Skills") {
		t.Fatalf("always section should not exist when no enabled always skills: %s", instructions)
	}
	if !strings.Contains(instructions, "<skill id=\"regular-enabled\" name=\"Regular Enabled\"") {
		t.Fatalf("expected regular skill summary to remain: %s", instructions)
	}
}

func TestFormatSkillSummaryXMLInstructionLength(t *testing.T) {
	skills := []*Skill{{
		ID:           "summary-a",
		Name:         "Summary A",
		Description:  "Summary description",
		Instructions: "αβ",
	}}

	summary := formatSkillSummaryXML(skills)
	if !strings.Contains(summary, `instructions_length="2"`) {
		t.Fatalf("expected rune length in summary, got: %s", summary)
	}
}

func TestValidateAlwaysWarnsWhenDisabled(t *testing.T) {
	validator := NewValidator()
	skill := &Skill{
		ID:           "always-disabled",
		Name:         "Always Disabled",
		Always:       true,
		Enabled:      false,
		Instructions: "test instructions",
	}
	diagnostics := validator.ValidateAlways(skill)
	if len(diagnostics) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diagnostics))
	}
	if diagnostics[0].Severity != DiagnosticWarning {
		t.Fatalf("expected warning severity, got %s", diagnostics[0].Severity)
	}
	if diagnostics[0].Field != "always" {
		t.Fatalf("expected always field diagnostic, got %s", diagnostics[0].Field)
	}
}

func TestCheckRequirementsReportIncludesStructuredMissingDeps(t *testing.T) {
	mgr := newSkillsTestManager(t)
	mgr.eligibilityCheck.SetPythonPackageInstalled(func(pkg string) bool {
		return false
	})
	mgr.eligibilityCheck.SetNodePackageInstalled(func(pkg string) bool {
		return pkg == "typescript"
	})
	mgr.skills["report-skill"] = &Skill{
		ID:      "report-skill",
		Name:    "Report Skill",
		Enabled: true,
		Requirements: &SkillRequirements{
			Binaries:       []string{"definitely-missing-bin"},
			AnyBinaries:    []string{"missing-a", "missing-b"},
			Env:            []string{"REPORT_SKILL_ENV"},
			ConfigPaths:    []string{"channels.discord"},
			PythonPackages: []string{"requests"},
			NodePackages:   []string{"typescript"},
		},
	}
	mgr.eligibilityCheck.SetConfigPathExists(func(path string) bool {
		return false
	})

	report, err := mgr.CheckRequirementsReport("report-skill")
	if err != nil {
		t.Fatalf("check requirements report: %v", err)
	}
	if report.Eligible {
		t.Fatalf("expected report to be ineligible")
	}
	if len(report.MissingBinaries) != 1 || report.MissingBinaries[0] != "definitely-missing-bin" {
		t.Fatalf("unexpected missing binaries: %#v", report.MissingBinaries)
	}
	if len(report.MissingAnyBinaries) != 2 {
		t.Fatalf("unexpected missing any-binaries: %#v", report.MissingAnyBinaries)
	}
	if len(report.MissingEnvVars) != 1 || report.MissingEnvVars[0] != "REPORT_SKILL_ENV" {
		t.Fatalf("unexpected missing env vars: %#v", report.MissingEnvVars)
	}
	if len(report.MissingPaths) != 1 || report.MissingPaths[0] != "channels.discord" {
		t.Fatalf("unexpected missing config paths: %#v", report.MissingPaths)
	}
	if len(report.MissingPythonPackages) != 1 || report.MissingPythonPackages[0] != "requests" {
		t.Fatalf("unexpected missing python packages: %#v", report.MissingPythonPackages)
	}
	if len(report.MissingNodePackages) != 0 {
		t.Fatalf("expected node packages to be satisfied, got %#v", report.MissingNodePackages)
	}
	if len(report.Reasons) == 0 {
		t.Fatalf("expected report reasons")
	}
}
