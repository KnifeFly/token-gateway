package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/KnifeFly/token-gateway/pkg/apperr"
	"go.opentelemetry.io/otel/attribute"
)

// GatewayEngine coordinates the data-plane request lifecycle.
type GatewayEngine struct {
	snapshot   SnapshotProvider
	classifier APIClassifier
	parser     RequestParser
	auth       Authenticator
	router     RoutePlanner
	admission  AdmissionController
	limiter    LimitEnforcer
	dispatcher ProviderDispatcher
	stream     StreamFinalizer
	settlement SettlementService
	observe    ObserveRecorder
}

// Option mutates GatewayEngine during construction.
type Option func(*GatewayEngine)

// WithSnapshot configures the snapshot provider.
func WithSnapshot(provider SnapshotProvider) Option {
	return func(e *GatewayEngine) { e.snapshot = provider }
}

// WithClassifier configures the API classifier.
func WithClassifier(classifier APIClassifier) Option {
	return func(e *GatewayEngine) { e.classifier = classifier }
}

// WithParser configures the request parser.
func WithParser(parser RequestParser) Option {
	return func(e *GatewayEngine) { e.parser = parser }
}

// WithAuthenticator configures the API key authenticator.
func WithAuthenticator(auth Authenticator) Option {
	return func(e *GatewayEngine) { e.auth = auth }
}

// WithRoutePlanner configures the route planner.
func WithRoutePlanner(router RoutePlanner) Option {
	return func(e *GatewayEngine) { e.router = router }
}

// WithDispatcher configures the provider dispatcher.
func WithDispatcher(dispatcher ProviderDispatcher) Option {
	return func(e *GatewayEngine) { e.dispatcher = dispatcher }
}

// WithAdmission configures admission control.
func WithAdmission(admission AdmissionController) Option {
	return func(e *GatewayEngine) { e.admission = admission }
}

// WithLimitEnforcer configures request limit enforcement.
func WithLimitEnforcer(limiter LimitEnforcer) Option {
	return func(e *GatewayEngine) { e.limiter = limiter }
}

// WithSettlement configures final request settlement.
func WithSettlement(settlement SettlementService) Option {
	return func(e *GatewayEngine) { e.settlement = settlement }
}

// WithStreamFinalizer configures stream close-time finalization.
func WithStreamFinalizer(stream StreamFinalizer) Option {
	return func(e *GatewayEngine) { e.stream = stream }
}

// WithObserveRecorder configures metrics, traces, and logs.
func WithObserveRecorder(observe ObserveRecorder) Option {
	return func(e *GatewayEngine) { e.observe = observe }
}

// New constructs a GatewayEngine.
func New(opts ...Option) (*GatewayEngine, error) {
	e := &GatewayEngine{}
	for _, opt := range opts {
		opt(e)
	}
	if e.settlement == nil {
		e.settlement = NoopSettlement{}
	}
	if e.admission == nil {
		e.admission = NoopAdmission{}
	}
	if e.limiter == nil {
		e.limiter = NoopLimitEnforcer{}
	}
	if e.stream == nil {
		e.stream = NoopStreamFinalizer{}
	}
	if e.observe == nil {
		e.observe = NoopObserveRecorder{}
	}
	if err := e.validate(); err != nil {
		return nil, err
	}
	return e, nil
}

// Handle runs the GatewayEngine lifecycle.
func (e *GatewayEngine) Handle(ctx context.Context, req IncomingRequest) (*GatewayResponse, error) {
	state := newState(req)
	defer state.Cleanup()

	var response *GatewayResponse
	var err error
	defer func() {
		e.observe.FinishRequest(ctx, state, response, err)
	}()

	// Step 1: classify and parse before attaching the runtime snapshot.
	if err = e.classifier.Classify(ctx, state); err != nil {
		response = e.errorResponse(state, err)
		return response, nil
	}
	if err = e.parser.Parse(ctx, state); err != nil {
		response = e.errorResponse(state, err)
		return response, nil
	}

	// Step 2: attach snapshot state and authenticate the caller.
	if err = e.snapshot.Attach(ctx, state); err != nil {
		response = e.errorResponse(state, err)
		return response, nil
	}
	if err = e.auth.Authenticate(ctx, state); err != nil {
		response = e.errorResponse(state, err)
		return response, nil
	}

	// Step 3: resolve routing and reserve local limits before provider dispatch.
	ctx, span := e.observe.StartSpan(ctx, "gateway.route",
		attribute.String("gateway.request_id", state.RequestID),
		attribute.String("gateway.model", state.RequestedModel),
	)
	if err = e.router.Plan(ctx, state); err != nil {
		span.RecordError(err)
		span.End()
		response = e.errorResponse(state, err)
		return response, nil
	}
	span.End()

	if err = e.admission.Reserve(ctx, state); err != nil {
		response = e.errorResponse(state, err)
		return response, nil
	}
	release, limitErr := e.limiter.Acquire(ctx, state)
	if limitErr != nil {
		_ = e.admission.Release(ctx, state, limitErr)
		err = limitErr
		response = e.errorResponse(state, err)
		return response, nil
	}
	state.AddLimitRelease(release)
	defer e.releaseLimits(ctx, state)

	// Step 4: dispatch to the selected provider and finalize billing.
	result, dispatchErr := e.dispatcher.Dispatch(ctx, state)
	if dispatchErr != nil {
		_ = e.admission.Release(ctx, state, dispatchErr)
		err = dispatchErr
		response = e.errorResponse(state, err)
		return response, nil
	}
	state.ProviderResult = result
	state.ActualUsage = result.Usage
	if result.Response != nil && result.Response.Stream != nil {
		response, err = e.stream.Wrap(ctx, state, result)
		if err != nil {
			response = e.errorResponse(state, err)
			return response, nil
		}
		return response, nil
	}
	if err = e.settlement.Settle(ctx, state); err != nil {
		recordErr := e.settlement.RecordFailed(ctx, state, err)
		if recordErr == nil {
			response = result.Response
			return response, nil
		}
		err = errors.Join(err, recordErr)
		response = e.errorResponse(state, err)
		return response, nil
	}
	response = result.Response
	return response, nil
}

