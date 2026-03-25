package types

import (
	"encoding/base64"
	"encoding/hex"
)

// GetUploadURLRequest is the body for getuploadurl API.
type GetUploadURLRequest struct {
	FileKey     string   `json:"filekey"`
	MediaType   int      `json:"media_type"`
	ToUserID    string   `json:"to_user_id"`
	RawSize     int      `json:"rawsize"`
	RawFileMD5  string   `json:"rawfilemd5"`
	FileSize    int      `json:"filesize"`
	NoNeedThumb bool     `json:"no_need_thumb"`
	AESKey      string   `json:"aeskey"`
	BaseInfo    BaseInfo `json:"base_info"`
}

// GetUploadURLResponse is the response from getuploadurl API.
type GetUploadURLResponse struct {
	Ret         int    `json:"ret"`
	ErrMsg      string `json:"errmsg,omitempty"`
	UploadParam string `json:"upload_param,omitempty"`
}

// UploadedFileInfo contains the result of a successful CDN upload.
type UploadedFileInfo struct {
	FileKey                     string
	DownloadEncryptedQueryParam string
	AESKey                      []byte
	FileSize                    int
	FileSizeCiphertext          int
}

// AESKeyHex returns the AES key as a hex-encoded string.
func (u *UploadedFileInfo) AESKeyHex() string {
	return hex.EncodeToString(u.AESKey)
}

// AESKeyBase64 returns the AES key encoded as base64(hex_string_bytes).
func (u *UploadedFileInfo) AESKeyBase64() string {
	hexStr := hex.EncodeToString(u.AESKey)
	return base64.StdEncoding.EncodeToString([]byte(hexStr))
}
