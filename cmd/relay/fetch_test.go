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

package main

import (
	"os"
	"testing"

	relaierrors "github.com/sirseerhq/sirseer-relay/internal/errors"
)

func TestParseRepository(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		{
			name:      "valid repository",
			input:     "golang/go",
			wantOwner: "golang",
			wantRepo:  "go",
			wantErr:   false,
		},
		{
			name:      "valid repository with dashes",
			input:     "kubernetes/kubernetes",
			wantOwner: "kubernetes",
			wantRepo:  "kubernetes",
			wantErr:   false,
		},
		{
			name:      "repository with spaces",
			input:     " golang / go ",
			wantOwner: "golang",
			wantRepo:  "go",
			wantErr:   false,
		},
		{
			name:    "missing slash",
			input:   "golang",
			wantErr: true,
		},
		{
			name:    "too many slashes",
			input:   "golang/go/extra",
			wantErr: true,
		},
		{
			name:    "empty owner",
			input:   "/go",
			wantErr: true,
		},
		{
			name:    "empty repo",
			input:   "golang/",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo, err := parseRepository(tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("parseRepository() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if owner != tt.wantOwner {
					t.Errorf("parseRepository() owner = %v, want %v", owner, tt.wantOwner)
				}
				if repo != tt.wantRepo {
					t.Errorf("parseRepository() repo = %v, want %v", repo, tt.wantRepo)
				}
			}
		})
	}
}

func TestGetToken(t *testing.T) {
	// Save original env var
	originalToken := os.Getenv("GITHUB_TOKEN")
	defer os.Setenv("GITHUB_TOKEN", originalToken)

	tests := []struct {
		name      string
		flagToken string
		envToken  string
		want      string
	}{
		{
			name:      "flag token takes precedence",
			flagToken: "flag-token",
			envToken:  "env-token",
			want:      "flag-token",
		},
		{
			name:      "env token when no flag",
			flagToken: "",
			envToken:  "env-token",
			want:      "env-token",
		},
		{
			name:      "empty when neither set",
			flagToken: "",
			envToken:  "",
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("GITHUB_TOKEN", tt.envToken)

			got := getToken(tt.flagToken)
			if got != tt.want {
				t.Errorf("getToken() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMapErrorToExitCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{
			name: "nil error",
			err:  nil,
			want: 0,
		},
		{
			name: "invalid token error",
			err:  relaierrors.ErrInvalidToken,
			want: 2,
		},
		{
			name: "repo not found error",
			err:  relaierrors.ErrRepoNotFound,
			want: 2,
		},
		{
			name: "rate limit error",
			err:  relaierrors.ErrRateLimit,
			want: 2,
		},
		{
			name: "network failure error",
			err:  relaierrors.ErrNetworkFailure,
			want: 3,
		},
		{
			name: "generic error",
			err:  os.ErrNotExist,
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapErrorToExitCode(tt.err)
			if got != tt.want {
				t.Errorf("mapErrorToExitCode() = %v, want %v", got, tt.want)
			}
		})
	}
}
