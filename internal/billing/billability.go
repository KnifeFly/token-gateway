package billing

import (
	"strconv"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	"github.com/KnifeFly/token-gateway/pkg/tokenusage"
)

const (
	// BillabilityOperationSync marks a synchronous provider interaction.
	BillabilityOperationSync = "sync"
	// BillabilityOperationStream marks a streaming provider interaction.
	BillabilityOperationStream = "stream"
	// BillabilityOperationTask marks an async task provider interaction.
	BillabilityOperationTask = "task"

	// BillabilityReasonProviderSuccess records a fully successful billable request.
	BillabilityReasonProviderSuccess = "provider_success"
	// BillabilityReasonPartialOutputClientDisconnected records billable partial stream output after client disconnect.
	BillabilityReasonPartialOutputClientDisconnected = "partial_output_client_disconnected"
	// BillabilityReasonPartialOutputDownstreamFailed records billable partial stream output after downstream write failure.
	BillabilityReasonPartialOutputDownstreamFailed = "partial_output_downstream_failed"
	// BillabilityReasonNoEffectiveOutput records non-billable interactions with no usable output.
	BillabilityReasonNoEffectiveOutput = "no_effective_output"
	// BillabilityReasonProviderError records non-billable provider failures.
	BillabilityReasonProviderError = "provider_error"
	// BillabilityReasonTaskCanceled records non-billable canceled tasks.
	BillabilityReasonTaskCanceled = "task_canceled"
	// BillabilityReasonTaskFailed records non-billable failed tasks.
	BillabilityReasonTaskFailed = "task_failed"
	// BillabilityReasonTaskNotSucceeded records non-billable tasks that are not terminal success.
	BillabilityReasonTaskNotSucceeded = "task_not_succeeded"
)

// BillabilityPolicy decides whether a completed provider interaction is billable.
type BillabilityPolicy struct{}

// BillabilityContext is the protocol-neutral input used for billability decisions.
type BillabilityContext struct {
	Operation       string
	Usage           tokenusage.Actual
	ResponseBytes   int64
	StreamChunks    int64
	TaskResultBytes int64
	DownstreamError string
	ProviderError   string
	TaskStatus      string
}

// BillabilityDecision records the auditable billability outcome.
type BillabilityDecision struct {
	Billable bool
	Reason   string
}

// NewBillabilityPolicy returns the default commercial billability policy.
func NewBillabilityPolicy() BillabilityPolicy {
	return BillabilityPolicy{}
}

// Decide returns whether usage should be charged and the stable reason code.
func (p BillabilityPolicy) Decide(ctx BillabilityContext) BillabilityDecision {
	if ctx.ProviderError != "" {
		return BillabilityDecision{Billable: false, Reason: BillabilityReasonProviderError}
	}
	if ctx.Operation == BillabilityOperationTask {
		switch ctx.TaskStatus {
		case "succeeded", "":
		case "canceled", "cancelled":
			return BillabilityDecision{Billable: false, Reason: BillabilityReasonTaskCanceled}
		case "failed", "expired":
			return BillabilityDecision{Billable: false, Reason: BillabilityReasonTaskFailed}
		default:
			return BillabilityDecision{Billable: false, Reason: BillabilityReasonTaskNotSucceeded}
		}
	}
	if !hasEffectiveOutput(ctx) {
		return BillabilityDecision{Billable: false, Reason: BillabilityReasonNoEffectiveOutput}
	}
	if ctx.Operation == BillabilityOperationStream {
		switch ctx.DownstreamError {
		case "client_disconnected":
			return BillabilityDecision{Billable: true, Reason: BillabilityReasonPartialOutputClientDisconnected}
		case "downstream_write_failed":
			return BillabilityDecision{Billable: true, Reason: BillabilityReasonPartialOutputDownstreamFailed}
		}
	}
	return BillabilityDecision{Billable: true, Reason: BillabilityReasonProviderSuccess}
}

// RequestBillabilityContext derives billability inputs from a gateway request.
func RequestBillabilityContext(state *engine.RequestState) BillabilityContext {
	if state == nil {
		return BillabilityContext{Operation: BillabilityOperationSync}
	}
	ctx := BillabilityContext{
		Operation: BillabilityOperationSync,
		Usage:     state.ActualUsage,
	}
	if state.Stream {
		ctx.Operation = BillabilityOperationStream
		ctx.StreamChunks = int64FromInternal(state, "stream_chunks")
		ctx.ResponseBytes = int64FromInternal(state, "stream_upstream_bytes")
		ctx.DownstreamError = stringFromInternal(state, "stream_downstream_error")
	} else if state.ProviderResult != nil && state.ProviderResult.Response != nil {
		ctx.ResponseBytes = int64(len(state.ProviderResult.Response.Body))
	}
	if state.ProviderResult == nil {
		ctx.ProviderError = stringFromInternal(state, "provider_error")
	}
	return ctx
}

func hasEffectiveOutput(ctx BillabilityContext) bool {
	return ctx.Usage.TotalTokens > 0 ||
		ctx.Usage.OutputTokens > 0 ||
		ctx.ResponseBytes > 0 ||
		ctx.StreamChunks > 0 ||
		ctx.TaskResultBytes > 0
}

func int64FromInternal(state *engine.RequestState, key string) int64 {
	if state == nil || state.Internal == nil {
		return 0
	}
	switch value := state.Internal[key].(type) {
	case int:
		return int64(value)
	case int64:
		return value
	case float64:
		return int64(value)
	case string:
		n, _ := strconv.ParseInt(value, 10, 64)
		return n
	default:
		return 0
	}
}

func stringFromInternal(state *engine.RequestState, key string) string {
	if state == nil || state.Internal == nil {
		return ""
	}
	value, _ := state.Internal[key].(string)
	return value
}
