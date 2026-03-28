package wechat

import (
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	wechatFilePathRegex  = regexp.MustCompile(`(?:^|[\s(])(/[^\s"'\x60\]\)]+)`)
	wechatBlankLineRegex = regexp.MustCompile(`\n{3,}`)
)

type attachmentKind string

const (
	attachmentKindImage attachmentKind = "image"
	attachmentKindVideo attachmentKind = "video"
	attachmentKindFile  attachmentKind = "file"
)

func extractSendableFilePaths(text string) (string, []string) {
	matches := wechatFilePathRegex.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return text, nil
	}

	seen := make(map[string]struct{}, len(matches))
	valid := make(map[string]struct{}, len(matches))
	paths := make([]string, 0, len(matches))

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		path := strings.TrimSpace(match[1])
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		if !isSendableFile(path) {
			continue
		}
		valid[path] = struct{}{}
		paths = append(paths, path)
	}

	if len(paths) == 0 {
		return text, nil
	}

	cleaned := wechatFilePathRegex.ReplaceAllStringFunc(text, func(match string) string {
		submatches := wechatFilePathRegex.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}
		path := strings.TrimSpace(submatches[1])
		if _, ok := valid[path]; !ok {
			return match
		}
		prefix := strings.TrimSuffix(match, path)
		if strings.TrimSpace(prefix) == "" {
			return ""
		}
		return prefix
	})
	cleaned = wechatBlankLineRegex.ReplaceAllString(cleaned, "\n\n")
	cleaned = strings.TrimSpace(cleaned)

	return cleaned, paths
}

func classifyWeChatAttachment(path string) (attachmentKind, error) {
	if !isSendableFile(path) {
		return "", fmt.Errorf("attachment file %s is not readable", path)
	}

	mimeType := detectMIMEType(path)
	switch {
	case isRasterImage(mimeType):
		return attachmentKindImage, nil
	case strings.HasPrefix(mimeType, "video/"):
		return attachmentKindVideo, nil
	default:
		return attachmentKindFile, nil
	}
}

func isSendableFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode().IsRegular()
}

func detectMIMEType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != "" {
		if byExt := mime.TypeByExtension(ext); byExt != "" {
			return normalizeMIMEType(byExt)
		}
	}

	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer func() {
		_ = file.Close()
	}()

	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil && n == 0 {
		return ""
	}

	return normalizeMIMEType(http.DetectContentType(buf[:n]))
}

func normalizeMIMEType(value string) string {
	mediaType, _, err := mime.ParseMediaType(value)
	if err != nil {
		return strings.TrimSpace(value)
	}
	return strings.TrimSpace(mediaType)
}

func isRasterImage(mimeType string) bool {
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "image/png", "image/jpeg", "image/jpg", "image/gif", "image/bmp", "image/tiff":
		return true
	default:
		return false
	}
}
