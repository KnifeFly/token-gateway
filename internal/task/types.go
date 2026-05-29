package task

import (
	"encoding/json"
	"time"

	"github.com/KnifeFly/token-gateway/pkg/tokenusage"
)

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
	StatusCanceled  Status = "canceled"
	StatusExpired   Status = "expired"

	KindImageGeneration       Kind              = "image.generation"
	KindImageEdit             Kind              = "image.edit"
	KindVideoGeneration       Kind              = "video.generation"
	KindAudioSpeech           Kind              = "audio.speech"
	KindAudioTranscription    Kind              = "audio.transcription"
	KindMusicGeneration       Kind              = "music.generation"
	ResourceTask              ResourceType      = "task"
	ResourceFile              ResourceType      = "file"
	IdempotencyStatusReserved IdempotencyStatus = "reserved"
	IdempotencyStatusBound    IdempotencyStatus = "bound"

	CallbackStatusPending   CallbackStatus = "pending"
	CallbackStatusDelivered CallbackStatus = "delivered"
	CallbackStatusFailed    CallbackStatus = "failed"
)

// Status is the durable internal async task state.
type Status string

// Kind identifies the unified media operation behind a task.
type Kind string

// ResourceType identifies the resource bound to an idempotency key.
type ResourceType string

// IdempotencyStatus is the durable idempotency record state.
type IdempotencyStatus string

// CallbackStatus is the callback outbox delivery state.
type CallbackStatus string

// Task is the durable async task aggregate used by M4 media workflows.
type Task struct {
	ID             string
	TenantID       string
	ProjectID      string
	APIKeyID       string
	RequestID      string
	IdempotencyKey string
	RequestHash    string
	Kind           Kind
	MediaType      string
	Model          string
	Status         Status
	Progress       int
	ProviderType   string
	ChannelID      string
	ProviderTaskID string
	Input          json.RawMessage
	Result         json.RawMessage
	Usage          tokenusage.Actual
	ErrorCode      string
	ErrorMessage   string
	CallbackURL    string
	Metadata       map[string]string
	BalanceHoldID  string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	CompletedAt    *time.Time
}

// IdempotencyRecord binds a client key and request hash to one resource.
type IdempotencyRecord struct {
	ID             string
	TenantID       string
	APIKeyID       string
	Endpoint       string
	IdempotencyKey string
	RequestHash    string
	ResourceType   ResourceType
	ResourceID     string
	Status         IdempotencyStatus
	ExpiresAt      time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// FileAsset is the normalized file object used by media inputs and results.
type FileAsset struct {
	ID           string
	TenantID     string
	ProjectID    string
	APIKeyID     string
	RequestID    string
	FileName     string
	OriginalName string
	SizeBytes    int64
	MIMEType     string
	UploadPath   string
	FileURL      string
	DownloadURL  string
	Source       string
	CreatedAt    time.Time
	ExpiresAt    *time.Time
}

// FileQuota summarizes file storage usage for one project.
type FileQuota struct {
	MaxFiles       int
	UsedFiles      int
	RemainingFiles int
	MaxBytes       int64
	UsedBytes      int64
}

// CallbackEvent is a retryable callback outbox row.
type CallbackEvent struct {
	ID          string
	TaskID      string
	TenantID    string
	ProjectID   string
	URL         string
	Payload     json.RawMessage
	Status      CallbackStatus
	RetryCount  int
	NextRetryAt time.Time
	LastError   string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ProviderTask is the upstream async task handle returned by a provider.
type ProviderTask struct {
	ExternalID string
	Status     Status
	Progress   int
}

// ProviderTaskResult is the normalized provider polling result.
type ProviderTaskResult struct {
	Status       Status
	Progress     int
	Result       json.RawMessage
	Usage        tokenusage.Actual
	ErrorCode    string
	ErrorMessage string
}
