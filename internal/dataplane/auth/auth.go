package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

const hashPrefix = "sha256:"

// CredentialExtractor extracts customer API keys from supported headers.
type CredentialExtractor struct{}

// Extract returns the API key from Authorization or X-API-Key headers.
func (CredentialExtractor) Extract(header http.Header) (string, error) {
	bearer := bearerToken(headerValue(header, "Authorization"))
	apiKey := strings.TrimSpace(headerValue(header, "X-API-Key"))
	switch {
	case bearer != "" && apiKey != "" && bearer != apiKey:
		return "", apperr.Unauthorized("conflicting credentials")
	case bearer != "":
		return bearer, nil
	case apiKey != "":
		return apiKey, nil
	default:
		return "", apperr.Unauthorized("missing api key")
	}
}

// SnapshotAuthenticator authenticates API keys against the pinned snapshot.
type SnapshotAuthenticator struct {
	extractor  CredentialExtractor
	revocation RevocationChecker
}

// RevocationChecker checks fast API key revocation state.
type RevocationChecker interface {
	IsRevoked(ctx context.Context, keyHash string) (bool, error)
}

// NewSnapshotAuthenticator returns an authenticator with optional fast revocation checks.
func NewSnapshotAuthenticator(revocation ...RevocationChecker) *SnapshotAuthenticator {
	auth := &SnapshotAuthenticator{extractor: CredentialExtractor{}}
	if len(revocation) > 0 {
		auth.revocation = revocation[0]
	}
	return auth
}

// Authenticate validates the caller against snapshot and revocation state.
func (a *SnapshotAuthenticator) Authenticate(ctx context.Context, state *engine.RequestState) error {
	if state.Snapshot == nil {
		return apperr.ConfigUnavailable("runtime snapshot is unavailable")
	}
	credential, err := a.extractor.Extract(state.Incoming.Header)
	if err != nil {
		return err
	}
	keyHash := HashAPIKey(credential)
	if state.Snapshot.IsAPIKeyRevoked(keyHash) {
		return apperr.Unauthorized("api key is revoked")
	}
	if a.revocation != nil {
		revoked, err := a.revocation.IsRevoked(ctx, keyHash)
		if err != nil {
			return err
		}
		if revoked {
			return apperr.Unauthorized("api key is revoked")
		}
	}
	apiKey, ok := state.Snapshot.LookupAPIKeyHash(keyHash)
	if !ok || !apiKey.Enabled {
		return apperr.Unauthorized("invalid api key")
	}
	state.Principal = &engine.Principal{
		TenantID:      apiKey.TenantID,
		ProjectID:     apiKey.ProjectID,
		APIKeyID:      apiKey.ID,
		AllowedModels: append([]string(nil), apiKey.AllowedModels...),
	}
	state.TenantID = apiKey.TenantID
	state.ProjectID = apiKey.ProjectID
	state.APIKeyID = apiKey.ID
	return nil
}

// HashAPIKey returns the stable snapshot hash for an API key.
func HashAPIKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hashPrefix + hex.EncodeToString(sum[:])
}

func bearerToken(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	prefix := "bearer "
	if len(value) <= len(prefix) || strings.ToLower(value[:len(prefix)]) != prefix {
		return ""
	}
	return strings.TrimSpace(value[len(prefix):])
}

func headerValue(header http.Header, name string) string {
	if value := header.Get(name); value != "" {
		return value
	}
	for key, values := range header {
		if strings.EqualFold(key, name) && len(values) > 0 {
			return values[0]
		}
	}
	return ""
}