func (e *GatewayEngine) validate() error {
	var errs []error
	if e.snapshot == nil {
		errs = append(errs, errors.New("snapshot provider is required"))
	}
	if e.classifier == nil {
		errs = append(errs, errors.New("classifier is required"))
	}
	if e.parser == nil {
		errs = append(errs, errors.New("parser is required"))
	}
	if e.auth == nil {
		errs = append(errs, errors.New("authenticator is required"))
	}
	if e.router == nil {
		errs = append(errs, errors.New("route planner is required"))
	}
	if e.dispatcher == nil {
		errs = append(errs, errors.New("provider dispatcher is required"))
	}
	if e.admission == nil {
		errs = append(errs, errors.New("admission controller is required"))
	}
	if e.limiter == nil {
		errs = append(errs, errors.New("limit enforcer is required"))
	}
	if e.stream == nil {
		errs = append(errs, errors.New("stream finalizer is required"))
	}
	return errors.Join(errs...)
}

func (e *GatewayEngine) releaseLimits(ctx context.Context, state *RequestState) {
	for i := len(state.LimitReleases) - 1; i >= 0; i-- {
		if state.LimitReleases[i] == nil {
			continue
		}
		_ = state.LimitReleases[i].Release(ctx)
	}
}

func (e *GatewayEngine) errorResponse(state *RequestState, err error) *GatewayResponse {
	status := http.StatusInternalServerError
	code := "service_error"
	message := "internal error"
	retryable := false
	errType := "service_error"
	if appErr, ok := apperr.As(err); ok {
		status = appErr.HTTPStatus
		code = externalCode(appErr.Code)
		message = appErr.SafeMessage()
		retryable = appErr.Temporary
		errType = externalType(appErr.Code)
	}
	body, marshalErr := json.Marshal(map[string]any{
		"error": map[string]any{
			"code":       code,
			"message":    message,
			"type":       errType,
			"request_id": state.RequestID,
			"retryable":  retryable,
		},
	})
	if marshalErr != nil {
		body = []byte(fmt.Sprintf(`{"error":{"code":"service_error","message":"internal error","type":"service_error","request_id":%q,"retryable":false}}`, state.RequestID))
	}
	return &GatewayResponse{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       append(body, '\n'),
	}
}

func externalCode(code apperr.Code) string {
	switch code {
	case apperr.CodeInvalidArgument:
		return "invalid_request"
	case apperr.CodeAmbiguousProtocol:
		return "ambiguous_protocol"
	case apperr.CodeNotFound:
		return "resource_not_found"
	case apperr.CodeConfigUnavailable, apperr.CodeServiceUnavailable, apperr.CodeSnapshotStale:
		return "service_unavailable"
	case apperr.CodeInternal:
		return "service_error"
	default:
		return string(code)
	}
}

func externalType(code apperr.Code) string {
	switch code {
	case apperr.CodeUnauthorized:
		return "authentication_error"
	case apperr.CodeForbidden:
		return "permission_error"
	case apperr.CodeProviderError:
		return "provider_error"
	case apperr.CodeRateLimited:
		return "rate_limit_error"
	case apperr.CodeInternal, apperr.CodeConfigUnavailable, apperr.CodeServiceUnavailable, apperr.CodeSnapshotStale:
		return "service_error"
	default:
		return "invalid_request_error"
	}
}
