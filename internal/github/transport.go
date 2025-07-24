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
	"fmt"
	"net/http"
	"time"

	"github.com/sirseerhq/sirseer-relay/internal/config"
	relaierrors "github.com/sirseerhq/sirseer-relay/internal/errors"
	"github.com/sirseerhq/sirseer-relay/internal/giterror"
	"github.com/sirseerhq/sirseer-relay/internal/ratelimit"
	"github.com/sirseerhq/sirseer-relay/pkg/version"
)

// StateSaver provides an interface for saving state during rate limit waits.
type StateSaver interface {
	Save() error
}

// rateLimitTransport adds rate limit detection and handling to HTTP requests.
// It wraps the auth transport and checks responses for rate limit headers.
type rateLimitTransport struct {
	base       http.RoundTripper
	detector   *ratelimit.Detector
	waiter     *ratelimit.Waiter
	config     *config.RateLimitConfig
	stateSaver StateSaver
}

// newRateLimitTransport creates a new transport with rate limit handling.
func newRateLimitTransport(token string, cfg *config.RateLimitConfig, stateSaver StateSaver) http.RoundTripper {
	authTransport := &authTransport{
		token: token,
		base:  http.DefaultTransport,
	}

	return &rateLimitTransport{
		base:       authTransport,
		detector:   ratelimit.NewDetector(),
		waiter:     ratelimit.NewWaiter(cfg.ShowProgress),
		config:     cfg,
		stateSaver: stateSaver,
	}
}

// RoundTrip implements http.RoundTripper with rate limit handling.
func (t *rateLimitTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Add standard headers
	req.Header.Set("User-Agent", fmt.Sprintf("sirseer-relay/%s", version.Version))

	// Execute the request
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	// Check for rate limiting
	if t.detector.IsRateLimited(resp) {
		info := t.detector.Detect(resp)

		if !t.config.AutoWait {
			// Return rate limit error without waiting
			return resp, fmt.Errorf("rate limit exceeded, reset at %s: %w",
				info.Reset.Format("3:04 PM"), relaierrors.ErrRateLimit)
		}

		// Save state before waiting
		if t.stateSaver != nil {
			// Save state before waiting - best effort
			_ = t.stateSaver.Save()
		}

		// Wait for rate limit to reset
		ctx := req.Context()
		if err := t.waiter.Wait(ctx, info); err != nil {
			return resp, fmt.Errorf("rate limit wait canceled: %w", err)
		}

		// Retry the request after waiting
		return t.RoundTrip(req)
	}

	return resp, nil
}

// retryTransport adds exponential backoff retry logic for transient failures.
type retryTransport struct {
	base       http.RoundTripper
	maxRetries int
}

// newRetryTransport creates a new transport with retry logic.
func newRetryTransport(base http.RoundTripper) http.RoundTripper {
	return &retryTransport{
		base:       base,
		maxRetries: 5,
	}
}

// RoundTrip implements http.RoundTripper with retry logic.
func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var lastErr error
	backoff := time.Second
	inspector := giterror.NewInspector()

	for attempt := 0; attempt < t.maxRetries; attempt++ {
		// Clone request for each attempt
		clonedReq := req.Clone(req.Context())

		resp, err := t.base.RoundTrip(clonedReq)

		// Success - return immediately
		if err == nil && !isRetryableStatusCode(resp.StatusCode) {
			return resp, nil
		}

		// Check if error is retryable
		if err != nil {
			if !inspector.IsRetryable(err) {
				return nil, err
			}
			lastErr = giterror.WithRetryInfo(err, attempt+1, t.maxRetries)
		} else {
			// Retryable status code
			lastErr = giterror.WithRetryInfo(
				fmt.Errorf("received status %d", resp.StatusCode),
				attempt+1, t.maxRetries)
			resp.Body.Close()
		}

		// Don't retry on the last attempt
		if attempt < t.maxRetries-1 {
			// Wait with exponential backoff
			select {
			case <-time.After(backoff):
				// Increase backoff for next attempt
				backoff *= 2
				if backoff > 30*time.Second {
					backoff = 30 * time.Second
				}
			case <-req.Context().Done():
				return nil, req.Context().Err()
			}
		}
	}

	return nil, giterror.WithUserAction(lastErr,
		"Network connection failed. Please check your internet connection and try again")
}

// isRetryableStatusCode checks if an HTTP status code should trigger a retry.
func isRetryableStatusCode(code int) bool {
	switch code {
	case http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

