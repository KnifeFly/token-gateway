package realtime

import (
	"strings"
	"time"

	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

// SessionStatus describes the lifecycle state reserved for future realtime work.
type SessionStatus string

const (
	SessionStatusPending SessionStatus = "pending"
	SessionStatusActive  SessionStatus = "active"
	SessionStatusExpired SessionStatus = "expired"
	SessionStatusClosed  SessionStatus = "closed"
)

// SessionRequest is the authenticated, tenant-scoped realtime session input.
type SessionRequest struct {
	TenantID          string
	ProjectID         string
	APIKeyID          string
	RequestID         string
	TraceID           string
	Model             string
	Modalities        []string
	Voice             string
	Instructions      string
	InputAudioFormat  string
	OutputAudioFormat string
	Metadata          map[string]any
	ExpiresIn         time.Duration
}

// Validate rejects malformed realtime session requests before engine dispatch.
func (r SessionRequest) Validate() error {
	if strings.TrimSpace(r.Model) == "" {
		return apperr.InvalidArgument("model is required")
	}
	return nil
}

// Session is the customer-visible realtime session contract.
type Session struct {
	ID           string
	Object       string
	TenantID     string
	ProjectID    string
	APIKeyID     string
	Model        string
	Status       SessionStatus
	ClientSecret string
	ExpiresAt    time.Time
	CreatedAt    time.Time
	WebSocketURL string
	WebRTCURL    string
	Metadata     map[string]any
	SnapshotRef  string
	RequestID    string
	TraceID      string
}
