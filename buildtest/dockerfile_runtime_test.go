package buildtest

import (
	"os"
	"strings"
	"testing"
)

func TestRuntimeDockerfileInstallsBrowserRuntimeDependency(t *testing.T) {
	content, err := os.ReadFile("../Dockerfile")
	if err != nil {
		t.Fatalf("read Dockerfile: %v", err)
	}

	dockerfile := string(content)
	marker := "# Runtime stage"
	idx := strings.Index(dockerfile, marker)
	if idx < 0 {
		t.Fatal("runtime stage base image not found in Dockerfile")
	}
	runtimeStage := dockerfile[idx:]

	if !strings.Contains(runtimeStage, "apk add --no-cache") {
		t.Fatal("runtime stage does not install packages with apk")
	}
	if !strings.Contains(runtimeStage, "tmux") {
		t.Fatal("runtime stage must include tmux for external tool sessions")
	}
	if !strings.Contains(runtimeStage, "wget") {
		t.Fatal("runtime stage must include wget for the image healthcheck")
	}
	if !strings.Contains(runtimeStage, "chromium") {
		t.Fatal("runtime stage must include chromium for browser tool readiness")
	}
}
