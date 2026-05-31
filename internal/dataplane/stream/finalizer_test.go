package stream

import (
	"context"
	"io"
	"sync/atomic"
	"testing"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	"github.com/KnifeFly/token-gateway/internal/provider/relay"
	"github.com/KnifeFly/token-gateway/pkg/tokenusage"
)

func TestAccountingStreamSettlesOnClose(t *testing.T) {
	settlement := &fakeSettlement{}
	state := &engine.RequestState{RequestID: "req_stream"}
	result := &engine.ProviderResult{
		Response: &engine.GatewayResponse{
			Stream: &relay.StaticStream{
				Chunks: [][]byte{[]byte("data: hello\n\n")},
				Actual: tokenusage.Actual{InputTokens: 3, OutputTokens: 4, TotalTokens: 7},
			},
		},
	}

	response, err := NewFinalizer(settlement, nil).Wrap(context.Background(), state, result)
	if err != nil {
		t.Fatalf("Wrap() error = %v", err)
	}
	if _, err := response.Stream.Recv(context.Background()); err != nil {
		t.Fatalf("Recv() error = %v", err)
	}
	if _, err := response.Stream.Recv(context.Background()); err != io.EOF {
		t.Fatalf("Recv() err = %v, want EOF", err)
	}
	if reporter, ok := response.Stream.(*AccountingStream); ok {
		reporter.ReportDownstreamError(context.Canceled)
	}
	if err := response.Stream.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := response.Stream.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
	if settlement.calls != 1 {
		t.Fatalf("settle calls = %d", settlement.calls)
	}
	if settlement.usage.TotalTokens != 7 {
		t.Fatalf("settled usage = %#v", settlement.usage)
	}
	if got := state.Internal["stream_chunks"]; got != int64(1) {
		t.Fatalf("stream_chunks = %#v", got)
	}
	if got := state.Internal["stream_downstream_error"]; got != "client_disconnected" {
		t.Fatalf("stream_downstream_error = %#v", got)
	}
}

func TestAccountingStreamReleasesLimitLeasesAfterClose(t *testing.T) {
	settlement := &fakeSettlement{}
	release := &countingRelease{}
	state := &engine.RequestState{RequestID: "req_stream"}
	state.AddLimitRelease(release)
	result := &engine.ProviderResult{
		Response: &engine.GatewayResponse{
			Stream: &relay.StaticStream{
				Chunks: [][]byte{[]byte("data: hello\n\n")},
				Actual: tokenusage.Actual{InputTokens: 1, OutputTokens: 1, TotalTokens: 2},
			},
		},
	}

	response, err := NewFinalizer(settlement, nil).Wrap(context.Background(), state, result)
	if err != nil {
		t.Fatalf("Wrap() error = %v", err)
	}
	if len(state.LimitReleases) != 0 {
		t.Fatalf("state still owns limit releases = %d", len(state.LimitReleases))
	}
	if release.calls.Load() != 0 {
		t.Fatalf("release before close = %d", release.calls.Load())
	}

	if err := response.Stream.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if release.calls.Load() != 1 {
		t.Fatalf("release calls = %d, want 1", release.calls.Load())
	}
	if err := response.Stream.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
	if release.calls.Load() != 1 {
		t.Fatalf("release calls after second close = %d, want 1", release.calls.Load())
	}
}

type fakeSettlement struct {
	calls int
	usage tokenusage.Actual
}

func (s *fakeSettlement) Settle(_ context.Context, state *engine.RequestState) error {
	s.calls++
	s.usage = state.ActualUsage
	return nil
}

func (s *fakeSettlement) RecordFailed(context.Context, *engine.RequestState, error) error {
	return nil
}

type countingRelease struct {
	calls atomic.Int64
}

func (r *countingRelease) Release(context.Context) error {
	r.calls.Add(1)
	return nil
}
