package types

const (
	// MessageTypeNone identifies an unknown sender role.
	MessageTypeNone = 0
	// MessageTypeUser identifies a user-originated message.
	MessageTypeUser = 1
	// MessageTypeBot identifies a bot-originated message.
	MessageTypeBot = 2

	// MessageStateNew identifies a newly created message.
	MessageStateNew = 0
	// MessageStateGenerating identifies an in-progress bot message.
	MessageStateGenerating = 1
	// MessageStateFinish identifies a completed message.
	MessageStateFinish = 2

	// ItemTypeNone identifies an empty item.
	ItemTypeNone = 0
	// ItemTypeText identifies a text item.
	ItemTypeText = 1
	// ItemTypeImage identifies an image item.
	ItemTypeImage = 2
	// ItemTypeVoice identifies a voice item.
	ItemTypeVoice = 3
	// ItemTypeFile identifies a file item.
	ItemTypeFile = 4
	// ItemTypeVideo identifies a video item.
	ItemTypeVideo = 5

	// TypingStatusTyping keeps the typing indicator active.
	TypingStatusTyping = 1
	// TypingStatusCancel clears the typing indicator.
	TypingStatusCancel = 2

	// UploadMediaTypeImage identifies an image upload.
	UploadMediaTypeImage = 1
	// UploadMediaTypeVideo identifies a video upload.
	UploadMediaTypeVideo = 2
	// UploadMediaTypeFile identifies a file upload.
	UploadMediaTypeFile = 3
	// UploadMediaTypeVoice identifies a voice upload.
	UploadMediaTypeVoice = 4

	// EncryptTypeAES128ECB identifies AES-128-ECB encryption.
	EncryptTypeAES128ECB = 1

	// VoiceEncodeTypePCM identifies PCM voice payloads.
	VoiceEncodeTypePCM = 1
	// VoiceEncodeTypeADPCM identifies ADPCM voice payloads.
	VoiceEncodeTypeADPCM = 2
	// VoiceEncodeTypeFeature identifies feature voice payloads.
	VoiceEncodeTypeFeature = 3
	// VoiceEncodeTypeSpeex identifies Speex voice payloads.
	VoiceEncodeTypeSpeex = 4
	// VoiceEncodeTypeAMR identifies AMR voice payloads.
	VoiceEncodeTypeAMR = 5
	// VoiceEncodeTypeSILK identifies SILK voice payloads.
	VoiceEncodeTypeSILK = 6
	// VoiceEncodeTypeMP3 identifies MP3 voice payloads.
	VoiceEncodeTypeMP3 = 7
	// VoiceEncodeTypeOggSpeex identifies Ogg Speex voice payloads.
	VoiceEncodeTypeOggSpeex = 8

	// QRStatusWait identifies an unscanned QR code.
	QRStatusWait = "wait"
	// QRStatusScanned identifies a scanned QR code awaiting confirmation.
	QRStatusScanned = "scaned"
	// QRStatusConfirmed identifies a confirmed QR code login.
	QRStatusConfirmed = "confirmed"
	// QRStatusExpired identifies an expired QR code.
	QRStatusExpired = "expired"
)
