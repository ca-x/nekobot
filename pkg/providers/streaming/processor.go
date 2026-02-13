// Package streaming provides utilities for handling streaming responses from LLM APIs.
package streaming

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"
)

// StreamFormat represents the format of the streaming response.
type StreamFormat int

const (
	// FormatSSE represents Server-Sent Events format (used by OpenAI, Claude).
	FormatSSE StreamFormat = iota
	// FormatJSONLines represents JSON-lines format (used by Gemini).
	FormatJSONLines
)

// StreamProcessor handles the reading and processing of streaming responses.
type StreamProcessor struct {
	format        StreamFormat
	reader        io.Reader
	scanner       *bufio.Scanner
	ctx           context.Context
	timeout       time.Duration
	keepAliveFreq time.Duration
	buffer        *bytes.Buffer
	lastRead      time.Time
}

// NewStreamProcessor creates a new streaming processor.
func NewStreamProcessor(ctx context.Context, reader io.Reader, format StreamFormat) *StreamProcessor {
	sp := &StreamProcessor{
		format:        format,
		reader:        reader,
		scanner:       bufio.NewScanner(reader),
		ctx:           ctx,
		timeout:       30 * time.Second,  // Default timeout between chunks
		keepAliveFreq: 15 * time.Second,  // Default keep-alive frequency
		buffer:        &bytes.Buffer{},
		lastRead:      time.Now(),
	}

	// Set buffer size for scanner (default is 64KB, we increase it)
	const maxScanTokenSize = 512 * 1024 // 512KB
	sp.scanner.Buffer(make([]byte, maxScanTokenSize), maxScanTokenSize)

	return sp
}

// SetTimeout sets the timeout duration between chunks.
func (sp *StreamProcessor) SetTimeout(timeout time.Duration) {
	sp.timeout = timeout
}

// SetKeepAliveFrequency sets the keep-alive frequency.
func (sp *StreamProcessor) SetKeepAliveFrequency(freq time.Duration) {
	sp.keepAliveFreq = freq
}

// ReadChunk reads the next chunk from the stream.
// Returns the chunk data, a boolean indicating if the stream is done, and an error.
func (sp *StreamProcessor) ReadChunk() ([]byte, bool, error) {
	switch sp.format {
	case FormatSSE:
		return sp.readSSEChunk()
	case FormatJSONLines:
		return sp.readJSONLinesChunk()
	default:
		return nil, true, fmt.Errorf("unsupported stream format: %d", sp.format)
	}
}

// readSSEChunk reads a chunk in Server-Sent Events format.
// SSE format:
//   event: message_type
//   data: {json}
//   <blank line>
func (sp *StreamProcessor) readSSEChunk() ([]byte, bool, error) {
	sp.buffer.Reset()
	var data []byte

	for {
		// Check context cancellation
		select {
		case <-sp.ctx.Done():
			return nil, true, sp.ctx.Err()
		default:
		}

		// Check timeout
		if time.Since(sp.lastRead) > sp.timeout {
			return nil, true, fmt.Errorf("stream timeout: no data received for %v", sp.timeout)
		}

		// Read next line
		if !sp.scanner.Scan() {
			if err := sp.scanner.Err(); err != nil {
				return nil, true, fmt.Errorf("reading stream: %w", err)
			}
			// EOF reached
			if len(data) > 0 {
				return data, false, nil
			}
			return nil, true, nil
		}

		line := sp.scanner.Text()
		sp.lastRead = time.Now()

		// Empty line marks end of event
		if line == "" {
			if len(data) > 0 {
				return data, false, nil
			}
			continue
		}

		// Parse SSE line
		if strings.HasPrefix(line, "event:") {
			// Event type - we could use this for filtering specific events
			_ = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		} else if strings.HasPrefix(line, "data:") {
			dataLine := strings.TrimSpace(strings.TrimPrefix(line, "data:"))

			// Check for stream termination markers
			if dataLine == "[DONE]" {
				return nil, true, nil
			}

			data = []byte(dataLine)
		} else if strings.HasPrefix(line, ":") {
			// Comment line (keep-alive), ignore
			continue
		}
	}
}

// readJSONLinesChunk reads a chunk in JSON-lines format.
// Each line is a complete JSON object.
func (sp *StreamProcessor) readJSONLinesChunk() ([]byte, bool, error) {
	for {
		// Check context cancellation
		select {
		case <-sp.ctx.Done():
			return nil, true, sp.ctx.Err()
		default:
		}

		// Check timeout
		if time.Since(sp.lastRead) > sp.timeout {
			return nil, true, fmt.Errorf("stream timeout: no data received for %v", sp.timeout)
		}

		// Read next line
		if !sp.scanner.Scan() {
			if err := sp.scanner.Err(); err != nil {
				return nil, true, fmt.Errorf("reading stream: %w", err)
			}
			// EOF reached
			return nil, true, nil
		}

		line := sp.scanner.Bytes()
		sp.lastRead = time.Now()

		// Skip empty lines
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}

		return line, false, nil
	}
}

// ProcessStream reads all chunks from the stream and calls the handler for each.
func (sp *StreamProcessor) ProcessStream(handler func(chunk []byte) error) error {
	for {
		chunk, done, err := sp.ReadChunk()
		if err != nil {
			return err
		}

		if done {
			break
		}

		if len(chunk) > 0 {
			if err := handler(chunk); err != nil {
				return fmt.Errorf("handling chunk: %w", err)
			}
		}
	}

	return nil
}

// StreamWriter is an interface for writing streaming responses to a destination.
type StreamWriter interface {
	// Write writes a chunk to the destination.
	Write(chunk []byte) error

	// WriteSSE writes a chunk in SSE format.
	WriteSSE(eventType string, data []byte) error

	// Flush flushes any buffered data.
	Flush() error

	// Close closes the stream writer.
	Close() error
}

// BufferedStreamWriter provides a buffered writer for streaming responses.
type BufferedStreamWriter struct {
	writer io.Writer
	buffer *bytes.Buffer
	format StreamFormat
}

// NewBufferedStreamWriter creates a new buffered stream writer.
func NewBufferedStreamWriter(writer io.Writer, format StreamFormat) *BufferedStreamWriter {
	return &BufferedStreamWriter{
		writer: writer,
		buffer: &bytes.Buffer{},
		format: format,
	}
}

// Write writes a raw chunk to the buffer.
func (w *BufferedStreamWriter) Write(chunk []byte) error {
	_, err := w.buffer.Write(chunk)
	return err
}

// WriteSSE writes a chunk in SSE format.
func (w *BufferedStreamWriter) WriteSSE(eventType string, data []byte) error {
	if eventType != "" {
		if _, err := fmt.Fprintf(w.buffer, "event: %s\n", eventType); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintf(w.buffer, "data: %s\n\n", data); err != nil {
		return err
	}

	return nil
}

// Flush flushes the buffer to the underlying writer.
func (w *BufferedStreamWriter) Flush() error {
	if w.buffer.Len() == 0 {
		return nil
	}

	if _, err := w.writer.Write(w.buffer.Bytes()); err != nil {
		return err
	}

	w.buffer.Reset()

	// If the writer supports flushing, flush it
	if flusher, ok := w.writer.(interface{ Flush() error }); ok {
		return flusher.Flush()
	}

	return nil
}

// Close flushes any remaining data and closes the writer.
func (w *BufferedStreamWriter) Close() error {
	if err := w.Flush(); err != nil {
		return err
	}

	if closer, ok := w.writer.(io.Closer); ok {
		return closer.Close()
	}

	return nil
}
