package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

const (
	legacyHashPrefix = "sha256:"
	hmacHashPrefix   = "hmac-sha256:"
)

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
	hasher     *APIKeyHasher
}

// RevocationChecker checks fast API key revocation state.
type RevocationChecker interface {
	IsRevoked(ctx context.Context, keyHash string) (bool, error)
}

// AuthenticatorOption customizes snapshot API key authentication.
type AuthenticatorOption func(*SnapshotAuthenticator)

// NewSnapshotAuthenticator returns an authenticator with optional fast revocation checks.
func NewSnapshotAuthenticator(revocation ...RevocationChecker) *SnapshotAuthenticator {
	return NewSnapshotAuthenticatorWithOptions(firstRevocation(revocation...), nil)
}

// NewSnapshotAuthenticatorWithOptions returns an authenticator with explicit options.
func NewSnapshotAuthenticatorWithOptions(revocation RevocationChecker, options ...AuthenticatorOption) *SnapshotAuthenticator {
	auth := &SnapshotAuthenticator{
		extractor:  CredentialExtractor{},
		revocation: revocation,
		hasher:     NewAPIKeyHasher(""),
	}
	for _, option := range options {
		if option != nil {
			option(auth)
		}
	}
	if auth.hasher == nil {
		auth.hasher = NewAPIKeyHasher("")
	}
	return auth
}

// WithAPIKeyHashSecret configures HMAC-SHA256 API key verification.
func WithAPIKeyHashSecret(secret string) AuthenticatorOption {
	return func(auth *SnapshotAuthenticator) {
		auth.hasher = NewAPIKeyHasher(secret)
	}
}

// WithAPIKeyHasher configures a custom API key hasher.
func WithAPIKeyHasher(hasher *APIKeyHasher) AuthenticatorOption {
	return func(auth *SnapshotAuthenticator) {
		auth.hasher = hasher
	}
}

func firstRevocation(revocation ...RevocationChecker) RevocationChecker {
	if len(revocation) > 0 {
		return revocation[0]
	}
	return nil
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
	for _, keyHash := range a.hasher.Candidates(credential) {
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
			continue
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
	return apperr.Unauthorized("invalid api key")
}

// APIKeyHasher hashes customer API keys for snapshot lookup.
type APIKeyHasher struct {
	secret []byte
}

// NewAPIKeyHasher returns an API key hasher. An empty secret keeps legacy SHA-256 hashing.
func NewAPIKeyHasher(secret string) *APIKeyHasher {
	return &APIKeyHasher{secret: []byte(strings.TrimSpace(secret))}
}

// Hash returns the default hash for newly created API keys.
func (h *APIKeyHasher) Hash(key string) string {
	if h != nil && len(h.secret) > 0 {
		return hmacHash(key, h.secret)
	}
	return HashAPIKey(key)
}

// Candidates returns hash lookup candidates for the compatibility migration window.
func (h *APIKeyHasher) Candidates(key string) []string {
	legacy := HashAPIKey(key)
	if h == nil || len(h.secret) == 0 {
		return []string{legacy}
	}
	hmacHash := hmacHash(key, h.secret)
	if hmacHash == legacy {
		return []string{hmacHash}
	}
	return []string{hmacHash, legacy}
}

// HashAPIKey returns the legacy stable snapshot hash for an API key.
func HashAPIKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return legacyHashPrefix + hex.EncodeToString(sum[:])
}

// HashAPIKeyHMAC returns the HMAC-SHA256 snapshot hash for an API key.
func HashAPIKeyHMAC(key string, secret string) string {
	return hmacHash(key, []byte(strings.TrimSpace(secret)))
}

func hmacHash(key string, secret []byte) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(key))
	return hmacHashPrefix + hex.EncodeToString(mac.Sum(nil))
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
