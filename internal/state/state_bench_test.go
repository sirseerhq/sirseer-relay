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

package state

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// BenchmarkSaveState benchmarks state saving operations
func BenchmarkSaveState(b *testing.B) {
	benchmarks := []struct {
		name         string
		lastPRNumber int
		fetchedPRs   int
		lastCursor   string
	}{
		{"Small_100PRs", 100, 100, "cursor_100"},
		{"Medium_1000PRs", 1000, 1000, "cursor_1000"},
		{"Large_10000PRs", 10000, 10000, "cursor_10000"},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			tempDir := b.TempDir()
			stateFile := filepath.Join(tempDir, "state.json")

			state := &FetchState{
				Repository:    "org/repo",
				LastPRNumber:  bm.lastPRNumber,
				LastFetchID:   "fetch_" + bm.lastCursor,
				LastPRDate:    time.Now().Add(-24 * time.Hour),
				LastFetchTime: time.Now(),
				TotalFetched:  bm.fetchedPRs,
				Version:       CurrentVersion,
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				if err := SaveState(state, stateFile); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkLoadState benchmarks state loading operations
func BenchmarkLoadState(b *testing.B) {
	tempDir := b.TempDir()
	stateFile := filepath.Join(tempDir, "state.json")

	// Create a test state file
	state := &FetchState{
		Repository:    "org/repo",
		LastPRNumber:  5000,
		LastFetchID:   "fetch_cursor_5000",
		LastPRDate:    time.Now().Add(-24 * time.Hour),
		LastFetchTime: time.Now(),
		TotalFetched:  5000,
		Version:       CurrentVersion,
	}

	if err := SaveState(state, stateFile); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		if _, err := LoadState(stateFile); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkStateChecksum benchmarks checksum calculation
func BenchmarkStateChecksum(b *testing.B) {
	benchmarks := []struct {
		name     string
		dataSize int
	}{
		{"Small_1KB", 1024},
		{"Medium_10KB", 10 * 1024},
		{"Large_100KB", 100 * 1024},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			// Generate test data
			data := make([]byte, bm.dataSize)
			for i := range data {
				data[i] = byte(i % 256)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				// calculateChecksum is an internal function, simulate its behavior
				hash := sha256.Sum256(data)
				_ = hex.EncodeToString(hash[:])
			}
		})
	}
}

// BenchmarkConcurrentStateSaves benchmarks concurrent state saves
func BenchmarkConcurrentStateSaves(b *testing.B) {
	tempDir := b.TempDir()

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			stateFile := filepath.Join(tempDir, fmt.Sprintf("state_%d.json", i%10))
			state := &FetchState{
				Repository:    "org/repo",
				LastPRNumber:  i,
				LastFetchID:   fmt.Sprintf("fetch_%d", i),
				LastPRDate:    time.Now().Add(-24 * time.Hour),
				LastFetchTime: time.Now(),
				TotalFetched:  i,
				Version:       CurrentVersion,
			}

			if err := SaveState(state, stateFile); err != nil {
				b.Fatal(err)
			}
			i++
		}
	})
}

// BenchmarkStateFileSize benchmarks the impact of state file size
func BenchmarkStateFileSize(b *testing.B) {
	benchmarks := []struct {
		name      string
		stateSize int
	}{
		{"Minimal", 1},
		{"Normal", 1000},
		{"Large", 10000},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			tempDir := b.TempDir()
			stateFile := filepath.Join(tempDir, "state.json")

			state := &FetchState{
				Repository:    "org/repo-with-very-long-name-to-increase-size",
				LastPRNumber:  bm.stateSize,
				LastFetchID:   "very_long_fetch_id_that_simulates_real_ids_" + string(make([]byte, 100)),
				LastPRDate:    time.Now().Add(-24 * time.Hour),
				LastFetchTime: time.Now(),
				TotalFetched:  bm.stateSize,
				Version:       CurrentVersion,
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				if err := SaveState(state, stateFile); err != nil {
					b.Fatal(err)
				}
				os.Remove(stateFile) // Clean up for next iteration
			}
		})
	}
}

// BenchmarkAtomicWrite benchmarks the atomic write pattern
func BenchmarkAtomicWrite(b *testing.B) {
	tempDir := b.TempDir()
	targetFile := filepath.Join(tempDir, "target.json")

	data := []byte(`{"test": "data", "value": 12345}`)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		tempFile := targetFile + ".tmp"

		// Write to temp file
		if err := os.WriteFile(tempFile, data, 0o644); err != nil {
			b.Fatal(err)
		}

		// Atomic rename
		if err := os.Rename(tempFile, targetFile); err != nil {
			b.Fatal(err)
		}
	}
}
