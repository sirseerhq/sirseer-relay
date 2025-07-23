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

// OutputWriter defines the interface for writing pull request data.
// This abstraction allows for different output formats (NDJSON, CSV, etc.)
// to be implemented in the future without changing the core logic.
type OutputWriter interface {
	// Write writes a single record to the output.
	// The record should be immediately flushed to avoid memory accumulation.
	Write(record interface{}) error

	// Close closes the underlying writer and releases any resources.
	// This should be called when all writing is complete.
	Close() error
}
