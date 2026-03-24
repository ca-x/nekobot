package skills

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"nekobot/pkg/config"
	"nekobot/pkg/logger"
)

func TestParseSkillContentAlwaysFromTopLevel(t *testing.T) {
	content := `---
id: top-level-always
name: top-level
always: true
---

Top-level always instructions`

	skill, err := parseSkillContent(content, "/tmp/top-level/SKILL.md")
	if err != nil {
		t.Fatalf("parse skill content: %v", err)
	}
	if !skill.Always {
		t.Fatalf("expected always=true from top-level frontmatter")
	}
}

func TestParseSkillContentAlwaysFromOpenclawMetadata(t *testing.T) {
	content := `---
id: metadata-always
name: metadata
metadata:
  openclaw:
    always: true
---

Metadata always instructions`

	skill, err := parseSkillContent(content, "/tmp/metadata/SKILL.md")
	if err != nil {
		t.Fatalf("parse skill content: %v", err)
	}
	if !skill.Always {
		t.Fatalf("expected always=true from metadata.openclaw.always")
	}
}

func TestParseSkillContentAlwaysDefaultsFalse(t *testing.T) {
	content := `---
id: no-always
name: no-always
---

No always instructions`

	skill, err := parseSkillContent(content, "/tmp/no-always/SKILL.md")
	if err != nil {
		t.Fatalf("parse skill content: %v", err)
	}
	if skill.Always {
		t.Fatalf("expected always=false when not configured")
	}
}

func TestParseSkillContentMergesOpenClawRequirements(t *testing.T) {
	content := `---
id: coding-agent
name: coding-agent
metadata:
  openclaw:
    requires:
      bins: ["git"]
      anyBins: ["claude", "codex"]
      env: ["OPENAI_API_KEY"]
      config: ["channels.discord"]
      os: ["linux", "darwin"]
      pythonPkgs: ["requests"]
      nodePkgs: ["typescript"]
---

Coding agent instructions`

	skill, err := parseSkillContent(content, "/tmp/coding-agent/SKILL.md")
	if err != nil {
		t.Fatalf("parse skill content: %v", err)
	}
	if skill.Requirements == nil {
		t.Fatalf("expected merged requirements")
	}
	if len(skill.Requirements.Binaries) != 1 || skill.Requirements.Binaries[0] != "git" {
		t.Fatalf("unexpected binaries: %#v", skill.Requirements.Binaries)
	}
	if len(skill.Requirements.AnyBinaries) != 2 || skill.Requirements.AnyBinaries[0] != "claude" || skill.Requirements.AnyBinaries[1] != "codex" {
		t.Fatalf("unexpected any binaries: %#v", skill.Requirements.AnyBinaries)
	}
	if len(skill.Requirements.Env) != 1 || skill.Requirements.Env[0] != "OPENAI_API_KEY" {
		t.Fatalf("unexpected env requirements: %#v", skill.Requirements.Env)
	}
	if len(skill.Requirements.ConfigPaths) != 1 || skill.Requirements.ConfigPaths[0] != "channels.discord" {
		t.Fatalf("unexpected config requirements: %#v", skill.Requirements.ConfigPaths)
	}
	if len(skill.Requirements.PythonPackages) != 1 || skill.Requirements.PythonPackages[0] != "requests" {
		t.Fatalf("unexpected python package requirements: %#v", skill.Requirements.PythonPackages)
	}
	if len(skill.Requirements.NodePackages) != 1 || skill.Requirements.NodePackages[0] != "typescript" {
		t.Fatalf("unexpected node package requirements: %#v", skill.Requirements.NodePackages)
	}
	osReq, ok := skill.Requirements.Custom["os"].([]string)
	if !ok || len(osReq) != 2 {
		t.Fatalf("expected os requirements merged, got %#v", skill.Requirements.Custom["os"])
	}
}

func TestParseSkillContentMergesInstallMetadata(t *testing.T) {
	content := `---
id: github
name: GitHub
metadata:
  goclaw:
    install:
      - kind: brew
        formula: gh
      - kind: apt
        package: gh
      - kind: custom
        command: ./scripts/install-gh.sh
        postHook: ./scripts/verify-gh.sh
---

GitHub instructions`

	skill, err := parseSkillContent(content, "/tmp/github/SKILL.md")
	if err != nil {
		t.Fatalf("parse skill content: %v", err)
	}
	if skill.Requirements == nil || skill.Requirements.Custom == nil {
		t.Fatalf("expected merged requirements with custom install metadata")
	}

	specs := ParseRequirementsToSpecs(skill.Requirements)
	if len(specs) != 3 {
		t.Fatalf("expected 3 install specs, got %d", len(specs))
	}

	if specs[0].Method != "brew" || specs[0].Package != "gh" {
		t.Fatalf("unexpected brew spec: %#v", specs[0])
	}
	if specs[1].Method != "apt" || specs[1].Package != "gh" {
		t.Fatalf("unexpected apt spec: %#v", specs[1])
	}
	if specs[2].Method != "command" || specs[2].Package != "./scripts/install-gh.sh" {
		t.Fatalf("unexpected command spec: %#v", specs[2])
	}
	if specs[2].PostHook != "./scripts/verify-gh.sh" {
		t.Fatalf("unexpected command post hook: %#v", specs[2].PostHook)
	}
}

