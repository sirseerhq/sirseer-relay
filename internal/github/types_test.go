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

package github

import (
	"encoding/json"
	"testing"
	"time"
)

func TestPullRequestJSONSerialization(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	closedAt := now.Add(24 * time.Hour)
	mergedAt := now.Add(25 * time.Hour)

	pr := PullRequest{
		Number:    123,
		Title:     "Fix bug in parser",
		State:     "closed",
		CreatedAt: now,
		UpdatedAt: now.Add(time.Hour),
		ClosedAt:  &closedAt,
		MergedAt:  &mergedAt,
		Author: Author{
			Login: "johndoe",
		},
	}

	// Test marshaling
	data, err := json.Marshal(pr)
	if err != nil {
		t.Fatalf("Failed to marshal PullRequest: %v", err)
	}

	// Test unmarshaling
	var decoded PullRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal PullRequest: %v", err)
	}

	// Verify fields
	if decoded.Number != pr.Number {
		t.Errorf("Number = %d, want %d", decoded.Number, pr.Number)
	}
	if decoded.Title != pr.Title {
		t.Errorf("Title = %q, want %q", decoded.Title, pr.Title)
	}
	if decoded.State != pr.State {
		t.Errorf("State = %q, want %q", decoded.State, pr.State)
	}
	if !decoded.CreatedAt.Equal(pr.CreatedAt) {
		t.Errorf("CreatedAt = %v, want %v", decoded.CreatedAt, pr.CreatedAt)
	}
	if decoded.Author.Login != pr.Author.Login {
		t.Errorf("Author.Login = %q, want %q", decoded.Author.Login, pr.Author.Login)
	}
}

func TestPullRequestOptionalFields(t *testing.T) {
	// Test PR with no closed/merged times
	pr := PullRequest{
		Number:    456,
		Title:     "Add new feature",
		State:     "open",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
		Author: Author{
			Login: "janedoe",
		},
	}

	data, err := json.Marshal(pr)
	if err != nil {
		t.Fatalf("Failed to marshal PullRequest: %v", err)
	}

	// Verify omitempty works - should not contain closed_at or merged_at
	jsonStr := string(data)
	if jsonStr == "" {
		t.Fatal("JSON string is empty")
	}
	
	// These fields should be omitted when nil
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Failed to unmarshal to map: %v", err)
	}
	
	if _, exists := m["closed_at"]; exists {
		t.Error("closed_at should be omitted when nil")
	}
	if _, exists := m["merged_at"]; exists {
		t.Error("merged_at should be omitted when nil")
	}
}

func TestFetchOptionsDefaults(t *testing.T) {
	if defaultPageSize != 50 {
		t.Errorf("defaultPageSize = %d, want 50", defaultPageSize)
	}
}