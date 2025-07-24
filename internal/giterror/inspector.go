package giterror

import (
	"errors"
	"strings"
)

// Inspector provides methods for analyzing GitHub API errors.
type Inspector interface {
	// IsAuthError returns true if the error represents an authentication or authorization failure.
	IsAuthError(err error) bool

	// IsNotFoundError returns true if the error represents a resource not found error.
	IsNotFoundError(err error) bool

	// IsRateLimitError returns true if the error represents a rate limit error.
	IsRateLimitError(err error) bool

	// IsComplexityError returns true if the error represents a query complexity error.
	IsComplexityError(err error) bool

	// IsNetworkError returns true if the error represents a network connectivity error.
	IsNetworkError(err error) bool
}

// GitHubErrorInspector implements the Inspector interface for GitHub API errors.
type GitHubErrorInspector struct{}

// NewInspector creates a new GitHubErrorInspector.
func NewInspector() Inspector {
	return &GitHubErrorInspector{}
}

// IsAuthError checks if the error is an authentication or authorization error.
func (i *GitHubErrorInspector) IsAuthError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "401") ||
		strings.Contains(errStr, "403") ||
		strings.Contains(errStr, "unauthorized") ||
		strings.Contains(errStr, "forbidden") ||
		strings.Contains(errStr, "bad credentials") ||
		strings.Contains(errStr, "authentication")
}

// IsNotFoundError checks if the error is a not found error.
func (i *GitHubErrorInspector) IsNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "404") ||
		strings.Contains(errStr, "not found") ||
		strings.Contains(errStr, "could not resolve to a repository")
}

// IsRateLimitError checks if the error is a rate limit error.
func (i *GitHubErrorInspector) IsRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "rate limit") ||
		strings.Contains(errStr, "429") ||
		strings.Contains(errStr, "api rate limit exceeded")
}

// IsComplexityError checks if the error is a query complexity error.
func (i *GitHubErrorInspector) IsComplexityError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "complexity") ||
		strings.Contains(errStr, "query has complexity") ||
		strings.Contains(errStr, "exceeds maximum")
}

// IsNetworkError checks if the error is a network connectivity error.
func (i *GitHubErrorInspector) IsNetworkError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "no such host") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "temporary failure") ||
		strings.Contains(errStr, "dial tcp") ||
		strings.Contains(errStr, "tls handshake") ||
		strings.Contains(errStr, "network is unreachable")
}

// ErrorChainInspector wraps a base inspector and adds support for checking errors
// in the error chain using errors.Is and errors.As.
type ErrorChainInspector struct {
	base Inspector
}

// NewErrorChainInspector creates a new ErrorChainInspector that checks both
// the error chain and falls back to string-based inspection.
func NewErrorChainInspector(base Inspector) Inspector {
	return &ErrorChainInspector{base: base}
}

// IsAuthError checks the error chain first, then falls back to base inspector.
func (e *ErrorChainInspector) IsAuthError(err error) bool {
	// Check for any known auth error types in the chain
	var authErr interface{ IsAuthError() bool }
	if errors.As(err, &authErr) && authErr.IsAuthError() {
		return true
	}
	return e.base.IsAuthError(err)
}

// IsNotFoundError checks the error chain first, then falls back to base inspector.
func (e *ErrorChainInspector) IsNotFoundError(err error) bool {
	var notFoundErr interface{ IsNotFoundError() bool }
	if errors.As(err, &notFoundErr) && notFoundErr.IsNotFoundError() {
		return true
	}
	return e.base.IsNotFoundError(err)
}

// IsRateLimitError checks the error chain first, then falls back to base inspector.
func (e *ErrorChainInspector) IsRateLimitError(err error) bool {
	var rateLimitErr interface{ IsRateLimitError() bool }
	if errors.As(err, &rateLimitErr) && rateLimitErr.IsRateLimitError() {
		return true
	}
	return e.base.IsRateLimitError(err)
}

// IsComplexityError checks the error chain first, then falls back to base inspector.
func (e *ErrorChainInspector) IsComplexityError(err error) bool {
	var complexityErr interface{ IsComplexityError() bool }
	if errors.As(err, &complexityErr) && complexityErr.IsComplexityError() {
		return true
	}
	return e.base.IsComplexityError(err)
}

// IsNetworkError checks the error chain first, then falls back to base inspector.
func (e *ErrorChainInspector) IsNetworkError(err error) bool {
	var networkErr interface{ IsNetworkError() bool }
	if errors.As(err, &networkErr) && networkErr.IsNetworkError() {
		return true
	}
	return e.base.IsNetworkError(err)
}
