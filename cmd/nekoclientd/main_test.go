package main

import (
	"path/filepath"
	"reflect"
	"testing"

	"nekobot/pkg/servicecontrol"
)

func TestNekoClientdServiceConfigIncludesConfigFlag(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "nekoclientd.json")
	got := servicecontrol.NekoClientdConfig(configFile).Arguments
	want := []string{"-c", configFile, "run"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected arguments %v, got %v", want, got)
	}
}
