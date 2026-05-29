package tokenusage

// Estimate is the gateway's pre-provider token estimate.
type Estimate struct {
	InputTokens  int64
	OutputTokens int64
}

// Actual is provider-reported token usage after a request completes.
type Actual struct {
	InputTokens  int64
	OutputTokens int64
	TotalTokens  int64
}

// EstimateFromBytes returns a conservative, deterministic estimate for M1.
func EstimateFromBytes(body []byte) Estimate {
	tokens := int64(len(body) / 4)
	if tokens == 0 && len(body) > 0 {
		tokens = 1
	}
	return Estimate{InputTokens: tokens}
}
