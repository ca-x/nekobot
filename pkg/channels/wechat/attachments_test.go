package wechat

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractSendableFilePaths(t *testing.T) {
	tmpDir := t.TempDir()
	imagePath := filepath.Join(tmpDir, "reply.png")
	otherPath := filepath.Join(tmpDir, "notes.txt")

	if err := os.WriteFile(imagePath, []byte("png"), 0o600); err != nil {
		t.Fatalf("write image file: %v", err)
	}
	if err := os.WriteFile(otherPath, []byte("txt"), 0o600); err != nil {
		t.Fatalf("write text file: %v", err)
	}

	text := "结果如下：\n" + imagePath + "\n\n附件补充 " + otherPath + "\n" + imagePath
	cleaned, files := extractSendableFilePaths(text)

	if cleaned != "结果如下：\n\n附件补充" {
		t.Fatalf("unexpected cleaned text: %q", cleaned)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
	if files[0] != imagePath {
		t.Fatalf("expected first file %q, got %q", imagePath, files[0])
	}
	if files[1] != otherPath {
		t.Fatalf("expected second file %q, got %q", otherPath, files[1])
	}
}

func TestClassifyWeChatAttachment(t *testing.T) {
	tmpDir := t.TempDir()

	imagePath := filepath.Join(tmpDir, "reply.png")
	videoPath := filepath.Join(tmpDir, "clip.mp4")
	filePath := filepath.Join(tmpDir, "report.svg")

	for _, tc := range []string{imagePath, videoPath, filePath} {
		if err := os.WriteFile(tc, []byte("content"), 0o600); err != nil {
			t.Fatalf("write %s: %v", tc, err)
		}
	}

	kind, err := classifyWeChatAttachment(imagePath)
	if err != nil {
		t.Fatalf("classify image: %v", err)
	}
	if kind != attachmentKindImage {
		t.Fatalf("expected image kind, got %q", kind)
	}

	kind, err = classifyWeChatAttachment(videoPath)
	if err != nil {
		t.Fatalf("classify video: %v", err)
	}
	if kind != attachmentKindVideo {
		t.Fatalf("expected video kind, got %q", kind)
	}

	kind, err = classifyWeChatAttachment(filePath)
	if err != nil {
		t.Fatalf("classify file: %v", err)
	}
	if kind != attachmentKindFile {
		t.Fatalf("expected file kind, got %q", kind)
	}
}
