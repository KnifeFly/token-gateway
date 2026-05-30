package apperr

import (
	"errors"
	"fmt"
	"net/http"
)

// Code identifies a stable external error class.
type Code string

const (
	CodeUnauthorized        Code = "unauthorized"
	CodeForbidden           Code = "forbidden"
	CodeInvalidArgument     Code = "invalid_argument"
	CodeNotFound            Code = "not_found"
	CodeRateLimited         Code = "rate_limited"
	CodeInsufficientBalance Code = "insufficient_balance"
	CodeProviderError       Code = "provider_error"
	CodeAmbiguousProtocol   Code = "ambiguous_protocol"
	CodeIdempotencyConflict Code = "idempotency_conflict"
	CodePolicyDenied        Code = "policy_denied"
	CodeSnapshotStale       Code = "snapshot_stale"
	CodeFeatureNotEnabled   Code = "feature_not_enabled"
	CodeConfigUnavailable   Code = "config_unavailable"
	CodeServiceUnavailable  Code = "service_unavailable"
	CodeInternal            Code = "internal_error"
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

func Unauthorized(message string, opts ...Option) *Error {
	return New(CodeUnauthorized, message, http.StatusUnauthorized, opts...)
}

func Forbidden(message string, opts ...Option) *Error {
	return New(CodeForbidden, message, http.StatusForbidden, opts...)
}

func InvalidArgument(message string, opts ...Option) *Error {
	return New(CodeInvalidArgument, message, http.StatusBadRequest, opts...)
}

func NotFound(message string, opts ...Option) *Error {
	return New(CodeNotFound, message, http.StatusNotFound, opts...)
}

func RateLimited(message string, opts ...Option) *Error {
	return New(CodeRateLimited, message, http.StatusTooManyRequests, opts...)
}

func InsufficientBalance(message string, opts ...Option) *Error {
	return New(CodeInsufficientBalance, message, http.StatusPaymentRequired, opts...)
}

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

func ConfigUnavailable(message string, opts ...Option) *Error {
	return New(CodeConfigUnavailable, message, http.StatusServiceUnavailable, opts...)
}

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
