package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"nekobot/pkg/config"
)

func TestServiceConfig_DefaultArguments(t *testing.T) {
	originalConfigPath := configPath
	t.Cleanup(func() {
		configPath = originalConfigPath
	})

	configPath = ""
	t.Setenv(config.ConfigPathEnv, "")

	got := ServiceConfig().Arguments
	want := []string{"gateway", "run"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected arguments %v, got %v", want, got)
	}
}

func TestServiceConfig_IncludesConfigFlagFromGlobalConfigPath(t *testing.T) {
	originalConfigPath := configPath
	t.Cleanup(func() {
		configPath = originalConfigPath
	})

	configFile := filepath.Join(t.TempDir(), "service-config.json")
	configPath = configFile
	t.Setenv(config.ConfigPathEnv, "")

	got := ServiceConfig().Arguments
	want := []string{"-c", configFile, "gateway", "run"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected arguments %v, got %v", want, got)
	}
}

func TestServiceConfig_UsesConfigPathEnvWhenFlagNotProvided(t *testing.T) {
	originalConfigPath := configPath
	t.Cleanup(func() {
		configPath = originalConfigPath
	})

	configPath = ""
	configFile := filepath.Join(t.TempDir(), "env-config.json")
	t.Setenv(config.ConfigPathEnv, configFile)

	got := ServiceConfig().Arguments
	want := []string{"-c", configFile, "gateway", "run"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected arguments %v, got %v", want, got)
	}
}

func TestRootPersistentPreRunPreservesConfigPathEnvWhenFlagUnset(t *testing.T) {
	originalConfigPath := configPath
	t.Cleanup(func() {
		configPath = originalConfigPath
	})

	configPath = ""
	configFile := filepath.Join(t.TempDir(), "env-config.json")
	t.Setenv(config.ConfigPathEnv, configFile)

	if err := rootCmd.PersistentPreRunE(rootCmd, nil); err != nil {
		t.Fatalf("PersistentPreRunE failed: %v", err)
	}

	if got := os.Getenv(config.ConfigPathEnv); got != configFile {
		t.Fatalf("expected %s to remain %q, got %q", config.ConfigPathEnv, configFile, got)
	}
}

func TestRootPersistentPreRunOverridesConfigPathEnvWhenFlagSet(t *testing.T) {
	originalConfigPath := configPath
	t.Cleanup(func() {
		configPath = originalConfigPath
	})

	configFile := filepath.Join(t.TempDir(), "flag-config.json")
	configPath = configFile
	t.Setenv(config.ConfigPathEnv, filepath.Join(t.TempDir(), "env-config.json"))

	if err := rootCmd.PersistentPreRunE(rootCmd, nil); err != nil {
		t.Fatalf("PersistentPreRunE failed: %v", err)
	}

	if got := os.Getenv(config.ConfigPathEnv); got != configFile {
		t.Fatalf("expected %s to be overridden to %q, got %q", config.ConfigPathEnv, configFile, got)
	}
}
