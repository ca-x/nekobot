package voice

import "encoding/binary"

// EncodeWAV wraps raw PCM data in a standard RIFF/WAVE header.
func EncodeWAV(pcmData []byte) []byte {
	const (
		sampleRate    = 24000
		numChannels   = 1
		bitsPerSample = 16
		byteRate      = sampleRate * numChannels * bitsPerSample / 8
		blockAlign    = numChannels * bitsPerSample / 8
		headerSize    = 44
	)

	dataSize := len(pcmData)
	fileSize := headerSize + dataSize - 8

	buf := make([]byte, headerSize+dataSize)

	copy(buf[0:4], "RIFF")
	binary.LittleEndian.PutUint32(buf[4:8], uint32(fileSize))
	copy(buf[8:12], "WAVE")

	copy(buf[12:16], "fmt ")
	binary.LittleEndian.PutUint32(buf[16:20], 16)
	binary.LittleEndian.PutUint16(buf[20:22], 1)
	binary.LittleEndian.PutUint16(buf[22:24], numChannels)
	binary.LittleEndian.PutUint32(buf[24:28], sampleRate)
	binary.LittleEndian.PutUint32(buf[28:32], byteRate)
	binary.LittleEndian.PutUint16(buf[32:34], blockAlign)
	binary.LittleEndian.PutUint16(buf[34:36], bitsPerSample)

	copy(buf[36:40], "data")
	binary.LittleEndian.PutUint32(buf[40:44], uint32(dataSize))
	copy(buf[headerSize:], pcmData)

	return buf
}
