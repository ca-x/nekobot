package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"nekobot/pkg/config"
)

func TestSkillsCommand_RegistersValidateAndInstallDeps(t *testing.T) {
	for _, path := range [][]string{
		{"skills", "validate", "demo-skill"},
		{"skills", "install-deps", "demo-skill"},
		{"skills", "search", "git"},
		{"skills", "install", "https://example.com/skills/repo.git"},
	} {
		cmd, _, err := rootCmd.Find(path)
		if err != nil {
			t.Fatalf("find command %v: %v", path, err)
		}
		if cmd == nil {
			t.Fatalf("expected command for %v", path)
		}
	}
}

func TestSkillsSearchCommand_RequiresAtLeastOneArg(t *testing.T) {
	if err := skillsSearchCmd.Args(skillsSearchCmd, nil); err == nil {
		t.Fatal("expected args validation error for empty args")
	}
	if err := skillsSearchCmd.Args(skillsSearchCmd, []string{"git"}); err != nil {
		t.Fatalf("expected valid args, got error: %v", err)
	}
}

func TestSkillsValidateCommand_RequiresExactlyOneArg(t *testing.T) {
	if err := skillsValidateCmd.Args(skillsValidateCmd, nil); err == nil {
		t.Fatal("expected args validation error for empty args")
	}
	if err := skillsValidateCmd.Args(skillsValidateCmd, []string{"a", "b"}); err == nil {
		t.Fatal("expected args validation error for extra args")
	}
	if err := skillsValidateCmd.Args(skillsValidateCmd, []string{"demo-skill"}); err != nil {
		t.Fatalf("expected valid args, got error: %v", err)
	}
}

func TestSkillsInstallDepsCommand_RequiresExactlyOneArg(t *testing.T) {
	if err := skillsInstallDepsCmd.Args(skillsInstallDepsCmd, nil); err == nil {
		t.Fatal("expected args validation error for empty args")
	}
	if err := skillsInstallDepsCmd.Args(skillsInstallDepsCmd, []string{"a", "b"}); err == nil {
		t.Fatal("expected args validation error for extra args")
	}
	if err := skillsInstallDepsCmd.Args(skillsInstallDepsCmd, []string{"demo-skill"}); err != nil {
		t.Fatalf("expected valid args, got error: %v", err)
	}
}

func TestSkillsInstallCommand_RequiresExactlyOneArg(t *testing.T) {
	if err := skillsInstallCmd.Args(skillsInstallCmd, nil); err == nil {
		t.Fatal("expected args validation error for empty args")
	}
	if err := skillsInstallCmd.Args(skillsInstallCmd, []string{"a", "b"}); err == nil {
		t.Fatal("expected args validation error for extra args")
	}
	if err := skillsInstallCmd.Args(skillsInstallCmd, []string{"demo-skill"}); err != nil {
		t.Fatalf("expected valid args, got error: %v", err)
	}
}

func TestRunSkillsValidateReportsMissingRequirements(t *testing.T) {
	root, cfgPath := writeSkillsCLIConfig(t)
	t.Setenv("HOME", root)
	t.Setenv(config.ConfigPathEnv, cfgPath)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	runSkillsValidate(cmd, []string{"demo-skill"})

	output := stdout.String()
	for _, fragment := range []string{
		"Skill: Demo Skill",
		"Eligible: no",
		"missing config paths: channels.discord",
		"missing python packages: nekobot_missing_python_pkg",
		"missing node packages: nekobot-missing-node-pkg",
	} {
		if !strings.Contains(output, fragment) {
			t.Fatalf("expected output to contain %q, got:\n%s", fragment, output)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got:\n%s", stderr.String())
	}
}

func TestRunSkillsInstallDepsRunsInstallers(t *testing.T) {
	root, cfgPath := writeSkillsCLIConfig(t)
	t.Setenv("HOME", root)
	t.Setenv(config.ConfigPathEnv, cfgPath)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	runSkillsInstallDeps(cmd, []string{"demo-skill"})

	output := stdout.String()
	for _, fragment := range []string{
		"Installing dependencies for skill demo-skill",
		"[ok] command printf demo-install-ok",
		"Installed 1/1 dependencies",
	} {
		if !strings.Contains(output, fragment) {
			t.Fatalf("expected output to contain %q, got:\n%s", fragment, output)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got:\n%s", stderr.String())
	}
}

func TestRunSkillsSearchReturnsRankedResults(t *testing.T) {
	root, cfgPath := writeSkillsCLIConfig(t)
	t.Setenv("HOME", root)
	t.Setenv(config.ConfigPathEnv, cfgPath)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	runSkillsSearch(cmd, []string{"git"})

	output := stdout.String()
	for _, fragment := range []string{
		"Search query: git",
		"git-helper",
		"Git Helper",
		"repo-advisor",
	} {
		if !strings.Contains(output, fragment) {
			t.Fatalf("expected output to contain %q, got:\n%s", fragment, output)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got:\n%s", stderr.String())
	}
}

func writeSkillsCLIConfig(t *testing.T) (string, string) {
	t.Helper()

	root := t.TempDir()
	skillsDir := filepath.Join(root, ".nekobot", "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatalf("mkdir skills dir: %v", err)
	}

	skillContent := `---
id: demo-skill
name: Demo Skill
enabled: true
requirements:
  config_paths:
    - channels.discord
  python_packages:
    - nekobot_missing_python_pkg
  node_packages:
    - nekobot-missing-node-pkg
  custom:
    install:
      - method: command
        package: "printf demo-install-ok"
---

Demo skill instructions that are long enough for validation.`
	if err := os.WriteFile(filepath.Join(skillsDir, "demo-skill.md"), []byte(skillContent), 0o644); err != nil {
		t.Fatalf("write skill file: %v", err)
	}

	searchSkill := `---
id: git-helper
name: Git Helper
description: Assist with git repositories
enabled: true
tags:
  - git
---

Use git commands carefully.`
	if err := os.WriteFile(filepath.Join(skillsDir, "git-helper.md"), []byte(searchSkill), 0o644); err != nil {
		t.Fatalf("write search skill: %v", err)
	}

	searchSkillTwo := `---
id: repo-advisor
name: Repository Advisor
description: Git workflows and repository maintenance
enabled: true
tags:
  - git
---

Advise on repository health.`
	if err := os.WriteFile(filepath.Join(skillsDir, "repo-advisor.md"), []byte(searchSkillTwo), 0o644); err != nil {
		t.Fatalf("write secondary search skill: %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.SkillsDir = skillsDir

	cfgPath := filepath.Join(root, ".nekobot", "config.json")
	if err := config.SaveToFile(cfg, cfgPath); err != nil {
		t.Fatalf("save config: %v", err)
	}

	return root, cfgPath
}
