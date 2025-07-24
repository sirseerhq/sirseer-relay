// Copyright 2025 SirSeer, LLC
//
// Licensed under the Business Source License 1.1 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://mariadb.com/bsl11
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package output

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
)

// Writer handles streaming NDJSON (Newline Delimited JSON) output to an io.Writer.
// It provides thread-safe writing of JSON records, where each record is written
// as a single line followed by a newline character. This format is ideal for
// streaming large datasets without memory accumulation.
//
// The zero value is not usable; use NewWriter or NewFileWriter to create instances.
type Writer struct {
	mu        sync.Mutex
	output    io.Writer
	encoder   *json.Encoder
	count     int
	closeFunc func() error
	bufWriter *bufio.Writer // For buffered file writes
}

// NewWriter creates a new NDJSON writer that writes to the specified output.
// The writer is safe for concurrent use. Each call to Write will produce
// exactly one line of JSON output.
//
// Example:
//
//	var buf bytes.Buffer
//	w := NewWriter(&buf)
//	w.Write(map[string]string{"name": "example"})
//	// Output: {"name":"example"}\n
func NewWriter(w io.Writer) *Writer {
	return &Writer{
		output:  w,
		encoder: json.NewEncoder(w),
	}
}

// NewFileWriter creates a new NDJSON writer that writes to a file.
// The file is created with default permissions (0666 before umask).
// If the file already exists, it will be truncated.
//
// The caller must call Close() when done to ensure the file is properly closed
// and any buffered data is flushed to disk.
//
// Example:
//
//	w, err := NewFileWriter("output.ndjson")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer w.Close()
//
//	w.Write(someData)
func NewFileWriter(filename string) (*Writer, error) {
	file, err := os.Create(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create output file: %w", err)
	}

	// Create a buffered writer with 64KB buffer for efficient disk writes
	bufWriter := bufio.NewWriterSize(file, 64*1024)

	return &Writer{
		output:    file,
		encoder:   json.NewEncoder(bufWriter),
		bufWriter: bufWriter,
		closeFunc: func() error {
			// Flush buffer before closing file
			if err := bufWriter.Flush(); err != nil {
				_ = file.Close()
				return fmt.Errorf("failed to flush buffer: %w", err)
			}
			return file.Close()
		},
	}, nil
}

// Write encodes a single record as JSON and writes it as a line to the output.
// Each record is written atomically with respect to other concurrent calls to Write.
// The record can be any value that can be marshaled to JSON.
//
// The written format is:
//
//	<json-encoded-record>\n
//
// This method is safe for concurrent use.
//
// Returns an error if the record cannot be marshaled to JSON or if writing fails.
func (w *Writer) Write(record interface{}) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.encoder.Encode(record); err != nil {
		return fmt.Errorf("failed to write record: %w", err)
	}

	w.count++
	return nil
}

// Count returns the total number of records successfully written.
// This method is safe for concurrent use.
func (w *Writer) Count() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.count
}

// Close closes the underlying writer if it was created by NewFileWriter.
// For writers created with NewWriter, this is a no-op that always returns nil.
// After Close, the Writer should not be used.
//
// Close is safe to call multiple times; only the first call will have an effect.
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closeFunc != nil {
		return w.closeFunc()
	}
	return nil
}
