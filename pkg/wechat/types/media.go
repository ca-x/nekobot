package types

// ImageItem holds image content.
type ImageItem struct {
	URL     string    `json:"url,omitempty"`
	Media   *CDNMedia `json:"media,omitempty"`
	MidSize int       `json:"mid_size,omitempty"`
}

// VideoItem holds video content.
type VideoItem struct {
	Media     *CDNMedia `json:"media,omitempty"`
	VideoSize int       `json:"video_size,omitempty"`
}

// FileItem holds file attachment content.
type FileItem struct {
	Media    *CDNMedia `json:"media,omitempty"`
	FileName string    `json:"file_name,omitempty"`
	Len      string    `json:"len,omitempty"`
}

// VoiceItem holds voice content with full metadata.
type VoiceItem struct {
	Media         *CDNMedia `json:"media,omitempty"`
	EncodeType    int       `json:"encode_type,omitempty"`
	BitsPerSample int       `json:"bits_per_sample,omitempty"`
	SampleRate    int       `json:"sample_rate,omitempty"`
	Playtime      int       `json:"playtime,omitempty"`
	Text          string    `json:"text,omitempty"`
}
