package daemonhost

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	daemonv1 "nekobot/gen/go/nekobot/daemon/v1"
)

func TestSafeWorkspacePathRejectsEscape(t *testing.T) {
	workspace := &daemonv1.Workspace{Path: t.TempDir()}
	_, _, _, err := safeWorkspacePath(workspace, "../secret.txt")
	if err == nil || !strings.Contains(err.Error(), "path escapes workspace root") {
		t.Fatalf("expected escape error, got %v", err)
	}
}

func TestListWorkspaceTreeSortsDirectoriesBeforeFiles(t *testing.T) {
	root := t.TempDir()
	_ = os.Mkdir(filepath.Join(root, "b-dir"), 0o755)
	_ = os.Mkdir(filepath.Join(root, "a-dir"), 0o755)
	_ = os.WriteFile(filepath.Join(root, "b.txt"), []byte("b"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "a.txt"), []byte("a"), 0o644)
	resp, err := ListWorkspaceTree(&daemonv1.RuntimeInventory{Workspaces: []*daemonv1.Workspace{{WorkspaceId: "ws-1", Path: root}}}, &daemonv1.ListWorkspaceTreeRequest{WorkspaceId: "ws-1"})
	if err != nil {
		t.Fatalf("ListWorkspaceTree: %v", err)
	}
	if len(resp.Entries) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(resp.Entries))
	}
	got := []string{resp.Entries[0].Name, resp.Entries[1].Name, resp.Entries[2].Name, resp.Entries[3].Name}
	want := []string{"a-dir", "b-dir", "a.txt", "b.txt"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected order: got %v want %v", got, want)
		}
	}
}

func TestReadWorkspaceFileTruncatesAtLimit(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, "demo.txt"), []byte("abcdef"), 0o644)
	resp, err := ReadWorkspaceFile(&daemonv1.RuntimeInventory{Workspaces: []*daemonv1.Workspace{{WorkspaceId: "ws-1", Path: root}}}, &daemonv1.ReadWorkspaceFileRequest{WorkspaceId: "ws-1", Path: "demo.txt", MaxBytes: 4})
	if err != nil {
		t.Fatalf("ReadWorkspaceFile: %v", err)
	}
	if resp.Content != "abcd" || !resp.Truncated || resp.SizeBytes != 6 {
		t.Fatalf("unexpected read response: %+v", resp)
	}
}

func TestHandleWorkspaceTreeRejectsEscapedPath(t *testing.T) {
	root := t.TempDir()
	_ = os.Mkdir(filepath.Join(root, "code"), 0o755)
	server := &Server{inventoryHome: root}
	info, err := BuildInfo("")
	if err != nil {
		t.Fatalf("BuildInfo: %v", err)
	}
	workspaceID := info.MachineId + ":default"
	payload, err := protojson.Marshal(&daemonv1.ListWorkspaceTreeRequest{WorkspaceId: workspaceID, Path: "../etc"})
	if err != nil {
		t.Fatalf("marshal escaped request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/workspace/tree", strings.NewReader(string(payload)))
	resp := httptest.NewRecorder()
	server.handleWorkspaceTree(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", resp.Code, resp.Body.String())
	}
}

func TestHandleWorkspaceFileReturnsContent(t *testing.T) {
	root := t.TempDir()
	codeRoot := filepath.Join(root, "code")
	_ = os.Mkdir(codeRoot, 0o755)
	_ = os.WriteFile(filepath.Join(codeRoot, "hello.txt"), []byte("hello daemon"), 0o644)
	server := &Server{inventoryHome: root}
	info, err := BuildInfo("")
	if err != nil {
		t.Fatalf("BuildInfo: %v", err)
	}
	workspaceID := info.MachineId + ":default"
	payload, err := protojson.Marshal(&daemonv1.ReadWorkspaceFileRequest{WorkspaceId: workspaceID, Path: "hello.txt"})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/workspace/file", strings.NewReader(string(payload)))
	resp := httptest.NewRecorder()
	server.handleWorkspaceFile(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", resp.Code, resp.Body.String())
	}
	var out daemonv1.ReadWorkspaceFileResponse
	if err := protojson.Unmarshal(resp.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if out.Content != "hello daemon" || out.Path != "hello.txt" || out.WorkspaceId != workspaceID {
		t.Fatalf("unexpected response: %+v", out)
	}
}
