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

// Package state provides atomic state persistence for tracking fetch progress.
//
// This package implements bulletproof state management to enable incremental
// fetching of pull requests. It uses atomic file operations, SHA256 checksums
// for integrity validation, and clear schema versioning for forward compatibility.
//
// State files are stored in a standard location (~/.sirseer/state/) and use
// a JSON format for human readability and debugging. Every state write is
// atomic, using a write-to-temp-and-rename pattern to prevent corruption
// during crashes or power loss.
//
// Example usage:
//
//	state := &FetchState{
//	    Repository:   "kubernetes/kubernetes",
//	    LastPRNumber: 12345,
//	    LastPRDate:   time.Now(),
//	}
//	err := SaveState(state, GetStateFilePath("kubernetes/kubernetes"))
package state
