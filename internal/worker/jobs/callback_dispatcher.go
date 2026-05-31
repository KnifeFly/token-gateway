package jobs

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"strconv"
	"time"

	tasksvc "github.com/KnifeFly/token-gateway/internal/task"
	"github.com/KnifeFly/token-gateway/pkg/egressguard"
)

// CallbackDispatcher delivers task callback outbox events with retry state.
type CallbackDispatcher struct {
	repo           tasksvc.Repository
	client         *http.Client
	metrics        *tasksvc.Metrics
	interval       time.Duration
	timeout        time.Duration
	limit          int
	egress         *egressguard.Guard
	secret         []byte
	maxRetry       int
	ownerID        string
	claimTimeout   time.Duration
	maxConcurrency int
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
		repo:           repo,
		client:         client,
		metrics:        metrics,
		interval:       interval,
		timeout:        30 * time.Second,
		limit:          limit,
		maxRetry:       5,
		ownerID:        newCallbackOwnerID(),
		claimTimeout:   2 * time.Minute,
		maxConcurrency: 4,
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

// WithCallbackOwnerID sets the durable claim owner for tests or fixed workers.
func WithCallbackOwnerID(ownerID string) CallbackDispatcherOption {
	return func(j *CallbackDispatcher) {
		if ownerID != "" {
			j.ownerID = ownerID
		}
	}
}

// WithCallbackClaimTimeout sets when processing callback rows may be reclaimed.
func WithCallbackClaimTimeout(timeout time.Duration) CallbackDispatcherOption {
	return func(j *CallbackDispatcher) {
		if timeout > 0 {
			j.claimTimeout = timeout
		}
	}
}

// WithCallbackMaxConcurrency sets the runner-level concurrency for callbacks.
func WithCallbackMaxConcurrency(maxConcurrency int) CallbackDispatcherOption {
	return func(j *CallbackDispatcher) {
		if maxConcurrency > 0 {
			j.maxConcurrency = maxConcurrency
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
	if j.maxConcurrency <= 0 {
		return 1
	}
	return j.maxConcurrency
}

// Run delivers due callback events and schedules retries for failures.
func (j *CallbackDispatcher) Run(ctx context.Context) error {
	if j == nil || j.repo == nil || j.client == nil {
		return nil
	}
	now := time.Now().UTC()
	ownerID := j.claimOwnerID()
	events, err := j.repo.ClaimDueCallbacks(ctx, ownerID, j.claimTimeout, j.limit, now)
	if err != nil {
		return err
	}
	for _, event := range events {
		if err := j.deliverOne(ctx, event, now, ownerID); err != nil {
			return err
		}
	}
	return nil
}

func (j *CallbackDispatcher) deliverOne(ctx context.Context, event tasksvc.CallbackEvent, now time.Time, ownerID string) error {
	if j.egress != nil {
		if err := j.egress.ValidateURL(ctx, event.URL); err != nil {
			return j.markFailure(ctx, event, now, ownerID, "egress_denied", err.Error(), 0, 0)
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, event.URL, bytes.NewReader(event.Payload))
	if err != nil {
		return j.markFailure(ctx, event, now, ownerID, "invalid_request", err.Error(), 0, 0)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Gateway-Task-ID", event.TaskID)
	req.Header.Set("X-Gateway-Callback-ID", event.ID)
	req.Header.Set("X-Gateway-Callback-Delivery-ID", callbackDeliveryID(event))
	j.sign(req, event, now)
	started := time.Now()
	response, err := j.client.Do(req)
	latency := time.Since(started)
	if err != nil {
		return j.markFailure(ctx, event, now, ownerID, "network_error", err.Error(), 0, latency)
	}
	if response.Body != nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
		_ = response.Body.Close()
	}
	statusCode := response.StatusCode
	class := statusClass(statusCode)
	if statusCode < 200 || statusCode >= 300 {
		return j.markFailure(ctx, event, now, ownerID, "http_"+class, response.Status, statusCode, latency)
	}
	if j.metrics != nil {
		j.metrics.RecordCallbackDelivery(class, "success")
	}
	return j.repo.MarkCallbackDelivered(ctx, event.ID, ownerID, statusCode, latency)
}

func (j *CallbackDispatcher) sign(req *http.Request, event tasksvc.CallbackEvent, now time.Time) {
	if len(j.secret) == 0 {
		return
	}
	timestamp := strconv.FormatInt(now.Unix(), 10)
	deliveryID := callbackDeliveryID(event)
	mac := hmac.New(sha256.New, j.secret)
	_, _ = mac.Write([]byte(timestamp))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write([]byte(deliveryID))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write(event.Payload)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	req.Header.Set("X-Gateway-Callback-Timestamp", timestamp)
	req.Header.Set("X-Gateway-Callback-Signature", signature)
	req.Header.Set("X-Gateway-Callback-Signature-Version", "v1")
}

func (j *CallbackDispatcher) markFailure(ctx context.Context, event tasksvc.CallbackEvent, now time.Time, ownerID string, reason string, lastError string, statusCode int, latency time.Duration) error {
	if j.metrics != nil {
		j.metrics.RecordCallbackRetry(reason)
	}
	if j.maxRetry > 0 && event.RetryCount+1 >= j.maxRetry {
		if j.metrics != nil {
			j.metrics.RecordCallbackDelivery(statusClass(statusCode), "dead_letter")
		}
		return j.repo.MarkCallbackDeadLetter(ctx, event.ID, ownerID, lastError, statusCode, latency)
	}
	if j.metrics != nil {
		j.metrics.RecordCallbackDelivery(statusClass(statusCode), "retry")
	}
	return j.repo.MarkCallbackFailed(ctx, event.ID, ownerID, nextCallbackRetry(now, event.RetryCount), lastError, statusCode, latency)
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

func callbackDeliveryID(event tasksvc.CallbackEvent) string {
	if event.DeliveryID != "" {
		return event.DeliveryID
	}
	return event.ID
}

func (j *CallbackDispatcher) claimOwnerID() string {
	if j == nil || j.ownerID == "" {
		return newCallbackOwnerID()
	}
	return j.ownerID + ":" + newCallbackOwnerID()
}

func newCallbackOwnerID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "callback_owner_" + strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return "callback_owner_" + hex.EncodeToString(b[:])
}
