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

package errors

import (
	"errors"
	"fmt"
	"testing"
)

func TestSentinelErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		sentinel error
		want     bool
	}{
		{
			name:     "direct invalid token error",
			err:      ErrInvalidToken,
			sentinel: ErrInvalidToken,
			want:     true,
		},
		{
			name:     "wrapped invalid token error",
			err:      fmt.Errorf("failed to authenticate: %w", ErrInvalidToken),
			sentinel: ErrInvalidToken,
			want:     true,
		},
		{
			name:     "different error type",
			err:      ErrRepoNotFound,
			sentinel: ErrInvalidToken,
			want:     false,
		},
		{
			name:     "wrapped network error",
			err:      fmt.Errorf("connection failed: %w", ErrNetworkFailure),
			sentinel: ErrNetworkFailure,
			want:     true,
		},
		{
			name:     "nil error",
			err:      nil,
			sentinel: ErrInvalidToken,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := errors.Is(tt.err, tt.sentinel)
			if got != tt.want {
				t.Errorf("errors.Is(%v, %v) = %v, want %v", tt.err, tt.sentinel, got, tt.want)
			}
		})
	}
}

func TestErrorMessages(t *testing.T) {
	tests := []struct {
		err  error
		want string
	}{
		{ErrInvalidToken, "invalid github token"},
		{ErrRepoNotFound, "repository not found"},
		{ErrNetworkFailure, "network connection failed"},
		{ErrRateLimit, "github rate limit exceeded"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}