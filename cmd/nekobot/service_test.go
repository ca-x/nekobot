package main

import (
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
