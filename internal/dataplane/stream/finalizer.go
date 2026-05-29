package stream

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	"github.com/KnifeFly/token-gateway/internal/provider/relay"
	"github.com/KnifeFly/token-gateway/pkg/tokenusage"
)

// Finalizer wraps provider streams and performs close-time settlement.
type Finalizer struct {
	settlement engine.SettlementService
	observe    engine.ObserveRecorder
}

// NewFinalizer returns a stream finalizer.
func NewFinalizer(settlement engine.SettlementService, observe engine.ObserveRecorder) *Finalizer {
	if settlement == nil {
		settlement = engine.NoopSettlement{}
	}
	if observe == nil {
		observe = engine.NoopObserveRecorder{}
	}
	return &Finalizer{settlement: settlement, observe: observe}
}

// Wrap replaces the provider stream with an accounting stream.
func (f *Finalizer) Wrap(_ context.Context, state *engine.RequestState, result *engine.ProviderResult) (*engine.GatewayResponse, error) {
	if result == nil || result.Response == nil || result.Response.Stream == nil {
		return nil, errors.New("stream response is required")
	}
	result.Response.Stream = &AccountingStream{
		source:     result.Response.Stream,
		state:      state,
		settlement: f.settlement,
		startedAt:  time.Now(),
	}
	return result.Response, nil
}

// AccountingStream tracks usage and settles exactly once at close.
type AccountingStream struct {
	source     relay.ProviderStream
	state      *engine.RequestState
	settlement engine.SettlementService
	startedAt  time.Time
	once       sync.Once
	closeErr   error
	chunks     int64
	bytes      int64
	firstToken time.Duration
	downstream error
}

// Recv returns the next upstream chunk.
func (s *AccountingStream) Recv(ctx context.Context) ([]byte, error) {
	chunk, err := s.source.Recv(ctx)
	if len(chunk) > 0 {
		if s.chunks == 0 {
			s.firstToken = time.Since(s.startedAt)
		}
		s.chunks++
		s.bytes += int64(len(chunk))
	}
	return chunk, err
}

// Usage returns the current final stream usage estimate.
func (s *AccountingStream) Usage() (usage tokenusage.Actual) {
	return s.source.Usage()
}

// ReportDownstreamError records a client-side stream write failure.
func (s *AccountingStream) ReportDownstreamError(err error) {
	if err == nil {
		return
	}
	s.downstream = err
	if s.state != nil {
		ensureInternal(s.state)
		s.state.Internal["stream_downstream_error"] = ClassifyDownstreamError(err)
	}
}

// Close closes the provider stream and settles the request once.
func (s *AccountingStream) Close() error {
	s.once.Do(func() {
		// Step 1: close upstream first so provider-owned resources are released.
		sourceErr := s.source.Close()
		usage := s.source.Usage()

		// Step 2: fall back to a byte-derived estimate when the stream has no usage event.
		if usage.TotalTokens == 0 && s.bytes > 0 {
			usage.InputTokens = s.state.EstimatedUsage.InputTokens
			usage.OutputTokens = s.bytes / 8
			if usage.OutputTokens == 0 {
				usage.OutputTokens = 1
			}
			usage.TotalTokens = usage.InputTokens + usage.OutputTokens
		}

		// Step 3: attach safe stream metadata before final settlement.
		s.state.ActualUsage = usage
		ensureInternal(s.state)
		s.state.Internal["stream_chunks"] = s.chunks
		s.state.Internal["stream_upstream_bytes"] = s.bytes
		if s.firstToken > 0 {
			s.state.Internal["stream_first_token_latency_ms"] = s.firstToken.Milliseconds()
		}
		if s.downstream != nil {
			s.state.Internal["stream_downstream_error"] = ClassifyDownstreamError(s.downstream)
		}

		// Step 4: persist settlement once with the final stream usage.
		if settleErr := s.settlement.Settle(context.Background(), s.state); settleErr != nil {
			recordErr := s.settlement.RecordFailed(context.Background(), s.state, settleErr)
			s.closeErr = errors.Join(sourceErr, settleErr, recordErr)
			return
		}
		s.closeErr = sourceErr
	})
	return s.closeErr
}

// ClassifyDownstreamError converts local stream write errors into safe classes.
func ClassifyDownstreamError(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, net.ErrClosed) {
		return "client_disconnected"
	}
	return "downstream_write_failed"
}

func ensureInternal(state *engine.RequestState) {
	if state != nil && state.Internal == nil {
		state.Internal = make(map[string]any)
	}
}