func TestLoadFromSourceLoadsDirectorySkillMarkdown(t *testing.T) {
	log, err := logger.New(&logger.Config{Level: logger.LevelError})
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}

	sourceRoot := filepath.Join(t.TempDir(), "skills")
	if err := os.MkdirAll(filepath.Join(sourceRoot, "weather"), 0755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}

	content := `---
id: weather
name: Weather
---

Weather instructions.`
	if err := os.WriteFile(filepath.Join(sourceRoot, "weather", "SKILL.md"), []byte(content), 0644); err != nil {
		t.Fatalf("write skill file: %v", err)
	}

	loader := &MultiPathLoader{
		log:     log,
		sources: nil,
	}
	loaded, err := loader.loadFromSource(SkillSource{
		Path: sourceRoot,
		Type: SourceLocal,
	})
	if err != nil {
		t.Fatalf("load from source: %v", err)
	}

	skill, ok := loaded["weather"]
	if !ok {
		t.Fatalf("expected directory skill to be discovered")
	}
	if want := filepath.Join(sourceRoot, "weather", "SKILL.md"); skill.FilePath != want {
		t.Fatalf("expected file path %q, got %q", want, skill.FilePath)
	}
}

func TestLoadFromSourceLoadsRootAndDirectorySkills(t *testing.T) {
	log, err := logger.New(&logger.Config{Level: logger.LevelError})
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}

	sourceRoot := filepath.Join(t.TempDir(), "skills")
	if err := os.MkdirAll(filepath.Join(sourceRoot, "weather"), 0755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}

	rootSkill := `---
id: root-note
name: Root Note
---

Root instructions.`
	if err := os.WriteFile(filepath.Join(sourceRoot, "root-note.md"), []byte(rootSkill), 0644); err != nil {
		t.Fatalf("write root skill file: %v", err)
	}

	dirSkill := `---
id: weather
name: Weather
---

Weather instructions.`
	if err := os.WriteFile(filepath.Join(sourceRoot, "weather", "SKILL.md"), []byte(dirSkill), 0644); err != nil {
		t.Fatalf("write directory skill file: %v", err)
	}

	loader := &MultiPathLoader{
		log:     log,
		sources: nil,
	}
	loaded, err := loader.loadFromSource(SkillSource{
		Path: sourceRoot,
		Type: SourceWorkspace,
	})
	if err != nil {
		t.Fatalf("load from source: %v", err)
	}

	if _, ok := loaded["root-note"]; !ok {
		t.Fatalf("expected root markdown skill to be discovered")
	}
	if _, ok := loaded["weather"]; !ok {
		t.Fatalf("expected directory SKILL.md skill to be discovered")
	}
}

func TestEligibilityCheckSupportsAnyBinariesAndMetadataOS(t *testing.T) {
	checker := NewEligibilityChecker()

	eligible, reasons := checker.Check(&Skill{
		ID:      "coding-agent",
		Enabled: true,
		Requirements: &SkillRequirements{
			AnyBinaries: []string{"definitely-missing-bin", "go"},
			Custom: map[string]interface{}{
				"os": []string{runtime.GOOS},
			},
		},
	})
	if !eligible {
		t.Fatalf("expected any-binary requirement to pass when one binary exists, got reasons: %v", reasons)
	}

	ineligible, reasons := checker.Check(&Skill{
		ID:      "wrong-os",
		Enabled: true,
		Requirements: &SkillRequirements{
			Custom: map[string]interface{}{
				"os": []string{"plan9"},
			},
		},
	})
	if ineligible {
		t.Fatalf("expected wrong-os skill to be ineligible")
	}
	if len(reasons) == 0 {
		t.Fatalf("expected ineligible reasons")
	}
}

func TestEligibilityCheckSupportsRuntimeConfigPaths(t *testing.T) {
	checker := NewEligibilityChecker()
	checker.SetConfigPathExists(func(path string) bool {
		return path == "channels.discord"
	})

	eligible, reasons := checker.Check(&Skill{
		ID:      "discord-helper",
		Enabled: true,
		Requirements: &SkillRequirements{
			ConfigPaths: []string{"channels.discord"},
		},
	})
	if !eligible {
		t.Fatalf("expected config-gated skill to be eligible, got reasons: %v", reasons)
	}

	ineligible, reasons := checker.Check(&Skill{
		ID:      "wechat-helper",
		Enabled: true,
		Requirements: &SkillRequirements{
			ConfigPaths: []string{"channels.wechat"},
		},
	})
	if ineligible {
		t.Fatalf("expected missing config path to make skill ineligible")
	}
	if len(reasons) != 1 || reasons[0] != "missing config paths: channels.wechat" {
		t.Fatalf("unexpected reasons: %v", reasons)
	}
}

func TestEligibilityCheckSupportsPythonAndNodePackages(t *testing.T) {
	checker := NewEligibilityChecker()
	checker.SetPythonPackageInstalled(func(pkg string) bool {
		return pkg == "requests"
	})
	checker.SetNodePackageInstalled(func(pkg string) bool {
		return false
	})

	eligible, reasons := checker.Check(&Skill{
		ID:      "package-skill",
		Enabled: true,
		Requirements: &SkillRequirements{
			PythonPackages: []string{"requests"},
			NodePackages:   []string{"typescript"},
		},
	})
	if eligible {
		t.Fatalf("expected missing node package to make skill ineligible")
	}
	if len(reasons) != 1 || reasons[0] != "missing node packages: typescript" {
		t.Fatalf("unexpected reasons: %v", reasons)
	}
}

func TestHasConfigPathChecksEnabledChannels(t *testing.T) {
	cfg := &config.Config{}
	cfg.Channels.Discord.Enabled = true
	cfg.Channels.WeChat.Enabled = false

	if !hasConfigPath(cfg, "channels.discord") {
		t.Fatalf("expected enabled discord channel config path to exist")
	}
	if hasConfigPath(cfg, "channels.wechat") {
		t.Fatalf("expected disabled wechat channel config path to be missing")
	}
	if hasConfigPath(cfg, "channels.unknown") {
		t.Fatalf("expected unknown config path to be missing")
	}
}
