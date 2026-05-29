package pricing

import (
	"testing"

	"github.com/KnifeFly/token-gateway/pkg/tokenusage"
)

func TestTokenPriceQuotesMicros(t *testing.T) {
	price := TokenPrice{Currency: "USD", InputMicrosPerToken: 2, OutputMicrosPerToken: 5}
	got := price.QuoteEstimate(tokenusage.Estimate{InputTokens: 10, OutputTokens: 3})
	if got.Currency != "USD" || got.Micros != 35 {
		t.Fatalf("amount = %#v", got)
	}
}
