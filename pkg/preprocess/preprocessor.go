// Package preprocess provides user input preprocessing with @file and @dir mention support.
// It parses file references in user messages, reads file contents, and injects them into prompts.
package preprocess

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	// fileMentionRegex matches @file and @dir mentions with optional path in parentheses or bare.
	// Examples: @file, @file.txt, @file(path/to/file.txt), @dir(path/to/dir)
	fileMentionRegex = regexp.MustCompile(`@(?:file|dir)(?:\(([^)]+)\))?|\@([a-zA-Z0-9._\-/]+\.[a-zA-Z0-9]+)`)

	// dirMentionRegex specifically matches @dir mentions
	dirMentionRegex = regexp.MustCompile(`@dir(?:\(([^)]+)\))?`)
)

// FileContent represents the content of a file with metadata.
type FileContent struct {
	Path        string    // Absolute path
	RelativePath string  // Relative path from workspace
	Content     string    // File content (may be truncated)
	Size        int64     // Original file size
	Truncated   bool      // Whether content was truncated
	Summary     string    // Summary for large files (if applicable)
	Hash        string    // SHA256 hash of content for cache validation
	ModTime     time.Time // File modification time
	Error       string    // Error message if file couldn't be read
}

// MentionType represents the type of mention.
type MentionType string

const (
	MentionFile MentionType = "file"
	MentionDir  MentionType = "dir"
)

// Mention represents a parsed @file or @dir mention.
type Mention struct {
	Type       MentionType
	RawMatch   string
	Path       string
	IsRelative bool
	Position   int // Position in original text
}

// PreprocessorConfig holds configuration for the preprocessor.
type PreprocessorConfig struct {
	// Workspace is the base directory for resolving relative paths.
	Workspace string

	// MaxFileSize is the maximum file size to read fully (bytes).
	// Files larger than this will be truncated or summarized.
	MaxFileSize int64

	// MaxTotalSize is the maximum total size of all file contents (bytes).
	MaxTotalSize int64

	// MaxFiles is the maximum number of files to process per message.
	MaxFiles int

	// MaxDirDepth is the maximum directory depth for @dir mentions.
	MaxDirDepth int

	// Enabled indicates whether preprocessing is active.
	Enabled bool

	// TruncateStrategy is "start", "end", or "middle" for large files.
	TruncateStrategy string

	// TruncateContext is the number of lines to keep when truncating.
	TruncateContext int
}

// DefaultConfig returns a configuration with sensible defaults.
func DefaultConfig() PreprocessorConfig {
	homeDir, _ := os.UserHomeDir()
	return PreprocessorConfig{
		Workspace:        filepath.Join(homeDir, ".nekobot", "workspace"),
		MaxFileSize:      100 * 1024,      // 100KB
		MaxTotalSize:     500 * 1024,      // 500KB total
		MaxFiles:         10,
		MaxDirDepth:      5,
		Enabled:          true,
		TruncateStrategy: "middle",
		TruncateContext:  50,
	}
}

// Preprocessor handles preprocessing of user inputs with file mentions.
type Preprocessor struct {
	config PreprocessorConfig

	// Cache for file contents to avoid re-reading same files.
	cacheMu sync.RWMutex
	cache   map[string]*cachedFileEntry
}

type cachedFileEntry struct {
	content   *FileContent
	hash      string
	modTime   time.Time
	accessed  time.Time
}

// NewPreprocessor creates a new preprocessor with the given configuration.
func NewPreprocessor(config PreprocessorConfig) *Preprocessor {
	if config.Workspace == "" {
		config.Workspace = DefaultConfig().Workspace
	}
	return &Preprocessor{
		config: config,
		cache:  make(map[string]*cachedFileEntry),
	}
}

// NewPreprocessorWithWorkspace creates a preprocessor with just a workspace path.
func NewPreprocessorWithWorkspace(workspace string) *Preprocessor {
	config := DefaultConfig()
	config.Workspace = workspace
	return NewPreprocessor(config)
}

// Result is the output of preprocessing.
type Result struct {
	// OriginalInput is the unmodified input.
	OriginalInput string

	// ProcessedInput is the input with mentions replaced by content references.
	ProcessedInput string

	// Files contains the content of all referenced files.
	Files []FileContent

	// FileReferences is the markdown-formatted file references section to inject.
	FileReferences string

	// Mentions is the list of parsed mentions.
	Mentions []Mention

	// Warnings contains any non-fatal warnings.
	Warnings []string
}

