package task

import (
	"context"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	"github.com/KnifeFly/token-gateway/internal/domain/pricing"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

// bridge.go translates data-plane async/file operations into durable task service calls.

// ProviderTaskDispatcher submits and controls upstream async tasks.
type ProviderTaskDispatcher interface {
	Submit(ctx context.Context, request ProviderTaskRequest) (*ProviderTask, error)
	Poll(ctx context.Context, task Task) (*ProviderTaskResult, error)
	Cancel(ctx context.Context, task Task) error
}

// ProviderTaskRequest contains the selected route and task to submit.
type ProviderTaskRequest struct {
	Task      Task
	Candidate engine.ProviderCandidate
	Channel   engine.ChannelView
	RequestID string
}

// AttemptRecorder persists async provider submit attempts.
type AttemptRecorder interface {
	RecordProviderAttempt(ctx context.Context, state *engine.RequestState, attempt engine.ProviderAttempt) error
}

// Bridge connects the data-plane engine to the M4 task service.
type Bridge struct {
	service    *Service
	dispatcher ProviderTaskDispatcher
	settlement Settlement
	price      pricing.TokenPrice
	attempts   AttemptRecorder
}

// NewBridge returns a task bridge.
func NewBridge(service *Service, dispatcher ProviderTaskDispatcher, settlement ...Settlement) *Bridge {
	bridge := &Bridge{service: service, dispatcher: dispatcher}
	if len(settlement) > 0 {
		bridge.settlement = settlement[0]
	}
	return bridge
}

// WithDefaultPrice configures the billing default price used for async price snapshots.
func (b *Bridge) WithDefaultPrice(price pricing.TokenPrice) *Bridge {
	if b != nil {
		b.price = price
	}
	return b
}

// WithAttemptRecorder configures durable async submit attempt auditing.
func (b *Bridge) WithAttemptRecorder(recorder AttemptRecorder) *Bridge {
	if b != nil {
		b.attempts = recorder
	}
	return b
}

// CheckIdempotency returns an existing async task before admission creates a new hold.
func (b *Bridge) CheckIdempotency(ctx context.Context, state *engine.RequestState) (*engine.GatewayResponse, bool, error) {
	if b == nil || b.service == nil || state.IdempotencyKey == "" {
		return nil, false, nil
	}
	task, hit, err := b.service.FindIdempotentTask(ctx, IdempotencyCheck{
		TenantID:       state.TenantID,
		APIKeyID:       state.APIKeyID,
		Endpoint:       state.Endpoint.Path,
		IdempotencyKey: state.IdempotencyKey,
		Body:           state.Parsed.RawBody,
	})
	if err != nil || !hit {
		return nil, false, err
	}
	response, err := TaskResponse(task)
	return response, true, err
}

// CreateAndDispatch creates an internal task, submits the provider task, and returns the task object.
func (b *Bridge) CreateAndDispatch(ctx context.Context, state *engine.RequestState) (*engine.GatewayResponse, bool, error) {
	if b == nil || b.service == nil || b.dispatcher == nil {
		return nil, false, apperr.ConfigUnavailable("task bridge is unavailable")
	}
	if state.Parsed.Media == nil {
		return nil, false, apperr.InvalidArgument("media request is required")
	}
	candidates, err := routeCandidates(state)
	if err != nil {
		return nil, false, err
	}
	task, hit, err := b.service.CreateMediaTask(ctx, CreateTaskRequest{
		TenantID:       state.TenantID,
		ProjectID:      state.ProjectID,
		APIKeyID:       state.APIKeyID,
		RequestID:      state.RequestID,
		Endpoint:       state.Endpoint.Path,
		IdempotencyKey: state.IdempotencyKey,
		Kind:           Kind(state.Parsed.Media.Kind),
		MediaType:      state.Parsed.Media.MediaType,
		Model:          state.RequestedModel,
		Input:          state.Parsed.RawBody,
		CallbackURL:    state.Parsed.Media.CallbackURL,
		Metadata:       state.Parsed.Media.Metadata,
		BalanceHoldID:  state.BalanceHoldID,
		PriceSnapshot:  b.priceSnapshot(state),
	})
	if err != nil {
		return nil, false, err
	}
	if hit {
		response, err := TaskResponse(task)
		return response, true, err
	}
	updated, providerTask, err := b.submitCreatedTask(ctx, state, *task, candidates)
	if err != nil {
		_, _ = b.service.MarkFailed(ctx, task.ID, "provider_submit_failed", "provider task submit failed")
		return nil, false, err
	}
	if providerTask.Status != "" && IsTerminal(providerTask.Status) {
		result := providerTask.ResultForTask()
		settlementTask := *updated
		settlementTask.Status = result.Status
		settlementTask.Result = result.Result
		settlementTask.Usage = result.Usage
		settlementTask.ErrorCode = result.ErrorCode
		settlementTask.ErrorMessage = result.ErrorMessage
		if err := SettleTerminalTask(ctx, b.settlement, settlementTask, result.Usage); err != nil {
			return nil, false, err
		}
		updated, err = b.service.CompleteTask(ctx, *updated, result)
		if err != nil {
			return nil, false, err
		}
	}
	response, err := TaskResponse(updated)
	return response, false, err
}

func (b *Bridge) submitCreatedTask(ctx context.Context, state *engine.RequestState, task Task, candidates []engine.ProviderCandidate) (*Task, *ProviderTask, error) {
	var lastErr error
	for idx, candidate := range candidates {
		channel, ok := state.Snapshot.LookupChannel(candidate.ChannelID)
		if !ok || !channel.Enabled {
			lastErr = apperr.ServiceUnavailable("provider channel is unavailable", apperr.WithTemporary())
			if err := b.recordAsyncSubmitAttempt(ctx, state, task, idx, len(candidates), candidate, false, lastErr); err != nil {
				return nil, nil, err
			}
			continue
		}
		providerTask, err := b.dispatcher.Submit(ctx, ProviderTaskRequest{
			Task:      task,
			Candidate: candidate,
			Channel:   channel,
			RequestID: state.RequestID,
		})
		if err != nil {
			lastErr = err
			if recordErr := b.recordAsyncSubmitAttempt(ctx, state, task, idx, len(candidates), candidate, false, err); recordErr != nil {
				return nil, nil, recordErr
			}
			if providerTask != nil && providerTask.ExternalID != "" {
				updated, markErr := b.service.MarkDispatched(ctx, task.ID, candidate.ProviderType, candidate.ChannelID, providerTask.ExternalID)
				if markErr != nil {
					return nil, nil, markErr
				}
				return updated, providerTask, nil
			}
			if asyncSubmitRetryable(err) {
				continue
			}
			break
		}
		if providerTask == nil || providerTask.ExternalID == "" {
			lastErr = apperr.ProviderError("provider task response is missing external task id")
			if err := b.recordAsyncSubmitAttempt(ctx, state, task, idx, len(candidates), candidate, false, lastErr); err != nil {
				return nil, nil, err
			}
			break
		}
		if err := b.recordAsyncSubmitAttempt(ctx, state, task, idx, len(candidates), candidate, true, nil); err != nil {
			return nil, nil, err
		}
		updated, err := b.service.MarkDispatched(ctx, task.ID, candidate.ProviderType, candidate.ChannelID, providerTask.ExternalID)
		return updated, providerTask, err
	}
	if lastErr == nil {
		lastErr = apperr.ServiceUnavailable("no route is available", apperr.WithTemporary())
	}
	return nil, nil, lastErr
}

func (b *Bridge) priceSnapshot(state *engine.RequestState) PriceSnapshot {
	if state == nil {
		return PriceSnapshot{}
	}
	publicModel := state.RequestedModel
	if state.ResolvedModel.PublicModel != "" {
		publicModel = state.ResolvedModel.PublicModel
	}
	snapshot := PriceSnapshot{
		PublicModel:           publicModel,
		EstimatedOutputTokens: state.EstimatedUsage.OutputTokens,
		EstimatedChargeMicros: state.EstimatedChargeMicros,
		RouteSnapshotVersion:  state.SnapshotRef.Version,
	}
	if state.RoutePlan != nil {
		snapshot.RoutePolicyID = state.RoutePlan.PolicyID
	}
	if state.PriceRule.Enabled {
		snapshot.Currency = state.PriceRule.Currency
		snapshot.Category = state.PriceRule.Category
		snapshot.Components = append([]pricing.Component(nil), state.PriceRule.Components...)
		snapshot.InputMicrosPerToken = state.PriceRule.InputMicrosPerToken
		snapshot.OutputMicrosPerToken = state.PriceRule.OutputMicrosPerToken
		if state.PriceRule.EstimatedOutputTokens > 0 {
			snapshot.EstimatedOutputTokens = state.PriceRule.EstimatedOutputTokens
		}
		snapshot.Source = "runtime_price_rule"
		return snapshot
	}
	if b != nil && b.price.Currency != "" {
		snapshot.Currency = b.price.Currency
		snapshot.InputMicrosPerToken = b.price.InputMicrosPerToken
		snapshot.OutputMicrosPerToken = b.price.OutputMicrosPerToken
		snapshot.Source = "gateway_default_price"
		return snapshot
	}
	if state.Currency != "" {
		snapshot.Currency = state.Currency
		snapshot.Source = "estimated_amount_only"
	}
	return snapshot
}

// HandleTaskOperation handles task get/cancel operations after authentication.
func (b *Bridge) HandleTaskOperation(ctx context.Context, state *engine.RequestState) (*engine.GatewayResponse, error) {
	if b == nil || b.service == nil {
		return nil, apperr.ConfigUnavailable("task bridge is unavailable")
	}
	if state.Parsed.Task == nil {
		return nil, apperr.InvalidArgument("task operation is required")
	}
	switch state.Parsed.Task.Operation {
	case engine.TaskOperationGet:
		task, err := b.service.GetTask(ctx, state.TenantID, state.ProjectID, state.Parsed.Task.TaskID)
		if err != nil {
			return nil, err
		}
		return TaskResponse(task)
	case engine.TaskOperationCancel:
		task, err := b.service.GetTask(ctx, state.TenantID, state.ProjectID, state.Parsed.Task.TaskID)
		if err != nil {
			return nil, err
		}
		wasTerminal := IsTerminal(task.Status)
		if !IsTerminal(task.Status) && b.dispatcher != nil {
			if err := b.dispatcher.Cancel(ctx, *task); err != nil {
				return nil, err
			}
		}
		task, err = b.service.CancelTask(ctx, state.TenantID, state.ProjectID, state.Parsed.Task.TaskID)
		if err != nil {
			return nil, err
		}
		if !wasTerminal {
			if err := SettleTerminalTask(ctx, b.settlement, *task, task.Usage); err != nil {
				return nil, err
			}
		}
		return TaskResponse(task)
	default:
		return nil, apperr.InvalidArgument("unsupported task operation")
	}
}

func routeCandidates(state *engine.RequestState) ([]engine.ProviderCandidate, error) {
	if state.RoutePlan == nil || len(state.RoutePlan.Candidates) == 0 {
		return nil, apperr.ServiceUnavailable("no route is available", apperr.WithTemporary())
	}
	return state.RoutePlan.Candidates, nil
}

func asyncSubmitRetryable(err error) bool {
	appErr, ok := apperr.As(err)
	if !ok {
		return false
	}
	return appErr.Temporary || appErr.Code == apperr.CodeServiceUnavailable || appErr.Code == apperr.CodeRateLimited
}

func (b *Bridge) recordAsyncSubmitAttempt(ctx context.Context, state *engine.RequestState, task Task, index int, candidateCount int, candidate engine.ProviderCandidate, success bool, err error) error {
	if state == nil {
		return nil
	}
	attempt := buildAsyncSubmitAttempt(state, task, index, candidateCount, candidate, success, err)
	state.Attempts = append(state.Attempts, attempt)
	if b == nil || b.attempts == nil {
		return nil
	}
	return b.attempts.RecordProviderAttempt(ctx, state, attempt)
}

func buildAsyncSubmitAttempt(state *engine.RequestState, task Task, index int, candidateCount int, candidate engine.ProviderCandidate, success bool, err error) engine.ProviderAttempt {
	if state == nil {
		return engine.ProviderAttempt{}
	}
	attempt := engine.ProviderAttempt{
		AttemptIndex:         index + 1,
		TaskID:               task.ID,
		ChannelID:            candidate.ChannelID,
		ProviderType:         candidate.ProviderType,
		PublicModel:          candidate.PublicModel,
		UpstreamModel:        candidate.UpstreamModel,
		Success:              success,
		RetryBudgetConsumed:  index + 1,
		RetryBudgetRemaining: max(candidateCount-index-1, 0),
		Final:                success || index >= candidateCount-1,
	}
	if index > 0 && state.RoutePlan != nil && index-1 < len(state.RoutePlan.Candidates) {
		prev := state.RoutePlan.Candidates[index-1]
		attempt.FallbackFromChannelID = prev.ChannelID
		attempt.FallbackFromProvider = prev.ProviderType
	}
	if err != nil {
		attempt.ErrorCode = "provider_submit_failed"
		if appErr, ok := apperr.As(err); ok {
			attempt.ErrorCode = string(appErr.Code)
			attempt.Retryable = appErr.Temporary
		}
		if !attempt.Retryable {
			attempt.Final = true
		}
	}
	return attempt
}
