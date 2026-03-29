package preprocess

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseMentions(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name:     "single file mention with path",
			input:    "Check @file(config.json) for settings",
			expected: 1,
		},
		{
			name:     "single file mention bare",
			input:    "Look at @README.md",
			expected: 1,
		},
		{
			name:     "dir mention",
			input:    "Review @dir(src/components)",
			expected: 1,
		},
		{
			name:     "multiple mentions",
			input:    "Check @file(main.go) and @file(utils.go)",
			expected: 2,
		},
		{
			name:     "mixed mentions",
			input:    "See @dir(docs) and @file(README.md)",
			expected: 2,
		},
		{
			name:     "no mentions",
			input:    "Just a normal message",
			expected: 0,
		},
		{
			name:     "duplicate mentions",
			input:    "Check @file(test.go) then @file(test.go) again",
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewPreprocessor(DefaultConfig())
			mentions := p.parseMentions(tt.input)
			if len(mentions) != tt.expected {
				t.Errorf("Expected %d mentions, got %d", tt.expected, len(mentions))
			}
		})
	}
}

func TestResolvePath(t *testing.T) {
	tmpDir := t.TempDir()
	config := PreprocessorConfig{
		Workspace: tmpDir,
	}
	p := NewPreprocessor(config)

	t.Run("absolute path", func(t *testing.T) {
		absPath := filepath.Join(tmpDir, "test.txt")
		abs, rel, err := p.resolvePath(absPath)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if abs != absPath {
			t.Errorf("Expected abs path %s, got %s", absPath, abs)
		}
		if rel != "test.txt" {
			t.Errorf("Expected rel path test.txt, got %s", rel)
		}
	})

	t.Run("relative path", func(t *testing.T) {
		abs, rel, err := p.resolvePath("subdir/test.txt")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		expected := filepath.Join(tmpDir, "subdir", "test.txt")
		if abs != expected {
			t.Errorf("Expected %s, got %s", expected, abs)
		}
		if rel != "subdir/test.txt" {
			t.Errorf("Expected rel subdir/test.txt, got %s", rel)
		}
	})

	t.Run("home directory expansion", func(t *testing.T) {
		homeDir, _ := os.UserHomeDir()
		abs, _, err := p.resolvePath("~/.bashrc")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		expected := filepath.Join(homeDir, ".bashrc")
		if abs != expected {
			t.Errorf("Expected %s, got %s", expected, abs)
		}
	})
}

func TestProcessFileMention(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test file
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := "Hello, World!"
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	config := PreprocessorConfig{
		Workspace:   tmpDir,
		MaxFileSize: 1024,
	}
	p := NewPreprocessor(config)

	mention := Mention{
		Type:       MentionFile,
		Path:       "test.txt",
		IsRelative: true,
	}

	result := p.processFileMention(mention)

	if result.Error != "" {
		t.Errorf("Unexpected error: %s", result.Error)
	}
	if result.Content != testContent {
		t.Errorf("Expected content %q, got %q", testContent, result.Content)
	}
	if result.Size != int64(len(testContent)) {
		t.Errorf("Expected size %d, got %d", len(testContent), result.Size)
	}
	if result.Truncated {
		t.Error("File should not be truncated")
	}
}

func TestProcessLargeFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create large test file
	testFile := filepath.Join(tmpDir, "large.txt")
	var content strings.Builder
	for i := 0; i < 200; i++ {
		content.WriteString("Line ")
		content.WriteString(string(rune(i)))
		content.WriteString("\n")
	}
	largeContent := content.String()
	if err := os.WriteFile(testFile, []byte(largeContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	config := PreprocessorConfig{
		Workspace:        tmpDir,
		MaxFileSize:      500, // Small limit to trigger truncation
		TruncateStrategy: "middle",
		TruncateContext:  10,
	}
	p := NewPreprocessor(config)

	mention := Mention{
		Type:       MentionFile,
		Path:       "large.txt",
		IsRelative: true,
	}

	result := p.processFileMention(mention)

	if result.Error != "" {
		t.Errorf("Unexpected error: %s", result.Error)
	}
	if !result.Truncated {
		t.Error("File should be truncated")
	}
	if result.Summary == "" {
		t.Error("Expected summary for truncated file")
	}
	if len(result.Content) < len(largeContent) {
		// Content should be shorter
		t.Logf("Original: %d bytes, Truncated: %d bytes", len(largeContent), len(result.Content))
	}
}

func TestProcessDirMention(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test directory structure
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	// Create test files
	files := map[string]string{
		"file1.txt":     "Content 1",
		"file2.go":      "package main",
		"subdir/file3.md": "# Title",
		".hidden":       "Hidden content", // Should be skipped
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create %s: %v", path, err)
		}
	}

	config := PreprocessorConfig{
		Workspace:   tmpDir,
		MaxFileSize: 1024,
		MaxDirDepth: 3,
	}
	p := NewPreprocessor(config)

	mention := Mention{
		Type:       MentionDir,
		Path:       ".",
		IsRelative: true,
	}

	results, warnings := p.processDirMention(mention)

	if len(warnings) > 0 {
		t.Errorf("Unexpected warnings: %v", warnings)
	}

	// Should have 3 files (excluding hidden)
	var validFiles int
	for _, r := range results {
		if r.Error == "" {
			validFiles++
		}
	}
	if validFiles != 3 {
		t.Errorf("Expected 3 valid files, got %d", validFiles)
	}
}

