package jobs

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"time"

	tasksvc "github.com/KnifeFly/token-gateway/internal/task"
	"github.com/KnifeFly/token-gateway/pkg/egressguard"
)

// CallbackDispatcher delivers task callback outbox events with retry state.
type CallbackDispatcher struct {
	repo     tasksvc.Repository
	client   *http.Client
	metrics  *tasksvc.Metrics
	interval time.Duration
	timeout  time.Duration
	limit    int
	egress   *egressguard.Guard
	secret   []byte
	maxRetry int
}

// CallbackDispatcherOption customizes callback delivery.
type CallbackDispatcherOption func(*CallbackDispatcher)

// NewCallbackDispatcher returns a callback dispatcher job.
func NewCallbackDispatcher(repo tasksvc.Repository, client *http.Client, interval time.Duration, limit int) *CallbackDispatcher {
	return NewCallbackDispatcherWithMetrics(repo, client, nil, interval, limit)
}

// NewCallbackDispatcherWithMetrics returns a callback dispatcher job with metrics.
func NewCallbackDispatcherWithMetrics(repo tasksvc.Repository, client *http.Client, metrics *tasksvc.Metrics, interval time.Duration, limit int, options ...CallbackDispatcherOption) *CallbackDispatcher {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	if limit <= 0 {
		limit = 100
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	dispatcher := &CallbackDispatcher{
		repo:     repo,
		client:   client,
		metrics:  metrics,
		interval: interval,
		timeout:  30 * time.Second,
		limit:    limit,
		maxRetry: 5,
	}
	for _, option := range options {
		if option != nil {
			option(dispatcher)
		}
	}
	return dispatcher
}

// WithCallbackEgressGuard validates callback URLs before delivery.
func WithCallbackEgressGuard(guard *egressguard.Guard) CallbackDispatcherOption {
	return func(j *CallbackDispatcher) {
		j.egress = guard
	}
}

// WithCallbackSigningSecret signs callback payloads with HMAC-SHA256.
func WithCallbackSigningSecret(secret string) CallbackDispatcherOption {
	return func(j *CallbackDispatcher) {
		j.secret = []byte(secret)
	}
}

// WithCallbackMaxRetries sets the number of delivery attempts before dead-letter.
func WithCallbackMaxRetries(maxRetries int) CallbackDispatcherOption {
	return func(j *CallbackDispatcher) {
		if maxRetries > 0 {
			j.maxRetry = maxRetries
		}
	}
}

// Name returns the stable worker job name.
func (j *CallbackDispatcher) Name() string {
	return "callback_dispatcher"
}

// Interval returns how often the job should run.
func (j *CallbackDispatcher) Interval() time.Duration {
	return j.interval
}

// Timeout returns the maximum runtime for one job execution.
func (j *CallbackDispatcher) Timeout() time.Duration {
	return j.timeout
}

// MaxConcurrency returns the maximum parallel executions for this job.
func (j *CallbackDispatcher) MaxConcurrency() int {
	return 4
}

// Run delivers due callback events and schedules retries for failures.
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
		if j.egress != nil {
			if err := j.egress.ValidateURL(ctx, event.URL); err != nil {
				if markErr := j.markFailure(ctx, event, now, "egress_denied", err.Error()); markErr != nil {
					return markErr
				}
				continue
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, event.URL, bytes.NewReader(event.Payload))
		if err != nil {
			if markErr := j.markFailure(ctx, event, now, "invalid_request", err.Error()); markErr != nil {
				return markErr
			}
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Gateway-Task-ID", event.TaskID)
		req.Header.Set("X-Gateway-Callback-ID", event.ID)
		j.sign(req, event, now)
		response, err := j.client.Do(req)
		if err != nil {
			if markErr := j.markFailure(ctx, event, now, "network_error", err.Error()); markErr != nil {
				return markErr
			}
			continue
		}
		_ = response.Body.Close()
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			if markErr := j.markFailure(ctx, event, now, "http_"+statusClass(response.StatusCode), response.Status); markErr != nil {
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

func (j *CallbackDispatcher) sign(req *http.Request, event tasksvc.CallbackEvent, now time.Time) {
	if len(j.secret) == 0 {
		return
	}
	timestamp := strconv.FormatInt(now.Unix(), 10)
	mac := hmac.New(sha256.New, j.secret)
	_, _ = mac.Write([]byte(timestamp))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write(event.Payload)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	req.Header.Set("X-Gateway-Callback-Timestamp", timestamp)
	req.Header.Set("X-Gateway-Callback-Signature", signature)
}

func (j *CallbackDispatcher) markFailure(ctx context.Context, event tasksvc.CallbackEvent, now time.Time, reason string, lastError string) error {
	if j.metrics != nil {
		j.metrics.RecordCallbackRetry(reason)
	}
	if j.maxRetry > 0 && event.RetryCount+1 >= j.maxRetry {
		return j.repo.MarkCallbackDeadLetter(ctx, event.ID, lastError)
	}
	return j.repo.MarkCallbackFailed(ctx, event.ID, nextCallbackRetry(now, event.RetryCount), lastError)
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
