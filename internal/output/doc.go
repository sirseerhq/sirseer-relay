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

// Package output provides utilities for writing data in NDJSON (Newline Delimited JSON) format.
// NDJSON is a convenient format for streaming large datasets where each line contains
// a valid JSON object. This format is particularly useful for log files, data exports,
// and streaming APIs.
//
// The primary type is Writer, which provides thread-safe writing of JSON records
// to an io.Writer or file. The package is designed to handle large volumes of data
// efficiently without accumulating records in memory.
//
// Example usage:
//
//	// Write to a file
//	w, err := output.NewFileWriter("data.ndjson")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer w.Close()
//
//	// Write records
//	for _, record := range records {
//	    if err := w.Write(record); err != nil {
//	        log.Printf("Failed to write record: %v", err)
//	    }
//	}
//
//	fmt.Printf("Wrote %d records\n", w.Count())
package output
