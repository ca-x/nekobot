package media

import (
	"bytes"
	"context"
	"crypto/aes"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	// Item types aligned with WeChat iLink message item kinds.
	ItemTypeText  = 1
	ItemTypeImage = 2
	ItemTypeVoice = 3
	ItemTypeFile  = 4
	ItemTypeVideo = 5

	defaultCDNBaseURL = "https://novac2c.cdn.weixin.qq.com/c2c"
	maxMediaBytes     = 50 * 1024 * 1024
)

// CDNMedia describes a WeChat CDN media reference.
type CDNMedia struct {
	EncryptQueryParam string
	AESKey            string
	EncryptType       int
}

// TextItem carries plain text content.
type TextItem struct {
	Text string
}

// VoiceItem carries WeChat voice metadata.
type VoiceItem struct {
	Media      *CDNMedia
	EncodeType int
	Text       string
}

// ImageItem carries WeChat image metadata.
type ImageItem struct {
	Media   *CDNMedia
	AESKey  string
	Data    string
	Format  string
	Thumb   *CDNMedia
	MidSize int
}

// FileItem carries WeChat file metadata.
type FileItem struct {
	Media    *CDNMedia
	FileName string
}

// VideoItem carries WeChat video metadata.
type VideoItem struct {
	Media *CDNMedia
	Thumb *CDNMedia
}

// Item is the protocol-independent WeChat message item.
type Item struct {
	Type  int
	Text  *TextItem
	Voice *VoiceItem
	Image *ImageItem
	File  *FileItem
	Video *VideoItem
}

// InboundMediaResult describes a downloaded inbound media file.
type InboundMediaResult struct {
	FilePath string
	MIMEType string
	FileName string
}

// MediaDownloader downloads inbound WeChat media payloads.
type MediaDownloader interface {
	DownloadFromItem(ctx context.Context, item *Item) (*InboundMediaResult, error)
}

// Transcriber converts audio payloads to text.
type Transcriber interface {
	Transcribe(ctx context.Context, audio []byte, filename string) (string, error)
}

// InboundProcessor assembles inbound WeChat items into agent-friendly text.
type InboundProcessor struct {
	downloader  MediaDownloader
	transcriber Transcriber
}

// NewInboundProcessor creates a reusable WeChat inbound media processor.
func NewInboundProcessor(downloader MediaDownloader, transcriber Transcriber) *InboundProcessor {
	return &InboundProcessor{downloader: downloader, transcriber: transcriber}
}

// Process builds a text body from inbound items, downloading and transcribing media when available.
func (p *InboundProcessor) Process(ctx context.Context, items []Item) (string, error) {
	if p == nil {
		return strings.TrimSpace(BuildBody(items, nil)), nil
	}

	var (
		mediaResult *InboundMediaResult
		firstErr    error
	)

	mediaItem := FindMainMediaItem(items)
	if p.downloader != nil && mediaItem != nil {
		result, err := p.downloader.DownloadFromItem(ctx, mediaItem)
		if err != nil {
			firstErr = err
		} else {
			mediaResult = result
		}
	}

	if mediaItem != nil && mediaItem.Type == ItemTypeVoice && mediaItem.Voice != nil && strings.TrimSpace(mediaItem.Voice.Text) == "" {
		if p.transcriber != nil && mediaResult != nil && strings.TrimSpace(mediaResult.FilePath) != "" {
			audio, err := os.ReadFile(mediaResult.FilePath)
			if err != nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("read downloaded voice media: %w", err)
				}
			} else {
				text, err := p.transcriber.Transcribe(ctx, audio, filepath.Base(mediaResult.FilePath))
				if err != nil {
					if firstErr == nil {
						firstErr = fmt.Errorf("transcribe voice media: %w", err)
					}
				} else {
					mediaItem.Voice.Text = strings.TrimSpace(text)
				}
			}
		}
	}

	return strings.TrimSpace(BuildBody(items, mediaResult)), firstErr
}

// Downloader downloads and decrypts WeChat CDN media files.
type Downloader struct {
	client     *http.Client
	baseDir    string
	cdnBaseURL string
}

// NewDownloader creates a media downloader rooted at the provided base directory.
func NewDownloader(baseDir string) *Downloader {
	return &Downloader{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseDir:    baseDir,
		cdnBaseURL: defaultCDNBaseURL,
	}
}

