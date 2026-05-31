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

// engine.go keeps the request lifecycle readable: snapshot, auth, policy, route, dispatch, stream, and settlement.

// GatewayEngine coordinates the data-plane request lifecycle.
type GatewayEngine struct {
	snapshot   SnapshotProvider
	classifier APIClassifier
	parser     RequestParser
	auth       Authenticator
	policy     PolicyEvaluator
	router     RoutePlanner
	admission  AdmissionController
	limiter    LimitEnforcer
	dispatcher ProviderDispatcher
	stream     StreamFinalizer
	tasks      TaskBridge
	files      FileService
	settlement SettlementService
	plugins    PluginManager
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

// WithPolicyEvaluator configures explicit data-plane policy evaluation.
func WithPolicyEvaluator(policy PolicyEvaluator) Option {
	return func(e *GatewayEngine) { e.policy = policy }
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

// WithTaskBridge configures async task handling.
func WithTaskBridge(tasks TaskBridge) Option {
	return func(e *GatewayEngine) { e.tasks = tasks }
}

// WithFileService configures transient input asset metadata and quota handling.
func WithFileService(files FileService) Option {
	return func(e *GatewayEngine) { e.files = files }
}

// WithPluginManager configures data-plane plugins.
func WithPluginManager(plugins PluginManager) Option {
	return func(e *GatewayEngine) { e.plugins = plugins }
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
	if e.policy == nil {
		e.policy = NoopPolicyEvaluator{}
	}
	if e.stream == nil {
		e.stream = NoopStreamFinalizer{}
	}
	if e.tasks == nil {
		e.tasks = NoopTaskBridge{}
	}
	if e.files == nil {
		e.files = NoopFileService{}
	}
	if e.plugins == nil {
		e.plugins = NoopPluginManager{}
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
		_ = e.runStage(ctx, state, "gateway.audit", func(stageCtx context.Context) error {
			return e.plugins.Run(stageCtx, "audit", state)
		})
		addSnapshotHeader(response, state)
		e.observe.FinishRequest(ctx, state, response, err)
	}()

	if err = e.runStage(ctx, state, "gateway.receive", func(stageCtx context.Context) error {
		if err := e.snapshot.Attach(stageCtx, state); err != nil {
			return err
		}
		return e.plugins.Run(stageCtx, "pre_request", state)
	}); err != nil {
		response = e.errorResponse(state, err)
		return response, nil
	}

	// Step 1: classify and parse after pinning the runtime snapshot.
	if err = e.runStage(ctx, state, "gateway.classify", func(stageCtx context.Context) error {
		if err := e.classifier.Classify(stageCtx, state); err != nil {
			return err
		}
		return e.parser.Parse(stageCtx, state)
	}); err != nil {
		response = e.errorResponse(state, err)
		return response, nil
	}

	// Step 2: authenticate the caller against the pinned snapshot.
	if err = e.runStage(ctx, state, "gateway.auth", func(stageCtx context.Context) error {
		return e.auth.Authenticate(stageCtx, state)
	}); err != nil {
		response = e.errorResponse(state, err)
		return response, nil
	}
	if err = e.runStage(ctx, state, "gateway.policy", func(stageCtx context.Context) error {
		return e.plugins.Run(stageCtx, "post_auth", state)
	}); err != nil {
		response = e.errorResponse(state, err)
		return response, nil
	}
	if err = e.runStage(ctx, state, "gateway.policy.evaluate", func(stageCtx context.Context) error {
		return e.evaluatePolicy(stageCtx, state)
	}); err != nil {
		response = e.errorResponse(state, err)
		return response, nil
	}
	if state.IsTaskOperation() {
		response, err = e.tasks.HandleTaskOperation(ctx, state)
		if err != nil {
			response = e.errorResponse(state, err)
		}
		return response, nil
	}
	if state.IsFileOperation() {
		response, err = e.files.HandleFileOperation(ctx, state)
		if err != nil {
			response = e.errorResponse(state, err)
		}
		return response, nil
	}
	if state.Async {
		var hit bool
		response, hit, err = e.tasks.CheckIdempotency(ctx, state)
		if err != nil {
			response = e.errorResponse(state, err)
			return response, nil
		}
		if hit {
			state.Internal["idempotency_hit"] = true
			return response, nil
		}
	}
	if err = e.runStage(ctx, state, "gateway.plugin.pre_prompt", func(stageCtx context.Context) error {
		return e.plugins.Run(stageCtx, "pre_prompt", state)
	}); err != nil {
		response = e.errorResponse(state, err)
		return response, nil
	}

	// Step 3: resolve routing and reserve local limits before provider dispatch.
	if err = e.runStage(ctx, state, "gateway.policy", func(stageCtx context.Context) error {
		return e.plugins.Run(stageCtx, "pre_route", state)
	}); err != nil {
		response = e.errorResponse(state, err)
		return response, nil
	}
	if err = e.runStage(ctx, state, "gateway.policy.pre_route", func(stageCtx context.Context) error {
		return e.evaluatePolicy(stageCtx, state)
	}); err != nil {
		response = e.errorResponse(state, err)
		return response, nil
	}
	if err = e.runStage(ctx, state, "gateway.route", func(stageCtx context.Context) error {
		if state.RoutePlan != nil {
			return nil
		}
		return e.router.Plan(stageCtx, state)
	}); err != nil {
		response = e.errorResponse(state, err)
		return response, nil
	}
	if err = e.runStage(ctx, state, "gateway.policy", func(stageCtx context.Context) error {
		return e.plugins.Run(stageCtx, "post_route", state)
	}); err != nil {
		response = e.errorResponse(state, err)
		return response, nil
	}
	if err = e.runStage(ctx, state, "gateway.policy.post_route", func(stageCtx context.Context) error {
		if err := e.evaluatePolicy(stageCtx, state); err != nil {
			return err
		}
		if state.RoutePlan == nil {
			return e.router.Plan(stageCtx, state)
		}
		return nil
	}); err != nil {
		response = e.errorResponse(state, err)
		return response, nil
	}

	if err = e.runStage(ctx, state, "gateway.admission", func(stageCtx context.Context) error {
		return e.admission.Reserve(stageCtx, state)
	}); err != nil {
		response = e.errorResponse(state, err)
		return response, nil
	}
	var release LimitRelease
	if err = e.runStage(ctx, state, "gateway.limit", func(stageCtx context.Context) error {
		var acquireErr error
		release, acquireErr = e.limiter.Acquire(stageCtx, state)
		return acquireErr
	}); err != nil {
		_ = e.admission.Release(ctx, state, err)
		response = e.errorResponse(state, err)
		return response, nil
	}
	state.AddLimitRelease(release)
	defer e.releaseLimits(ctx, state)

	if state.Async {
		var hit bool
		response, hit, err = e.tasks.CreateAndDispatch(ctx, state)
		if err != nil {
			_ = e.admission.Release(ctx, state, err)
			response = e.errorResponse(state, err)
			return response, nil
		}
		if hit {
			_ = e.admission.Release(ctx, state, errAsyncIdempotencyReplay)
			state.Internal["idempotency_hit"] = true
		}
		return response, nil
	}

	// Step 4: dispatch to the selected provider and finalize billing.
	if err = e.runStage(ctx, state, "gateway.plugin.pre_provider", func(stageCtx context.Context) error {
		return e.plugins.Run(stageCtx, "pre_provider", state)
	}); err != nil {
		_ = e.admission.Release(ctx, state, err)
		response = e.errorResponse(state, err)
		return response, nil
	}
	result, dispatchErr := e.dispatcher.Dispatch(ctx, state)
	if dispatchErr != nil {
		_ = e.admission.Release(ctx, state, dispatchErr)
		err = dispatchErr
		response = e.errorResponse(state, err)
		return response, nil
	}
	state.ProviderResult = result
	state.ActualUsage = result.Usage
	if err = e.runStage(ctx, state, "gateway.plugin.post_provider", func(stageCtx context.Context) error {
		return e.plugins.Run(stageCtx, "post_provider", state)
	}); err != nil {
		response, err = e.settleBeforePolicyError(ctx, state, err)
		return response, nil
	}
	if result.Response != nil && result.Response.Stream != nil {
		if err = e.runStage(ctx, state, "gateway.stream", func(stageCtx context.Context) error {
			var wrapErr error
			response, wrapErr = e.stream.Wrap(stageCtx, state, result)
			return wrapErr
		}); err != nil {
			response = e.errorResponse(state, err)
			return response, nil
		}
		return response, nil
	}
	if err = e.runStage(ctx, state, "gateway.plugin.pre_settlement", func(stageCtx context.Context) error {
		return e.plugins.Run(stageCtx, "pre_settlement", state)
	}); err != nil {
		response, err = e.settleBeforePolicyError(ctx, state, err)
		return response, nil
	}
	if err = e.runStage(ctx, state, "gateway.settlement", func(stageCtx context.Context) error {
		return e.settlement.Settle(stageCtx, state)
	}); err != nil {
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

func (e *GatewayEngine) evaluatePolicy(ctx context.Context, state *RequestState) error {
	decision, err := e.policy.Evaluate(ctx, state)
	if err != nil {
		return err
	}
	if decision.Action == "" {
		decision.Action = PolicyAllow
	}
	state.PolicyDecision = decision
	if len(decision.Metadata) > 0 {
		if state.Metadata == nil {
			state.Metadata = map[string]string{}
		}
		for key, value := range decision.Metadata {
			state.Metadata[key] = value
		}
	}
	switch decision.Action {
	case PolicyAllow:
		return nil
	case PolicyDeny:
		message := decision.Reason
		if message == "" {
			message = "request denied by policy"
		}
		return apperr.PolicyDenied(message)
	case PolicyDegrade:
		if decision.DegradeModel == "" {
			return apperr.ConfigUnavailable("policy degrade model is required")
		}
		state.RequestedModel = decision.DegradeModel
		clearRouteSelection(state)
		return nil
	case PolicyRouteOverride:
		if decision.RoutePlan == nil || len(decision.RoutePlan.Candidates) == 0 {
			return apperr.ConfigUnavailable("policy route override requires candidates")
		}
		if err := constrainRouteOverride(state, decision.RoutePlan); err != nil {
			return err
		}
		state.RoutePlan = decision.RoutePlan
		return nil
	default:
		return apperr.ConfigUnavailable("policy decision action is not supported")
	}
}

func clearRouteSelection(state *RequestState) {
	if state == nil {
		return
	}
	state.ResolvedModel = ModelView{}
	state.PriceRule = PriceRuleView{}
	state.LimitRule = LimitRuleView{}
	state.RoutePlan = nil
}

func constrainRouteOverride(state *RequestState, plan *RoutePlan) error {
	if state == nil || state.Snapshot == nil {
		return apperr.ConfigUnavailable("runtime snapshot is unavailable")
	}
	model, ok := state.Snapshot.LookupModel(state.RequestedModel)
	if !ok || !model.Enabled {
		return apperr.NotFound("model not found")
	}
	if state.Principal == nil {
		return apperr.Unauthorized("authentication is required")
	}
	if !principalAllowsModel(state.Principal.AllowedModels, state.RequestedModel, model) {
		return apperr.Forbidden("model is not allowed")
	}
	if !protocolMatchesPolicyModel(state.ProtocolMode, model.Protocol) {
		return apperr.InvalidArgument("model protocol does not match endpoint")
	}
	for _, candidate := range plan.Candidates {
		channel, ok := state.Snapshot.LookupChannel(candidate.ChannelID)
		if !ok || !channel.Enabled {
			return apperr.ServiceUnavailable("provider channel is unavailable", apperr.WithTemporary())
		}
		if candidate.ProviderType != "" && candidate.ProviderType != channel.ProviderType {
			return apperr.ConfigUnavailable("route override provider does not match channel")
		}
		upstreamModel := channel.Models[model.PublicModel]
		if upstreamModel == "" {
			return apperr.ConfigUnavailable("route override channel does not serve model")
		}
		if candidate.PublicModel != "" && candidate.PublicModel != model.PublicModel {
			return apperr.ConfigUnavailable("route override public model does not match request")
		}
		if candidate.UpstreamModel != "" && candidate.UpstreamModel != upstreamModel {
			return apperr.ConfigUnavailable("route override upstream model does not match channel mapping")
		}
	}
	state.ResolvedModel = model
	if price, ok := state.Snapshot.LookupPrice(model.PublicModel); ok {
		state.PriceRule = price
	}
	if limit, ok := state.Snapshot.LookupLimit(model.PublicModel); ok {
		state.LimitRule = limit
	}
	return nil
}

func principalAllowsModel(allowed []string, requested string, model ModelView) bool {
	if listAllowsModel(allowed, "*") || listAllowsModel(allowed, model.PublicModel) || listAllowsModel(allowed, requested) {
		return true
	}
	for _, alias := range model.Aliases {
		if listAllowsModel(allowed, alias) {
			return true
		}
	}
	return false
}

func listAllowsModel(allowed []string, model string) bool {
	for _, value := range allowed {
		if value == model {
			return true
		}
	}
	return false
}

func protocolMatchesPolicyModel(requestMode ProtocolMode, modelMode ProtocolMode) bool {
	return requestMode == "" || modelMode == "" || requestMode == modelMode
}

func addSnapshotHeader(response *GatewayResponse, state *RequestState) {
	if response == nil || state == nil || state.SnapshotRef.Version == "" {
		return
	}
	if response.Header == nil {
		response.Header = http.Header{}
	}
	response.Header.Set("X-Gateway-Snapshot-Version", state.SnapshotRef.Version)
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
	if e.policy == nil {
		errs = append(errs, errors.New("policy evaluator is required"))
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

var errAsyncIdempotencyReplay = errors.New("async idempotency replay")

func (e *GatewayEngine) runStage(ctx context.Context, state *RequestState, name string, fn func(context.Context) error) error {
	stageCtx, span := e.observe.StartSpan(ctx, name, stageAttrs(state)...)
	err := fn(stageCtx)
	if err != nil {
		span.RecordError(err)
	}
	span.End()
	return err
}

func stageAttrs(state *RequestState) []attribute.KeyValue {
	if state == nil {
		return nil
	}
	return []attribute.KeyValue{
		attribute.String("gateway.protocol", stringOrUnknown(string(state.ProtocolMode))),
		attribute.String("gateway.canonical_api", stringOrUnknown(string(state.CanonicalAPI))),
		attribute.String("gateway.model", stringOrUnknown(state.RequestedModel)),
	}
}

func stringOrUnknown(value string) string {
	if value == "" {
		return "unknown"
	}
	return value
}

func (e *GatewayEngine) settleBeforePolicyError(ctx context.Context, state *RequestState, policyErr error) (*GatewayResponse, error) {
	if settleErr := e.settlement.Settle(ctx, state); settleErr != nil {
		recordErr := e.settlement.RecordFailed(ctx, state, settleErr)
		if recordErr != nil {
			err := errors.Join(policyErr, settleErr, recordErr)
			return e.errorResponse(state, err), err
		}
	}
	return e.errorResponse(state, policyErr), policyErr
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
	case apperr.CodeConfigUnavailable, apperr.CodeServiceUnavailable:
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
	case apperr.CodeForbidden, apperr.CodePolicyDenied:
		return "permission_error"
	case apperr.CodeProviderError:
		return "provider_error"
	case apperr.CodeRateLimited:
		return "rate_limit_error"
	case apperr.CodeInternal, apperr.CodeConfigUnavailable, apperr.CodeServiceUnavailable:
		return "service_error"
	default:
		return "invalid_request_error"
	}
}
