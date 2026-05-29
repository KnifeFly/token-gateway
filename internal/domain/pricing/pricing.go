package pricing

import (
	"github.com/KnifeFly/token-gateway/pkg/money"
	"github.com/KnifeFly/token-gateway/pkg/tokenusage"
)

// TokenPrice stores customer-facing token prices in micros per token.
type TokenPrice struct {
	Currency             string
	InputMicrosPerToken  int64
	OutputMicrosPerToken int64
}

// QuoteEstimate quotes a pre-provider usage estimate.
func (p TokenPrice) QuoteEstimate(usage tokenusage.Estimate) money.Amount {
	return money.New(p.Currency, usage.InputTokens*p.InputMicrosPerToken+usage.OutputTokens*p.OutputMicrosPerToken)
}

// QuoteActual quotes provider-reported usage.
func (p TokenPrice) QuoteActual(usage tokenusage.Actual) money.Amount {
	return money.New(p.Currency, usage.InputTokens*p.InputMicrosPerToken+usage.OutputTokens*p.OutputMicrosPerToken)
}
