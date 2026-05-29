package dispatch

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	"github.com/KnifeFly/token-gateway/internal/provider"
	"github.com/KnifeFly/token-gateway/internal/provider/relay"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
	"go.opentelemetry.io/otel/attribute"
)

// Dispatcher calls registered provider adapters and records attempts.
type Dispatcher struct {
	registry *provider.Registry
	observe  engine.ObserveRecorder
	logger   *slog.Logger
}

func New(registry *provider.Registry, observe engine.ObserveRecorder, logger *slog.Logger) *Dispatcher {
	if logger == nil {
		logger = slog.Default()
	}
	if observe == nil {
		observe = engine.NoopObserveRecorder{}
	}
	return &Dispatcher{registry: registry, observe: observe, logger: logger}
}

func (d *Dispatcher) Dispatch(ctx context.Context, state *engine.RequestState) (*engine.ProviderResult, error) {
	if state.RoutePlan == nil || len(state.RoutePlan.Candidates) == 0 {
		return nil, apperr.ServiceUnavailable("no route is available", apperr.WithTemporary())
	}
	var lastErr error
	for _, candidate := range state.RoutePlan.Candidates {
		adapter, ok := d.registry.Adapter(candidate.ProviderType)
		if !ok {
			lastErr = apperr.ServiceUnavailable("provider adapter is unavailable", apperr.WithTemporary())
			continue
		}
		channel, ok := state.Snapshot.LookupChannel(candidate.ChannelID)
		if !ok || !channel.Enabled {
			lastErr = apperr.ServiceUnavailable("provider channel is unavailable", apperr.WithTemporary())
			continue
		}
		started := time.Now()
		spanCtx, span := d.observe.StartSpan(ctx, "gateway.provider_attempt",
			attribute.String("gateway.provider", candidate.ProviderType),
			attribute.String("gateway.channel_id", candidate.ChannelID),
			attribute.String("gateway.model", candidate.PublicModel),
		)
		response, err := adapter.ChatCompletions(spanCtx, relay.ChannelConfig{
			ChannelID:     candidate.ChannelID,
			ProviderType:  candidate.ProviderType,
			BaseURL:       channel.BaseURL,
			APIKey:        channel.APIKey,
			UpstreamModel: candidate.UpstreamModel,
			Timeout:       candidate.Timeout,
		}, relay.ChatCompletionRequest{
			PublicModel:   candidate.PublicModel,
			UpstreamModel: candidate.UpstreamModel,
			RawBody:       state.Parsed.RawBody,
			RequestID:     state.RequestID,
		})
		attempt := engine.ProviderAttempt{
			ChannelID:    candidate.ChannelID,
			ProviderType: candidate.ProviderType,
			PublicModel:  candidate.PublicModel,
			StartedAt:    started,
			Duration:     time.Since(started),
		}
		if err != nil {
			span.RecordError(err)
			lastErr = mapProviderError(err)
			attempt.ErrorCode = providerErrorCode(err)
			attempt.StatusCode = providerStatusCode(err)
			state.Attempts = append(state.Attempts, attempt)
			d.observe.RecordProviderAttempt(ctx, state, attempt)
			span.End()
			if !providerRetryable(err) {
				continue
			}
			continue
		}
		attempt.Success = true
		attempt.StatusCode = response.StatusCode
		state.Attempts = append(state.Attempts, attempt)
		d.observe.RecordProviderAttempt(ctx, state, attempt)
		span.End()
		return &engine.ProviderResult{
			Candidate: candidate,
			Response: &engine.GatewayResponse{
				StatusCode: response.StatusCode,
				Header:     response.Header,
				Body:       response.Body,
				Usage:      response.Usage,
			},
			Usage: response.Usage,
		}, nil
	}
	if lastErr == nil {
		lastErr = apperr.ServiceUnavailable("provider is unavailable", apperr.WithTemporary())
	}
	return nil, lastErr
}

func mapProviderError(err error) error {
	var providerErr *relay.ProviderError
	if !errors.As(err, &providerErr) {
		return apperr.ProviderError("provider request failed", apperr.WithCause(err), apperr.WithTemporary())
	}
	switch providerErr.StatusCode {
	case http.StatusTooManyRequests:
		return apperr.RateLimited("provider rate limited the request", apperr.WithCause(err), apperr.WithTemporary())
	case http.StatusUnauthorized, http.StatusForbidden:
		return apperr.ProviderError("provider authentication failed", apperr.WithCause(err))
	default:
		opts := []apperr.Option{apperr.WithCause(err)}
		if providerErr.Retryable {
			opts = append(opts, apperr.WithTemporary())
		}
		return apperr.ProviderError("provider request failed", opts...)
	}
}

func providerRetryable(err error) bool {
	var providerErr *relay.ProviderError
	return errors.As(err, &providerErr) && providerErr.Retryable
}

func providerErrorCode(err error) string {
	var providerErr *relay.ProviderError
	if errors.As(err, &providerErr) {
		return providerErr.Code
	}
	return "provider_error"
}

func providerStatusCode(err error) int {
	var providerErr *relay.ProviderError
	if errors.As(err, &providerErr) {
		return providerErr.StatusCode
	}
	return http.StatusBadGateway
}
