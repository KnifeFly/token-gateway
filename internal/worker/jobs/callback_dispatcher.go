package jobs

import (
	"bytes"
	"context"
	"net/http"
	"strconv"
	"time"

	tasksvc "github.com/KnifeFly/token-gateway/internal/task"
)

// CallbackDispatcher delivers task callback outbox events with retry state.
type CallbackDispatcher struct {
	repo     tasksvc.Repository
	client   *http.Client
	metrics  *tasksvc.Metrics
	interval time.Duration
	timeout  time.Duration
	limit    int
}

// NewCallbackDispatcher returns a callback dispatcher job.
func NewCallbackDispatcher(repo tasksvc.Repository, client *http.Client, interval time.Duration, limit int) *CallbackDispatcher {
	return NewCallbackDispatcherWithMetrics(repo, client, nil, interval, limit)
}

// NewCallbackDispatcherWithMetrics returns a callback dispatcher job with metrics.
func NewCallbackDispatcherWithMetrics(repo tasksvc.Repository, client *http.Client, metrics *tasksvc.Metrics, interval time.Duration, limit int) *CallbackDispatcher {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	if limit <= 0 {
		limit = 100
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &CallbackDispatcher{
		repo:     repo,
		client:   client,
		metrics:  metrics,
		interval: interval,
		timeout:  30 * time.Second,
		limit:    limit,
	}
}

func (j *CallbackDispatcher) Name() string {
	return "callback_dispatcher"
}

func (j *CallbackDispatcher) Interval() time.Duration {
	return j.interval
}

func (j *CallbackDispatcher) Timeout() time.Duration {
	return j.timeout
}

func (j *CallbackDispatcher) MaxConcurrency() int {
	return 4
}

func (j *CallbackDispatcher) Run(ctx context.Context) error {
	if j == nil || j.repo == nil || j.client == nil {
		return nil
	}
	now := time.Now().UTC()
	events, err := j.repo.ListDueCallbacks(ctx, j.limit, now)
	if err != nil {
		return err
	}
	for _, event := range events {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, event.URL, bytes.NewReader(event.Payload))
		if err != nil {
			j.metrics.RecordCallbackRetry("invalid_request")
			if markErr := j.repo.MarkCallbackFailed(ctx, event.ID, nextCallbackRetry(now, event.RetryCount), err.Error()); markErr != nil {
				return markErr
			}
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Gateway-Task-ID", event.TaskID)
		req.Header.Set("X-Gateway-Callback-ID", event.ID)
		response, err := j.client.Do(req)
		if err != nil {
			j.metrics.RecordCallbackRetry("network_error")
			if markErr := j.repo.MarkCallbackFailed(ctx, event.ID, nextCallbackRetry(now, event.RetryCount), err.Error()); markErr != nil {
				return markErr
			}
			continue
		}
		_ = response.Body.Close()
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			j.metrics.RecordCallbackRetry("http_" + statusClass(response.StatusCode))
			if markErr := j.repo.MarkCallbackFailed(ctx, event.ID, nextCallbackRetry(now, event.RetryCount), response.Status); markErr != nil {
				return markErr
			}
			continue
		}
		if err := j.repo.MarkCallbackDelivered(ctx, event.ID); err != nil {
			return err
		}
	}
	return nil
}

func nextCallbackRetry(now time.Time, retryCount int) time.Time {
	delay := time.Duration(retryCount+1) * time.Minute
	if delay > 30*time.Minute {
		delay = 30 * time.Minute
	}
	return now.Add(delay)
}

func statusClass(status int) string {
	if status <= 0 {
		return "none"
	}
	return strconv.Itoa(status/100) + "xx"
}