// Process preprocesses the input text, resolving @file and @dir mentions.
func (p *Preprocessor) Process(input string) (*Result, error) {
	result := &Result{
		OriginalInput:  input,
		ProcessedInput: input,
	}

	if !p.config.Enabled {
		return result, nil
	}

	// Parse all mentions
	mentions := p.parseMentions(input)
	if len(mentions) == 0 {
		return result, nil
	}

	// Limit number of files
	if len(mentions) > p.config.MaxFiles {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Too many mentions (%d), limited to %d", len(mentions), p.config.MaxFiles))
		mentions = mentions[:p.config.MaxFiles]
	}

	result.Mentions = mentions

	// Process each mention
	var files []FileContent
	var totalSize int64
	var fileRefs []string

	for _, mention := range mentions {
		if mention.Type == MentionDir {
			// Handle @dir mention
			dirFiles, warnings := p.processDirMention(mention)
			result.Warnings = append(result.Warnings, warnings...)

			for _, f := range dirFiles {
				if totalSize >= p.config.MaxTotalSize {
					result.Warnings = append(result.Warnings, "Total size limit reached, skipping remaining files")
					break
				}
				totalSize += f.Size
				files = append(files, f)
				if f.Error == "" {
					fileRefs = append(fileRefs, p.formatFileReference(f))
				}
			}
		} else {
			// Handle @file mention
			file := p.processFileMention(mention)
			if totalSize+file.Size >= p.config.MaxTotalSize {
				result.Warnings = append(result.Warnings, "Total size limit reached, skipping file: "+file.Path)
				continue
			}
			totalSize += file.Size
			files = append(files, file)
			if file.Error == "" {
				fileRefs = append(fileRefs, p.formatFileReference(file))
			}
		}
	}

	result.Files = files

	// Build file references section
	if len(fileRefs) > 0 {
		result.FileReferences = strings.Join(fileRefs, "\n\n")
		// Replace mentions with [see referenced files] marker
		result.ProcessedInput = p.replaceMentionsWithMarkers(input, mentions)
	}

	return result, nil
}

// parseMentions extracts all @file and @dir mentions from input.
func (p *Preprocessor) parseMentions(input string) []Mention {
	var mentions []Mention

	matches := fileMentionRegex.FindAllStringSubmatchIndex(input, -1)
	seen := make(map[string]bool)

	for _, match := range matches {
		rawMatch := input[match[0]:match[1]]
		position := match[0]

		// Determine if it's a @dir mention
		isDir := dirMentionRegex.MatchString(rawMatch)

		var path string
		if match[2] >= 0 {
			// Path in parentheses: @file(path) or @dir(path)
			path = input[match[2]:match[3]]
		} else if match[4] >= 0 {
			// Bare mention: @file.txt
			path = input[match[4]:match[5]]
		} else {
			// Bare @file or @dir without extension - use placeholder
			if isDir {
				path = "."
			} else {
				continue // Skip bare @file without path
			}
		}

		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}

		// Deduplicate
		key := string(MentionFile) + ":" + path
		if isDir {
			key = string(MentionDir) + ":" + path
		}
		if seen[key] {
			continue
		}
		seen[key] = true

		mentionType := MentionFile
		if isDir {
			mentionType = MentionDir
		}

		mentions = append(mentions, Mention{
			Type:       mentionType,
			RawMatch:   rawMatch,
			Path:       path,
			IsRelative: !filepath.IsAbs(path),
			Position:   position,
		})
	}

	// Sort by position for consistent replacement
	sort.Slice(mentions, func(i, j int) bool {
		return mentions[i].Position < mentions[j].Position
	})

	return mentions
}

// processFileMention resolves and reads a single file mention.
func (p *Preprocessor) processFileMention(mention Mention) FileContent {
	// Resolve path
	absPath, relPath, err := p.resolvePath(mention.Path)
	if err != nil {
		return FileContent{
			Path:  mention.Path,
			Error: err.Error(),
		}
	}

	// Check cache
	if cached := p.getCached(absPath); cached != nil {
		return *cached
	}

	// Read file
	content, err := p.readFile(absPath)
	if err != nil {
		result := FileContent{
			Path:  absPath,
			Error: err.Error(),
		}
		p.cacheFile(absPath, &result)
		return result
	}

	// Handle large files
	var finalContent string
	var truncated bool
	var summary string

	if int64(len(content)) > p.config.MaxFileSize {
		finalContent, summary = p.truncateContent(content)
		truncated = true
	} else {
		finalContent = content
	}

	// Compute hash
	hash := p.computeHash(finalContent)

	// Get file info
	info, err := os.Stat(absPath)
	var modTime time.Time
	if err == nil {
		modTime = info.ModTime()
	}

	result := FileContent{
		Path:        absPath,
		RelativePath: relPath,
		Content:     finalContent,
		Size:        int64(len(content)),
		Truncated:   truncated,
		Summary:     summary,
		Hash:        hash,
		ModTime:     modTime,
	}

	p.cacheFile(absPath, &result)
	return result
}

