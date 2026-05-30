package tokenusage

// Estimate is the gateway's pre-provider token estimate.
type Estimate struct {
	InputTokens  int64
	OutputTokens int64
}

// Actual is provider-reported token usage after a request completes.
type Actual struct {
	InputTokens              int64
	OutputTokens             int64
	TotalTokens              int64
	CachedInputTokens        int64
	CacheCreationInputTokens int64
	CacheReadInputTokens     int64
	ReasoningTokens          int64
	AudioInputTokens         int64
	AudioOutputTokens        int64
	ImageInputTokens         int64
	ImageOutputTokens        int64
	VideoInputTokens         int64
	VideoOutputTokens        int64
}

// EstimateFromBytes returns a conservative, deterministic estimate for M1.
func EstimateFromBytes(body []byte) Estimate {
	tokens := int64(len(body) / 4)
	if tokens == 0 && len(body) > 0 {
		tokens = 1
	}
	return Estimate{InputTokens: tokens}
}

// Merge overlays provider usage updates while preserving earlier stream counters.
func Merge(base Actual, update Actual) Actual {
	inputChanged := false
	outputChanged := false
	if update.InputTokens != 0 {
		base.InputTokens = update.InputTokens
		inputChanged = true
	}
	if update.OutputTokens != 0 {
		base.OutputTokens = update.OutputTokens
		outputChanged = true
	}
	if update.TotalTokens != 0 {
		base.TotalTokens = update.TotalTokens
	}
	if update.CachedInputTokens != 0 {
		base.CachedInputTokens = update.CachedInputTokens
	}
	if update.CacheCreationInputTokens != 0 {
		base.CacheCreationInputTokens = update.CacheCreationInputTokens
	}
	if update.CacheReadInputTokens != 0 {
		base.CacheReadInputTokens = update.CacheReadInputTokens
	}
	if update.ReasoningTokens != 0 {
		base.ReasoningTokens = update.ReasoningTokens
	}
	if update.AudioInputTokens != 0 {
		base.AudioInputTokens = update.AudioInputTokens
	}
	if update.AudioOutputTokens != 0 {
		base.AudioOutputTokens = update.AudioOutputTokens
	}
	if update.ImageInputTokens != 0 {
		base.ImageInputTokens = update.ImageInputTokens
	}
	if update.ImageOutputTokens != 0 {
		base.ImageOutputTokens = update.ImageOutputTokens
	}
	if update.VideoInputTokens != 0 {
		base.VideoInputTokens = update.VideoInputTokens
	}
	if update.VideoOutputTokens != 0 {
		base.VideoOutputTokens = update.VideoOutputTokens
	}
	if base.TotalTokens == 0 && (base.InputTokens != 0 || base.OutputTokens != 0) {
		base.TotalTokens = base.InputTokens + base.OutputTokens
	}
	if (inputChanged || outputChanged) && base.InputTokens != 0 && base.OutputTokens != 0 &&
		(update.TotalTokens == 0 || update.InputTokens == 0 || update.OutputTokens == 0) {
		base.TotalTokens = base.InputTokens + base.OutputTokens
	}
	return base
}
