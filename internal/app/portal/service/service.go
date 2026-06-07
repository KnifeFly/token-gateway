package service

import (
	"time"

	portalapp "github.com/KnifeFly/token-gateway/internal/app/portal"
	"github.com/KnifeFly/token-gateway/internal/billing/reporting"
	"github.com/KnifeFly/token-gateway/internal/controlplane/configadmin"
	tasksvc "github.com/KnifeFly/token-gateway/internal/task"
)

const (
	defaultSessionTTL  = 12 * time.Hour
	sessionIDBytes     = 32
	csrfTokenBytes     = 32
	defaultPortalLimit = 50
	maxPortalLimit     = 200
)

// Service coordinates browser-facing Portal Web BFF use cases.
type Service struct {
	snapshot  SnapshotProvider
	auth      Authenticator
	admin     *configadmin.Service
	reporting *reporting.Service
	tasks     tasksvc.Repository
	snapshots SnapshotRefresher
	sessions  portalapp.SessionStore
	now       func() time.Time
	ttl       time.Duration
}

// Option customizes the Portal Web service.
type Option func(*Service)

// WithClock configures a deterministic clock for tests.
func WithClock(now func() time.Time) Option {
	return func(s *Service) {
		if now != nil {
			s.now = now
		}
	}
}

// WithSessionTTL configures browser session lifetime.
func WithSessionTTL(ttl time.Duration) Option {
	return func(s *Service) {
		if ttl > 0 {
			s.ttl = ttl
		}
	}
}

// WithSnapshotRefresher publishes and swaps the runtime snapshot after mutable Portal changes.
func WithSnapshotRefresher(refresher SnapshotRefresher) Option {
	return func(s *Service) {
		s.snapshots = refresher
	}
}

// New returns a Portal Web BFF service.
func New(snapshot SnapshotProvider, auth Authenticator, adminService *configadmin.Service, reportingService *reporting.Service, tasks tasksvc.Repository, sessions portalapp.SessionStore, opts ...Option) *Service {
	s := &Service{
		snapshot:  snapshot,
		auth:      auth,
		admin:     adminService,
		reporting: reportingService,
		tasks:     tasks,
		sessions:  sessions,
		now:       func() time.Time { return time.Now().UTC() },
		ttl:       defaultSessionTTL,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(s)
		}
	}
	return s
}