// processDirMention resolves and reads all files in a directory mention.
func (p *Preprocessor) processDirMention(mention Mention) ([]FileContent, []string) {
	var files []FileContent
	var warnings []string

	// Resolve path
	absPath, _, err := p.resolvePath(mention.Path)
	if err != nil {
		return files, []string{err.Error()}
	}

	// Check if it's a directory
	info, err := os.Stat(absPath)
	if err != nil {
		return files, []string{fmt.Sprintf("Cannot access %s: %v", mention.Path, err)}
	}
	if !info.IsDir() {
		return files, []string{fmt.Sprintf("%s is not a directory", mention.Path)}
	}

	// Walk directory
	err = filepath.Walk(absPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip unreadable paths
		}

		// Skip directories
		if info.IsDir() {
			// Check depth
			relPath, _ := filepath.Rel(absPath, path)
			depth := len(strings.Split(relPath, string(filepath.Separator)))
			if depth > p.config.MaxDirDepth {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip hidden files and common non-text files
		if p.shouldSkipFile(path, info) {
			return nil
		}

		// Create file mention and process
		fileMention := Mention{
			Type:     MentionFile,
			Path:     path,
			IsRelative: false,
		}
		file := p.processFileMention(fileMention)
		files = append(files, file)

		return nil
	})

	if err != nil {
		warnings = append(warnings, fmt.Sprintf("Error walking directory: %v", err))
	}

	return files, warnings
}

// shouldSkipFile determines if a file should be skipped.
func (p *Preprocessor) shouldSkipFile(path string, info os.FileInfo) bool {
	name := filepath.Base(path)

	// Skip hidden files
	if strings.HasPrefix(name, ".") {
		return true
	}

	// Skip common binary/non-text files
	skipExtensions := map[string]bool{
		".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
		".ico": true, ".svg": true, ".webp": true,
		".pdf": true, ".doc": true, ".docx": true, ".xls": true,
		".xlsx": true, ".ppt": true, ".pptx": true,
		".zip": true, ".tar": true, ".gz": true, ".rar": true,
		".7z": true, ".bz2": true, ".xz": true,
		".mp3": true, ".mp4": true, ".avi": true, ".mov": true,
		".wav": true, ".flac": true, ".ogg": true,
		".exe": true, ".dll": true, ".so": true, ".dylib": true,
		".bin": true, ".dat": true,
	}

	ext := strings.ToLower(filepath.Ext(name))
	return skipExtensions[ext]
}

// resolvePath converts a relative path to absolute and returns relative path from workspace.
func (p *Preprocessor) resolvePath(path string) (absPath, relPath string, err error) {
	path = strings.TrimSpace(path)

	// Handle ~ expansion
	if strings.HasPrefix(path, "~") {
		homeDir, _ := os.UserHomeDir()
		path = filepath.Join(homeDir, path[1:])
	}

	// If absolute path
	if filepath.IsAbs(path) {
		absPath = filepath.Clean(path)
		// Try to compute relative path from workspace
		if rel, err := filepath.Rel(p.config.Workspace, absPath); err == nil {
			relPath = rel
		} else {
			relPath = absPath
		}
		return absPath, relPath, nil
	}

	// Relative path - resolve from workspace
	absPath = filepath.Join(p.config.Workspace, path)
	absPath = filepath.Clean(absPath)
	relPath = path

	return absPath, relPath, nil
}

// readFile reads a file with basic error handling.
func (p *Preprocessor) readFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// truncateContent truncates large files according to the configured strategy.
func (p *Preprocessor) truncateContent(content string) (truncated string, summary string) {
	lines := strings.Split(content, "\n")
	totalLines := len(lines)
	contextLines := p.config.TruncateContext

	if contextLines <= 0 {
		contextLines = 50
	}

	if totalLines <= contextLines*2 {
		return content, ""
	}

	var sb strings.Builder
	summary = fmt.Sprintf("[File truncated: showing %d of %d lines]", contextLines*2, totalLines)

	switch p.config.TruncateStrategy {
	case "start":
		sb.WriteString(strings.Join(lines[:contextLines], "\n"))
		sb.WriteString("\n\n")
		sb.WriteString(summary)
		truncated = sb.String()

	case "end":
		sb.WriteString(summary)
		sb.WriteString("\n\n")
		sb.WriteString(strings.Join(lines[totalLines-contextLines:], "\n"))
		truncated = sb.String()

	default:
		sb.WriteString(strings.Join(lines[:contextLines], "\n"))
		sb.WriteString("\n\n")
		sb.WriteString(summary)
		sb.WriteString("\n\n")
		sb.WriteString(strings.Join(lines[totalLines-contextLines:], "\n"))
		truncated = sb.String()
	}

	return truncated, summary
}

