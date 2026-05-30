package apperr

import (
	"errors"
	"fmt"
	"net/http"
)

// Code identifies a stable external error class.
type Code string

const (
	// CodeUnauthorized identifies missing or invalid authentication.
	CodeUnauthorized Code = "unauthorized"
	// CodeForbidden identifies authenticated callers without permission.
	CodeForbidden Code = "forbidden"
	// CodeInvalidArgument identifies malformed client input.
	CodeInvalidArgument Code = "invalid_argument"
	// CodeNotFound identifies a missing requested resource.
	CodeNotFound Code = "not_found"
	// CodeRateLimited identifies requests rejected by a limit.
	CodeRateLimited Code = "rate_limited"
	// CodeInsufficientBalance identifies requests without enough prepaid balance.
	CodeInsufficientBalance Code = "insufficient_balance"
	// CodeProviderError identifies upstream provider failures.
	CodeProviderError Code = "provider_error"
	// CodeAmbiguousProtocol identifies requests that match more than one protocol.
	CodeAmbiguousProtocol Code = "ambiguous_protocol"
	// CodeIdempotencyConflict identifies reused idempotency keys with different payloads.
	CodeIdempotencyConflict Code = "idempotency_conflict"
	// CodePolicyDenied identifies requests blocked by policy.
	CodePolicyDenied Code = "policy_denied"
	// CodeSnapshotStale identifies requests rejected because runtime config is too old.
	CodeSnapshotStale Code = "snapshot_stale"
	// CodeFeatureNotEnabled identifies explicitly disabled product capabilities.
	CodeFeatureNotEnabled Code = "feature_not_enabled"
	// CodeConfigUnavailable identifies missing runtime configuration.
	CodeConfigUnavailable Code = "config_unavailable"
	// CodeServiceUnavailable identifies transient service failures.
	CodeServiceUnavailable Code = "service_unavailable"
	// CodeInternal identifies unexpected internal failures.
	CodeInternal Code = "internal_error"
)

// Error is the application error type returned across transport boundaries.
type Error struct {
	Code       Code
	Message    string
	HTTPStatus int
	Temporary  bool
	Safe       bool
	Cause      error
}

// Error returns the safe message and stable code.
func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Message == "" {
		return string(e.Code)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap exposes the underlying cause for internal logging and checks.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// SafeMessage returns a client-safe message.
func (e *Error) SafeMessage() string {
	if e == nil {
		return ""
	}
	if e.Safe && e.Message != "" {
		return e.Message
	}
	return defaultMessage(e.Code)
}

// New creates an application error.
func New(code Code, message string, status int, opts ...Option) *Error {
	e := &Error{
		Code:       code,
		Message:    message,
		HTTPStatus: status,
		Safe:       true,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Option mutates an Error during construction.
type Option func(*Error)

// WithCause records an internal cause without making it client-visible.
func WithCause(err error) Option {
	return func(e *Error) {
		e.Cause = err
	}
}

// WithTemporary marks the error as transient.
func WithTemporary() Option {
	return func(e *Error) {
		e.Temporary = true
	}
}

// WithUnsafeMessage prevents Message from being returned to clients.
func WithUnsafeMessage() Option {
	return func(e *Error) {
		e.Safe = false
	}
}

// As returns an Error from err, if present.
func As(err error) (*Error, bool) {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr, true
	}
	return nil, false
}

// Unauthorized returns a 401 application error.
func Unauthorized(message string, opts ...Option) *Error {
	return New(CodeUnauthorized, message, http.StatusUnauthorized, opts...)
}

// Forbidden returns a 403 application error.
func Forbidden(message string, opts ...Option) *Error {
	return New(CodeForbidden, message, http.StatusForbidden, opts...)
}

// InvalidArgument returns a 400 application error.
func InvalidArgument(message string, opts ...Option) *Error {
	return New(CodeInvalidArgument, message, http.StatusBadRequest, opts...)
}

// NotFound returns a 404 application error.
func NotFound(message string, opts ...Option) *Error {
	return New(CodeNotFound, message, http.StatusNotFound, opts...)
}

// RateLimited returns a 429 application error.
func RateLimited(message string, opts ...Option) *Error {
	return New(CodeRateLimited, message, http.StatusTooManyRequests, opts...)
}

// InsufficientBalance returns a 402 application error.
func InsufficientBalance(message string, opts ...Option) *Error {
	return New(CodeInsufficientBalance, message, http.StatusPaymentRequired, opts...)
}

// ProviderError returns a 502 application error for upstream provider failures.
func ProviderError(message string, opts ...Option) *Error {
	return New(CodeProviderError, message, http.StatusBadGateway, opts...)
}

// AmbiguousProtocol returns an error when a request cannot be assigned to one protocol.
func AmbiguousProtocol(message string, opts ...Option) *Error {
	return New(CodeAmbiguousProtocol, message, http.StatusBadRequest, opts...)
}

// PolicyDenied returns an error when security policy blocks a request.
func PolicyDenied(message string, opts ...Option) *Error {
	return New(CodePolicyDenied, message, http.StatusForbidden, opts...)
}

// ConfigUnavailable returns a 503 application error for missing runtime configuration.
func ConfigUnavailable(message string, opts ...Option) *Error {
	return New(CodeConfigUnavailable, message, http.StatusServiceUnavailable, opts...)
}

// ServiceUnavailable returns a 503 application error for transient service failures.
func ServiceUnavailable(message string, opts ...Option) *Error {
	return New(CodeServiceUnavailable, message, http.StatusServiceUnavailable, opts...)
}

// SnapshotStale returns an error when the runtime snapshot is too old to use.
func SnapshotStale(message string, opts ...Option) *Error {
	return New(CodeSnapshotStale, message, http.StatusServiceUnavailable, opts...)
}

// FeatureNotEnabled returns an error for explicitly disabled product capabilities.
func FeatureNotEnabled(message string, opts ...Option) *Error {
	return New(CodeFeatureNotEnabled, message, http.StatusNotImplemented, opts...)
}

// Internal returns a 500 application error.
func Internal(message string, opts ...Option) *Error {
	return New(CodeInternal, message, http.StatusInternalServerError, opts...)
}

func defaultMessage(code Code) string {
	switch code {
	case CodeUnauthorized:
		return "authentication is required"
	case CodeForbidden:
		return "permission denied"
	case CodeInvalidArgument:
		return "invalid request"
	case CodeNotFound:
		return "resource not found"
	case CodeRateLimited:
		return "rate limit exceeded"
	case CodeInsufficientBalance:
		return "insufficient balance"
	case CodeProviderError:
		return "provider error"
	case CodeAmbiguousProtocol:
		return "ambiguous protocol"
	case CodeIdempotencyConflict:
		return "idempotency conflict"
	case CodePolicyDenied:
		return "policy denied"
	case CodeSnapshotStale:
		return "configuration snapshot is stale"
	case CodeFeatureNotEnabled:
		return "feature is not enabled"
	case CodeConfigUnavailable:
		return "configuration unavailable"
	case CodeServiceUnavailable:
		return "service unavailable"
	default:
		return "internal error"
	}
}