// DownloadFromItem downloads and decrypts a media item when possible.
func (d *Downloader) DownloadFromItem(ctx context.Context, item *Item) (*InboundMediaResult, error) {
	if d == nil || item == nil {
		return nil, nil
	}

	switch item.Type {
	case ItemTypeImage:
		if item.Image == nil || item.Image.Media == nil || strings.TrimSpace(item.Image.Media.EncryptQueryParam) == "" {
			return nil, nil
		}
		aesKey := strings.TrimSpace(item.Image.AESKey)
		if aesKey != "" {
			aesKey = base64.StdEncoding.EncodeToString([]byte(strings.TrimSpace(aesKey)))
		} else if item.Image.Media != nil {
			aesKey = strings.TrimSpace(item.Image.Media.AESKey)
		}

		var data []byte
		var err error
		if aesKey != "" {
			data, err = d.downloadAndDecrypt(ctx, item.Image.Media.EncryptQueryParam, aesKey)
		} else {
			data, err = d.downloadPlain(ctx, item.Image.Media.EncryptQueryParam)
		}
		if err != nil {
			return nil, err
		}
		path, err := d.writeTempFile(data, ".jpg")
		if err != nil {
			return nil, err
		}
		return &InboundMediaResult{FilePath: path, MIMEType: "image/jpeg"}, nil
	case ItemTypeVoice:
		if item.Voice == nil {
			return nil, nil
		}
		if strings.TrimSpace(item.Voice.Text) != "" {
			return nil, nil
		}
		if item.Voice.Media == nil || strings.TrimSpace(item.Voice.Media.EncryptQueryParam) == "" || strings.TrimSpace(item.Voice.Media.AESKey) == "" {
			return nil, nil
		}
		data, err := d.downloadAndDecrypt(ctx, item.Voice.Media.EncryptQueryParam, item.Voice.Media.AESKey)
		if err != nil {
			return nil, err
		}
		path, err := d.writeTempFile(data, ".silk")
		if err != nil {
			return nil, err
		}
		return &InboundMediaResult{FilePath: path, MIMEType: "audio/silk"}, nil
	case ItemTypeFile:
		if item.File == nil || item.File.Media == nil || strings.TrimSpace(item.File.Media.EncryptQueryParam) == "" || strings.TrimSpace(item.File.Media.AESKey) == "" {
			return nil, nil
		}
		data, err := d.downloadAndDecrypt(ctx, item.File.Media.EncryptQueryParam, item.File.Media.AESKey)
		if err != nil {
			return nil, err
		}
		ext := filepath.Ext(strings.TrimSpace(item.File.FileName))
		if ext == "" {
			ext = ".bin"
		}
		path, err := d.writeTempFile(data, ext)
		if err != nil {
			return nil, err
		}
		return &InboundMediaResult{
			FilePath: path,
			MIMEType: GuessMIME(item.File.FileName),
			FileName: strings.TrimSpace(item.File.FileName),
		}, nil
	case ItemTypeVideo:
		if item.Video == nil || item.Video.Media == nil || strings.TrimSpace(item.Video.Media.EncryptQueryParam) == "" || strings.TrimSpace(item.Video.Media.AESKey) == "" {
			return nil, nil
		}
		data, err := d.downloadAndDecrypt(ctx, item.Video.Media.EncryptQueryParam, item.Video.Media.AESKey)
		if err != nil {
			return nil, err
		}
		path, err := d.writeTempFile(data, ".mp4")
		if err != nil {
			return nil, err
		}
		return &InboundMediaResult{FilePath: path, MIMEType: "video/mp4"}, nil
	default:
		return nil, nil
	}
}

