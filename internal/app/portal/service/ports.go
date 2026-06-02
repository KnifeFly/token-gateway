package service

import (
	"context"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
)

// SnapshotProvider attaches a runtime snapshot to a request state.
type SnapshotProvider interface {
	Attach(context.Context, *engine.RequestState) error
}

// Authenticator authenticates a request state against a snapshot.
type Authenticator interface {
	Authenticate(context.Context, *engine.RequestState) error
}

// SnapshotRefresher activates customer-visible control-plane changes in the runtime snapshot.
type SnapshotRefresher interface {
	RefreshSnapshot(ctx context.Context) error
}
