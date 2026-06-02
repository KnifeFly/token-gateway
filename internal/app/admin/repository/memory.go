package repository

import (
	"sync"

	adminapp "github.com/KnifeFly/token-gateway/internal/app/admin"
)

// MemoryRepository stores Admin operators, sessions, and audit events for tests and local development.
type MemoryRepository struct {
	mu        sync.RWMutex
	operators map[string]adminapp.Operator
	sessions  map[string]adminapp.Session
	audit     map[string]adminapp.AuditEvent
}

// NewMemoryRepository returns an empty Admin app repository.
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		operators: map[string]adminapp.Operator{},
		sessions:  map[string]adminapp.Session{},
		audit:     map[string]adminapp.AuditEvent{},
	}
}
