package voice

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Decoder converts voice data to WAV format.
type Decoder interface {
	DecodeToWAV(data []byte) ([]byte, error)
}

// ExecSILKDecoder decodes SILK audio by invoking an external decoder binary.
type ExecSILKDecoder struct {
	BinaryPath string
}

// NewExecSILKDecoder creates a new ExecSILKDecoder with the given binary path.
func NewExecSILKDecoder(binaryPath string) *ExecSILKDecoder {
	return &ExecSILKDecoder{BinaryPath: binaryPath}
}

// DecodeToWAV decodes SILK data to WAV format.
func (d *ExecSILKDecoder) DecodeToWAV(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty SILK data")
	}

	if data[0] == 0x02 {
		data = data[1:]
	}

	tmpDir, err := os.MkdirTemp("", "silk-decode-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	tmpInput := filepath.Join(tmpDir, "input.silk")
	tmpOutput := filepath.Join(tmpDir, "output.pcm")

	if err := os.WriteFile(tmpInput, data, 0o600); err != nil {
		return nil, fmt.Errorf("write temp input: %w", err)
	}

	cmd := exec.Command(d.BinaryPath, tmpInput, tmpOutput)
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("silk decoder: %w (output: %s)", err, string(output))
	}

	pcmData, err := os.ReadFile(tmpOutput)
	if err != nil {
		return nil, fmt.Errorf("read PCM output: %w", err)
	}

	return EncodeWAV(pcmData), nil
}

// NoOpDecoder returns data as-is without any decoding.
type NoOpDecoder struct{}

// DecodeToWAV returns the input data unchanged.
func (d *NoOpDecoder) DecodeToWAV(data []byte) ([]byte, error) {
	return data, nil
}