// ExtractText returns the concatenated text content from the item list.
func ExtractText(items []Item) string {
	parts := make([]string, 0, len(items))
	for _, item := range items {
		if item.Type == ItemTypeText && item.Text != nil && strings.TrimSpace(item.Text.Text) != "" {
			parts = append(parts, strings.TrimSpace(item.Text.Text))
			continue
		}
		if item.Type == ItemTypeVoice && item.Voice != nil && strings.TrimSpace(item.Voice.Text) != "" {
			parts = append(parts, "语音转写: "+strings.TrimSpace(item.Voice.Text))
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

// FindMainMediaItem returns the first media-bearing item in the list.
func FindMainMediaItem(items []Item) *Item {
	for i := range items {
		switch items[i].Type {
		case ItemTypeImage:
			if items[i].Image != nil && items[i].Image.Media != nil && strings.TrimSpace(items[i].Image.Media.EncryptQueryParam) != "" {
				return &items[i]
			}
		case ItemTypeVoice:
			if items[i].Voice != nil && items[i].Voice.Media != nil && strings.TrimSpace(items[i].Voice.Media.EncryptQueryParam) != "" {
				return &items[i]
			}
		case ItemTypeFile:
			if items[i].File != nil && items[i].File.Media != nil && strings.TrimSpace(items[i].File.Media.EncryptQueryParam) != "" {
				return &items[i]
			}
		case ItemTypeVideo:
			if items[i].Video != nil && items[i].Video.Media != nil && strings.TrimSpace(items[i].Video.Media.EncryptQueryParam) != "" {
				return &items[i]
			}
		}
	}
	return nil
}

// IsMediaOnlyMessage reports whether the item list has no textual content.
func IsMediaOnlyMessage(items []Item) bool {
	return strings.TrimSpace(ExtractText(items)) == "" && FindMainMediaItem(items) != nil
}

// BuildBody builds a text payload suitable for the agent from text and downloaded media.
func BuildBody(items []Item, mediaResult *InboundMediaResult) string {
	parts := make([]string, 0, 4)
	if text := ExtractText(items); text != "" {
		parts = append(parts, text)
	}
	if mediaResult != nil {
		label := "附件"
		switch {
		case strings.HasPrefix(mediaResult.MIMEType, "image/"):
			label = "图片"
		case strings.HasPrefix(mediaResult.MIMEType, "audio/"):
			label = "音频"
		case strings.HasPrefix(mediaResult.MIMEType, "video/"):
			label = "视频"
		}
		segment := fmt.Sprintf("%s已下载到本地: %s", label, mediaResult.FilePath)
		if strings.TrimSpace(mediaResult.FileName) != "" {
			segment += fmt.Sprintf(" (filename: %s)", mediaResult.FileName)
		}
		parts = append(parts, segment)
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

// ParseAESKey decodes a WeChat AES key from base64(raw bytes) or base64(hex string).
func ParseAESKey(aesKeyBase64 string) ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(aesKeyBase64))
	if err != nil {
		return nil, fmt.Errorf("decode aes key: %w", err)
	}
	if len(decoded) == 16 {
		return decoded, nil
	}
	if len(decoded) == 32 {
		hexDecoded := make([]byte, 16)
		for i := 0; i < 16; i++ {
			var value byte
			for j := 0; j < 2; j++ {
				ch := decoded[i*2+j]
				value <<= 4
				switch {
				case ch >= '0' && ch <= '9':
					value |= ch - '0'
				case ch >= 'a' && ch <= 'f':
					value |= ch - 'a' + 10
				case ch >= 'A' && ch <= 'F':
					value |= ch - 'A' + 10
				default:
					return nil, fmt.Errorf("invalid aes hex digit %q", ch)
				}
			}
			hexDecoded[i] = value
		}
		return hexDecoded, nil
	}
	return nil, fmt.Errorf("unexpected aes key length: %d", len(decoded))
}

// DecryptAESECB decrypts AES-128-ECB ciphertext with PKCS7 padding.
func DecryptAESECB(ciphertext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("new cipher: %w", err)
	}
	if len(ciphertext)%block.BlockSize() != 0 {
		return nil, fmt.Errorf("ciphertext is not aligned to block size")
	}

	plain := make([]byte, len(ciphertext))
	for offset := 0; offset < len(ciphertext); offset += block.BlockSize() {
		block.Decrypt(plain[offset:offset+block.BlockSize()], ciphertext[offset:offset+block.BlockSize()])
	}
	return pkcs7Unpad(plain, block.BlockSize())
}

// GuessMIME guesses a content type from a filename.
func GuessMIME(filename string) string {
	switch strings.ToLower(filepath.Ext(strings.TrimSpace(filename))) {
	case ".pdf":
		return "application/pdf"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".mp3":
		return "audio/mpeg"
	case ".mp4":
		return "video/mp4"
	case ".txt":
		return "text/plain"
	case ".doc":
		return "application/msword"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".xls":
		return "application/vnd.ms-excel"
	case ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".zip":
		return "application/zip"
	default:
		return "application/octet-stream"
	}
}

func (d *Downloader) downloadAndDecrypt(ctx context.Context, encryptQueryParam, aesKey string) ([]byte, error) {
	encrypted, err := d.downloadPlain(ctx, encryptQueryParam)
	if err != nil {
		return nil, err
	}
	key, err := ParseAESKey(aesKey)
	if err != nil {
		return nil, err
	}
	return DecryptAESECB(encrypted, key)
}

func (d *Downloader) downloadPlain(ctx context.Context, encryptQueryParam string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.cdnDownloadURL(encryptQueryParam), nil)
	if err != nil {
		return nil, fmt.Errorf("create cdn request: %w", err)
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download cdn media: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("cdn status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxMediaBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read cdn media: %w", err)
	}
	if len(data) > maxMediaBytes {
		return nil, fmt.Errorf("media exceeds max size")
	}
	return data, nil
}

func (d *Downloader) writeTempFile(data []byte, ext string) (string, error) {
	if err := os.MkdirAll(d.baseDir, 0o755); err != nil {
		return "", fmt.Errorf("create media dir: %w", err)
	}
	filename := filepath.Join(d.baseDir, "wx-"+uuid.NewString()+ext)
	if err := os.WriteFile(filename, data, 0o644); err != nil {
		return "", fmt.Errorf("write temp media: %w", err)
	}
	return filename, nil
}

func (d *Downloader) cdnDownloadURL(encryptQueryParam string) string {
	return d.cdnBaseURL + "/download?encrypted_query_param=" + urlEncode(encryptQueryParam)
}

func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 || len(data)%blockSize != 0 {
		return nil, fmt.Errorf("invalid padded data size")
	}
	padding := int(data[len(data)-1])
	if padding == 0 || padding > blockSize || padding > len(data) {
		return nil, fmt.Errorf("invalid padding size")
	}
	for i := len(data) - padding; i < len(data); i++ {
		if int(data[i]) != padding {
			return nil, fmt.Errorf("invalid pkcs7 padding")
		}
	}
	return bytes.Clone(data[:len(data)-padding]), nil
}

func urlEncode(value string) string {
	replacer := strings.NewReplacer(
		"%", "%25",
		" ", "%20",
		"?", "%3F",
		"&", "%26",
		"=", "%3D",
		"+", "%2B",
		"/", "%2F",
	)
	return replacer.Replace(value)
}
