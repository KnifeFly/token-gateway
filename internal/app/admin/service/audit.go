package service

import (
	"context"
	"encoding/json"
	"strings"

	adminapp "github.com/KnifeFly/token-gateway/internal/app/admin"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

// ListAuditEvents returns redacted durable Admin audit events.
func (s *Service) ListAuditEvents(ctx context.Context, actor adminapp.Actor, filter adminapp.AuditFilter) (adminapp.ListResponse[adminapp.AuditEvent], error) {
	if err := s.Authorize(actor, "read", "audit"); err != nil {
		return adminapp.ListResponse[adminapp.AuditEvent]{}, err
	}
	events, err := s.repo.ListAuditEvents(ctx, filter)
	return adminapp.ListResponse[adminapp.AuditEvent]{Data: events}, err
}

func mutate[T any](ctx context.Context, s *Service, actor adminapp.Actor, opts adminapp.MutationOptions, action string, resource string, resourceID string, before any, fn func() (T, error)) (T, error) {
	var zero T
	if s == nil || s.repo == nil {
		return zero, apperr.ConfigUnavailable("admin web repository is unavailable")
	}
	if err := validateMutationOptions(opts); err != nil {
		return zero, err
	}
	if err := s.Authorize(actor, action, resource); err != nil {
		audit := adminapp.AuditEvent{
			ID:             newID("audit"),
			Actor:          actor,
			Action:         action,
			Resource:       resource,
			ResourceID:     resourceID,
			RequestID:      opts.RequestID,
			IdempotencyKey: opts.IdempotencyKey,
			Reason:         opts.Reason,
			Status:         auditStatusFailed,
			ErrorCode:      errorCodeOf(err),
			Before:         redactedJSON(before),
			RemoteAddr:     strings.TrimSpace(opts.RemoteAddr),
			UserAgentHash:  hashText(opts.UserAgent),
			CreatedAt:      s.now(),
		}
		_, _ = s.repo.CreateAuditEvent(ctx, audit)
		return zero, err
	}

	result, err := fn()
	status := auditStatusOK
	errorCode := ""
	if err != nil {
		status = auditStatusFailed
		errorCode = errorCodeOf(err)
	}
	audit := adminapp.AuditEvent{
		ID:             newID("audit"),
		Actor:          actor,
		Action:         action,
		Resource:       resource,
		ResourceID:     resourceID,
		RequestID:      opts.RequestID,
		IdempotencyKey: opts.IdempotencyKey,
		Reason:         opts.Reason,
		Status:         status,
		ErrorCode:      errorCode,
		Before:         redactedJSON(before),
		After:          redactedJSON(result),
		RemoteAddr:     strings.TrimSpace(opts.RemoteAddr),
		UserAgentHash:  hashText(opts.UserAgent),
		CreatedAt:      s.now(),
	}
	if _, auditErr := s.repo.CreateAuditEvent(ctx, audit); auditErr != nil && err == nil {
		return zero, auditErr
	}
	return result, err
}

func validateMutationOptions(opts adminapp.MutationOptions) error {
	if strings.TrimSpace(opts.IdempotencyKey) == "" {
		return apperr.InvalidArgument("idempotency key is required")
	}
	if strings.TrimSpace(opts.Reason) == "" {
		return apperr.InvalidArgument("reason is required")
	}
	return nil
}

func redactedJSON(value any) json.RawMessage {
	content, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var decoded any
	if err := json.Unmarshal(content, &decoded); err != nil {
		return nil
	}
	redacted := redactValue(decoded, "")
	content, err = json.Marshal(redacted)
	if err != nil {
		return nil
	}
	return content
}

func redactValue(value any, key string) any {
	if sensitiveKey(key) {
		return "[REDACTED]"
	}
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for childKey, childValue := range typed {
			out[childKey] = redactValue(childValue, childKey)
		}
		return out
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, redactValue(item, key))
		}
		return out
	default:
		return value
	}
}

func sensitiveKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	switch key {
	case "password", "api_key", "key_hash", "plaintext_key", "encrypted_api_key", "access_token", "refresh_token", "prompt", "response", "payload":
		return true
	}
	return strings.Contains(key, "secret") || strings.Contains(key, "credential") || strings.Contains(key, "ciphertext")
}

func safeShort(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
	}
	if len(value) > 240 {
		return value[:240]
	}
	return value
}

func errorCodeOf(err error) string {
	if appErr, ok := apperr.As(err); ok {
		return string(appErr.Code)
	}
	return string(apperr.CodeInternal)
}
