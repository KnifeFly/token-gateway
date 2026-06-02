package service

import (
	"context"
	"strings"

	adminapp "github.com/KnifeFly/token-gateway/internal/app/admin"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

// ListOperators returns safe Admin operator metadata.
func (s *Service) ListOperators(ctx context.Context, actor adminapp.Actor) (adminapp.ListResponse[adminapp.Operator], error) {
	if err := s.Authorize(actor, "read", "operator"); err != nil {
		return adminapp.ListResponse[adminapp.Operator]{}, err
	}
	operators, err := s.repo.ListOperators(ctx)
	return adminapp.ListResponse[adminapp.Operator]{Data: operators}, err
}

// CreateOperator creates an Admin operator and audits the mutation.
func (s *Service) CreateOperator(ctx context.Context, actor adminapp.Actor, request adminapp.OperatorCreateRequest, opts adminapp.MutationOptions) (adminapp.Operator, error) {
	return mutate(ctx, s, actor, opts, "write", "operator", "", request, func() (adminapp.Operator, error) {
		email := normalizeEmail(request.Email)
		if email == "" || strings.TrimSpace(request.Password) == "" {
			return adminapp.Operator{}, apperr.InvalidArgument("email and password are required")
		}
		if _, ok, err := s.repo.GetOperatorByEmail(ctx, email); err != nil || ok {
			if ok {
				return adminapp.Operator{}, apperr.InvalidArgument("operator email already exists")
			}
			return adminapp.Operator{}, err
		}
		enabled := request.Enabled
		if !enabled {
			enabled = true
		}
		now := s.now()
		operator := adminapp.Operator{
			ID:           newID("operator"),
			Email:        email,
			DisplayName:  strings.TrimSpace(request.DisplayName),
			PasswordHash: HashPassword(request.Password),
			Roles:        normalizeRoles(request.Roles),
			Enabled:      enabled,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if len(operator.Roles) == 0 {
			operator.Roles = []adminapp.Role{adminapp.RoleReadOnly}
		}
		return s.repo.SaveOperator(ctx, operator)
	})
}

// DisableOperator disables an Admin operator and audits the mutation.
func (s *Service) DisableOperator(ctx context.Context, actor adminapp.Actor, operatorID string, opts adminapp.MutationOptions) (adminapp.Operator, error) {
	return mutate(ctx, s, actor, opts, "write", "operator", operatorID, map[string]string{"id": operatorID}, func() (adminapp.Operator, error) {
		if operatorID == actor.OperatorID {
			return adminapp.Operator{}, apperr.InvalidArgument("operators cannot disable themselves")
		}
		operator, ok, err := s.repo.DisableOperator(ctx, operatorID, s.now())
		if err != nil {
			return adminapp.Operator{}, err
		}
		if !ok {
			return adminapp.Operator{}, apperr.NotFound("operator not found")
		}
		return operator, nil
	})
}
