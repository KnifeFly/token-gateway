package realtime

import (
	"context"

	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

// Engine reserves realtime session and connection behavior without binding a provider.
type Engine interface {
	CreateSession(ctx context.Context, req SessionRequest) (*Session, error)
	GetSession(ctx context.Context, tenantID, projectID, sessionID string) (*Session, error)
	HandleConnection(ctx context.Context, conn Connection) error
}

// Connection is the minimal WebSocket/WebRTC bridge reserved for a future realtime engine.
type Connection interface {
	SessionID() string
	Close(code int, reason string) error
}

// DisabledEngine explicitly rejects realtime calls while the feature is reserved.
type DisabledEngine struct{}

// CreateSession reports that realtime sessions are not enabled.
func (DisabledEngine) CreateSession(context.Context, SessionRequest) (*Session, error) {
	return nil, apperr.FeatureNotEnabled("realtime sessions are not enabled")
}

// GetSession reports that realtime sessions are not enabled.
func (DisabledEngine) GetSession(context.Context, string, string, string) (*Session, error) {
	return nil, apperr.FeatureNotEnabled("realtime sessions are not enabled")
}

// HandleConnection reports that realtime connections are not enabled.
func (DisabledEngine) HandleConnection(context.Context, Connection) error {
	return apperr.FeatureNotEnabled("realtime websocket is not enabled")
}
