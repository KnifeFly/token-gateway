package task

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

func requestHash(body []byte) string {
	sum := sha256.Sum256(body)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func newIdempotencyRecord(tenantID, apiKeyID, endpoint, key, hash string, resourceType ResourceType, ttl time.Duration, now time.Time) *IdempotencyRecord {
	if key == "" {
		return nil
	}
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &IdempotencyRecord{
		ID:             newID("idem"),
		TenantID:       tenantID,
		APIKeyID:       apiKeyID,
		Endpoint:       endpoint,
		IdempotencyKey: key,
		RequestHash:    hash,
		ResourceType:   resourceType,
		Status:         IdempotencyStatusReserved,
		ExpiresAt:      now.Add(ttl),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func checkIdempotencyHash(record *IdempotencyRecord, hash string) error {
	if record == nil {
		return nil
	}
	if record.RequestHash != hash {
		return apperr.New(apperr.CodeIdempotencyConflict, "idempotency key was used with a different request body", 409)
	}
	return nil
}
