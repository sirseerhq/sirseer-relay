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
	"io"
	"testing"
	"time"
)

// samplePR represents a typical pull request structure for benchmarking
type samplePR struct {
	Number     int       `json:"number"`
	Title      string    `json:"title"`
	Body       string    `json:"body"`
	State      string    `json:"state"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	ClosedAt   time.Time `json:"closed_at,omitempty"`
	MergedAt   time.Time `json:"merged_at,omitempty"`
	User       string    `json:"user"`
	HeadBranch string    `json:"head_branch"`
	BaseBranch string    `json:"base_branch"`
}

// createSamplePR creates a realistic PR structure for benchmarking
func createSamplePR(num int) samplePR {
	now := time.Now()
	return samplePR{
		Number:     num,
		Title:      "feat: add support for enhanced performance monitoring and optimization",
		Body:       "This PR implements comprehensive performance monitoring capabilities including real-time metrics collection, automated alerting based on configurable thresholds, and detailed performance reports. The implementation focuses on minimal overhead while providing maximum visibility into system behavior.",
		State:      "closed",
		CreatedAt:  now.Add(-72 * time.Hour),
		UpdatedAt:  now.Add(-2 * time.Hour),
		ClosedAt:   now.Add(-1 * time.Hour),
		MergedAt:   now.Add(-1 * time.Hour),
		User:       "developer123",
		HeadBranch: "feature/performance-monitoring",
		BaseBranch: "main",
	}
}

// BenchmarkWriter_Write benchmarks writing single records
func BenchmarkWriter_Write(b *testing.B) {
	w := NewWriter(io.Discard)
	pr := createSamplePR(1)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		if err := w.Write(pr); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkWriter_WriteLarge benchmarks writing many records sequentially
func BenchmarkWriter_WriteLarge(b *testing.B) {
	benchmarks := []struct {
		name  string
		count int
	}{
		{"100PRs", 100},
		{"1000PRs", 1000},
		{"10000PRs", 10000},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				b.StopTimer()
				w := NewWriter(io.Discard)
				b.StartTimer()

				for j := 0; j < bm.count; j++ {
					pr := createSamplePR(j)
					if err := w.Write(pr); err != nil {
						b.Fatal(err)
					}
				}
			}
		})
	}
}

// BenchmarkWriter_Concurrent benchmarks concurrent writes
func BenchmarkWriter_Concurrent(b *testing.B) {
	w := NewWriter(io.Discard)
	pr := createSamplePR(1)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if err := w.Write(pr); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkFileWriter_Write benchmarks file-based writing with buffering
func BenchmarkFileWriter_Write(b *testing.B) {
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		tempFile := b.TempDir() + "/bench.ndjson"
		w, err := NewFileWriter(tempFile)
		if err != nil {
			b.Fatal(err)
		}
		b.StartTimer()

		// Write 1000 PRs to simulate realistic usage
		for j := 0; j < 1000; j++ {
			pr := createSamplePR(j)
			if err := w.Write(pr); err != nil {
				b.Fatal(err)
			}
		}

		b.StopTimer()
		w.Close()
		b.StartTimer()
	}
}

// BenchmarkWriter_MemoryUsage tracks memory allocation patterns
func BenchmarkWriter_MemoryUsage(b *testing.B) {
	benchmarks := []struct {
		name      string
		prCount   int
		batchSize int
	}{
		{"Small_NoBatch", 100, 1},
		{"Medium_NoBatch", 1000, 1},
		{"Large_NoBatch", 10000, 1},
		{"Large_Batch10", 10000, 10},
		{"Large_Batch100", 10000, 100},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				w := NewWriter(io.Discard)

				for j := 0; j < bm.prCount; j += bm.batchSize {
					// Simulate batch processing
					for k := 0; k < bm.batchSize && j+k < bm.prCount; k++ {
						pr := createSamplePR(j + k)
						if err := w.Write(pr); err != nil {
							b.Fatal(err)
						}
					}
				}
			}
		})
	}
}
