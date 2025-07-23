package output

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRecord is a test structure for NDJSON writing
type TestRecord struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

func TestNewWriter(t *testing.T) {
	var buf bytes.Buffer
	writer := NewWriter(&buf)

	if writer == nil {
		t.Fatal("NewWriter returned nil")
	}
	if writer.output != &buf {
		t.Error("Writer output doesn't match provided buffer")
	}
	if writer.encoder == nil {
		t.Error("Writer encoder is nil")
	}
	if writer.count != 0 {
		t.Errorf("Initial count should be 0, got %d", writer.count)
	}
}

func TestWriter_Write(t *testing.T) {
	tests := []struct {
		name    string
		records []TestRecord
		want    []string
	}{
		{
			name: "single record",
			records: []TestRecord{
				{ID: 1, Name: "Test One", Active: true},
			},
			want: []string{
				`{"id":1,"name":"Test One","active":true}`,
			},
		},
		{
			name: "multiple records",
			records: []TestRecord{
				{ID: 1, Name: "Test One", Active: true},
				{ID: 2, Name: "Test Two", Active: false},
				{ID: 3, Name: "Test Three", Active: true},
			},
			want: []string{
				`{"id":1,"name":"Test One","active":true}`,
				`{"id":2,"name":"Test Two","active":false}`,
				`{"id":3,"name":"Test Three","active":true}`,
			},
		},
		{
			name:    "empty records",
			records: []TestRecord{},
			want:    []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			writer := NewWriter(&buf)

			// Write all records
			for _, record := range tt.records {
				if err := writer.Write(record); err != nil {
					t.Fatalf("Write failed: %v", err)
				}
			}

			// Check count
			if writer.Count() != len(tt.records) {
				t.Errorf("Count mismatch: got %d, want %d", writer.Count(), len(tt.records))
			}

			// Check output
			output := strings.TrimSpace(buf.String())
			if output == "" && len(tt.want) == 0 {
				return // Both empty, test passes
			}

			lines := strings.Split(output, "\n")
			if len(lines) != len(tt.want) {
				t.Fatalf("Line count mismatch: got %d, want %d", len(lines), len(tt.want))
			}

			for i, line := range lines {
				// Parse both actual and expected as JSON to compare
				var actual, expected map[string]interface{}
				if err := json.Unmarshal([]byte(line), &actual); err != nil {
					t.Fatalf("Failed to parse actual JSON at line %d: %v", i, err)
				}
				if err := json.Unmarshal([]byte(tt.want[i]), &expected); err != nil {
					t.Fatalf("Failed to parse expected JSON at line %d: %v", i, err)
				}

				// Compare JSON objects
				if !jsonEqual(actual, expected) {
					t.Errorf("Line %d mismatch:\ngot:  %s\nwant: %s", i, line, tt.want[i])
				}
			}
		})
	}
}

func TestWriter_Concurrent(t *testing.T) {
	var buf bytes.Buffer
	writer := NewWriter(&buf)

	// Number of goroutines and records per goroutine
	numGoroutines := 10
	recordsPerGoroutine := 100
	totalRecords := numGoroutines * recordsPerGoroutine

	// Channel to collect errors
	errCh := make(chan error, numGoroutines)

	// Launch concurrent writers
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			for j := 0; j < recordsPerGoroutine; j++ {
				record := TestRecord{
					ID:     goroutineID*recordsPerGoroutine + j,
					Name:   "Concurrent Test",
					Active: true,
				}
				if err := writer.Write(record); err != nil {
					errCh <- err
					return
				}
			}
			errCh <- nil
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < numGoroutines; i++ {
		if err := <-errCh; err != nil {
			t.Fatalf("Concurrent write failed: %v", err)
		}
	}

	// Check count
	if writer.Count() != totalRecords {
		t.Errorf("Count mismatch: got %d, want %d", writer.Count(), totalRecords)
	}

	// Check that all lines are valid JSON
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != totalRecords {
		t.Errorf("Line count mismatch: got %d, want %d", len(lines), totalRecords)
	}

	for i, line := range lines {
		var record TestRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Errorf("Invalid JSON at line %d: %v", i, err)
		}
	}
}

func TestNewFileWriter(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test.ndjson")

	// Create file writer
	writer, err := NewFileWriter(filename)
	if err != nil {
		t.Fatalf("NewFileWriter failed: %v", err)
	}
	defer writer.Close()

	// Write test data
	testRecords := []TestRecord{
		{ID: 1, Name: "File Test One", Active: true},
		{ID: 2, Name: "File Test Two", Active: false},
	}

	for _, record := range testRecords {
		if err := writer.Write(record); err != nil {
			t.Fatalf("Write failed: %v", err)
		}
	}

	// Close the writer
	if err := writer.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Read and verify file contents
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != len(testRecords) {
		t.Fatalf("Line count mismatch: got %d, want %d", len(lines), len(testRecords))
	}

	for i, line := range lines {
		var record TestRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("Failed to parse JSON at line %d: %v", i, err)
		}
		if record.ID != testRecords[i].ID {
			t.Errorf("ID mismatch at line %d: got %d, want %d", i, record.ID, testRecords[i].ID)
		}
	}
}

func TestNewFileWriter_Error(t *testing.T) {
	// Try to create file in non-existent directory
	_, err := NewFileWriter("/non/existent/path/test.ndjson")
	if err == nil {
		t.Error("Expected error for non-existent directory, got nil")
	}
}

func TestWriter_WriteError(t *testing.T) {
	var buf bytes.Buffer
	writer := NewWriter(&buf)

	// Create a channel that can't be marshaled to JSON
	badData := make(chan int)

	err := writer.Write(badData)
	if err == nil {
		t.Error("Expected error when writing non-marshalable data")
	}
}

// jsonEqual compares two JSON objects for equality
func jsonEqual(a, b map[string]interface{}) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || !deepEqual(v, bv) {
			return false
		}
	}
	return true
}

// deepEqual performs deep equality check for JSON values
func deepEqual(a, b interface{}) bool {
	switch av := a.(type) {
	case map[string]interface{}:
		bv, ok := b.(map[string]interface{})
		if !ok {
			return false
		}
		return jsonEqual(av, bv)
	case []interface{}:
		bv, ok := b.([]interface{})
		if !ok || len(av) != len(bv) {
			return false
		}
		for i := range av {
			if !deepEqual(av[i], bv[i]) {
				return false
			}
		}
		return true
	default:
		return a == b
	}
}