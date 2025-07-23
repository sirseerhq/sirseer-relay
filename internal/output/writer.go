package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
)

// Writer handles streaming NDJSON output to a file or io.Writer.
// It ensures memory-efficient writing without accumulating data.
type Writer struct {
	mu       sync.Mutex
	output   io.Writer
	encoder  *json.Encoder
	count    int
	closeFunc func() error
}

// NewWriter creates a new NDJSON writer that writes to the specified output.
func NewWriter(w io.Writer) *Writer {
	return &Writer{
		output:  w,
		encoder: json.NewEncoder(w),
	}
}

// NewFileWriter creates a new NDJSON writer that writes to a file.
// The caller must call Close() when done to ensure the file is properly closed.
func NewFileWriter(filename string) (*Writer, error) {
	file, err := os.Create(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create output file: %w", err)
	}

	return &Writer{
		output:    file,
		encoder:   json.NewEncoder(file),
		closeFunc: file.Close,
	}, nil
}

// Write writes a single record as NDJSON.
// Each record is immediately flushed to the output.
func (w *Writer) Write(record interface{}) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.encoder.Encode(record); err != nil {
		return fmt.Errorf("failed to write record: %w", err)
	}

	w.count++
	return nil
}

// Count returns the number of records written.
func (w *Writer) Count() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.count
}

// Close closes the underlying writer if it's a file.
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closeFunc != nil {
		return w.closeFunc()
	}
	return nil
}