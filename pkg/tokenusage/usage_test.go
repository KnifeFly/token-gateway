package tokenusage

import "testing"

func TestMergePreservesStreamUsageCounters(t *testing.T) {
	base := Actual{InputTokens: 10, CachedInputTokens: 2}
	update := Actual{OutputTokens: 4, ReasoningTokens: 1}

	got := Merge(base, update)
	if got.InputTokens != 10 || got.OutputTokens != 4 || got.TotalTokens != 14 {
		t.Fatalf("usage = %#v", got)
	}
	if got.CachedInputTokens != 2 || got.ReasoningTokens != 1 {
		t.Fatalf("detail usage = %#v", got)
	}
}
