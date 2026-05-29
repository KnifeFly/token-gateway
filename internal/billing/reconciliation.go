package billing

import "context"

// ReconciliationService detects ledger and balance mismatches.
type ReconciliationService struct {
	repo Repository
}

func NewReconciliationService(repo Repository) *ReconciliationService {
	return &ReconciliationService{repo: repo}
}

func (s *ReconciliationService) FindIssues(ctx context.Context) ([]ReconciliationIssue, error) {
	if s == nil || s.repo == nil {
		return nil, nil
	}
	return s.repo.Reconcile(ctx)
}
