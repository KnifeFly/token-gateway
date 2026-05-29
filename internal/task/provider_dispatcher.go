package task

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/KnifeFly/token-gateway/pkg/tokenusage"
)

// MockProviderTaskDispatcher is the deterministic local M4 provider-task client.
type MockProviderTaskDispatcher struct{}

// NewMockProviderTaskDispatcher returns a mock provider-task dispatcher.
func NewMockProviderTaskDispatcher() *MockProviderTaskDispatcher {
	return &MockProviderTaskDispatcher{}
}

// Submit returns a stable external task id without calling a real provider.
func (d *MockProviderTaskDispatcher) Submit(_ context.Context, request ProviderTaskRequest) (*ProviderTask, error) {
	externalID := fmt.Sprintf("external_%s_%s", request.Candidate.ChannelID, request.Task.ID)
	return &ProviderTask{ExternalID: externalID, Status: StatusRunning, Progress: 1}, nil
}

// Poll completes the mock task with one normalized result URL.
func (d *MockProviderTaskDispatcher) Poll(_ context.Context, task Task) (*ProviderTaskResult, error) {
	resultURL := fmt.Sprintf("mock://%s/%s", strings.ReplaceAll(string(task.Kind), ".", "_"), task.ID)
	result, _ := json.Marshal(map[string]any{"results": []string{resultURL}})
	usage := tokenusage.Actual{
		InputTokens:  int64(len(task.Input) / 4),
		OutputTokens: 32,
	}
	usage.TotalTokens = usage.InputTokens + usage.OutputTokens
	return &ProviderTaskResult{
		Status:   StatusSucceeded,
		Progress: 100,
		Result:   result,
		Usage:    usage,
	}, nil
}

// Cancel acknowledges local task cancellation.
func (d *MockProviderTaskDispatcher) Cancel(context.Context, Task) error {
	return nil
}
