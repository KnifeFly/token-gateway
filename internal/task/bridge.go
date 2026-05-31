package task

import (
	"context"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
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

// Bridge connects the data-plane engine to the M4 task service.
type Bridge struct {
	service    *Service
	dispatcher ProviderTaskDispatcher
	settlement Settlement
}

// NewBridge returns a task bridge.
func NewBridge(service *Service, dispatcher ProviderTaskDispatcher, settlement ...Settlement) *Bridge {
	bridge := &Bridge{service: service, dispatcher: dispatcher}
	if len(settlement) > 0 {
		bridge.settlement = settlement[0]
	}
	return bridge
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
	candidate, err := firstCandidate(state)
	if err != nil {
		return nil, false, err
	}
	channel, ok := state.Snapshot.LookupChannel(candidate.ChannelID)
	if !ok || !channel.Enabled {
		return nil, false, apperr.ServiceUnavailable("provider channel is unavailable", apperr.WithTemporary())
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
	})
	if err != nil {
		return nil, false, err
	}
	if hit {
		response, err := TaskResponse(task)
		return response, true, err
	}
	providerTask, err := b.dispatcher.Submit(ctx, ProviderTaskRequest{
		Task:      *task,
		Candidate: candidate,
		Channel:   channel,
		RequestID: state.RequestID,
	})
	if err != nil {
		_, _ = b.service.MarkFailed(ctx, task.ID, "provider_submit_failed", "provider task submit failed")
		return nil, false, err
	}
	updated, err := b.service.MarkDispatched(ctx, task.ID, candidate.ProviderType, candidate.ChannelID, providerTask.ExternalID)
	if err != nil {
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

func firstCandidate(state *engine.RequestState) (engine.ProviderCandidate, error) {
	if state.RoutePlan == nil || len(state.RoutePlan.Candidates) == 0 {
		return engine.ProviderCandidate{}, apperr.ServiceUnavailable("no route is available", apperr.WithTemporary())
	}
	return state.RoutePlan.Candidates[0], nil
}
