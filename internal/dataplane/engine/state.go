package engine

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/KnifeFly/token-gateway/pkg/tokenusage"
)

// RequestState carries one request through the data-plane hot path.
type RequestState struct {
	RequestID       string
	TraceID         string
	ClientRequestID string
	StartedAt       time.Time

	Incoming     IncomingRequest
	ProtocolMode ProtocolMode
	CanonicalAPI CanonicalAPI
	Endpoint     EndpointSpec

	Principal *Principal
	TenantID  string
	ProjectID string
	APIKeyID  string
	ClientIP  string

	RequestedModel string
	ResolvedModel  ModelView
	PriceRule      PriceRuleView
	LimitRule      LimitRuleView
	Stream         bool
	Async          bool
	IdempotencyKey string

	Snapshot    SnapshotView
	SnapshotRef SnapshotRef

	Parsed         ParsedRequest
	EstimatedUsage tokenusage.Estimate
	ActualUsage    tokenusage.Actual
	PolicyDecision PolicyDecision
	RoutePlan      *RoutePlan
	ProviderResult *ProviderResult
	Attempts       []ProviderAttempt

	Currency              string
	EstimatedChargeMicros int64
	ActualChargeMicros    int64
	BalanceHoldID         string
	BalanceAccountID      string
	SettlementID          string
	LimitReleases         []LimitRelease

	Metadata map[string]string
	Internal map[string]any
}

// IsTaskOperation reports whether this request is task query/control.
func (s *RequestState) IsTaskOperation() bool {
	return s != nil && s.Parsed.Task != nil
}

// IsFileOperation reports whether this request is file metadata/quota.
func (s *RequestState) IsFileOperation() bool {
	return s != nil && s.Parsed.File != nil
}

func newState(req IncomingRequest) *RequestState {
	clientRequestID := headerValue(req.Header, "X-Client-Request-ID")
	if clientRequestID == "" {
		clientRequestID = headerValue(req.Header, "X-Request-ID")
	}

	requestID := headerValue(req.Header, "X-Gateway-Request-ID")
	if requestID == "" {
		requestID = newID()
	}
	traceID := req.Header.Get("X-Trace-ID")
	if traceID == "" {
		traceID = newID()
	}
	return &RequestState{
		RequestID:       requestID,
		TraceID:         traceID,
		ClientRequestID: clientRequestID,
		StartedAt:       time.Now().UTC(),
		Incoming:        req,
		ClientIP:        req.RemoteAddr,
		Metadata:        make(map[string]string),
		Internal:        make(map[string]any),
	}
}

func headerValue(header http.Header, key string) string {
	values := header.Values(key)
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	for headerKey, values := range header {
		if strings.EqualFold(headerKey, key) && len(values) > 0 {
			return values[0]
		}
	}
	return ""
}

// SetProtocol pins the protocol mode once.
func (s *RequestState) SetProtocol(mode ProtocolMode) error {
	if s.ProtocolMode != "" {
		return nil
	}
	s.ProtocolMode = mode
	return nil
}

// PinSnapshot pins the snapshot once for request lifetime.
func (s *RequestState) PinSnapshot(snapshot SnapshotView) {
	if s.Snapshot != nil {
		return
	}
	s.Snapshot = snapshot
	s.SnapshotRef = snapshot.Ref()
}

// Cleanup closes request resources owned by state.
func (s *RequestState) Cleanup() {
	if s == nil || s.Incoming.Body == nil {
		return
	}
	_ = s.Incoming.Body.Close()
}

// AddLimitRelease records a lease that should be released before request or stream end.
func (s *RequestState) AddLimitRelease(release LimitRelease) {
	if release == nil {
		return
	}
	s.LimitReleases = append(s.LimitReleases, release)
}

// DrainLimitReleases transfers lease ownership to a later close finalizer.
func (s *RequestState) DrainLimitReleases() []LimitRelease {
	if s == nil || len(s.LimitReleases) == 0 {
		return nil
	}
	releases := append([]LimitRelease(nil), s.LimitReleases...)
	s.LimitReleases = nil
	return releases
}

func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(b[:])
}