func TestProcess(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("Test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	config := PreprocessorConfig{
		Workspace:    tmpDir,
		MaxFileSize:  1024,
		MaxTotalSize: 5000, // Allow enough for test files
		MaxFiles:     10,
		Enabled:      true,
	}
	p := NewPreprocessor(config)

	input := "Check @file(test.txt) for details"
	result, err := p.Process(input)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(result.Files) != 1 {
		t.Errorf("Expected 1 file, got %d", len(result.Files))
	}
	if result.FileReferences == "" {
		t.Error("Expected file references")
	}
	if !strings.Contains(result.ProcessedInput, "[Referenced:") {
		t.Error("Expected mention to be replaced with marker")
	}
}

func TestProcessDisabled(t *testing.T) {
	config := PreprocessorConfig{
		Enabled: false,
	}
	p := NewPreprocessor(config)

	input := "Check @file(test.txt) for details"
	result, err := p.Process(input)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.ProcessedInput != input {
		t.Error("Expected input to remain unchanged when disabled")
	}
	if len(result.Files) != 0 {
		t.Errorf("Expected 0 files, got %d", len(result.Files))
	}
}

func TestCache(t *testing.T) {
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "test.txt")
	content := "Test content"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	config := PreprocessorConfig{
		Workspace:   tmpDir,
		MaxFileSize: 1024,
	}
	p := NewPreprocessor(config)

	// First read
	mention := Mention{
		Type:       MentionFile,
		Path:       "test.txt",
		IsRelative: true,
	}
	result1 := p.processFileMention(mention)

	// Second read (should use cache)
	result2 := p.processFileMention(mention)

	if result1.Hash != result2.Hash {
		t.Error("Cache should return same hash")
	}

	// Check stats
	count, size := p.GetCacheStats()
	if count != 1 {
		t.Errorf("Expected cache count 1, got %d", count)
	}
	if size < int64(len(content)) {
		t.Errorf("Expected cache size >= %d, got %d", len(content), size)
	}

	// Clear cache
	p.ClearCache()
	count, _ = p.GetCacheStats()
	if count != 0 {
		t.Errorf("Expected cache count 0 after clear, got %d", count)
	}
}

func TestDetectLanguage(t *testing.T) {
	p := NewPreprocessor(DefaultConfig())

	tests := []struct {
		ext      string
		expected string
	}{
		{".go", "go"},
		{".py", "python"},
		{".js", "javascript"},
		{".ts", "typescript"},
		{".rs", "rust"},
		{".unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			result := p.detectLanguage(tt.ext)
			if result != tt.expected {
				t.Errorf("Expected %s for %s, got %s", tt.expected, tt.ext, result)
			}
		})
	}
}

func TestTruncateContent(t *testing.T) {
	tests := []struct {
		name       string
		strategy   string
		lineCount  int
		context    int
		expectHead bool
		expectTail bool
	}{
		{"start", "start", 100, 10, true, false},
		{"end", "end", 100, 10, false, true},
		{"middle", "middle", 100, 10, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := PreprocessorConfig{
				TruncateStrategy: tt.strategy,
				TruncateContext:  tt.context,
			}
			p := NewPreprocessor(config)

			var content strings.Builder
			for i := 0; i < tt.lineCount; i++ {
				content.WriteString(fmt.Sprintf("Line %d\n", i))
			}

			truncated, summary := p.truncateContent(content.String())

			if summary == "" {
				t.Error("Expected summary")
			}

			if tt.expectHead && !strings.HasPrefix(strings.TrimSpace(truncated), "Line 0") {
				t.Errorf("Expected first lines in truncated content, got: %q", truncated[:50])
			}
			if tt.expectTail && !strings.HasSuffix(strings.TrimSpace(truncated), "Line 99") {
				t.Errorf("Expected last lines in truncated content, got: %q", truncated[len(truncated)-50:])
			}
		})
	}
}

func TestBuildContextInjection(t *testing.T) {
	result := &Result{
		FileReferences: "### test.txt\n```\ncontent\n```",
	}

	injection := result.BuildContextInjection()
	if injection == "" {
		t.Error("Expected injection string")
	}
	if !strings.Contains(injection, "# Referenced Files") {
		t.Error("Expected header in injection")
	}

	// Test empty case
	emptyResult := &Result{}
	injection = emptyResult.BuildContextInjection()
	if injection != "" {
		t.Error("Expected empty injection for empty result")
	}
}

func TestFormatFileReference(t *testing.T) {
	config := DefaultConfig()
	p := NewPreprocessor(config)

	file := FileContent{
		Path:         "/workspace/test.go",
		RelativePath: "test.go",
		Content:      "package main",
		Size:         12,
		Truncated:    false,
	}

	ref := p.formatFileReference(file)

	if !strings.Contains(ref, "### test.go") {
		t.Error("Expected path in reference")
	}
	if !strings.Contains(ref, "```go") {
		t.Error("Expected language identifier")
	}
	if !strings.Contains(ref, "package main") {
		t.Error("Expected content in reference")
	}
}
