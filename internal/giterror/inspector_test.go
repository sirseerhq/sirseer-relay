package giterror

import (
	"errors"
	"fmt"
	"testing"
)

func TestGitHubErrorInspector_IsAuthError(t *testing.T) {
	inspector := NewInspector()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "401 unauthorized",
			err:  errors.New("401 Unauthorized"),
			want: true,
		},
		{
			name: "403 forbidden",
			err:  errors.New("403 Forbidden"),
			want: true,
		},
		{
			name: "bad credentials",
			err:  errors.New("Bad credentials"),
			want: true,
		},
		{
			name: "authentication required",
			err:  errors.New("Authentication required"),
			want: true,
		},
		{
			name: "wrapped auth error",
			err:  fmt.Errorf("failed to query: %w", errors.New("401 Unauthorized")),
			want: true,
		},
		{
			name: "not an auth error",
			err:  errors.New("something went wrong"),
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := inspector.IsAuthError(tt.err); got != tt.want {
				t.Errorf("IsAuthError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGitHubErrorInspector_IsNotFoundError(t *testing.T) {
	inspector := NewInspector()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "404 not found",
			err:  errors.New("404 Not Found"),
			want: true,
		},
		{
			name: "resource not found",
			err:  errors.New("Resource not found"),
			want: true,
		},
		{
			name: "could not resolve repository",
			err:  errors.New("Could not resolve to a Repository with the name 'org/repo'"),
			want: true,
		},
		{
			name: "wrapped not found error",
			err:  fmt.Errorf("failed to fetch: %w", errors.New("404 Not Found")),
			want: true,
		},
		{
			name: "not a not found error",
			err:  errors.New("internal server error"),
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := inspector.IsNotFoundError(tt.err); got != tt.want {
				t.Errorf("IsNotFoundError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGitHubErrorInspector_IsRateLimitError(t *testing.T) {
	inspector := NewInspector()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "rate limit exceeded",
			err:  errors.New("API rate limit exceeded"),
			want: true,
		},
		{
			name: "429 too many requests",
			err:  errors.New("429 Too Many Requests"),
			want: true,
		},
		{
			name: "rate limit message",
			err:  errors.New("You have exceeded a secondary rate limit"),
			want: true,
		},
		{
			name: "wrapped rate limit error",
			err:  fmt.Errorf("github api error: %w", errors.New("API rate limit exceeded")),
			want: true,
		},
		{
			name: "not a rate limit error",
			err:  errors.New("timeout occurred"),
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := inspector.IsRateLimitError(tt.err); got != tt.want {
				t.Errorf("IsRateLimitError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGitHubErrorInspector_IsComplexityError(t *testing.T) {
	inspector := NewInspector()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "query complexity",
			err:  errors.New("Query has complexity 120001, which exceeds max complexity of 120000"),
			want: true,
		},
		{
			name: "exceeds maximum",
			err:  errors.New("Field 'pullRequests' exceeds maximum complexity"),
			want: true,
		},
		{
			name: "complexity limit",
			err:  errors.New("query complexity limit exceeded"),
			want: true,
		},
		{
			name: "wrapped complexity error",
			err:  fmt.Errorf("graphql error: %w", errors.New("Query has complexity 150000")),
			want: true,
		},
		{
			name: "not a complexity error",
			err:  errors.New("invalid query syntax"),
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := inspector.IsComplexityError(tt.err); got != tt.want {
				t.Errorf("IsComplexityError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGitHubErrorInspector_IsNetworkError(t *testing.T) {
	inspector := NewInspector()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "connection refused",
			err:  errors.New("dial tcp 127.0.0.1:443: connection refused"),
			want: true,
		},
		{
			name: "no such host",
			err:  errors.New("dial tcp: lookup api.github.com: no such host"),
			want: true,
		},
		{
			name: "timeout",
			err:  errors.New("request timeout after 30s"),
			want: true,
		},
		{
			name: "temporary failure",
			err:  errors.New("temporary failure in name resolution"),
			want: true,
		},
		{
			name: "tls handshake error",
			err:  errors.New("tls handshake timeout"),
			want: true,
		},
		{
			name: "network unreachable",
			err:  errors.New("network is unreachable"),
			want: true,
		},
		{
			name: "wrapped network error",
			err:  fmt.Errorf("failed to connect: %w", errors.New("connection refused")),
			want: true,
		},
		{
			name: "not a network error",
			err:  errors.New("invalid json response"),
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := inspector.IsNetworkError(tt.err); got != tt.want {
				t.Errorf("IsNetworkError() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Custom error types for testing ErrorChainInspector
type authError struct{}

func (authError) Error() string     { return "custom auth error" }
func (authError) IsAuthError() bool { return true }

type rateLimitError struct{}

func (rateLimitError) Error() string          { return "custom rate limit error" }
func (rateLimitError) IsRateLimitError() bool { return true }

func TestErrorChainInspector(t *testing.T) {
	baseInspector := NewInspector()
	chainInspector := NewErrorChainInspector(baseInspector)

	tests := []struct {
		name   string
		err    error
		method string
		want   bool
	}{
		{
			name:   "custom auth error type",
			err:    authError{},
			method: "auth",
			want:   true,
		},
		{
			name:   "wrapped custom auth error",
			err:    fmt.Errorf("operation failed: %w", authError{}),
			method: "auth",
			want:   true,
		},
		{
			name:   "custom rate limit error type",
			err:    rateLimitError{},
			method: "ratelimit",
			want:   true,
		},
		{
			name:   "falls back to string checking",
			err:    errors.New("401 Unauthorized"),
			method: "auth",
			want:   true,
		},
		{
			name:   "no match in chain or string",
			err:    errors.New("some other error"),
			method: "auth",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got bool
			switch tt.method {
			case "auth":
				got = chainInspector.IsAuthError(tt.err)
			case "ratelimit":
				got = chainInspector.IsRateLimitError(tt.err)
			}
			if got != tt.want {
				t.Errorf("ErrorChainInspector.%s() = %v, want %v", tt.method, got, tt.want)
			}
		})
	}
}
