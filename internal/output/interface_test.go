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
	"bytes"
	"testing"
)

// Compile-time check that Writer implements OutputWriter
var _ OutputWriter = (*Writer)(nil)

func TestWriterImplementsInterface(t *testing.T) {
	// This test ensures our NDJSON Writer satisfies the OutputWriter interface
	buf := &bytes.Buffer{}
	writer := NewWriter(buf)
	
	// Test that we can use it as an OutputWriter
	var w OutputWriter = writer
	
	// Test Write method
	err := w.Write(map[string]string{"test": "data"})
	if err != nil {
		t.Errorf("Write() error = %v", err)
	}
	
	// Test Close method
	err = w.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}
	
	// Verify data was written
	if buf.Len() == 0 {
		t.Error("Expected data to be written to buffer")
	}
}