// Package service coordinates browser-facing Admin Web BFF use cases.
package service

import (
	"context"
	"time"

	adminapp "github.com/KnifeFly/token-gateway/internal/app/admin"
	"github.com/KnifeFly/token-gateway/internal/billing"
	"github.com/KnifeFly/token-gateway/internal/billing/reporting"
	"github.com/KnifeFly/token-gateway/internal/controlplane/configadmin"
	cpsnapshot "github.com/KnifeFly/token-gateway/internal/controlplane/snapshot"
	tasksvc "github.com/KnifeFly/token-gateway/internal/task"
)

const (
	defaultSessionTTL = 12 * time.Hour
	sessionIDBytes    = 32
	csrfTokenBytes    = 32
	auditStatusOK     = "success"
	auditStatusFailed = "failed"
)

// SnapshotManager publishes, rolls back, and diagnoses runtime snapshots through the owner service.
type SnapshotManager interface {
	Publish(ctx context.Context) (*cpsnapshot.RuntimeSnapshot, error)
	Rollback(ctx context.Context) (*cpsnapshot.RuntimeSnapshot, error)
	Diagnostics(ctx context.Context) (*cpsnapshot.Diagnostics, error)
}

// Service coordinates Admin Web BFF authorization, audit, and owner-service workflows.
type Service struct {
	repo              adminapp.Repository
	owner             *configadmin.Service
	commercial        *reporting.Service
	tasks             tasksvc.Repository
	failedSettlements *billing.FailedSettlementService
	snapshots         SnapshotManager
	now               func() time.Time
	ttl               time.Duration
}

// Option customizes the Admin Web service.
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

// WithCommercialReporting attaches commercial reporting read models.
func WithCommercialReporting(commercial *reporting.Service) Option {
	return func(s *Service) {
		s.commercial = commercial
	}
}

// WithTaskRepository attaches task and callback read models.
func WithTaskRepository(tasks tasksvc.Repository) Option {
	return func(s *Service) {
		s.tasks = tasks
	}
}

// WithFailedSettlementService attaches the owner repair workflow.
func WithFailedSettlementService(failedSettlements *billing.FailedSettlementService) Option {
	return func(s *Service) {
		s.failedSettlements = failedSettlements
	}
}

// WithSnapshotManager attaches the owner snapshot workflow.
func WithSnapshotManager(snapshots SnapshotManager) Option {
	return func(s *Service) {
		s.snapshots = snapshots
	}
}

// New returns an Admin Web BFF application service.
func New(repo adminapp.Repository, owner *configadmin.Service, opts ...Option) *Service {
	s := &Service{
		repo:  repo,
		owner: owner,
		now:   func() time.Time { return time.Now().UTC() },
		ttl:   defaultSessionTTL,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(s)
		}
	}
	return s
}