// computeHash computes SHA256 hash of content.
func (p *Preprocessor) computeHash(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}

// formatFileReference creates a markdown reference for a file.
func (p *Preprocessor) formatFileReference(file FileContent) string {
	var sb strings.Builder

	path := file.RelativePath
	if path == "" || path == "." {
		path = file.Path
	}

	sb.WriteString(fmt.Sprintf("### %s", path))

	if file.Truncated {
		sb.WriteString(" (truncated)")
	}

	sb.WriteString("\n\n")
	sb.WriteString("```")

	// Try to detect language from extension
	ext := strings.ToLower(filepath.Ext(path))
	language := p.detectLanguage(ext)
	if language != "" {
		sb.WriteString(language)
	}

	sb.WriteString("\n")
	sb.WriteString(file.Content)
	sb.WriteString("\n```")

	return sb.String()
}

// detectLanguage returns the language identifier for a file extension.
func (p *Preprocessor) detectLanguage(ext string) string {
	languages := map[string]string{
		".go":    "go",
		".py":    "python",
		".js":    "javascript",
		".ts":    "typescript",
		".tsx":   "typescript",
		".jsx":   "javascript",
		".rs":    "rust",
		".java":  "java",
		".c":     "c",
		".cpp":   "cpp",
		".h":     "c",
		".hpp":   "cpp",
		".cs":    "csharp",
		".rb":    "ruby",
		".php":   "php",
		".swift": "swift",
		".kt":    "kotlin",
		".scala": "scala",
		".sh":    "bash",
		".bash":  "bash",
		".zsh":   "bash",
		".fish":  "fish",
		".ps1":   "powershell",
		".yaml":  "yaml",
		".yml":   "yaml",
		".json":  "json",
		".toml":  "toml",
		".xml":   "xml",
		".html":  "html",
		".css":   "css",
		".scss":  "scss",
		".sql":   "sql",
		".md":    "markdown",
		".txt":   "",
	}
	return languages[ext]
}

// replaceMentionsWithMarkers replaces @file mentions with reference markers.
func (p *Preprocessor) replaceMentionsWithMarkers(input string, mentions []Mention) string {
	result := input
	offset := 0

	for _, mention := range mentions {
		start := mention.Position + offset
		end := start + len(mention.RawMatch)

		// Replace with a marker that indicates referenced content is appended
		marker := fmt.Sprintf("[Referenced: %s]", mention.Path)
		result = result[:start] + marker + result[end:]
		offset += len(marker) - len(mention.RawMatch)
	}

	return result
}

// getCached retrieves cached file content if still valid.
func (p *Preprocessor) getCached(path string) *FileContent {
	p.cacheMu.RLock()
	defer p.cacheMu.RUnlock()

	entry, exists := p.cache[path]
	if !exists {
		return nil
	}

	// Check if file has been modified
	info, err := os.Stat(path)
	if err != nil {
		return nil
	}

	if info.ModTime().Equal(entry.modTime) {
		entry.accessed = time.Now()
		return entry.content
	}

	return nil
}

// cacheFile stores file content in cache.
func (p *Preprocessor) cacheFile(path string, content *FileContent) {
	p.cacheMu.Lock()
	defer p.cacheMu.Unlock()

	p.cache[path] = &cachedFileEntry{
		content:  content,
		hash:     content.Hash,
		modTime:  content.ModTime,
		accessed: time.Now(),
	}
}

// ClearCache clears the file content cache.
func (p *Preprocessor) ClearCache() {
	p.cacheMu.Lock()
	defer p.cacheMu.Unlock()
	p.cache = make(map[string]*cachedFileEntry)
}

// GetCacheStats returns cache statistics.
func (p *Preprocessor) GetCacheStats() (count int, totalSize int64) {
	p.cacheMu.RLock()
	defer p.cacheMu.RUnlock()

	count = len(p.cache)
	for _, entry := range p.cache {
		totalSize += entry.content.Size
	}
	return
}

// BuildContextInjection builds the context injection string for the processed result.
func (r *Result) BuildContextInjection() string {
	if r.FileReferences == "" {
		return ""
	}
	return "\n\n# Referenced Files\n\n" + r.FileReferences
}

// InjectToPrompt injects the file references into a prompt.
func (r *Result) InjectToPrompt(prompt string) string {
	if r.FileReferences == "" {
		return prompt
	}
	return prompt + r.BuildContextInjection()
}

// InjectToUserMessage injects file references into a user message.
func (r *Result) InjectToUserMessage(message string) string {
	if r.FileReferences == "" {
		return message
	}
	// Append processed input with file references
	return r.ProcessedInput + r.BuildContextInjection()
}
