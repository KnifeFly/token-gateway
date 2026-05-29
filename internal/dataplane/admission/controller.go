package admission

import (
	"context"
	"time"

	"github.com/KnifeFly/token-gateway/internal/billing"
	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	"github.com/KnifeFly/token-gateway/internal/domain/pricing"
	"github.com/KnifeFly/token-gateway/pkg/money"
)

// PriceEstimator quotes the request before provider dispatch.
type PriceEstimator struct {
	price               pricing.TokenPrice
	defaultOutputTokens int64
}

func NewPriceEstimator(price pricing.TokenPrice, defaultOutputTokens int64) *PriceEstimator {
	return &PriceEstimator{price: price, defaultOutputTokens: defaultOutputTokens}
}

func (e *PriceEstimator) Estimate(state *engine.RequestState) money.Amount {
	estimate := state.EstimatedUsage
	if estimate.OutputTokens == 0 {
		estimate.OutputTokens = e.defaultOutputTokens
	}
	state.EstimatedUsage = estimate
	return e.price.QuoteEstimate(estimate)
}

// Controller reserves balance before provider calls.
type Controller struct {
	balance *billing.BalanceService
	quoter  *PriceEstimator
	holdTTL time.Duration
}

func NewController(balance *billing.BalanceService, quoter *PriceEstimator, holdTTL time.Duration) *Controller {
	if holdTTL <= 0 {
		holdTTL = 10 * time.Minute
	}
	return &Controller{balance: balance, quoter: quoter, holdTTL: holdTTL}
}

func (c *Controller) Reserve(ctx context.Context, state *engine.RequestState) error {
	if c == nil || c.balance == nil || c.quoter == nil {
		return nil
	}
	amount := c.quoter.Estimate(state)
	state.Currency = amount.Currency
	state.EstimatedChargeMicros = amount.Micros
	hold, err := c.balance.CreateHold(ctx, billing.HoldRequest{
		RequestID:    state.RequestID,
		TenantID:     state.TenantID,
		ProjectID:    state.ProjectID,
		APIKeyID:     state.APIKeyID,
		Currency:     amount.Currency,
		AmountMicros: amount.Micros,
		ExpiresAt:    time.Now().UTC().Add(c.holdTTL),
	})
	if err != nil {
		return err
	}
	state.BalanceHoldID = hold.ID
	state.BalanceAccountID = hold.AccountID
	return nil
}

func (c *Controller) Release(ctx context.Context, state *engine.RequestState, cause error) error {
	if c == nil || c.balance == nil || state.BalanceHoldID == "" {
		return nil
	}
	reason := "released"
	if cause != nil {
		reason = cause.Error()
	}
	return c.balance.ReleaseHold(ctx, state.BalanceHoldID, reason)
}
